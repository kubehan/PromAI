package agent

import "github.com/jay-y/pi/pkg/ai"

// AgentEvent 代理事件
type AgentEvent interface {
	GetType() string
}

// AgentEventStart 代理开始事件
type AgentEventStart struct {
	Type string `json:"type"`
}

func (e *AgentEventStart) GetType() string {
	return e.Type
}

// AgentEventEnd 代理结束事件
type AgentEventEnd struct {
	Type     string         `json:"type"`
	Messages []ai.Message `json:"messages"`
}

func (e *AgentEventEnd) GetType() string {
	return e.Type
}

// AgentEventTurnStart 轮次开始事件
type AgentEventTurnStart struct {
	Type string `json:"type"`
}

func (e *AgentEventTurnStart) GetType() string {
	return e.Type
}

// AgentEventTurnEnd 轮次结束事件
type AgentEventTurnEnd struct {
	Type        string              `json:"type"`
	Message     ai.Message        `json:"message"`
	ToolResults []ai.ToolResultMessage `json:"toolResults"`
}

func (e *AgentEventTurnEnd) GetType() string {
	return e.Type
}

// AgentEventMessageStart 消息开始事件
type AgentEventMessageStart struct {
	Type    string       `json:"type"`
	Message ai.Message `json:"message"`
}

func (e *AgentEventMessageStart) GetType() string {
	return e.Type
}

// AgentEventMessageUpdate 消息更新事件
type AgentEventMessageUpdate struct {
	Type                  string                   `json:"type"`
	Message               ai.Message             `json:"message"`
	AssistantMessageEvent ai.AssistantMessageEvent `json:"assistantMessageEvent"`
}

func (e *AgentEventMessageUpdate) GetType() string {
	return e.Type
}

// AgentEventMessageEnd 消息结束事件
type AgentEventMessageEnd struct {
	Type    string       `json:"type"`
	Message ai.Message `json:"message"`
}

func (e *AgentEventMessageEnd) GetType() string {
	return e.Type
}

// AgentEventToolExecutionStart 工具执行开始事件
type AgentEventToolExecutionStart struct {
	Type       string      `json:"type"`
	ToolCallID string      `json:"toolCallId"`
	ToolName   string      `json:"toolName"`
	Args       any `json:"args"`
}

func (e *AgentEventToolExecutionStart) GetType() string {
	return e.Type
}

// AgentEventToolExecutionUpdate 工具执行更新事件
type AgentEventToolExecutionUpdate struct {
	Type          string      `json:"type"`
	ToolCallID    string      `json:"toolCallId"`
	ToolName      string      `json:"toolName"`
	Args          any `json:"args"`
	PartialResult any `json:"partialResult"`
}

func (e *AgentEventToolExecutionUpdate) GetType() string {
	return e.Type
}

// AgentEventToolExecutionEnd 工具执行结束事件
type AgentEventToolExecutionEnd struct {
	Type       string      `json:"type"`
	ToolCallID string      `json:"toolCallId"`
	ToolName   string      `json:"toolName"`
	Result     any `json:"result"`
	IsError    bool        `json:"isError"`
}

func (e *AgentEventToolExecutionEnd) GetType() string {
	return e.Type
}
