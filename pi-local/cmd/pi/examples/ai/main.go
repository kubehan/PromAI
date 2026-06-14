package main

import (
	"context"
	"fmt"

	"github.com/jay-y/pi/pkg/ai"
)

type Test interface {
	Exec()
}

type Xxx struct {
	Test Test
}

// func (t *TestImpl) Exec() {
// 	fmt.Println("TestImpl Exec")
// }

func main() {
	// 注册内置 API 提供者
	ai.RegisterBuiltInApiProviders()
	ollamaModel := &ai.BaseModel{
		ID:            "qwen3-coder-next:q8_0",
		Name:          "ollama/qwen3-coder-next:q8_0",
		// ID:            "qwen3.5:122b",
		// Name:          "ollama/qwen3.5:122b",
		API:           ai.ModelApi(ai.ApiOpenAICompletions),
		Provider:      ai.ModelProvider("ollama"),
		BaseURL:       "http://127.0.0.1:11434/v1",
		Reasoning:     false,
		Input:         []string{"text"},
		Cost:          ai.ModelCost{},
		ContextWindow: 128000,
		MaxTokens:     32000,
	}

	// 构建对话上下文
	cxt := ai.Context{
		SystemPrompt: "You are a helpful assistant.",
		Messages: []ai.Message{
			ai.NewUserMessage("你好！测试下对话速度～"),
		},
	}

	// 流式调用
	stream, err := ai.Stream(ollamaModel, cxt, &ai.ProviderStreamOptions{
		StreamOptions: ai.StreamOptions{
			Ctx: context.Background(),
			APIKey: "ollama",
		},
	})
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	for event := range stream.Events() {
		// jsonEvent, _ := json.Marshal(event)
		// fmt.Printf("Event: %s\n", string(jsonEvent))
		switch e := event.(type) {
		case *ai.AssistantMessageEventTextDelta:
			fmt.Print(e.Delta)
		}
	}

	// 获取最终结果
	if result := stream.Result(); result != nil {
		fmt.Printf("\nTotal tokens: %d in, %d out\n", result.Usage.Input, result.Usage.Output)
		fmt.Printf("Cost: $%.4f\n", result.Usage.Cost.Total)
	}

	// 完整调用（非流式）
	response, err := ai.Complete(ollamaModel, cxt, &ai.ProviderStreamOptions{
		StreamOptions: ai.StreamOptions{
			Ctx: context.Background(),
			APIKey: "ollama",
		},
	})
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	for _, block := range response.Content {
		if t, ok := block.(*ai.TextContentBlock); ok {
			fmt.Println(t.Text)
		}
	}
}
