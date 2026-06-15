package session

import (
	"github.com/jay-y/pi/pkg/ai"
	agent "github.com/jay-y/pi/pkg/ai-agent"
)

// CommandContext 命令上下文
type CommandContext interface{}

// UIContext UI 上下文
type UIContext interface{}

// CommandContextActions 命令上下文操作
type CommandContextActions interface{}

// ExtensionRunner 扩展运行器接口
type ExtensionRunner interface {
	Emit(event any) (any, error)
	EmitInput(text string, images []ai.ImageContentBlock, source string) *InputResult
	EmitBeforeAgentStart(text string, images []ai.ImageContentBlock, systemPrompt string) *BeforeAgentStartResult
	HasHandlers(eventType string) bool
	GetCommand(name string) *CommandInfo
	CreateCommandContext() CommandContext
	EmitError(errorInfo *ErrorInfo)
	SetUIContext(ctx UIContext)
	BindCommandContext(actions CommandContextActions)
	OnError(listener ErrorListener) func()
	BindCore(core CoreBindings, session SessionBindings)
	GetRegisteredCommandsWithPaths() []CommandWithPath
}

// InputResult 输入结果
type InputResult struct {
	Action string         `json:"action"` // "handled", "transform", "continue"
	Text   string         `json:"text,omitempty"`
	Images []ai.ImageContentBlock `json:"images,omitempty"`
}

// BeforeAgentStartResult 代理启动前结果
type BeforeAgentStartResult struct {
	Messages     []CustomMessage `json:"messages,omitempty"`
	SystemPrompt string          `json:"systemPrompt,omitempty"`
}

// CommandInfo 命令信息
type CommandInfo struct {
	Name        string                                      `json:"name"`
	Description string                                      `json:"description"`
	Handler     func(args string, ctx CommandContext) error `json:"-"`
}

// ErrorInfo 错误信息
type ErrorInfo struct {
	ExtensionPath string `json:"extensionPath"`
	Event         string `json:"event"`
	Error         string `json:"error"`
}

// ErrorListener 错误监听器
type ErrorListener func(errorInfo *ErrorInfo)

// CoreBindings 核心绑定
type CoreBindings struct {
	SendMessage      func(message CustomMessage, options *SendMessageOptions)
	SendUserMessage  func(content any, options *SendMessageOptions)
	AppendEntry      func(customType string, data any)
	SetSessionName   func(name string)
	GetSessionName   func() string
	SetLabel         func(entryId, label string)
	GetActiveTools   func() []string
	GetAllTools      func() []ToolInfo
	SetActiveTools   func(toolNames []string)
	GetCommands      func() []SlashCommandInfo
	SetModel         func(model ai.Model) bool
	GetThinkingLevel func() ThinkingLevel
	SetThinkingLevel func(level ThinkingLevel)
}

// SessionBindings 会话绑定
type SessionBindings struct {
	GetModel           func() ai.Model
	IsIdle             func() bool
	Abort              func()
	HasPendingMessages func() bool
	Shutdown           func()
	GetContextUsage    func() *ContextUsage
	Compact            func(options *CompactOptions)
	GetSystemPrompt    func() string
}

// CompactOptions 压缩选项
type CompactOptions struct {
	CustomInstructions string `json:"customInstructions,omitempty"`
	OnComplete         func(result *CompactionResult)
	OnError            func(err error)
}

// CommandWithPath 带路径的命令
type CommandWithPath struct {
	Command       CommandInfo `json:"command"`
	ExtensionPath string      `json:"extensionPath"`
}

// ExtensionBindings 扩展绑定
type ExtensionBindings struct {
	UIContext             UIContext             `json:"-"`
	CommandContextActions CommandContextActions `json:"-"`
	ShutdownHandler       func()                `json:"-"`
	OnError               ErrorListener         `json:"-"`
}

// SlashCommandInfo 斜杠命令信息
type SlashCommandInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Source      string `json:"source"`
	Location    string `json:"location,omitempty"`
	Path        string `json:"path,omitempty"`
}

// ExtensionRunnerRef 扩展运行器引用
type ExtensionRunnerRef struct {
	Current ExtensionRunner `json:"-"`
}

// ExtensionsResult 扩展结果
type ExtensionsResult struct {
	Extensions []ExtensionInfo `json:"extensions"`
	Runtime    RuntimeInfo     `json:"runtime"`
}

// ExtensionInfo 扩展信息
type ExtensionInfo struct {
	Path string `json:"path"`
}

// RuntimeInfo 运行时信息
type RuntimeInfo struct {
	FlagValues map[string]any `json:"flagValues"`
}

// BindExtensions 绑定扩展
func (s *AgentSession) BindExtensions(bindings ExtensionBindings) error {
	if bindings.UIContext != nil {
		s.extensionUIContext = bindings.UIContext
	}
	if bindings.CommandContextActions != nil {
		s.extensionCommandContextActions = bindings.CommandContextActions
	}
	if bindings.ShutdownHandler != nil {
		s.extensionShutdownHandler = bindings.ShutdownHandler
	}
	if bindings.OnError != nil {
		s.extensionErrorListener = bindings.OnError
	}

	if s.extensionRunner != nil {
		s.applyExtensionBindings(s.extensionRunner)
	}

	return nil
}

// applyExtensionBindings 应用扩展绑定
func (s *AgentSession) applyExtensionBindings(runner ExtensionRunner) {
	runner.SetUIContext(s.extensionUIContext)
	runner.BindCommandContext(s.extensionCommandContextActions)

	if s.extensionErrorListener != nil {
		runner.OnError(s.extensionErrorListener)
	}
}

// HasExtensionHandlers 检查是否有扩展处理器
func (s *AgentSession) HasExtensionHandlers(eventType string) bool {
	if s.extensionRunner == nil {
		return false
	}
	return s.extensionRunner.HasHandlers(eventType)
}

// ExtensionRunner 获取扩展运行器
func (s *AgentSession) ExtensionRunner() ExtensionRunner {
	return s.extensionRunner
}

// emitExtensionEvent 发送扩展事件
func (s *AgentSession) emitExtensionEvent(event agent.AgentEvent) {
	if s.extensionRunner == nil {
		return
	}

	// TODO: 实现扩展事件发送
}

// buildExtensionRuntime 构建扩展运行时环境
func (s *AgentSession) buildExtensionRuntime(options *BuildRuntimeOptions) {
	// TODO: 实现扩展加载和工具注册
}
