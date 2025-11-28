# ECH Tunnel 多平台编译脚本 (PowerShell)

$ErrorActionPreference = "Stop"

# 版本信息
try {
    $VERSION = git describe --tags --always 2>$null
    $GIT_COMMIT = git rev-parse --short HEAD 2>$null
} catch {
    $VERSION = "dev"
    $GIT_COMMIT = "unknown"
}
$BUILD_TIME = Get-Date -Format "yyyy-MM-dd_HH:mm:ss" -AsUTC

# 编译选项
$LDFLAGS = "-s -w -X main.Version=$VERSION -X main.GitCommit=$GIT_COMMIT -X main.BuildTime=$BUILD_TIME"

# 输出目录
$OUTPUT_DIR = "dist"
if (Test-Path $OUTPUT_DIR) {
    Remove-Item -Recurse -Force $OUTPUT_DIR
}
New-Item -ItemType Directory -Force -Path $OUTPUT_DIR | Out-Null

# 编译目标
$PLATFORMS = @(
    @{OS="windows"; Arch="amd64"},
    @{OS="windows"; Arch="arm64"},
    @{OS="linux";   Arch="amd64"},
    @{OS="linux";   Arch="arm64"},
    @{OS="darwin";  Arch="amd64"},
    @{OS="darwin";  Arch="arm64"}
)

Write-Host "========================================" -ForegroundColor Cyan
Write-Host "  ECH Tunnel 多平台编译" -ForegroundColor Cyan
Write-Host "  版本: $VERSION" -ForegroundColor Green
Write-Host "  提交: $GIT_COMMIT" -ForegroundColor Green
Write-Host "  时间: $BUILD_TIME" -ForegroundColor Green
Write-Host "========================================" -ForegroundColor Cyan
Write-Host ""

# 编译函数
function Build-Target {
    param(
        [string]$OS,
        [string]$Arch
    )
    
    $OutputName = "ech-tunnel-$OS-$Arch"
    if ($OS -eq "windows") {
        $OutputName = "$OutputName.exe"
    }
    
    Write-Host "📦 编译 $OS/$Arch..." -ForegroundColor Yellow
    
    $env:CGO_ENABLED = "0"
    $env:GOOS = $OS
    $env:GOARCH = $Arch
    
    go build -trimpath -ldflags=$LDFLAGS -o "$OUTPUT_DIR\$OutputName" .
    
    if ($LASTEXITCODE -eq 0) {
        Write-Host "✅ $OutputName 编译成功" -ForegroundColor Green
        $Size = (Get-Item "$OUTPUT_DIR\$OutputName").Length / 1MB
        Write-Host "   大小: $([Math]::Round($Size, 2)) MB" -ForegroundColor Gray
    } else {
        Write-Host "❌ $OutputName 编译失败" -ForegroundColor Red
        exit 1
    }
    Write-Host ""
}

# 执行编译
foreach ($Platform in $PLATFORMS) {
    Build-Target -OS $Platform.OS -Arch $Platform.Arch
}

Write-Host "========================================" -ForegroundColor Cyan
Write-Host "✨ 编译完成！" -ForegroundColor Green
Write-Host "输出目录: $OUTPUT_DIR\" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
Get-ChildItem $OUTPUT_DIR | Format-Table Name, @{Label="Size (MB)"; Expression={[Math]::Round($_.Length/1MB, 2)}}
