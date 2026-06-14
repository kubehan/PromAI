package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/jay-y/pi/pkg/ai"
	agent "github.com/jay-y/pi/pkg/ai-agent"
	session "github.com/jay-y/pi/pkg/ai-agent-session"
	tools "github.com/jay-y/pi/pkg/ai-agent-tools"
)

// MockFailingProvider 模拟一个会失败的 API 提供者，用于测试重试机制
type MockFailingProvider struct {
	failCount    int
	maxFailures  int
	lastErrorMsg string
}

func NewMockFailingProvider(maxFailures int) *MockFailingProvider {
	return &MockFailingProvider{
		maxFailures: maxFailures,
	}
}

func (p *MockFailingProvider) GetAPI() ai.ModelApi {
	return ai.ModelApi("mock-failing")
}

func (p *MockFailingProvider) Stream(model ai.Model, context ai.Context, options *ai.StreamOptions) *ai.AssistantMessageEventStream {
	stream := ai.NewAssistantMessageEventStream()

	go func() {
		p.failCount++

		// 模拟前几次调用失败
		if p.failCount <= p.maxFailures {
			errorMsg := fmt.Sprintf("server error: service overloaded (attempt %d)", p.failCount)
			p.lastErrorMsg = errorMsg

			// 发送错误事件
			stream.Push(&ai.AssistantMessageEventError{
				Type:   ai.ASSISTANT_MESSAGE_EVENT_ERROR,
				Reason: ai.StopReasonError,
				Error: &ai.AssistantMessage{
					Role:         "assistant",
					StopReason:   ai.StopReasonError,
					ErrorMessage: errorMsg,
				},
			})
			stream.End(&ai.AssistantMessage{
				Role:         "assistant",
				StopReason:   ai.StopReasonError,
				ErrorMessage: errorMsg,
			})
			return
		}

		// 最后一次调用成功
		stream.Push(&ai.AssistantMessageEventStart{
			Type: ai.ASSISTANT_MESSAGE_EVENT_START,
			Partial: &ai.AssistantMessage{
				Role: "assistant",
			},
		})

		stream.Push(&ai.AssistantMessageEventTextStart{
			Type: ai.ASSISTANT_MESSAGE_EVENT_TEXT_START,
			Partial: &ai.AssistantMessage{
				Role: "assistant",
				Content: []ai.ContentBlock{
					&ai.TextContentBlock{Type: "text", Text: ""},
				},
			},
		})

		stream.Push(&ai.AssistantMessageEventTextDelta{
			Type:    ai.ASSISTANT_MESSAGE_EVENT_TEXT_DELTA,
			Delta:   "重试机制测试成功！",
			Partial: &ai.AssistantMessage{
				Role: "assistant",
				Content: []ai.ContentBlock{
					&ai.TextContentBlock{Type: "text", Text: "重试机制测试成功！"},
				},
			},
		})

		stream.Push(&ai.AssistantMessageEventTextEnd{
			Type: ai.ASSISTANT_MESSAGE_EVENT_TEXT_END,
			Partial: &ai.AssistantMessage{
				Role: "assistant",
				Content: []ai.ContentBlock{
					&ai.TextContentBlock{Type: "text", Text: "重试机制测试成功！"},
				},
			},
		})

		stream.Push(&ai.AssistantMessageEventDone{
			Type:   ai.ASSISTANT_MESSAGE_EVENT_DONE,
			Reason: ai.StopReasonStop,
			Message: &ai.AssistantMessage{
				Role:       "assistant",
				StopReason: ai.StopReasonStop,
				Content: []ai.ContentBlock{
					&ai.TextContentBlock{Type: "text", Text: "重试机制测试成功！"},
				},
			},
		})

		stream.End(&ai.AssistantMessage{
			Role:       "assistant",
			StopReason: ai.StopReasonStop,
			Content: []ai.ContentBlock{
				&ai.TextContentBlock{Type: "text", Text: "重试机制测试成功！"},
			},
		})
	}()

	return stream
}

func (p *MockFailingProvider) StreamSimple(model ai.Model, context ai.Context, options *ai.SimpleStreamOptions) *ai.AssistantMessageEventStream {
	streamOptions := &ai.StreamOptions{
		APIKey:          options.APIKey,
		Headers:         options.Headers,
		MaxTokens:       options.MaxTokens,
		Temperature:     options.Temperature,
		ReasoningEffort: string(options.Reasoning),
	}
	return p.Stream(model, context, streamOptions)
}

var _ ai.ApiProvider = (*MockFailingProvider)(nil)

func main() {
	ctx := context.Background()
	ai.RegisterBuiltInApiProviders()

	// 使用正常的 Ollama 模型配置
	ollamaModel := &ai.BaseModel{
		ID:            "qwen3-coder-next:q8_0",
		Name:          "ollama/qwen3-coder-next:q8_0",
		API:           ai.ModelApi(ai.ApiOpenAICompletions),
		Provider:      ai.ModelProvider("ollama"),
		BaseURL:       "http://127.0.0.1:11434/v1",
		Reasoning:     true,
		Input:         []string{"text"},
		Cost:          ai.ModelCost{},
		ContextWindow: 128000,
		MaxTokens:     32000,
		Compat: &session.OpenAICompletionsCompat{
			ThinkingFormat: "qwen",
		},
	}

	listener := func(event session.AgentSessionEvent) {
		fmt.Printf("Event: %s\n", event.GetType())
		switch ae := event.(type) {
		case *session.AutoRetryStartEvent:
			fmt.Printf("\n🔄 [自动重试开始] 第 %d/%d 次尝试，延迟 %d ms\n",
				ae.Attempt, ae.MaxAttempts, ae.DelayMs)
			fmt.Printf("   错误信息: %s\n", ae.ErrorMessage)

		case *session.AutoRetryEndEvent:
			if ae.Success {
				fmt.Printf("\n✅ [自动重试成功] 第 %d 次尝试成功\n", ae.Attempt)
			} else {
				fmt.Printf("\n❌ [自动重试结束] 第 %d 次尝试失败\n", ae.Attempt)
				if ae.FinalError != "" {
					fmt.Printf("   最终错误: %s\n", ae.FinalError)
				}
			}

		case *agent.AgentEventMessageUpdate:
			switch e := ae.AssistantMessageEvent.(type) {
			case *ai.AssistantMessageEventTextDelta:
				fmt.Print(e.Delta)
			}

		case *agent.AgentEventToolExecutionStart:
			argsJSON, _ := json.Marshal(ae.Args)
			fmt.Printf("\n[工具] %s(%s)\n", ae.ToolName, json.RawMessage(argsJSON))

		case *agent.AgentEventToolExecutionEnd:
			resultJSON, _ := json.Marshal(ae.Result)
			fmt.Printf("[结果] %s\n", json.RawMessage(resultJSON))

		case *agent.AgentEventTurnEnd:
			messageJSON, _ := json.Marshal(ae.Message)
			fmt.Printf("\n[回合结束]: %s\n", json.RawMessage(messageJSON))

		case *agent.AgentEventEnd:
			fmt.Printf("\n--- 会话完成，共 %d 条新消息 ---\n", len(ae.Messages))
		}
	}

	al := agent.NewAgent(agent.AgentOptions{
		InitialState: &agent.AgentState{
			SystemPrompt: "你是CC，一个全能的AI助手。",
			Model:        ollamaModel,
			// Model:        mockFailingModel,
		},
	})

	cwd, err := os.Getwd()
	if err != nil {
		fmt.Println("Error getting current working directory:", err)
		return
	}

	al.SetTools(tools.CreateCodingTools(cwd))

	agentSession := session.NewAgentSession(&session.AgentSessionConfig{
		Agent:           al,
		ModelRegistry:   &session.ModelRegistry{},
		SessionManager:  session.InMemorySessionManager(cwd),
		SettingsManager: &session.SettingsManager{},
	})

	// 订阅事件并打印详细信息
	agentSession.Subscribe(listener)

	// 示例1：正常对话（无重试）
	fmt.Println("==================================================")
	fmt.Println("示例1: 正常对话")
	fmt.Println("==================================================")

	taskDone := make(chan struct{})
	// go func() {
	// 	defer close(taskDone)
	// 	if err := agentSession.Prompt(ctx, "当前目录有哪些文件？简单解读下README.md的内容", nil); err != nil {
	// 		log.Fatal(err)
	// 	}
	// }()
	// agentSession.FollowUp(ctx, "分析下pkg里的所有文件，然后按实际情况修正完善下/Users/creator/Desktop/Workspace/codes/github.com/jay-y/pi/README.md的内容", nil)
	go func() {
		defer close(taskDone)
		if err := agentSession.Prompt(ctx, "你好，请简单介绍一下你自己", nil); err != nil {
			fmt.Printf("Error: %v\n", err)
		}
	}()

	<-taskDone

	// 等待一下再开始下一个示例
	time.Sleep(1 * time.Second)

	// 注册模拟失败的 API 提供者
	ai.RegisterApiProvider(NewMockFailingProvider(3), "mock-failing")
	// 使用模拟失败的模型
	mockFailingModel := &ai.BaseModel{
		ID:            "mock-failing",
		Name:          "mock-failing",
		API:           ai.ModelApi("mock-failing"),
		APIKey:        "mock-api-key",
		Provider:      ai.ModelProvider("mock-failing"),
		Input:         []string{"text"},
	}
	al = agent.NewAgent(agent.AgentOptions{
		InitialState: &agent.AgentState{
			SystemPrompt: "你是CC，一个全能的AI助手。",
			Model:        mockFailingModel,
		},
	})
	agentSession.SetAgent(al)
	agentSession.Dispose()
	agentSession.ReconnectToAgent()
	agentSession.Subscribe(listener)

	// 示例2：模拟重试场景
	fmt.Println("\n==================================================")
	fmt.Println("示例2: 模拟重试场景")
	fmt.Println("==================================================")
	fmt.Println("\n注：要测试真实的重试机制，可以：")
	fmt.Println("1. 暂时断开网络连接")
	fmt.Println("2. 或者修改代码使用 MockFailingProvider 模拟失败")
	fmt.Println("3. 或者配置错误的 API 地址触发连接错误")

	if err := agentSession.Prompt(ctx, "你好，请简单介绍一下你自己", nil); err != nil {
		fmt.Println("成功")
	} else {
		fmt.Println("失败")
	}

		// 打印当前重试统计
	fmt.Printf("\n当前重试尝试次数: %d\n", agentSession.GetRetryAttempt())

	// 打印所有消息
	fmt.Println("\n--- 所有消息记录 ---")
	for i, msg := range agentSession.Messages() {
		fmt.Printf("[%d] %+v\n", i, msg)
	}
}
