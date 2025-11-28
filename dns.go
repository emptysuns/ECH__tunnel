package main

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"time"
)

// ======================== ECH 相关（客户端） ========================

const (
	typeHTTPS = 65 // DNS HTTPS 记录类型
)

// 客户端启动时查询 ECH 配置并缓存
func prepareECH() error {
	for {
		var echBase64 string
		var err error

		// 优先使用 DoH (DNS over HTTPS) 查询,绕过软路由DNS重定向
		log.Printf("[客户端] 使用 DoH 查询 ECH: %s", echDomain)
		echBase64, err = queryHTTPSRecordViaDoH(echDomain)

		if err != nil {
			log.Printf("[客户端] DoH 查询失败: %v，尝试传统DNS查询...", err)
			// DoH失败,回退到传统UDP DNS查询
			log.Printf("[客户端] 使用 DNS 服务器查询 ECH: %s -> %s", dnsServer, echDomain)
			echBase64, err = queryHTTPSRecord(echDomain, dnsServer)
			if err != nil {
				log.Printf("[客户端] DNS 查询失败: %v，2秒后重试...", err)
				time.Sleep(2 * time.Second)
				continue
			}
		}

		if echBase64 == "" {
			log.Printf("[客户端] 未找到 ECH 参数（HTTPS RR key=echconfig/5），2秒后重试...")
			time.Sleep(2 * time.Second)
			continue
		}
		raw, err := base64.StdEncoding.DecodeString(echBase64)
		if err != nil {
			log.Printf("[客户端] ECH Base64 解码失败: %v，2秒后重试...", err)
			time.Sleep(2 * time.Second)
			continue
		}
		echListMu.Lock()
		echList = raw
		echListMu.Unlock()
		log.Printf("[客户端] ECHConfigList 长度: %d 字节", len(raw))
		return nil
	}
}

// 刷新 ECH 配置（用于重试）
func refreshECH() error {
	log.Printf("[ECH] 刷新 ECH 公钥配置...")
	return prepareECH()
}

func getECHList() ([]byte, error) {
	echListMu.RLock()
	defer echListMu.RUnlock()
	if len(echList) == 0 {
		return nil, errors.New("ECH 配置尚未加载")
	}
	return echList, nil
}

// queryHTTPSRecordViaDoH 使用 DoH (DNS over HTTPS) 查询 HTTPS 记录
// 优点：可以绕过软路由的 DNS 重定向（53/UDP 端口拦截）
func queryHTTPSRecordViaDoH(domain string) (string, error) {
	// 使用 DNSPod 的 DoH 服务 (国内访问更快)
	const dohURL = "https://doh.pub/dns-query"

	// 构建标准的 DNS 查询包
	query := buildDNSQuery(domain, typeHTTPS)

	// 创建 HTTP 客户端，设置合理超时
	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	// 发送 POST 请求（DNS wireformat over HTTPS）
	req, err := http.NewRequest("POST", dohURL, bytes.NewReader(query))
	if err != nil {
		return "", fmt.Errorf("创建 DoH 请求失败: %w", err)
	}

	// 设置必需的 HTTP 头部
	req.Header.Set("Content-Type", "application/dns-message")
	req.Header.Set("Accept", "application/dns-message")

	// 执行请求
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("DoH 请求失败: %w", err)
	}
	defer resp.Body.Close()

	// 检查 HTTP 状态码
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("DoH 服务器返回错误状态码: %d", resp.StatusCode)
	}

	// 读取 DNS 响应数据
	response, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取 DoH 响应失败: %w", err)
	}

	// 解析 DNS 响应，提取 ECH 配置
	return parseDNSResponse(response)
}

func queryHTTPSRecord(domain, dnsServer string) (string, error) {
	query := buildDNSQuery(domain, typeHTTPS)

	conn, err := net.Dial("udp", dnsServer)
	if err != nil {
		return "", fmt.Errorf("连接 DNS 服务器失败: %v", err)
	}
	defer conn.Close()

	// 设置 2 秒超时
	conn.SetDeadline(time.Now().Add(2 * time.Second))

	if _, err = conn.Write(query); err != nil {
		return "", fmt.Errorf("发送查询失败: %v", err)
	}

	response := make([]byte, 4096)
	n, err := conn.Read(response)
	if err != nil {
		if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
			return "", fmt.Errorf("DNS 查询超时")
		}
		return "", fmt.Errorf("读取 DNS 响应失败: %v", err)
	}
	return parseDNSResponse(response[:n])
}

func buildDNSQuery(domain string, qtype uint16) []byte {
	query := make([]byte, 0, 512)
	// Header
	query = append(query, 0x00, 0x01)                         // ID
	query = append(query, 0x01, 0x00)                         // 标准查询
	query = append(query, 0x00, 0x01)                         // QDCOUNT = 1
	query = append(query, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00) // AN/NS/AR = 0
	// QNAME
	for _, label := range strings.Split(domain, ".") {
		query = append(query, byte(len(label)))
		query = append(query, []byte(label)...)
	}
	query = append(query, 0x00) // root
	// QTYPE/QCLASS
	query = append(query, byte(qtype>>8), byte(qtype))
	query = append(query, 0x00, 0x01) // IN
	return query
}

func parseDNSResponse(response []byte) (string, error) {
	if len(response) < 12 {
		return "", fmt.Errorf("响应长度无效")
	}
	ancount := binary.BigEndian.Uint16(response[6:8])
	if ancount == 0 {
		return "", fmt.Errorf("未找到回答记录")
	}
	// 跳过 Question
	offset := 12
	for offset < len(response) && response[offset] != 0 {
		offset += int(response[offset]) + 1
	}
	offset += 5 // null + type + class

	// Answers
	for i := 0; i < int(ancount); i++ {
		if offset >= len(response) {
			break
		}
		// NAME（可能压缩）
		if response[offset]&0xC0 == 0xC0 {
			offset += 2
		} else {
			for offset < len(response) && response[offset] != 0 {
				offset += int(response[offset]) + 1
			}
			offset++
		}
		if offset+10 > len(response) {
			break
		}
		rrType := binary.BigEndian.Uint16(response[offset : offset+2])
		offset += 8 // type(2) + class(2) + ttl(4)
		dataLen := binary.BigEndian.Uint16(response[offset : offset+2])
		offset += 2
		if offset+int(dataLen) > len(response) {
			break
		}
		data := response[offset : offset+int(dataLen)]
		offset += int(dataLen)

		if rrType == typeHTTPS {
			if ech := parseHTTPSRecord(data); ech != "" {
				return ech, nil
			}
		}
	}
	return "", nil
}

// 仅抽取 SvcParamKey == 5 (ECHConfigList/echconfig)
func parseHTTPSRecord(data []byte) string {
	if len(data) < 2 {
		return ""
	}
	// 跳 priority(2)
	offset := 2
	// 跳 targetName
	if offset < len(data) && data[offset] == 0 {
		offset++
	} else {
		for offset < len(data) && data[offset] != 0 {
			offset += int(data[offset]) + 1
		}
		offset++
	}
	// SvcParams
	for offset+4 <= len(data) {
		key := binary.BigEndian.Uint16(data[offset : offset+2])
		length := binary.BigEndian.Uint16(data[offset+2 : offset+4])
		offset += 4
		if offset+int(length) > len(data) {
			break
		}
		value := data[offset : offset+int(length)]
		offset += int(length)
		if key == 5 {
			return base64.StdEncoding.EncodeToString(value)
		}
	}
	return ""
}
