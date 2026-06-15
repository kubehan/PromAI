package ai

// AssistantMessageEvent 助手消息事件接口
type AssistantMessageEvent interface {
	GetType() string
}

// AssistantMessageEventStart 开始事件
type AssistantMessageEventStart struct {
	Type    string            `json:"type"`
	Partial *AssistantMessage `json:"partial"`
}

func (e *AssistantMessageEventStart) GetType() string {
	if e.Type == "" {
		e.Type = ASSISTANT_MESSAGE_EVENT_START
	}
	return e.Type 
}

// AssistantMessageEventTextStart 文本开始事件
type AssistantMessageEventTextStart struct {
	Type         string            `json:"type"`
	ContentIndex int               `json:"contentIndex"`
	Partial      *AssistantMessage `json:"partial"`
}

func (e *AssistantMessageEventTextStart) GetType() string {
	if e.Type == "" {
		e.Type = ASSISTANT_MESSAGE_EVENT_TEXT_START
	}
	return e.Type 
}

// AssistantMessageEventTextDelta 文本增量事件
type AssistantMessageEventTextDelta struct {
	Type         string            `json:"type"`
	ContentIndex int               `json:"contentIndex"`
	Delta        string            `json:"delta"`
	Partial      *AssistantMessage `json:"partial"`
}

func (e *AssistantMessageEventTextDelta) GetType() string {
	if e.Type == "" {
		e.Type = ASSISTANT_MESSAGE_EVENT_TEXT_DELTA
	}
	return e.Type 
}

// AssistantMessageEventTextEnd 文本结束事件
type AssistantMessageEventTextEnd struct {
	Type         string            `json:"type"`
	ContentIndex int               `json:"contentIndex"`
	Content      string            `json:"content"`
	Partial      *AssistantMessage `json:"partial"`
}

func (e *AssistantMessageEventTextEnd) GetType() string {
	if e.Type == "" {
		e.Type = ASSISTANT_MESSAGE_EVENT_TEXT_END
	}
	return e.Type 
}

// AssistantMessageEventThinkingStart 思考开始事件
type AssistantMessageEventThinkingStart struct {
	Type         string            `json:"type"`
	ContentIndex int               `json:"contentIndex"`
	Partial      *AssistantMessage `json:"partial"`
}

func (e *AssistantMessageEventThinkingStart) GetType() string {
	if e.Type == "" {
		e.Type = ASSISTANT_MESSAGE_EVENT_THINKING_START
	}
	return e.Type 
}

// AssistantMessageEventThinkingDelta 思考增量事件
type AssistantMessageEventThinkingDelta struct {
	Type         string            `json:"type"`
	ContentIndex int               `json:"contentIndex"`
	Delta        string            `json:"delta"`
	Partial      *AssistantMessage `json:"partial"`
}

func (e *AssistantMessageEventThinkingDelta) GetType() string {
	if e.Type == "" {
		e.Type = ASSISTANT_MESSAGE_EVENT_THINKING_DELTA
	}
	return e.Type 
}

// AssistantMessageEventThinkingEnd 思考结束事件
type AssistantMessageEventThinkingEnd struct {
	Type         string            `json:"type"`
	ContentIndex int               `json:"contentIndex"`
	Content      string            `json:"content"`
	Partial      *AssistantMessage `json:"partial"`
}

func (e *AssistantMessageEventThinkingEnd) GetType() string {
	if e.Type == "" {
		e.Type = ASSISTANT_MESSAGE_EVENT_THINKING_END
	}
	return e.Type 
}

// AssistantMessageEventToolCallStart 工具调用开始事件
type AssistantMessageEventToolCallStart struct {
	Type         string            `json:"type"`
	ContentIndex int               `json:"contentIndex"`
	Partial      *AssistantMessage `json:"partial"`
}

func (e *AssistantMessageEventToolCallStart) GetType() string {
	if e.Type == "" {
		e.Type = ASSISTANT_MESSAGE_EVENT_TOOLCALL_START
	}
	return e.Type 
}

// AssistantMessageEventToolCallDelta 工具调用增量事件
type AssistantMessageEventToolCallDelta struct {
	Type         string            `json:"type"`
	ContentIndex int               `json:"contentIndex"`
	Delta        string            `json:"delta"`
	Partial      *AssistantMessage `json:"partial"`
}

func (e *AssistantMessageEventToolCallDelta) GetType() string {
	if e.Type == "" {
		e.Type = ASSISTANT_MESSAGE_EVENT_TOOLCALL_DELTA
	}
	return e.Type 
}

// AssistantMessageEventToolCallEnd 工具调用结束事件
type AssistantMessageEventToolCallEnd struct {
	Type         string            `json:"type"`
	ContentIndex int               `json:"contentIndex"`
	ToolCall     *ToolCall          `json:"toolCall"`
	Partial      *AssistantMessage `json:"partial"`
}

func (e *AssistantMessageEventToolCallEnd) GetType() string {
	if e.Type == "" {
		e.Type = ASSISTANT_MESSAGE_EVENT_TOOLCALL_END
	}
	return e.Type 
}

// AssistantMessageEventDone 完成事件
type AssistantMessageEventDone struct {
	Type    string            `json:"type"`
	Reason  StopReason        `json:"reason"`
	Message *AssistantMessage `json:"message"`
}

func (e *AssistantMessageEventDone) GetType() string { 
	if e.Type == "" {
		e.Type = ASSISTANT_MESSAGE_EVENT_DONE
	}
	return e.Type 
}

// AssistantMessageEventError 错误事件
type AssistantMessageEventError struct {
	Type   string            `json:"type"`
	Reason StopReason        `json:"reason"`
	Error  *AssistantMessage `json:"error"`
}

func (e *AssistantMessageEventError) GetType() string {
	if e.Type == "" {
		e.Type = ASSISTANT_MESSAGE_EVENT_ERROR
	}
	return e.Type 
}