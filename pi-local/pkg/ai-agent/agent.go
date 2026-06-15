package agent

import (
	"context"
	"fmt"
	"sync"

	"github.com/jay-y/pi/pkg/ai"
)

// Agent 代理接口
type AgentInterface interface {
	Subscribe(listener func(event AgentEvent)) (unsubscribe func())
	GetState() *AgentState
	SetModel(model ai.Model)
	SetThinkingLevel(level ai.ThinkingLevel)
	SetTools(tools []AgentTool)
	SetSystemPrompt(prompt string)
	Prompt(ctx context.Context, input any, images ...*ai.ImageContentBlock) error
	Steer(message ai.Message) error
	FollowUp(message ai.Message) error
	Abort()
	Reset()
	Continue(ctx context.Context) error
	ReplaceMessages(messages []ai.Message)
	AppendMessage(message ai.Message)
	ClearAllQueues()
	HasQueuedMessages() bool
	WaitForIdle() error
	GetSteeringMode() string
	GetFollowUpMode() string
	SetSteeringMode(mode string)
	SetFollowUpMode(mode string)
	SetSessionID(sessionID string)
}

// AgentContext 代理上下文
type AgentContext struct {
	SystemPrompt string `json:"systemPrompt"`
	Messages     []ai.Message `json:"messages"`
	Tools        []AgentTool `json:"tools"`
}

// AgentState 代理状态
type AgentState struct {
	SystemPrompt     string `json:"systemPrompt"`
	Model            ai.Model `json:"model"`
	ThinkingLevel    ai.ThinkingLevel `json:"thinkingLevel"`
	Tools            []AgentTool `json:"tools"`
	Messages         []ai.Message `json:"messages"`
	IsStreaming      bool `json:"isStreaming"`
	StreamMessage    ai.Message `json:"streamMessage"`
	PendingToolCalls map[string]bool `json:"pendingToolCalls"`
	Error            string `json:"error"`
}

// StreamFn 自定义的流式调用函数
type StreamFn func(model ai.Model, ctx ai.Context, opts *ai.SimpleStreamOptions) (*ai.AssistantMessageEventStream, error)

// AgentOptions 代理配置选项
type AgentOptions struct {
	InitialState     *AgentState
	ConvertToLLM     func(messages []ai.Message) ([]ai.Message, error)
	TransformContext func(messages []ai.Message, ctx context.Context) ([]ai.Message, error)
	SteeringMode     string
	FollowUpMode     string
	StreamFn         StreamFn
	SessionID        string
	GetApiKey        func(provider string) (string, error)
	ThinkingBudgets  *ai.ThinkingBudgets
	Transport        ai.Transport
	MaxRetryDelayMs  int
}

// Agent 代理类
type Agent struct {
	state            *AgentState
	listeners        []func(e AgentEvent)
	mu               sync.RWMutex
	convertToLLM     func(messages []ai.Message) ([]ai.Message, error)
	transformContext func(messages []ai.Message, ctx context.Context) ([]ai.Message, error)
	steeringQueue    []ai.Message
	followUpQueue    []ai.Message
	steeringMode     string
	followUpMode     string
	streamFn         StreamFn
	sessionID        string
	getApiKey        func(provider string) (string, error)
	runningPrompt    chan struct{}
	thinkingBudgets  *ai.ThinkingBudgets
	transport        ai.Transport
	maxRetryDelayMs  int
	ctx              context.Context
	cancel           context.CancelFunc
}

// NewAgent 创建新的代理
func NewAgent(opts ...AgentOptions) *Agent {
	ctx, cancel := context.WithCancel(context.Background())

	agent := &Agent{
		state: &AgentState{
			SystemPrompt:     "",
			Model:            nil,
			ThinkingLevel:    ai.ThinkingLevelOff,
			Tools:            []AgentTool{},
			Messages:         []ai.Message{},
			IsStreaming:      false,
			StreamMessage:    nil,
			PendingToolCalls: map[string]bool{},
			Error:            "",
		},
		listeners:        []func(e AgentEvent){},
		convertToLLM:     defaultConvertToLLM,
		transformContext: nil,
		steeringQueue:    []ai.Message{},
		followUpQueue:    []ai.Message{},
		steeringMode:     "one-at-a-time",
		followUpMode:     "one-at-a-time",
		streamFn:         ai.StreamSimple,
		sessionID:        "",
		getApiKey:        nil,
		runningPrompt:    nil,
		thinkingBudgets:  nil,
		transport:        ai.TransportSSE,
		maxRetryDelayMs:  60000,
		ctx:              ctx,
		cancel:           cancel,
	}

	for _, opt := range opts {
		if opt.InitialState != nil {
			// merge opt.InitialState with agent.state using a helper function
			mergeAgentState(agent.state, opt.InitialState)
		}
		if opt.ConvertToLLM != nil {
			agent.convertToLLM = opt.ConvertToLLM
		}
		if opt.TransformContext != nil {
			agent.transformContext = opt.TransformContext
		}
		if opt.SteeringMode != "" {
			agent.steeringMode = opt.SteeringMode
		}
		if opt.FollowUpMode != "" {
			agent.followUpMode = opt.FollowUpMode
		}
		if opt.StreamFn != nil {
			agent.streamFn = opt.StreamFn
		}
		agent.sessionID = opt.SessionID
		agent.getApiKey = opt.GetApiKey
		agent.thinkingBudgets = opt.ThinkingBudgets
		if opt.Transport != "" {
			agent.transport = opt.Transport
		}
		if opt.MaxRetryDelayMs > 0 {
			agent.maxRetryDelayMs = opt.MaxRetryDelayMs
		}
	}

	return agent
}

func (a *Agent) GetState() *AgentState {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.state
}

// GetSessionID 获取当前的会话 ID
func (a *Agent) GetSessionID() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.sessionID
}

// SetSessionID 设置会话 ID
func (a *Agent) SetSessionID(sessionID string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.sessionID = sessionID
}

// GetThinkingBudgets 获取当前的思考预算
func (a *Agent) GetThinkingBudgets() *ai.ThinkingBudgets {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.thinkingBudgets
}

// SetThinkingBudgets 设置思考预算
func (a *Agent) SetThinkingBudgets(budgets *ai.ThinkingBudgets) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.thinkingBudgets = budgets
}

// GetTransport 获取当前的传输方式
func (a *Agent) GetTransport() ai.Transport {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.transport
}

// SetTransport 设置传输方式
func (a *Agent) SetTransport(transport ai.Transport) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.transport = transport
}

// GetMaxRetryDelayMs 获取最大重试延迟
func (a *Agent) GetMaxRetryDelayMs() int {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.maxRetryDelayMs
}

// SetMaxRetryDelayMs 设置最大重试延迟
func (a *Agent) SetMaxRetryDelayMs(delay int) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.maxRetryDelayMs = delay
}

// Subscribe 订阅代理事件
func (a *Agent) Subscribe(fn func(e AgentEvent)) func() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.listeners = append(a.listeners, fn)
	index := len(a.listeners) - 1
	return func() {
		a.mu.Lock()
		defer a.mu.Unlock()
		if index < len(a.listeners) {
			a.listeners = append(a.listeners[:index], a.listeners[index+1:]...)
		}
	}
}

// SetSystemPrompt 设置系统提示
func (a *Agent) SetSystemPrompt(prompt string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.state.SystemPrompt = prompt
}

// SetModel 设置模型
func (a *Agent) SetModel(model ai.Model) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.state.Model = model
}

// SetThinkingLevel 设置思考级别
func (a *Agent) SetThinkingLevel(level ai.ThinkingLevel) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.state.ThinkingLevel = level
}

// SetSteeringMode 设置引导模式
func (a *Agent) SetSteeringMode(mode string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.steeringMode = mode
}

// GetSteeringMode 获取引导模式
func (a *Agent) GetSteeringMode() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.steeringMode
}

// SetFollowUpMode 设置跟进模式
func (a *Agent) SetFollowUpMode(mode string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.followUpMode = mode
}

// GetFollowUpMode 获取跟进模式
func (a *Agent) GetFollowUpMode() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.followUpMode
}

// SetTools 设置工具
func (a *Agent) SetTools(tools []AgentTool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.state.Tools = tools
}

// ReplaceMessages 替换所有消息
func (a *Agent) ReplaceMessages(messages []ai.Message) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.state.Messages = append([]ai.Message{}, messages...)
}

// AppendMessage 追加一条消息
func (a *Agent) AppendMessage(message ai.Message) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.state.Messages = append(a.state.Messages, message)
}

// Steer 发送引导消息来中断当前的代理运行
func (a *Agent) Steer(message ai.Message) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.steeringQueue = append(a.steeringQueue, message)
	return nil
}

// FollowUp 发送跟进消息，在代理结束后处理
func (a *Agent) FollowUp(message ai.Message) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.followUpQueue = append(a.followUpQueue, message)
	return nil
}

// ClearSteeringQueue 清空引导队列
func (a *Agent) ClearSteeringQueue() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.steeringQueue = []ai.Message{}
}

// ClearFollowUpQueue 清空跟进队列
func (a *Agent) ClearFollowUpQueue() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.followUpQueue = []ai.Message{}
}

// ClearAllQueues 清空所有队列
func (a *Agent) ClearAllQueues() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.steeringQueue = []ai.Message{}
	a.followUpQueue = []ai.Message{}
}

// HasQueuedMessages 检查是否有排队的消息
func (a *Agent) HasQueuedMessages() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return len(a.steeringQueue) > 0 || len(a.followUpQueue) > 0
}

// dequeueSteeringMessages 从引导队列中取出消息
func (a *Agent) dequeueSteeringMessages() []ai.Message {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.steeringMode == "one-at-a-time" {
		if len(a.steeringQueue) > 0 {
			first := a.steeringQueue[0]
			a.steeringQueue = a.steeringQueue[1:]
			return []ai.Message{first}
		}
		return []ai.Message{}
	}

	steering := append([]ai.Message{}, a.steeringQueue...)
	a.steeringQueue = []ai.Message{}
	return steering
}

// dequeueFollowUpMessages 从跟进队列中取出消息
func (a *Agent) dequeueFollowUpMessages() []ai.Message {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.followUpMode == "one-at-a-time" {
		if len(a.followUpQueue) > 0 {
			first := a.followUpQueue[0]
			a.followUpQueue = a.followUpQueue[1:]
			return []ai.Message{first}
		}
		return []ai.Message{}
	}

	followUp := append([]ai.Message{}, a.followUpQueue...)
	a.followUpQueue = []ai.Message{}
	return followUp
}

// ClearMessages 清空所有消息
func (a *Agent) ClearMessages() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.state.Messages = []ai.Message{}
}

// Abort 中止当前的操作
func (a *Agent) Abort() {
	if a.cancel != nil {
		a.cancel()
	}
}

// WaitForIdle 等待代理空闲
func (a *Agent) WaitForIdle() error {
	a.mu.RLock()
	if a.runningPrompt == nil {
		a.mu.RUnlock()
		return nil
	}
	ch := a.runningPrompt
	a.mu.RUnlock()
	<-ch
	return nil
}

// Reset 重置代理状态
func (a *Agent) Reset() {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.state.Messages = []ai.Message{}
	a.state.IsStreaming = false
	a.state.StreamMessage = nil
	a.state.PendingToolCalls = map[string]bool{}
	a.state.Error = ""
	a.steeringQueue = []ai.Message{}
	a.followUpQueue = []ai.Message{}
}

// Prompt 发送提示消息
func (a *Agent) Prompt(ctx context.Context, input any, images ...*ai.ImageContentBlock) error {
	a.mu.Lock()
	if a.state.IsStreaming {
		a.mu.Unlock()
		return fmt.Errorf("Agent is already processing a prompt. Use steer() or followUp() to queue messages, or wait for completion.")
	}

	model := a.state.Model
	if model == nil {
		a.mu.Unlock()
		return fmt.Errorf("No model configured")
	}

	var msgs []ai.Message

	switch v := input.(type) {
	case []ai.Message:
		msgs = v
	case string:
		content := []ai.ContentBlock{ai.NewTextContentBlock(v)}
		if len(images) > 0 {
			for _, img := range images {
				content = append(content, img)
			}
		}
		msgs = []ai.Message{ai.NewUserMessage(content)}
	case ai.Message:
		msgs = []ai.Message{v}
	default:
		a.mu.Unlock()
		return fmt.Errorf("Invalid input type")
	}

	a.state.IsStreaming = true
	a.state.StreamMessage = nil
	a.state.Error = ""
	a.mu.Unlock()

	return a.runLoop(ctx, msgs, nil)
}

// Continue 从当前上下文继续
func (a *Agent) Continue(ctx context.Context) error {
	a.mu.Lock()
	if a.state.IsStreaming {
		a.mu.Unlock()
		return fmt.Errorf("Agent is already processing. Wait for completion before continuing.")
	}

	messages := a.state.Messages
	if len(messages) == 0 {
		a.mu.Unlock()
		return fmt.Errorf("No messages to continue from")
	}

	lastMsg := messages[len(messages)-1]
	if _, ok := lastMsg.(*ai.AssistantMessage); ok {
	// if am, ok := lastMsg.(*ai.AssistantMessage); ok {	
		// 先释放锁，再调用 dequeue 方法，避免死锁
		a.mu.Unlock()
		
		queuedSteering := a.dequeueSteeringMessages()
		if len(queuedSteering) > 0 {
			a.mu.Lock()
			a.state.IsStreaming = true
			a.mu.Unlock()
			return a.runLoop(ctx, queuedSteering, &struct{ SkipInitialSteeringPoll bool }{SkipInitialSteeringPoll: true})
		}

		queuedFollowUp := a.dequeueFollowUpMessages()
		if len(queuedFollowUp) > 0 {
			a.mu.Lock()
			a.state.IsStreaming = true
			a.mu.Unlock()
			return a.runLoop(ctx, queuedFollowUp, nil)
		}

		// 如果最后一条消息是错误状态，允许继续（用于重试机制）
		// if am.StopReason == "error" {
		// 	a.mu.Lock()
		// 	a.state.IsStreaming = true
		// 	a.mu.Unlock()
		// 	return a.runLoop(ctx, nil, nil)
		// }
		a.mu.Unlock()

		return fmt.Errorf("Cannot continue from message role: assistant")
	}

	a.state.IsStreaming = true
	a.mu.Unlock()

	return a.runLoop(ctx, nil, nil)
}

// runLoop 运行代理循环
func (a *Agent) runLoop(ctx context.Context, messages []ai.Message, options *struct{ SkipInitialSteeringPoll bool }) error {
	a.mu.Lock()
	model := a.state.Model
	if model == nil {
		a.mu.Unlock()
		return fmt.Errorf("No model configured")
	}

	a.runningPrompt = make(chan struct{})
	defer func() {
		a.mu.Lock()
		if a.runningPrompt != nil {
			close(a.runningPrompt)
		}
		a.runningPrompt = nil
		a.mu.Unlock()
	}()

	reasoning := a.state.ThinkingLevel
	if reasoning == ai.ThinkingLevelOff {
		reasoning = ""
	}

	agentContext := AgentContext{
		SystemPrompt: a.state.SystemPrompt,
		Messages:     append([]ai.Message{}, a.state.Messages...),
		Tools:        append([]AgentTool{}, a.state.Tools...),
	}

	skipInitialSteeringPoll := false
	if options != nil {
		skipInitialSteeringPoll = options.SkipInitialSteeringPoll
	}

	config := AgentLoopConfig{
		Model:            model,
		Reasoning:        reasoning,
		SessionID:        a.sessionID,
		Transport:        a.transport,
		ThinkingBudgets:  a.thinkingBudgets,
		MaxRetryDelayMs:  a.maxRetryDelayMs,
		ConvertToLLM:     a.convertToLLM,
		TransformContext: a.transformContext,
		GetApiKey:        a.getApiKey,
		GetSteeringMessages: func() ([]ai.Message, error) {
			if skipInitialSteeringPoll {
				skipInitialSteeringPoll = false
				return []ai.Message{}, nil
			}
			return a.dequeueSteeringMessages(), nil
		},
		GetFollowUpMessages: func() ([]ai.Message, error) {
			return a.dequeueFollowUpMessages(), nil
		},
	}

	a.mu.Unlock()

	loopCtx, loopCancel := context.WithCancel(ctx)
	defer loopCancel()

	var stream *AgentEventStream
	if messages != nil {
		stream = AgentLoop(messages, agentContext, config, loopCtx, a.streamFn)
	} else {
		stream = AgentLoopContinue(agentContext, config, loopCtx, a.streamFn)
	}

	var partial ai.Message = nil

	for event := range stream.Events() {
		a.handleEvent(event, &partial)
		a.emit(event)
	}

	a.mu.Lock()
	if partial != nil {
		if am, ok := partial.(*ai.AssistantMessage); ok {
			onlyEmpty := true
			for _, c := range am.Content {
				switch content := c.(type) {
				case *ai.ThinkingContentBlock:
					if len(content.Thinking) > 0 {
						onlyEmpty = false
					}
				case *ai.TextContentBlock:
					if len(content.Text) > 0 {
						onlyEmpty = false
					}
				case *ai.ToolCall:
					if len(content.Name) > 0 {
						onlyEmpty = false
					}
				}
			}
			if !onlyEmpty {
				a.state.Messages = append(a.state.Messages, partial)
			}
		}
	}
	a.state.IsStreaming = false
	a.state.StreamMessage = nil
	a.mu.Unlock()

	return nil
}

// handleEvent 处理单个事件
func (a *Agent) handleEvent(event AgentEvent, partial *ai.Message) {
	a.mu.Lock()
	defer a.mu.Unlock()

	switch e := event.(type) {
	case *AgentEventMessageStart:
		*partial = e.Message
		a.state.StreamMessage = e.Message
	case *AgentEventMessageUpdate:
		*partial = e.Message
		a.state.StreamMessage = e.Message
	case *AgentEventMessageEnd:
		*partial = nil
		a.state.StreamMessage = nil
		a.state.Messages = append(a.state.Messages, e.Message)
	case *AgentEventToolExecutionStart:
		a.state.PendingToolCalls[e.ToolCallID] = true
	case *AgentEventToolExecutionEnd:
		delete(a.state.PendingToolCalls, e.ToolCallID)
	case *AgentEventTurnEnd:
		if am, ok := e.Message.(*ai.AssistantMessage); ok && len(am.ErrorMessage) > 0 {
			a.state.Error = am.ErrorMessage
		}
	case *AgentEventEnd:
		a.state.IsStreaming = false
		a.state.StreamMessage = nil
	}
}

// emit 发出事件给所有监听器
func (a *Agent) emit(event AgentEvent) {
	a.mu.RLock()
	listeners := make([]func(e AgentEvent), 0, len(a.listeners))
	listeners = append(listeners, a.listeners...)
	a.mu.RUnlock()

	for _, fn := range listeners {
		fn(event)
	}
}

// 确保 Agent 实现了 AgentInterface
var _ AgentInterface = (*Agent)(nil)

// defaultConvertToLLM 默认的消息转换函数
func defaultConvertToLLM(messages []ai.Message) ([]ai.Message, error) {
	var result []ai.Message
	for _, msg := range messages {
		switch m := msg.(type) {
		case *ai.UserMessage, *ai.AssistantMessage, *ai.ToolResultMessage:
			result = append(result, m)
		}
	}
	return result, nil
}

// mergeAgentState 将 src 中的非零值合并到 dst
func mergeAgentState(dst, src *AgentState) {
	if src.SystemPrompt != "" {
		dst.SystemPrompt = src.SystemPrompt
	}
	if src.Model != nil {
		dst.Model = src.Model
	}
	if src.ThinkingLevel != ai.ThinkingLevelOff {
		dst.ThinkingLevel = src.ThinkingLevel
	}
	if len(src.Tools) != 0 {
		dst.Tools = src.Tools
	}
	if len(src.Messages) > 0 {
		dst.Messages = src.Messages
	}
	if src.IsStreaming {
		dst.IsStreaming = src.IsStreaming
	}
	if src.StreamMessage != nil {
		dst.StreamMessage = src.StreamMessage
	}
	// 特殊处理：如果 src.PendingToolCalls 不为 nil，则使用它
	// 这允许用户传入空 map 来清空 PendingToolCalls
	if src.PendingToolCalls != nil {
		dst.PendingToolCalls = src.PendingToolCalls
	}
	if src.Error != "" {
		dst.Error = src.Error
	}
}