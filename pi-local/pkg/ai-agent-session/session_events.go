package session

import (
	"github.com/jay-y/pi/pkg/ai"
	agent "github.com/jay-y/pi/pkg/ai-agent"
)

// AgentSessionEvent 会话特定事件，扩展了核心 AgentEvent
type AgentSessionEvent interface {
	agent.AgentEvent
}

// AutoCompactionStartEvent 自动压缩开始事件
type AutoCompactionStartEvent struct {
	Type   string `json:"type"`
	Reason string `json:"reason"` // "threshold" | "overflow"
}

func (e *AutoCompactionStartEvent) GetType() string {
	return e.Type
}

// AutoCompactionEndEvent 自动压缩结束事件
type AutoCompactionEndEvent struct {
	Type         string            `json:"type"`
	Result       *CompactionResult `json:"result,omitempty"`
	Aborted      bool              `json:"aborted"`
	WillRetry    bool              `json:"willRetry"`
	ErrorMessage string            `json:"errorMessage,omitempty"`
}

func (e *AutoCompactionEndEvent) GetType() string {
	return e.Type
}

// AutoRetryStartEvent 自动重试开始事件
type AutoRetryStartEvent struct {
	Type         string `json:"type"`
	Attempt      int    `json:"attempt"`
	MaxAttempts  int    `json:"maxAttempts"`
	DelayMs      int    `json:"delayMs"`
	ErrorMessage string `json:"errorMessage"`
}

func (e *AutoRetryStartEvent) GetType() string {
	return e.Type
}

// AutoRetryEndEvent 自动重试结束事件
type AutoRetryEndEvent struct {
	Type       string `json:"type"`
	Success    bool   `json:"success"`
	Attempt    int    `json:"attempt"`
	FinalError string `json:"finalError,omitempty"`
}

func (e *AutoRetryEndEvent) GetType() string {
	return e.Type
}

// SessionSwitchEvent 会话切换事件
type SessionSwitchEvent struct {
	Type                string `json:"type"`
	Reason              string `json:"reason"` // "new" | "switch" | "fork"
	PreviousSessionFile string `json:"previousSessionFile,omitempty"`
}

func (e *SessionSwitchEvent) GetType() string {
	return e.Type
}

// ModelSelectEvent 模型选择事件
type ModelSelectEvent struct {
	Type          string `json:"type"`
	Model         ai.Model  `json:"model"`
	PreviousModel ai.Model  `json:"previousModel,omitempty"`
	Source        string `json:"source"` // "set" | "cycle" | "restore"
}

func (e *ModelSelectEvent) GetType() string {
	return e.Type
}

// SessionStartEvent 会话开始事件
type SessionStartEvent struct {
	Type string `json:"type"`
}

func (e *SessionStartEvent) GetType() string {
	return e.Type
}

// SessionBeforeSwitchResult 会话切换前结果
type SessionBeforeSwitchResult struct {
	Cancel bool `json:"cancel"`
}

// SessionBeforeCompactResult 会话压缩前结果
type SessionBeforeCompactResult struct {
	Cancel     bool              `json:"cancel"`
	Compaction *CompactionResult `json:"compaction,omitempty"`
}

// SessionBeforeForkResult 会话分叉前结果
type SessionBeforeForkResult struct {
	Cancel bool `json:"cancel"`
}

// SessionBeforeTreeResult 会话树前结果
type SessionBeforeTreeResult struct {
	Cancel bool `json:"cancel"`
}

// AgentSessionEventListener 会话事件监听器
type AgentSessionEventListener func(event AgentSessionEvent)
