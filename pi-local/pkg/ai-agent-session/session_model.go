package session

import (
	"context"
	"sync"

	"github.com/jay-y/pi/pkg/ai"
)

// 错误定义
var (
	ErrNoApiKey = &ModelError{Message: "No API key found for the model"}
)

// ModelError 模型错误
type ModelError struct {
	Message string
}

func (e *ModelError) Error() string {
	return e.Message
}

// modelMu 模型互斥锁
var modelMu sync.Mutex

// ScopedModel 范围模型
type ScopedModel struct {
	Model         ai.Model         `json:"model"`
	ThinkingLevel ai.ThinkingLevel `json:"thinkingLevel"`
}


// SetModel 设置模型
func (s *AgentSession) SetModel(ctx context.Context, model ai.Model) error {
	apiKey, err := s.modelRegistry.GetApiKey(model)
	if err != nil {
		return err
	}
	if apiKey == "" {
		return ErrNoApiKey
	}

	previousModel := s.agent.GetState().Model
	s.agent.SetModel(model)
	s.sessionManager.AppendModelChange(string(model.GetProvider()), model.GetID())
	s.settingsManager.SetDefaultModelAndProvider(string(model.GetProvider()), model.GetID())

	// 重新调整思考级别
	s.SetThinkingLevel(s.agent.GetState().ThinkingLevel)

	// 发送模型选择事件
	s.emitModelSelect(model, previousModel, "set")

	return nil
}

// ModelCycleResult 模型循环结果
type ModelCycleResult struct {
	Model         ai.Model         `json:"model"`
	ThinkingLevel ThinkingLevel `json:"thinkingLevel"`
	IsScoped      bool          `json:"isScoped"`
}

// CycleModel 循环切换模型
func (s *AgentSession) CycleModel(ctx context.Context, direction string) (*ModelCycleResult, error) {
	if len(s.scopedModels) > 0 {
		return s.cycleScopedModel(ctx, direction)
	}
	return s.cycleAvailableModel(ctx, direction)
}

// cycleScopedModel 循环范围模型
func (s *AgentSession) cycleScopedModel(ctx context.Context, direction string) (*ModelCycleResult, error) {
	scopedModels, err := s.getScopedModelsWithApiKey()
	if err != nil {
		return nil, err
	}

	if len(scopedModels) <= 1 {
		return nil, nil
	}

	currentModel := s.agent.GetState().Model
	currentIndex := 0

	// 找到当前模型索引
	for i, sm := range scopedModels {
		if modelsAreEqual(sm.Model, currentModel) {
			currentIndex = i
			break
		}
	}

	// 计算下一个索引
	var nextIndex int
	if direction == "forward" {
		nextIndex = (currentIndex + 1) % len(scopedModels)
	} else {
		nextIndex = (currentIndex - 1 + len(scopedModels)) % len(scopedModels)
	}

	next := scopedModels[nextIndex]

	// 应用模型
	s.agent.SetModel(next.Model)
	s.sessionManager.AppendModelChange(string(next.Model.GetProvider()), next.Model.GetID())
	s.settingsManager.SetDefaultModelAndProvider(string(next.Model.GetProvider()), next.Model.GetID())

	// 应用思考级别
	s.SetThinkingLevel(next.ThinkingLevel)

	// 发送事件
	s.emitModelSelect(next.Model, currentModel, "cycle")

	return &ModelCycleResult{
		Model:         next.Model,
		ThinkingLevel: s.agent.GetState().ThinkingLevel,
		IsScoped:      true,
	}, nil
}

// cycleAvailableModel 循环可用模型
func (s *AgentSession) cycleAvailableModel(ctx context.Context, direction string) (*ModelCycleResult, error) {
	availableModels, err := s.modelRegistry.GetAvailable()
	if err != nil {
		return nil, err
	}

	if len(availableModels) <= 1 {
		return nil, nil
	}

	currentModel := s.agent.GetState().Model
	currentIndex := 0

	// 找到当前模型索引
	for i, m := range availableModels {
		if modelsAreEqual(m, currentModel) {
			currentIndex = i
			break
		}
	}

	// 计算下一个索引
	var nextIndex int
	if direction == "forward" {
		nextIndex = (currentIndex + 1) % len(availableModels)
	} else {
		nextIndex = (currentIndex - 1 + len(availableModels)) % len(availableModels)
	}

	nextModel := availableModels[nextIndex]

	apiKey, err := s.modelRegistry.GetApiKey(nextModel)
	if err != nil {
		return nil, err
	}
	if apiKey == "" {
		return nil, ErrNoApiKey
	}

	s.agent.SetModel(nextModel)
	s.sessionManager.AppendModelChange(string(nextModel.GetProvider()), nextModel.GetID())
	s.settingsManager.SetDefaultModelAndProvider(string(nextModel.GetProvider()), nextModel.GetID())

	// 重新调整思考级别
	s.SetThinkingLevel(s.agent.GetState().ThinkingLevel)

	// 发送事件
	s.emitModelSelect(nextModel, currentModel, "cycle")

	return &ModelCycleResult{
		Model:         nextModel,
		ThinkingLevel: s.agent.GetState().ThinkingLevel,
		IsScoped:      false,
	}, nil
}

// getScopedModelsWithApiKey 获取有 API Key 的范围模型
func (s *AgentSession) getScopedModelsWithApiKey() ([]ScopedModel, error) {
	apiKeysByProvider := make(map[string]string)
	var result []ScopedModel

	for _, scoped := range s.scopedModels {
		provider := string(scoped.Model.GetProvider())

		var apiKey string
		var err error

		if key, exists := apiKeysByProvider[provider]; exists {
			apiKey = key
		} else {
			apiKey, err = s.modelRegistry.GetApiKeyForProvider(provider)
			if err == nil && apiKey != "" {
				apiKeysByProvider[provider] = apiKey
			}
		}

		if apiKey != "" {
			result = append(result, scoped)
		}
	}

	return result, nil
}

// SetThinkingLevel 设置思考级别
func (s *AgentSession) SetThinkingLevel(level ThinkingLevel) {
	availableLevels := s.GetAvailableThinkingLevels()
	effectiveLevel := level

	// 检查级别是否可用
	available := false
	for _, l := range availableLevels {
		if l == level {
			available = true
			break
		}
	}

	if !available {
		effectiveLevel = s.clampThinkingLevel(level, availableLevels)
	}

	// 只有在真正改变时才持久化
	isChanging := effectiveLevel != s.agent.GetState().ThinkingLevel

	s.agent.SetThinkingLevel(effectiveLevel)

	if isChanging {
		s.sessionManager.AppendThinkingLevelChangeFromLevel(effectiveLevel)
		s.settingsManager.SetDefaultThinkingLevel(effectiveLevel)
	}
}

// CycleThinkingLevel 循环切换思考级别
func (s *AgentSession) CycleThinkingLevel() ThinkingLevel {
	if !s.SupportsThinking() {
		return ThinkingLevelOff
	}

	levels := s.GetAvailableThinkingLevels()
	currentLevel := s.agent.GetState().ThinkingLevel

	currentIndex := 0
	for i, l := range levels {
		if l == currentLevel {
			currentIndex = i
			break
		}
	}

	nextIndex := (currentIndex + 1) % len(levels)
	nextLevel := levels[nextIndex]

	s.SetThinkingLevel(nextLevel)
	return nextLevel
}

// GetAvailableThinkingLevels 获取可用的思考级别
func (s *AgentSession) GetAvailableThinkingLevels() []ThinkingLevel {
	if !s.SupportsThinking() {
		return []ThinkingLevel{ThinkingLevelOff}
	}

	if s.SupportsXhighThinking() {
		return ThinkingLevelsWithXHigh
	}
	return ThinkingLevels
}

// SupportsThinking 检查是否支持思考
func (s *AgentSession) SupportsThinking() bool {
	model := s.agent.GetState().Model
	if model == nil {
		return false
	}
	return model.GetReasoning()
}

// SupportsXhighThinking 检查是否支持 xhigh 思考级别
func (s *AgentSession) SupportsXhighThinking() bool {
	model := s.agent.GetState().Model
	if model == nil {
		return false
	}
	return supportsXhigh(model)
}

// clampThinkingLevel 调整思考级别到可用范围
func (s *AgentSession) clampThinkingLevel(level ThinkingLevel, availableLevels []ThinkingLevel) ThinkingLevel {
	ordered := ThinkingLevelsWithXHigh
	available := make(map[ThinkingLevel]bool)
	for _, l := range availableLevels {
		available[l] = true
	}

	// 找到请求级别的索引
	requestedIndex := -1
	for i, l := range ordered {
		if l == level {
			requestedIndex = i
			break
		}
	}

	if requestedIndex == -1 {
		if len(availableLevels) > 0 {
			return availableLevels[0]
		}
		return ThinkingLevelOff
	}

	// 从请求级别开始向上查找
	for i := requestedIndex; i < len(ordered); i++ {
		if available[ordered[i]] {
			return ordered[i]
		}
	}

	// 从请求级别开始向下查找
	for i := requestedIndex - 1; i >= 0; i-- {
		if available[ordered[i]] {
			return ordered[i]
		}
	}

	if len(availableLevels) > 0 {
		return availableLevels[0]
	}
	return ThinkingLevelOff
}

// emitModelSelect 发送模型选择事件
func (s *AgentSession) emitModelSelect(nextModel, previousModel ai.Model, source string) {
	if s.extensionRunner == nil {
		return
	}
	if modelsAreEqual(previousModel, nextModel) {
		return
	}

	// TODO: 发送扩展事件
}

// modelsAreEqual 检查模型是否相等
func modelsAreEqual(a, b ai.Model) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return a.GetProvider() == b.GetProvider() && a.GetID() == b.GetID()
}

// supportsXhigh 检查模型是否支持 xhigh 思考级别
func supportsXhigh(model ai.Model) bool {
	// 检查模型 ID 是否在支持 xhigh 的列表中
	xhighModels := []string{
		"claude-3-7-sonnet",
		"claude-3.7-sonnet",
		"o3-mini",
		"o1",
	}

	modelId := model.GetID()
	for _, m := range xhighModels {
		if containsIgnoreCase(modelId, m) {
			return true
		}
	}

	return false
}

// SetSteeringMode 设置 steering 模式
func (s *AgentSession) SetSteeringMode(mode string) {
	s.agent.SetSteeringMode(mode)
	s.settingsManager.SetSteeringMode(mode)
}

// SetFollowUpMode 设置 follow-up 模式
func (s *AgentSession) SetFollowUpMode(mode string) {
	s.agent.SetFollowUpMode(mode)
	s.settingsManager.SetFollowUpMode(mode)
}