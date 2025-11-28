package main

import (
	"sync"
)

// ======================== 全局参数 ========================

var (
	listenAddr    string
	forwardAddr   string
	ipAddr        string
	certFile      string
	keyFile       string
	token         string
	cidrs         string
	connectionNum int

	// 新增 ECH/DNS 参数
	dnsServer string // -dns
	echDomain string // -ech

	// 运行期缓存的 ECHConfigList
	echListMu sync.RWMutex
	echList   []byte

	// 多通道连接池
	echPool *ECHPool

	// 性能优化: 内存池
	bufferPool = sync.Pool{
		New: func() interface{} {
			// 使用 1MB 超大缓冲区以提升峰值性能
			buf := make([]byte, 1048576) // 1MB
			return &buf
		},
	}

	// 性能优化: 小缓冲区池(用于协议头等)
	smallBufferPool = sync.Pool{
		New: func() interface{} {
			buf := make([]byte, 4096) // 4KB
			return &buf
		},
	}
)
