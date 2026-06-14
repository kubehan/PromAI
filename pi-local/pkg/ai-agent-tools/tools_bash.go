package tools

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/jay-y/pi/pkg/ai"
	agent "github.com/jay-y/pi/pkg/ai-agent"
)

// BashToolInput Bash 工具输入
type BashToolInput struct {
	Command string `json:"command"`
	Timeout int    `json:"timeout,omitempty"`
}

// BashToolDetails Bash 工具详细信息
type BashToolDetails struct {
	Truncation     *TruncationResult `json:"truncation,omitempty"`
	FullOutputPath string           `json:"fullOutputPath,omitempty"`
}

// BashOperations Bash 操作接口
type BashOperations interface {
	Exec(ctx context.Context, command string, cwd string, options BashExecOptions) (*BashExecResult, error)
}

// BashExecOptions Bash 执行选项
type BashExecOptions struct {
	OnData  func([]byte)
	Timeout time.Duration
	Env     []string
}

// BashExecResult Bash 执行结果
type BashExecResult struct {
	ExitCode *int
}

// DefaultBashOperations 默认 Bash 操作
type DefaultBashOperations struct{}

func (o *DefaultBashOperations) Exec(ctx context.Context, command string, cwd string, options BashExecOptions) (*BashExecResult, error) {
	if _, err := os.Stat(cwd); os.IsNotExist(err) {
		return nil, fmt.Errorf("working directory does not exist: %s", cwd)
	}
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/bash"
	}
	args := []string{"-c", command}
	cmd := exec.CommandContext(ctx, shell, args...)
	cmd.Dir = cwd
	cmd.Env = options.Env
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stdoutPipe)
		for scanner.Scan() {
			options.OnData(scanner.Bytes())
		}
	}()
	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stderrPipe)
		for scanner.Scan() {
			options.OnData(scanner.Bytes())
		}
	}()
	wg.Wait()
	exitCode := 0
	if cmd.ProcessState != nil {
		exitCode = cmd.ProcessState.ExitCode()
	}
	return &BashExecResult{ExitCode: &exitCode}, nil
}

// BashTool Bash 工具
type BashTool struct {
	cwd          string
	operations    BashOperations
	commandPrefix string
}

func NewBashTool(cwd string, options ...BashToolOption) agent.AgentTool {
	tool := &BashTool{
		cwd:       cwd,
		operations: &DefaultBashOperations{},
	}
	for _, opt := range options {
		if opt != nil { opt(tool) }
	}
	return agent.NewAgentTool(tool)
}

type BashToolOption func(*BashTool)

func WithBashOperations(ops BashOperations) BashToolOption {
	return func(t *BashTool) {
		t.operations = ops
	}
}

func WithBashCommandPrefix(prefix string) BashToolOption {
	return func(t *BashTool) {
		t.commandPrefix = prefix
	}
}

func (t *BashTool) GetName() string { return "bash" }
func (t *BashTool) GetLabel() string { return "bash" }
func (t *BashTool) GetDescription() string {
	return fmt.Sprintf("Execute a bash command in: %s. Output truncated to last %d lines or %dKB.",
		t.cwd, DEFAULT_MAX_LINES, DEFAULT_MAX_BYTES/1024)
}
func (t *BashTool) GetParameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"command": map[string]any{
				"type":        "string",
				"description": "Bash command to execute",
			},
			"timeout": map[string]any{
				"type":        "number",
				"description": "Timeout in seconds",
			},
		},
		"required": []string{"command"},
	}
}

func (t *BashTool) Execute(ctx context.Context, params map[string]any, onUpdate func(partialResult *agent.AgentToolResult)) (*agent.AgentToolResult, error) {
	input, err := ValidateToolParams[BashToolInput](params)
	if err != nil {
		return nil, err
	}
	
	command := input.Command
	if command == "" {
		return nil, fmt.Errorf("command is required")
	}
	
	var timeout time.Duration
	if input.Timeout > 0 {
		timeout = time.Duration(input.Timeout) * time.Second
	}
	if t.commandPrefix != "" {
		command = t.commandPrefix + "\n" + command
	}
	var mu sync.Mutex
	var chunks [][]byte
	var totalBytes int
	var tempFilePath string
	var tempFile *os.File
	onData := func(data []byte) {
		mu.Lock()
		defer mu.Unlock()
		totalBytes += len(data)
		if totalBytes > DEFAULT_MAX_BYTES && tempFile == nil {
			tempFilePath = filepath.Join(os.TempDir(), fmt.Sprintf("pi-bash-%x.log", time.Now().UnixNano()))
			var err error
			tempFile, err = os.Create(tempFilePath)
			if err == nil {
				for _, chunk := range chunks {
					tempFile.Write(chunk)
				}
			}
		}
		if tempFile != nil {
			tempFile.Write(data)
		}
		chunks = append(chunks, data)
		if onUpdate != nil {
			fullBuffer := bytes.Join(chunks, []byte{})
			fullText := string(fullBuffer)
			truncation := TruncateTail(fullText)
			onUpdate(&agent.AgentToolResult{
				Content: []ai.ContentBlock{ai.NewTextContentBlock(truncation.Content)},
				Details: &BashToolDetails{
					Truncation:     &truncation,
					FullOutputPath: tempFilePath,
				},
			})
		}
	}
	execCtx := ctx
	if timeout > 0 {
		var cancel context.CancelFunc
		execCtx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	result, err := t.operations.Exec(execCtx, command, t.cwd, BashExecOptions{
		OnData:  onData,
		Timeout: timeout,
		Env:     os.Environ(),
	})
	if tempFile != nil {
		tempFile.Close()
	}
	mu.Lock()
	defer mu.Unlock()
	fullBuffer := bytes.Join(chunks, []byte{})
	fullOutput := string(fullBuffer)
	truncation := TruncateTail(fullOutput)
	var outputText string
	if truncation.Content == "" {
		outputText = "(no output)"
	} else {
		outputText = truncation.Content
	}
	var details *BashToolDetails
	if truncation.Truncated {
		details = &BashToolDetails{
			Truncation:     &truncation,
			FullOutputPath: tempFilePath,
		}
		startLine := truncation.TotalLines - truncation.OutputLines + 1
		endLine := truncation.TotalLines
		if truncation.LastLinePartial {
			lastLineSize := FormatSize(len(bytes.Split(fullBuffer, []byte("\n"))[len(bytes.Split(fullBuffer, []byte("\n")))-1]))
			outputText += fmt.Sprintf("\n\n[Showing last %s of line %d (line is %s). Full output: %s]",
				FormatSize(truncation.OutputBytes), endLine, lastLineSize, tempFilePath)
		} else if truncation.TruncatedBy == "lines" {
			outputText += fmt.Sprintf("\n\n[Showing lines %d-%d of %d. Full output: %s]",
				startLine, endLine, truncation.TotalLines, tempFilePath)
		} else {
			outputText += fmt.Sprintf("\n\n[Showing lines %d-%d of %d (%s limit). Full output: %s]",
				startLine, endLine, truncation.TotalLines, FormatSize(DEFAULT_MAX_BYTES), tempFilePath)
		}
	}
	if err != nil {
		if ctx.Err() == context.Canceled {
			return nil, fmt.Errorf("%s\n\nCommand aborted", outputText)
		}
		if execCtx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("%s\n\nCommand timed out after %d seconds", outputText, input.Timeout)
		}
		return nil, err
	}
	if result.ExitCode != nil && *result.ExitCode != 0 {
		return nil, fmt.Errorf("%s\n\nCommand exited with code %d", outputText, *result.ExitCode)
	}
	return &agent.AgentToolResult{
		Content: []ai.ContentBlock{ai.NewTextContentBlock(outputText)},
		Details: details,
	}, nil
}