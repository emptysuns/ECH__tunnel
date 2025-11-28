# 编译说明

## 快速开始

### 前置要求
- Go 1.21 或更高版本
- Git (用于克隆仓库)

### 本地编译

#### 1. 编译当前平台版本
```bash
cd server
go build -o ech-tunnel
```

#### 2. 编译指定平台版本

**Windows (64位)**
```bash
GOOS=windows GOARCH=amd64 go build -o ech-tunnel-windows-amd64.exe
```

**Windows (ARM64)**
```bash
GOOS=windows GOARCH=arm64 go build -o ech-tunnel-windows-arm64.exe
```

**Linux (64位)**
```bash
GOOS=linux GOARCH=amd64 go build -o ech-tunnel-linux-amd64
```

**Linux (ARM64)**
```bash
GOOS=linux GOARCH=arm64 go build -o ech-tunnel-linux-arm64
```

**macOS (Intel)**
```bash
GOOS=darwin GOARCH=amd64 go build -o ech-tunnel-darwin-amd64
```

**macOS (Apple Silicon)**
```bash
GOOS=darwin GOARCH=arm64 go build -o ech-tunnel-darwin-arm64
```

#### 3. 编译所有平台（使用脚本）

**Linux/macOS**
```bash
chmod +x build.sh
./build.sh
```

**Windows (PowerShell)**
```powershell
.\build.ps1
```

编译结果将保存在 `dist/` 目录下。

---

## 编译优化

### 减小二进制体积
```bash
go build -ldflags="-s -w" -o ech-tunnel
```
- `-s`: 去除符号表
- `-w`: 去除 DWARF 调试信息

### 添加版本信息
```bash
VERSION=$(git describe --tags --always)
BUILD_TIME=$(date -u '+%Y-%m-%d_%H:%M:%S')
go build -ldflags="-X main.Version=${VERSION} -X main.BuildTime=${BUILD_TIME}" -o ech-tunnel
```

### 静态编译 (Linux)
```bash
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -installsuffix cgo -o ech-tunnel
```

---

## 依赖管理

### 下载依赖
```bash
go mod download
```

### 更新依赖
```bash
go get -u ./...
go mod tidy
```

### 验证依赖
```bash
go mod verify
```

---

## 自动化构建

本项目配置了 GitHub Actions，当代码推送到仓库时会自动编译以下平台：

- ✅ Windows (amd64, arm64)
- ✅ Linux (amd64, arm64)
- ✅ macOS (amd64, arm64)
- ✅ FreeBSD (amd64, arm64)

构建产物会自动上传到 GitHub Releases（当推送 tag 时）。

---

## 疑难解答

### 问题：依赖下载失败
**解决方案**：配置 Go 代理
```bash
# 国内用户推荐
go env -w GOPROXY=https://goproxy.cn,direct
```

### 问题：编译速度慢
**解决方案**：启用编译缓存
```bash
go env -w GOCACHE=/path/to/cache
```

### 问题：交叉编译 CGO 项目失败
**解决方案**：本项目不依赖 CGO，禁用即可
```bash
CGO_ENABLED=0 go build
```
