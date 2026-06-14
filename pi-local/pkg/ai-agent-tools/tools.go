package tools

import (
	agent "github.com/jay-y/pi/pkg/ai-agent"
)

// CreateCodingTools 创建编码工具集合
func CreateCodingTools(cwd string) []agent.AgentTool {
	return []agent.AgentTool{
		NewReadTool(cwd),
		NewBashTool(cwd),
		NewEditTool(cwd),
		NewWriteTool(cwd),
	}
}

// CreateReadOnlyTools 创建只读工具集合
func CreateReadOnlyTools(cwd string) []agent.AgentTool {
	return []agent.AgentTool{
		NewReadTool(cwd),
		NewGrepTool(cwd),
		NewFindTool(cwd),
		NewLsTool(cwd),
	}
}

// CreateAllTools 创建所有工具
func CreateAllTools(cwd string) map[string]agent.AgentTool {
	return map[string]agent.AgentTool{
		"read":  NewReadTool(cwd),
		"bash":  NewBashTool(cwd),
		"edit":  NewEditTool(cwd),
		"write": NewWriteTool(cwd),
		"grep":  NewGrepTool(cwd),
		"find":  NewFindTool(cwd),
		"ls":    NewLsTool(cwd),
	}
}