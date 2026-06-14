package session

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// CollisionInfo 碰撞信息
type CollisionInfo struct {
	ResourceType string `json:"resourceType"` // "skill", "prompt", "theme"
	Name         string `json:"name"`
	WinnerPath   string `json:"winnerPath"`
	LoserPath    string `json:"loserPath"`
}

// ResourceDiagnostic 资源诊断信息
type ResourceDiagnostic struct {
	Type      string            `json:"type"`                // "warning", "collision"
	Message   string            `json:"message"`
	Path      string            `json:"path,omitempty"`
	Collision *CollisionInfo    `json:"collision,omitempty"`
}

// PromptsResult 提示模板结果
type PromptsResult struct {
	Prompts     []PromptTemplate     `json:"prompts"`
	Diagnostics []ResourceDiagnostic `json:"diagnostics,omitempty"`
}

// PromptTemplate 提示模板
type PromptTemplate struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	FilePath    string `json:"filePath"`
	Source      string `json:"source"`
	Template    string `json:"template"`
}

// AgentsFile 代理文件
type AgentsFile struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

// AgentsFilesResult 代理文件结果
type AgentsFilesResult struct {
	AgentsFiles []AgentsFile `json:"agentsFiles"`
}

// PathMetadata 路径元数据
type PathMetadata struct {
	Source string `json:"source"` // "local", "cli", "auto"
	Scope  string `json:"scope"`  // "user", "project", "temporary"
	Origin string `json:"origin"` // "top-level", "package"
}

// ResourceExtensionPaths 资源扩展路径
type ResourceExtensionPaths struct {
	SkillPaths  []PathMetadataEntry
	PromptPaths []PathMetadataEntry
	ThemePaths  []PathMetadataEntry
}

// PathMetadataEntry 带元数据的路径条目
type PathMetadataEntry struct {
	Path     string
	Metadata PathMetadata
}

// Theme 主题定义
type Theme struct {
	Name       string `json:"name"`
	SourcePath string `json:"sourcePath,omitempty"`
}

// ThemesResult 主题结果
type ThemesResult struct {
	Themes      []Theme
	Diagnostics []ResourceDiagnostic
}

// ResourceLoader 资源加载器接口
type ResourceLoader interface {
	GetSkills() *SkillsResult
	GetPrompts() *PromptsResult
	GetThemes() *ThemesResult
	GetAgentsFiles() *AgentsFilesResult
	GetSystemPrompt() string
	GetAppendSystemPrompt() []string
	GetPathMetadata() map[string]PathMetadata
	ExtendResources(paths ResourceExtensionPaths)
	Reload() error
}

// DefaultResourceLoader 默认资源加载器实现
type DefaultResourceLoader struct {
	cwd                        string
	agentDir                   string
	settingsManager            *SettingsManager
	additionalSkillPaths       []string
	additionalPromptPaths      []string
	additionalThemePaths       []string
	systemPromptSource         string
	appendSystemPromptSource   string
	noSkills                   bool
	noPromptTemplates          bool
	noThemes                   bool

	skills              []SkillInfo
	skillDiagnostics    []ResourceDiagnostic
	prompts             []PromptTemplate
	promptDiagnostics   []ResourceDiagnostic
	themes              []Theme
	themeDiagnostics    []ResourceDiagnostic
	agentsFiles         []AgentsFile
	systemPrompt        string
	appendSystemPrompt  []string
	pathMetadata        map[string]PathMetadata

	mu sync.RWMutex
}

// DefaultResourceLoaderOptions 默认资源加载器选项
type DefaultResourceLoaderOptions struct {
	Cwd                      string
	AgentDir                 string
	SettingsManager          *SettingsManager
	AdditionalSkillPaths     []string
	AdditionalPromptPaths    []string
	AdditionalThemePaths     []string
	SystemPrompt             string
	AppendSystemPrompt       string
	NoSkills                 bool
	NoPromptTemplates        bool
	NoThemes                 bool
}

// NewDefaultResourceLoader 创建默认资源加载器
func NewDefaultResourceLoader(options DefaultResourceLoaderOptions) *DefaultResourceLoader {
	cwd := options.Cwd
	if cwd == "" {
		cwd, _ = os.Getwd()
	}

	agentDir := options.AgentDir
	if agentDir == "" {
		agentDir = GetAgentDir()
	}

	loader := &DefaultResourceLoader{
		cwd:                      cwd,
		agentDir:                 agentDir,
		settingsManager:          options.SettingsManager,
		additionalSkillPaths:     options.AdditionalSkillPaths,
		additionalPromptPaths:    options.AdditionalPromptPaths,
		additionalThemePaths:     options.AdditionalThemePaths,
		systemPromptSource:       options.SystemPrompt,
		appendSystemPromptSource: options.AppendSystemPrompt,
		noSkills:                 options.NoSkills,
		noPromptTemplates:        options.NoPromptTemplates,
		noThemes:                 options.NoThemes,
		pathMetadata:             make(map[string]PathMetadata),
	}

	// 初始加载
	loader.Reload()

	return loader
}

// GetSkills 获取技能
func (rl *DefaultResourceLoader) GetSkills() *SkillsResult {
	rl.mu.RLock()
	defer rl.mu.RUnlock()

	return &SkillsResult{
		Skills:      copySkills(rl.skills),
		Diagnostics: copyDiagnostics(rl.skillDiagnostics),
	}
}

// GetPrompts 获取提示模板
func (rl *DefaultResourceLoader) GetPrompts() *PromptsResult {
	rl.mu.RLock()
	defer rl.mu.RUnlock()

	return &PromptsResult{
		Prompts:     copyPrompts(rl.prompts),
		Diagnostics: copyDiagnostics(rl.promptDiagnostics),
	}
}

// GetThemes 获取主题
func (rl *DefaultResourceLoader) GetThemes() *ThemesResult {
	rl.mu.RLock()
	defer rl.mu.RUnlock()

	return &ThemesResult{
		Themes:      copyThemes(rl.themes),
		Diagnostics: copyDiagnostics(rl.themeDiagnostics),
	}
}

// GetAgentsFiles 获取代理文件
func (rl *DefaultResourceLoader) GetAgentsFiles() *AgentsFilesResult {
	rl.mu.RLock()
	defer rl.mu.RUnlock()

	return &AgentsFilesResult{
		AgentsFiles: copyAgentsFiles(rl.agentsFiles),
	}
}

// GetSystemPrompt 获取系统提示
func (rl *DefaultResourceLoader) GetSystemPrompt() string {
	rl.mu.RLock()
	defer rl.mu.RUnlock()
	return rl.systemPrompt
}

// GetAppendSystemPrompt 获取追加系统提示
func (rl *DefaultResourceLoader) GetAppendSystemPrompt() []string {
	rl.mu.RLock()
	defer rl.mu.RUnlock()
	return copyStrings(rl.appendSystemPrompt)
}

// GetPathMetadata 获取路径元数据
func (rl *DefaultResourceLoader) GetPathMetadata() map[string]PathMetadata {
	rl.mu.RLock()
	defer rl.mu.RUnlock()

	result := make(map[string]PathMetadata)
	for k, v := range rl.pathMetadata {
		result[k] = v
	}
	return result
}

// ExtendResources 扩展资源
func (rl *DefaultResourceLoader) ExtendResources(paths ResourceExtensionPaths) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	if len(paths.SkillPaths) > 0 {
		skillPaths := make([]string, 0, len(paths.SkillPaths))
		for _, entry := range paths.SkillPaths {
			resolved := rl.resolveResourcePath(entry.Path)
			skillPaths = append(skillPaths, resolved)
			rl.pathMetadata[resolved] = entry.Metadata
		}
		rl.updateSkillsFromPaths(skillPaths)
	}

	if len(paths.PromptPaths) > 0 {
		promptPaths := make([]string, 0, len(paths.PromptPaths))
		for _, entry := range paths.PromptPaths {
			resolved := rl.resolveResourcePath(entry.Path)
			promptPaths = append(promptPaths, resolved)
			rl.pathMetadata[resolved] = entry.Metadata
		}
		rl.updatePromptsFromPaths(promptPaths)
	}

	if len(paths.ThemePaths) > 0 {
		themePaths := make([]string, 0, len(paths.ThemePaths))
		for _, entry := range paths.ThemePaths {
			resolved := rl.resolveResourcePath(entry.Path)
			themePaths = append(themePaths, resolved)
			rl.pathMetadata[resolved] = entry.Metadata
		}
		rl.updateThemesFromPaths(themePaths)
	}
}

// Reload 重新加载资源
func (rl *DefaultResourceLoader) Reload() error {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	// 加载技能
	skillPaths := rl.additionalSkillPaths
	if !rl.noSkills {
		defaultPaths := rl.getDefaultSkillPaths()
		skillPaths = rl.mergePaths(defaultPaths, skillPaths)
	}
	rl.updateSkillsFromPaths(skillPaths)

	// 加载提示模板
	promptPaths := rl.additionalPromptPaths
	if !rl.noPromptTemplates {
		defaultPaths := rl.getDefaultPromptPaths()
		promptPaths = rl.mergePaths(defaultPaths, promptPaths)
	}
	rl.updatePromptsFromPaths(promptPaths)

	// 加载主题
	themePaths := rl.additionalThemePaths
	if !rl.noThemes {
		defaultPaths := rl.getDefaultThemePaths()
		themePaths = rl.mergePaths(defaultPaths, themePaths)
	}
	rl.updateThemesFromPaths(themePaths)

	// 加载代理文件
	rl.agentsFiles = rl.loadProjectContextFiles()

	// 加载系统提示
	rl.systemPrompt = rl.resolvePromptInput(rl.systemPromptSource, "system prompt")
	if rl.systemPrompt == "" {
		rl.systemPrompt = rl.discoverSystemPromptFile()
	}

	// 加载追加系统提示
	appendSource := rl.appendSystemPromptSource
	if appendSource == "" {
		appendSource = rl.discoverAppendSystemPromptFile()
	}
	resolvedAppend := rl.resolvePromptInput(appendSource, "append system prompt")
	if resolvedAppend != "" {
		rl.appendSystemPrompt = []string{resolvedAppend}
	} else {
		rl.appendSystemPrompt = []string{}
	}

	return nil
}

// updateSkillsFromPaths 从路径更新技能
func (rl *DefaultResourceLoader) updateSkillsFromPaths(paths []string) {
	if rl.noSkills && len(paths) == 0 {
		rl.skills = []SkillInfo{}
		rl.skillDiagnostics = []ResourceDiagnostic{}
		return
	}

	skills, diagnostics := rl.loadSkills(paths)
	rl.skills = skills
	rl.skillDiagnostics = diagnostics
}

// updatePromptsFromPaths 从路径更新提示模板
func (rl *DefaultResourceLoader) updatePromptsFromPaths(paths []string) {
	if rl.noPromptTemplates && len(paths) == 0 {
		rl.prompts = []PromptTemplate{}
		rl.promptDiagnostics = []ResourceDiagnostic{}
		return
	}

	prompts, diagnostics := rl.loadPromptTemplates(paths)
	rl.prompts = prompts
	rl.promptDiagnostics = diagnostics
}

// updateThemesFromPaths 从路径更新主题
func (rl *DefaultResourceLoader) updateThemesFromPaths(paths []string) {
	if rl.noThemes && len(paths) == 0 {
		rl.themes = []Theme{}
		rl.themeDiagnostics = []ResourceDiagnostic{}
		return
	}

	themes, diagnostics := rl.loadThemes(paths)
	rl.themes = themes
	rl.themeDiagnostics = diagnostics
}

// loadSkills 加载技能
func (rl *DefaultResourceLoader) loadSkills(paths []string) ([]SkillInfo, []ResourceDiagnostic) {
	var skills []SkillInfo
	var diagnostics []ResourceDiagnostic

	// 默认技能路径
	defaultPaths := []string{
		filepath.Join(rl.agentDir, "skills"),
		filepath.Join(rl.cwd, ".pi", "skills"),
	}

	for _, dir := range defaultPaths {
		if !sliceContains(paths, dir) {
			paths = append([]string{dir}, paths...)
		}
	}

	seen := make(map[string]bool)

	for _, path := range paths {
		resolved := rl.resolveResourcePath(path)
		if seen[resolved] {
			continue
		}
		seen[resolved] = true

		info, err := os.Stat(resolved)
		if err != nil {
			if !os.IsNotExist(err) {
				diagnostics = append(diagnostics, ResourceDiagnostic{
					Type:    "warning",
					Message: fmt.Sprintf("cannot access skill path: %v", err),
					Path:    resolved,
				})
			}
			continue
		}

		if info.IsDir() {
			skills = rl.loadSkillsFromDir(resolved, skills, &diagnostics)
		} else {
			skill := rl.loadSkillFromFile(resolved)
			if skill != nil {
				skills = append(skills, *skill)
			}
		}
	}

	return skills, diagnostics
}

// loadSkillsFromDir 从目录加载技能
func (rl *DefaultResourceLoader) loadSkillsFromDir(dir string, skills []SkillInfo, diagnostics *[]ResourceDiagnostic) []SkillInfo {
	entries, err := os.ReadDir(dir)
	if err != nil {
		*diagnostics = append(*diagnostics, ResourceDiagnostic{
			Type:    "warning",
			Message: fmt.Sprintf("cannot read skill directory: %v", err),
			Path:    dir,
		})
		return skills
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if !strings.HasSuffix(name, ".md") || name == "SKILL.md" {
			continue
		}

		filePath := filepath.Join(dir, name)
		skill := rl.loadSkillFromFile(filePath)
		if skill != nil {
			skills = append(skills, *skill)
		}
	}

	return skills
}

// loadSkillFromFile 从文件加载技能
func (rl *DefaultResourceLoader) loadSkillFromFile(filePath string) *SkillInfo {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil
	}

	// 简单解析技能文件
	name := strings.TrimSuffix(filepath.Base(filePath), ".md")
	contentStr := string(content)

	// 提取描述（第一行）
	lines := strings.Split(contentStr, "\n")
	description := ""
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			description = line
			break
		}
	}

	return &SkillInfo{
		Name:        name,
		Description: description,
		FilePath:    filePath,
		BaseDir:     filepath.Dir(filePath),
		Source:      "local",
	}
}

// loadPromptTemplates 加载提示模板
func (rl *DefaultResourceLoader) loadPromptTemplates(paths []string) ([]PromptTemplate, []ResourceDiagnostic) {
	var prompts []PromptTemplate
	var diagnostics []ResourceDiagnostic

	// 默认提示模板路径
	defaultPaths := []string{
		filepath.Join(rl.agentDir, "prompts"),
		filepath.Join(rl.cwd, ".pi", "prompts"),
	}

	for _, dir := range defaultPaths {
		if !sliceContains(paths, dir) {
			paths = append([]string{dir}, paths...)
		}
	}

	seen := make(map[string]bool)

	for _, path := range paths {
		resolved := rl.resolveResourcePath(path)
		if seen[resolved] {
			continue
		}
		seen[resolved] = true

		info, err := os.Stat(resolved)
		if err != nil {
			if !os.IsNotExist(err) {
				diagnostics = append(diagnostics, ResourceDiagnostic{
					Type:    "warning",
					Message: fmt.Sprintf("cannot access prompt path: %v", err),
					Path:    resolved,
				})
			}
			continue
		}

		if info.IsDir() {
			prompts = rl.loadPromptsFromDir(resolved, prompts, &diagnostics)
		} else {
			prompt := rl.loadPromptFromFile(resolved)
			if prompt != nil {
				prompts = append(prompts, *prompt)
			}
		}
	}

	// 去重
	return rl.dedupePrompts(prompts, diagnostics)
}

// loadPromptsFromDir 从目录加载提示模板
func (rl *DefaultResourceLoader) loadPromptsFromDir(dir string, prompts []PromptTemplate, diagnostics *[]ResourceDiagnostic) []PromptTemplate {
	entries, err := os.ReadDir(dir)
	if err != nil {
		*diagnostics = append(*diagnostics, ResourceDiagnostic{
			Type:    "warning",
			Message: fmt.Sprintf("cannot read prompt directory: %v", err),
			Path:    dir,
		})
		return prompts
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if !strings.HasSuffix(name, ".md") {
			continue
		}

		filePath := filepath.Join(dir, name)
		prompt := rl.loadPromptFromFile(filePath)
		if prompt != nil {
			prompts = append(prompts, *prompt)
		}
	}

	return prompts
}

// loadPromptFromFile 从文件加载提示模板
func (rl *DefaultResourceLoader) loadPromptFromFile(filePath string) *PromptTemplate {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil
	}

	name := strings.TrimSuffix(filepath.Base(filePath), ".md")
	contentStr := string(content)

	// 提取描述（第一行）
	lines := strings.Split(contentStr, "\n")
	description := ""
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			description = line
			break
		}
	}

	return &PromptTemplate{
		Name:        name,
		Description: description,
		Template:    contentStr,
		FilePath:    filePath,
	}
}

// dedupePrompts 去重提示模板
func (rl *DefaultResourceLoader) dedupePrompts(prompts []PromptTemplate, diagnostics []ResourceDiagnostic) ([]PromptTemplate, []ResourceDiagnostic) {
	seen := make(map[string]*PromptTemplate)

	for _, prompt := range prompts {
		if existing, ok := seen[prompt.Name]; ok {
			diagnostics = append(diagnostics, ResourceDiagnostic{
				Type:    "collision",
				Message: fmt.Sprintf(`name "/%s" collision`, prompt.Name),
				Path:    prompt.FilePath,
			})
			// 保留第一个（已存在的）
			_ = existing
		} else {
			seen[prompt.Name] = &prompt
		}
	}

	result := make([]PromptTemplate, 0, len(seen))
	for _, prompt := range seen {
		result = append(result, *prompt)
	}

	return result, diagnostics
}

// loadThemes 加载主题
func (rl *DefaultResourceLoader) loadThemes(paths []string) ([]Theme, []ResourceDiagnostic) {
	var themes []Theme
	var diagnostics []ResourceDiagnostic

	// 默认主题路径
	defaultPaths := []string{
		filepath.Join(rl.agentDir, "themes"),
		filepath.Join(rl.cwd, ".pi", "themes"),
	}

	for _, dir := range defaultPaths {
		if !sliceContains(paths, dir) {
			paths = append([]string{dir}, paths...)
		}
	}

	seen := make(map[string]bool)

	for _, path := range paths {
		resolved := rl.resolveResourcePath(path)
		if seen[resolved] {
			continue
		}
		seen[resolved] = true

		info, err := os.Stat(resolved)
		if err != nil {
			if !os.IsNotExist(err) {
				diagnostics = append(diagnostics, ResourceDiagnostic{
					Type:    "warning",
					Message: fmt.Sprintf("cannot access theme path: %v", err),
					Path:    resolved,
				})
			}
			continue
		}

		if info.IsDir() {
			themes = rl.loadThemesFromDir(resolved, themes, &diagnostics)
		} else if strings.HasSuffix(resolved, ".json") {
			theme := rl.loadThemeFromFile(resolved)
			if theme != nil {
				themes = append(themes, *theme)
			}
		} else {
			diagnostics = append(diagnostics, ResourceDiagnostic{
				Type:    "warning",
				Message: "theme path is not a json file",
				Path:    resolved,
			})
		}
	}

	// 去重
	return rl.dedupeThemes(themes, diagnostics)
}

// loadThemesFromDir 从目录加载主题
func (rl *DefaultResourceLoader) loadThemesFromDir(dir string, themes []Theme, diagnostics *[]ResourceDiagnostic) []Theme {
	entries, err := os.ReadDir(dir)
	if err != nil {
		*diagnostics = append(*diagnostics, ResourceDiagnostic{
			Type:    "warning",
			Message: fmt.Sprintf("cannot read theme directory: %v", err),
			Path:    dir,
		})
		return themes
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if !strings.HasSuffix(name, ".json") {
			continue
		}

		filePath := filepath.Join(dir, name)
		theme := rl.loadThemeFromFile(filePath)
		if theme != nil {
			themes = append(themes, *theme)
		}
	}

	return themes
}

// loadThemeFromFile 从文件加载主题
func (rl *DefaultResourceLoader) loadThemeFromFile(filePath string) *Theme {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil
	}

	name := strings.TrimSuffix(filepath.Base(filePath), ".json")
	_ = content

	return &Theme{
		Name:       name,
		SourcePath: filePath,
	}
}

// dedupeThemes 去重主题
func (rl *DefaultResourceLoader) dedupeThemes(themes []Theme, diagnostics []ResourceDiagnostic) ([]Theme, []ResourceDiagnostic) {
	seen := make(map[string]*Theme)

	for _, theme := range themes {
		name := theme.Name
		if name == "" {
			name = "unnamed"
		}

		if existing, ok := seen[name]; ok {
			diagnostics = append(diagnostics, ResourceDiagnostic{
				Type:    "collision",
				Message: fmt.Sprintf(`name "%s" collision`, name),
				Path:    theme.SourcePath,
			})
			_ = existing
		} else {
			seen[name] = &theme
		}
	}

	result := make([]Theme, 0, len(seen))
	for _, theme := range seen {
		result = append(result, *theme)
	}

	return result, diagnostics
}

// loadProjectContextFiles 加载项目上下文文件
func (rl *DefaultResourceLoader) loadProjectContextFiles() []AgentsFile {
	var files []AgentsFile
	seen := make(map[string]bool)

	// 全局上下文文件
	globalContext := rl.loadContextFileFromDir(rl.agentDir)
	if globalContext != nil {
		files = append(files, *globalContext)
		seen[globalContext.Path] = true
	}

	// 祖先目录上下文文件
	currentDir := rl.cwd
	root := "/"

	var ancestorFiles []AgentsFile

	for {
		contextFile := rl.loadContextFileFromDir(currentDir)
		if contextFile != nil && !seen[contextFile.Path] {
			ancestorFiles = append([]AgentsFile{*contextFile}, ancestorFiles...)
			seen[contextFile.Path] = true
		}

		if currentDir == root {
			break
		}

		parentDir := filepath.Dir(currentDir)
		if parentDir == currentDir {
			break
		}
		currentDir = parentDir
	}

	files = append(files, ancestorFiles...)

	return files
}

// loadContextFileFromDir 从目录加载上下文文件
func (rl *DefaultResourceLoader) loadContextFileFromDir(dir string) *AgentsFile {
	candidates := []string{"AGENTS.md", "CLAUDE.md"}

	for _, filename := range candidates {
		filePath := filepath.Join(dir, filename)
		content, err := os.ReadFile(filePath)
		if err == nil {
			return &AgentsFile{
				Path:    filePath,
				Content: string(content),
			}
		}
	}

	return nil
}

// resolvePromptInput 解析提示输入
func (rl *DefaultResourceLoader) resolvePromptInput(input string, description string) string {
	if input == "" {
		return ""
	}

	if _, err := os.Stat(input); err == nil {
		content, err := os.ReadFile(input)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Could not read %s file %s: %v\n", description, input, err)
			return input
		}
		return string(content)
	}

	return input
}

// discoverSystemPromptFile 发现系统提示文件
func (rl *DefaultResourceLoader) discoverSystemPromptFile() string {
	projectPath := filepath.Join(rl.cwd, ".pi", "SYSTEM.md")
	if _, err := os.Stat(projectPath); err == nil {
		return projectPath
	}

	globalPath := filepath.Join(rl.agentDir, "SYSTEM.md")
	if _, err := os.Stat(globalPath); err == nil {
		return globalPath
	}

	return ""
}

// discoverAppendSystemPromptFile 发现追加系统提示文件
func (rl *DefaultResourceLoader) discoverAppendSystemPromptFile() string {
	projectPath := filepath.Join(rl.cwd, ".pi", "APPEND_SYSTEM.md")
	if _, err := os.Stat(projectPath); err == nil {
		return projectPath
	}

	globalPath := filepath.Join(rl.agentDir, "APPEND_SYSTEM.md")
	if _, err := os.Stat(globalPath); err == nil {
		return globalPath
	}

	return ""
}

// getDefaultSkillPaths 获取默认技能路径
func (rl *DefaultResourceLoader) getDefaultSkillPaths() []string {
	return []string{
		filepath.Join(rl.agentDir, "skills"),
		filepath.Join(rl.cwd, ".pi", "skills"),
	}
}

// getDefaultPromptPaths 获取默认提示模板路径
func (rl *DefaultResourceLoader) getDefaultPromptPaths() []string {
	return []string{
		filepath.Join(rl.agentDir, "prompts"),
		filepath.Join(rl.cwd, ".pi", "prompts"),
	}
}

// getDefaultThemePaths 获取默认主题路径
func (rl *DefaultResourceLoader) getDefaultThemePaths() []string {
	return []string{
		filepath.Join(rl.agentDir, "themes"),
		filepath.Join(rl.cwd, ".pi", "themes"),
	}
}

// mergePaths 合并路径
func (rl *DefaultResourceLoader) mergePaths(primary, additional []string) []string {
	merged := make([]string, 0)
	seen := make(map[string]bool)

	for _, p := range append(primary, additional...) {
		resolved := rl.resolveResourcePath(p)
		if seen[resolved] {
			continue
		}
		seen[resolved] = true
		merged = append(merged, resolved)
	}

	return merged
}

// resolveResourcePath 解析资源路径
func (rl *DefaultResourceLoader) resolveResourcePath(p string) string {
	trimmed := strings.TrimSpace(p)
	expanded := trimmed

	home, _ := os.UserHomeDir()

	if trimmed == "~" {
		expanded = home
	} else if strings.HasPrefix(trimmed, "~/") {
		expanded = filepath.Join(home, trimmed[2:])
	} else if strings.HasPrefix(trimmed, "~") {
		expanded = filepath.Join(home, trimmed[1:])
	}

	return filepath.Join(rl.cwd, expanded)
}

// 辅助函数

func copySkills(skills []SkillInfo) []SkillInfo {
	result := make([]SkillInfo, len(skills))
	copy(result, skills)
	return result
}

func copyPrompts(prompts []PromptTemplate) []PromptTemplate {
	result := make([]PromptTemplate, len(prompts))
	copy(result, prompts)
	return result
}

func copyThemes(themes []Theme) []Theme {
	result := make([]Theme, len(themes))
	copy(result, themes)
	return result
}

func copyAgentsFiles(files []AgentsFile) []AgentsFile {
	result := make([]AgentsFile, len(files))
	copy(result, files)
	return result
}

func copyDiagnostics(diagnostics []ResourceDiagnostic) []ResourceDiagnostic {
	result := make([]ResourceDiagnostic, len(diagnostics))
	copy(result, diagnostics)
	return result
}

func copyStrings(strs []string) []string {
	result := make([]string, len(strs))
	copy(result, strs)
	return result
}

// sliceContains 检查字符串切片是否包含指定元素
func sliceContains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// Ensure DefaultResourceLoader implements ResourceLoader
var _ ResourceLoader = (*DefaultResourceLoader)(nil)
