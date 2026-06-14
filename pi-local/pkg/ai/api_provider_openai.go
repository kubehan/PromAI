package ai

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

// OpenAICompletionsProvider OpenAI Completions 提供者
type OpenAICompletionsProvider struct {
	client *http.Client
}

// NewOpenAICompletionsApiProvider 创建新的提供者
func NewOpenAICompletionsApiProvider() *OpenAICompletionsProvider {
	return &OpenAICompletionsProvider{
		client: &http.Client{Timeout: 120 * time.Second},
	}
}

func (p *OpenAICompletionsProvider) GetAPI() ModelApi {
	return ModelApi(ApiOpenAICompletions)
}

// Stream 流式调用
func (p *OpenAICompletionsProvider) Stream(
	model Model,
	ctx Context,
	opts *StreamOptions,
) *AssistantMessageEventStream {
	stream := NewAssistantMessageEventStream()

	go func() {
		defer func() {
			if r := recover(); r != nil {
				stream.Push(&AssistantMessageEventError{
					Type:   ASSISTANT_MESSAGE_EVENT_ERROR,
					Error:  &AssistantMessage{
						StopReason: StopReasonError,
						ErrorMessage: fmt.Sprintf("panic: %v", r),
						Timestamp: time.Now().Unix(),
					},
				})
				stream.End(nil)
			}
		}()

		output := &AssistantMessage{
			Role:      "assistant",
			Content:   []ContentBlock{},
			API:       model.GetAPI(),
			Provider:  model.GetProvider(),
			Model:     model.GetID(),
			Usage:     Usage{},
			StopReason: StopReasonStop,
			Timestamp: time.Now().UnixMilli(),
		}

		if err := p.doStream(model, ctx, opts, stream, output); err != nil {
			output.StopReason = StopReasonError
			output.ErrorMessage = err.Error()
			stream.Push(&AssistantMessageEventError{
				Type:   ASSISTANT_MESSAGE_EVENT_ERROR,
				Reason: output.StopReason,
				Error:  output,
			})
			stream.End(output)
		} else {
			stream.Push(&AssistantMessageEventDone{
				Type:    ASSISTANT_MESSAGE_EVENT_DONE,
				Reason:  output.StopReason,
				Message: output,
			})
			stream.End(output)
		}
	}()

	return stream
}

func (p *OpenAICompletionsProvider) StreamSimple(
	model Model,
	ctx Context,
	opts *SimpleStreamOptions,
) *AssistantMessageEventStream {
	apiKey := opts.APIKey
	if apiKey == "" {
		apiKey = getEnvApiKey(model.GetProvider())
	}
	streamOptions := &StreamOptions{
		APIKey:          apiKey,
		Headers:         opts.Headers,
		MaxTokens:       opts.MaxTokens,
		Temperature:     opts.Temperature,
		ReasoningEffort: string(opts.ReasoningEffort),
	}
	return p.Stream(model, ctx, streamOptions)
}

// doStream 执行流式请求
func (p *OpenAICompletionsProvider) doStream(
	model Model,
	ctx Context,
	opts *StreamOptions,
	stream *AssistantMessageEventStream,
	output *AssistantMessage,
) error {
	apiKey := opts.APIKey
	if apiKey == "" {
		apiKey = getEnvApiKey(model.GetProvider())
	}

	req, err := p.buildRequest(model, ctx, opts, apiKey)
	if err != nil {
		return err
	}

	log.Printf("[DEBUG] doStream sending request...")
	resp, err := p.client.Do(req)
	if err != nil {
		log.Printf("[DEBUG] doStream request failed: %v", err)
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	log.Printf("[DEBUG] doStream response status: %d", resp.StatusCode)

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		log.Printf("[DEBUG] doStream error body: %s", string(body))
		return fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	return p.processStream(resp.Body, stream, output, model)
}

// buildRequest 构建 HTTP 请求
func (p *OpenAICompletionsProvider) buildRequest(
	model Model,
	ctx Context,
	opts *StreamOptions,
	apiKey string,
) (*http.Request, error) {
	params := p.buildParams(model, ctx, opts)

	body, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("marshal params: %w", err)
	}

	log.Printf("[DEBUG] buildRequest URL: %s", model.GetBaseURL()+"/chat/completions")
	log.Printf("[DEBUG] buildRequest Body: %s", string(body))

	req, err := http.NewRequest("POST", model.GetBaseURL()+"/chat/completions", strings.NewReader(string(body)))
	if err != nil {
		log.Printf("[DEBUG] buildRequest NewRequest error: %v", err)
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	log.Printf("[DEBUG] buildRequest Authorization: Bearer %s...", apiKey[:min(len(apiKey), 8)])

	// 合并模型头部
	for k, v := range model.GetHeaders() {
		req.Header.Set(k, v)
		log.Printf("[DEBUG] buildRequest header %s: %s", k, v)
	}

	// 合并选项头部
	for k, v := range opts.Headers {
		req.Header.Set(k, v)
		log.Printf("[DEBUG] buildRequest opts header %s: %s", k, v)
	}

	return req, nil
}

// buildParams 构建请求参数
func (p *OpenAICompletionsProvider) buildParams(
	model Model,
	ctx Context,
	opts *StreamOptions,
) map[string]any {
	params := map[string]any{
		"model":    model.GetID(),
		"stream":   true,
		"messages": p.convertMessages(model, ctx),
	}

	if opts.MaxTokens > 0 {
		params["max_completion_tokens"] = opts.MaxTokens
	}

	if opts.Temperature != nil {
		params["temperature"] = *opts.Temperature
	}

	if len(ctx.Tools) > 0 {
		params["tools"] = p.convertTools(ctx.Tools)
	}

	return params
}

// convertMessages 转换消息格式
func (p *OpenAICompletionsProvider) convertMessages(model Model, ctx Context) []map[string]any {
	var messages []map[string]any

	if ctx.SystemPrompt != "" {
		messages = append(messages, map[string]any{
			"role":    "system",
			"content": ctx.SystemPrompt,
		})
	}

	for _, msg := range ctx.Messages {
		switch m := msg.(type) {
		case *UserMessage:
			messages = append(messages, p.convertUserMessage(m))
		case *AssistantMessage:
			messages = append(messages, p.convertAssistantMessage(m))
		case *ToolResultMessage:
			messages = append(messages, p.convertToolResultMessage(m))
		}
	}

	return messages
}

// convertUserMessage 转换用户消息
func (p *OpenAICompletionsProvider) convertUserMessage(msg *UserMessage) map[string]any {
	if content, ok := msg.Content.(string); ok {
		return map[string]any{
			"role":    "user",
			"content": content,
		}
	}

	// 处理多模态内容
	var contentParts []map[string]any
	for _, block := range msg.Content.([]ContentBlock) {
		switch b := block.(type) {
		case *TextContentBlock:
			contentParts = append(contentParts, map[string]any{
				"type": "text",
				"text": b.Text,
			})
		case *ImageContentBlock:
			contentParts = append(contentParts, map[string]any{
				"type": "image_url",
				"image_url": map[string]any{
					"url": fmt.Sprintf("data:%s;base64,%s", b.MimeType, b.Data),
				},
			})
		}
	}

	return map[string]any{
		"role":    "user",
		"content": contentParts,
	}
}

// convertAssistantMessage 转换助手消息
func (p *OpenAICompletionsProvider) convertAssistantMessage(msg *AssistantMessage) map[string]any {
	var content string
	var toolCalls []map[string]any

	for _, block := range msg.Content {
		switch b := block.(type) {
		case *TextContentBlock:
			content += b.Text
		case *ToolCall:
			// 将 Arguments 转换为 JSON 字符串
			var argsJSON string
			if b.Arguments != nil {
				if bytes, err := json.Marshal(b.Arguments); err == nil {
					argsJSON = string(bytes)
				}
			}
			
			toolCalls = append(toolCalls, map[string]any{
				"id":   b.ID,
				"type": "function",
				"function": map[string]any{
					"name":      b.Name,
					"arguments": argsJSON,
				},
			})
		}
	}

	result := map[string]any{
		"role": "assistant",
	}

	if content != "" {
		result["content"] = content
	}

	if len(toolCalls) > 0 {
		result["tool_calls"] = toolCalls
	}

	return result
}

// convertToolResultMessage 转换工具结果消息
func (p *OpenAICompletionsProvider) convertToolResultMessage(msg *ToolResultMessage) map[string]any {
	// 将 ContentBlock 数组转换为字符串
	var contentStr string
	for _, block := range msg.Content {
		if tc, ok := block.(*TextContentBlock); ok {
			contentStr += tc.Text
		}
	}
	
	return map[string]any{
		"role":         "tool",
		"tool_call_id": msg.ToolCallID,
		"content":      contentStr,
	}
}

// convertTools 转换工具定义
func (p *OpenAICompletionsProvider) convertTools(tools []Tool) []map[string]any {
	var result []map[string]any
	for _, tool := range tools {
		result = append(result, map[string]any{
			"type": "function",
			"function": map[string]any{
				"name":        tool.Name,
				"description": tool.Description,
				"parameters":  tool.Parameters,
			},
		})
	}
	return result
}

// processStream 处理流式响应
func (p *OpenAICompletionsProvider) processStream(
	reader io.Reader,
	stream *AssistantMessageEventStream,
	output *AssistantMessage,
	model Model,
) error {
	// 使用 SSE 解析器
	parser := NewSSEParser(reader)

		var currentBlock ContentBlock
		var blocks []ContentBlock
		toolCallBlocks := map[int]*ToolCall{}
		toolCallRawArgs := map[int]string{}

	// 发送开始事件
	// stream.Push(&AssistantMessageEventStart{
	// 	Type:    ASSISTANT_MESSAGE_EVENT_START,
	// 	Partial: output,
	// })

	for {
		event, err := parser.Next()
		if err == io.EOF {
			log.Printf("[DEBUG] processStream EOF")
			break
		}
		if err != nil {
			log.Printf("[DEBUG] processStream parse error: %v", err)
			return fmt.Errorf("parse stream: %w", err)
		}

		log.Printf("[DEBUG] processStream raw event: %s", event.Data[:min(len(event.Data), 500)])

		if event.Data == "[DONE]" {
			log.Printf("[DEBUG] processStream [DONE]")
			break
		}

		var chunk ChatCompletionChunk
		if err := json.Unmarshal([]byte(event.Data), &chunk); err != nil {
			log.Printf("[DEBUG] processStream unmarshal error (skipped): %v, data: %s", err, event.Data[:min(len(event.Data), 200)])
			continue // 忽略解析错误
		}

		// 处理 usage
		if chunk.Usage != nil {
			var cachedTokens int
			if promptTokensDetails := chunk.Usage.PromptTokensDetails; promptTokensDetails != nil {
				cachedTokens = promptTokensDetails.CachedTokens
			} else {
				cachedTokens = 0
			}
			var reasoningTokens int
			if completionTokensDetails := chunk.Usage.CompletionTokensDetails; completionTokensDetails != nil {
				reasoningTokens = completionTokensDetails.ReasoningTokens
			} else {
				reasoningTokens = 0
			}
			var outputTokens int
			if completionTokensDetails := chunk.Usage.CompletionTokensDetails; completionTokensDetails != nil {
				outputTokens = completionTokensDetails.AcceptedPredictionTokens + completionTokensDetails.RejectedPredictionTokens
			} else {
				outputTokens = 0
			}
			inputTokens := chunk.Usage.PromptTokens - cachedTokens
			output.Usage.Input = inputTokens
			output.Usage.Output = outputTokens + reasoningTokens
			output.Usage.TotalTokens = inputTokens + outputTokens + reasoningTokens
			output.Usage.CacheRead = cachedTokens
			output.Usage.TotalTokens = inputTokens + outputTokens + reasoningTokens
			output.Usage.Cost = calculateCost(model, &output.Usage)
		}

		if len(chunk.Choices) == 0 {
			continue
		}

		choice := chunk.Choices[0]

		// 处理停止原因
		if choice.FinishReason != "" {
			output.StopReason = p.mapStopReason(choice.FinishReason)
		}

		// 处理内容增量
		if choice.Delta.Content != "" {
			if currentBlock == nil || currentBlock.GetType() != "text" {
				// 结束工具调用块（如有）
				if len(toolCallBlocks) > 0 {
					finishToolCallBlocks(stream, blocks, toolCallBlocks, output)
					currentBlock = nil
				}
				// 结束当前块
				if currentBlock != nil {
					p.finishBlock(stream, currentBlock, len(blocks)-1, output)
				}

				// 开始新块
				currentBlock = &TextContentBlock{Type: "text"}
				blocks = append(blocks, currentBlock)
				output.Content = blocks

				stream.Push(&AssistantMessageEventTextStart{
					Type:         ASSISTANT_MESSAGE_EVENT_TEXT_START,
					ContentIndex: len(blocks) - 1,
					Partial:      output,
				})
			}

			if textBlock, ok := currentBlock.(*TextContentBlock); ok {
				textBlock.Text += choice.Delta.Content
				stream.Push(&AssistantMessageEventTextDelta{
					Type:         ASSISTANT_MESSAGE_EVENT_TEXT_DELTA,
					ContentIndex: len(blocks) - 1,
					Delta:        choice.Delta.Content,
					Partial:      output,
				})
			}
		}

		// 处理思考内容
		if choice.Delta.ReasoningContent != "" {
			if currentBlock == nil || currentBlock.GetType() != "thinking" {
				// 结束工具调用块（如有）
				if len(toolCallBlocks) > 0 {
					finishToolCallBlocks(stream, blocks, toolCallBlocks, output)
					currentBlock = nil
				}
				if currentBlock != nil {
					p.finishBlock(stream, currentBlock, len(blocks)-1, output)
				}

				currentBlock = &ThinkingContentBlock{
					Type:              "thinking",
					ThinkingSignature: "reasoning_content",
				}
				blocks = append(blocks, currentBlock)
				output.Content = blocks

				stream.Push(&AssistantMessageEventThinkingStart{
					Type:         ASSISTANT_MESSAGE_EVENT_THINKING_START,
					ContentIndex: len(blocks) - 1,
					Partial:      output,
				})
			}

			if thinkingBlock, ok := currentBlock.(*ThinkingContentBlock); ok {
				thinkingBlock.Thinking += choice.Delta.ReasoningContent
				stream.Push(&AssistantMessageEventThinkingDelta{
					Type:         ASSISTANT_MESSAGE_EVENT_THINKING_DELTA,
					ContentIndex: len(blocks) - 1,
					Delta:        choice.Delta.ReasoningContent,
					Partial:      output,
				})
			}
		}

		// 处理工具调用
		for _, tCall := range choice.Delta.ToolCalls {
			idx := tCall.Index
			tc, exists := toolCallBlocks[idx]
			if !exists {
				if currentBlock != nil && currentBlock.GetType() != "toolCall" {
					p.finishBlock(stream, currentBlock, len(blocks)-1, output)
					currentBlock = nil
				}

				tc = &ToolCall{
					Type: "toolCall",
				}
				toolCallBlocks[idx] = tc
				blocks = append(blocks, tc)
				output.Content = blocks
				currentBlock = tc

				stream.Push(&AssistantMessageEventToolCallStart{
					Type:         ASSISTANT_MESSAGE_EVENT_TOOLCALL_START,
					ContentIndex: len(blocks) - 1,
					Partial:      output,
				})
			}

			if tCall.ID != "" {
				tc.ID = tCall.ID
			}
			if tCall.Function.Name != "" {
				tc.Name = tCall.Function.Name
			}
			if tCall.Function.Arguments != "" {
				toolCallRawArgs[idx] += tCall.Function.Arguments
				tc.Arguments = p.parseStreamingJSON(toolCallRawArgs[idx])
			}

			stream.Push(&AssistantMessageEventToolCallDelta{
				Type:         ASSISTANT_MESSAGE_EVENT_TOOLCALL_DELTA,
				ContentIndex: toolCallIndexInBlocks(blocks, tc),
				Delta:        tCall.Function.Arguments,
				Partial:      output,
			})
		}
	}

	// 结束最后一个块
	if currentBlock != nil {
		p.finishBlock(stream, currentBlock, len(blocks)-1, output)
	}
	// 结束未 finish 的工具调用块
	finishToolCallBlocks(stream, blocks, toolCallBlocks, output)

	return nil
}

// finishBlock 结束内容块
func (p *OpenAICompletionsProvider) finishBlock(
	stream *AssistantMessageEventStream,
	block ContentBlock,
	index int,
	output *AssistantMessage,
) {
	switch b := block.(type) {
	case *TextContentBlock:
		stream.Push(&AssistantMessageEventTextEnd{
			Type:         ASSISTANT_MESSAGE_EVENT_TEXT_END,
			ContentIndex: index,
			Content:      b.Text,
			Partial:      output,
		})
	case *ThinkingContentBlock:
		stream.Push(&AssistantMessageEventThinkingEnd{
			Type:         ASSISTANT_MESSAGE_EVENT_THINKING_END,
			ContentIndex: index,
			Content:      b.Thinking,
			Partial:      output,
		})
	case *ToolCall:
		stream.Push(&AssistantMessageEventToolCallEnd{
			Type:         ASSISTANT_MESSAGE_EVENT_TOOLCALL_END,
			ContentIndex: index,
			ToolCall:     b,
			Partial:      output,
		})
	}
}

// mapStopReason 映射停止原因
func (p *OpenAICompletionsProvider) mapStopReason(reason string) StopReason {
	switch reason {
	case "stop":
		return "stop"
	case "length":
		return "length"
	case "tool_calls":
		return "toolUse"
	case "content_filter":
		return "error"
	default:
		return "stop"
	}
}

// parseStreamingJSON 解析流式 JSON
func (p *OpenAICompletionsProvider) parseStreamingJSON(data string) map[string]any {
	var result map[string]any
	json.Unmarshal([]byte(data), &result)
	return result
}

// toolCallIndexInBlocks 查找 tool call block 在 blocks 中的索引
func toolCallIndexInBlocks(blocks []ContentBlock, target *ToolCall) int {
	for i, b := range blocks {
		if b == target {
			return i
		}
	}
	return -1
}

// finishToolCallBlocks 结束所有工具调用块（在切换到文本/思考块前调用）
func finishToolCallBlocks(stream *AssistantMessageEventStream, blocks []ContentBlock, toolCallBlocks map[int]*ToolCall, output *AssistantMessage) {
	for _, tc := range toolCallBlocks {
		idx := toolCallIndexInBlocks(blocks, tc)
		if idx < 0 {
			continue
		}
		// 跳过已经 finish 过的块（后面有其他 block）
		if len(blocks) > idx+1 {
			continue
		}
		stream.Push(&AssistantMessageEventToolCallEnd{
			Type:         ASSISTANT_MESSAGE_EVENT_TOOLCALL_END,
			ContentIndex: idx,
			ToolCall:     tc,
			Partial:      output,
		})
	}
}

// 确保 OpenAICompletionsProvider 实现 ApiProvider 接口
var _ ApiProvider = (*OpenAICompletionsProvider)(nil)

// ChatCompletionChunk OpenAI 流式响应块
type ChatCompletionChunk struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index        int `json:"index"`
		Delta        struct {
			Role            string     `json:"role"`
			Content         string     `json:"content"`
			ReasoningContent string    `json:"reasoning_content"`
			ToolCalls       []struct {
				Index    int `json:"index"`
				ID       string `json:"id"`
				Type     string `json:"type"`
				Function struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls"`
		} `json:"delta"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage *struct {
		PromptTokens     int `json:"prompt_tokens"`
		PromptTokensDetails *struct {
			AudioTokens int `json:"audio_tokens"`
			CachedTokens int `json:"cached_tokens"`
		} `json:"prompt_tokens_details"`
		CompletionTokens int `json:"completion_tokens"`
		CompletionTokensDetails *struct {
			AcceptedPredictionTokens int `json:"accepted_prediction_tokens"`
			AudioTokens int `json:"audio_tokens"`
			ReasoningTokens int `json:"reasoning_tokens"`
			RejectedPredictionTokens int `json:"rejected_prediction_tokens"`
		} `json:"completion_tokens_details"`
		TotalTokens      int `json:"total_tokens"`
		CachedTokens     int `json:"cached_tokens"`
	} `json:"usage"`
}

// SSEParser SSE 解析器
type SSEParser struct {
	reader *bufio.Reader
}

func NewSSEParser(reader io.Reader) *SSEParser {
	return &SSEParser{reader: bufio.NewReader(reader)}
}

func (p *SSEParser) Next() (*SSEEvent, error) {
	for {
		line, err := p.reader.ReadString('\n')
		if err != nil {
			return nil, err
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if strings.HasPrefix(line, "data:") {
			data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			return &SSEEvent{Data: data}, nil
		}
	}
}

type SSEEvent struct {
	Data string
}