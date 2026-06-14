package ai

// ToolCall 工具调用结构体
type ToolCall struct {
	Type             string         `json:"type"`
	ID               string         `json:"id"`
	Name             string         `json:"name"`
	Arguments        any 			`json:"arguments"`
	ThoughtSignature *string        `json:"thoughtSignature,omitempty"`
}

func (tc *ToolCall) GetType() string {
	return tc.Type
}

func (tc *ToolCall) GetID() string {
	return tc.ID
}

func (tc *ToolCall) GetName() string {
	return tc.Name
}

func (tc *ToolCall) GetArguments() any {
	return tc.Arguments
}

func (tc *ToolCall) GetThoughtSignature() *string {
	return tc.ThoughtSignature
}

// NewToolCall 创建新的工具调用
func NewToolCall(id, name string, arguments map[string]any) *ToolCall {
	return &ToolCall{
		Type:      "toolCall",
		ID:        id,
		Name:      name,
		Arguments: &arguments,
	}
}