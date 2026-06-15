package piagent

import "time"

type ChatRequest struct {
	Message   string `json:"message"`
	SessionID string `json:"session_id"`
	ModelName string `json:"model_name"`
}

type ChatResponse struct {
	SessionID string `json:"session_id"`
}

type SessionInfo struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	MsgCount  int64     `json:"msg_count"`
	ModelName string    `json:"model_name,omitempty"`
}

type SessionDetail struct {
	SessionInfo
	Messages []SessionMessage `json:"messages"`
}

type SessionMessage struct {
	ID        uint      `json:"id"`
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

type SSEType string

const (
	SSETypeText      SSEType = "text"
	SSETypeToolStart SSEType = "tool_start"
	SSETypeToolEnd   SSEType = "tool_end"
	SSETypeDone      SSEType = "done"
	SSETypeError     SSEType = "error"
)

type SSEEvent struct {
	Type     SSEType `json:"type"`
	Content  string  `json:"content,omitempty"`
	ToolName string  `json:"tool_name,omitempty"`
}

type TestModelRequest struct {
	Name          string `json:"name"`
	Provider      string `json:"provider"`
	Model         string `json:"model"`
	BaseURL       string `json:"base_url"`
	APIKey        string `json:"api_key"`
	ThinkingLevel string `json:"thinking_level"`
	MaxTokens     int    `json:"max_tokens"`
	ProxyURL      string `json:"proxy_url"`
}
