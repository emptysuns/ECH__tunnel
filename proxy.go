package main

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
)

// ======================== 代理服务器（SOCKS5 + HTTP） ========================

// SOCKS5 认证方法常量
const (
	NoAuth       = uint8(0x00)
	UserPassAuth = uint8(0x02)
	NoAcceptable = uint8(0xFF)
)

// SOCKS5 请求命令
const (
	ConnectCmd      = uint8(0x01)
	BindCmd         = uint8(0x02)
	UDPAssociateCmd = uint8(0x03)
)

// SOCKS5 地址类型
const (
	IPv4Addr   = uint8(0x01)
	DomainAddr = uint8(0x03)
	IPv6Addr   = uint8(0x04)
)

// SOCKS5 响应状态码
const (
	Succeeded               = uint8(0x00)
	GeneralFailure          = uint8(0x01)
	ConnectionNotAllowed    = uint8(0x02)
	NetworkUnreachable      = uint8(0x03)
	HostUnreachable         = uint8(0x04)
	ConnectionRefused       = uint8(0x05)
	TTLExpired              = uint8(0x06)
	CommandNotSupported     = uint8(0x07)
	AddressTypeNotSupported = uint8(0x08)
)

type ProxyConfig struct {
	Username string
	Password string
	Host     string
}

func parseProxyAddr(addr string) (*ProxyConfig, error) {
	// 格式: proxy://[user:pass@]ip:port
	addr = strings.TrimPrefix(addr, "proxy://")

	config := &ProxyConfig{}

	// 检查是否有认证信息
	if strings.Contains(addr, "@") {
		parts := strings.SplitN(addr, "@", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("无效的代理地址格式")
		}

		// 解析用户名密码
		auth := parts[0]
		if strings.Contains(auth, ":") {
			authParts := strings.SplitN(auth, ":", 2)
			config.Username = authParts[0]
			config.Password = authParts[1]
		}

		config.Host = parts[1]
	} else {
		config.Host = addr
	}

	return config, nil
}

func runProxyServer(addr, wsServerAddr string) {
	if wsServerAddr == "" {
		log.Fatal("代理服务器需要指定 WebSocket 服务端地址 (-f)")
	}

	// 验证必须使用 wss://（强制 ECH）
	u, err := url.Parse(wsServerAddr)
	if err != nil {
		log.Fatalf("解析 WebSocket 服务端地址失败: %v", err)
	}
	if u.Scheme != "wss" {
		log.Fatalf("[代理] 仅支持 wss://（客户端必须使用 ECH/TLS1.3）")
	}

	config, err := parseProxyAddr(addr)
	if err != nil {
		log.Fatalf("解析代理地址失败: %v", err)
	}

	listener, err := net.Listen("tcp", config.Host)
	if err != nil {
		log.Fatalf("代理监听失败 %s: %v", config.Host, err)
	}
	defer listener.Close()

	log.Printf("代理服务器启动（支持 SOCKS5 和 HTTP）监听: %s", config.Host)
	if config.Username != "" {
		log.Printf("代理认证已启用，用户名: %s", config.Username)
	}

	echPool = NewECHPool(wsServerAddr, connectionNum)
	echPool.Start()

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("接受连接失败: %v", err)
			continue
		}

		go handleProxyConnection(conn, config)
	}
}

func handleProxyConnection(conn net.Conn, config *ProxyConfig) {
	defer conn.Close()

	clientAddr := conn.RemoteAddr().String()
	log.Printf("[代理:%s] 新连接", clientAddr)

	// 设置连接超时
	conn.SetDeadline(time.Now().Add(30 * time.Second))

	// 读取第一个字节判断协议类型
	buf := make([]byte, 1)
	if _, err := io.ReadFull(conn, buf); err != nil {
		log.Printf("[代理:%s] 读取第一个字节失败: %v", clientAddr, err)
		return
	}

	firstByte := buf[0]

	// SOCKS5: 第一个字节是 0x05
	if firstByte == 0x05 {
		log.Printf("[代理:%s] 检测到 SOCKS5 协议", clientAddr)
		handleSOCKS5Protocol(conn, config, clientAddr)
		return
	}

	// HTTP: 第一个字节是字母 (GET, POST, CONNECT, HEAD, PUT, DELETE, OPTIONS, PATCH)
	if firstByte == 'G' || firstByte == 'P' || firstByte == 'C' || firstByte == 'H' ||
		firstByte == 'D' || firstByte == 'O' {
		log.Printf("[代理:%s] 检测到 HTTP 协议", clientAddr)
		handleHTTPProtocol(conn, config, clientAddr, firstByte)
		return
	}

	log.Printf("[代理:%s] 未知协议，第一个字节: 0x%02X", clientAddr, firstByte)
}

// ======================== SOCKS5 协议处理 ========================

func handleSOCKS5Protocol(conn net.Conn, config *ProxyConfig, clientAddr string) {
	// 处理认证方法协商（需要读取剩余的认证方法）
	buf := make([]byte, 1)
	if _, err := io.ReadFull(conn, buf); err != nil {
		log.Printf("[SOCKS5:%s] 读取认证方法数量失败: %v", clientAddr, err)
		return
	}
	nMethods := buf[0]

	methods := make([]byte, nMethods)
	if _, err := io.ReadFull(conn, methods); err != nil {
		log.Printf("[SOCKS5:%s] 读取认证方法失败: %v", clientAddr, err)
		return
	}

	// 选择认证方法
	var method uint8 = NoAuth
	if config.Username != "" && config.Password != "" {
		method = UserPassAuth
		found := false
		for _, m := range methods {
			if m == UserPassAuth {
				found = true
				break
			}
		}
		if !found {
			method = NoAcceptable
		}
	}

	// 发送选择的认证方法
	response := []byte{0x05, method}
	if _, err := conn.Write(response); err != nil {
		log.Printf("[SOCKS5:%s] 发送认证方法响应失败: %v", clientAddr, err)
		return
	}

	if method == NoAcceptable {
		log.Printf("[SOCKS5:%s] 没有可接受的认证方法", clientAddr)
		return
	}

	// 处理用户名密码认证
	if method == UserPassAuth {
		if err := handleSOCKS5UserPassAuth(conn, config); err != nil {
			log.Printf("[SOCKS5:%s] 用户名密码认证失败: %v", clientAddr, err)
			return
		}
	}

	// 处理客户端请求
	if err := handleSOCKS5Request(conn, clientAddr, config); err != nil {
		log.Printf("[SOCKS5:%s] 处理请求失败: %v", clientAddr, err)
		return
	}
}

func handleSOCKS5UserPassAuth(conn net.Conn, config *ProxyConfig) error {
	buf := make([]byte, 2)
	if _, err := io.ReadFull(conn, buf); err != nil {
		return fmt.Errorf("读取用户名密码认证头失败: %v", err)
	}

	version := buf[0]
	userLen := buf[1]

	if version != 1 {
		return fmt.Errorf("不支持的认证版本: %d", version)
	}

	// 读取用户名
	userBuf := make([]byte, userLen)
	if _, err := io.ReadFull(conn, userBuf); err != nil {
		return fmt.Errorf("读取用户名失败: %v", err)
	}

	// 读取密码长度
	passLenBuf := make([]byte, 1)
	if _, err := io.ReadFull(conn, passLenBuf); err != nil {
		return fmt.Errorf("读取密码长度失败: %v", err)
	}
	passLen := passLenBuf[0]

	// 读取密码
	passBuf := make([]byte, passLen)
	if _, err := io.ReadFull(conn, passBuf); err != nil {
		return fmt.Errorf("读取密码失败: %v", err)
	}

	// 验证用户名密码
	user := string(userBuf)
	pass := string(passBuf)

	var status byte = 0x00 // 0x00表示成功
	if user != config.Username || pass != config.Password {
		status = 0x01 // 认证失败
	}

	// 发送认证结果
	response := []byte{0x01, status}
	if _, err := conn.Write(response); err != nil {
		return fmt.Errorf("发送认证响应失败: %v", err)
	}

	if status != 0x00 {
		return fmt.Errorf("用户名或密码错误")
	}

	return nil
}

func handleSOCKS5Request(conn net.Conn, clientAddr string, config *ProxyConfig) error {
	// 读取请求头
	buf := make([]byte, 4)
	if _, err := io.ReadFull(conn, buf); err != nil {
		return fmt.Errorf("读取请求头失败: %v", err)
	}

	version := buf[0]
	command := buf[1]
	atyp := buf[3]

	if version != 5 {
		return fmt.Errorf("不支持的SOCKS版本: %d", version)
	}

	// 读取目标地址
	var host string
	switch atyp {
	case IPv4Addr:
		buf = make([]byte, 4)
		if _, err := io.ReadFull(conn, buf); err != nil {
			return fmt.Errorf("读取IPv4地址失败: %v", err)
		}
		host = net.IP(buf).String()

	case DomainAddr:
		buf = make([]byte, 1)
		if _, err := io.ReadFull(conn, buf); err != nil {
			return fmt.Errorf("读取域名长度失败: %v", err)
		}
		domainLen := buf[0]
		buf = make([]byte, domainLen)
		if _, err := io.ReadFull(conn, buf); err != nil {
			return fmt.Errorf("读取域名失败: %v", err)
		}
		host = string(buf)

	case IPv6Addr:
		buf = make([]byte, 16)
		if _, err := io.ReadFull(conn, buf); err != nil {
			return fmt.Errorf("读取IPv6地址失败: %v", err)
		}
		host = net.IP(buf).String()

	default:
		sendSOCKS5ErrorResponse(conn, AddressTypeNotSupported)
		return fmt.Errorf("不支持的地址类型: %d", atyp)
	}

	// 读取端口
	buf = make([]byte, 2)
	if _, err := io.ReadFull(conn, buf); err != nil {
		return fmt.Errorf("读取端口失败: %v", err)
	}
	port := int(buf[0])<<8 | int(buf[1])

	// 目标地址
	var target string
	if atyp == IPv6Addr {
		target = fmt.Sprintf("[%s]:%d", host, port)
	} else {
		target = fmt.Sprintf("%s:%d", host, port)
	}

	log.Printf("[SOCKS5:%s] 请求访问目标: %s (命令: %d)", clientAddr, target, command)

	// 处理不同的命令
	switch command {
	case ConnectCmd:
		return handleSOCKS5Connect(conn, target, clientAddr)
	case UDPAssociateCmd:
		return handleSOCKS5UDPAssociate(conn, clientAddr, config)
	case BindCmd:
		sendSOCKS5ErrorResponse(conn, CommandNotSupported)
		return fmt.Errorf("BIND命令暂不支持")
	default:
		sendSOCKS5ErrorResponse(conn, CommandNotSupported)
		return fmt.Errorf("不支持的命令类型: %d", command)
	}
}

func sendSOCKS5ErrorResponse(conn net.Conn, status uint8) {
	response := []byte{0x05, status, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
	conn.Write(response)
}

func sendSOCKS5SuccessResponse(conn net.Conn) error {
	// 简单返回成功响应（绑定地址为 0.0.0.0:0）
	response := []byte{0x05, Succeeded, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
	_, err := conn.Write(response)
	return err
}

func handleSOCKS5Connect(conn net.Conn, target, clientAddr string) error {
	connID := uuid.New().String()
	_ = conn.SetDeadline(time.Time{})
	_ = conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
	buffer := make([]byte, 32768)
	n, _ := conn.Read(buffer)
	_ = conn.SetReadDeadline(time.Time{})
	first := ""
	if n > 0 {
		first = string(buffer[:n])
	}

	echPool.RegisterAndClaim(connID, target, first, conn)
	if !echPool.WaitConnected(connID, 5*time.Second) {
		sendSOCKS5ErrorResponse(conn, GeneralFailure)
		return fmt.Errorf("SOCKS5 CONNECT 超时")
	}
	if err := sendSOCKS5SuccessResponse(conn); err != nil {
		return fmt.Errorf("发送SOCKS5成功响应失败: %v", err)
	}

	defer func() {
		_ = echPool.SendClose(connID)
		_ = conn.Close()
		echPool.mu.Lock()
		delete(echPool.tcpMap, connID)
		echPool.mu.Unlock()
		log.Printf("[SOCKS5:%s] 连接断开，已发送 CLOSE 通知", clientAddr)
	}()

	buf := make([]byte, 32768)
	for {
		n, err := conn.Read(buf)
		if err != nil {
			return nil
		}
		if err := echPool.SendData(connID, buf[:n]); err != nil {
			log.Printf("[SOCKS5] 发送数据到通道失败: %v", err)
			return err
		}
	}
}

// ======================== HTTP 代理协议处理 ========================

func handleHTTPProtocol(conn net.Conn, config *ProxyConfig, clientAddr string, firstByte byte) {
	// 读取完整的第一行（HTTP 请求行）
	reader := bufio.NewReader(io.MultiReader(bytes.NewReader([]byte{firstByte}), conn))

	// 读取请求行
	requestLine, err := reader.ReadString('\n')
	if err != nil {
		log.Printf("[HTTP:%s] 读取请求行失败: %v", clientAddr, err)
		return
	}

	// 解析请求行: METHOD URL HTTP/VERSION
	parts := strings.SplitN(strings.TrimSpace(requestLine), " ", 3)
	if len(parts) != 3 {
		log.Printf("[HTTP:%s] 无效的请求行: %s", clientAddr, requestLine)
		return
	}

	method := parts[0]
	requestURL := parts[1]

	log.Printf("[HTTP:%s] %s %s", clientAddr, method, requestURL)

	// CONNECT 方法：建立隧道
	if method == "CONNECT" {
		handleHTTPConnect(conn, reader, config, clientAddr, requestURL)
		return
	}

	// 其他方法（GET, POST 等）：转发 HTTP 请求
	handleHTTPForward(conn, reader, config, clientAddr, method, requestURL)
}

// handleHTTPConnect 处理 HTTP CONNECT 方法（用于 HTTPS）
func handleHTTPConnect(conn net.Conn, reader *bufio.Reader, config *ProxyConfig, clientAddr, target string) {
	log.Printf("[HTTP:%s] CONNECT 到 %s", clientAddr, target)

	// 读取并验证请求头（包括认证）
	headers, err := readHTTPHeaders(reader)
	if err != nil {
		log.Printf("[HTTP:%s] 读取请求头失败: %v", clientAddr, err)
		conn.Write([]byte("HTTP/1.1 400 Bad Request\r\n\r\n"))
		return
	}

	// 验证认证（如果配置了）
	if config.Username != "" && config.Password != "" {
		authHeader := headers["Proxy-Authorization"]
		if !validateProxyAuth(authHeader, config.Username, config.Password) {
			log.Printf("[HTTP:%s] 认证失败", clientAddr)
			conn.Write([]byte("HTTP/1.1 407 Proxy Authentication Required\r\nProxy-Authenticate: Basic realm=\"Proxy\"\r\n\r\n"))
			return
		}
	}

	// 使用连接池建立连接
	connID := uuid.New().String()
	_ = conn.SetDeadline(time.Time{})

	echPool.RegisterAndClaim(connID, target, "", conn)
	if !echPool.WaitConnected(connID, 5*time.Second) {
		log.Printf("[HTTP:%s] CONNECT 超时", clientAddr)
		conn.Write([]byte("HTTP/1.1 504 Gateway Timeout\r\n\r\n"))
		return
	}

	// 发送成功响应
	_, err = conn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n"))
	if err != nil {
		log.Printf("[HTTP:%s] 发送响应失败: %v", clientAddr, err)
		return
	}

	log.Printf("[HTTP:%s] CONNECT 隧道已建立到 %s", clientAddr, target)

	defer func() {
		_ = echPool.SendClose(connID)
		_ = conn.Close()
		echPool.mu.Lock()
		delete(echPool.tcpMap, connID)
		echPool.mu.Unlock()
		log.Printf("[HTTP:%s] CONNECT 隧道关闭", clientAddr)
	}()

	// 转发数据
	buf := make([]byte, 32768)
	for {
		n, err := conn.Read(buf)
		if err != nil {
			return
		}
		if err := echPool.SendData(connID, buf[:n]); err != nil {
			log.Printf("[HTTP:%s] 发送数据失败: %v", clientAddr, err)
			return
		}
	}
}

// handleHTTPForward 处理普通 HTTP 请求（GET, POST 等）
func handleHTTPForward(conn net.Conn, reader *bufio.Reader, config *ProxyConfig, clientAddr, method, requestURL string) {
	log.Printf("[HTTP:%s] 转发 %s %s", clientAddr, method, requestURL)

	// 解析目标 URL
	parsedURL, err := url.Parse(requestURL)
	if err != nil {
		log.Printf("[HTTP:%s] 解析 URL 失败: %v", clientAddr, err)
		conn.Write([]byte("HTTP/1.1 400 Bad Request\r\n\r\n"))
		return
	}

	// 读取请求头
	headers, err := readHTTPHeaders(reader)
	if err != nil {
		log.Printf("[HTTP:%s] 读取请求头失败: %v", clientAddr, err)
		conn.Write([]byte("HTTP/1.1 400 Bad Request\r\n\r\n"))
		return
	}

	// 验证认证（如果配置了）
	if config.Username != "" && config.Password != "" {
		authHeader := headers["Proxy-Authorization"]
		if !validateProxyAuth(authHeader, config.Username, config.Password) {
			log.Printf("[HTTP:%s] 认证失败", clientAddr)
			conn.Write([]byte("HTTP/1.1 407 Proxy Authentication Required\r\nProxy-Authenticate: Basic realm=\"Proxy\"\r\n\r\n"))
			return
		}
	}

	// 确定目标地址
	target := parsedURL.Host
	if !strings.Contains(target, ":") {
		if parsedURL.Scheme == "https" {
			target += ":443"
		} else {
			target += ":80"
		}
	}

	// 读取请求体（如果有）
	var bodyData []byte
	if contentLength, ok := headers["Content-Length"]; ok {
		var length int
		fmt.Sscanf(contentLength, "%d", &length)
		if length > 0 && length < 10*1024*1024 { // 限制最大 10MB
			bodyData = make([]byte, length)
			_, err := io.ReadFull(reader, bodyData)
			if err != nil {
				log.Printf("[HTTP:%s] 读取请求体失败: %v", clientAddr, err)
				conn.Write([]byte("HTTP/1.1 400 Bad Request\r\n\r\n"))
				return
			}
		}
	}

	// 构建转发请求
	var requestBuffer bytes.Buffer

	// 修改请求行：使用相对路径
	path := parsedURL.Path
	if path == "" {
		path = "/"
	}
	if parsedURL.RawQuery != "" {
		path += "?" + parsedURL.RawQuery
	}
	requestBuffer.WriteString(fmt.Sprintf("%s %s HTTP/1.1\r\n", method, path))

	// 写入请求头（移除代理相关头部）
	for key, value := range headers {
		if key != "Proxy-Authorization" && key != "Proxy-Connection" {
			requestBuffer.WriteString(fmt.Sprintf("%s: %s\r\n", key, value))
		}
	}

	// 确保有 Host 头
	if _, ok := headers["Host"]; !ok {
		requestBuffer.WriteString(fmt.Sprintf("Host: %s\r\n", parsedURL.Host))
	}

	requestBuffer.WriteString("\r\n")

	// 写入请求体
	if len(bodyData) > 0 {
		requestBuffer.Write(bodyData)
	}

	firstFrameData := requestBuffer.String()

	// 使用连接池建立连接
	connID := uuid.New().String()
	_ = conn.SetDeadline(time.Time{})

	echPool.RegisterAndClaim(connID, target, firstFrameData, conn)
	if !echPool.WaitConnected(connID, 5*time.Second) {
		log.Printf("[HTTP:%s] 连接超时", clientAddr)
		conn.Write([]byte("HTTP/1.1 504 Gateway Timeout\r\n\r\n"))
		return
	}

	log.Printf("[HTTP:%s] 请求已转发到 %s", clientAddr, target)

	defer func() {
		_ = echPool.SendClose(connID)
		_ = conn.Close()
		echPool.mu.Lock()
		delete(echPool.tcpMap, connID)
		echPool.mu.Unlock()
		log.Printf("[HTTP:%s] 请求处理完成", clientAddr)
	}()

	// 等待响应（响应会通过连接池返回到 conn）
	// 这里只需要保持连接，直到任一方关闭
	buf := make([]byte, 32768)
	for {
		n, err := conn.Read(buf)
		if err != nil {
			return
		}
		// 客户端发送的后续数据（如果有）也转发
		if err := echPool.SendData(connID, buf[:n]); err != nil {
			log.Printf("[HTTP:%s] 发送数据失败: %v", clientAddr, err)
			return
		}
	}
}

// readHTTPHeaders 读取 HTTP 请求头
func readHTTPHeaders(reader *bufio.Reader) (map[string]string, error) {
	headers := make(map[string]string)

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return nil, err
		}

		line = strings.TrimSpace(line)
		if line == "" {
			break // 空行表示头部结束
		}

		// 解析头部：Key: Value
		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			headers[key] = value
		}
	}

	return headers, nil
}

// validateProxyAuth 验证 HTTP 代理认证
func validateProxyAuth(authHeader, username, password string) bool {
	if authHeader == "" {
		return false
	}

	// 解析 Basic 认证：Basic <base64>
	const prefix = "Basic "
	if !strings.HasPrefix(authHeader, prefix) {
		return false
	}

	encoded := strings.TrimPrefix(authHeader, prefix)
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return false
	}

	// 格式：username:password
	credentials := string(decoded)
	parts := strings.SplitN(credentials, ":", 2)
	if len(parts) != 2 {
		return false
	}

	return parts[0] == username && parts[1] == password
}
