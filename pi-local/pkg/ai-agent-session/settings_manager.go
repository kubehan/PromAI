package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// CompactionSettings 压缩设置
type CompactionSettings struct {
	Enabled         *bool `json:"enabled,omitempty"`         // default: true
	ReserveTokens   *int  `json:"reserveTokens,omitempty"`   // default: 16384
	KeepRecentTokens *int `json:"keepRecentTokens,omitempty"` // default: 20000
}

// BranchSummarySettings 分支摘要设置
type BranchSummarySettings struct {
	ReserveTokens *int `json:"reserveTokens,omitempty"` // default: 16384
}

// RetrySettings 重试设置
type RetrySettings struct {
	Enabled    *bool `json:"enabled,omitempty"`    // default: true
	MaxRetries *int  `json:"maxRetries,omitempty"` // default: 3
	BaseDelayMs *int `json:"baseDelayMs,omitempty"` // default: 2000
	MaxDelayMs *int  `json:"maxDelayMs,omitempty"`  // default: 60000
}

// TerminalSettings 终端设置
type TerminalSettings struct {
	ShowImages   *bool `json:"showImages,omitempty"`   // default: true
	ClearOnShrink *bool `json:"clearOnShrink,omitempty"` // default: false
}

// ImageSettings 图片设置
type ImageSettings struct {
	AutoResize *bool `json:"autoResize,omitempty"` // default: true
	BlockImages *bool `json:"blockImages,omitempty"` // default: false
}

// ThinkingBudgetsSettings 思考预算设置
type ThinkingBudgetsSettings struct {
	Minimal *int `json:"minimal,omitempty"`
	Low     *int `json:"low,omitempty"`
	Medium  *int `json:"medium,omitempty"`
	High    *int `json:"high,omitempty"`
}

// MarkdownSettings Markdown设置
type MarkdownSettings struct {
	CodeBlockIndent *string `json:"codeBlockIndent,omitempty"` // default: "  "
}

// PackageSource 包源
type PackageSource struct {
	Source   string   `json:"source,omitempty"`
	Extensions []string `json:"extensions,omitempty"`
	Skills   []string `json:"skills,omitempty"`
	Prompts  []string `json:"prompts,omitempty"`
	Themes   []string `json:"themes,omitempty"`
}

// IsString 检查是否为字符串形式
func (p PackageSource) IsString() bool {
	return p.Source != "" && len(p.Extensions) == 0 && len(p.Skills) == 0 && len(p.Prompts) == 0 && len(p.Themes) == 0
}

// Settings 设置结构
type Settings struct {
	LastChangelogVersion   *string                   `json:"lastChangelogVersion,omitempty"`
	DefaultProvider        *string                   `json:"defaultProvider,omitempty"`
	BaseModel           *string                   `json:"defaultModel,omitempty"`
	DefaultThinkingLevel   *ThinkingLevel        `json:"defaultThinkingLevel,omitempty"`
	Transport              *string                   `json:"transport,omitempty"` // default: "sse"
	SteeringMode           *string                   `json:"steeringMode,omitempty"` // "all" | "one-at-a-time"
	FollowUpMode           *string                   `json:"followUpMode,omitempty"` // "all" | "one-at-a-time"
	Theme                  *string                   `json:"theme,omitempty"`
	Compaction             *CompactionSettings       `json:"compaction,omitempty"`
	BranchSummary          *BranchSummarySettings    `json:"branchSummary,omitempty"`
	Retry                  *RetrySettings            `json:"retry,omitempty"`
	HideThinkingBlock      *bool                     `json:"hideThinkingBlock,omitempty"`
	ShellPath              *string                   `json:"shellPath,omitempty"`
	QuietStartup           *bool                     `json:"quietStartup,omitempty"`
	ShellCommandPrefix     *string                   `json:"shellCommandPrefix,omitempty"`
	CollapseChangelog      *bool                     `json:"collapseChangelog,omitempty"`
	Packages               []PackageSource           `json:"packages,omitempty"`
	Extensions             []string                  `json:"extensions,omitempty"`
	Skills                 []string                  `json:"skills,omitempty"`
	Prompts                []string                  `json:"prompts,omitempty"`
	Themes                 []string                  `json:"themes,omitempty"`
	EnableSkillCommands    *bool                     `json:"enableSkillCommands,omitempty"`
	Terminal               *TerminalSettings         `json:"terminal,omitempty"`
	Images                 *ImageSettings            `json:"images,omitempty"`
	EnabledModels          []string                  `json:"enabledModels,omitempty"`
	DoubleEscapeAction     *string                   `json:"doubleEscapeAction,omitempty"` // "fork" | "tree" | "none"
	ThinkingBudgets        *ThinkingBudgetsSettings  `json:"thinkingBudgets,omitempty"`
	EditorPaddingX         *int                      `json:"editorPaddingX,omitempty"`
	AutocompleteMaxVisible *int                      `json:"autocompleteMaxVisible,omitempty"`
	ShowHardwareCursor     *bool                     `json:"showHardwareCursor,omitempty"`
	Markdown               *MarkdownSettings         `json:"markdown,omitempty"`
}

// SettingsScope 设置范围
type SettingsScope string

const (
	SettingsScopeGlobal  SettingsScope = "global"
	SettingsScopeProject SettingsScope = "project"
)

// SettingsStorage 设置存储接口
type SettingsStorage interface {
	WithLock(scope SettingsScope, fn func(current *string) *string)
}

// SettingsError 设置错误
type SettingsError struct {
	Scope SettingsScope
	Error error
}

// FileSettingsStorage 文件设置存储
type FileSettingsStorage struct {
	globalSettingsPath  string
	projectSettingsPath string
}

// NewFileSettingsStorage 创建文件设置存储
func NewFileSettingsStorage(cwd, agentDir string) *FileSettingsStorage {
	return &FileSettingsStorage{
		globalSettingsPath:  filepath.Join(agentDir, "settings.json"),
		projectSettingsPath: filepath.Join(cwd, ".pi", "settings.json"),
	}
}

// WithLock 加锁操作
func (s *FileSettingsStorage) WithLock(scope SettingsScope, fn func(current *string) *string) {
	path := s.globalSettingsPath
	if scope == SettingsScopeProject {
		path = s.projectSettingsPath
	}

	dir := filepath.Dir(path)

	var current *string
	if data, err := os.ReadFile(path); err == nil {
		str := string(data)
		current = &str
	}

	next := fn(current)
	if next != nil {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return
		}
		os.WriteFile(path, []byte(*next), 0644)
	}
}

// InMemorySettingsStorage 内存设置存储
type InMemorySettingsStorage struct {
	global  *string
	project *string
	mu      sync.RWMutex
}

// NewInMemorySettingsStorage 创建内存设置存储
func NewInMemorySettingsStorage() *InMemorySettingsStorage {
	return &InMemorySettingsStorage{}
}

// WithLock 加锁操作
func (s *InMemorySettingsStorage) WithLock(scope SettingsScope, fn func(current *string) *string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var current *string
	if scope == SettingsScopeGlobal {
		current = s.global
	} else {
		current = s.project
	}

	next := fn(current)
	if next != nil {
		if scope == SettingsScopeGlobal {
			s.global = next
		} else {
			s.project = next
		}
	}
}

// SettingsManager 设置管理器
type SettingsManager struct {
	storage                      SettingsStorage
	globalSettings               Settings
	projectSettings              Settings
	settings                     Settings
	modifiedFields               map[string]bool
	modifiedNestedFields         map[string]map[string]bool
	modifiedProjectFields        map[string]bool
	modifiedProjectNestedFields  map[string]map[string]bool
	globalSettingsLoadError      error
	projectSettingsLoadError     error
	writeQueue                   chan func()
	errors                       []SettingsError
	mu                           sync.RWMutex
}

// NewSettingsManager 创建设置管理器
func NewSettingsManager(storage SettingsStorage, globalSettings, projectSettings Settings, globalLoadError, projectLoadError error, initialErrors []SettingsError) *SettingsManager {
	sm := &SettingsManager{
		storage:                     storage,
		globalSettings:              globalSettings,
		projectSettings:             projectSettings,
		modifiedFields:              make(map[string]bool),
		modifiedNestedFields:        make(map[string]map[string]bool),
		modifiedProjectFields:       make(map[string]bool),
		modifiedProjectNestedFields: make(map[string]map[string]bool),
		globalSettingsLoadError:     globalLoadError,
		projectSettingsLoadError:    projectLoadError,
		errors:                      initialErrors,
		writeQueue:                  make(chan func(), 100),
	}
	sm.settings = sm.deepMergeSettings(globalSettings, projectSettings)
	go sm.processWriteQueue()
	return sm
}

// processWriteQueue 处理写入队列
func (sm *SettingsManager) processWriteQueue() {
	for task := range sm.writeQueue {
		task()
	}
}

// deepMergeSettings 深度合并设置
func (sm *SettingsManager) deepMergeSettings(base, overrides Settings) Settings {
	result := base

	// 使用JSON序列化/反序列化进行深度复制和合并
	baseJSON, _ := json.Marshal(base)
	json.Unmarshal(baseJSON, &result)

	overridesJSON, _ := json.Marshal(overrides)
	var overridesMap map[string]interface{}
	json.Unmarshal(overridesJSON, &overridesMap)

	resultJSON, _ := json.Marshal(result)
	var resultMap map[string]interface{}
	json.Unmarshal(resultJSON, &resultMap)

	for key, value := range overridesMap {
		if value != nil {
			resultMap[key] = value
		}
	}

	mergedJSON, _ := json.Marshal(resultMap)
	json.Unmarshal(mergedJSON, &result)

	return result
}

// CreateSettingsManager 创建设置管理器（从文件）
func CreateSettingsManager(cwd, agentDir string) *SettingsManager {
	storage := NewFileSettingsStorage(cwd, agentDir)
	return SettingsManagerFromStorage(storage)
}

// SettingsManagerFromStorage 从存储创建设置管理器
func SettingsManagerFromStorage(storage SettingsStorage) *SettingsManager {
	globalLoad := tryLoadFromStorage(storage, SettingsScopeGlobal)
	projectLoad := tryLoadFromStorage(storage, SettingsScopeProject)

	var initialErrors []SettingsError
	if globalLoad.error != nil {
		initialErrors = append(initialErrors, SettingsError{Scope: SettingsScopeGlobal, Error: globalLoad.error})
	}
	if projectLoad.error != nil {
		initialErrors = append(initialErrors, SettingsError{Scope: SettingsScopeProject, Error: projectLoad.error})
	}

	return NewSettingsManager(storage, globalLoad.settings, projectLoad.settings, globalLoad.error, projectLoad.error, initialErrors)
}

// InMemorySettingsManager 创建内存设置管理器
func InMemorySettingsManager(settings Settings) *SettingsManager {
	storage := NewInMemorySettingsStorage()
	return NewSettingsManager(storage, settings, Settings{}, nil, nil, nil)
}

// loadResult 加载结果
type loadResult struct {
	settings Settings
	error    error
}

// loadFromStorage 从存储加载设置
func loadFromStorage(storage SettingsStorage, scope SettingsScope) Settings {
	var content *string
	storage.WithLock(scope, func(current *string) *string {
		content = current
		return nil
	})

	if content == nil {
		return Settings{}
	}

	var settings Settings
	if err := json.Unmarshal([]byte(*content), &settings); err != nil {
		panic(err)
	}
	return migrateSettings(settings)
}

// tryLoadFromStorage 尝试从存储加载设置
func tryLoadFromStorage(storage SettingsStorage, scope SettingsScope) loadResult {
	defer func() {
		if r := recover(); r != nil {
			// 捕获panic并返回错误
		}
	}()

	settings := loadFromStorage(storage, scope)
	return loadResult{settings: settings, error: nil}
}

// migrateSettings 迁移设置
func migrateSettings(settings Settings) Settings {
	// 迁移 queueMode -> steeringMode
	// 注意：在Go版本中，我们需要通过JSON处理
	settingsJSON, _ := json.Marshal(settings)
	var settingsMap map[string]interface{}
	json.Unmarshal(settingsJSON, &settingsMap)

	if queueMode, ok := settingsMap["queueMode"]; ok && settingsMap["steeringMode"] == nil {
		settingsMap["steeringMode"] = queueMode
		delete(settingsMap, "queueMode")
	}

	// 迁移 websockets boolean -> transport enum
	if settingsMap["transport"] == nil {
		if websockets, ok := settingsMap["websockets"].(bool); ok {
			if websockets {
				settingsMap["transport"] = "websocket"
			} else {
				settingsMap["transport"] = "sse"
			}
			delete(settingsMap, "websockets")
		}
	}

	migratedJSON, _ := json.Marshal(settingsMap)
	json.Unmarshal(migratedJSON, &settings)
	return settings
}

// GetGlobalSettings 获取全局设置
func (sm *SettingsManager) GetGlobalSettings() Settings {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	var cloned Settings
	data, _ := json.Marshal(sm.globalSettings)
	json.Unmarshal(data, &cloned)
	return cloned
}

// GetProjectSettings 获取项目设置
func (sm *SettingsManager) GetProjectSettings() Settings {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	var cloned Settings
	data, _ := json.Marshal(sm.projectSettings)
	json.Unmarshal(data, &cloned)
	return cloned
}

// Reload 重新加载设置
func (sm *SettingsManager) Reload() {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	globalLoad := tryLoadFromStorage(sm.storage, SettingsScopeGlobal)
	if globalLoad.error == nil {
		sm.globalSettings = globalLoad.settings
		sm.globalSettingsLoadError = nil
	} else {
		sm.globalSettingsLoadError = globalLoad.error
		sm.recordError(SettingsScopeGlobal, globalLoad.error)
	}

	sm.modifiedFields = make(map[string]bool)
	sm.modifiedNestedFields = make(map[string]map[string]bool)
	sm.modifiedProjectFields = make(map[string]bool)
	sm.modifiedProjectNestedFields = make(map[string]map[string]bool)

	projectLoad := tryLoadFromStorage(sm.storage, SettingsScopeProject)
	if projectLoad.error == nil {
		sm.projectSettings = projectLoad.settings
		sm.projectSettingsLoadError = nil
	} else {
		sm.projectSettingsLoadError = projectLoad.error
		sm.recordError(SettingsScopeProject, projectLoad.error)
	}

	sm.settings = sm.deepMergeSettings(sm.globalSettings, sm.projectSettings)
}

// ApplyOverrides 应用覆盖设置
func (sm *SettingsManager) ApplyOverrides(overrides Settings) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.settings = sm.deepMergeSettings(sm.settings, overrides)
}

// markModified 标记全局字段已修改
func (sm *SettingsManager) markModified(field string, nestedKey ...string) {
	sm.modifiedFields[field] = true
	if len(nestedKey) > 0 && nestedKey[0] != "" {
		if sm.modifiedNestedFields[field] == nil {
			sm.modifiedNestedFields[field] = make(map[string]bool)
		}
		sm.modifiedNestedFields[field][nestedKey[0]] = true
	}
}

// markProjectModified 标记项目字段已修改
func (sm *SettingsManager) markProjectModified(field string, nestedKey ...string) {
	sm.modifiedProjectFields[field] = true
	if len(nestedKey) > 0 && nestedKey[0] != "" {
		if sm.modifiedProjectNestedFields[field] == nil {
			sm.modifiedProjectNestedFields[field] = make(map[string]bool)
		}
		sm.modifiedProjectNestedFields[field][nestedKey[0]] = true
	}
}

// recordError 记录错误
func (sm *SettingsManager) recordError(scope SettingsScope, err error) {
	sm.errors = append(sm.errors, SettingsError{Scope: scope, Error: err})
}

// clearModifiedScope 清除修改标记
func (sm *SettingsManager) clearModifiedScope(scope SettingsScope) {
	if scope == SettingsScopeGlobal {
		sm.modifiedFields = make(map[string]bool)
		sm.modifiedNestedFields = make(map[string]map[string]bool)
	} else {
		sm.modifiedProjectFields = make(map[string]bool)
		sm.modifiedProjectNestedFields = make(map[string]map[string]bool)
	}
}

// persistScopedSettings 持久化范围设置
func (sm *SettingsManager) persistScopedSettings(scope SettingsScope, snapshotSettings Settings, modifiedFields map[string]bool, modifiedNestedFields map[string]map[string]bool) {
	sm.storage.WithLock(scope, func(current *string) *string {
		var currentFileSettings Settings
		if current != nil {
			currentFileSettings = migrateSettings(Settings{})
			json.Unmarshal([]byte(*current), &currentFileSettings)
		}

		mergedSettings := currentFileSettings

		settingsJSON, _ := json.Marshal(snapshotSettings)
		var settingsMap map[string]interface{}
		json.Unmarshal(settingsJSON, &settingsMap)

		mergedJSON, _ := json.Marshal(mergedSettings)
		var mergedMap map[string]interface{}
		json.Unmarshal(mergedJSON, &mergedMap)

		for field := range modifiedFields {
			value := settingsMap[field]
			if nestedKeys, ok := modifiedNestedFields[field]; ok && len(nestedKeys) > 0 {
				if nestedMap, ok := value.(map[string]interface{}); ok {
					baseNested := make(map[string]interface{})
					if existingNested, ok := mergedMap[field].(map[string]interface{}); ok {
						for k, v := range existingNested {
							baseNested[k] = v
						}
					}
					for nestedKey := range nestedKeys {
						if v, ok := nestedMap[nestedKey]; ok {
							baseNested[nestedKey] = v
						}
					}
					mergedMap[field] = baseNested
				}
			} else {
				mergedMap[field] = value
			}
		}

		resultJSON, _ := json.MarshalIndent(mergedMap, "", "  ")
		result := string(resultJSON)
		return &result
	})
}

// save 保存全局设置
func (sm *SettingsManager) save() {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.settings = sm.deepMergeSettings(sm.globalSettings, sm.projectSettings)

	if sm.globalSettingsLoadError != nil {
		return
	}

	snapshotGlobalSettings := sm.globalSettings
	modifiedFields := make(map[string]bool)
	for k, v := range sm.modifiedFields {
		modifiedFields[k] = v
	}
	modifiedNestedFields := make(map[string]map[string]bool)
	for k, v := range sm.modifiedNestedFields {
		nestedCopy := make(map[string]bool)
		for nk, nv := range v {
			nestedCopy[nk] = nv
		}
		modifiedNestedFields[k] = nestedCopy
	}

	sm.writeQueue <- func() {
		sm.persistScopedSettings(SettingsScopeGlobal, snapshotGlobalSettings, modifiedFields, modifiedNestedFields)
		sm.clearModifiedScope(SettingsScopeGlobal)
	}
}

// saveProjectSettings 保存项目设置
func (sm *SettingsManager) saveProjectSettings(settings Settings) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.projectSettings = settings
	sm.settings = sm.deepMergeSettings(sm.globalSettings, sm.projectSettings)

	if sm.projectSettingsLoadError != nil {
		return
	}

	snapshotProjectSettings := sm.projectSettings
	modifiedFields := make(map[string]bool)
	for k, v := range sm.modifiedProjectFields {
		modifiedFields[k] = v
	}
	modifiedNestedFields := make(map[string]map[string]bool)
	for k, v := range sm.modifiedProjectNestedFields {
		nestedCopy := make(map[string]bool)
		for nk, nv := range v {
			nestedCopy[nk] = nv
		}
		modifiedNestedFields[k] = nestedCopy
	}

	sm.writeQueue <- func() {
		sm.persistScopedSettings(SettingsScopeProject, snapshotProjectSettings, modifiedFields, modifiedNestedFields)
		sm.clearModifiedScope(SettingsScopeProject)
	}
}

// Flush 刷新写入队列
func (sm *SettingsManager) Flush() {
	done := make(chan struct{})
	sm.writeQueue <- func() {
		close(done)
	}
	<-done
}

// DrainErrors 获取并清空错误
func (sm *SettingsManager) DrainErrors() []SettingsError {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	drained := make([]SettingsError, len(sm.errors))
	copy(drained, sm.errors)
	sm.errors = nil
	return drained
}

// GetLastChangelogVersion 获取最后更新日志版本
func (sm *SettingsManager) GetLastChangelogVersion() *string {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.settings.LastChangelogVersion
}

// SetLastChangelogVersion 设置最后更新日志版本
func (sm *SettingsManager) SetLastChangelogVersion(version string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.globalSettings.LastChangelogVersion = &version
	sm.markModified("lastChangelogVersion")
	sm.save()
}

// GetDefaultProvider 获取默认提供商
func (sm *SettingsManager) GetDefaultProvider() *string {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.settings.DefaultProvider
}

// GetDefaultModel 获取默认模型
func (sm *SettingsManager) GetDefaultModel() *string {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.settings.BaseModel
}

// SetDefaultProvider 设置默认提供商
func (sm *SettingsManager) SetDefaultProvider(provider string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.globalSettings.DefaultProvider = &provider
	sm.markModified("defaultProvider")
	sm.save()
}

// SetDefaultModel 设置默认模型
func (sm *SettingsManager) SetDefaultModel(modelID string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.globalSettings.BaseModel = &modelID
	sm.markModified("defaultModel")
	sm.save()
}

// SetDefaultModelAndProvider 设置默认模型和提供商
func (sm *SettingsManager) SetDefaultModelAndProvider(provider, modelID string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.globalSettings.DefaultProvider = &provider
	sm.globalSettings.BaseModel = &modelID
	sm.markModified("defaultProvider")
	sm.markModified("defaultModel")
	sm.save()
}

// GetSteeringMode 获取引导模式
func (sm *SettingsManager) GetSteeringMode() string {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	if sm.settings.SteeringMode != nil {
		return *sm.settings.SteeringMode
	}
	return "one-at-a-time"
}

// SetSteeringMode 设置引导模式
func (sm *SettingsManager) SetSteeringMode(mode string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.globalSettings.SteeringMode = &mode
	sm.markModified("steeringMode")
	sm.save()
}

// GetFollowUpMode 获取跟进模式
func (sm *SettingsManager) GetFollowUpMode() string {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	if sm.settings.FollowUpMode != nil {
		return *sm.settings.FollowUpMode
	}
	return "one-at-a-time"
}

// SetFollowUpMode 设置跟进模式
func (sm *SettingsManager) SetFollowUpMode(mode string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.globalSettings.FollowUpMode = &mode
	sm.markModified("followUpMode")
	sm.save()
}

// GetTheme 获取主题
func (sm *SettingsManager) GetTheme() *string {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.settings.Theme
}

// SetTheme 设置主题
func (sm *SettingsManager) SetTheme(theme string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.globalSettings.Theme = &theme
	sm.markModified("theme")
	sm.save()
}

// GetDefaultThinkingLevel 获取默认思考级别
func (sm *SettingsManager) GetDefaultThinkingLevel() *ThinkingLevel {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.settings.DefaultThinkingLevel
}

// SetDefaultThinkingLevel 设置默认思考级别
func (sm *SettingsManager) SetDefaultThinkingLevel(level ThinkingLevel) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.globalSettings.DefaultThinkingLevel = &level
	sm.markModified("defaultThinkingLevel")
	sm.save()
}

// GetTransport 获取传输方式
func (sm *SettingsManager) GetTransport() string {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	if sm.settings.Transport != nil {
		return *sm.settings.Transport
	}
	return "sse"
}

// SetTransport 设置传输方式
func (sm *SettingsManager) SetTransport(transport string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.globalSettings.Transport = &transport
	sm.markModified("transport")
	sm.save()
}

// GetCompactionEnabled 获取压缩是否启用
func (sm *SettingsManager) GetCompactionEnabled() bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	if sm.settings.Compaction != nil && sm.settings.Compaction.Enabled != nil {
		return *sm.settings.Compaction.Enabled
	}
	return true
}

// SetCompactionEnabled 设置压缩是否启用
func (sm *SettingsManager) SetCompactionEnabled(enabled bool) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	if sm.globalSettings.Compaction == nil {
		sm.globalSettings.Compaction = &CompactionSettings{}
	}
	sm.globalSettings.Compaction.Enabled = &enabled
	sm.markModified("compaction", "enabled")
	sm.save()
}

// GetCompactionReserveTokens 获取压缩保留令牌数
func (sm *SettingsManager) GetCompactionReserveTokens() int {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	if sm.settings.Compaction != nil && sm.settings.Compaction.ReserveTokens != nil {
		return *sm.settings.Compaction.ReserveTokens
	}
	return 16384
}

// GetCompactionKeepRecentTokens 获取压缩保留最近令牌数
func (sm *SettingsManager) GetCompactionKeepRecentTokens() int {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	if sm.settings.Compaction != nil && sm.settings.Compaction.KeepRecentTokens != nil {
		return *sm.settings.Compaction.KeepRecentTokens
	}
	return 20000
}

// GetCompactionSettings 获取压缩设置
func (sm *SettingsManager) GetCompactionSettings() CompactionSettings {
	return CompactionSettings{
		Enabled:         boolPtr(sm.GetCompactionEnabled()),
		ReserveTokens:   intPtr(sm.GetCompactionReserveTokens()),
		KeepRecentTokens: intPtr(sm.GetCompactionKeepRecentTokens()),
	}
}

// GetBranchSummarySettings 获取分支摘要设置
func (sm *SettingsManager) GetBranchSummarySettings() BranchSummarySettings {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	reserveTokens := 16384
	if sm.settings.BranchSummary != nil && sm.settings.BranchSummary.ReserveTokens != nil {
		reserveTokens = *sm.settings.BranchSummary.ReserveTokens
	}
	return BranchSummarySettings{ReserveTokens: &reserveTokens}
}

// GetRetryEnabled 获取重试是否启用
func (sm *SettingsManager) GetRetryEnabled() bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	if sm.settings.Retry != nil && sm.settings.Retry.Enabled != nil {
		return *sm.settings.Retry.Enabled
	}
	return true
}

// SetRetryEnabled 设置重试是否启用
func (sm *SettingsManager) SetRetryEnabled(enabled bool) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	if sm.globalSettings.Retry == nil {
		sm.globalSettings.Retry = &RetrySettings{}
	}
	sm.globalSettings.Retry.Enabled = &enabled
	sm.markModified("retry", "enabled")
	sm.save()
}

// GetRetrySettings 获取重试设置
func (sm *SettingsManager) GetRetrySettings() RetrySettings {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	enabled := true
	maxRetries := 3
	baseDelayMs := 2000
	maxDelayMs := 60000

	if sm.settings.Retry != nil {
		if sm.settings.Retry.Enabled != nil {
			enabled = *sm.settings.Retry.Enabled
		}
		if sm.settings.Retry.MaxRetries != nil {
			maxRetries = *sm.settings.Retry.MaxRetries
		}
		if sm.settings.Retry.BaseDelayMs != nil {
			baseDelayMs = *sm.settings.Retry.BaseDelayMs
		}
		if sm.settings.Retry.MaxDelayMs != nil {
			maxDelayMs = *sm.settings.Retry.MaxDelayMs
		}
	}

	return RetrySettings{
		Enabled:     &enabled,
		MaxRetries:  &maxRetries,
		BaseDelayMs: &baseDelayMs,
		MaxDelayMs:  &maxDelayMs,
	}
}

// GetHideThinkingBlock 获取是否隐藏思考块
func (sm *SettingsManager) GetHideThinkingBlock() bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	if sm.settings.HideThinkingBlock != nil {
		return *sm.settings.HideThinkingBlock
	}
	return false
}

// SetHideThinkingBlock 设置是否隐藏思考块
func (sm *SettingsManager) SetHideThinkingBlock(hide bool) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.globalSettings.HideThinkingBlock = &hide
	sm.markModified("hideThinkingBlock")
	sm.save()
}

// GetShellPath 获取Shell路径
func (sm *SettingsManager) GetShellPath() *string {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.settings.ShellPath
}

// SetShellPath 设置Shell路径
func (sm *SettingsManager) SetShellPath(path *string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.globalSettings.ShellPath = path
	sm.markModified("shellPath")
	sm.save()
}

// GetQuietStartup 获取是否静默启动
func (sm *SettingsManager) GetQuietStartup() bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	if sm.settings.QuietStartup != nil {
		return *sm.settings.QuietStartup
	}
	return false
}

// SetQuietStartup 设置是否静默启动
func (sm *SettingsManager) SetQuietStartup(quiet bool) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.globalSettings.QuietStartup = &quiet
	sm.markModified("quietStartup")
	sm.save()
}

// GetShellCommandPrefix 获取Shell命令前缀
func (sm *SettingsManager) GetShellCommandPrefix() *string {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.settings.ShellCommandPrefix
}

// SetShellCommandPrefix 设置Shell命令前缀
func (sm *SettingsManager) SetShellCommandPrefix(prefix *string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.globalSettings.ShellCommandPrefix = prefix
	sm.markModified("shellCommandPrefix")
	sm.save()
}

// GetCollapseChangelog 获取是否折叠更新日志
func (sm *SettingsManager) GetCollapseChangelog() bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	if sm.settings.CollapseChangelog != nil {
		return *sm.settings.CollapseChangelog
	}
	return false
}

// SetCollapseChangelog 设置是否折叠更新日志
func (sm *SettingsManager) SetCollapseChangelog(collapse bool) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.globalSettings.CollapseChangelog = &collapse
	sm.markModified("collapseChangelog")
	sm.save()
}

// GetPackages 获取包源
func (sm *SettingsManager) GetPackages() []PackageSource {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	if sm.settings.Packages != nil {
		result := make([]PackageSource, len(sm.settings.Packages))
		copy(result, sm.settings.Packages)
		return result
	}
	return nil
}

// SetPackages 设置包源
func (sm *SettingsManager) SetPackages(packages []PackageSource) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.globalSettings.Packages = packages
	sm.markModified("packages")
	sm.save()
}

// SetProjectPackages 设置项目包源
func (sm *SettingsManager) SetProjectPackages(packages []PackageSource) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	projectSettings := sm.projectSettings
	projectSettings.Packages = packages
	sm.markProjectModified("packages")
	sm.saveProjectSettings(projectSettings)
}

// GetExtensionPaths 获取扩展路径
func (sm *SettingsManager) GetExtensionPaths() []string {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	if sm.settings.Extensions != nil {
		result := make([]string, len(sm.settings.Extensions))
		copy(result, sm.settings.Extensions)
		return result
	}
	return nil
}

// SetExtensionPaths 设置扩展路径
func (sm *SettingsManager) SetExtensionPaths(paths []string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.globalSettings.Extensions = paths
	sm.markModified("extensions")
	sm.save()
}

// SetProjectExtensionPaths 设置项目扩展路径
func (sm *SettingsManager) SetProjectExtensionPaths(paths []string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	projectSettings := sm.projectSettings
	projectSettings.Extensions = paths
	sm.markProjectModified("extensions")
	sm.saveProjectSettings(projectSettings)
}

// GetSkillPaths 获取技能路径
func (sm *SettingsManager) GetSkillPaths() []string {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	if sm.settings.Skills != nil {
		result := make([]string, len(sm.settings.Skills))
		copy(result, sm.settings.Skills)
		return result
	}
	return nil
}

// SetSkillPaths 设置技能路径
func (sm *SettingsManager) SetSkillPaths(paths []string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.globalSettings.Skills = paths
	sm.markModified("skills")
	sm.save()
}

// SetProjectSkillPaths 设置项目技能路径
func (sm *SettingsManager) SetProjectSkillPaths(paths []string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	projectSettings := sm.projectSettings
	projectSettings.Skills = paths
	sm.markProjectModified("skills")
	sm.saveProjectSettings(projectSettings)
}

// GetPromptTemplatePaths 获取提示模板路径
func (sm *SettingsManager) GetPromptTemplatePaths() []string {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	if sm.settings.Prompts != nil {
		result := make([]string, len(sm.settings.Prompts))
		copy(result, sm.settings.Prompts)
		return result
	}
	return nil
}

// SetPromptTemplatePaths 设置提示模板路径
func (sm *SettingsManager) SetPromptTemplatePaths(paths []string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.globalSettings.Prompts = paths
	sm.markModified("prompts")
	sm.save()
}

// SetProjectPromptTemplatePaths 设置项目提示模板路径
func (sm *SettingsManager) SetProjectPromptTemplatePaths(paths []string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	projectSettings := sm.projectSettings
	projectSettings.Prompts = paths
	sm.markProjectModified("prompts")
	sm.saveProjectSettings(projectSettings)
}

// GetThemePaths 获取主题路径
func (sm *SettingsManager) GetThemePaths() []string {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	if sm.settings.Themes != nil {
		result := make([]string, len(sm.settings.Themes))
		copy(result, sm.settings.Themes)
		return result
	}
	return nil
}

// SetThemePaths 设置主题路径
func (sm *SettingsManager) SetThemePaths(paths []string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.globalSettings.Themes = paths
	sm.markModified("themes")
	sm.save()
}

// SetProjectThemePaths 设置项目主题路径
func (sm *SettingsManager) SetProjectThemePaths(paths []string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	projectSettings := sm.projectSettings
	projectSettings.Themes = paths
	sm.markProjectModified("themes")
	sm.saveProjectSettings(projectSettings)
}

// GetEnableSkillCommands 获取是否启用技能命令
func (sm *SettingsManager) GetEnableSkillCommands() bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	if sm.settings.EnableSkillCommands != nil {
		return *sm.settings.EnableSkillCommands
	}
	return true
}

// SetEnableSkillCommands 设置是否启用技能命令
func (sm *SettingsManager) SetEnableSkillCommands(enabled bool) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.globalSettings.EnableSkillCommands = &enabled
	sm.markModified("enableSkillCommands")
	sm.save()
}

// GetThinkingBudgets 获取思考预算
func (sm *SettingsManager) GetThinkingBudgets() *ThinkingBudgetsSettings {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.settings.ThinkingBudgets
}

// GetShowImages 获取是否显示图片
func (sm *SettingsManager) GetShowImages() bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	if sm.settings.Terminal != nil && sm.settings.Terminal.ShowImages != nil {
		return *sm.settings.Terminal.ShowImages
	}
	return true
}

// SetShowImages 设置是否显示图片
func (sm *SettingsManager) SetShowImages(show bool) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	if sm.globalSettings.Terminal == nil {
		sm.globalSettings.Terminal = &TerminalSettings{}
	}
	sm.globalSettings.Terminal.ShowImages = &show
	sm.markModified("terminal", "showImages")
	sm.save()
}

// GetClearOnShrink 获取是否在缩小时清除
func (sm *SettingsManager) GetClearOnShrink() bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	if sm.settings.Terminal != nil && sm.settings.Terminal.ClearOnShrink != nil {
		return *sm.settings.Terminal.ClearOnShrink
	}
	// 检查环境变量
	if os.Getenv("PI_CLEAR_ON_SHRINK") == "1" {
		return true
	}
	return false
}

// SetClearOnShrink 设置在缩小时清除
func (sm *SettingsManager) SetClearOnShrink(enabled bool) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	if sm.globalSettings.Terminal == nil {
		sm.globalSettings.Terminal = &TerminalSettings{}
	}
	sm.globalSettings.Terminal.ClearOnShrink = &enabled
	sm.markModified("terminal", "clearOnShrink")
	sm.save()
}

// GetImageAutoResize 获取图片自动调整大小
func (sm *SettingsManager) GetImageAutoResize() bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	if sm.settings.Images != nil && sm.settings.Images.AutoResize != nil {
		return *sm.settings.Images.AutoResize
	}
	return true
}

// SetImageAutoResize 设置图片自动调整大小
func (sm *SettingsManager) SetImageAutoResize(enabled bool) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	if sm.globalSettings.Images == nil {
		sm.globalSettings.Images = &ImageSettings{}
	}
	sm.globalSettings.Images.AutoResize = &enabled
	sm.markModified("images", "autoResize")
	sm.save()
}

// GetBlockImages 获取是否阻止图片
func (sm *SettingsManager) GetBlockImages() bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	if sm.settings.Images != nil && sm.settings.Images.BlockImages != nil {
		return *sm.settings.Images.BlockImages
	}
	return false
}

// SetBlockImages 设置是否阻止图片
func (sm *SettingsManager) SetBlockImages(blocked bool) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	if sm.globalSettings.Images == nil {
		sm.globalSettings.Images = &ImageSettings{}
	}
	sm.globalSettings.Images.BlockImages = &blocked
	sm.markModified("images", "blockImages")
	sm.save()
}

// GetEnabledModels 获取启用的模型
func (sm *SettingsManager) GetEnabledModels() []string {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	if sm.settings.EnabledModels != nil {
		result := make([]string, len(sm.settings.EnabledModels))
		copy(result, sm.settings.EnabledModels)
		return result
	}
	return nil
}

// SetEnabledModels 设置启用的模型
func (sm *SettingsManager) SetEnabledModels(patterns []string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.globalSettings.EnabledModels = patterns
	sm.markModified("enabledModels")
	sm.save()
}

// GetDoubleEscapeAction 获取双击转义动作
func (sm *SettingsManager) GetDoubleEscapeAction() string {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	if sm.settings.DoubleEscapeAction != nil {
		return *sm.settings.DoubleEscapeAction
	}
	return "tree"
}

// SetDoubleEscapeAction 设置双击转义动作
func (sm *SettingsManager) SetDoubleEscapeAction(action string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.globalSettings.DoubleEscapeAction = &action
	sm.markModified("doubleEscapeAction")
	sm.save()
}

// GetShowHardwareCursor 获取是否显示硬件光标
func (sm *SettingsManager) GetShowHardwareCursor() bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	if sm.settings.ShowHardwareCursor != nil {
		return *sm.settings.ShowHardwareCursor
	}
	if os.Getenv("PI_HARDWARE_CURSOR") == "1" {
		return true
	}
	return false
}

// SetShowHardwareCursor 设置是否显示硬件光标
func (sm *SettingsManager) SetShowHardwareCursor(enabled bool) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.globalSettings.ShowHardwareCursor = &enabled
	sm.markModified("showHardwareCursor")
	sm.save()
}

// GetEditorPaddingX 获取编辑器水平内边距
func (sm *SettingsManager) GetEditorPaddingX() int {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	if sm.settings.EditorPaddingX != nil {
		return *sm.settings.EditorPaddingX
	}
	return 0
}

// SetEditorPaddingX 设置编辑器水平内边距
func (sm *SettingsManager) SetEditorPaddingX(padding int) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	// 限制范围在0-3之间
	if padding < 0 {
		padding = 0
	} else if padding > 3 {
		padding = 3
	}
	sm.globalSettings.EditorPaddingX = &padding
	sm.markModified("editorPaddingX")
	sm.save()
}

// GetAutocompleteMaxVisible 获取自动完成最大可见项
func (sm *SettingsManager) GetAutocompleteMaxVisible() int {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	if sm.settings.AutocompleteMaxVisible != nil {
		return *sm.settings.AutocompleteMaxVisible
	}
	return 5
}

// SetAutocompleteMaxVisible 设置自动完成最大可见项
func (sm *SettingsManager) SetAutocompleteMaxVisible(maxVisible int) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	// 限制范围在3-20之间
	if maxVisible < 3 {
		maxVisible = 3
	} else if maxVisible > 20 {
		maxVisible = 20
	}
	sm.globalSettings.AutocompleteMaxVisible = &maxVisible
	sm.markModified("autocompleteMaxVisible")
	sm.save()
}

// GetCodeBlockIndent 获取代码块缩进
func (sm *SettingsManager) GetCodeBlockIndent() string {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	if sm.settings.Markdown != nil && sm.settings.Markdown.CodeBlockIndent != nil {
		return *sm.settings.Markdown.CodeBlockIndent
	}
	return "  "
}

// Helper functions
func boolPtr(b bool) *bool {
	return &b
}

func intPtr(i int) *int {
	return &i
}

// Interface methods implementation check
func (sm *SettingsManager) GetSettings() Settings {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	var cloned Settings
	data, _ := json.Marshal(sm.settings)
	json.Unmarshal(data, &cloned)
	return cloned
}

// GetSetting returns a specific setting value by key
func (sm *SettingsManager) GetSetting(key string) (interface{}, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	
	settingsJSON, _ := json.Marshal(sm.settings)
	var settingsMap map[string]interface{}
	json.Unmarshal(settingsJSON, &settingsMap)
	
	value, ok := settingsMap[key]
	return value, ok
}

// SetSetting sets a specific setting value by key
func (sm *SettingsManager) SetSetting(key string, value interface{}) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	
	settingsJSON, _ := json.Marshal(sm.globalSettings)
	var settingsMap map[string]interface{}
	json.Unmarshal(settingsJSON, &settingsMap)
	
	settingsMap[key] = value
	
	resultJSON, _ := json.Marshal(settingsMap)
	json.Unmarshal(resultJSON, &sm.globalSettings)
	
	sm.markModified(key)
	sm.save()
	
	return nil
}

// Validate validates all settings
func (sm *SettingsManager) Validate() error {
	// 验证设置的有效性
	if sm.settings.DefaultThinkingLevel != nil {
		level := *sm.settings.DefaultThinkingLevel
		validLevels := []ThinkingLevel{ThinkingLevelOff, ThinkingLevelMinimal, ThinkingLevelLow, ThinkingLevelMedium, ThinkingLevelHigh, ThinkingLevelXHigh}
		found := false
		for _, valid := range validLevels {
			if level == valid {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("invalid thinking level: %s", level)
		}
	}
	
	if sm.settings.SteeringMode != nil {
		mode := *sm.settings.SteeringMode
		if mode != "all" && mode != "one-at-a-time" {
			return fmt.Errorf("invalid steering mode: %s", mode)
		}
	}
	
	if sm.settings.FollowUpMode != nil {
		mode := *sm.settings.FollowUpMode
		if mode != "all" && mode != "one-at-a-time" {
			return fmt.Errorf("invalid follow-up mode: %s", mode)
		}
	}
	
	if sm.settings.DoubleEscapeAction != nil {
		action := *sm.settings.DoubleEscapeAction
		if action != "fork" && action != "tree" && action != "none" {
			return fmt.Errorf("invalid double escape action: %s", action)
		}
	}
	
	return nil
}

// Reset resets settings to defaults
func (sm *SettingsManager) Reset() error {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	
	sm.globalSettings = Settings{}
	sm.projectSettings = Settings{}
	sm.settings = Settings{}
	sm.modifiedFields = make(map[string]bool)
	sm.modifiedNestedFields = make(map[string]map[string]bool)
	sm.modifiedProjectFields = make(map[string]bool)
	sm.modifiedProjectNestedFields = make(map[string]map[string]bool)
	
	// 保存空设置
	sm.save()
	
	return nil
}

// Export exports settings to a map
func (sm *SettingsManager) Export() (map[string]interface{}, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	
	settingsJSON, _ := json.Marshal(sm.settings)
	var result map[string]interface{}
	json.Unmarshal(settingsJSON, &result)
	
	return result, nil
}

// Import imports settings from a map
func (sm *SettingsManager) Import(data map[string]interface{}) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	
	dataJSON, _ := json.Marshal(data)
	var settings Settings
	if err := json.Unmarshal(dataJSON, &settings); err != nil {
		return err
	}
	
	sm.globalSettings = settings
	sm.settings = sm.deepMergeSettings(sm.globalSettings, sm.projectSettings)
	
	// 标记所有字段为已修改
	for key := range data {
		sm.markModified(key)
	}
	
	sm.save()
	
	return nil
}
