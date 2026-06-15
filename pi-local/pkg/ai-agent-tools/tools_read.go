package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jay-y/pi/pkg/ai"
	agent "github.com/jay-y/pi/pkg/ai-agent"
)

// ReadToolInput 读取工具的输入参数
type ReadToolInput struct {
	Path   string `json:"path"`
	Offset *int   `json:"offset,omitempty"`
	Limit  *int   `json:"limit,omitempty"`
}

// ReadToolDetails 读取工具的详细信息
type ReadToolDetails struct {
	Truncation *TruncationResult `json:"truncation,omitempty"`
}

// ReadOperations 读取工具的操作接口
type ReadOperations interface {
	ReadFile(absolutePath string) ([]byte, error)
	Access(absolutePath string) error
	DetectImageMimeType(absolutePath string) (string, error)
}

// DefaultReadOperations 默认的读取操作
type DefaultReadOperations struct{}

// ReadFile 读取文件内容
func (d *DefaultReadOperations) ReadFile(absolutePath string) ([]byte, error) {
	return os.ReadFile(absolutePath)
}

// Access 检查文件是否可读
func (d *DefaultReadOperations) Access(absolutePath string) error {
	_, err := os.ReadFile(absolutePath)
	return err
}

// DetectImageMimeType 检测图片的 MIME 类型
func (d *DefaultReadOperations) DetectImageMimeType(absolutePath string) (string, error) {
	// 简单实现：根据文件扩展名判断
	ext := strings.ToLower(filepath.Ext(absolutePath))
	switch ext {
	case ".jpg", ".jpeg":
		return "image/jpeg", nil
	case ".png":
		return "image/png", nil
	case ".gif":
		return "image/gif", nil
	case ".webp":
		return "image/webp", nil
	default:
		return "", nil
	}
}

// ReadToolOptions 读取工具的选项
// type ReadToolOptions struct {
// 	AutoResizeImages bool
// 	Operations       ReadOperations
// }

// ReadTool Read 工具
type ReadTool struct {
	cwd             string
	operations       ReadOperations
	autoResizeImages bool
}

func NewReadTool(cwd string, options ...ReadToolOption) agent.AgentTool {
	tool := &ReadTool{
		cwd:             cwd,
		operations:       &DefaultReadOperations{},
		autoResizeImages: true,
	}
	for _, opt := range options {
		if opt != nil { opt(tool) }
	}
	return agent.NewAgentTool(tool)
}

type ReadToolOption func(*ReadTool)

func WithReadOperations(ops ReadOperations) ReadToolOption {
	return func(t *ReadTool) {
		t.operations = ops
	}
}

func WithAutoResizeImages(autoResize bool) ReadToolOption {
	return func(t *ReadTool) {
		t.autoResizeImages = autoResize
	}
}

func (t *ReadTool) GetName() string { return "read" }
func (t *ReadTool) GetLabel() string { return "read" }
func (t *ReadTool) GetDescription() string {
	return fmt.Sprintf("Read the contents of a file. Supports text files and images (jpg, png, gif, webp). Images are sent as attachments. For text files, output is truncated to %d lines or %dKB (whichever is hit first). Use offset/limit for large files. When you need the full file, continue with offset until complete.", DEFAULT_MAX_LINES, DEFAULT_MAX_BYTES/1024)
}
func (t *ReadTool) GetParameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "Path to file to read",
			},
			"offset": map[string]any{
				"type":        "number",
				"description": "Line number to start reading from (1-indexed)",
			},
			"limit": map[string]any{
				"type":        "number",
				"description": "Maximum number of lines to read",
			},
		},
		"required": []string{"path"},
	}
}

func (t *ReadTool) Execute(ctx context.Context, params map[string]any, onUpdate func(partialResult *agent.AgentToolResult)) (*agent.AgentToolResult, error) {
	input, err := ValidateToolParams[ReadToolInput](params)
	if err != nil {
		return nil, err
	}
	
	path := input.Path
	if path == "" {
		return nil, fmt.Errorf("path is required")
	}
	
	var offset, limit int
	if input.Offset != nil {
		offset = *input.Offset
	}
	if input.Limit != nil {
		limit = *input.Limit
	}
	
	absolutePath, err := ResolvePath(path, t.cwd)
	if err != nil {
		return nil, err
	}
	// 检查文件可读
	if err := t.operations.Access(absolutePath); err != nil {
		return nil, fmt.Errorf("cannot access file: %w", err)
	}
	if ctx.Err() == context.Canceled {
		return nil, ctx.Err()
	}
	// 检测文件类型
	mimeType, _ := t.operations.DetectImageMimeType(absolutePath)
	if mimeType != "" {
		return t.readImage(absolutePath, mimeType)
	}

	return t.readText(absolutePath, offset, limit)
}

// readImage 读取图片文件
func (t *ReadTool) readImage(path, mimeType string) (*agent.AgentToolResult, error) {
	data, err := t.operations.ReadFile(path)
	if err != nil {
		return nil, err
	}
	base64 := string(data)
	var content []ai.ContentBlock
	// if t.autoResizeImages {
	// 	// TODO: Implement image resizing
	// 	content = []ContentBlock{
	// 		ai.NewTextContentBlock(fmt.Sprintf("Read image file [%s]", mimeType)),
	// 		ai.NewImageContentBlock(base64, mimeType),
	// 	}
	// } else {
	// 	content = []ContentBlock{
	// 		ai.NewTextContentBlock(fmt.Sprintf("Read image file [%s]", mimeType)),
	// 		ai.NewImageContentBlock(base64, mimeType),
	// 	}
	// }
	content = []ai.ContentBlock{
		ai.NewTextContentBlock(fmt.Sprintf("Read image file [%s]", mimeType)),
		ai.NewImageContentBlock(base64, mimeType),
	}
	return &agent.AgentToolResult{Content: content}, nil
}

// readText 读取文本文件
func (t *ReadTool) readText(path string, offset, limit int) (*agent.AgentToolResult, error) {
	data, err := t.operations.ReadFile(path)
	if err != nil {
		return nil, err
	}
	textContent := string(data)
	allLines := strings.Split(textContent, "\n")
	totalFileLines := len(allLines)
	startLine := 0
	if offset > 0 {
		startLine = offset - 1
	}
	if startLine >= totalFileLines {
		return nil, fmt.Errorf("offset %d is beyond end of file (%d lines total)", offset, totalFileLines)
	}
	var selectedContent string
	var userLimitedLines int
	if limit > 0 {
		endLine := startLine + limit
		if endLine > totalFileLines {
			endLine = totalFileLines
		}
		selectedContent = strings.Join(allLines[startLine:endLine], "\n")
		userLimitedLines = endLine - startLine
	} else {
		selectedContent = strings.Join(allLines[startLine:], "\n")
	}
	truncation := TruncateHead(selectedContent)
	var outputText string
	var details *ReadToolDetails
	if truncation.FirstLineExceeds {
		firstLineSize := FormatSize(len(allLines[startLine]))
		outputText = fmt.Sprintf("[Line %d is %s, exceeds %s limit. Use bash: sed -n '%dp' %s | head -c %d]",
			startLine+1, firstLineSize, FormatSize(DEFAULT_MAX_BYTES), path, DEFAULT_MAX_BYTES)
		details = &ReadToolDetails{Truncation: &truncation}
	} else if truncation.Truncated {
		endLineDisplay := startLine + truncation.OutputLines
		nextOffset := endLineDisplay + 1
		outputText = truncation.Content
		if truncation.TruncatedBy == "lines" {
			outputText += fmt.Sprintf("\n\n[Showing lines %d-%d of %d. Use offset=%d to continue.]",
				startLine+1, endLineDisplay, totalFileLines, nextOffset)
		} else {
			outputText += fmt.Sprintf("\n\n[Showing lines %d-%d of %d (%s limit). Use offset=%d to continue.]",
				startLine+1, endLineDisplay, totalFileLines, FormatSize(DEFAULT_MAX_BYTES), nextOffset)
		}
		details = &ReadToolDetails{Truncation: &truncation}
	} else if userLimitedLines > 0 && startLine+userLimitedLines < totalFileLines {
		remaining := totalFileLines - (startLine + userLimitedLines)
		nextOffset := startLine + userLimitedLines + 1
		outputText = truncation.Content
		outputText += fmt.Sprintf("\n\n[%d more lines in file. Use offset=%d to continue.]", remaining, nextOffset)
	} else {
		outputText = truncation.Content
	}
	return &agent.AgentToolResult{
		Content: []ai.ContentBlock{ai.NewTextContentBlock(outputText)},
		Details: details,
	}, nil
}