package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"runtime"
	"strings"
)

// 版本信息（由编译时注入）
var (
	Version   = "dev"
	GitCommit = "unknown"
	BuildTime = "unknown"
)

var showVersion bool

func init() {
	flag.BoolVar(&showVersion, "version", false, "显示版本信息")
	flag.BoolVar(&showVersion, "v", false, "显示版本信息（简写）")

	flag.StringVar(&listenAddr, "l", "", "监听地址 (tcp://监听1/目标1,监听2/目标2,... 或 ws://ip:port/path 或 wss://ip:port/path 或 proxy://[user:pass@]ip:port)")
	flag.StringVar(&forwardAddr, "f", "", "服务地址 (格式: wss://host:port/path)")
	flag.StringVar(&ipAddr, "ip", "", "指定解析的IP地址（仅客户端：将 wss 主机名定向到该 IP 连接）")
	flag.StringVar(&certFile, "cert", "", "TLS证书文件路径（默认:自动生成，仅服务端）")
	flag.StringVar(&keyFile, "key", "", "TLS密钥文件路径（默认:自动生成，仅服务端）")
	flag.StringVar(&token, "token", "", "身份验证令牌（WebSocket Subprotocol）")
	flag.StringVar(&cidrs, "cidr", "0.0.0.0/0,::/0", "允许的来源 IP 范围 (CIDR),多个范围用逗号分隔")
	flag.StringVar(&dnsServer, "dns", "119.29.29.29:53", "查询 ECH 公钥所用的 DNS 服务器")
	flag.StringVar(&echDomain, "ech", "cloudflare-ech.com", "用于查询 ECH 公钥的域名")
	flag.IntVar(&connectionNum, "n", 3, "WebSocket连接数量")
}

func main() {
	flag.Parse()

	// 处理版本信息显示
	if showVersion {
		fmt.Printf("ECH Tunnel %s\n", Version)
		fmt.Printf("Git Commit: %s\n", GitCommit)
		fmt.Printf("Build Time: %s\n", BuildTime)
		fmt.Printf("Go Version: %s\n", runtime.Version())
		fmt.Printf("OS/Arch: %s/%s\n", runtime.GOOS, runtime.GOARCH)
		os.Exit(0)
	}

	log.Println("[系统] 启动暴力优化模式: 自适应缓冲 + 激进拥塞控制")

	if strings.HasPrefix(listenAddr, "ws://") || strings.HasPrefix(listenAddr, "wss://") {
		runWebSocketServer(listenAddr)
		return
	}
	if strings.HasPrefix(listenAddr, "tcp://") {
		// 客户端模式：预先获取 ECH 公钥（失败则直接退出，严格禁止回退）
		if err := prepareECH(); err != nil {
			log.Fatalf("[客户端] 获取 ECH 公钥失败: %v", err)
		}
		runTCPClient(listenAddr, forwardAddr)
		return
	}
	if strings.HasPrefix(listenAddr, "proxy://") {
		// 代理模式（支持 SOCKS5 和 HTTP）：预先获取 ECH 公钥
		if err := prepareECH(); err != nil {
			log.Fatalf("[代理] 获取 ECH 公钥失败: %v", err)
		}
		runProxyServer(listenAddr, forwardAddr)
		return
	}

	log.Fatal("监听地址格式错误，请使用 ws://, wss://, tcp:// 或 proxy:// 前缀")
}
