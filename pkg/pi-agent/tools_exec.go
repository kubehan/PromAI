package piagent

import (
	"context"
	"encoding/json"
	"strconv"

	"github.com/jay-y/pi/pkg/ai"
	agent "github.com/jay-y/pi/pkg/ai-agent"
)

// ExecuteCommandTool 执行 Shell 命令工具
type ExecuteCommandTool struct{}

func NewExecuteCommandTool() *ExecuteCommandTool { return &ExecuteCommandTool{} }

func (t *ExecuteCommandTool) GetName() string        { return "execute_command" }
func (t *ExecuteCommandTool) GetLabel() string       { return "执行命令" }
func (t *ExecuteCommandTool) GetDescription() string { return "执行 Shell 命令并返回输出" }

func (t *ExecuteCommandTool) GetParameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"command": map[string]any{"type": "string", "description": "要执行的 Shell 命令"},
			"host_id": map[string]any{"type": "integer", "description": "目标主机 ID（如需）"},
		},
		"required": []string{"command"},
	}
}

func (t *ExecuteCommandTool) Execute(ctx context.Context, params map[string]any, onUpdate func(*agent.AgentToolResult)) (*agent.AgentToolResult, error) {
	cmd, _ := params["command"].(string)
	if cmd == "" {
		return t.textResult("参数缺失：需要 command。"), nil
	}
	return t.textResult("execute_command 已移除，不再支持在该上下文中执行受控主机命令。"), nil
}

func (t *ExecuteCommandTool) textResult(s string) *agent.AgentToolResult {
	return &agent.AgentToolResult{Content: []ai.ContentBlock{ai.NewTextContentBlock(s)}}
}

func toUint(v interface{}) uint {
	switch n := v.(type) {
	case float64:
		return uint(n)
	case int:
		return uint(n)
	case int64:
		return uint(n)
	case json.Number:
		i, _ := n.Int64()
		return uint(i)
	case string:
		i, _ := strconv.ParseUint(n, 10, 64)
		return uint(i)
	}
	return 0
}
