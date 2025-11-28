package main

import (
	"io"
	"strings"
)

// isNormalCloseError 判断错误是否为正常的网络连接关闭
// 正常关闭包括：EOF、连接已关闭、管道损坏、对端重置连接等
// 这些错误不需要记录为异常错误日志
func isNormalCloseError(err error) bool {
	if err == nil {
		return false
	}

	// EOF 是最常见的正常关闭标志
	if err == io.EOF {
		return true
	}

	// 检查其他常见的正常关闭模式
	errStr := err.Error()
	return strings.Contains(errStr, "use of closed network connection") ||
		strings.Contains(errStr, "broken pipe") ||
		strings.Contains(errStr, "connection reset by peer") ||
		strings.Contains(errStr, "normal closure")
}
