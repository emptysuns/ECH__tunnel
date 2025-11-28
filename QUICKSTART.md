# 快速开始指南

## 📦 获取程序

### 方式 1：下载预编译版本（推荐）

从 [GitHub Releases](https://github.com/你的用户名/ech_tunnel/releases) 页面下载适合您系统的版本：

- **Windows 64位**: `ech-tunnel-windows-amd64.exe`
- **Windows ARM64**: `ech-tunnel-windows-arm64.exe`
- **Linux 64位**: `ech-tunnel-linux-amd64`
- **Linux ARM64**: `ech-tunnel-linux-arm64`
- **macOS Intel**: `ech-tunnel-darwin-amd64`
- **macOS Apple Silicon**: `ech-tunnel-darwin-arm64`

### 方式 2：从源码编译

```bash
# 克隆仓库
git clone https://github.com/你的用户名/ech_tunnel.git
cd ech_tunnel/server

# 编译当前平台
go build -o ech-tunnel

# 或使用编译脚本编译所有平台
./build.sh        # Linux/macOS
.\build.ps1       # Windows
```

---

## 🚀 部署步骤

### 第一步：在服务器上运行

```bash
# 监听 8080 端口，设置 token 为 "my_secret_token"
./ech-tunnel -l "wss://0.0.0.0:8080/tunnel" -token "my_secret_token"
```

**参数说明**:
- `-l`: 监听地址，使用 `wss://` 前缀表示服务端模式
- `-token`: 可选，用于客户端认证

**输出示例**:
```
WebSocket 服务端使用自签名证书启动，监听 0.0.0.0:8080/tunnel
```

---

### 第二步：在本地运行客户端

#### 选项 A：SOCKS5/HTTP 代理模式

```bash
# 启动 SOCKS5 + HTTP 混合代理，监听本地 1080 端口
./ech-tunnel -l "proxy://127.0.0.1:1080" \
             -f "wss://你的服务器IP或域名:8080/tunnel" \
             -token "my_secret_token"
```

**浏览器配置**:
1. 打开浏览器代理设置
2. SOCKS5 代理：`127.0.0.1:1080`
3. 或 HTTP 代理：`127.0.0.1:1080`

#### 选项 B：TCP 端口转发模式

```bash
# 转发本地 3306 端口到远程数据库
./ech-tunnel -l "tcp://127.0.0.1:3306/192.168.1.100:3306" \
             -f "wss://你的服务器IP或域名:8080/tunnel" \
             -token "my_secret_token"
```

---

## 🔧 常用场景

### 场景 1：科学上网

**服务器端** (境外 VPS):
```bash
./ech-tunnel -l "wss://0.0.0.0:443/ws" -token "password123" \
             -cert /path/to/cert.pem -key /path/to/key.pem
```

**客户端** (本地):
```bash
./ech-tunnel -l "proxy://127.0.0.1:1080" \
             -f "wss://your-domain.com:443/ws" \
             -token "password123"
```

### 场景 2：远程数据库访问

**服务器端** (内网网关):
```bash
./ech-tunnel -l "wss://0.0.0.0:8080/db" -token "db_token"
```

**客户端** (办公电脑):
```bash
# MySQL
./ech-tunnel -l "tcp://127.0.0.1:3306/192.168.1.10:3306" \
             -f "wss://gateway.company.com:8080/db" \
             -token "db_token"

# PostgreSQL
./ech-tunnel -l "tcp://127.0.0.1:5432/192.168.1.11:5432" \
             -f "wss://gateway.company.com:8080/db" \
             -token "db_token"
```

### 场景 3：内网穿透

**服务器端** (公网 VPS):
```bash
./ech-tunnel -l "wss://0.0.0.0:8443/nat" -token "nat_secret"
```

**客户端** (内网设备):
```bash
# 将内网 HTTP 服务暴露到公网
./ech-tunnel -l "tcp://0.0.0.0:80/127.0.0.1:8080" \
             -f "wss://vps-ip:8443/nat" \
             -token "nat_secret"
```

---

## 🔒 安全建议

### 使用 TLS 证书

**获取免费证书** (Let's Encrypt):
```bash
# 使用 certbot
sudo certbot certonly --standalone -d your-domain.com
```

**启动服务端**:
```bash
./ech-tunnel -l "wss://0.0.0.0:443/tunnel" \
             -cert /etc/letsencrypt/live/your-domain.com/fullchain.pem \
             -key /etc/letsencrypt/live/your-domain.com/privkey.pem \
             -token "your_strong_token"
```

### 加强认证

```bash
# 设置强密码作为 token
TOKEN=$(openssl rand -hex 32)
echo "Token: $TOKEN"

# 服务端
./ech-tunnel -l "wss://0.0.0.0:443/ws" -token "$TOKEN"

# 客户端
./ech-tunnel -l "proxy://127.0.0.1:1080" \
             -f "wss://your-server.com:443/ws" \
             -token "$TOKEN"
```

### 限制来源 IP

```bash
# 仅允许特定 IP 段访问
./ech-tunnel -l "wss://0.0.0.0:443/ws" \
             -cidr "1.2.3.0/24,10.0.0.0/8" \
             -token "secure_token"
```

---

## 🛠️ 命令行参数完整列表

```bash
./ech-tunnel --help
```

| 参数 | 说明 | 示例 |
|------|------|------|
| `-l` | 监听地址 | `wss://0.0.0.0:8080/ws` |
| `-f` | 转发地址（客户端） | `wss://server.com:8080/ws` |
| `-token` | 认证令牌 | `my_secret_token` |
| `-cert` | TLS 证书路径 | `/path/to/cert.pem` |
| `-key` | TLS 密钥路径 | `/path/to/key.pem` |
| `-cidr` | 允许的 IP 范围 | `192.168.0.0/16` |
| `-dns` | DNS 服务器 | `8.8.8.8:53` |
| `-ech` | ECH 域名 | `cloudflare-ech.com` |
| `-n` | 连接池大小 | `5` |
| `-ip` | 指定解析 IP | `1.2.3.4` |
| `-version` | 显示版本信息 | - |

---

## 📊 查看版本信息

```bash
./ech-tunnel -version
```

输出示例:
```
ECH Tunnel v1.0.0
Git Commit: a1b2c3d
Build Time: 2025-11-28_06:24:00
Go Version: go1.21.5
OS/Arch: linux/amd64
```

---

## 🐛 故障排除

### 问题 1：连接失败

**检查步骤**:
```bash
# 1. 测试服务端是否可达
telnet your-server.com 8080

# 2. 检查防火墙
sudo ufw status
sudo firewall-cmd --list-all

# 3. 查看服务端日志
./ech-tunnel -l "wss://0.0.0.0:8080/ws" -token "test"
```

### 问题 2：ECH 配置获取失败

**解决方案**:
```bash
# 更换 DNS 服务器
./ech-tunnel -l "proxy://127.0.0.1:1080" \
             -f "wss://server.com:8080/ws" \
             -dns "8.8.8.8:53"

# 或使用其他 ECH 域名
./ech-tunnel -l "proxy://127.0.0.1:1080" \
             -f "wss://server.com:8080/ws" \
             -ech "cloudflare.com"
```

### 问题 3：性能不佳

**优化方案**:
```bash
# 增加连接池大小
./ech-tunnel -l "proxy://127.0.0.1:1080" \
             -f "wss://server.com:8080/ws" \
             -n 10
```

---

## 📚 更多文档

- [完整 README](README.md) - 详细介绍和原理说明
- [编译文档](BUILD.md) - 如何从源码编译
- [代码审查报告](CODE_REVIEW.md) - 代码质量分析

---

## 💬 获取帮助

- 提交 Issue: https://github.com/你的用户名/ech_tunnel/issues
- 查看 Wiki: https://github.com/你的用户名/ech_tunnel/wiki
