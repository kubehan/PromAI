package session

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const (
	// MaxNameLength 技能名称最大长度
	MaxNameLength = 64
	// MaxDescriptionLength 描述最大长度
	MaxDescriptionLength = 1024
)

// IgnoreFileNames 忽略文件名列表
var IgnoreFileNames = []string{".gitignore", ".ignore", ".fdignore"}

// SkillsResult 技能结果
type SkillsResult struct {
	Skills      []SkillInfo        `json:"skills"`
	Diagnostics []ResourceDiagnostic `json:"diagnostics,omitempty"`
}

// SkillInfo 技能信息
type SkillInfo struct {
	Name                 string `json:"name"`
	Description          string `json:"description"`
	FilePath             string `json:"filePath"`
	BaseDir              string `json:"baseDir"`
	Source               string `json:"source"`
	DisableModelInvocation bool  `json:"disableModelInvocation"`
}

// SkillFrontmatter 技能前置元数据
type SkillFrontmatter struct {
	Name                 string `json:"name,omitempty"`
	Description          string `json:"description,omitempty"`
	DisableModelInvocation bool  `json:"disable-model-invocation,omitempty"`
}

// LoadSkillsResult 加载技能结果
type LoadSkillsResult struct {
	Skills      []SkillInfo
	Diagnostics []ResourceDiagnostic
}

// LoadSkillsFromDirOptions 从目录加载技能的选项
type LoadSkillsFromDirOptions struct {
	Dir    string
	Source string
}

// LoadSkillsFromDir 从目录加载技能
func LoadSkillsFromDir(options LoadSkillsFromDirOptions) LoadSkillsResult {
	return loadSkillsFromDirInternal(options.Dir, options.Source, true, nil, "")
}

// loadSkillsFromDirInternal 内部加载技能函数
func loadSkillsFromDirInternal(dir string, source string, includeRootFiles bool, ignoreMatcher *IgnoreMatcher, rootDir string) LoadSkillsResult {
	var skills []SkillInfo
	var diagnostics []ResourceDiagnostic

	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return LoadSkillsResult{Skills: skills, Diagnostics: diagnostics}
	}

	root := rootDir
	if root == "" {
		root = dir
	}

	ig := ignoreMatcher
	if ig == nil {
		ig = NewIgnoreMatcher()
	}
	addIgnoreRules(ig, dir, root)

	entries, err := os.ReadDir(dir)
	if err != nil {
		return LoadSkillsResult{Skills: skills, Diagnostics: diagnostics}
	}

	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".") {
			continue
		}

		// Skip node_modules to avoid scanning dependencies
		if entry.Name() == "node_modules" {
			continue
		}

		fullPath := filepath.Join(dir, entry.Name())

		// For symlinks, check if they point to a directory and follow them
		isDirectory := entry.IsDir()
		isFile := false
		if entry.Type()&os.ModeSymlink != 0 {
			info, err := os.Stat(fullPath)
			if err != nil {
				// Broken symlink, skip it
				continue
			}
			isDirectory = info.IsDir()
			isFile = !isDirectory
		} else if !isDirectory {
			isFile = true
		}

		relPath, _ := filepath.Rel(root, fullPath)
		relPath = toPosixPath(relPath)
		ignorePath := relPath
		if isDirectory {
			ignorePath += "/"
		}
		if ig.Ignores(ignorePath) {
			continue
		}

		if isDirectory {
			subResult := loadSkillsFromDirInternal(fullPath, source, false, ig, root)
			skills = append(skills, subResult.Skills...)
			diagnostics = append(diagnostics, subResult.Diagnostics...)
			continue
		}

		if !isFile {
			continue
		}

		isRootMd := includeRootFiles && strings.HasSuffix(entry.Name(), ".md")
		isSkillMd := !includeRootFiles && entry.Name() == "SKILL.md"
		if !isRootMd && !isSkillMd {
			continue
		}

		result := loadSkillFromFile(fullPath, source)
		if result.Skill != nil {
			skills = append(skills, *result.Skill)
		}
		diagnostics = append(diagnostics, result.Diagnostics...)
	}

	return LoadSkillsResult{Skills: skills, Diagnostics: diagnostics}
}

// loadSkillFromFileResult 从文件加载技能的结果
type loadSkillFromFileResult struct {
	Skill       *SkillInfo
	Diagnostics []ResourceDiagnostic
}

// loadSkillFromFile 从文件加载技能
func loadSkillFromFile(filePath string, source string) loadSkillFromFileResult {
	var diagnostics []ResourceDiagnostic

	rawContent, err := os.ReadFile(filePath)
	if err != nil {
		message := fmt.Sprintf("failed to read skill file: %v", err)
		diagnostics = append(diagnostics, ResourceDiagnostic{
			Type:    "warning",
			Message: message,
			Path:    filePath,
		})
		return loadSkillFromFileResult{Skill: nil, Diagnostics: diagnostics}
	}

	frontmatter, body := parseFrontmatter(string(rawContent))
	skillDir := filepath.Dir(filePath)
	parentDirName := filepath.Base(skillDir)

	// Validate description
	descErrors := validateDescription(frontmatter.Description)
	for _, error := range descErrors {
		diagnostics = append(diagnostics, ResourceDiagnostic{
			Type:    "warning",
			Message: error,
			Path:    filePath,
		})
	}

	// Use name from frontmatter, or fall back to parent directory name
	name := frontmatter.Name
	if name == "" {
		name = parentDirName
	}

	// Validate name
	nameErrors := validateName(name, parentDirName)
	for _, error := range nameErrors {
		diagnostics = append(diagnostics, ResourceDiagnostic{
			Type:    "warning",
			Message: error,
			Path:    filePath,
		})
	}

	// Still load skill even with warnings (unless description is completely missing)
	if frontmatter.Description == "" || strings.TrimSpace(frontmatter.Description) == "" {
		return loadSkillFromFileResult{Skill: nil, Diagnostics: diagnostics}
	}

	_ = body // Not used in current implementation

	return loadSkillFromFileResult{
		Skill: &SkillInfo{
			Name:                 name,
			Description:          frontmatter.Description,
			FilePath:             filePath,
			BaseDir:              skillDir,
			Source:               source,
			DisableModelInvocation: frontmatter.DisableModelInvocation,
		},
		Diagnostics: diagnostics,
	}
}

// validateName 验证技能名称
func validateName(name string, parentDirName string) []string {
	var errors []string

	if name != parentDirName {
		errors = append(errors, fmt.Sprintf(`name "%s" does not match parent directory "%s"`, name, parentDirName))
	}

	if len(name) > MaxNameLength {
		errors = append(errors, fmt.Sprintf(`name exceeds %d characters (%d)`, MaxNameLength, len(name)))
	}

	matched, _ := regexp.MatchString(`^[a-z0-9-]+$`, name)
	if !matched {
		errors = append(errors, "name contains invalid characters (must be lowercase a-z, 0-9, hyphens only)")
	}

	if strings.HasPrefix(name, "-") || strings.HasSuffix(name, "-") {
		errors = append(errors, "name must not start or end with a hyphen")
	}

	if strings.Contains(name, "--") {
		errors = append(errors, "name must not contain consecutive hyphens")
	}

	return errors
}

// validateDescription 验证描述
func validateDescription(description string) []string {
	var errors []string

	if description == "" || strings.TrimSpace(description) == "" {
		errors = append(errors, "description is required")
	} else if len(description) > MaxDescriptionLength {
		errors = append(errors, fmt.Sprintf(`description exceeds %d characters (%d)`, MaxDescriptionLength, len(description)))
	}

	return errors
}

// FormatSkillsForPrompt 格式化技能以包含在系统提示中
func FormatSkillsForPrompt(skills []SkillInfo) string {
	var visibleSkills []SkillInfo
	for _, s := range skills {
		if !s.DisableModelInvocation {
			visibleSkills = append(visibleSkills, s)
		}
	}

	if len(visibleSkills) == 0 {
		return ""
	}

	lines := []string{
		"",
		"",
		"The following skills provide specialized instructions for specific tasks.",
		"Use read tool to load a skill's file when task matches its description.",
		"When a skill file references a relative path, resolve it against skill directory (parent of SKILL.md / dirname of the path) and use that absolute path in tool commands.",
		"",
		"<available_skills>",
	}

	for _, skill := range visibleSkills {
		lines = append(lines, "  <skill>")
		lines = append(lines, fmt.Sprintf("    <name>%s</name>", escapeXml(skill.Name)))
		lines = append(lines, fmt.Sprintf("    <description>%s</description>", escapeXml(skill.Description)))
		lines = append(lines, fmt.Sprintf("    <location>%s</location>", escapeXml(skill.FilePath)))
		lines = append(lines, "  </skill>")
	}

	lines = append(lines, "</available_skills>")

	return strings.Join(lines, "\n")
}

// escapeXml 转义 XML 特殊字符
func escapeXml(str string) string {
	str = strings.ReplaceAll(str, "&", "&amp;")
	str = strings.ReplaceAll(str, "<", "&lt;")
	str = strings.ReplaceAll(str, ">", "&gt;")
	str = strings.ReplaceAll(str, "\"", "&quot;")
	str = strings.ReplaceAll(str, "'", "&apos;")
	return str
}

// LoadSkillsOptions 加载技能选项
type LoadSkillsOptions struct {
	Cwd             string
	AgentDir        string
	SkillPaths      []string
	IncludeDefaults *bool
}

// LoadSkills 从所有配置位置加载技能
func LoadSkills(options LoadSkillsOptions) LoadSkillsResult {
	cwd := options.Cwd
	if cwd == "" {
		cwd, _ = os.Getwd()
	}

	agentDir := options.AgentDir
	if agentDir == "" {
		agentDir = GetAgentDir()
	}

	includeDefaults := true
	if options.IncludeDefaults != nil {
		includeDefaults = *options.IncludeDefaults
	}

	skillMap := make(map[string]SkillInfo)
	realPathSet := make(map[string]bool)
	var allDiagnostics []ResourceDiagnostic
	var collisionDiagnostics []ResourceDiagnostic

	addSkills := func(result LoadSkillsResult) {
		allDiagnostics = append(allDiagnostics, result.Diagnostics...)
		for _, skill := range result.Skills {
			// Resolve symlinks to detect duplicate files
			realPath, err := filepath.EvalSymlinks(skill.FilePath)
			if err != nil {
				realPath = skill.FilePath
			}

			// Skip silently if we've already loaded this exact file (via symlink)
			if realPathSet[realPath] {
				continue
			}

			existing, exists := skillMap[skill.Name]
			if exists {
				collisionDiagnostics = append(collisionDiagnostics, ResourceDiagnostic{
					Type:    "collision",
					Message: fmt.Sprintf(`name "%s" collision`, skill.Name),
					Path:    skill.FilePath,
					Collision: &CollisionInfo{
						ResourceType: "skill",
						Name:         skill.Name,
						WinnerPath:   existing.FilePath,
						LoserPath:    skill.FilePath,
					},
				})
			} else {
				skillMap[skill.Name] = skill
				realPathSet[realPath] = true
			}
		}
	}

	if includeDefaults {
		userSkillsDir := filepath.Join(agentDir, "skills")
		projectSkillsDir := filepath.Join(cwd, ".pi", "skills")
		addSkills(loadSkillsFromDirInternal(userSkillsDir, "user", true, nil, ""))
		addSkills(loadSkillsFromDirInternal(projectSkillsDir, "project", true, nil, ""))
	}

	userSkillsDir := filepath.Join(agentDir, "skills")
	projectSkillsDir := filepath.Join(cwd, ".pi", "skills")

	isUnderPath := func(target string, root string) bool {
		normalizedRoot := filepath.Clean(root)
		if target == normalizedRoot {
			return true
		}
		prefix := normalizedRoot
		if !strings.HasSuffix(prefix, string(filepath.Separator)) {
			prefix += string(filepath.Separator)
		}
		return strings.HasPrefix(target, prefix)
	}

	getSource := func(resolvedPath string) string {
		if !includeDefaults {
			if isUnderPath(resolvedPath, userSkillsDir) {
				return "user"
			}
			if isUnderPath(resolvedPath, projectSkillsDir) {
				return "project"
			}
		}
		return "path"
	}

	for _, rawPath := range options.SkillPaths {
		resolvedPath := resolveSkillPath(rawPath, cwd)
		if _, err := os.Stat(resolvedPath); os.IsNotExist(err) {
			allDiagnostics = append(allDiagnostics, ResourceDiagnostic{
				Type:    "warning",
				Message: "skill path does not exist",
				Path:    resolvedPath,
			})
			continue
		}

		info, err := os.Stat(resolvedPath)
		if err != nil {
			message := fmt.Sprintf("failed to read skill path: %v", err)
			allDiagnostics = append(allDiagnostics, ResourceDiagnostic{
				Type:    "warning",
				Message: message,
				Path:    resolvedPath,
			})
			continue
		}

		source := getSource(resolvedPath)
		if info.IsDir() {
			addSkills(loadSkillsFromDirInternal(resolvedPath, source, true, nil, ""))
		} else if info.Mode().IsRegular() && strings.HasSuffix(resolvedPath, ".md") {
			result := loadSkillFromFile(resolvedPath, source)
			if result.Skill != nil {
				addSkills(LoadSkillsResult{
					Skills:      []SkillInfo{*result.Skill},
					Diagnostics: result.Diagnostics,
				})
			} else {
				allDiagnostics = append(allDiagnostics, result.Diagnostics...)
			}
		} else {
			allDiagnostics = append(allDiagnostics, ResourceDiagnostic{
				Type:    "warning",
				Message: "skill path is not a markdown file",
				Path:    resolvedPath,
			})
		}
	}

	skills := make([]SkillInfo, 0, len(skillMap))
	for _, skill := range skillMap {
		skills = append(skills, skill)
	}

	return LoadSkillsResult{
		Skills:      skills,
		Diagnostics: append(allDiagnostics, collisionDiagnostics...),
	}
}

// normalizePath 规范化路径
func normalizePath(input string) string {
	trimmed := strings.TrimSpace(input)
	if trimmed == "~" {
		home, _ := os.UserHomeDir()
		return home
	}
	if strings.HasPrefix(trimmed, "~/") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, trimmed[2:])
	}
	if strings.HasPrefix(trimmed, "~") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, trimmed[1:])
	}
	return trimmed
}

// resolveSkillPath 解析技能路径
func resolveSkillPath(p string, cwd string) string {
	normalized := normalizePath(p)
	if filepath.IsAbs(normalized) {
		return normalized
	}
	return filepath.Join(cwd, normalized)
}

// toPosixPath 转换为 POSIX 路径
func toPosixPath(p string) string {
	return strings.ReplaceAll(p, string(filepath.Separator), "/")
}

// IgnoreMatcher 忽略匹配器
type IgnoreMatcher struct {
	patterns   []string
	negations  []string
}

// NewIgnoreMatcher 创建新的忽略匹配器
func NewIgnoreMatcher() *IgnoreMatcher {
	return &IgnoreMatcher{
		patterns:  make([]string, 0),
		negations: make([]string, 0),
	}
}

// Add 添加忽略规则
func (ig *IgnoreMatcher) Add(patterns []string) {
	for _, pattern := range patterns {
		if strings.HasPrefix(pattern, "!") {
			ig.negations = append(ig.negations, pattern[1:])
		} else {
			ig.patterns = append(ig.patterns, pattern)
		}
	}
}

// Ignores 检查路径是否被忽略
func (ig *IgnoreMatcher) Ignores(path string) bool {
	// 先检查否定规则
	for _, negation := range ig.negations {
		if matchIgnorePattern(path, negation) {
			return false
		}
	}
	
	// 检查正向规则
	for _, pattern := range ig.patterns {
		if matchIgnorePattern(path, pattern) {
			return true
		}
	}
	return false
}

// matchIgnorePattern 检查路径是否匹配忽略模式
func matchIgnorePattern(path string, pattern string) bool {
	// 处理目录通配符 **
	if strings.Contains(pattern, "**") {
		parts := strings.Split(pattern, "**")
		if len(parts) == 2 {
			prefix := parts[0]
			suffix := parts[1]
			if prefix != "" && !strings.HasPrefix(path, prefix) {
				return false
			}
			if suffix != "" && !strings.HasSuffix(path, suffix) {
				return false
			}
			return true
		}
	}
	
	// 处理 * 通配符
	if strings.Contains(pattern, "*") {
		regexPattern := "^" + strings.ReplaceAll(
			strings.ReplaceAll(pattern, ".", "\\."),
			"*", ".*",
		) + "$"
		matched, _ := regexp.MatchString(regexPattern, path)
		return matched
	}
	
	// 处理目录前缀匹配
	if strings.HasSuffix(pattern, "/") {
		return strings.HasPrefix(path+"/", pattern)
	}
	
	// 精确匹配或前缀匹配
	return path == pattern || strings.HasPrefix(path, pattern+"/")
}

// addIgnoreRules 添加忽略规则
func addIgnoreRules(ig *IgnoreMatcher, dir string, rootDir string) {
	relativeDir, _ := filepath.Rel(rootDir, dir)
	prefix := ""
	if relativeDir != "" && relativeDir != "." {
		prefix = toPosixPath(relativeDir) + "/"
	}

	for _, filename := range IgnoreFileNames {
		ignorePath := filepath.Join(dir, filename)
		if _, err := os.Stat(ignorePath); os.IsNotExist(err) {
			continue
		}

		content, err := os.ReadFile(ignorePath)
		if err != nil {
			continue
		}

		// 处理 \r\n 和 \n 行分隔符
		contentStr := strings.ReplaceAll(string(content), "\r\n", "\n")
		lines := strings.Split(contentStr, "\n")
		patterns := make([]string, 0)
		for _, line := range lines {
			pattern := prefixIgnorePattern(line, prefix)
			if pattern != "" {
				patterns = append(patterns, pattern)
			}
		}

		if len(patterns) > 0 {
			ig.Add(patterns)
		}
	}
}

// prefixIgnorePattern 为忽略模式添加前缀
func prefixIgnorePattern(line string, prefix string) string {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return ""
	}
	if strings.HasPrefix(trimmed, "#") && !strings.HasPrefix(trimmed, "\\#") {
		return ""
	}

	pattern := trimmed
	negated := false

	if strings.HasPrefix(pattern, "!") {
		negated = true
		pattern = pattern[1:]
	} else if strings.HasPrefix(pattern, "\\!") {
		pattern = pattern[1:]
	}

	if strings.HasPrefix(pattern, "/") {
		pattern = pattern[1:]
	}

	prefixed := prefix + pattern
	if negated {
		return "!" + prefixed
	}
	return prefixed
}

// parseFrontmatter 解析前置元数据
func parseFrontmatter(content string) (SkillFrontmatter, string) {
	lines := strings.Split(content, "\n")
	
	if len(lines) == 0 {
		return SkillFrontmatter{}, content
	}

	// Check for YAML frontmatter delimiter
	if !strings.HasPrefix(strings.TrimSpace(lines[0]), "---") {
		return SkillFrontmatter{}, content
	}

	var frontmatterLines []string
	var bodyLines []string
	inFrontmatter := true

	for i, line := range lines {
		if i == 0 {
			continue // Skip first ---
		}

		trimmed := strings.TrimSpace(line)
		if trimmed == "---" {
			inFrontmatter = false
			continue
		}

		if inFrontmatter {
			frontmatterLines = append(frontmatterLines, line)
		} else {
			bodyLines = append(bodyLines, line)
		}
	}

	frontmatter := SkillFrontmatter{}
	for _, line := range frontmatterLines {
		if strings.Contains(line, ":") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				key := strings.TrimSpace(parts[0])
				value := strings.TrimSpace(parts[1])
				
				// Remove quotes if present
				if strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"") {
					value = value[1 : len(value)-1]
				} else if strings.HasPrefix(value, "'") && strings.HasSuffix(value, "'") {
					value = value[1 : len(value)-1]
				}

				switch key {
				case "name":
					frontmatter.Name = value
				case "description":
					frontmatter.Description = value
				case "disable-model-invocation":
					frontmatter.DisableModelInvocation = strings.ToLower(value) == "true" || value == "1"
				}
			}
		}
	}

	body := strings.Join(bodyLines, "\n")
	return frontmatter, body
}

// ParsedSkillBlock 解析的技能块
type ParsedSkillBlock struct {
	Name        string `json:"name"`
	Location    string `json:"location"`
	Content     string `json:"content"`
	UserMessage string `json:"userMessage,omitempty"`
}

// ParseSkillBlock 解析技能块
func ParseSkillBlock(text string) *ParsedSkillBlock {
	// 使用正则表达式
	// 格式: <skill name="..." location="...">\n...\n</skill>
	re := regexp.MustCompile(`^<skill name="([^"]+)" location="([^"]+)">\n([\s\S]*?)\n</skill>(?:\n\n([\s\S]+))?$`)
	matches := re.FindStringSubmatch(text)
	if matches == nil {
		return nil
	}
	userMessage := strings.Trim(matches[4], " ")
	return &ParsedSkillBlock{
		Name:       matches[1],
		Location:   matches[2],
		Content:    matches[3],
		UserMessage: userMessage,
	}
}