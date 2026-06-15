package ai

import (
	"fmt"
	"sync"
)

// Context AI上下文
type Context struct {
	SystemPrompt string    `json:"systemPrompt,omitempty"`
	Messages     []Message `json:"messages"`
	Tools        []Tool    `json:"tools,omitempty"`
}

// ApiProvider API 提供者接口
type ApiProvider interface {
	GetAPI() ModelApi
	Stream(model Model, ctx Context, opts *StreamOptions) *AssistantMessageEventStream
	StreamSimple(model Model, ctx Context, opts *SimpleStreamOptions) *AssistantMessageEventStream
}

// apiProvider 内部实现
type apiProvider struct {
	api           ModelApi
	stream        func(Model, Context, *StreamOptions) *AssistantMessageEventStream
	streamSimple  func(Model, Context, *SimpleStreamOptions) *AssistantMessageEventStream
}

func (p *apiProvider) GetAPI() ModelApi { return p.api }

func (p *apiProvider) Stream(model Model, ctx Context, opts *StreamOptions) *AssistantMessageEventStream {
	if model.GetAPI() != p.api {
		panic(fmt.Sprintf("Mismatched api: %s expected %s", model.GetAPI(), p.api))
	}
	return p.stream(model, ctx, opts)
}

func (p *apiProvider) StreamSimple(model Model, ctx Context, opts *SimpleStreamOptions) *AssistantMessageEventStream {
	if model.GetAPI() != p.api {
		panic(fmt.Sprintf("Mismatched api: %s expected %s", model.GetAPI(), p.api))
	}
	return p.streamSimple(model, ctx, opts)
}

// NewApiProvider 创建 API 提供者
func NewApiProvider(
	api ModelApi,
	stream func(Model, Context, *StreamOptions) *AssistantMessageEventStream,
	streamSimple func(Model, Context, *SimpleStreamOptions) *AssistantMessageEventStream,
) ApiProvider {
	return &apiProvider{
		api:          api,
		stream:       stream,
		streamSimple: streamSimple,
	}
}

// 确保实现了 ApiProvider 接口
var _ ApiProvider = &apiProvider{}

// ApiProviderRegistry 注册表
type ApiProviderRegistry struct {
	mu       sync.RWMutex
	providers map[ModelApi]ApiProvider
	sources   map[string][]ModelApi  // sourceId -> apis
}

func NewApiProviderRegistry() *ApiProviderRegistry {
	return &ApiProviderRegistry{
		providers: make(map[ModelApi]ApiProvider),
		sources:   make(map[string][]ModelApi),
	}
}

func (r *ApiProviderRegistry) Register(provider ApiProvider, sourceID ...string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	r.providers[provider.GetAPI()] = provider
	
	if len(sourceID) > 0 && sourceID[0] != "" {
		r.sources[sourceID[0]] = append(r.sources[sourceID[0]], provider.GetAPI())
	}
}

func (r *ApiProviderRegistry) Get(api ModelApi) (ApiProvider, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.providers[api]
	return p, ok
}

func (r *ApiProviderRegistry) GetAll() []ApiProvider {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	result := make([]ApiProvider, 0, len(r.providers))
	for _, p := range r.providers {
		result = append(result, p)
	}
	return result
}

func (r *ApiProviderRegistry) UnregisterBySource(sourceID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	if apis, ok := r.sources[sourceID]; ok {
		for _, api := range apis {
			delete(r.providers, api)
		}
		delete(r.sources, sourceID)
	}
}

func (r *ApiProviderRegistry) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	r.providers = make(map[ModelApi]ApiProvider)
	r.sources = make(map[string][]ModelApi)
}

var (
	apiProviderRegistry = NewApiProviderRegistry()
)

// RegisterBuiltInApiProviders 注册内置 API 提供者
func RegisterBuiltInApiProviders() {
	RegisterApiProvider(NewOpenAICompletionsApiProvider(), string(ApiOpenAICompletions))
}

// RegisterApiProvider 注册 API 提供者
func RegisterApiProvider(provider ApiProvider, sourceID ...string) {
	apiProviderRegistry.Register(provider, sourceID...)
}

// GetApiProvider 获取 API 提供者
func GetApiProvider(api ModelApi) (ApiProvider, bool) {
	return apiProviderRegistry.Get(api)
}

// GetApiProviders 获取所有 API 提供者
func GetApiProviders() []ApiProvider {
	return apiProviderRegistry.GetAll()
}

// UnregisterApiProviders 取消注册来自特定源的 API 提供者
func UnregisterApiProviders(sourceID string) {
	apiProviderRegistry.UnregisterBySource(sourceID)
}

// ClearApiProviders 清空所有 API 提供者
func ClearApiProviders() {
	apiProviderRegistry.Clear()
}

// ResolveApiProvider 解析 API 提供者
func ResolveApiProvider(api ModelApi) (ApiProvider, error) {
	provider, ok := GetApiProvider(api)
	if !ok {
		return nil, fmt.Errorf("no API provider registered for api: %s", api)
	}
	return provider, nil
}