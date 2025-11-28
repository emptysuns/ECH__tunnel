# ECH Tunnel

> **High-Performance Covert Tunnel based on TLS 1.3 Encrypted Client Hello (ECH)**

[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/yourusername/ech_tunnel)](https://goreportcard.com/report/github.com/yourusername/ech_tunnel)

## 📖 Introduction

**ECH Tunnel** is a next-generation tunneling tool designed to bypass network censorship and Deep Packet Inspection (DPI) that relies on SNI (Server Name Indication) sniffing. 

By leveraging **TLS 1.3 Encrypted Client Hello (ECH)**, it encrypts the entire Client Hello handshake message, including the SNI. To an outside observer, your traffic appears to be connecting to a generic, legitimate ECH-enabled provider (e.g., Cloudflare), while it is actually being routed to your private server.

## 🚀 Key Features

- **🛡️ Anti-SNI Sniffing**: Completely hides the target domain name during the TLS handshake, protecting your privacy and preventing SNI-based blocking.
- **⚡ High Performance**:
  - **Multiplexing**: Runs multiple logical connections over a single WebSocket connection to reduce handshake latency.
  - **Connection Pooling**: Pre-establishes connections to minimize setup time.
  - **Adaptive Buffering**: Dynamically adjusts buffer sizes based on network conditions.
  - **Optimized TCP**: Disables Nagle's algorithm and enables Keep-Alive for lower latency.
- **🔌 Multi-Protocol Support**:
  - **SOCKS5 Proxy**: Supports UDP Associate and User/Password authentication.
  - **HTTP/HTTPS Proxy**: Supports CONNECT method and Basic authentication.
  - **TCP Forwarding**: Maps local ports to remote targets transparently.
- **🌐 WebSocket Transport**: Uses standard WebSocket (WSS) protocol to penetrate firewalls and CDNs.

## 🛠️ Usage

### Command Line Arguments

| Flag | Description | Default |
|------|-------------|---------|
| `-l` | **Listen Address**. Determines mode based on prefix (`ws://`, `wss://`, `tcp://`, `proxy://`). | (Required) |
| `-f` | **Forward Address**. The WebSocket server address to connect to (Client mode only). | (Required for Client) |
| `-token` | **Auth Token**. Shared secret between client and server. | `""` |
| `-ech` | **ECH Domain**. The decoy domain used to fetch ECH configs (e.g., a Cloudflare domain). | `cloudflare-ech.com` |
| `-dns` | **DNS Server**. DNS server used to query ECH public keys. | `119.29.29.29:53` |
| `-n` | **Connection Pool**. Number of concurrent WebSocket connections. | `3` |
| `-cert` | TLS Certificate file path (Server only). | Auto-generated |
| `-key` | TLS Key file path (Server only). | Auto-generated |

### Examples

#### 1. Start Server
Run on your remote server.
```bash
# Listen on port 8080 with a secret token
./ech_tunnel -l "wss://0.0.0.0:8080/ws" -token "my_secret_token"
```

#### 2. Start SOCKS5 & HTTP Proxy Client
Run on your local machine.
```bash
# Start proxy on localhost:1080
./ech_tunnel -l "proxy://127.0.0.1:1080" -f "wss://your-server.com/ws" -token "my_secret_token"

# With Authentication (User: admin, Pass: 123456)
./ech_tunnel -l "proxy://admin:123456@127.0.0.1:1080" -f "wss://your-server.com/ws" -token "my_secret_token"
```

#### 3. Start TCP Forwarding Client
Forward a local port to a remote service.
```bash
# Forward local 3306 to remote database at 192.168.1.100:3306
./ech_tunnel -l "tcp://127.0.0.1:3306/192.168.1.100:3306" -f "wss://your-server.com/ws" -token "my_secret_token"
```

---

<details>
<summary><strong>🇨🇳 点击这里查看中文说明 (Click here for Chinese Version)</strong></summary>

# ECH Tunnel (中文介绍)

> **基于 TLS 1.3 Encrypted Client Hello (ECH) 的高性能隐蔽隧道**

## 📖 简介

**ECH Tunnel** 是一款新一代的隧道工具，旨在绕过基于 SNI (Server Name Indication) 嗅探的网络审查和干扰。

通过利用 **TLS 1.3 ECH (Encrypted Client Hello)** 技术，它能够加密包含 SNI 在内的整个 Client Hello 握手消息。在外部观察者看来，您的流量似乎是连接到了一个支持 ECH 的普通公共服务提供商（如 Cloudflare），而实际上流量被安全地路由到了您的私有服务器。

## 🚀 核心特点

- **🛡️ 抗 SNI 阻断**: 在 TLS 握手阶段彻底隐藏真实的目标域名，有效防止防火墙识别和阻断。
- **⚡ 高性能架构**:
  - **多路复用 (Multiplexing)**: 在单条 WebSocket 连接上并发处理多个用户连接，显著降低握手延迟。
  - **连接池**: 预先建立长连接池，减少连接建立时间。
  - **自适应缓冲**: 根据网络状况动态调整内存缓冲区大小，优化吞吐量。
  - **TCP 优化**: 禁用 Nagle 算法，启用 Keep-Alive，降低传输延迟。
- **🔌 多协议支持**:
  - **SOCKS5 代理**: 完整支持 UDP Associate 和用户名/密码认证。
  - **HTTP/HTTPS 代理**: 支持 CONNECT 隧道和 Basic 认证。
  - **TCP 端口转发**: 将本地端口流量透明转发到远程目标。
- **🌐 WebSocket 传输**: 使用标准的 WebSocket (WSS) 协议，具有极佳的防火墙穿透能力。

## 🛠️ 使用方法

### 命令行参数

| 参数 | 说明 | 默认值 |
|------|------|--------|
| `-l` | **监听地址**。根据前缀决定工作模式 (`ws://`, `wss://`, `tcp://`, `proxy://`)。 | (必填) |
| `-f` | **转发地址**。客户端连接的 WebSocket 服务端地址。 | (客户端必填) |
| `-token` | **认证令牌**。客户端和服务端必须保持一致。 | `""` |
| `-ech` | **ECH 诱饵域名**。用于获取 ECH 配置的域名 (通常是 CDN 的域名)。 | `cloudflare-ech.com` |
| `-dns` | **DNS 服务器**。用于查询 ECH 公钥的 DNS。 | `119.29.29.29:53` |
| `-n` | **连接池大小**。保持的 WebSocket 并发连接数。 | `3` |
| `-cert` | TLS 证书文件路径 (仅服务端)。 | 自动生成 |
| `-key` | TLS 密钥文件路径 (仅服务端)。 | 自动生成 |

### 使用示例

#### 1. 启动服务端
在远程服务器上运行。
```bash
# 在 8080 端口监听，设置 Token 为 "my_secret_token"
./ech_tunnel -l "wss://0.0.0.0:8080/ws" -token "my_secret_token"
```

#### 2. 启动 SOCKS5 & HTTP 代理客户端
在本地机器上运行。
```bash
# 在本地 1080 端口开启代理
./ech_tunnel -l "proxy://127.0.0.1:1080" -f "wss://your-server.com/ws" -token "my_secret_token"

# 开启带认证的代理 (用户名: admin, 密码: 123456)
./ech_tunnel -l "proxy://admin:123456@127.0.0.1:1080" -f "wss://your-server.com/ws" -token "my_secret_token"
```

#### 3. 启动 TCP 端口转发客户端
将本地端口映射到远程服务。
```bash
# 将本地 3306 端口转发到远程数据库 192.168.1.100:3306
./ech_tunnel -l "tcp://127.0.0.1:3306/192.168.1.100:3306" -f "wss://your-server.com/ws" -token "my_secret_token"
```

</details>
