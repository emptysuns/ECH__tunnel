package main

import (
	"fmt"
	"log"
	"net"

	"github.com/google/uuid"
)

// handleSOCKS5UDPAssociate 处理 SOCKS5 UDP ASSOCIATE 命令
func handleSOCKS5UDPAssociate(conn net.Conn, clientAddr string, config *ProxyConfig) error {
	// 创建 UDP 监听器
	udpListener, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	if err != nil {
		sendSOCKS5ErrorResponse(conn, GeneralFailure)
		return fmt.Errorf("创建 UDP 监听器失败: %v", err)
	}

	// 获取绑定的地址
	boundAddr := udpListener.LocalAddr().(*net.UDPAddr)
	log.Printf("[SOCKS5:%s] UDP ASSOCIATE 绑定到 %s", clientAddr, boundAddr.String())

	// 发送成功响应（包含绑定地址）
	response := []byte{0x05, Succeeded, 0x00, IPv4Addr}
	response = append(response, boundAddr.IP.To4()...)
	response = append(response, byte(boundAddr.Port>>8), byte(boundAddr.Port))
	if _, err := conn.Write(response); err != nil {
		udpListener.Close()
		return fmt.Errorf("发送响应失败: %v", err)
	}

	// 创建 UDP 关联
	connID := uuid.New().String()
	assoc := &UDPAssociation{
		connID:      connID,
		tcpConn:     conn,
		udpListener: udpListener,
		pool:        echPool,
		done:        make(chan bool, 1),
		connected:   make(chan bool, 1),
	}

	// 注册到连接池
	echPool.RegisterUDP(connID, assoc)

	// 启动 UDP 中继
	go assoc.handleUDPRelay()

	// 监控 TCP 连接（SOCKS5 规范：TCP 断开时终止 UDP 关联）
	buf := make([]byte, 1)
	for {
		_, err := conn.Read(buf)
		if err != nil {
			log.Printf("[SOCKS5:%s] TCP 连接断开，关闭 UDP 关联", clientAddr)
			break
		}
	}

	// 清理
	assoc.Close()
	return nil
}
