# 自动化编译部署完成 ✅

## 📦 已创建的文件

### 1. **编译文档**
- `BUILD.md` - 详细的编译说明和疑难解答

### 2. **编译脚本**
- `build.sh` - Linux/macOS 多平台编译脚本
- `build.ps1` - Windows PowerShell 编译脚本

### 3. **GitHub Actions 工作流**
- `.github/workflows/build.yml` - 自动化 CI/CD 配置

### 4. **快速开始指南**
- `QUICKSTART.md` - 部署和使用教程

---

## 🚀 如何使用

### 本地编译

#### Linux/macOS:
```bash
cd server
chmod +x build.sh
./build.sh
```

#### Windows (PowerShell):
```powershell
cd server
.\build.ps1
```

编译结果保存在 `dist/` 目录，包括：
- Windows (amd64, arm64)
- Linux (amd64, arm64)
- macOS (amd64, arm64)
- FreeBSD (amd64, arm64)

---

## 🔄 GitHub Actions 自动构建

### 触发条件

自动构建会在以下情况触发：

1. **推送代码到主分支**
   ```bash
   git push origin main
   ```

2. **推送标签（发布新版本）**
   ```bash
   git tag v1.0.0
   git push origin v1.0.0
   ```

3. **创建 Pull Request**

4. **手动触发** - 在 GitHub 仓库的 Actions 页面点击 "Run workflow"

### 构建流程

```
代码变动
  ↓
GitHub Actions 检测
  ↓
并行编译 8 个平台
  ├─ Windows amd64
  ├─ Windows arm64
  ├─ Linux amd64
  ├─ Linux arm64
  ├─ macOS amd64
  ├─ macOS arm64
  ├─ FreeBSD amd64
  └─ FreeBSD arm64
  ↓
生成 SHA256 校验文件
  ↓
上传为 Artifacts (保留 7 天)
  ↓
[如果是 Tag 推送]
创建 GitHub Release
  ├─ 自动生成发布说明
  ├─ 附加所有二进制文件
  └─ 附加 SHA256 校验文件
```

---

## 📋 发布新版本流程

### 1. 准备发布

```bash
# 确保所有改动已提交
git add .
git commit -m "feat: 添加新功能"
git push origin main

# 确认构建通过
# 访问 https://github.com/你的用户名/ech_tunnel/actions
```

### 2. 打标签并推送

```bash
# 创建版本标签 (遵循语义化版本)
git tag -a v1.0.0 -m "Release v1.0.0

新功能:
- 支持 SOCKS5 UDP Associate
- 修复二进制数据传输问题
- 优化代码结构和注释
"

# 推送标签到 GitHub
git push origin v1.0.0
```

### 3. 等待自动构建

- GitHub Actions 会自动开始构建
- 大约 5-10 分钟后完成
- 所有平台的二进制文件会自动上传到 Releases 页面

### 4. 编辑发布说明（可选）

访问 `https://github.com/你的用户名/ech_tunnel/releases`，可以进一步编辑发布说明。

---

## 🔍 版本信息

程序内置了版本显示功能：

```bash
./ech-tunnel -version
```

输出：
```
ECH Tunnel v1.0.0
Git Commit: a1b2c3d
Build Time: 2025-11-28_06:24:00
Go Version: go1.21.5
OS/Arch: linux/amd64
```

版本信息在编译时自动注入，无需手动修改代码。

---

## 📊 支持的平台和架构

| 操作系统 | 架构 | 文件名示例 |
|---------|------|-----------|
| Windows | amd64 | `ech-tunnel-windows-amd64.exe` |
| Windows | arm64 | `ech-tunnel-windows-arm64.exe` |
| Linux | amd64 | `ech-tunnel-linux-amd64` |
| Linux | arm64 | `ech-tunnel-linux-arm64` |
| macOS | amd64 | `ech-tunnel-darwin-amd64` |
| macOS | arm64 | `ech-tunnel-darwin-arm64` |
| FreeBSD | amd64 | `ech-tunnel-freebsd-amd64` |
| FreeBSD | arm64 | `ech-tunnel-freebsd-arm64` |

---

## ⚙️ 高级配置

### 自定义 GitHub Actions

编辑 `.github/workflows/build.yml` 可以：

1. **添加更多平台**
   ```yaml
   - os: openbsd
     arch: amd64
     runner: ubuntu-latest
   ```

2. **修改 Go 版本**
   ```yaml
   go-version: '1.22'  # 使用更新的 Go 版本
   ```

3. **启用代码测试**
   ```yaml
   - name: Run tests
     run: go test -v ./...
   ```

### 优化编译参数

编辑 `build.sh` 或 `build.ps1`，修改 `LDFLAGS`：

```bash
# 添加更多编译标志
LDFLAGS="-s -w -X main.Version=${VERSION} -X main.Author=YourName"
```

---

## 🔐 安全注意事项

### 1. Token 保护

如果需要使用 GitHub Secrets（如发布到私有仓库）：

```yaml
# .github/workflows/build.yml
env:
  GITHUB_TOKEN: ${{ secrets.CUSTOM_TOKEN }}
```

### 2. 代码签名（可选）

为 Windows 程序添加数字签名：

```yaml
- name: Sign Windows binary
  if: matrix.os == 'windows'
  run: |
    signtool sign /f cert.pfx /p ${{ secrets.CERT_PASSWORD }} dist/*.exe
```

---

## 📝 变更日志

建议维护 `CHANGELOG.md` 文件记录版本变更：

```markdown
# Changelog

## [1.0.0] - 2025-11-28

### Added
- 完整的 SOCKS5/HTTP 代理功能
- UDP Associate 支持
- 自动化 CI/CD 构建

### Fixed
- 修复二进制数据传输错误
- 改进错误处理

### Changed
- 优化代码注释和结构
- 提升性能参数
```

---

## ✅ 验证编译产物

### 校验文件完整性

每个发布的二进制文件都带有 SHA256 校验：

**Linux/macOS**:
```bash
sha256sum -c ech-tunnel-linux-amd64.sha256
```

**Windows**:
```powershell
$hash = (Get-FileHash ech-tunnel-windows-amd64.exe).Hash
$expected = (Get-Content ech-tunnel-windows-amd64.exe.sha256).Split()[0]
$hash -eq $expected
```

---

## 🎯 下一步

1. **测试编译脚本**
   ```bash
   ./build.sh  # 确保本地编译成功
   ```

2. **推送到 GitHub**
   ```bash
   git add .
   git commit -m "chore: 添加自动化构建配置"
   git push origin main
   ```

3. **观察首次构建**
   - 访问 GitHub Actions 页面
   - 确认所有平台构建成功

4. **发布首个版本**
   ```bash
   git tag v1.0.0
   git push origin v1.0.0
   ```

---

## 📚 相关文档

- [README.md](README.md) - 项目介绍
- [BUILD.md](BUILD.md) - 详细编译说明
- [QUICKSTART.md](QUICKSTART.md) - 快速开始指南
- [CODE_REVIEW.md](CODE_REVIEW.md) - 代码审查报告

---

**🎉 自动化编译系统已就绪！**
