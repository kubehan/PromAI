package session

import (
	"context"
	"encoding/json"
	"sync"

	agent "github.com/jay-y/pi/pkg/ai-agent"
)

// 错误定义
var (
	ErrToolNotFound  = &ToolError{Message: "Tool not found"}
	ErrToolNoExecute = &ToolError{Message: "Tool has no execute function"}
)

// ToolError 工具错误
type ToolError struct {
	Message string
}

func (e *ToolError) Error() string {
	return e.Message
}

// toolRegistryMu 工具注册表互斥锁
var toolRegistryMu sync.RWMutex

// ToolInfo 工具信息
type ToolInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Parameters  string `json:"parameters"`
}

// GetActiveToolNames 获取当前活动工具名称
func (s *AgentSession) GetActiveToolNames() []string {
	tools := s.agent.GetState().Tools
	result := make([]string, len(tools))
	for i, t := range tools {
		result[i] = t.Name
	}
	return result
}

// GetAllTools 获取所有配置的工具
func (s *AgentSession) GetAllTools() []ToolInfo {
	s.toolRegistryMu.RLock()
	defer s.toolRegistryMu.RUnlock()

	result := make([]ToolInfo, 0, len(s.toolRegistry))
	for _, t := range s.toolRegistry {
		result = append(result, ToolInfo{
			Name:        t.Name,
			Description: t.Description,
			Parameters:  parametersToString(t.Parameters),
		})
	}
	return result
}

// SetActiveToolsByName 按名称设置活动工具
func (s *AgentSession) SetActiveToolsByName(toolNames []string) {
	var tools []agent.AgentTool
	var validToolNames []string

	for _, name := range toolNames {
		if tool, exists := s.toolRegistry[name]; exists {
			tools = append(tools, tool)
			validToolNames = append(validToolNames, name)
		}
	}

	s.agent.SetTools(tools)

	// 重建系统提示
	s.baseSystemPrompt = s.rebuildSystemPrompt(validToolNames)
	s.agent.SetSystemPrompt(s.baseSystemPrompt)
}

// RegisterTool 注册工具
func (s *AgentSession) RegisterTool(tool agent.AgentTool) {
	s.toolRegistryMu.Lock()
	defer s.toolRegistryMu.Unlock()

	s.toolRegistry[tool.Name] = tool
	s.baseToolRegistry[tool.Name] = tool
}

// UnregisterTool 注销工具
func (s *AgentSession) UnregisterTool(name string) {
	s.toolRegistryMu.Lock()
	defer s.toolRegistryMu.Unlock()

	delete(s.toolRegistry, name)
	delete(s.baseToolRegistry, name)
}

// GetTool 获取工具
func (s *AgentSession) GetTool(name string) *agent.AgentTool {
	s.toolRegistryMu.RLock()
	defer s.toolRegistryMu.RUnlock()

	if tool, exists := s.toolRegistry[name]; exists {
		return &tool
	}
	return nil
}

// HasTool 检查是否有工具
func (s *AgentSession) HasTool(name string) bool {
	s.toolRegistryMu.RLock()
	defer s.toolRegistryMu.RUnlock()

	_, exists := s.toolRegistry[name]
	return exists
}

// EnableTool 启用工具
func (s *AgentSession) EnableTool(name string) bool {
	s.toolRegistryMu.Lock()
	defer s.toolRegistryMu.Unlock()

	tool, exists := s.baseToolRegistry[name]
	if !exists {
		return false
	}

	s.toolRegistry[name] = tool
	s.updateActiveTools()
	return true
}

// DisableTool 禁用工具
func (s *AgentSession) DisableTool(name string) bool {
	s.toolRegistryMu.Lock()
	defer s.toolRegistryMu.Unlock()

	if _, exists := s.toolRegistry[name]; !exists {
		return false
	}

	delete(s.toolRegistry, name)
	s.updateActiveTools()
	return true
}

// updateActiveTools 更新活动工具
func (s *AgentSession) updateActiveTools() {
	var tools []agent.AgentTool
	for _, tool := range s.toolRegistry {
		tools = append(tools, tool)
	}
	s.agent.SetTools(tools)
}

// rebuildSystemPrompt 重建系统提示
func (s *AgentSession) rebuildSystemPrompt(toolNames []string) string {
	// 过滤有效的工具名称
	var validToolNames []string
	for _, name := range toolNames {
		if _, exists := s.baseToolRegistry[name]; exists {
			validToolNames = append(validToolNames, name)
		}
	}

	// 获取资源加载器的系统提示
	var loaderSystemPrompt string
	var appendSystemPrompt string
	var loadedSkills []SkillInfo
	var loadedContextFiles []string

	if s.resourceLoader != nil {
		loaderSystemPrompt = s.resourceLoader.GetSystemPrompt()
		appendPrompts := s.resourceLoader.GetAppendSystemPrompt()
		if len(appendPrompts) > 0 {
			for i, p := range appendPrompts {
				if i > 0 {
					appendSystemPrompt += "\n\n"
				}
				appendSystemPrompt += p
			}
		}

		// 获取技能和上下文文件
		loadedSkills = s.resourceLoader.GetSkills().Skills
		agentsFilesResult := s.resourceLoader.GetAgentsFiles()
		for _, af := range agentsFilesResult.AgentsFiles {
			loadedContextFiles = append(loadedContextFiles, af.Content)
		}
	}

	// 构建系统提示
	return buildSystemPrompt(&BuildSystemPromptOptions{
		Cwd:                s.cwd,
		Skills:             loadedSkills,
		ContextFiles:       loadedContextFiles,
		SelectedTools:      validToolNames,
		CustomPrompt:       loaderSystemPrompt,
		AppendSystemPrompt: appendSystemPrompt,
	})
}

// BuildSystemPromptOptions 构建系统提示选项
type BuildSystemPromptOptions struct {
	Cwd                string      `json:"cwd"`
	Skills             []SkillInfo `json:"skills,omitempty"`
	ContextFiles       []string    `json:"contextFiles,omitempty"`
	CustomPrompt       string      `json:"customPrompt,omitempty"`
	AppendSystemPrompt string      `json:"appendSystemPrompt,omitempty"`
	SelectedTools      []string    `json:"selectedTools,omitempty"`
}

// buildSystemPrompt 构建系统提示
func buildSystemPrompt(opts *BuildSystemPromptOptions) string {
	var prompt string

	// 添加工作目录信息
	if opts.Cwd != "" {
		prompt += "Working directory: " + opts.Cwd + "\n\n"
	}

	// 添加上下文文件
	if len(opts.ContextFiles) > 0 {
		for _, content := range opts.ContextFiles {
			if content != "" {
				prompt += content + "\n\n"
			}
		}
	}

	// 添加自定义提示
	if opts.CustomPrompt != "" {
		prompt += opts.CustomPrompt + "\n\n"
	}

	// 添加技能信息
	if len(opts.Skills) > 0 {
		prompt += "Available skills:\n"
		for _, skill := range opts.Skills {
			prompt += "- " + skill.Name
			if skill.Description != "" {
				prompt += ": " + skill.Description
			}
			prompt += "\n"
		}
		prompt += "\n"
	}

	// 添加工具信息
	if len(opts.SelectedTools) > 0 {
		prompt += "Available tools:\n"
		for _, name := range opts.SelectedTools {
			prompt += "- " + name + "\n"
		}
		prompt += "\n"
	}

	// 添加追加提示
	if opts.AppendSystemPrompt != "" {
		prompt += opts.AppendSystemPrompt + "\n\n"
	}

	return prompt
}

// parametersToString 将参数转换为字符串
func parametersToString(params map[string]any) string {
	if params == nil {
		return ""
	}
	paramsBytes, _ := json.Marshal(params)
	return string(paramsBytes)
}

// ToolRegistry 工具注册表
type ToolRegistry struct {
	tools map[string]agent.AgentTool
	mu    sync.RWMutex
}

// NewToolRegistry 创建工具注册表
func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{
		tools: make(map[string]agent.AgentTool),
	}
}

// Register 注册工具
func (r *ToolRegistry) Register(tool agent.AgentTool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tools[tool.Name] = tool
}

// Unregister 注销工具
func (r *ToolRegistry) Unregister(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.tools, name)
}

// Get 获取工具
func (r *ToolRegistry) Get(name string) *agent.AgentTool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if tool, exists := r.tools[name]; exists {
		return &tool
	}
	return nil
}

// GetAll 获取所有工具
func (r *ToolRegistry) GetAll() []agent.AgentTool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]agent.AgentTool, 0, len(r.tools))
	for _, tool := range r.tools {
		result = append(result, tool)
	}
	return result
}

// Names 获取所有工具名称
func (r *ToolRegistry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]string, 0, len(r.tools))
	for name := range r.tools {
		result = append(result, name)
	}
	return result
}

// Has 检查是否有工具
func (r *ToolRegistry) Has(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, exists := r.tools[name]
	return exists
}

// Count 获取工具数量
func (r *ToolRegistry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.tools)
}

// Clear 清空所有工具
func (r *ToolRegistry) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tools = make(map[string]agent.AgentTool)
}

// ExecuteTool 执行工具
func (s *AgentSession) ExecuteTool(ctx context.Context, toolName string, args map[string]any) (any, error) {
	tool := s.GetTool(toolName)
	if tool == nil {
		return nil, ErrToolNotFound
	}

	if tool.Execute == nil {
		return nil, ErrToolNoExecute
	}

	return tool.Execute(ctx, args, nil)
}