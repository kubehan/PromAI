package session

// CustomMessage 自定义消息
type CustomMessage struct {
	Role       string      `json:"role"`
	CustomType string      `json:"customType,omitempty"`
	Content    interface{} `json:"content"`
	Display    bool        `json:"display,omitempty"`
	Details    interface{} `json:"details,omitempty"`
	Timestamp  int64       `json:"timestamp,omitempty"`
}

// GetRole 实现 Message 接口
func (c *CustomMessage) GetRole() string {
	return c.Role
}

// BashExecutionMessage Bash 执行消息
type BashExecutionMessage struct {
	Role               string `json:"role"`
	Type               string `json:"type"`
	Command            string `json:"command"`
	Output             string `json:"output"`
	ExitCode           *int   `json:"exitCode,omitempty"`
	Cancelled          bool   `json:"cancelled"`
	Truncated          bool   `json:"truncated"`
	FullOutputPath     string `json:"fullOutputPath,omitempty"`
	Timestamp          int64  `json:"timestamp"`
	ExcludeFromContext bool   `json:"excludeFromContext,omitempty"`
}

// GetRole 实现 Message 接口
func (m *BashExecutionMessage) GetRole() string {
	return m.Role
}