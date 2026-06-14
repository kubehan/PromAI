package session

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jay-y/pi/pkg/ai"
)

// 错误定义
var (
	ErrPromptFailed = &PromptError{Message: "Failed to send prompt"}
)

// PromptError 提示错误
type PromptError struct {
	Message string
}

func (e *PromptError) Error() string {
	return e.Message
}

// PromptOptions 提示选项
type PromptOptions struct {
	ExpandPromptTemplates bool           `json:"expandPromptTemplates"`
	Images                []ai.ImageContentBlock `json:"images,omitempty"`
	StreamingBehavior     string         `json:"streamingBehavior,omitempty"` // "steer" | "followUp"
	Source                string         `json:"source,omitempty"`
}

// SendMessageOptions 发送消息选项
type SendMessageOptions struct {
	TriggerTurn bool   `json:"triggerTurn,omitempty"`
	DeliverAs   string `json:"deliverAs,omitempty"` // "steer", "followUp", "nextTurn"
}

// Prompt 发送提示到代理
func (s *AgentSession) Prompt(ctx context.Context, text string, options *PromptOptions) error {
	if options == nil {
		options = &PromptOptions{}
	}

	expandPromptTemplates := options.ExpandPromptTemplates
	if !expandPromptTemplates {
		expandPromptTemplates = true
	}

	// 如果正在流式传输，根据 streamingBehavior 排队
	if s.agent.GetState().IsStreaming {
		if options.StreamingBehavior == "" {
			return errors.New("Agent is already processing. Specify streamingBehavior ('steer' or 'followUp') to queue the message")
		}

		if options.StreamingBehavior == "followUp" {
			return s.queueFollowUp(ctx, text, options.Images)
		}
		return s.queueSteer(ctx, text, options.Images)
	}

	// 刷新待处理的 bash 消息
	s.flushPendingBashMessages()

	// 验证模型
	model := s.agent.GetState().Model
	if model == nil {
		return ErrNoModelSelected
	}

	// 验证 API Key
	apiKey, err := s.modelRegistry.GetApiKey(model)
	if err != nil {
		return err
	}
	if apiKey == "" && model.GetProvider() != "ollama" {
		return fmt.Errorf("No API key found for %s", model.GetProvider())
	}

	// 检查是否需要压缩
	lastAssistant := s.findLastAssistantMessage()
	if lastAssistant != nil {
		s.checkCompaction(lastAssistant, false)
	}

	// 构建消息数组
	var messages []ai.Message

	// 添加用户消息
	userContent := []ai.ContentBlock{ai.NewTextContentBlock(text)}
	for _, img := range options.Images {
		userContent = append(userContent, &img)
	}

	userMessage := &ai.UserMessage{
		Role:      "user",
		Content:   userContent,
		Timestamp: time.Now().UnixMilli(),
	}
	messages = append(messages, userMessage)

	// 注入待处理的 "nextTurn" 消息
	s.messagesMu.RLock()
	for _, msg := range s.pendingNextTurnMessages {
		messages = append(messages, &msg)
	}
	s.messagesMu.RUnlock()

	// 清空待处理消息
	s.messagesMu.Lock()
	s.pendingNextTurnMessages = make([]CustomMessage, 0)
	s.messagesMu.Unlock()

	// 发送提示
	err = s.agent.Prompt(ctx, messages)
	if err != nil {
		return err
	}

	// 等待重试完成
	return s.WaitForRetry(ctx)
}

// Steer 发送 steering 消息中断代理
func (s *AgentSession) Steer(ctx context.Context, text string, images []ai.ImageContentBlock) error {
	// 扩展技能命令和提示模板
	expandedText := s.expandSkillCommand(text)

	return s.queueSteer(ctx, expandedText, images)
}

// FollowUp 发送 follow-up 消息
func (s *AgentSession) FollowUp(ctx context.Context, text string, images []ai.ImageContentBlock) error {
	// 扩展技能命令和提示模板
	expandedText := s.expandSkillCommand(text)

	return s.queueFollowUp(ctx, expandedText, images)
}

// queueSteer 内部：排队 steering 消息
func (s *AgentSession) queueSteer(ctx context.Context, text string, images []ai.ImageContentBlock) error {
	s.messagesMu.Lock()
	s.steeringMessages = append(s.steeringMessages, text)
	s.messagesMu.Unlock()

	content := []ai.ContentBlock{ai.NewTextContentBlock(text)}
	for _, img := range images {
		content = append(content, &img)
	}

	message := &ai.UserMessage{
		Role:      "user",
		Content:   content,
		Timestamp: time.Now().UnixMilli(),
	}

	return s.agent.Steer(message)
}

// queueFollowUp 内部：排队 follow-up 消息
func (s *AgentSession) queueFollowUp(ctx context.Context, text string, images []ai.ImageContentBlock) error {
	s.messagesMu.Lock()
	s.followUpMessages = append(s.followUpMessages, text)
	s.messagesMu.Unlock()

	content := []ai.ContentBlock{ai.NewTextContentBlock(text)}
	for _, img := range images {
		content = append(content, &img)
	}

	message := &ai.UserMessage{
		Role:      "user",
		Content:   content,
		Timestamp: time.Now().UnixMilli(),
	}

	return s.agent.FollowUp(message)
}

// SendCustomMessage 发送自定义消息
func (s *AgentSession) SendCustomMessage(ctx context.Context, message CustomMessage, options *SendMessageOptions) error {
	if options == nil {
		options = &SendMessageOptions{}
	}

	message.Timestamp = time.Now().UnixMilli()

	if options.DeliverAs == "nextTurn" {
		s.messagesMu.Lock()
		s.pendingNextTurnMessages = append(s.pendingNextTurnMessages, message)
		s.messagesMu.Unlock()
		return nil
	}

	if s.agent.GetState().IsStreaming {
		// 当流式传输时，需要将 CustomMessage 转换为 Message
		// 这里简化处理，直接返回错误
		return errors.New("cannot send custom message while streaming")
	}

	if options.TriggerTurn {
		return s.agent.Prompt(ctx, []ai.Message{&message})
	}

	s.agent.AppendMessage(&message)
	s.sessionManager.AppendCustomMessageEntryWithAnyDisplay(
		message.CustomType,
		message.Content,
		message.Display,
		message.Details,
	)

	return nil
}

// SendUserMessage 发送用户消息
func (s *AgentSession) SendUserMessage(ctx context.Context, content any, options *SendMessageOptions) error {
	var text string
	var images []ai.ImageContentBlock

	switch c := content.(type) {
	case string:
		text = c
	case []ai.ContentBlock:
		for _, block := range c {
			if tc, ok := block.(*ai.TextContentBlock); ok {
				text += tc.Text
			} else if ic, ok := block.(*ai.ImageContentBlock); ok {
				images = append(images, *ic)
			}
		}
	}

	return s.Prompt(ctx, text, &PromptOptions{
		ExpandPromptTemplates: false,
		StreamingBehavior:     options.DeliverAs,
		Images:                images,
		Source:                "extension",
	})
}

// expandSkillCommand 扩展技能命令
func (s *AgentSession) expandSkillCommand(text string) string {
	// TODO: 实现技能命令扩展
	return text
}

// findLastAssistantMessage 查找最后的助手消息
func (s *AgentSession) findLastAssistantMessage() *ai.AssistantMessage {
	messages := s.agent.GetState().Messages
	for i := len(messages) - 1; i >= 0; i-- {
		if am, ok := messages[i].(*ai.AssistantMessage); ok {
			return am
		}
	}
	return nil
}

// flushPendingBashMessages 刷新待处理的 bash 消息
func (s *AgentSession) flushPendingBashMessages() {
	s.messagesMu.Lock()
	defer s.messagesMu.Unlock()

	if len(s.pendingBashMessages) == 0 {
		return
	}

	for _, bashMessage := range s.pendingBashMessages {
		s.agent.AppendMessage(&bashMessage)
		s.sessionManager.AppendMessage(&bashMessage)
	}

	s.pendingBashMessages = make([]BashExecutionMessage, 0)
}