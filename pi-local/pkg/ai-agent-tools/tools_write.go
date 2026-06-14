package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/jay-y/pi/pkg/ai"
	agent "github.com/jay-y/pi/pkg/ai-agent"
)

// WriteToolInput Write 工具输入
type WriteToolInput struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

// WriteOperations Write 操作接口
type WriteOperations interface {
	WriteFile(path string, content string) error
	Mkdir(dir string) error
}

// DefaultWriteOperations 默认 Write 操作
type DefaultWriteOperations struct{}

func (o *DefaultWriteOperations) WriteFile(path string, content string) error {
	return os.WriteFile(path, []byte(content), 0644)
}

func (o *DefaultWriteOperations) Mkdir(dir string) error {
	return os.MkdirAll(dir, 0755)
}

// WriteTool Write 工具
type WriteTool struct {
	cwd        string
	operations WriteOperations
}

func NewWriteTool(cwd string, options ...WriteToolOption) agent.AgentTool {
	tool := &WriteTool{
		cwd:        cwd,
		operations: &DefaultWriteOperations{},
	}
	for _, opt := range options {
		if opt != nil { opt(tool) }
	}
	return agent.NewAgentTool(tool)
}

type WriteToolOption func(*WriteTool)

func WithWriteOperations(ops WriteOperations) WriteToolOption {
	return func(t *WriteTool) {
		t.operations = ops
	}
}

func (t *WriteTool) GetName() string { return "write" }
func (t *WriteTool) GetLabel() string { return "write" }
func (t *WriteTool) GetDescription() string {
	return "Write content to a file. Creates file if it doesn't exist, overwrites if it does."
}
func (t *WriteTool) GetParameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "Path to file to write",
			},
			"content": map[string]any{
				"type":        "string",
				"description": "Content to write to file",
			},
		},
		"required": []string{"path", "content"},
	}
}

func (t *WriteTool) Execute(ctx context.Context, params map[string]any, onUpdate func(*agent.AgentToolResult)) (*agent.AgentToolResult, error) {
	input, err := ValidateToolParams[WriteToolInput](params)
	if err != nil {
		return nil, err
	}
	path := input.Path
	content := input.Content

	absolutePath, err := ResolvePath(path, t.cwd)
	if err != nil {
		return nil, err
	}
	dir := filepath.Dir(absolutePath)

	if err := t.operations.Mkdir(dir); err != nil {
		return nil, fmt.Errorf("cannot create directory: %w", err)
	}

	if ctx.Err() == context.Canceled {
		return nil, ctx.Err()
	}

	if err := t.operations.WriteFile(absolutePath, content); err != nil {
		return nil, fmt.Errorf("cannot write file: %w", err)
	}

	return &agent.AgentToolResult{
		Content: []ai.ContentBlock{
			ai.NewTextContentBlock(fmt.Sprintf("Successfully wrote %d bytes to %s", len(content), path)),
		},
	}, nil
}
