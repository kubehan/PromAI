package tools

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
)

// TruncationResult 截断结果
type TruncationResult struct {
	Content            string
	Truncated          bool
	TruncatedBy        string // "lines" | "bytes"
	FirstLineExceeds   bool
	LastLinePartial    bool
	OutputLines        int
	OutputBytes        int
	TotalLines         int
	TotalBytes         int
	TruncatedLineCount int
}

// 默认常量
const (
	DEFAULT_MAX_LINES    = 2000
	DEFAULT_MAX_BYTES    = 50 * 1024 // 50KB
	GREP_MAX_LINE_LENGTH = 2000
)

// TruncateHead 截断文本头部（保留前面，丢弃后面）
func TruncateHead(content string) TruncationResult {
	lines := strings.Split(content, "\n")
	totalLines := len(lines)
	totalBytes := len(content)

	// 计算保留的行数
	keepLines := totalLines
	if keepLines > DEFAULT_MAX_LINES {
		keepLines = DEFAULT_MAX_LINES
	}

	// 构建结果
	resultLines := lines[:keepLines]
	result := strings.Join(resultLines, "\n")

	// 检查是否需要按字节截断
	truncatedBy := ""
	if len(result) > DEFAULT_MAX_BYTES {
		// 需要按字节截断
		truncatedBy = "bytes"
		// 从后往前找到不超过限制的位置
		byteCount := 0
		endIndex := 0
		for i, line := range resultLines {
			lineBytes := len(line) + 1 // +1 for newline
			if byteCount+lineBytes > DEFAULT_MAX_BYTES {
				endIndex = i
				break
			}
			byteCount += lineBytes
			endIndex = i + 1
		}
		resultLines = resultLines[:endIndex]
		result = strings.Join(resultLines, "\n")
	} else if keepLines < totalLines {
		truncatedBy = "lines"
	}

	outputLines := len(resultLines)
	outputBytes := len(result)

	return TruncationResult{
		Content:            result,
		Truncated:          outputLines < totalLines || outputBytes < totalBytes,
		TruncatedBy:        truncatedBy,
		FirstLineExceeds:   false,
		LastLinePartial:    false,
		OutputLines:        outputLines,
		OutputBytes:        outputBytes,
		TotalLines:         totalLines,
		TotalBytes:         totalBytes,
		TruncatedLineCount: totalLines - outputLines,
	}
}

// TruncateTail 截断文本尾部（保留后面，丢弃前面）
func TruncateTail(content string) TruncationResult {
	lines := strings.Split(content, "\n")
	totalLines := len(lines)
	totalBytes := len(content)

	// 从后往前计算需要保留的行
	selectedLines := []string{}
	truncatedBy := ""
	outputBytes := 0

	for i := len(lines) - 1; i >= 0; i-- {
		line := lines[i]
		lineBytes := len(line) + 1 // +1 for newline

		// 检查是否超过字节限制
		if outputBytes+lineBytes > DEFAULT_MAX_BYTES {
			truncatedBy = "bytes"
			break
		}

		selectedLines = append([]string{line}, selectedLines...)
		outputBytes += lineBytes

		// 检查是否超过行数限制
		if len(selectedLines) >= DEFAULT_MAX_LINES {
			truncatedBy = "lines"
			break
		}
	}

	// 重新计算输出字节数
	outputBytes = 0
	for _, line := range selectedLines {
		outputBytes += len(line) + 1
	}
	if len(selectedLines) > 0 {
		outputBytes -= 1 // 减去最后多加的换行符
	}

	result := strings.Join(selectedLines, "\n")

	return TruncationResult{
		Content:            result,
		Truncated:          len(selectedLines) < totalLines || outputBytes < totalBytes,
		TruncatedBy:        truncatedBy,
		OutputLines:        len(selectedLines),
		OutputBytes:        outputBytes,
		TotalLines:         totalLines,
		TotalBytes:         totalBytes,
		LastLinePartial:    totalBytes > outputBytes && len(selectedLines) > 0,
		TruncatedLineCount: totalLines - len(selectedLines),
	}
}

// TruncateLine 截断单行文本
func TruncateLine(line string) (string, bool) {
	if len(line) <= GREP_MAX_LINE_LENGTH {
		return line, false
	}
	return truncateString(line, GREP_MAX_LINE_LENGTH), true
}

// truncateString 截断字符串
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}

// ResolvePath 解析相对路径
func ResolvePath(path, cwd string) (string, error) {
	if filepath.IsAbs(path) {
		return path, nil
	}
	return filepath.Join(cwd, path), nil
}

// max 取最大值
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// min 取最小值
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// FormatSize 格式化文件大小
func FormatSize(bytes int) string {
	const (
		KB = 1024
		MB = 1024 * KB
		GB = 1024 * MB
	)

	switch {
	case bytes >= GB:
		return fmt.Sprintf("%.2f GB", float64(bytes)/GB)
	case bytes >= MB:
		return fmt.Sprintf("%.2f MB", float64(bytes)/MB)
	case bytes >= KB:
		return fmt.Sprintf("%.2f KB", float64(bytes)/KB)
	default:
		return fmt.Sprintf("%d bytes", bytes)
	}
}

// indexOf 查找字符串在切片中的索引
func indexOf(slice []string, item string) int {
	for i, s := range slice {
		if s == item {
			return i
		}
	}
	return -1
}

// removeAtIndex 从切片中移除指定索引的元素
func removeAtIndex(slice []string, index int) []string {
	if index < 0 || index >= len(slice) {
		return slice
	}
	return append(slice[:index], slice[index+1:]...)
}

// reverseString 反转字符串
func reverseString(s string) string {
	runes := []rune(s)
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	return string(runes)
}

// ValidateToolParams 校验并解析工具参数
// 使用 JSON 序列化/反序列化来确保类型正确转换
func ValidateToolParams[T any](params map[string]any) (T, error) {
	var result T

	// 先将 params 序列化为 JSON
	jsonBytes, err := json.Marshal(params)
	if err != nil {
		return result, fmt.Errorf("failed to marshal params: %w", err)
	}

	// 再反序列化到目标结构体
	if err := json.Unmarshal(jsonBytes, &result); err != nil {
		return result, fmt.Errorf("failed to unmarshal params: %w", err)
	}

	// 校验必填字段
	if err := validateRequiredFields(&result); err != nil {
		return result, err
	}

	return result, nil
}

// validateRequiredFields 校验必填字段
func validateRequiredFields[T any](result *T) error {
	v := reflect.ValueOf(result).Elem()
	t := v.Type()

	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		fieldType := t.Field(i)

		// 检查 json tag 中是否有 required 标记
		tag := fieldType.Tag.Get("json")
		if tag == "-" {
			continue
		}

		// 对于字符串类型，检查是否为空
		if field.Kind() == reflect.String && field.String() == "" {
			// 获取字段名（优先使用 json tag 中的名称）
			fieldName := fieldType.Name
			if idx := strings.Index(tag, ","); idx != -1 {
				fieldName = tag[:idx]
			} else if tag != "" {
				fieldName = tag
			}

			// 检查是否有 omitempty
			if !strings.Contains(tag, "omitempty") {
				return fmt.Errorf("missing required field: %s", fieldName)
			}
		}
	}

	return nil
}
