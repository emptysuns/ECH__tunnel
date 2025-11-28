package main

import (
	"encoding/binary"
	"fmt"
	"log"
	"net"
	"strings"
	"sync"
	"time"
)

// UDP关联结构（使用连接池）
type UDPAssociation struct {
	connID        string
	tcpConn       net.Conn
	udpListener   *net.UDPConn
	clientUDPAddr *net.UDPAddr
	pool          *ECHPool
	mu            sync.Mutex
	closed        bool
	done          chan bool
	connected     chan bool
	receiving     bool
}

// handleUDPRelay 处理UDP数据中继（使用连接池）
func (assoc *UDPAssociation) handleUDPRelay() {
	buffer := make([]byte, 65535)

	for {
		n, srcAddr, err := assoc.udpListener.ReadFromUDP(buffer)
		if err != nil {
			if !isNormalCloseError(err) {
				log.Printf("[UDP:%s] 读取失败: %v", assoc.connID, err)
			}
			assoc.done <- true
			return
		}

		// 第一次收到UDP包时，记录客户端UDP地址
		if assoc.clientUDPAddr == nil {
			assoc.mu.Lock()
			if assoc.clientUDPAddr == nil {
				assoc.clientUDPAddr = srcAddr
				log.Printf("[UDP:%s] 客户端UDP地址: %s", assoc.connID, srcAddr.String())
			}
			assoc.mu.Unlock()
		} else {
			// 验证UDP包来自正确的客户端
			if assoc.clientUDPAddr.String() != srcAddr.String() {
				log.Printf("[UDP:%s] 忽略来自未授权地址的UDP包: %s", assoc.connID, srcAddr.String())
				continue
			}
		}

		log.Printf("[UDP:%s] 收到UDP数据包，大小: %d", assoc.connID, n)

		// 处理UDP数据包
		go assoc.handleUDPPacket(buffer[:n])
	}
}

// handleUDPPacket 处理单个UDP数据包（通过连接池）
func (assoc *UDPAssociation) handleUDPPacket(packet []byte) {
	// 解析SOCKS5 UDP请求头
	target, data, err := parseSOCKS5UDPPacket(packet)
	if err != nil {
		log.Printf("[UDP:%s] 解析UDP数据包失败: %v", assoc.connID, err)
		return
	}

	log.Printf("[UDP:%s] 目标: %s, 数据长度: %d", assoc.connID, target, len(data))

	// 通过连接池发送数据
	if err := assoc.sendUDPData(target, data); err != nil {
		log.Printf("[UDP:%s] 发送数据失败: %v", assoc.connID, err)
		return
	}
}

// sendUDPData 通过连接池发送UDP数据
func (assoc *UDPAssociation) sendUDPData(target string, data []byte) error {
	assoc.mu.Lock()
	defer assoc.mu.Unlock()

	if assoc.closed {
		return fmt.Errorf("关联已关闭")
	}

	// 只在第一次发送时建立连接
	if !assoc.receiving {
		assoc.receiving = true
		// 发送UDP_CONNECT消息（包含目标地址）
		if err := assoc.pool.SendUDPConnect(assoc.connID, target); err != nil {
			return fmt.Errorf("发送UDP_CONNECT失败: %v", err)
		}

		// 等待连接成功
		go func() {
			if !assoc.pool.WaitConnected(assoc.connID, 5*time.Second) {
				log.Printf("[UDP:%s] 连接超时", assoc.connID)
				assoc.done <- true
				return
			}
			log.Printf("[UDP:%s] 连接已建立", assoc.connID)
		}()
	}

	// 发送实际数据
	if err := assoc.pool.SendUDPData(assoc.connID, data); err != nil {
		return fmt.Errorf("发送UDP数据失败: %v", err)
	}

	return nil
}

// handleUDPResponse 处理从WebSocket返回的UDP数据
func (assoc *UDPAssociation) handleUDPResponse(addrData string, data []byte) {
	// 解析地址 "host:port"
	parts := strings.Split(addrData, ":")
	if len(parts) != 2 {
		log.Printf("[UDP:%s] 无效的地址格式: %s", assoc.connID, addrData)
		return
	}

	host := parts[0]
	port := 0
	fmt.Sscanf(parts[1], "%d", &port)

	// 构建SOCKS5 UDP响应包
	packet, err := buildSOCKS5UDPPacket(host, port, data)
	if err != nil {
		log.Printf("[UDP:%s] 构建响应包失败: %v", assoc.connID, err)
		return
	}

	// 发送回客户端
	if assoc.clientUDPAddr != nil {
		assoc.mu.Lock()
		_, err = assoc.udpListener.WriteToUDP(packet, assoc.clientUDPAddr)
		assoc.mu.Unlock()

		if err != nil {
			log.Printf("[UDP:%s] 发送UDP响应失败: %v", assoc.connID, err)
			assoc.done <- true
			return
		}

		log.Printf("[UDP:%s] 已发送UDP响应: %s:%d, 大小: %d", assoc.connID, host, port, len(data))
	}
}

func (assoc *UDPAssociation) IsClosed() bool {
	assoc.mu.Lock()
	defer assoc.mu.Unlock()
	return assoc.closed
}

func (assoc *UDPAssociation) Close() {
	assoc.mu.Lock()
	defer assoc.mu.Unlock()

	if assoc.closed {
		return
	}

	assoc.closed = true

	// 通过连接池关闭UDP连接
	if assoc.pool != nil {
		assoc.pool.SendUDPClose(assoc.connID)
	}

	if assoc.udpListener != nil {
		assoc.udpListener.Close()
	}

	log.Printf("[UDP:%s] 关联资源已清理", assoc.connID)
}

// parseSOCKS5UDPPacket 解析SOCKS5 UDP数据包
func parseSOCKS5UDPPacket(packet []byte) (string, []byte, error) {
	if len(packet) < 10 {
		return "", nil, fmt.Errorf("数据包太短")
	}

	// RSV (2字节) + FRAG (1字节)
	if packet[0] != 0 || packet[1] != 0 {
		return "", nil, fmt.Errorf("无效的RSV字段")
	}

	frag := packet[2]
	if frag != 0 {
		return "", nil, fmt.Errorf("不支持分片 (FRAG=%d)", frag)
	}

	atyp := packet[3]
	offset := 4

	var host string
	switch atyp {
	case IPv4Addr:
		if len(packet) < offset+4 {
			return "", nil, fmt.Errorf("IPv4地址不完整")
		}
		host = net.IP(packet[offset : offset+4]).String()
		offset += 4

	case DomainAddr:
		if len(packet) < offset+1 {
			return "", nil, fmt.Errorf("域名长度字段缺失")
		}
		domainLen := int(packet[offset])
		offset++
		if len(packet) < offset+domainLen {
			return "", nil, fmt.Errorf("域名数据不完整")
		}
		host = string(packet[offset : offset+domainLen])
		offset += domainLen

	case IPv6Addr:
		if len(packet) < offset+16 {
			return "", nil, fmt.Errorf("IPv6地址不完整")
		}
		host = net.IP(packet[offset : offset+16]).String()
		offset += 16

	default:
		return "", nil, fmt.Errorf("不支持的地址类型: %d", atyp)
	}

	// 端口
	if len(packet) < offset+2 {
		return "", nil, fmt.Errorf("端口字段缺失")
	}
	port := int(packet[offset])<<8 | int(packet[offset+1])
	offset += 2

	// 实际数据
	data := packet[offset:]

	var target string
	if atyp == IPv6Addr {
		target = fmt.Sprintf("[%s]:%d", host, port)
	} else {
		target = fmt.Sprintf("%s:%d", host, port)
	}

	return target, data, nil
}

// buildSOCKS5UDPPacket 构建SOCKS5 UDP响应数据包
func buildSOCKS5UDPPacket(host string, port int, data []byte) ([]byte, error) {
	packet := make([]byte, 0, 1024)

	// RSV (2字节) + FRAG (1字节)
	packet = append(packet, 0x00, 0x00, 0x00)

	// 解析地址类型
	ip := net.ParseIP(host)
	if ip != nil {
		if ip4 := ip.To4(); ip4 != nil {
			// IPv4
			packet = append(packet, IPv4Addr)
			packet = append(packet, ip4...)
		} else {
			// IPv6
			packet = append(packet, IPv6Addr)
			packet = append(packet, ip...)
		}
	} else {
		// 域名
		if len(host) > 255 {
			return nil, fmt.Errorf("域名过长")
		}
		packet = append(packet, DomainAddr)
		packet = append(packet, byte(len(host)))
		packet = append(packet, []byte(host)...)
	}

	// 端口
	portBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(portBytes, uint16(port))
	packet = append(packet, portBytes...)

	// 数据
	packet = append(packet, data...)

	return packet, nil
}
