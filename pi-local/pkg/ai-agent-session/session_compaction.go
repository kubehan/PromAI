package session

import (
	"context"
	"sync"

	"github.com/jay-y/pi/pkg/ai"
)

// 错误定义
var (
	ErrNoModelSelected     = &CompactionError{Message: "No model selected"}
	ErrNothingToCompact    = &CompactionError{Message: "Nothing to compact (session too small)"}
	ErrAlreadyCompacted    = &CompactionError{Message: "Already compacted"}
	ErrCompactionCancelled = &CompactionError{Message: "Compaction cancelled"}
)

// CompactionError 压缩错误
type CompactionError struct {
	Message string
}

func (e *CompactionError) Error() string {
	return e.Message
}

// compactionMu 压缩互斥锁
var compactionMu sync.Mutex

// CompactionResult 压缩结果
type CompactionResult struct {
	Summary          string `json:"summary"`
	FirstKeptEntryId string `json:"firstKeptEntryId"`
	TokensBefore     int    `json:"tokensBefore"`
	Details          any    `json:"details,omitempty"`
}

// CompactionPreparation 压缩准备
type CompactionPreparation struct {
	EntriesToCompact []SessionEntry `json:"entriesToCompact"`
	FirstKeptEntryId string         `json:"firstKeptEntryId"`
	TokensBefore     int            `json:"tokensBefore"`
}

// checkCompaction 检查是否需要压缩并执行
func (s *AgentSession) checkCompaction(assistantMessage *ai.AssistantMessage, skipAbortedCheck bool) {
	settings := s.settingsManager.GetCompactionSettings()
	if settings.Enabled == nil || !*settings.Enabled {
		return
	}

	// 跳过被中止的消息（用户取消）
	if skipAbortedCheck && assistantMessage.StopReason == StopReasonAborted {
		return
	}

	contextWindow := 0
	if s.agent.GetState().Model != nil {
		contextWindow = s.agent.GetState().Model.GetContextWindow()
	}

	// 检查是否是上下文溢出错误
	if s.isContextOverflow(assistantMessage, contextWindow) {
		// 移除错误消息
		messages := s.agent.GetState().Messages
		if len(messages) > 0 {
			if lastMsg, ok := messages[len(messages)-1].(*ai.AssistantMessage); ok {
				if lastMsg.StopReason == StopReasonError {
					s.agent.ReplaceMessages(messages[:len(messages)-1])
				}
			}
		}
		s.runAutoCompaction("overflow", true)
		return
	}

	// 检查阈值
	if assistantMessage.StopReason == StopReasonError {
		return
	}

	contextTokens := calculateContextTokens(assistantMessage.Usage)
	if s.shouldCompact(contextTokens, contextWindow, &settings) {
		s.runAutoCompaction("threshold", false)
	}
}

// isContextOverflow 检查是否是上下文溢出
func (s *AgentSession) isContextOverflow(msg *ai.AssistantMessage, contextWindow int) bool {
	if msg.StopReason != StopReasonError {
		return false
	}

	// 检查错误消息是否包含溢出关键词
	overflowPatterns := []string{
		"context length exceeded",
		"maximum context length",
		"token limit exceeded",
		"context too long",
		"reduce the length",
	}

	for _, pattern := range overflowPatterns {
		if containsIgnoreCase(msg.ErrorMessage, pattern) {
			return true
		}
	}

	// 检查 token 使用是否超过上下文窗口
	if contextWindow > 0 && msg.Usage.TotalTokens > contextWindow {
		return true
	}

	return false
}

// calculateContextTokens 计算上下文 token 数
func calculateContextTokens(usage ai.Usage) int {
	return usage.Input + usage.CacheRead
}

// shouldCompact 检查是否应该压缩
func (s *AgentSession) shouldCompact(contextTokens, contextWindow int, settings *CompactionSettings) bool {
	if contextWindow <= 0 {
		return false
	}

	// 使用默认阈值 80%
	thresholdPercent := 80.0
	if settings.ReserveTokens != nil && *settings.ReserveTokens > 0 {
		thresholdPercent = float64(contextWindow-*settings.ReserveTokens) / float64(contextWindow) * 100
	}

	percentUsed := float64(contextTokens) / float64(contextWindow) * 100
	return percentUsed >= thresholdPercent
}

// runAutoCompaction 运行自动压缩
func (s *AgentSession) runAutoCompaction(reason string, willRetry bool) {
	settings := s.settingsManager.GetCompactionSettings()

	s.emit(&AutoCompactionStartEvent{
		Type:   "auto_compaction_start",
		Reason: reason,
	})

	ctx, cancel := context.WithCancel(context.Background())
	s.autoCompactionAbortController = cancel

	defer func() {
		cancel()
		s.autoCompactionAbortController = nil
	}()

	// 检查模型和 API Key
	model := s.agent.GetState().Model
	if model == nil {
		s.emit(&AutoCompactionEndEvent{
			Type:      "auto_compaction_end",
			Aborted:   false,
			WillRetry: false,
		})
		return
	}

	apiKey, err := s.modelRegistry.GetApiKey(model)
	if err != nil || apiKey == "" {
		s.emit(&AutoCompactionEndEvent{
			Type:      "auto_compaction_end",
			Aborted:   false,
			WillRetry: false,
		})
		return
	}

	// 准备压缩
	pathEntries := s.sessionManager.GetBranch()
	preparation := s.prepareCompaction(pathEntries, &settings)
	if preparation == nil {
		s.emit(&AutoCompactionEndEvent{
			Type:      "auto_compaction_end",
			Aborted:   false,
			WillRetry: false,
		})
		return
	}

	// 执行压缩
	result, err := s.executeCompaction(ctx, preparation, model, apiKey, nil)
	if err != nil {
		errorMsg := "compaction failed"
		if reason == "overflow" {
			errorMsg = "Context overflow recovery failed: " + err.Error()
		} else {
			errorMsg = "Auto-compaction failed: " + err.Error()
		}
		s.emit(&AutoCompactionEndEvent{
			Type:         "auto_compaction_end",
			Aborted:      false,
			WillRetry:    false,
			ErrorMessage: errorMsg,
		})
		return
	}

	// 检查是否被中止
	select {
	case <-ctx.Done():
		s.emit(&AutoCompactionEndEvent{
			Type:      "auto_compaction_end",
			Aborted:   true,
			WillRetry: false,
		})
		return
	default:
	}

	// 保存压缩结果
	s.sessionManager.AppendCompaction(
		result.Summary,
		result.FirstKeptEntryId,
		result.TokensBefore,
		result.Details,
		false,
	)

	// 更新消息
	sessionContext := s.sessionManager.BuildSessionContext()
	s.agent.ReplaceMessages(sessionContext.Messages)

	s.emit(&AutoCompactionEndEvent{
		Type:      "auto_compaction_end",
		Result:    result,
		Aborted:   false,
		WillRetry: willRetry,
	})

	// 如果需要重试，触发继续
	if willRetry {
		go func() {
			messages := s.agent.GetState().Messages
			if len(messages) > 0 {
				if lastMsg, ok := messages[len(messages)-1].(*ai.AssistantMessage); ok {
					if lastMsg.StopReason == StopReasonError {
						s.agent.ReplaceMessages(messages[:len(messages)-1])
					}
				}
			}
			s.agent.Continue(context.Background())
		}()
	} else if s.agent.HasQueuedMessages() {
		go func() {
			s.agent.Continue(context.Background())
		}()
	}
}

// prepareCompaction 准备压缩
func (s *AgentSession) prepareCompaction(entries []SessionEntry, settings *CompactionSettings) *CompactionPreparation {
	// 默认最小消息数
	minMessages := 4
	if len(entries) < minMessages {
		return nil
	}

	// 找到最后一个压缩条目
	lastCompactionIndex := -1
	for i := len(entries) - 1; i >= 0; i-- {
		if entries[i].GetType() == "compaction" {
			lastCompactionIndex = i
			break
		}
	}

	// 确定要压缩的条目
	var entriesToCompact []SessionEntry
	var firstKeptEntryId string

	if lastCompactionIndex >= 0 {
		// 从最后一个压缩之后开始
		entriesToCompact = make([]SessionEntry, 0)
		for i := lastCompactionIndex + 1; i < len(entries); i++ {
			entriesToCompact = append(entriesToCompact, entries[i])
		}
		if lastCompactionIndex < len(entries) {
			firstKeptEntryId = entries[lastCompactionIndex].GetID()
		}
	} else {
		// 从头开始压缩
		entriesToCompact = entries
	}

	if len(entriesToCompact) == 0 {
		return nil
	}

	return &CompactionPreparation{
		EntriesToCompact: entriesToCompact,
		FirstKeptEntryId: firstKeptEntryId,
		TokensBefore:     0, // TODO: 实际计算
	}
}

// executeCompaction 执行压缩
func (s *AgentSession) executeCompaction(ctx context.Context, preparation *CompactionPreparation, model ai.Model, apiKey string, customInstructions *string) (*CompactionResult, error) {
	// TODO: 实现实际的压缩逻辑
	// 这需要调用 LLM 来生成摘要

	return &CompactionResult{
		Summary:          "Session context has been compacted.",
		FirstKeptEntryId: preparation.FirstKeptEntryId,
		TokensBefore:     preparation.TokensBefore,
	}, nil
}

// Compact 手动压缩
func (s *AgentSession) Compact(ctx context.Context, customInstructions string) (*CompactionResult, error) {
	s.DisconnectFromAgent()
	defer s.ReconnectToAgent()

	_, cancel := context.WithCancel(ctx)
	s.compactionAbortController = cancel
	defer func() {
		cancel()
		s.compactionAbortController = nil
	}()

	model := s.agent.GetState().Model
	if model == nil {
		return nil, ErrNoModelSelected
	}

	apiKey, err := s.modelRegistry.GetApiKey(model)
	if err != nil {
		return nil, err
	}

	pathEntries := s.sessionManager.GetBranch()
	settings := s.settingsManager.GetCompactionSettings()

	preparation := s.prepareCompaction(pathEntries, &settings)
	if preparation == nil {
		return nil, ErrNothingToCompact
	}

	result, err := s.executeCompaction(ctx, preparation, model, apiKey, &customInstructions)
	if err != nil {
		return nil, err
	}

	s.sessionManager.AppendCompaction(
		result.Summary,
		result.FirstKeptEntryId,
		result.TokensBefore,
		result.Details,
		false,
	)

	sessionContext := s.sessionManager.BuildSessionContext()
	s.agent.ReplaceMessages(sessionContext.Messages)

	return result, nil
}

// AbortCompaction 中止压缩
func (s *AgentSession) AbortCompaction() {
	if s.compactionAbortController != nil {
		s.compactionAbortController()
	}
	if s.autoCompactionAbortController != nil {
		s.autoCompactionAbortController()
	}
}

// AbortBranchSummary 中止分支摘要
func (s *AgentSession) AbortBranchSummary() {
	if s.branchSummaryAbortController != nil {
		s.branchSummaryAbortController()
	}
}

// SetAutoCompactionEnabled 设置自动压缩开关
func (s *AgentSession) SetAutoCompactionEnabled(enabled bool) {
	s.settingsManager.SetCompactionEnabled(enabled)
}

// AutoCompactionEnabled 获取自动压缩开关
func (s *AgentSession) AutoCompactionEnabled() bool {
	return s.settingsManager.GetCompactionEnabled()
}

// GetCompactionSettings 获取压缩设置
func (s *AgentSession) GetCompactionSettings() *CompactionSettings {
	settings := s.settingsManager.GetCompactionSettings()
	return &settings
}