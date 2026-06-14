package agent

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/jay-y/pi/pkg/ai"
)

// ProxyAssistantMessageEvent 代理服务返回的助手消息事件
type ProxyAssistantMessageEvent struct {
	Type             ProxyAssistantMessageEventType `json:"type"`
	ContentIndex     int                            `json:"contentIndex,omitempty"`
	Delta            string                         `json:"delta,omitempty"`
	ContentSignature string                         `json:"contentSignature,omitempty"`
	ID               string                         `json:"id,omitempty"`
	ToolName         string                         `json:"toolName,omitempty"`
	Reason           ai.StopReason                  `json:"reason,omitempty"`
	Usage            *ai.Usage                      `json:"usage,omitempty"`
	ErrorMessage     string                         `json:"errorMessage,omitempty"`
}

// ProxyAssistantMessageEventType 代理助手消息事件类型
type ProxyAssistantMessageEventType string

const (
	ProxyAssistantMessageEventStart         ProxyAssistantMessageEventType = "start"
	ProxyAssistantMessageEventTextStart     ProxyAssistantMessageEventType = "text_start"
	ProxyAssistantMessageEventTextDelta     ProxyAssistantMessageEventType = "text_delta"
	ProxyAssistantMessageEventTextEnd       ProxyAssistantMessageEventType = "text_end"
	ProxyAssistantMessageEventThinkingStart ProxyAssistantMessageEventType = "thinking_start"
	ProxyAssistantMessageEventThinkingDelta ProxyAssistantMessageEventType = "thinking_delta"
	ProxyAssistantMessageEventThinkingEnd   ProxyAssistantMessageEventType = "thinking_end"
	ProxyAssistantMessageEventToolCallStart ProxyAssistantMessageEventType = "toolcall_start"
	ProxyAssistantMessageEventToolCallDelta ProxyAssistantMessageEventType = "toolcall_delta"
	ProxyAssistantMessageEventToolCallEnd   ProxyAssistantMessageEventType = "toolcall_end"
	ProxyAssistantMessageEventDone          ProxyAssistantMessageEventType = "done"
	ProxyAssistantMessageEventError         ProxyAssistantMessageEventType = "error"
)

// ProxyStreamOptions 代理流式调用选项
type ProxyStreamOptions struct {
	ai.SimpleStreamOptions
	AuthToken string
	ProxyURL  string
}

// StreamProxy 通过代理服务器进行 LLM 流式调用
func StreamProxy(model ai.Model, ctx ai.Context, opts ProxyStreamOptions) *ai.AssistantMessageEventStream {
	stream := ai.NewAssistantMessageEventStream()

	go func() {
		partial := &ai.AssistantMessage{
			Role:       "assistant",
			StopReason: ai.StopReasonStop,
			Content:    []ai.ContentBlock{},
			API:        model.GetAPI(),
			Provider:   model.GetProvider(),
			Model:      model.GetID(),
			Usage:      ai.Usage{},
			Timestamp:  time.Now().UnixMilli(),
		}

		var reader io.ReadCloser

		reqBody, err := json.Marshal(map[string]any{
			"model":   model,
			"context": ctx,
			"options": map[string]any{
				"temperature": opts.Temperature,
				"maxTokens":   opts.MaxTokens,
				"reasoning":   opts.Reasoning,
			},
		})
		if err != nil {
			pushProxyError(stream, partial, err.Error(), ai.StopReasonError)
			return
		}

		httpReq, err := http.NewRequestWithContext(opts.Ctx, "POST", fmt.Sprintf("%s/api/stream", opts.ProxyURL), bytes.NewBuffer(reqBody))
		if err != nil {
			pushProxyError(stream, partial, err.Error(), ai.StopReasonError)
			return
		}

		httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", opts.AuthToken))
		httpReq.Header.Set("Content-Type", "application/json")

		client := &http.Client{}
		resp, err := client.Do(httpReq)
		if err != nil {
			pushProxyError(stream, partial, err.Error(), ai.StopReasonError)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			var errorMsg string
			errorMsg = fmt.Sprintf("Proxy error: %d %s", resp.StatusCode, resp.Status)
			var errorData map[string]any
			if jsonErr := json.NewDecoder(resp.Body).Decode(&errorData); jsonErr == nil {
				if errStr, ok := errorData["error"].(string); ok {
					errorMsg = fmt.Sprintf("Proxy error: %s", errStr)
				}
			}
			pushProxyError(stream, partial, errorMsg, ai.StopReasonError)
			return
		}

		reader = resp.Body
		scanner := bufio.NewScanner(reader)
		scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

		for scanner.Scan() {
			select {
			case <-opts.Ctx.Done():
				pushProxyError(stream, partial, opts.Ctx.Err().Error(), ai.StopReasonAborted)
				return
			default:
			}

			line := scanner.Text()
			if strings.HasPrefix(line, "data: ") {
				data := strings.TrimSpace(line[6:])
				if data != "" {
					var proxyEvent ProxyAssistantMessageEvent
					if err := json.Unmarshal([]byte(data), &proxyEvent); err != nil {
						continue
					}

					event := processProxyEvent(&proxyEvent, partial)
					if event != nil {
						stream.Push(event)
					}
				}
			}
		}

		if err := scanner.Err(); err != nil {
			pushProxyError(stream, partial, err.Error(), ai.StopReasonError)
			return
		}

		select {
		case <-opts.Ctx.Done():
			pushProxyError(stream, partial, opts.Ctx.Err().Error(), ai.StopReasonAborted)
		default:
			stream.End(partial)
		}
	}()

	return stream
}

// pushProxyError 推送代理错误事件
func pushProxyError(stream *ai.AssistantMessageEventStream, partial *ai.AssistantMessage, errorMsg string, reason ai.StopReason) {
	partial.StopReason = reason
	partial.ErrorMessage = errorMsg
	stream.Push(&ai.AssistantMessageEventError{
		Type:   "error",
		Reason: reason,
		Error:  partial,
	})
	stream.End(partial)
}

// processProxyEvent 处理代理事件并转换为内部事件格式
func processProxyEvent(proxyEvent *ProxyAssistantMessageEvent, partial *ai.AssistantMessage) ai.AssistantMessageEvent {
	switch proxyEvent.Type {
	case ProxyAssistantMessageEventStart:
		return &ai.AssistantMessageEventStart{
			Type:    "start",
			Partial: partial,
		}

	case ProxyAssistantMessageEventTextStart:
		for len(partial.Content) <= proxyEvent.ContentIndex {
			partial.Content = append(partial.Content, nil)
		}
		partial.Content[proxyEvent.ContentIndex] = &ai.TextContentBlock{
			Type: "text",
			Text: "",
		}
		return &ai.AssistantMessageEventTextStart{
			Type:         "text_start",
			ContentIndex: proxyEvent.ContentIndex,
			Partial:      partial,
		}

	case ProxyAssistantMessageEventTextDelta:
		if proxyEvent.ContentIndex < len(partial.Content) {
			if content, ok := partial.Content[proxyEvent.ContentIndex].(*ai.TextContentBlock); ok {
				content.Text += proxyEvent.Delta
				return &ai.AssistantMessageEventTextDelta{
					Type:         "text_delta",
					ContentIndex: proxyEvent.ContentIndex,
					Delta:        proxyEvent.Delta,
					Partial:      partial,
				}
			}
		}

	case ProxyAssistantMessageEventTextEnd:
		if proxyEvent.ContentIndex < len(partial.Content) {
			if content, ok := partial.Content[proxyEvent.ContentIndex].(*ai.TextContentBlock); ok {
				content.TextSignature = proxyEvent.ContentSignature
				return &ai.AssistantMessageEventTextEnd{
					Type:         "text_end",
					ContentIndex: proxyEvent.ContentIndex,
					Content:      content.Text,
					Partial:      partial,
				}
			}
		}

	case ProxyAssistantMessageEventThinkingStart:
		for len(partial.Content) <= proxyEvent.ContentIndex {
			partial.Content = append(partial.Content, nil)
		}
		partial.Content[proxyEvent.ContentIndex] = &ai.ThinkingContentBlock{
			Type:     "thinking",
			Thinking: "",
		}
		return &ai.AssistantMessageEventThinkingStart{
			Type:         "thinking_start",
			ContentIndex: proxyEvent.ContentIndex,
			Partial:      partial,
		}

	case ProxyAssistantMessageEventThinkingDelta:
		if proxyEvent.ContentIndex < len(partial.Content) {
			if content, ok := partial.Content[proxyEvent.ContentIndex].(*ai.ThinkingContentBlock); ok {
				content.Thinking += proxyEvent.Delta
				return &ai.AssistantMessageEventThinkingDelta{
					Type:         "thinking_delta",
					ContentIndex: proxyEvent.ContentIndex,
					Delta:        proxyEvent.Delta,
					Partial:      partial,
				}
			}
		}

	case ProxyAssistantMessageEventThinkingEnd:
		if proxyEvent.ContentIndex < len(partial.Content) {
			if content, ok := partial.Content[proxyEvent.ContentIndex].(*ai.ThinkingContentBlock); ok {
				content.ThinkingSignature = proxyEvent.ContentSignature
				return &ai.AssistantMessageEventThinkingEnd{
					Type:         "thinking_end",
					ContentIndex: proxyEvent.ContentIndex,
					Content:      content.Thinking,
					Partial:      partial,
				}
			}
		}

	case ProxyAssistantMessageEventToolCallStart:
		for len(partial.Content) <= proxyEvent.ContentIndex {
			partial.Content = append(partial.Content, nil)
		}
		partial.Content[proxyEvent.ContentIndex] = &ai.ToolCall{
			Type:      "toolCall",
			ID:        proxyEvent.ID,
			Name:      proxyEvent.ToolName,
			Arguments: &map[string]any{},
		}
		return &ai.AssistantMessageEventToolCallStart{
			Type:         "toolcall_start",
			ContentIndex: proxyEvent.ContentIndex,
			Partial:      partial,
		}

	case ProxyAssistantMessageEventToolCallDelta:
		if proxyEvent.ContentIndex < len(partial.Content) {
			if content, ok := partial.Content[proxyEvent.ContentIndex].(*ai.ToolCall); ok {
				// 在实际代码中，我们应该保存 partialJson
				// 这里我们只是简单地尝试解析
				content.Arguments = parseStreamingJson(fmt.Sprintf("%v", content.Arguments) + proxyEvent.Delta)
				return &ai.AssistantMessageEventToolCallDelta{
					Type:         "toolcall_delta",
					ContentIndex: proxyEvent.ContentIndex,
					Delta:        proxyEvent.Delta,
					Partial:      partial,
				}
			}
		}

	case ProxyAssistantMessageEventToolCallEnd:
		if proxyEvent.ContentIndex < len(partial.Content) {
			if content, ok := partial.Content[proxyEvent.ContentIndex].(*ai.ToolCall); ok {
				return &ai.AssistantMessageEventToolCallEnd{
					Type:         "toolcall_end",
					ContentIndex: proxyEvent.ContentIndex,
					ToolCall:     content,
					Partial:      partial,
				}
			}
		}

	case ProxyAssistantMessageEventDone:
		partial.StopReason = proxyEvent.Reason
		if proxyEvent.Usage != nil {
			partial.Usage = *proxyEvent.Usage
		}
		return &ai.AssistantMessageEventDone{
			Type:    "done",
			Reason:  proxyEvent.Reason,
			Message: partial,
		}

	case ProxyAssistantMessageEventError:
		partial.StopReason = proxyEvent.Reason
		partial.ErrorMessage = proxyEvent.ErrorMessage
		if proxyEvent.Usage != nil {
			partial.Usage = *proxyEvent.Usage
		}
		return &ai.AssistantMessageEventError{
			Type:   "error",
			Reason: proxyEvent.Reason,
			Error:  partial,
		}
	}

	return nil
}
