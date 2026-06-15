package ai

// ContentBlock 内容块接口
type ContentBlock interface {
	GetType() string
}

// TextContent 文本内容块
type TextContentBlock struct {
	Type           string `json:"type"`
	Text           string `json:"text"`
	TextSignature  string `json:"textSignature,omitempty"`
}

func (t *TextContentBlock) GetType() string {
	return t.Type
}

// NewTextContent 创建新的文本内容块
func NewTextContentBlock(text string) *TextContentBlock {
	return &TextContentBlock{
		Type: "text",
		Text: text,
	}
}

// ThinkingContent 思考内容块
type ThinkingContentBlock struct {
	Type              string `json:"type"`
	Thinking          string `json:"thinking"`
	ThinkingSignature string `json:"thinkingSignature,omitempty"`
}

func (t *ThinkingContentBlock) GetType() string {
	return t.Type
}

// NewThinkingContent 创建新的思考内容块
func NewThinkingContentBlock(thinking string) *ThinkingContentBlock {
	return &ThinkingContentBlock{
		Type:     "thinking",
		Thinking: thinking,
	}
}

// ImageContent 图片内容块
type ImageContentBlock struct {
	Type     string `json:"type"`
	Data     string `json:"data"` // base64 编码的图片数据
	MimeType string `json:"mimeType"` // 例如 "image/jpeg", "image/png"
}

func (i *ImageContentBlock) GetType() string {
	return i.Type
}

// NewImageContent 创建新的图片内容块
func NewImageContentBlock(data, mimeType string) *ImageContentBlock {
	return &ImageContentBlock{
		Type:     "image",
		Data:     data,
		MimeType: mimeType,
	}
}

// Message 消息接口
type Message interface {
	GetRole() string
}

// UserMessage 用户消息
type UserMessage struct {
	Role      string         `json:"role"`
	Content   any            `json:"content"` // string 或 []ContentBlock
	Timestamp int64         `json:"timestamp"` // Unix 毫秒时间戳
}

func (u *UserMessage) GetRole() string {
	return u.Role
}

// NewUserMessage 创建新的用户消息
func NewUserMessage(content any) *UserMessage {
	return &UserMessage{
		Role:      "user",
		Content:   content,
		Timestamp: getCurrentTimestamp(),
	}
}

// AssistantMessage 助手消息
type AssistantMessage struct {
	Role         string                `json:"role"`
	Content      []ContentBlock        `json:"content"`
	API          ModelApi              `json:"api"`
	Provider     ModelProvider         `json:"provider"`
	Model        string                `json:"model"`
	Usage        Usage                 `json:"usage"`
	StopReason   StopReason            `json:"stopReason"`
	ErrorMessage string                `json:"errorMessage,omitempty"`
	Timestamp    int64                 `json:"timestamp"` // Unix 毫秒时间戳
}

func (a *AssistantMessage) GetRole() string {
	return a.Role
}

// NewAssistantMessage 创建新的助手消息
func NewAssistantMessage(api ModelApi, provider ModelProvider, model string) *AssistantMessage {
	return &AssistantMessage{
		Role:       "assistant",
		Content:    []ContentBlock{},
		API:        api,
		Provider:   provider,
		Model:      model,
		StopReason: StopReasonStop,
		Timestamp:  getCurrentTimestamp(),
	}
}

// ToolResultMessage 工具结果消息
type ToolResultMessage struct {
	Role        string                `json:"role"`
	ToolCallID  string                `json:"toolCallId"`
	ToolName    string                `json:"toolName"`
	Content     []ContentBlock        `json:"content"` // 支持文本和图片
	Details     any           		  `json:"details,omitempty"`
	IsError     bool                  `json:"isError"`
	Timestamp   int64                 `json:"timestamp"` // Unix 毫秒时间戳
}

func (t *ToolResultMessage) GetRole() string {
	return t.Role
}

// NewToolResultMessage 创建新的工具结果消息
func NewToolResultMessage(toolCallID, toolName string, content []ContentBlock, isError bool) *ToolResultMessage {
	return &ToolResultMessage{
		Role:       "toolResult",
		ToolCallID: toolCallID,
		ToolName:   toolName,
		Content:    content,
		IsError:    isError,
		Timestamp:  getCurrentTimestamp(),
	}
}