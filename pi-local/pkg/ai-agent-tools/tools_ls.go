package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jay-y/pi/pkg/ai"
	agent "github.com/jay-y/pi/pkg/ai-agent"
)

// LsToolInput Ls 工具输入
type LsToolInput struct {
	Path  string `json:"path,omitempty"`
	Limit int    `json:"limit,omitempty"`
}

// LsToolDetails Ls 工具详细信息
type LsToolDetails struct {
	Truncation        *TruncationResult `json:"truncation,omitempty"`
	EntryLimitReached int             `json:"entryLimitReached,omitempty"`
}

// LsOperations Ls 操作接口
type LsOperations interface {
	Exists(path string) bool
	Stat(path string) (os.FileInfo, error)
	ReadDir(path string) ([]string, error)
}

// DefaultLsOperations 默认 Ls 操作
type DefaultLsOperations struct{}

func (o *DefaultLsOperations) Exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func (o *DefaultLsOperations) Stat(path string) (os.FileInfo, error) {
	return os.Stat(path)
}

func (o *DefaultLsOperations) ReadDir(path string) ([]string, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}

	names := make([]string, len(entries))
	for i, entry := range entries {
		names[i] = entry.Name()
	}

	return names, nil
}

// LsTool Ls 工具
type LsTool struct {
	cwd        string
	operations LsOperations
}

func NewLsTool(cwd string, options ...LsToolOption) agent.AgentTool {
	tool := &LsTool{
		cwd:        cwd,
		operations: &DefaultLsOperations{},
	}
	for _, opt := range options {
		if opt != nil { opt(tool) }
	}
	return agent.NewAgentTool(tool)
}

type LsToolOption func(*LsTool)

func WithLsOperations(ops LsOperations) LsToolOption {
	return func(t *LsTool) {
		t.operations = ops
	}
}

func (t *LsTool) GetName() string { return "ls" }
func (t *LsTool) GetLabel() string { return "ls" }
func (t *LsTool) GetDescription() string {
	return fmt.Sprintf("List directory contents. Output truncated to %d entries or %dKB.",
		500, DEFAULT_MAX_BYTES/1024)
}
func (t *LsTool) GetParameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "Directory to list",
			},
			"limit": map[string]any{
				"type":        "number",
				"description": "Maximum number of entries",
			},
		},
	}
}

func (t *LsTool) Execute(ctx context.Context, params map[string]any, onUpdate func(*agent.AgentToolResult)) (*agent.AgentToolResult, error) {
	input, err := ValidateToolParams[LsToolInput](params)
	if err != nil {
		return nil, err
	}
	path := input.Path
	limit := input.Limit
	if limit == 0 {
		limit = 500
	}

	dirPath, err := ResolvePath(path, t.cwd)
	if err != nil {
		return nil, err
	}

	if !t.operations.Exists(dirPath) {
		return nil, fmt.Errorf("path not found: %s", dirPath)
	}

	stat, err := t.operations.Stat(dirPath)
	if err != nil {
		return nil, err
	}

	if !stat.IsDir() {
		return nil, fmt.Errorf("not a directory: %s", dirPath)
	}

	if ctx.Err() == context.Canceled {
		return nil, ctx.Err()
	}

	entries, err := t.operations.ReadDir(dirPath)
	if err != nil {
		return nil, fmt.Errorf("cannot read directory: %w", err)
	}

	sort.Slice(entries, func(i, j int) bool {
		return strings.ToLower(entries[i]) < strings.ToLower(entries[j])
	})

	results := []string{}
	entryLimitReached := false

	for _, entry := range entries {
		if len(results) >= limit {
			entryLimitReached = true
			break
		}

		fullPath := filepath.Join(dirPath, entry)
		suffix := ""

		entryStat, err := t.operations.Stat(fullPath)
		if err != nil {
			continue
		}

		if entryStat.IsDir() {
			suffix = "/"
		}

		results = append(results, entry+suffix)
	}

	if len(results) == 0 {
		return &agent.AgentToolResult{
			Content: []ai.ContentBlock{ai.NewTextContentBlock("(empty directory)")},
		}, nil
	}

	rawOutput := strings.Join(results, "\n")
	truncation := TruncateHead(rawOutput)

	output := truncation.Content
	details := &LsToolDetails{}
	notices := []string{}

	if entryLimitReached {
		notices = append(notices, fmt.Sprintf("%d entries limit reached. Use limit=%d for more", limit, limit*2))
		details.EntryLimitReached = limit
	}

	if truncation.Truncated {
		notices = append(notices, fmt.Sprintf("%s limit reached", FormatSize(DEFAULT_MAX_BYTES)))
		details.Truncation = &truncation
	}

	if len(notices) > 0 {
		output += "\n\n[" + strings.Join(notices, ". ") + "]"
	}

	return &agent.AgentToolResult{
		Content: []ai.ContentBlock{ai.NewTextContentBlock(output)},
		Details: details,
	}, nil
}