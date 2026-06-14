package tools

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/jay-y/pi/pkg/ai"
	agent "github.com/jay-y/pi/pkg/ai-agent"
)

// EditToolInput Edit 工具输入
type EditToolInput struct {
	Path    string `json:"path"`
	OldText string `json:"oldText"`
	NewText string `json:"newText"`
}

// EditToolDetails Edit 工具详细信息
type EditToolDetails struct {
	Diff             string `json:"diff"`
	FirstChangedLine int    `json:"firstChangedLine,omitempty"`
}

// EditOperations Edit 操作接口
type EditOperations interface {
	ReadFile(path string) ([]byte, error)
	WriteFile(path string, content string) error
	Mkdir(dir string) error
	Access(path string) error
}

// DefaultEditOperations 默认 Edit 操作
type DefaultEditOperations struct{}

func (o *DefaultEditOperations) ReadFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

func (o *DefaultEditOperations) WriteFile(path string, content string) error {
	return os.WriteFile(path, []byte(content), 0644)
}

func (o *DefaultEditOperations) Access(path string) error {
	_, err := os.Stat(path)
	if err != nil {
		return err
	}
	return nil
}

func (o *DefaultEditOperations) Mkdir(dir string) error {
	return os.MkdirAll(dir, 0755)
}

// EditTool Edit 工具
type EditTool struct {
	cwd        string
	operations EditOperations
}

func NewEditTool(cwd string, options ...EditToolOption) agent.AgentTool {
	tool := &EditTool{
		cwd:        cwd,
		operations: &DefaultEditOperations{},
	}
	for _, opt := range options {
		if opt != nil { opt(tool) }
	}
	return agent.NewAgentTool(tool)
}

type EditToolOption func(*EditTool)

func WithEditOperations(ops EditOperations) EditToolOption {
	return func(t *EditTool) {
		t.operations = ops
	}
}

func (t *EditTool) GetName() string        { return "edit" }
func (t *EditTool) GetLabel() string       { return "edit" }
func (t *EditTool) GetDescription() string { return "Edit a file by replacing exact text. The oldText must match exactly." }
func (t *EditTool) GetParameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "Path to file to edit",
			},
			"oldText": map[string]any{
				"type":        "string",
				"description": "Exact text to find and replace",
			},
			"newText": map[string]any{
				"type":        "string",
				"description": "New text to replace with",
			},
		},
		"required": []string{"path", "oldText", "newText"},
	}
}

func (t *EditTool) Execute(ctx context.Context, params map[string]any, onUpdate func(*agent.AgentToolResult)) (*agent.AgentToolResult, error) {
	input, err := ValidateToolParams[EditToolInput](params)
	if err != nil {
		return nil, err
	}
	path := input.Path
	if path == "" {
		return nil, fmt.Errorf("path is required")
	}
	oldText := input.OldText
	newText := input.NewText
	absolutePath, err := ResolvePath(path, t.cwd)
	if err != nil {
		return nil, err
	}
	if err := t.operations.Access(absolutePath); err != nil {
		return nil, fmt.Errorf("file not found: %s", path)
	}
	if ctx.Err() == context.Canceled {
		return nil, ctx.Err()
	}
	buffer, err := t.operations.ReadFile(absolutePath)
	if err != nil {
		return nil, err
	}
	rawContent := string(buffer)
	// 处理 BOM
	bom, content := stripBom(rawContent)
	// 检测换行符
	originalEnding := detectLineEnding(content)
	// 归一化换行符
	normalizedContent := normalizeToLF(content)
	normalizedOldText := normalizeToLF(oldText)
	normalizedNewText := normalizeToLF(newText)
	// 模糊匹配旧文本
	matchResult := fuzzyFindText(normalizedContent, normalizedOldText)
	if !matchResult.Found {
		return nil, fmt.Errorf("could not find the exact text in %s. The old text must match exactly including all whitespace and newlines", path)
	}
	fuzzyContent := normalizeForFuzzyMatch(normalizedContent)
	fuzzyOldText := normalizeForFuzzyMatch(normalizedOldText)
	// 检查旧文本是否唯一
	occurrences := strings.Count(fuzzyContent, fuzzyOldText)
	if occurrences > 1 {
		return nil, fmt.Errorf("found %d occurrences of the text in %s. The text must be unique. Please provide more context to make it unique", occurrences, path)
	}
	if ctx.Err() == context.Canceled {
		return nil, ctx.Err()
	}
	baseContent := matchResult.ContentForReplacement
	newContent := baseContent[:matchResult.Index] + normalizedNewText + baseContent[matchResult.Index+matchResult.MatchLength:]
	if baseContent == newContent {
		return nil, fmt.Errorf("no changes made to %s. The replacement produced identical content", path)
	}
	finalContent := bom + restoreLineEndings(newContent, originalEnding)
	if err := t.operations.WriteFile(absolutePath, finalContent); err != nil {
		return nil, err
	}
	diffResult := generateDiffString(baseContent, newContent)
	return &agent.AgentToolResult{
		Content: []ai.ContentBlock{
			ai.NewTextContentBlock(fmt.Sprintf("Successfully replaced text in %s.", path)),
		},
		Details: &EditToolDetails{
			Diff:             diffResult.Diff,
			FirstChangedLine: diffResult.FirstChangedLine,
		},
	}, nil
}