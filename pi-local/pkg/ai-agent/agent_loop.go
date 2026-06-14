package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jay-y/pi/pkg/ai"
)

// AgentLoopConfig 代理循环的配置
type AgentLoopConfig struct {
	Model           ai.Model `json:"model"`
	Reasoning       ai.ThinkingLevel `json:"reasoning"`
	SessionID       string        `json:"sessionID"`
	Transport       ai.Transport  `json:"transport"`
	ThinkingBudgets *ai.ThinkingBudgets `json:"thinkingBudgets"`
	MaxRetryDelayMs int           `json:"maxRetryDelayMs"`

	ConvertToLLM     func(messages []ai.Message) ([]ai.Message, error)
	TransformContext func(messages []ai.Message, ctx context.Context) ([]ai.Message, error)

	GetApiKey           func(provider string) (string, error)
	GetSteeringMessages func() ([]ai.Message, error)
	GetFollowUpMessages func() ([]ai.Message, error)
}

// AgentEventStream 代理事件流
type AgentEventStream struct {
	*ai.EventStream[AgentEvent, []ai.Message]
}

// NewAgentEventStream 创建新的代理事件流
func NewAgentEventStream() *AgentEventStream {
	isComplete := func(event AgentEvent) bool {
		_, ok := event.(*AgentEventEnd)
		return ok
	}

	extractResult := func(event AgentEvent) []ai.Message {
		if e, ok := event.(*AgentEventEnd); ok {
			return e.Messages
		}
		return []ai.Message{}
	}

	return &AgentEventStream{
		EventStream: ai.NewEventStream(isComplete, extractResult),
	}
}

// AgentLoop 启动代理循环
func AgentLoop(prompts []ai.Message, context AgentContext, config AgentLoopConfig, ctx context.Context, streamFn StreamFn) *AgentEventStream {
	stream := NewAgentEventStream()

	go func() {
		newMessages := make([]ai.Message, len(prompts))
		copy(newMessages, prompts)
		currentContext := AgentContext{
			SystemPrompt: context.SystemPrompt,
			Messages:     append(append([]ai.Message{}, context.Messages...), prompts...),
			Tools:        context.Tools,
		}

		stream.Push(&AgentEventStart{Type: AGENT_MESSAGE_EVENT_AGENT_START})
		stream.Push(&AgentEventTurnStart{Type: AGENT_MESSAGE_EVENT_TURN_START})
		for _, prompt := range prompts {
			stream.Push(&AgentEventMessageStart{Type: AGENT_MESSAGE_EVENT_MESSAGE_START, Message: prompt})
			stream.Push(&AgentEventMessageEnd{Type: AGENT_MESSAGE_EVENT_MESSAGE_END, Message: prompt})
		}

		runLoop(currentContext, newMessages, config, ctx, stream, streamFn)
	}()

	return stream
}

// AgentLoopContinue 继续代理循环
func AgentLoopContinue(context AgentContext, config AgentLoopConfig, ctx context.Context, streamFn StreamFn) *AgentEventStream {
	if len(context.Messages) == 0 {
		panic("Cannot continue: no messages in context")
	}

	lastMsg := context.Messages[len(context.Messages)-1]
	if _, ok := lastMsg.(*ai.AssistantMessage); ok {
		panic("Cannot continue from message role: assistant")
	}
	// if am, ok := lastMsg.(*ai.AssistantMessage); ok {
	// 	// 如果最后一条消息是错误状态，允许继续（用于重试机制）
	// 	if am.StopReason != "error" {
	// 		panic("Cannot continue from message role: assistant")
	// 	}
	// }

	stream := NewAgentEventStream()

	go func() {
		newMessages := []ai.Message{}
		currentContext := AgentContext{
			SystemPrompt: context.SystemPrompt,
			Messages:     append([]ai.Message{}, context.Messages...),
			Tools:        context.Tools,
		}

		stream.Push(&AgentEventStart{Type: AGENT_MESSAGE_EVENT_AGENT_START})
		stream.Push(&AgentEventTurnStart{Type: AGENT_MESSAGE_EVENT_TURN_START})

		runLoop(currentContext, newMessages, config, ctx, stream, streamFn)
	}()

	return stream
}

// runLoop 执行主要的循环逻辑
func runLoop(
	currentContext AgentContext,
	newMessages []ai.Message,
	config AgentLoopConfig,
	ctx context.Context,
	stream *AgentEventStream,
	streamFn StreamFn,
) {
	firstTurn := true

	var pendingMessages []ai.Message
	if config.GetSteeringMessages != nil {
		msgs, _ := config.GetSteeringMessages()
		pendingMessages = msgs
	}

	for {
		hasMoreToolCalls := true
		var steeringAfterTools []ai.Message

		for hasMoreToolCalls || len(pendingMessages) > 0 {
			if !firstTurn {
				stream.Push(&AgentEventTurnStart{Type: AGENT_MESSAGE_EVENT_TURN_START})
			} else {
				firstTurn = false
			}

			if len(pendingMessages) > 0 {
				for _, msg := range pendingMessages {
					stream.Push(&AgentEventMessageStart{Type: AGENT_MESSAGE_EVENT_MESSAGE_START, Message: msg})
					stream.Push(&AgentEventMessageEnd{Type: AGENT_MESSAGE_EVENT_MESSAGE_END, Message: msg})
					currentContext.Messages = append(currentContext.Messages, msg)
					newMessages = append(newMessages, msg)
				}
				pendingMessages = []ai.Message{}
			}

			message, err := streamAssistantResponse(currentContext, config, ctx, stream, streamFn)
			if err != nil {
				stream.Push(&AgentEventEnd{Type: AGENT_MESSAGE_EVENT_AGENT_END, Messages: newMessages})
				stream.End(newMessages)
				return
			}

			newMessages = append(newMessages, message)

			assistantMsg, ok := message.(*ai.AssistantMessage)
			if !ok {
				stream.Push(&AgentEventEnd{Type: AGENT_MESSAGE_EVENT_AGENT_END, Messages: newMessages})
				stream.End(newMessages)
				return
			}

			if assistantMsg.StopReason == ai.StopReasonError || assistantMsg.StopReason == ai.StopReasonAborted {
				stream.Push(&AgentEventTurnEnd{
					Type:        AGENT_MESSAGE_EVENT_TURN_END,
					Message:     message,
					ToolResults: []ai.ToolResultMessage{},
				})
				stream.Push(&AgentEventEnd{Type: AGENT_MESSAGE_EVENT_AGENT_END, Messages: newMessages})
				stream.End(newMessages)
				return
			}

			toolCalls := []*ai.ToolCall{}
			for _, c := range assistantMsg.Content {
				if tc, ok := c.(*ai.ToolCall); ok {
					toolCalls = append(toolCalls, tc)
				}
			}
			hasMoreToolCalls = len(toolCalls) > 0

			toolResults := []ai.ToolResultMessage{}
			if hasMoreToolCalls {
				result, err := executeToolCalls(ctx, currentContext.Tools, assistantMsg, stream, config.GetSteeringMessages)
				if err == nil {
					toolResults = append(toolResults, result.ToolResults...)
					steeringAfterTools = result.SteeringMessages
				}

				currentContext.Messages = append(currentContext.Messages, assistantMsg)
				newMessages = append(newMessages, assistantMsg)

				for _, resultMsg := range toolResults {
					msgCopy := resultMsg
					currentContext.Messages = append(currentContext.Messages, &msgCopy)
					newMessages = append(newMessages, &msgCopy)
				}
			}

			stream.Push(&AgentEventTurnEnd{
				Type:        AGENT_MESSAGE_EVENT_TURN_END,
				Message:     message,
				ToolResults: toolResults,
			})

			if len(steeringAfterTools) > 0 {
				pendingMessages = steeringAfterTools
				steeringAfterTools = nil
			} else if config.GetSteeringMessages != nil {
				msgs, _ := config.GetSteeringMessages()
				pendingMessages = msgs
			}
		}

		var followUpMessages []ai.Message
		if config.GetFollowUpMessages != nil {
			msgs, _ := config.GetFollowUpMessages()
			followUpMessages = msgs
		}

		if len(followUpMessages) > 0 {
			pendingMessages = followUpMessages
			continue
		}

		break
	}

	stream.Push(&AgentEventEnd{Type: AGENT_MESSAGE_EVENT_AGENT_END, Messages: newMessages})
	stream.End(newMessages)
}

// streamAssistantResponse 流式获取助手响应
func streamAssistantResponse(
	context AgentContext,
	config AgentLoopConfig,
	ctx context.Context,
	stream *AgentEventStream,
	streamFn StreamFn,
) (ai.Message, error) {
	var messages []ai.Message
	if config.TransformContext != nil {
		var err error
		messages, err = config.TransformContext(context.Messages, ctx)
		if err != nil {
			return nil, err
		}
	} else {
		messages = context.Messages
	}

	llmMessages, err := config.ConvertToLLM(messages)
	if err != nil {
		return nil, err
	}

	llmContext := ai.Context{
		SystemPrompt: context.SystemPrompt,
		Messages:     llmMessages,
		Tools:        convertAgentToolsToAITools(context.Tools),
	}

	var sfn StreamFn
	if streamFn != nil {
		sfn = streamFn
	} else {
		sfn = ai.StreamSimple
	}

	apiKey := ""
	if config.GetApiKey != nil {
		apiKey, _ = config.GetApiKey(string(config.Model.GetProvider()))
	}

	opts := &ai.SimpleStreamOptions{
		StreamOptions: ai.StreamOptions{
			Ctx:             ctx,
			APIKey:          apiKey,
			Transport:       config.Transport,
			SessionID:       config.SessionID,
			MaxRetryDelayMs: config.MaxRetryDelayMs,
		},
		Reasoning:       config.Reasoning,
		ThinkingBudgets: config.ThinkingBudgets,
	}

	resp, err := sfn(config.Model, llmContext, opts)
	if err != nil {
		return nil, err
	}

	var partialMessage *ai.AssistantMessage
	addedPartial := false

	for event := range resp.Events() {
		switch e := event.(type) {
		case *ai.AssistantMessageEventStart:
			partialMessage = e.Partial
			context.Messages = append(context.Messages, partialMessage)
			addedPartial = true
			stream.Push(&AgentEventMessageStart{Type: AGENT_MESSAGE_EVENT_MESSAGE_START, Message: partialMessage})

		case *ai.AssistantMessageEventTextStart, *ai.AssistantMessageEventTextDelta, *ai.AssistantMessageEventTextEnd,
			*ai.AssistantMessageEventThinkingStart, *ai.AssistantMessageEventThinkingDelta, *ai.AssistantMessageEventThinkingEnd,
			*ai.AssistantMessageEventToolCallStart, *ai.AssistantMessageEventToolCallDelta, *ai.AssistantMessageEventToolCallEnd:
			if partialMessage != nil {
				switch ev := event.(type) {
				case *ai.AssistantMessageEventTextStart:
					partialMessage = ev.Partial
				case *ai.AssistantMessageEventTextDelta:
					partialMessage = ev.Partial
				case *ai.AssistantMessageEventTextEnd:
					partialMessage = ev.Partial
				case *ai.AssistantMessageEventThinkingStart:
					partialMessage = ev.Partial
				case *ai.AssistantMessageEventThinkingDelta:
					partialMessage = ev.Partial
				case *ai.AssistantMessageEventThinkingEnd:
					partialMessage = ev.Partial
				case *ai.AssistantMessageEventToolCallStart:
					partialMessage = ev.Partial
				case *ai.AssistantMessageEventToolCallDelta:
					partialMessage = ev.Partial
				case *ai.AssistantMessageEventToolCallEnd:
					partialMessage = ev.Partial
				}

				if len(context.Messages) > 0 {
					context.Messages[len(context.Messages)-1] = partialMessage
				}

				stream.Push(&AgentEventMessageUpdate{
					Type:                  AGENT_MESSAGE_EVENT_MESSAGE_UPDATE,
					Message:               partialMessage,
					AssistantMessageEvent: event,
				})
			}

		case *ai.AssistantMessageEventDone, *ai.AssistantMessageEventError:
			result := resp.Result()
			if addedPartial && len(context.Messages) > 0 {
				context.Messages[len(context.Messages)-1] = result
			} else {
				context.Messages = append(context.Messages, result)
			}

			if !addedPartial {
				stream.Push(&AgentEventMessageStart{Type: AGENT_MESSAGE_EVENT_MESSAGE_START, Message: result})
			}
			stream.Push(&AgentEventMessageEnd{Type: AGENT_MESSAGE_EVENT_MESSAGE_END, Message: result})
			return result, nil
		}
	}

	return resp.Result(), nil
}

// toolExecutionResult 工具执行结果
type toolExecutionResult struct {
	ToolResults      []ai.ToolResultMessage
	SteeringMessages []ai.Message
}

// executeToolCalls 执行工具调用
func executeToolCalls(
	ctx context.Context,
	tools []AgentTool,
	assistantMessage *ai.AssistantMessage,
	stream *AgentEventStream,
	getSteeringMessages func() ([]ai.Message, error),
) (*toolExecutionResult, error) {
	toolCalls := []*ai.ToolCall{}
	for _, c := range assistantMessage.Content {
		if tc, ok := c.(*ai.ToolCall); ok {
			toolCalls = append(toolCalls, tc)
		}
	}

	results := []ai.ToolResultMessage{}
	var steeringMessages []ai.Message

	for i, toolCall := range toolCalls {
		var tool *AgentTool
		for _, t := range tools {
			if t.Name == toolCall.Name {
				tool = &t
				break
			}
		}

		stream.Push(&AgentEventToolExecutionStart{
			Type:       AGENT_MESSAGE_EVENT_TOOL_EXECUTION_START,
			ToolCallID: toolCall.ID,
			ToolName:   toolCall.Name,
			Args:       toolCall.Arguments,
		})

		var result AgentToolResult
		isError := false

		if tool == nil {
			result = AgentToolResult{
				Content: []ai.ContentBlock{ai.NewTextContentBlock(fmt.Sprintf("Tool %s not found", toolCall.Name))},
				Details: map[string]any{},
			}
			isError = true
		} else {
			validatedArgs, err := validateToolArguments(tool, toolCall)
			if err != nil {
				result = AgentToolResult{
					Content: []ai.ContentBlock{ai.NewTextContentBlock(err.Error())},
					Details: map[string]any{},
				}
				isError = true
			} else {
				r, err := tool.Execute(ctx, validatedArgs,  func(partialResult *AgentToolResult) {
					stream.Push(&AgentEventToolExecutionUpdate{
						Type:          AGENT_MESSAGE_EVENT_TOOL_EXECUTION_UPDATE,
						ToolCallID:    toolCall.ID,
						ToolName:      toolCall.Name,
						Args:          toolCall.Arguments,
						PartialResult: partialResult,
					})
				})
				if err != nil {
					result = AgentToolResult{
						Content: []ai.ContentBlock{ai.NewTextContentBlock(err.Error())},
						Details: map[string]any{},
					}
					isError = true
				} else {
					result = *r
				}
			}
		}

		stream.Push(&AgentEventToolExecutionEnd{
			Type:       AGENT_MESSAGE_EVENT_TOOL_EXECUTION_END,
			ToolName:   toolCall.Name,
			Result:     result,
			IsError:    isError,
		})

		toolResultMessage := ai.NewToolResultMessage(
			toolCall.ID,
			toolCall.Name,
			result.Content,
			isError,
		)
		if result.Details != nil {
			toolResultMessage.Details = result.Details
		}

		results = append(results, *toolResultMessage)
		stream.Push(&AgentEventMessageStart{Type: AGENT_MESSAGE_EVENT_MESSAGE_START, Message: toolResultMessage})
		stream.Push(&AgentEventMessageEnd{Type: AGENT_MESSAGE_EVENT_MESSAGE_END, Message: toolResultMessage})

		if getSteeringMessages != nil {
			msgs, _ := getSteeringMessages()
			if len(msgs) > 0 {
				steeringMessages = msgs
				remainingCalls := toolCalls[i+1:]
				for _, skipped := range remainingCalls {
					results = append(results, *skipToolCall(skipped, stream))
				}
				break
			}
		}
	}

	return &toolExecutionResult{
		ToolResults:      results,
		SteeringMessages: steeringMessages,
	}, nil
}

// skipToolCall 跳过工具调用
func skipToolCall(toolCall *ai.ToolCall, stream *AgentEventStream) *ai.ToolResultMessage {
	result := AgentToolResult{
		Content: []ai.ContentBlock{ai.NewTextContentBlock("Skipped due to queued user message.")},
		Details: map[string]any{},
	}

	stream.Push(&AgentEventToolExecutionStart{
		Type:       AGENT_MESSAGE_EVENT_TOOL_EXECUTION_START,
		ToolCallID: toolCall.ID,
		ToolName:   toolCall.Name,
		Args:       toolCall.Arguments,
	})

	stream.Push(&AgentEventToolExecutionEnd{
		Type:       AGENT_MESSAGE_EVENT_TOOL_EXECUTION_END,
		ToolName:   toolCall.Name,
		Result:     result,
		IsError:    true,
	})

	toolResultMessage := ai.NewToolResultMessage(
		toolCall.ID,
		toolCall.Name,
		result.Content,
		true,
	)

	stream.Push(&AgentEventMessageStart{Type: AGENT_MESSAGE_EVENT_MESSAGE_START, Message: toolResultMessage})
	stream.Push(&AgentEventMessageEnd{Type: AGENT_MESSAGE_EVENT_MESSAGE_END, Message: toolResultMessage})

	return toolResultMessage
}

// validateToolArguments 验证工具参数
func validateToolArguments(tool *AgentTool, toolCall *ai.ToolCall) (map[string]any, error) {
	return toolCall.Arguments.(map[string]any), nil
}

// convertAgentToolsToAITools 将 AgentTool 转换为 ai.Tool
func convertAgentToolsToAITools(agentTools []AgentTool) []ai.Tool {
	result := make([]ai.Tool, len(agentTools))
	for i, t := range agentTools {
		result[i] = ai.Tool{
			Name:        t.Name,
			Description: t.Description,
			Parameters:  t.Parameters,
		}
	}
	return result
}

// parseStreamingJson 解析流式 JSON
func parseStreamingJson(s string) map[string]any {
	var result map[string]any
	if err := json.Unmarshal([]byte(s), &result); err == nil {
		return result
	}
	return map[string]any{}
}

// getCurrentTimestamp 获取当前时间戳
func getCurrentTimestamp() int64 {
	return time.Now().UnixMilli()
}
