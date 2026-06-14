package session

import (
	"os"
	"path/filepath"
	"time"
)

// GetAgentDir 获取代理目录
func GetAgentDir() string {
	// 从环境变量或默认位置获取
	if dir := os.Getenv("PI_AGENT_DIR"); dir != "" {
		return dir
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".pi")
}

// ResolveConfigValue 解析配置值（支持环境变量）
func ResolveConfigValue(value string) string {
	// 简单的环境变量解析
	if len(value) > 0 && value[0] == '$' {
		envVar := value[1:]
		if envValue := os.Getenv(envVar); envValue != "" {
			return envValue
		}
	}
	return value
}

// ResolveHeaders 解析头部配置
func ResolveHeaders(headers map[string]string) map[string]string {
	if headers == nil {
		return nil
	}
	resolved := make(map[string]string)
	for k, v := range headers {
		resolved[k] = ResolveConfigValue(v)
	}
	return resolved
}

func indexOf(slice []string, item string) int {
	for i, s := range slice {
		if s == item {
			return i
		}
	}
	return -1
}

func removeAtIndex(slice []string, index int) []string {
	if index < 0 || index >= len(slice) {
		return slice
	}
	return append(slice[:index], slice[index+1:]...)
}

func getCurrentTimestamp() int64 {
	return time.Now().UnixMilli()
}

// containsIgnoreCase 检查字符串是否包含子串（忽略大小写）
func containsIgnoreCase(s, substr string) bool {
	sLower := toLower(s)
	substrLower := toLower(substr)
	return contains(sLower, substrLower)
}

// toLower 转换为小写
func toLower(s string) string {
	result := make([]byte, len(s))
	for i, c := range s {
		if c >= 'A' && c <= 'Z' {
			result[i] = byte(c + 32)
		} else {
			result[i] = byte(c)
		}
	}
	return string(result)
}

// contains 检查字符串是否包含子串
func contains(s, substr string) bool {
	if len(substr) > len(s) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// randomInt 生成随机整数（简化实现）
func randomInt(min, max int) int {
	if min >= max {
		return min
	}
	// 简化实现，实际应使用 crypto/rand
	return min + (int(getCurrentTimestamp())%1000)*(max-min)/1000
}