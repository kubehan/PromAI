package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/jay-y/pi/pkg/ai"
)

// OpenRouterRouting OpenRouter路由
type OpenRouterRouting struct {
	Only  []string `json:"only,omitempty"`
	Order []string `json:"order,omitempty"`
}

// VercelGatewayRouting Vercel网关路由
type VercelGatewayRouting struct {
	Only  []string `json:"only,omitempty"`
	Order []string `json:"order,omitempty"`
}

// OpenAICompletionsCompat OpenAI完成兼容
type OpenAICompletionsCompat struct {
	SupportsStore                    bool                  `json:"supportsStore,omitempty"`
	SupportsDeveloperRole            bool                  `json:"supportsDeveloperRole,omitempty"`
	SupportsReasoningEffort          bool                  `json:"supportsReasoningEffort,omitempty"`
	SupportsUsageInStreaming         bool                  `json:"supportsUsageInStreaming,omitempty"`
	MaxTokensField                   string                `json:"maxTokensField,omitempty"`
	RequiresToolResultName           bool                  `json:"requiresToolResultName,omitempty"`
	RequiresAssistantAfterToolResult bool                  `json:"requiresAssistantAfterToolResult,omitempty"`
	RequiresThinkingAsText           bool                  `json:"requiresThinkingAsText,omitempty"`
	RequiresMistralToolIds           bool                  `json:"requiresMistralToolIds,omitempty"`
	ThinkingFormat                   string                `json:"thinkingFormat,omitempty"`
	OpenRouterRouting                *OpenRouterRouting    `json:"openRouterRouting,omitempty"`
	VercelGatewayRouting             *VercelGatewayRouting `json:"vercelGatewayRouting,omitempty"`
	SupportsStrictMode               bool                  `json:"supportsStrictMode,omitempty"`
}

// OpenAIResponsesCompat OpenAI响应兼容
type OpenAIResponsesCompat struct{}


// ModelOverride 模型覆盖配置
type ModelOverride struct {
	Name          *string                  `json:"name,omitempty"`
	Reasoning     *bool                    `json:"reasoning,omitempty"`
	Input         []string                 `json:"input,omitempty"` // "text" | "image"
	Cost          *ai.ModelCost            `json:"cost,omitempty"`
	ContextWindow *int                     `json:"contextWindow,omitempty"`
	MaxTokens     *int                     `json:"maxTokens,omitempty"`
	Headers       map[string]string        `json:"headers,omitempty"`
	Compat        *OpenAICompletionsCompat `json:"compat,omitempty"`
}

// ModelDefinition 模型定义
type ModelDefinition struct {
	ID            string                   `json:"id"`
	Name          *string                  `json:"name,omitempty"`
	API           *string                  `json:"api,omitempty"`
	Reasoning     *bool                    `json:"reasoning,omitempty"`
	Input         []string                 `json:"input,omitempty"` // "text" | "image"
	Cost          *ai.ModelCost            `json:"cost,omitempty"`
	ContextWindow *int                     `json:"contextWindow,omitempty"`
	MaxTokens     *int                     `json:"maxTokens,omitempty"`
	Headers       map[string]string        `json:"headers,omitempty"`
	Compat        *OpenAICompletionsCompat `json:"compat,omitempty"`
}

// ProviderConfig 提供商配置
type ProviderConfig struct {
	BaseURL        string                   `json:"baseUrl,omitempty"`
	APIKey         string                   `json:"apiKey,omitempty"`
	API            string                   `json:"api,omitempty"`
	Headers        map[string]string        `json:"headers,omitempty"`
	AuthHeader     *bool                    `json:"authHeader,omitempty"`
	Models         []ModelDefinition        `json:"models,omitempty"`
	ModelOverrides map[string]ModelOverride `json:"modelOverrides,omitempty"`
}

// ModelsConfig 模型配置
type ModelsConfig struct {
	Providers map[string]ProviderConfig `json:"providers"`
}

// ProviderOverride 提供商覆盖配置
type ProviderOverride struct {
	BaseURL string            `json:"baseUrl,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
	APIKey  string            `json:"apiKey,omitempty"`
}

// CustomModelsResult 自定义模型加载结果
type CustomModelsResult struct {
	Models         []*ai.BaseModel
	Overrides      map[string]ProviderOverride
	ModelOverrides map[string]map[string]ModelOverride
	Error          error
}

// ProviderConfigInput 提供商配置输入
type ProviderConfigInput struct {
	BaseURL    string
	APIKey     string
	API        string
	Headers    map[string]string
	AuthHeader *bool
	Models     []ModelDefinitionInput
}

// ModelDefinitionInput 模型定义输入
type ModelDefinitionInput struct {
	ID            string
	Name          string
	API           string
	Reasoning     bool
	Input         []string
	Cost          ai.ModelCost
	ContextWindow int
	MaxTokens     int
	Headers       map[string]string
	Compat        *OpenAICompletionsCompat
}

// ModelRegistry 模型注册表
type ModelRegistry struct {
	authStorage           AuthStorage
	modelsJsonPath        string
	models                []*ai.BaseModel
	customProviderApiKeys map[string]string
	registeredProviders   map[string]ProviderConfigInput
	loadError             error
	mu                    sync.RWMutex
}

func (mr *ModelRegistry) GetModel(provider ModelProvider, modelID string) (ai.Model, bool) {
	mr.mu.RLock()
	defer mr.mu.RUnlock()
	model := mr.Find(string(provider), modelID)
	if model == nil {
		return nil, false
	}
	return model, true
}

func (mr *ModelRegistry) GetModels(provider ModelProvider) []ai.Model {
	mr.mu.RLock()
	defer mr.mu.RUnlock()

	var models []ai.Model
	for _, model := range mr.models {
		if model.Provider == provider {
			models = append(models, model)
		}
	}
	return models
}

// NewModelRegistry 创建新的模型注册表
func NewModelRegistry(authStorage *AuthStorage, modelsJsonPath *string) *ModelRegistry {
	if modelsJsonPath == nil {
		*modelsJsonPath = filepath.Join(GetAgentDir(), "models.json")
	}

	mr := &ModelRegistry{
		authStorage:           *authStorage,
		modelsJsonPath:        *modelsJsonPath,
		models:                make([]*ai.BaseModel, 0),
		customProviderApiKeys: make(map[string]string),
		registeredProviders:   make(map[string]ProviderConfigInput),
	}

	// 设置回退解析器
	if authStorage != nil {
		(*authStorage).SetFallbackResolver(func(provider string) *string {
			if keyConfig, ok := mr.customProviderApiKeys[provider]; ok {
				resolved := ResolveConfigValue(keyConfig)
				if resolved != "" {
					return &resolved
				}
			}
			return nil
		})
	}

	mr.loadModels()
	return mr
}

// Refresh 刷新模型列表
func (mr *ModelRegistry) Refresh() {
	mr.mu.Lock()
	defer mr.mu.Unlock()

	mr.customProviderApiKeys = make(map[string]string)
	mr.loadError = nil
	mr.loadModels()

	for providerName, config := range mr.registeredProviders {
		mr.applyProviderConfig(providerName, config)
	}
}

// GetError 获取加载错误
func (mr *ModelRegistry) GetError() error {
	mr.mu.RLock()
	defer mr.mu.RUnlock()
	return mr.loadError
}

// loadModels 加载模型
func (mr *ModelRegistry) loadModels() {
	var customModels []*ai.BaseModel
	var overrides map[string]ProviderOverride
	var modelOverrides map[string]map[string]ModelOverride
	var loadErr error

	if mr.modelsJsonPath != "" {
		result := mr.loadCustomModels(mr.modelsJsonPath)
		customModels = result.Models
		overrides = result.Overrides
		modelOverrides = result.ModelOverrides
		loadErr = result.Error
	} else {
		overrides = make(map[string]ProviderOverride)
		modelOverrides = make(map[string]map[string]ModelOverride)
	}

	if loadErr != nil {
		mr.loadError = loadErr
		// 即使自定义模型加载失败，也保留内置模型
	}

	builtInModels := mr.loadBuiltInModels(overrides, modelOverrides)
	combined := mr.mergeCustomModels(builtInModels, customModels)

	// 让OAuth提供商修改它们的模型
	for _, oauthProvider := range mr.authStorage.GetOAuthProviders() {
		cred := mr.authStorage.Get(oauthProvider.GetID())
		if cred != nil && cred.Type == "oauth" {
			// 转换为 Model 切片
			aiModels := make([]ai.Model, len(combined))
			for i, m := range combined {
				aiModels[i] = m
			}
			modified := oauthProvider.ModifyModels(aiModels, cred)
			// 转换回 ai.BaseModel 切片
			combined = make([]*ai.BaseModel, len(modified))
			for i, m := range modified {
				if rm, ok := m.(*ai.BaseModel); ok {
					combined[i] = rm
				}
			}
		}
	}

	mr.models = combined
}

// loadBuiltInModels 加载内置模型
func (mr *ModelRegistry) loadBuiltInModels(
	overrides map[string]ProviderOverride,
	modelOverrides map[string]map[string]ModelOverride,
) []*ai.BaseModel {
	// 获取所有内置提供商和模型
	providers := ai.GetApiProviders()
	var models []*ai.BaseModel

	for _, provider := range providers {
		providerModels := mr.GetModels(ai.ModelProvider(provider.GetAPI()))
		providerOverride := overrides[string(provider.GetAPI())]
		perModelOverrides := modelOverrides[string(provider.GetAPI())]

		for _, m := range providerModels {
			model := &ai.BaseModel{
				ID:            m.GetID(),
				Name:          m.GetName(),
				API:           m.GetAPI(),
				Provider:      m.GetProvider(),
				BaseURL:       m.GetBaseURL(),
				Reasoning:     m.GetReasoning(),
				Input:         m.GetInput(),
				Cost:          m.GetCost(),
				ContextWindow: m.GetContextWindow(),
				MaxTokens:     m.GetMaxTokens(),
				Headers:       m.GetHeaders(),
				Compat:        m.GetCompat(),
			}

			// 应用提供商级别的baseUrl/headers覆盖
			if providerOverride.BaseURL != "" || len(providerOverride.Headers) > 0 {
				resolvedHeaders := ResolveHeaders(providerOverride.Headers)
				if resolvedHeaders != nil {
					model.Headers = mergeHeaders(model.Headers, resolvedHeaders)
				}
				if providerOverride.BaseURL != "" {
					model.BaseURL = providerOverride.BaseURL
				}
			}

			// 应用模型级别的覆盖
			if perModelOverrides != nil {
				if modelOverride, ok := perModelOverrides[model.ID]; ok {
					model = applyModelOverride(model, modelOverride)
				}
			}

			models = append(models, model)
		}
	}

	return models
}

// mergeCustomModels 合并自定义模型
func (mr *ModelRegistry) mergeCustomModels(builtInModels []*ai.BaseModel, customModels []*ai.BaseModel) []*ai.BaseModel {
	merged := make([]*ai.BaseModel, len(builtInModels))
	copy(merged, builtInModels)

	for _, customModel := range customModels {
		found := false
		for i, m := range merged {
			if m.Provider == customModel.Provider && m.ID == customModel.ID {
				merged[i] = customModel
				found = true
				break
			}
		}
		if !found {
			merged = append(merged, customModel)
		}
	}

	return merged
}

// applyModelOverride 应用模型覆盖
func applyModelOverride(model *ai.BaseModel, override ModelOverride) *ai.BaseModel {
	result := &ai.BaseModel{
		ID:            model.ID,
		Name:          model.Name,
		API:           model.API,
		Provider:      model.Provider,
		BaseURL:       model.BaseURL,
		Reasoning:     model.Reasoning,
		Input:         model.Input,
		Cost:          model.Cost,
		ContextWindow: model.ContextWindow,
		MaxTokens:     model.MaxTokens,
		Headers:       model.Headers,
		Compat:        model.Compat,
	}

	// 简单字段覆盖
	if override.Name != nil {
		result.Name = *override.Name
	}
	if override.Reasoning != nil {
		result.Reasoning = *override.Reasoning
	}
	if override.Input != nil {
		result.Input = override.Input
	}
	if override.ContextWindow != nil {
		result.ContextWindow = *override.ContextWindow
	}
	if override.MaxTokens != nil {
		result.MaxTokens = *override.MaxTokens
	}

	// 合并成本
	if override.Cost != nil {
		result.Cost = ai.ModelCost{
			Input:      override.Cost.Input,
			Output:     override.Cost.Output,
			CacheRead:  override.Cost.CacheRead,
			CacheWrite: override.Cost.CacheWrite,
		}
	}

	// 合并头部
	if override.Headers != nil {
		resolvedHeaders := ResolveHeaders(override.Headers)
		if resolvedHeaders != nil {
			result.Headers = mergeHeaders(result.Headers, resolvedHeaders)
		}
	}

	// 合并兼容配置
	if override.Compat != nil {
		result.Compat = mergeCompat(result.Compat, override.Compat)
	}

	return result
}

// mergeCompat 合并兼容配置
func mergeCompat(baseCompat any, overrideCompat *OpenAICompletionsCompat) any {
	if overrideCompat == nil {
		return baseCompat
	}
	return overrideCompat
}

// mergeHeaders 合并头部
func mergeHeaders(base, override map[string]string) map[string]string {
	if override == nil {
		return base
	}
	if base == nil {
		return override
	}
	merged := make(map[string]string)
	for k, v := range base {
		merged[k] = v
	}
	for k, v := range override {
		merged[k] = v
	}
	return merged
}

// loadCustomModels 加载自定义模型
func (mr *ModelRegistry) loadCustomModels(modelsJsonPath string) CustomModelsResult {
	if _, err := os.Stat(modelsJsonPath); os.IsNotExist(err) {
		return CustomModelsResult{
			Models:         make([]*ai.BaseModel, 0),
			Overrides:      make(map[string]ProviderOverride),
			ModelOverrides: make(map[string]map[string]ModelOverride),
			Error:          nil,
		}
	}

	content, err := os.ReadFile(modelsJsonPath)
	if err != nil {
		return CustomModelsResult{
			Models:         make([]*ai.BaseModel, 0),
			Overrides:      make(map[string]ProviderOverride),
			ModelOverrides: make(map[string]map[string]ModelOverride),
			Error:          fmt.Errorf("failed to read models.json: %w", err),
		}
	}

	var config ModelsConfig
	if err := json.Unmarshal(content, &config); err != nil {
		return CustomModelsResult{
			Models:         make([]*ai.BaseModel, 0),
			Overrides:      make(map[string]ProviderOverride),
			ModelOverrides: make(map[string]map[string]ModelOverride),
			Error:          fmt.Errorf("failed to parse models.json: %w", err),
		}
	}

	// 验证配置
	if err := mr.validateConfig(&config); err != nil {
		return CustomModelsResult{
			Models:         make([]*ai.BaseModel, 0),
			Overrides:      make(map[string]ProviderOverride),
			ModelOverrides: make(map[string]map[string]ModelOverride),
			Error:          err,
		}
	}

	overrides := make(map[string]ProviderOverride)
	modelOverrides := make(map[string]map[string]ModelOverride)

	for providerName, providerConfig := range config.Providers {
		// 应用提供商级别的baseUrl/headers/apiKey覆盖
		if providerConfig.BaseURL != "" || len(providerConfig.Headers) > 0 || providerConfig.APIKey != "" {
			overrides[providerName] = ProviderOverride{
				BaseURL: providerConfig.BaseURL,
				Headers: providerConfig.Headers,
				APIKey:  providerConfig.APIKey,
			}
		}

		// 存储API key用于回退解析器
		if providerConfig.APIKey != "" {
			mr.customProviderApiKeys[providerName] = providerConfig.APIKey
		}

		if len(providerConfig.ModelOverrides) > 0 {
			modelOverrides[providerName] = providerConfig.ModelOverrides
		}
	}

	models := mr.parseModels(&config)

	return CustomModelsResult{
		Models:         models,
		Overrides:      overrides,
		ModelOverrides: modelOverrides,
		Error:          nil,
	}
}

// validateConfig 验证配置
func (mr *ModelRegistry) validateConfig(config *ModelsConfig) error {
	for providerName, providerConfig := range config.Providers {
		hasProviderAPI := providerConfig.API != ""
		models := providerConfig.Models
		hasModelOverrides := len(providerConfig.ModelOverrides) > 0

		if len(models) == 0 {
			// 仅覆盖配置：需要baseUrl或modelOverrides（或两者）
			if providerConfig.BaseURL == "" && !hasModelOverrides {
				return fmt.Errorf("provider %s: must specify \"baseUrl\", \"modelOverrides\", or \"models\"", providerName)
			}
		} else {
			// 自定义模型需要endpoint + auth
			if providerConfig.BaseURL == "" {
				return fmt.Errorf("provider %s: \"baseUrl\" is required when defining custom models", providerName)
			}
			if providerConfig.APIKey == "" {
				return fmt.Errorf("provider %s: \"apiKey\" is required when defining custom models", providerName)
			}
		}

		for _, modelDef := range models {
			hasModelAPI := modelDef.API != nil && *modelDef.API != ""

			if !hasProviderAPI && !hasModelAPI {
				return fmt.Errorf("provider %s, model %s: no \"api\" specified. Set at provider or model level", providerName, modelDef.ID)
			}

			if modelDef.ID == "" {
				return fmt.Errorf("provider %s: model missing \"id\"", providerName)
			}

			if modelDef.ContextWindow != nil && *modelDef.ContextWindow <= 0 {
				return fmt.Errorf("provider %s, model %s: invalid contextWindow", providerName, modelDef.ID)
			}
			if modelDef.MaxTokens != nil && *modelDef.MaxTokens <= 0 {
				return fmt.Errorf("provider %s, model %s: invalid maxTokens", providerName, modelDef.ID)
			}
		}
	}

	return nil
}

// parseModels 解析模型
func (mr *ModelRegistry) parseModels(config *ModelsConfig) []*ai.BaseModel {
	var models []*ai.BaseModel

	for providerName, providerConfig := range config.Providers {
		modelDefs := providerConfig.Models
		if len(modelDefs) == 0 {
			continue // 仅覆盖，没有自定义模型
		}

		// 存储API key用于认证解析
		if providerConfig.APIKey != "" {
			mr.customProviderApiKeys[providerName] = providerConfig.APIKey
		}

		for _, modelDef := range modelDefs {
			api := providerConfig.API
			if modelDef.API != nil && *modelDef.API != "" {
				api = *modelDef.API
			}
			if api == "" {
				continue
			}

			// 合并头部：提供商头部是基础，模型头部覆盖
			providerHeaders := ResolveHeaders(providerConfig.Headers)
			modelHeaders := ResolveHeaders(modelDef.Headers)
			headers := mergeHeaders(providerHeaders, modelHeaders)

			// 如果authHeader为true，添加Authorization头部
			if providerConfig.AuthHeader != nil && *providerConfig.AuthHeader && providerConfig.APIKey != "" {
				resolvedKey := ResolveConfigValue(providerConfig.APIKey)
				if resolvedKey != "" {
					headers = mergeHeaders(headers, map[string]string{"Authorization": "Bearer " + resolvedKey})
				}
			}

			// 应用默认值
			name := modelDef.ID
			if modelDef.Name != nil {
				name = *modelDef.Name
			}

			reasoning := false
			if modelDef.Reasoning != nil {
				reasoning = *modelDef.Reasoning
			}

			input := []string{"text"}
			if modelDef.Input != nil {
				input = modelDef.Input
			}

			cost := ai.ModelCost{Input: 0, Output: 0, CacheRead: 0, CacheWrite: 0}
			if modelDef.Cost != nil {
				cost = ai.ModelCost{
					Input:      modelDef.Cost.Input,
					Output:     modelDef.Cost.Output,
					CacheRead:  modelDef.Cost.CacheRead,
					CacheWrite: modelDef.Cost.CacheWrite,
				}
			}

			contextWindow := 128000
			if modelDef.ContextWindow != nil {
				contextWindow = *modelDef.ContextWindow
			}

			maxTokens := 16384
			if modelDef.MaxTokens != nil {
				maxTokens = *modelDef.MaxTokens
			}

			models = append(models, &ai.BaseModel{
				ID:            modelDef.ID,
				Name:          name,
				API:           ModelApi(api),
				Provider:      ModelProvider(providerName),
				BaseURL:       providerConfig.BaseURL,
				Reasoning:     reasoning,
				Input:         input,
				Cost:          cost,
				ContextWindow: contextWindow,
				MaxTokens:     maxTokens,
				Headers:       headers,
				Compat:        modelDef.Compat,
			})
		}
	}

	return models
}

// GetAll 获取所有模型
func (mr *ModelRegistry) GetAll() []ai.Model {
	mr.mu.RLock()
	defer mr.mu.RUnlock()

	result := make([]ai.Model, len(mr.models))
	for i, m := range mr.models {
		result[i] = m
	}
	return result
}

// GetAvailable 获取可用模型（有认证的）
func (mr *ModelRegistry) GetAvailable() ([]ai.Model, error) {
	mr.mu.RLock()
	defer mr.mu.RUnlock()

	var available []ai.Model
	for _, m := range mr.models {
		if mr.authStorage.HasAuth(string(m.Provider)) {
			available = append(available, m)
		}
	}
	return available, nil
}

// Find 查找模型
func (mr *ModelRegistry) Find(provider string, modelId string) ai.Model {
	mr.mu.RLock()
	defer mr.mu.RUnlock()

	for _, m := range mr.models {
		if string(m.Provider) == provider && m.ID == modelId {
			return m
		}
	}
	return nil
}

// GetApiKey 获取模型的API key
func (mr *ModelRegistry) GetApiKey(model ai.Model) (string, error) {
	if model.GetAPIKey() != "" {
		return model.GetAPIKey(), nil
	} else if mr.authStorage != nil {
		return mr.authStorage.GetApiKey(string(model.GetProvider()))
	}
	return "", nil
}

// GetApiKeyForProvider 获取提供商的API key
func (mr *ModelRegistry) GetApiKeyForProvider(provider string) (string, error) {
	return mr.authStorage.GetApiKey(provider)
}

// IsUsingOAuth 检查模型是否使用OAuth
func (mr *ModelRegistry) IsUsingOAuth(model ai.Model) bool {
	cred := mr.authStorage.Get(string(model.GetProvider()))
	return cred != nil && cred.Type == "oauth"
}

// RegisterProvider 注册提供商
func (mr *ModelRegistry) RegisterProvider(providerName string, config ProviderConfigInput) error {
	mr.mu.Lock()
	defer mr.mu.Unlock()

	mr.registeredProviders[providerName] = config
	return mr.applyProviderConfig(providerName, config)
}

// applyProviderConfig 应用提供商配置
func (mr *ModelRegistry) applyProviderConfig(providerName string, config ProviderConfigInput) error {
	// 存储API key用于认证解析
	if config.APIKey != "" {
		mr.customProviderApiKeys[providerName] = config.APIKey
	}

	if len(config.Models) > 0 {
		// 完全替换：移除该提供商的所有现有模型
		var filtered []*ai.BaseModel
		for _, m := range mr.models {
			if string(m.Provider) != providerName {
				filtered = append(filtered, m)
			}
		}
		mr.models = filtered

		// 验证必需字段
		if config.BaseURL == "" {
			return fmt.Errorf("provider %s: \"baseUrl\" is required when defining models", providerName)
		}
		if config.APIKey == "" {
			return fmt.Errorf("provider %s: \"apiKey\" is required when defining models", providerName)
		}

		// 解析并添加新模型
		for _, modelDef := range config.Models {
			api := modelDef.API
			if api == "" {
				return fmt.Errorf("provider %s, model %s: no \"api\" specified", providerName, modelDef.ID)
			}

			// 合并头部
			providerHeaders := ResolveHeaders(config.Headers)
			modelHeaders := ResolveHeaders(modelDef.Headers)
			headers := mergeHeaders(providerHeaders, modelHeaders)

			// 如果authHeader为true，添加Authorization头部
			if config.AuthHeader != nil && *config.AuthHeader && config.APIKey != "" {
				resolvedKey := ResolveConfigValue(config.APIKey)
				if resolvedKey != "" {
					headers = mergeHeaders(headers, map[string]string{"Authorization": "Bearer " + resolvedKey})
				}
			}

			mr.models = append(mr.models, &ai.BaseModel{
				ID:            modelDef.ID,
				Name:          modelDef.Name,
				API:           ModelApi(api),
				Provider:      ModelProvider(providerName),
				BaseURL:       config.BaseURL,
				Reasoning:     modelDef.Reasoning,
				Input:         modelDef.Input,
				Cost:          modelDef.Cost,
				ContextWindow: modelDef.ContextWindow,
				MaxTokens:     modelDef.MaxTokens,
				Headers:       headers,
				Compat:        modelDef.Compat,
			})
		}
	} else if config.BaseURL != "" {
		// 仅覆盖：更新现有模型的baseUrl/headers
		resolvedHeaders := ResolveHeaders(config.Headers)
		for i, m := range mr.models {
			if string(m.Provider) != providerName {
				continue
			}
			if config.BaseURL != "" {
				mr.models[i].BaseURL = config.BaseURL
			}
			if resolvedHeaders != nil {
				mr.models[i].Headers = mergeHeaders(m.Headers, resolvedHeaders)
			}
		}
	}

	return nil
}