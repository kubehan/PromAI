package session

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/jay-y/pi/pkg/ai"
	agent "github.com/jay-y/pi/pkg/ai-agent"
)

// ToolDefinition 工具定义
type ToolDefinition struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Parameters  string `json:"parameters"`
	Execute     func(ctx context.Context, args any) (any, error)
}

// BuildRuntimeOptions 构建运行时选项
type BuildRuntimeOptions struct {
	ActiveToolNames          []string       `json:"activeToolNames,omitempty"`
	FlagValues               map[string]any `json:"flagValues,omitempty"`
	IncludeAllExtensionTools bool           `json:"includeAllExtensionTools,omitempty"`
}

// AgentSessionConfig 会话配置
type AgentSessionConfig struct {
	Agent                  *agent.Agent         `json:"-"`
	SessionManager         *SessionManager       `json:"-"`
	SettingsManager        *SettingsManager     `json:"-"`
	Cwd                    string               `json:"cwd"`
	ScopedModels           []ScopedModel        `json:"scopedModels,omitempty"`
	ResourceLoader         ResourceLoader       `json:"-"`
	CustomTools            []ToolDefinition      `json:"customTools,omitempty"`
	ModelRegistry          *ModelRegistry       `json:"-"`
	InitialActiveToolNames []string             `json:"initialActiveToolNames,omitempty"`
	BaseToolsOverride      map[string]agent.AgentTool `json:"baseToolsOverride,omitempty"`
	ExtensionRunnerRef     *ExtensionRunnerRef  `json:"-"`
}

// AgentSession 代理会话，封装了代理生命周期和会话管理
type AgentSession struct {
	agent           *agent.Agent
	sessionManager  *SessionManager
	settingsManager *SettingsManager
	modelRegistry   *ModelRegistry
	resourceLoader  ResourceLoader

	scopedModels []ScopedModel

	// 事件订阅状态
	unsubscribeAgent func()
	eventListeners   []AgentSessionEventListener
	eventListenersMu sync.RWMutex

	// 消息队列
	steeringMessages        []string
	followUpMessages        []string
	pendingNextTurnMessages []CustomMessage
	messagesMu              sync.RWMutex

	// 压缩状态
	compactionAbortController     context.CancelFunc
	autoCompactionAbortController context.CancelFunc

	// 分支摘要状态
	branchSummaryAbortController context.CancelFunc

	// 重试状态
	retryAbortController context.CancelFunc
	retryAttempt         int
	retryWg              sync.WaitGroup
	retryMu              sync.Mutex

	// Bash 执行状态
	bashAbortController context.CancelFunc
	bashRunning         bool
	bashMu              sync.Mutex
	pendingBashMessages []BashExecutionMessage

	// 扩展系统
	extensionRunner    ExtensionRunner
	turnIndex          int
	extensionRunnerRef *ExtensionRunnerRef

	// 工具注册
	baseToolRegistry map[string]agent.AgentTool
	toolRegistry     map[string]agent.AgentTool
	toolRegistryMu   sync.RWMutex
	customTools      []ToolDefinition

	// 其他状态
	cwd                    string
	initialActiveToolNames []string
	baseToolsOverride      map[string]agent.AgentTool
	baseSystemPrompt       string

	// 扩展绑定
	extensionUIContext             UIContext
	extensionCommandContextActions CommandContextActions
	extensionShutdownHandler       func()
	extensionErrorListener         ErrorListener
	extensionErrorUnsubscriber     func()

	// 跟踪最后的助手消息用于自动压缩检查
	lastAssistantMessage *ai.AssistantMessage
	lastAssistantMu      sync.RWMutex

	// 事件队列（串行处理事件）
	eventQueue chan func()
	eventQueueWg sync.WaitGroup

	// 重试 Promise 机制（同步创建）
	retryPromise     chan struct{}
	retryPromiseMu   sync.Mutex
}

// NewAgentSession 创建新的代理会话
func NewAgentSession(config *AgentSessionConfig) *AgentSession {
	s := &AgentSession{
		agent:                   config.Agent,
		sessionManager:          config.SessionManager,
		settingsManager:         config.SettingsManager,
		modelRegistry:           config.ModelRegistry,
		resourceLoader:          config.ResourceLoader,
		scopedModels:            config.ScopedModels,
		customTools:             config.CustomTools,
		cwd:                     config.Cwd,
		extensionRunnerRef:      config.ExtensionRunnerRef,
		initialActiveToolNames:  config.InitialActiveToolNames,
		baseToolsOverride:       config.BaseToolsOverride,
		steeringMessages:        make([]string, 0),
		followUpMessages:        make([]string, 0),
		pendingNextTurnMessages: make([]CustomMessage, 0),
		eventListeners:          make([]AgentSessionEventListener, 0),
		baseToolRegistry:        make(map[string]agent.AgentTool),
		toolRegistry:            make(map[string]agent.AgentTool),
		eventQueue:              make(chan func(), 100), // 事件队列，串行处理
	}

	// 启动事件处理 goroutine
	s.eventQueueWg.Add(1)
	go s.processEventQueue()

	// 订阅代理事件
	s.unsubscribeAgent = s.agent.Subscribe(s.handleAgentEvent)

	// 构建运行时
	s.buildRuntime(&BuildRuntimeOptions{
		ActiveToolNames:          s.initialActiveToolNames,
		IncludeAllExtensionTools: true,
	})

	return s
}

// processEventQueue 处理事件队列
func (s *AgentSession) processEventQueue() {
	defer s.eventQueueWg.Done()
	for fn := range s.eventQueue {
		fn()
	}
}

// SetAgent 设置会话的代理
func (s *AgentSession) SetAgent(agent *agent.Agent) {
	s.agent = agent
}

// buildRuntime 构建运行时
func (s *AgentSession) buildRuntime(options *BuildRuntimeOptions) {
	// 初始化基础工具注册表
	if s.baseToolsOverride != nil {
		for name, tool := range s.baseToolsOverride {
			s.baseToolRegistry[name] = tool
		}
	}

	// 实现扩展加载和工具注册
	s.buildExtensionRuntime(options)
}

// handleAgentEvent 处理代理事件（借鉴 TypeScript 设计，使用事件队列）
func (s *AgentSession) handleAgentEvent(event agent.AgentEvent) {
	// 同步创建重试 Promise
	// 确保 waitForRetry() 不会错过正在进行的重试
	s.createRetryPromiseForAgentEnd(event)

	// 将事件处理放入队列，串行执行
	select {
	case s.eventQueue <- func() {
		s.processAgentEvent(event)
	}:
	default:
		// 队列已满，直接处理（降级处理）
		s.processAgentEvent(event)
	}
}

// createRetryPromiseForAgentEnd 为 agent_end 事件创建重试 Promise
func (s *AgentSession) createRetryPromiseForAgentEnd(event agent.AgentEvent) {
	e, ok := event.(*agent.AgentEventEnd)
	if !ok {
		return
	}

	s.retryPromiseMu.Lock()
	defer s.retryPromiseMu.Unlock()

	// 如果已经有重试 Promise，不创建新的
	if s.retryPromise != nil {
		return
	}

	// 检查是否是可重试错误
	lastAssistant := s.findLastAssistantInMessages(e.Messages)
	if lastAssistant == nil || !s.isRetryableError(lastAssistant) {
		return
	}

	// 同步创建重试 Promise
	s.retryPromise = make(chan struct{})
}

// findLastAssistantInMessages 在消息列表中查找最后一条助手消息
func (s *AgentSession) findLastAssistantInMessages(messages []ai.Message) *ai.AssistantMessage {
	for i := len(messages) - 1; i >= 0; i-- {
		if msg, ok := messages[i].(*ai.AssistantMessage); ok {
			return msg
		}
	}
	return nil
}

// processAgentEvent 处理代理事件（在事件队列中串行执行）
func (s *AgentSession) processAgentEvent(event agent.AgentEvent) {
	// 当用户消息开始时，检查是否来自队列并移除
	if e, ok := event.(*agent.AgentEventMessageStart); ok {
		if msg, ok := e.Message.(*ai.UserMessage); ok {
			messageText := s.getUserMessageText(msg)
			if messageText != "" {
				s.messagesMu.Lock()
				// 检查 steering 队列
				steeringIndex := indexOf(s.steeringMessages, messageText)
				if steeringIndex != -1 {
					s.steeringMessages = removeAtIndex(s.steeringMessages, steeringIndex)
				} else {
					// 检查 follow-up 队列
					followUpIndex := indexOf(s.followUpMessages, messageText)
					if followUpIndex != -1 {
						s.followUpMessages = removeAtIndex(s.followUpMessages, followUpIndex)
					}
				}
				s.messagesMu.Unlock()
			}
		}
	}

	// 发送到扩展
	s.emitExtensionEvent(event)

	// 通知所有监听器
	s.emit(event)

	// 处理会话持久化
	if e, ok := event.(*agent.AgentEventMessageEnd); ok {
		s.handleMessageEnd(e)
	}

	// 在代理完成后检查自动重试和自动压缩
	if e, ok := event.(*agent.AgentEventEnd); ok {
		s.handleAgentEnd(e)
	}
}

// getUserMessageText 提取用户消息文本
func (s *AgentSession) getUserMessageText(message ai.Message) string {
	if message.GetRole() != "user" {
		return ""
	}

	um, ok := message.(*ai.UserMessage)
	if !ok {
		return ""
	}

	switch content := um.Content.(type) {
	case string:
		return content
	case []ai.ContentBlock:
		var text string
		for _, block := range content {
			if tc, ok := block.(*ai.TextContentBlock); ok {
				text += tc.Text
			}
		}
		return text
	}
	return ""
}

// handleMessageEnd 处理消息结束事件
func (s *AgentSession) handleMessageEnd(event *agent.AgentEventMessageEnd) {
	// 常规 LLM 消息 - 持久化到会话
	s.sessionManager.AppendMessage(event.Message)

	// 跟踪助手消息用于自动压缩
	if am, ok := event.Message.(*ai.AssistantMessage); ok {
		s.lastAssistantMu.Lock()
		s.lastAssistantMessage = am
		s.lastAssistantMu.Unlock()

		// 成功响应时重置重试计数器并解析 Promise
		if am.StopReason != StopReasonError && s.retryAttempt > 0 {
			s.emit(&AutoRetryEndEvent{
				Type:    "auto_retry_end",
				Success: true,
				Attempt: s.retryAttempt,
			})
			s.retryAttempt = 0
			s.resolveRetry() // 解析 Promise，让 WaitForRetry 返回
		}
	}
}

// handleAgentEnd 处理代理结束事件（借鉴 TypeScript 设计，简化重试链）
func (s *AgentSession) handleAgentEnd(event *agent.AgentEventEnd) {
	s.lastAssistantMu.RLock()
	msg := s.lastAssistantMessage
	s.lastAssistantMu.RUnlock()

	if msg == nil {
		return
	}

	// 检查可重试错误
	if s.isRetryableError(msg) {
		// 触发重试，如果成功则返回，让重试完成后再次触发 handleAgentEnd
		if s.handleRetryableError(msg) {
			return
		}
		// 重试未触发（达到最大次数），解析 Promise 并继续处理
		s.resolveRetry()
	} else {
		// 非可重试错误，解析 Promise（如果有）
		s.retryPromiseMu.Lock()
		hasPromise := s.retryPromise != nil
		s.retryPromiseMu.Unlock()
		if hasPromise {
			s.resolveRetry()
		}
	}

	s.lastAssistantMu.Lock()
	s.lastAssistantMessage = nil
	s.lastAssistantMu.Unlock()

	// 检查压缩
	s.checkCompaction(msg, true)
}

// emit 发送事件到所有监听器
func (s *AgentSession) emit(event AgentSessionEvent) {
	s.eventListenersMu.RLock()
	listeners := make([]AgentSessionEventListener, len(s.eventListeners))
	copy(listeners, s.eventListeners)
	s.eventListenersMu.RUnlock()

	for _, listener := range listeners {
		listener(event)
	}
}

// Subscribe 订阅代理事件
func (s *AgentSession) Subscribe(listener AgentSessionEventListener) func() {
	s.eventListenersMu.Lock()
	s.eventListeners = append(s.eventListeners, listener)
	s.eventListenersMu.Unlock()

	return func() {
		s.eventListenersMu.Lock()
		for i, l := range s.eventListeners {
			if &l == &listener {
				s.eventListeners = append(s.eventListeners[:i], s.eventListeners[i+1:]...)
				break
			}
		}
		s.eventListenersMu.Unlock()
	}
}

// Dispose 清理所有监听器并断开代理连接
func (s *AgentSession) Dispose() {
	if s.unsubscribeAgent != nil {
		s.unsubscribeAgent()
		s.unsubscribeAgent = nil
	}
	s.eventListenersMu.Lock()
	s.eventListeners = nil
	s.eventListenersMu.Unlock()
}

// disconnectFromAgent 临时断开代理事件
func (s *AgentSession) DisconnectFromAgent() {
	if s.unsubscribeAgent != nil {
		s.unsubscribeAgent()
		s.unsubscribeAgent = nil
	}
}

// reconnectToAgent 重新连接代理事件
func (s *AgentSession) ReconnectToAgent() {
	if s.unsubscribeAgent != nil {
		return
	}
	s.unsubscribeAgent = s.agent.Subscribe(s.handleAgentEvent)
}

// State 获取代理状态
func (s *AgentSession) State() *agent.AgentState {
	return s.agent.GetState()
}

// Model 获取当前模型
func (s *AgentSession) Model() ai.Model {
	return s.agent.GetState().Model
}

// ThinkingLevel 获取当前思考级别
func (s *AgentSession) ThinkingLevel() ThinkingLevel {
	return s.agent.GetState().ThinkingLevel
}

// IsStreaming 检查是否正在流式传输
func (s *AgentSession) IsStreaming() bool {
	return s.agent.GetState().IsStreaming
}

// SystemPrompt 获取系统提示
func (s *AgentSession) SystemPrompt() string {
	return s.agent.GetState().SystemPrompt
}

// GetPromptTemplates 获取文件提示模板
func (s *AgentSession) GetPromptTemplates() []PromptTemplate {
	if s.resourceLoader == nil {
		return nil
	}
	return s.resourceLoader.GetPrompts().Prompts
}

// GetSkills 获取可用技能
func (s *AgentSession) GetSkills() []SkillInfo {
	if s.resourceLoader == nil {
		return nil
	}
	return s.resourceLoader.GetSkills().Skills
}

// RetryAttempt 获取当前重试尝试次数
func (s *AgentSession) RetryAttempt() int {
	return s.retryAttempt
}

// Messages 获取所有消息
func (s *AgentSession) Messages() []ai.Message {
	return s.agent.GetState().Messages
}

// SteeringMode 获取 steering 模式
func (s *AgentSession) SteeringMode() string {
	return s.agent.GetSteeringMode()
}

// FollowUpMode 获取 follow-up 模式
func (s *AgentSession) FollowUpMode() string {
	return s.agent.GetFollowUpMode()
}

// SessionFile 获取会话文件路径
func (s *AgentSession) SessionFile() string {
	return s.sessionManager.GetSessionFile()
}

// SessionId 获取会话 ID
func (s *AgentSession) SessionId() string {
	return s.sessionManager.GetSessionID()
}

// SessionName 获取会话名称
func (s *AgentSession) SessionName() string {
	return s.sessionManager.GetSessionName()
}

// SetSessionName 设置会话名称
func (s *AgentSession) SetSessionName(name string) {
	s.sessionManager.AppendSessionInfo(name)
}

// ScopedModels 获取范围模型
func (s *AgentSession) ScopedModels() []ScopedModel {
	return s.scopedModels
}

// SetScopedModels 设置范围模型
func (s *AgentSession) SetScopedModels(models []ScopedModel) {
	s.scopedModels = models
}

// ExtendResources 扩展资源
func (s *AgentSession) ExtendResources(paths ResourceExtensionPaths) error {
	if s.resourceLoader == nil {
		return fmt.Errorf("resource loader not initialized")
	}
	s.resourceLoader.ExtendResources(paths)
	// 重建系统提示
	s.rebuildAndSetSystemPrompt()
	return nil
}

// ReloadResources 重新加载资源
func (s *AgentSession) ReloadResources() error {
	if s.resourceLoader == nil {
		return fmt.Errorf("resource loader not initialized")
	}
	if err := s.resourceLoader.Reload(); err != nil {
		return err
	}
	// 重建系统提示
	s.rebuildAndSetSystemPrompt()
	return nil
}

// rebuildAndSetSystemPrompt 重建并设置系统提示
func (s *AgentSession) rebuildAndSetSystemPrompt() {
	activeTools := s.GetActiveToolNames()
	newPrompt := s.rebuildSystemPrompt(activeTools)
	s.agent.SetSystemPrompt(newPrompt)
}

// IsCompacting 检查是否正在压缩
func (s *AgentSession) IsCompacting() bool {
	return s.autoCompactionAbortController != nil || s.compactionAbortController != nil
}

// PendingMessageCount 获取待处理消息数量
func (s *AgentSession) PendingMessageCount() int {
	s.messagesMu.RLock()
	defer s.messagesMu.RUnlock()
	return len(s.steeringMessages) + len(s.followUpMessages)
}

// GetSteeringMessages 获取 steering 消息
func (s *AgentSession) GetSteeringMessages() []string {
	s.messagesMu.RLock()
	defer s.messagesMu.RUnlock()
	result := make([]string, len(s.steeringMessages))
	copy(result, s.steeringMessages)
	return result
}

// GetFollowUpMessages 获取 follow-up 消息
func (s *AgentSession) GetFollowUpMessages() []string {
	s.messagesMu.RLock()
	defer s.messagesMu.RUnlock()
	result := make([]string, len(s.followUpMessages))
	copy(result, s.followUpMessages)
	return result
}

// ClearQueue 清除所有队列消息
func (s *AgentSession) ClearQueue() map[string][]string {
	s.messagesMu.Lock()
	defer s.messagesMu.Unlock()

	steering := make([]string, len(s.steeringMessages))
	copy(steering, s.steeringMessages)
	followUp := make([]string, len(s.followUpMessages))
	copy(followUp, s.followUpMessages)

	s.steeringMessages = make([]string, 0)
	s.followUpMessages = make([]string, 0)
	s.agent.ClearAllQueues()

	return map[string][]string{
		"steering": steering,
		"followUp": followUp,
	}
}

// Abort 中止当前操作
func (s *AgentSession) Abort(ctx context.Context) error {
	s.abortRetry()
	s.agent.Abort()
	return s.agent.WaitForIdle()
}

// WaitForIdle 等待代理空闲
func (s *AgentSession) WaitForIdle() error {
	return s.agent.WaitForIdle()
}

// abortRetry 中止重试（使用 Promise 机制）
func (s *AgentSession) abortRetry() {
	if s.retryAbortController != nil {
		s.retryAbortController()
	}
	s.resolveRetry()
}

// resolveRetry 解决重试 Promise
func (s *AgentSession) resolveRetry() {
	s.retryPromiseMu.Lock()
	defer s.retryPromiseMu.Unlock()

	if s.retryPromise != nil {
		close(s.retryPromise)
		s.retryPromise = nil
	}
}

// AbortRetry 中止重试（公开方法）
func (s *AgentSession) AbortRetry() {
	s.abortRetry()
}

// WaitForRetry 等待重试完成
// 如果正在进行重试，等待其完成；否则立即返回
func (s *AgentSession) WaitForRetry(ctx context.Context) error {
	s.retryPromiseMu.Lock()
	promise := s.retryPromise
	s.retryPromiseMu.Unlock()

	if promise == nil {
		return nil
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-promise:
		return nil
	}
}

// ============================================================================
// 接口方法实现 - Getter 方法
// ============================================================================

// GetAgent 获取代理
func (s *AgentSession) GetAgent() *agent.Agent {
	return s.agent
}

// GetSessionManager 获取会话管理器
func (s *AgentSession) GetSessionManager() *SessionManager {
	return s.sessionManager
}

// GetSettingsManager 获取设置管理器
func (s *AgentSession) GetSettingsManager() *SettingsManager {
	return s.settingsManager
}

// GetModelRegistry 获取模型注册表
func (s *AgentSession) GetModelRegistry() *ModelRegistry {
	return s.modelRegistry
}

// GetResourceLoader 获取资源加载器
func (s *AgentSession) GetResourceLoader() ResourceLoader {
	return s.resourceLoader
}

// ============================================================================
// 接口方法实现 - Bash 执行
// ============================================================================

// BashExecutionOptions Bash 执行选项
type BashExecutionOptions struct {
	TimeoutMs           int
	WorkingDir          string
	Env                 map[string]string
	Stdin               string
	DisableContextFiles bool
	ExcludeFromContext  bool
}

// BashResult Bash 执行结果
type BashResult struct {
	Output         string // 合并的 stdout + stderr 输出
	ExitCode       *int   // 进程退出码（如果被取消则为 nil）
	Cancelled      bool   // 是否通过信号取消
	Truncated      bool   // 输出是否被截断
	FullOutputPath string // 完整输出的临时文件路径（如果输出超过截断阈值）
}

// BashRecordOptions Bash 记录选项
type BashRecordOptions struct {
	ExcludeFromContext bool
}

// ExecuteBash 执行 Bash 命令
func (s *AgentSession) ExecuteBash(ctx context.Context, command string, options *BashExecutionOptions) (*BashResult, error) {
	s.bashMu.Lock()
	s.bashRunning = true
	s.bashMu.Unlock()

	defer func() {
		s.bashMu.Lock()
		s.bashRunning = false
		s.bashAbortController = nil
		s.bashMu.Unlock()
	}()

	ctx, cancel := context.WithCancel(ctx)
	s.bashMu.Lock()
	s.bashAbortController = cancel
	s.bashMu.Unlock()

	workingDir := s.cwd
	if options != nil && options.WorkingDir != "" {
		workingDir = options.WorkingDir
	}

	result, err := s.executeBashCommand(ctx, command, workingDir, options)
	if err != nil {
		cancel()
		return nil, err
	}

	excludeFromContext := options != nil && options.ExcludeFromContext
	s.RecordBashResult(command, result, &BashRecordOptions{ExcludeFromContext: excludeFromContext})

	return result, nil
}

// executeBashCommand 实际执行 Bash 命令
func (s *AgentSession) executeBashCommand(ctx context.Context, command, workingDir string, options *BashExecutionOptions) (*BashResult, error) {
	shell := "/bin/bash"
	shellArgs := []string{"-c", command}

	execCmd := exec.CommandContext(ctx, shell, shellArgs...)
	execCmd.Dir = workingDir

	if options != nil {
		if len(options.Env) > 0 {
			execCmd.Env = os.Environ()
			for k, v := range options.Env {
				execCmd.Env = append(execCmd.Env, fmt.Sprintf("%s=%s", k, v))
			}
		}
		if options.Stdin != "" {
			execCmd.Stdin = strings.NewReader(options.Stdin)
		}
	}

	var stdout, stderr bytes.Buffer
	execCmd.Stdout = &stdout
	execCmd.Stderr = &stderr

	err := execCmd.Run()

	result := &BashResult{
		Output:    stdout.String() + stderr.String(),
		Cancelled: ctx.Err() == context.Canceled,
	}

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode := exitErr.ExitCode()
			result.ExitCode = &exitCode
		} else if ctx.Err() == context.Canceled {
			result.Cancelled = true
		} else {
			return nil, err
		}
	} else {
		exitCode := 0
		result.ExitCode = &exitCode
	}

	return result, nil
}

// RecordBashResult 记录 Bash 执行结果
func (s *AgentSession) RecordBashResult(command string, result *BashResult, options *BashRecordOptions) {
	bashMessage := BashExecutionMessage{
		Role:      "bashExecution",
		Type:      "bash_execution",
		Command:   command,
		Output:    result.Output,
		ExitCode:  result.ExitCode,
		Cancelled: result.Cancelled,
		Truncated: result.Truncated,
		Timestamp: time.Now().UnixMilli(),
	}

	if options != nil {
		bashMessage.ExcludeFromContext = options.ExcludeFromContext
	}

	if s.IsStreaming() {
		s.messagesMu.Lock()
		s.pendingBashMessages = append(s.pendingBashMessages, bashMessage)
		s.messagesMu.Unlock()
	} else {
		s.agent.AppendMessage(&bashMessage)
		s.sessionManager.AppendMessage(&bashMessage)
	}
}

// AbortBash 中止 Bash 执行
func (s *AgentSession) AbortBash() {
	s.bashMu.Lock()
	defer s.bashMu.Unlock()
	if s.bashAbortController != nil {
		s.bashAbortController()
	}
}

// IsBashRunning 检查 Bash 是否正在运行
func (s *AgentSession) IsBashRunning() bool {
	s.bashMu.Lock()
	defer s.bashMu.Unlock()
	return s.bashRunning
}

// HasPendingBashMessages 检查是否有待处理的 Bash 消息
func (s *AgentSession) HasPendingBashMessages() bool {
	s.messagesMu.Lock()
	defer s.messagesMu.Unlock()
	return len(s.pendingBashMessages) > 0
}

// ============================================================================
// 接口方法实现 - 会话管理
// ============================================================================

// ForkResult 分支结果
type ForkResult struct {
	SelectedText string
	Cancelled    bool
}

// ForkableMessage 可分支消息
type ForkableMessage struct {
	EntryId string
	Text    string
}

// NavigateTreeOptions 导航树选项
type NavigateTreeOptions struct {
	TargetId          string  // 目标条目 ID
	Summarize         bool    // 是否用户想要摘要被放弃的分支
	CustomInstructions string // 摘要的自定义指令
	ReplaceInstructions bool   // 是否用自定义指令替换默认提示
	Label             string  // 附加到分支摘要条目的标签
}

// NavigateTreeResult 导航树结果
type NavigateTreeResult struct {
	EditorText    string           // 如果是用户消息，返回文本用于编辑器预填充
	Cancelled     bool             // 是否被扩展取消
	Aborted       bool             // 是否被中止
	SummaryEntry  *BranchSummaryEntry // 摘要条目（如果有）
}

// SwitchSession 切换会话
func (s *AgentSession) SwitchSession(ctx context.Context, sessionPath string) (bool, error) {
	previousSessionFile := s.sessionManager.GetSessionFile()

	s.DisconnectFromAgent()

	if err := s.Abort(ctx); err != nil {
		return false, err
	}

	s.messagesMu.Lock()
	s.steeringMessages = make([]string, 0)
	s.followUpMessages = make([]string, 0)
	s.pendingNextTurnMessages = make([]CustomMessage, 0)
	s.messagesMu.Unlock()

	s.sessionManager.SetSessionFile(sessionPath)

	sessionContext := s.sessionManager.BuildSessionContext()
	s.agent.ReplaceMessages(sessionContext.Messages)

	if sessionContext.Model != nil {
		availableModels, err := s.modelRegistry.GetAvailable()
		if err == nil {
			for _, m := range availableModels {
				if string(m.GetProvider()) == sessionContext.Model.Provider && m.GetID() == sessionContext.Model.ModelID {
					s.agent.SetModel(m)
					break
				}
			}
		}
	}

	if sessionContext.ThinkingLevel != "" {
		s.agent.SetThinkingLevel(ThinkingLevel(sessionContext.ThinkingLevel))
	}

	s.ReconnectToAgent()

	s.emit(&SessionSwitchEvent{
		Type:                "session_switch",
		Reason:              "resume",
		PreviousSessionFile: previousSessionFile,
	})

	return true, nil
}

// Fork 创建分支
func (s *AgentSession) Fork(ctx context.Context, entryId string) (*ForkResult, error) {
	previousSessionFile := s.sessionManager.GetSessionFile()
	selectedEntry := s.sessionManager.GetEntry(entryId)

	if selectedEntry == nil {
		return nil, errors.New("entry not found")
	}

	msgEntry, ok := selectedEntry.(*SessionMessageEntry)
	if !ok {
		return nil, errors.New("invalid entry type for forking")
	}

	role, _ := msgEntry.Message["role"].(string)
	if role != "user" {
		return nil, errors.New("can only fork from user messages")
	}

	selectedText := s.extractUserMessageText(msgEntry.Message)

	s.messagesMu.Lock()
	s.pendingNextTurnMessages = make([]CustomMessage, 0)
	s.messagesMu.Unlock()

	parentID := selectedEntry.GetParentID()
	if parentID == "" {
		s.sessionManager.NewSession(&NewSessionOptions{ParentSession: previousSessionFile})
	} else {
		s.sessionManager.CreateBranchedSession(parentID)
	}

	sessionContext := s.sessionManager.BuildSessionContext()
	s.agent.ReplaceMessages(sessionContext.Messages)

	s.emit(&SessionSwitchEvent{
		Type:                "session_switch",
		Reason:              "fork",
		PreviousSessionFile: previousSessionFile,
	})

	return &ForkResult{
		SelectedText: selectedText,
		Cancelled:    false,
	}, nil
}

// extractUserMessageText 提取用户消息文本
func (s *AgentSession) extractUserMessageText(msgMap map[string]any) string {
	content, ok := msgMap["content"]
	if !ok {
		return ""
	}

	switch c := content.(type) {
	case string:
		return c
	case []ai.ContentBlock:
		var text string
		for _, block := range c {
			if tc, ok := block.(*ai.TextContentBlock); ok {
				text += tc.Text
			}
		}
		return text
	case []interface{}:
		var text string
		for _, item := range c {
			if m, ok := item.(map[string]interface{}); ok {
				if t, ok := m["text"].(string); ok {
					text += t
				}
			}
		}
		return text
	}
	return ""
}

// NavigateTree 导航会话树
func (s *AgentSession) NavigateTree(ctx context.Context, options *NavigateTreeOptions) (*NavigateTreeResult, error) {
	if options == nil {
		return nil, errors.New("options is required")
	}

	targetID := options.TargetId
	if targetID == "" {
		return nil, errors.New("targetId is required")
	}

	oldLeafID := s.sessionManager.GetLeafID()

	if targetID == oldLeafID {
		return &NavigateTreeResult{Cancelled: false}, nil
	}

	targetEntry := s.sessionManager.GetEntry(targetID)
	if targetEntry == nil {
		return nil, fmt.Errorf("entry %s not found", targetID)
	}

	var editorText string

	if msgEntry, ok := targetEntry.(*SessionMessageEntry); ok {
		role, _ := msgEntry.Message["role"].(string)
		if role == "user" {
			editorText = s.extractUserMessageText(msgEntry.Message)
			s.sessionManager.Branch(targetEntry.GetParentID())
		} else {
			s.sessionManager.Branch(targetID)
		}
	} else if customEntry, ok := targetEntry.(*CustomMessageEntry); ok {
		switch c := customEntry.Content.(type) {
		case string:
			editorText = c
		case []interface{}:
			for _, item := range c {
				if m, ok := item.(map[string]interface{}); ok {
					if t, ok := m["text"].(string); ok {
						editorText += t
					}
				}
			}
		}
		s.sessionManager.Branch(targetEntry.GetParentID())
	} else {
		s.sessionManager.Branch(targetID)
	}

	if options.Label != "" {
		s.sessionManager.AppendLabelChange(targetID, options.Label)
	}

	sessionContext := s.sessionManager.BuildSessionContext()
	s.agent.ReplaceMessages(sessionContext.Messages)

	return &NavigateTreeResult{
		EditorText: editorText,
		Cancelled:  false,
	}, nil
}

// GetUserMessagesForForking 获取可用于分支的用户消息
func (s *AgentSession) GetUserMessagesForForking() []ForkableMessage {
	entries := s.sessionManager.GetEntries()
	result := []ForkableMessage{}

	for _, entry := range entries {
		msgEntry, ok := entry.(*SessionMessageEntry)
		if !ok {
			continue
		}

		role, _ := msgEntry.Message["role"].(string)
		if role != "user" {
			continue
		}

		text := s.extractUserMessageText(msgEntry.Message)
		if text != "" {
			result = append(result, ForkableMessage{
				EntryId: entry.GetID(),
				Text:    text,
			})
		}
	}

	return result
}

// ============================================================================
// 接口方法实现 - 统计和导出
// ============================================================================

// ContextUsage 上下文使用情况
type ContextUsage struct {
	Tokens        *int   // 估算的上下文令牌数，如果未知则为 nil（例如压缩后，下一次 LLM 响应之前）
	ContextWindow int    // 上下文窗口大小
	Percent       *float64 // 上下文使用百分比，如果令牌未知则为 nil
}

// GetContextUsage 获取上下文使用情况
func (s *AgentSession) GetContextUsage() *ContextUsage {
	model := s.Model()
	if model == nil {
		return nil
	}

	contextWindow := model.GetContextWindow()
	if contextWindow <= 0 {
		return nil
	}

	// 估算当前上下文令牌数
	messages := s.Messages()
	estimatedTokens := s.estimateContextTokens(messages)

	if estimatedTokens < 0 {
		return &ContextUsage{
			Tokens:        nil,
			ContextWindow: contextWindow,
			Percent:       nil,
		}
	}

	percent := float64(estimatedTokens) / float64(contextWindow) * 100

	return &ContextUsage{
		Tokens:        &estimatedTokens,
		ContextWindow: contextWindow,
		Percent:       &percent,
	}
}

// estimateContextTokens 估算上下文令牌数
func (s *AgentSession) estimateContextTokens(messages []ai.Message) int {
	totalTokens := 0

	for _, msg := range messages {
		switch m := msg.(type) {
		case *ai.AssistantMessage:
			// 助手消息通常包含使用统计
			totalTokens += m.Usage.Input + m.Usage.Output
		case *ai.UserMessage:
			// 用户消息估算：每个字符约 0.25 个 token
			switch content := m.Content.(type) {
			case string:
				totalTokens += len(content) / 4
			case []ai.ContentBlock:
				for _, block := range content {
					if tc, ok := block.(*ai.TextContentBlock); ok {
						totalTokens += len(tc.Text) / 4
					}
				}
			}
		}
	}

	return totalTokens
}

// ExportToHtml 导出为 HTML
func (s *AgentSession) ExportToHtml(ctx context.Context, outputPath string) (string, error) {
	// HTML 导出需要实现完整的模板系统
	// 这里提供一个基础实现框架
	return "", errors.New("ExportToHtml requires HTML template implementation")
}

// GetLastAssistantText 获取最后一条助手文本
func (s *AgentSession) GetLastAssistantText() string {
	messages := s.Messages()

	for i := len(messages) - 1; i >= 0; i-- {
		msg := messages[i]
		assistantMsg, ok := msg.(*ai.AssistantMessage)
		if !ok {
			continue
		}

		// 跳过中止的空消息
		if assistantMsg.StopReason == "aborted" && len(assistantMsg.Content) == 0 {
			continue
		}

		// 提取文本内容
		var text string
		for _, block := range assistantMsg.Content {
			if tc, ok := block.(*ai.TextContentBlock); ok {
				text += tc.Text
			}
		}

		if text != "" {
			return strings.TrimSpace(text)
		}
	}

	return ""
}

// NewSession 创建新会话
func (s *AgentSession) NewSession(ctx context.Context, options *NewSessionOptions) error {
	previousSessionFile := s.sessionManager.GetSessionFile()
	// 断开代理连接
	s.DisconnectFromAgent()
	// 中止当前操作
	if err := s.Abort(ctx); err != nil {
		return err
	}
	// 重置代理
	s.agent.Reset()
	// 创建新会话
	s.sessionManager.NewSession(options)
	// 更新会话 ID
	s.agent.SetSessionID(s.sessionManager.GetSessionID())
	// 清空消息队列
	s.messagesMu.Lock()
	s.steeringMessages = make([]string, 0)
	s.followUpMessages = make([]string, 0)
	s.pendingNextTurnMessages = make([]CustomMessage, 0)
	s.messagesMu.Unlock()
	// 记录思考级别变更
	s.sessionManager.AppendThinkingLevelChangeFromLevel(s.agent.GetState().ThinkingLevel)
	// 重新连接代理
	s.ReconnectToAgent()
	// 发送会话切换事件
	s.emit(&SessionSwitchEvent{
		Type:                "session_switch",
		Reason:              "new",
		PreviousSessionFile: previousSessionFile,
	})
	return nil
}

// SessionStats 会话统计
type SessionStats struct {
	SessionFile       string     `json:"sessionFile"`
	SessionId         string     `json:"sessionId"`
	UserMessages      int        `json:"userMessages"`
	AssistantMessages int        `json:"assistantMessages"`
	ToolCalls         int        `json:"toolCalls"`
	ToolResults       int        `json:"toolResults"`
	TotalMessages     int        `json:"totalMessages"`
	Tokens            TokenStats `json:"tokens"`
	Cost              float64    `json:"cost"`
}

// TokenStats Token 统计
type TokenStats struct {
	Input      int `json:"input"`
	Output     int `json:"output"`
	CacheRead  int `json:"cacheRead"`
	CacheWrite int `json:"cacheWrite"`
	Total      int `json:"total"`
}

// GetSessionStats 获取会话统计
func (s *AgentSession) GetSessionStats() *SessionStats {
	messages := s.agent.GetState().Messages

	stats := &SessionStats{
		SessionFile:   s.sessionManager.GetSessionFile(),
		SessionId:     s.sessionManager.GetSessionID(),
		TotalMessages: len(messages),
		Tokens: TokenStats{
			Input:      0,
			Output:     0,
			CacheRead:  0,
			CacheWrite: 0,
			Total:      0,
		},
		Cost: 0,
	}

	for _, msg := range messages {
		switch m := msg.(type) {
		case *ai.UserMessage:
			stats.UserMessages++
		case *ai.AssistantMessage:
			stats.AssistantMessages++
			stats.Tokens.Input += m.Usage.Input
			stats.Tokens.Output += m.Usage.Output
			stats.Tokens.CacheRead += m.Usage.CacheRead
			stats.Tokens.CacheWrite += m.Usage.CacheWrite
			stats.Tokens.Total += m.Usage.TotalTokens
			stats.Cost += m.Usage.Cost.Total

			// 统计工具调用
			for _, block := range m.Content {
				if _, ok := block.(*ai.ToolCall); ok {
					stats.ToolCalls++
				}
			}
		case *ai.ToolResultMessage:
			stats.ToolResults++
		}
	}

	return stats
}