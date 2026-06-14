package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/jay-y/pi/pkg/ai"
	agent "github.com/jay-y/pi/pkg/ai-agent"
	tools "github.com/jay-y/pi/pkg/ai-agent-tools"
)

func WriteFileAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Create temp file in the same directory (ensures atomic rename works)
	// Using a hidden prefix (.tmp-) to avoid issues with some tools
	tmpFile, err := os.OpenFile(
		filepath.Join(dir, fmt.Sprintf(".tmp-%d-%d", os.Getpid(), time.Now().UnixNano())),
		os.O_WRONLY|os.O_CREATE|os.O_EXCL,
		perm,
	)
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}

	tmpPath := tmpFile.Name()
	cleanup := true

	defer func() {
		if cleanup {
			tmpFile.Close()
			_ = os.Remove(tmpPath)
		}
	}()

	// Write data to temp file
	// Note: Original file is untouched at this point
	if _, err := tmpFile.Write(data); err != nil {
		return fmt.Errorf("failed to write temp file: %w", err)
	}

	// CRITICAL: Force sync to storage medium before any other operations.
	// This ensures data is physically written to disk, not just cached.
	// Essential for SD cards, eMMC, and other flash storage on edge devices.
	if err := tmpFile.Sync(); err != nil {
		return fmt.Errorf("failed to sync temp file: %w", err)
	}

	// Set file permissions before closing
	if err := tmpFile.Chmod(perm); err != nil {
		return fmt.Errorf("failed to set permissions: %w", err)
	}

	// Close file before rename (required on Windows)
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("failed to close temp file: %w", err)
	}

	// Atomic rename: temp file becomes the target
	// On POSIX: rename() is atomic
	// On Windows: Rename() is atomic for files
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	// Sync directory to ensure rename is durable
	// This prevents the renamed file from disappearing after a crash
	if dirFile, err := os.Open(dir); err == nil {
		_ = dirFile.Sync()
		dirFile.Close()
	}

	// Success: skip cleanup (file was renamed, no temp to remove)
	cleanup = false
	return nil
}

func main() {
	ctx := context.Background()

	ai.RegisterBuiltInApiProviders()
	ollamaModel := &ai.BaseModel{
		ID:            "qwen3-coder-next:q8_0",
		Name:          "ollama/qwen3-coder-next:q8_0",
		// ID:            "qwen3.5:35b",
		// Name:          "ollama/qwen3.5:35b",
		API:           ai.ModelApi(ai.ApiOpenAICompletions),
		Provider:      ai.ModelProvider("ollama"),
		BaseURL:       "http://127.0.0.1:11434/v1",
		Reasoning:     false,
		Input:         []string{"text"},
		Cost:          ai.ModelCost{},
		ContextWindow: 128000,
		MaxTokens:     32000,
	}
	al := agent.NewAgent(agent.AgentOptions{
		InitialState: &agent.AgentState{
			SystemPrompt:     "你是一个助手。一步一步完成任务。",
			Model:            ollamaModel,
		},
	})
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Println("Error getting current working directory:", err)
		return
	}
	al.SetTools([]agent.AgentTool{
		tools.NewReadTool(cwd),
		tools.NewWriteTool(cwd),
		tools.NewBashTool(cwd),
		tools.NewWriteTool(cwd),
	})

	fmt.Println("Agent created, subscribing to events...")
	// 订阅事件 — 一个 subscribe 搞定所有 UI 更新
	unsubscribe := al.Subscribe(func(event agent.AgentEvent) {
		switch ae := event.(type) {
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
		case *agent.AgentEventEnd:
			fmt.Printf("\n--- 完成，共 %d 条新消息 ---\n", len(ae.Messages))
		default:
			jsonEvent, _ := json.Marshal(event)
			fmt.Printf("Event: %s\n", string(jsonEvent))
		}
	})
	defer unsubscribe()

	// === 场景 1: Steering — 用户中途改主意 ===
	fmt.Println("\n=== 场景 1: Steering — 用户中途改主意 ===")

	// 启动任务
	taskDone := make(chan struct{})
	go func() {
		defer close(taskDone)
		if err := al.Prompt(ctx, "帮我写 3 个文件：config.ts, utils.ts, main.ts"); err != nil {
			fmt.Printf("Error: %v\n", err)
		}
	}()

	// 1 秒后用户说 "算了只要 main.ts"
	go func() {
		time.Sleep(1 * time.Second)
		al.Steer(ai.NewUserMessage("停下！只需要 main.ts 就行了，其他不要"))
	}()

	<-taskDone
	// Agent 在完成当前工具后，收到 steering 消息，跳过剩余工具，按新指令继续

	// === 场景 2: Follow-up — 做完了再加任务 ===
	fmt.Println("\n=== 场景 2: Follow-up — 做完了再加任务 ===")

	// 启动任务
	taskDone2 := make(chan struct{})
	go func() {
		defer close(taskDone2)
		if err := al.Prompt(ctx, "写一个 hello world"); err != nil {
			fmt.Printf("Error: %v\n", err)
		}
	}()

	// 发送 follow-up 消息
	al.FollowUp(ai.NewUserMessage("写完后帮我加上错误处理"))

	<-taskDone2
	// Agent 完成 hello world 后，不会停止，而是继续处理 follow-up 消息
}
