#!/bin/bash

# ECH Tunnel 多平台编译脚本

set -e

# 版本信息
VERSION=$(git describe --tags --always 2>/dev/null || echo "dev")
BUILD_TIME=$(date -u '+%Y-%m-%d_%H:%M:%S')
GIT_COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")

# 编译选项
LDFLAGS="-s -w -X main.Version=${VERSION} -X main.GitCommit=${GIT_COMMIT} -X main.BuildTime=${BUILD_TIME}"

# 输出目录
OUTPUT_DIR="dist"
rm -rf ${OUTPUT_DIR}
mkdir -p ${OUTPUT_DIR}

# 定义编译目标
declare -a PLATFORMS=(
    "windows/amd64"
    "windows/arm64"
    "linux/amd64"
    "linux/arm64"
    "darwin/amd64"
    "darwin/arm64"
    "freebsd/amd64"
    "freebsd/arm64"
)

echo "========================================"
echo "  ECH Tunnel 多平台编译"
echo "  版本: ${VERSION}"
echo "  提交: ${GIT_COMMIT}"
echo "  时间: ${BUILD_TIME}"
echo "========================================"
echo ""

# 编译函数
build() {
    local os=$1
    local arch=$2
    local output_name="ech-tunnel-${os}-${arch}"
    
    if [ "$os" = "windows" ]; then
        output_name="${output_name}.exe"
    fi
    
    echo "📦 编译 ${os}/${arch}..."
    
    CGO_ENABLED=0 GOOS=${os} GOARCH=${arch} go build \
        -trimpath \
        -ldflags="${LDFLAGS}" \
        -o "${OUTPUT_DIR}/${output_name}" \
        .
    
    if [ $? -eq 0 ]; then
        echo "✅ ${output_name} 编译成功"
        
        # 计算文件大小
        if [ "$os" = "darwin" ]; then
            size=$(ls -lh "${OUTPUT_DIR}/${output_name}" | awk '{print $5}')
        else
            size=$(du -h "${OUTPUT_DIR}/${output_name}" | cut -f1)
        fi
        echo "   大小: ${size}"
    else
        echo "❌ ${output_name} 编译失败"
        return 1
    fi
    echo ""
}

# 执行编译
for platform in "${PLATFORMS[@]}"; do
    IFS='/' read -r os arch <<< "$platform"
    build "$os" "$arch"
done

echo "========================================"
echo "✨ 编译完成！"
echo "输出目录: ${OUTPUT_DIR}/"
echo "========================================"
ls -lh ${OUTPUT_DIR}/
