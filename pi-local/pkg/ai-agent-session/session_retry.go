package session

import (
	"context"
	"fmt"
	"time"

	"github.com/jay-y/pi/pkg/ai"
)

// RetryConfig 重试配置
type RetryConfig struct {
	MaxAttempts  int     `json:"maxAttempts"`
	BaseDelayMs  int     `json:"baseDelayMs"`
	MaxDelayMs   int     `json:"maxDelayMs"`
	JitterFactor float64 `json:"jitterFactor"`
}

// DefaultRetryConfig 默认重试配置
var DefaultRetryConfig = RetryConfig{
	MaxAttempts:  3,
	BaseDelayMs:  1000,
	MaxDelayMs:   30000,
	JitterFactor: 0.1,
}

// isRetryableError 检查是否是可重试错误
func (s *AgentSession) isRetryableError(msg *ai.AssistantMessage) bool {
	if msg.StopReason != StopReasonError || msg.ErrorMessage == "" {
		return false
	}

	// Context overflow 由压缩处理，不是重试
	// 检查错误消息是否包含可重试的关键词
	err := msg.ErrorMessage
	// Match: overloaded_error, rate limit, 429, 500, 502, 503, 504, service unavailable, connection errors, fetch failed, terminated, retry delay exceeded
	return containsIgnoreCase(err, "overloaded") ||
		containsIgnoreCase(err, "rate limit") ||
		containsIgnoreCase(err, "too many requests") ||
		containsIgnoreCase(err, "429") ||
		containsIgnoreCase(err, "500") ||
		containsIgnoreCase(err, "502") ||
		containsIgnoreCase(err, "503") ||
		containsIgnoreCase(err, "504") ||
		containsIgnoreCase(err, "service unavailable") ||
		containsIgnoreCase(err, "server error") ||
		containsIgnoreCase(err, "internal error") ||
		containsIgnoreCase(err, "connection error") ||
		containsIgnoreCase(err, "connection refused") ||
		containsIgnoreCase(err, "other side closed") ||
		containsIgnoreCase(err, "fetch failed") ||
		containsIgnoreCase(err, "upstream connect") ||
		containsIgnoreCase(err, "reset before headers") ||
		containsIgnoreCase(err, "terminated") ||
		containsIgnoreCase(err, "retry delay")
}

// handleRetryableError 处理可重试错误
// 简化重试链：重试失败后自然触发下一次 handleAgentEnd
func (s *AgentSession) handleRetryableError(msg *ai.AssistantMessage) bool {
	s.retryMu.Lock()
	defer s.retryMu.Unlock()

	config := DefaultRetryConfig

	// 检查是否启用重试
	if !s.AutoRetryEnabled() {
		s.resolveRetry()
		return false
	}

	// 增加重试计数
	s.retryAttempt++

	// 检查是否超过最大重试次数
	if s.retryAttempt > config.MaxAttempts {
		// 达到最大重试次数，重置并重试 Promise
		s.emit(&AutoRetryEndEvent{
			Type:       "auto_retry_end",
			Success:    false,
			Attempt:    s.retryAttempt - 1,
			FinalError: msg.ErrorMessage,
		})
		s.retryAttempt = 0
		s.resolveRetry()
		return false
	}

	// 计算延迟（指数退避）
	delayMs := calculateBackoffDelay(s.retryAttempt, config)

	s.emit(&AutoRetryStartEvent{
		Type:         "auto_retry_start",
		Attempt:      s.retryAttempt,
		MaxAttempts:  config.MaxAttempts,
		DelayMs:      delayMs,
		ErrorMessage: msg.ErrorMessage,
	})

	// 从 agent state 中移除错误消息
	messages := s.agent.GetState().Messages
	if len(messages) > 0 && messages[len(messages)-1].GetRole() == "assistant" {
		s.agent.ReplaceMessages(messages[:len(messages)-1])
	}

	// 创建可取消的上下文
	ctx, cancel := context.WithCancel(context.Background())
	s.retryAbortController = cancel

	// 启动重试 goroutine
	go func() {
		defer cancel()

		select {
		case <-ctx.Done():
			// 被取消
			attempt := s.retryAttempt
			s.retryAttempt = 0
			s.retryAbortController = nil
			s.emit(&AutoRetryEndEvent{
				Type:       "auto_retry_end",
				Success:    false,
				Attempt:    attempt,
				FinalError: "Retry cancelled",
			})
			s.resolveRetry()
			return
		case <-time.After(time.Duration(delayMs) * time.Millisecond):
			// 延迟结束，执行重试
			// 使用 setTimeout 模式跳出事件处理器链
			go func() {
				// 注意：这里不重置 retryAttempt，让下一次 agent_end 来处理
				// 如果重试成功，handleMessageEnd 会重置 retryAttempt
				// 如果重试失败，会再次触发 handleAgentEnd 并继续重试
				err := s.agent.Continue(ctx)
				if err != nil {
					// 重试失败 - 会被下一次 agent_end 捕获
					// 不需要特殊处理
				}
			}()
		}
	}()

	return true
}

// calculateBackoffDelay 计算退避延迟
func calculateBackoffDelay(attempt int, config RetryConfig) int {
	// 指数退避：baseDelay * 2^(attempt-1)
	delay := config.BaseDelayMs << (attempt - 1)

	// 应用最大延迟限制
	if delay > config.MaxDelayMs {
		delay = config.MaxDelayMs
	}

	// 添加抖动
	if config.JitterFactor > 0 {
		jitter := int(float64(delay) * config.JitterFactor)
		delay += randomInt(-jitter, jitter)
	}

	return delay
}

// GetRetryAttempt 获取当前重试尝试次数
func (s *AgentSession) GetRetryAttempt() int {
	s.retryMu.Lock()
	defer s.retryMu.Unlock()
	return s.retryAttempt
}

// ResetRetryAttempt 重置重试尝试次数
func (s *AgentSession) ResetRetryAttempt() {
	s.retryMu.Lock()
	defer s.retryMu.Unlock()
	s.retryAttempt = 0
}

// SetRetryConfig 设置重试配置
func (s *AgentSession) SetRetryConfig(config RetryConfig) {
	// 可以扩展为存储在会话中
}

// IsRetrying 检查是否正在重试
func (s *AgentSession) IsRetrying() bool {
	s.retryMu.Lock()
	defer s.retryMu.Unlock()
	return s.retryAttempt > 0 && s.retryAbortController != nil
}

// RetryStats 重试统计
type RetryStats struct {
	TotalAttempts int    `json:"totalAttempts"`
	SuccessCount  int    `json:"successCount"`
	FailureCount  int    `json:"failureCount"`
	LastError     string `json:"lastError,omitempty"`
}

// GetRetryStats 获取重试统计
func (s *AgentSession) GetRetryStats() *RetryStats {
	return &RetryStats{
		TotalAttempts: s.retryAttempt,
	}
}

// AutoRetryEnabled 获取自动重试启用状态
func (s *AgentSession) AutoRetryEnabled() bool {
	return s.settingsManager.GetRetryEnabled()
}

// SetAutoRetryEnabled 设置自动重试启用状态
func (s *AgentSession) SetAutoRetryEnabled(enabled bool) {
	s.settingsManager.SetRetryEnabled(enabled)
}

// FormatRetryError 格式化重试错误消息
func FormatRetryError(attempt, maxAttempts int, delayMs int, originalError string) string {
	return fmt.Sprintf(
		"Request failed (attempt %d/%d). Retrying in %dms. Error: %s",
		attempt, maxAttempts, delayMs, originalError,
	)
}
