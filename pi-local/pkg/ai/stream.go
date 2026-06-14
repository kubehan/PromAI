package ai

import (
	"context"
	"fmt"
)

// StreamOptions 流式选项
type StreamOptions struct {
	Temperature     *float64          `json:"temperature,omitempty"`
	MaxTokens       int               `json:"maxTokens,omitempty"`
	Ctx             context.Context   `json:"-"`
	APIKey          string            `json:"apiKey,omitempty"`
	Transport       Transport         `json:"transport,omitempty"`
	CacheRetention  CacheRetention    `json:"cacheRetention,omitempty"`
	SessionID       string            `json:"sessionId,omitempty"`
	OnPayload       func(payload any) `json:"-"`
	Headers         map[string]string `json:"headers,omitempty"`
	MaxRetryDelayMs int               `json:"maxRetryDelayMs,omitempty"`
	Metadata        map[string]any    `json:"metadata,omitempty"`
	ReasoningEffort string            `json:"reasoningEffort,omitempty"`
}

// ProviderStreamOptions 提供程序流式选项
type ProviderStreamOptions struct {
	StreamOptions
	Extra map[string]any `json:"-"`
}

// SimpleStreamOptions 简单流式选项
type SimpleStreamOptions struct {
	StreamOptions
	Reasoning       ThinkingLevel    `json:"reasoning,omitempty"`
	ThinkingBudgets *ThinkingBudgets `json:"thinkingBudgets,omitempty"`
}

// Stream 流式调用 LLM
func Stream(model Model, ctx Context, opts *ProviderStreamOptions) (*AssistantMessageEventStream, error) {
	provider, err := ResolveApiProvider(model.GetAPI())
	if err != nil {
		return nil, fmt.Errorf("failed to resolve provider: %w", err)
	}
	streamOpts := &StreamOptions{}
	if opts != nil {
		streamOpts = &opts.StreamOptions
	}
	// 设置上下文超时
	if streamOpts.Ctx == nil {
		streamOpts.Ctx = context.Background()
	}
	return provider.Stream(model, ctx, streamOpts), nil
}

// Complete 完成式调用 LLM（非流式）
func Complete(model Model, ctx Context, opts *ProviderStreamOptions) (*AssistantMessage, error) {
	s, err := Stream(model, ctx, opts)
	if err != nil {
		return nil, err
	}
	// 消费事件流直到完成
	for range s.Events() {
	}
	return s.Result(), nil
}

// StreamSimple 简单流式调用（带思考选项）
func StreamSimple(model Model, ctx Context, opts *SimpleStreamOptions) (*AssistantMessageEventStream, error) {
	provider, err := ResolveApiProvider(model.GetAPI())
	if err != nil {
		return nil, fmt.Errorf("failed to resolve provider: %w", err)
	}
	return provider.StreamSimple(model, ctx, opts), nil
}

// CompleteSimple 简单完成式调用（带思考选项）
func CompleteSimple(model Model, ctx Context, opts *SimpleStreamOptions) (*AssistantMessage, error) {
	s, err := StreamSimple(model, ctx, opts)
	if err != nil {
		return nil, err
	}
	// 消费事件流直到完成
	for range s.Events() {
	}
	return s.Result(), nil
}
