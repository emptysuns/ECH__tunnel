package main

import (
	"fmt"
	"log"
	"net"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

// ======================== TCP 正向转发客户端（采用 ECH） ========================

func runTCPClient(listenForwardAddr, wsServerAddr string) {
	// 移除 tcp:// 前缀
	rulesStr := strings.TrimPrefix(listenForwardAddr, "tcp://")

	// 按逗号分割多个规则
	rules := strings.Split(rulesStr, ",")

	if len(rules) == 0 {
		log.Fatal("TCP 地址格式错误，应为 tcp://监听地址/目标地址[,监听地址/目标地址...]")
	}

	if wsServerAddr == "" {
		log.Fatal("TCP 正向转发客户端需要指定 WebSocket 服务端地址 (-f)")
	}

	u, err := url.Parse(wsServerAddr)
	if err != nil {
		log.Fatalf("[客户端] 无效的 WebSocket 服务端地址: %v", err)
	}
	if u.Scheme != "wss" {
		log.Fatalf("[客户端] 仅支持 wss://（客户端必须使用 ECH/TLS1.3）")
	}

	echPool = NewECHPool(wsServerAddr, connectionNum)
	echPool.Start()

	var wg sync.WaitGroup

	// 为每个规则启动监听器（多通道模型：启动固定数量的 WebSocket 长连接池）
	for _, rule := range rules {
		rule = strings.TrimSpace(rule)
		if rule == "" {
			continue
		}

		parts := strings.Split(rule, "/")
		if len(parts) != 2 {
			log.Fatalf("规则格式错误: %s，应为 监听地址/目标地址", rule)
		}

		listenAddress := strings.TrimSpace(parts[0])
		targetAddress := strings.TrimSpace(parts[1])

		wg.Add(1)
		go func(listen, target string) {
			defer wg.Done()
			startMultiChannelTCPForwarder(listen, target, echPool)
		}(listenAddress, targetAddress)

		log.Printf("[客户端] 已添加转发规则: %s -> %s", listenAddress, targetAddress)
	}

	log.Printf("[客户端] 共启动 %d 个TCP转发监听器(多通道)", len(rules))

	// 等待所有监听器
	wg.Wait()
}

func startMultiChannelTCPForwarder(listenAddress, targetAddress string, pool *ECHPool) {
	listener, err := net.Listen("tcp", listenAddress)
	if err != nil {
		log.Fatalf("TCP监听失败 %s: %v", listenAddress, err)
	}
	log.Printf("[客户端] TCP正向转发(多通道)监听: %s -> %s", listenAddress, targetAddress)

	// 复用全局池

	// 接受 TCP 连接
	for {
		tcpConn, err := listener.Accept()
		if err != nil {
			if !strings.Contains(err.Error(), "use of closed network connection") {
				log.Printf("[客户端] 接受TCP连接失败 %s: %v", listenAddress, err)
			}
			return
		}

		connID := uuid.New().String()
		log.Printf("[客户端] 新的TCP连接 %s，连接ID: %s", tcpConn.RemoteAddr(), connID)

		// 读取第一帧
		_ = tcpConn.SetReadDeadline(time.Now().Add(5 * time.Second))
		buffer := make([]byte, 32768)
		n, _ := tcpConn.Read(buffer)
		_ = tcpConn.SetReadDeadline(time.Time{})
		first := ""
		if n > 0 {
			first = string(buffer[:n])
		}

		pool.RegisterAndClaim(connID, targetAddress, first, tcpConn)

		if !pool.WaitConnected(connID, 5*time.Second) {
			log.Printf("[客户端] 连接 %s 建立超时，关闭", connID)
			_ = tcpConn.Close()
			continue
		}

		// 性能优化: 设置TCP参数
		if tcpConnReal, ok := tcpConn.(*net.TCPConn); ok {
			_ = tcpConnReal.SetNoDelay(true)
			_ = tcpConnReal.SetKeepAlive(true)
			_ = tcpConnReal.SetKeepAlivePeriod(30 * time.Second)
			_ = tcpConnReal.SetReadBuffer(1048576)
			_ = tcpConnReal.SetWriteBuffer(1048576)
		}

		go func(cID string, c net.Conn) {
			defer func() {
				_ = pool.SendClose(cID)
				_ = c.Close()
			}()

			// 集成自适应监控
			monitor := NewAdaptiveMonitor()

			for {
				// 自适应调整缓冲区大小 (现在固定为 1MB)
				currentBufSize := monitor.GetBufferSize()
				var buf []byte
				var bufPtr *[]byte

				if currentBufSize == 1048576 {
					bufPtr = bufferPool.Get().(*[]byte)
					buf = *bufPtr
				} else {
					buf = make([]byte, currentBufSize)
				}

				n, err := c.Read(buf)

				// 归还缓冲区
				if bufPtr != nil {
					bufferPool.Put(bufPtr)
				}

				if err != nil {
					return
				}

				// 更新监控数据
				monitor.Update(n)

				if err := pool.SendData(cID, buf[:n]); err != nil {
					log.Printf("[客户端] 发送数据到通道失败: %v", err)
					return
				}
			}
		}(connID, tcpConn)
	}
}

// dialWebSocketWithECH 建立 WebSocket 连接（带 ECH 重试）
func dialWebSocketWithECH(wsServerAddr string, maxRetries int) (*websocket.Conn, error) {
	u, err := url.Parse(wsServerAddr)
	if err != nil {
		return nil, fmt.Errorf("解析 wsServerAddr 失败: %v", err)
	}
	serverName := u.Hostname()

	for attempt := 1; attempt <= maxRetries; attempt++ {
		echBytes, echErr := getECHList()
		if echErr != nil {
			log.Printf("[ECH] 获取 ECH 配置失败: %v", echErr)
			if attempt < maxRetries {
				log.Printf("[ECH] 尝试刷新 ECH 配置...")
				if refreshErr := refreshECH(); refreshErr != nil {
					log.Printf("[ECH] 刷新失败: %v", refreshErr)
				}
				continue
			}
			return nil, fmt.Errorf("ECH 配置不可用: %v", echErr)
		}

		tlsCfg, tlsErr := buildTLSConfigWithECH(serverName, echBytes)
		if tlsErr != nil {
			return nil, fmt.Errorf("构建 TLS(ECH) 配置失败: %v", tlsErr)
		}

		// 性能优化: 配置WebSocket Dialer
		dialer := websocket.Dialer{
			TLSClientConfig: tlsCfg,
			Subprotocols: func() []string {
				if token == "" {
					return nil
				}
				return []string{token}
			}(),
			HandshakeTimeout:  10 * time.Second,
			ReadBufferSize:    1048576, // 1MB
			WriteBufferSize:   1048576, // 1MB
			EnableCompression: true,    // 启用压缩
		}

		// 如果指定了IP地址，配置自定义拨号器（SNI 仍为 serverName）
		if ipAddr != "" {
			dialer.NetDial = func(network, address string) (net.Conn, error) {
				_, port, err := net.SplitHostPort(address)
				if err != nil {
					return nil, err
				}
				address = net.JoinHostPort(ipAddr, port)
				return net.DialTimeout(network, address, 10*time.Second)
			}
		}

		// 连接到WebSocket服务端（必须 wss）
		wsConn, _, dialErr := dialer.Dial(wsServerAddr, nil)
		if dialErr != nil {
			// 检查是否为 ECH 相关错误
			if strings.Contains(dialErr.Error(), "ECH") || strings.Contains(dialErr.Error(), "ech") {
				log.Printf("[ECH] 连接失败（可能 ECH 公钥已轮换）: %v", dialErr)
				if attempt < maxRetries {
					log.Printf("[ECH] 尝试刷新 ECH 配置并重试 (尝试 %d/%d)...", attempt, maxRetries)
					if refreshErr := refreshECH(); refreshErr != nil {
						log.Printf("[ECH] 刷新失败: %v", refreshErr)
					}
					time.Sleep(time.Second)
					continue
				}
			}
			return nil, dialErr
		}

		return wsConn, nil
	}

	return nil, fmt.Errorf("WebSocket 连接失败，已达最大重试次数")
}
