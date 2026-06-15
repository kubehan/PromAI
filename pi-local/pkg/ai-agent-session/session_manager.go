package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jay-y/pi/pkg/ai"
)

const CurrentSessionVersion = 3

// SessionHeader 会话头部
type SessionHeader struct {
	Type         string `json:"type"`
	Version      int    `json:"version,omitempty"`
	ID           string `json:"id"`
	Timestamp    string `json:"timestamp"`
	Cwd          string `json:"cwd"`
	ParentSession string `json:"parentSession,omitempty"`
}

// SessionEntryBase 会话条目基础结构
type SessionEntryBase struct {
	Type      string `json:"type"`
	ID        string `json:"id"`
	ParentID  string `json:"parentId"`
	Timestamp string `json:"timestamp"`
}

// SessionMessageEntry 消息条目
type SessionMessageEntry struct {
	SessionEntryBase
	Message map[string]any `json:"message"`
}

// ThinkingLevelChangeEntry 思考级别变化条目
type ThinkingLevelChangeEntry struct {
	SessionEntryBase
	ThinkingLevel string `json:"thinkingLevel"`
}

// ModelChangeEntry 模型变化条目
type ModelChangeEntry struct {
	SessionEntryBase
	Provider string `json:"provider"`
	ModelID  string `json:"modelId"`
}

// CompactionEntry 压缩条目
type CompactionEntry struct {
	SessionEntryBase
	Summary             string      `json:"summary"`
	FirstKeptEntryID    string      `json:"firstKeptEntryId"`
	FirstKeptEntryIndex *int        `json:"firstKeptEntryIndex,omitempty"` // v1 sessions
	TokensBefore        int         `json:"tokensBefore"`
	Details             interface{} `json:"details,omitempty"`
	FromHook            bool        `json:"fromHook,omitempty"`
}

// CustomEntry 自定义条目
type CustomEntry struct {
	SessionEntryBase
	CustomType string      `json:"customType"`
	Data       interface{} `json:"data,omitempty"`
}

// LabelEntry 标签条目
type LabelEntry struct {
	SessionEntryBase
	TargetID string  `json:"targetId"`
	Label    *string `json:"label"`
}

// SessionInfoEntry 会话信息条目
type SessionInfoEntry struct {
	SessionEntryBase
	Name string `json:"name,omitempty"`
}

// CustomMessageEntry 自定义消息条目
type CustomMessageEntry struct {
	SessionEntryBase
	CustomType string      `json:"customType"`
	Content    interface{} `json:"content"`
	Details    interface{} `json:"details,omitempty"`
	Display    bool        `json:"display"`
}

// BranchSummaryEntry 分支摘要条目（扩展SessionEntryBase）
type BranchSummaryEntry struct {
	SessionEntryBase
	FromID   string      `json:"fromId"`
	Summary  string      `json:"summary"`
	Details  interface{} `json:"details,omitempty"`
	FromHook bool        `json:"fromHook,omitempty"`
}

// SessionContext 文件会话上下文（用于session-manager）
type SessionContext struct {
	Messages      []ai.Message `json:"messages"`
	ThinkingLevel string              `json:"thinkingLevel"`
	Model         *ModelInfo          `json:"model"`
}

// NewSessionOptions 新会话选项
type NewSessionOptions struct {
	ParentSession string `json:"parentSession,omitempty"`
}

// SessionEntry 会话条目接口（用于session-manager内部）
type SessionEntry interface {
	GetID() string
	GetParentID() string
	GetTimestamp() string
	GetType() string
}

// GetID 获取条目ID
func (e SessionEntryBase) GetID() string {
	return e.ID
}

// GetParentID 获取父条目ID
func (e SessionEntryBase) GetParentID() string {
	return e.ParentID
}

// GetTimestamp 获取时间戳
func (e SessionEntryBase) GetTimestamp() string {
	return e.Timestamp
}

// GetType 获取类型
func (e SessionEntryBase) GetType() string {
	return e.Type
}

// FileEntry 文件条目类型
type FileEntry interface {
	GetType() string
}

// GetType 实现 FileEntry 接口
func (h *SessionHeader) GetType() string {
	return h.Type
}

// SessionTreeNode 会话树节点
type SessionTreeNode struct {
	Entry    SessionEntry       `json:"entry"`
	Children []SessionTreeNode `json:"children"`
	Label    *string           `json:"label,omitempty"`
}

// ModelInfo 模型信息
type ModelInfo struct {
	Provider string `json:"provider"`
	ModelID  string `json:"modelId"`
}

// SessionInfo 会话信息
type SessionInfo struct {
	Path            string    `json:"path"`
	ID              string    `json:"id"`
	Cwd             string    `json:"cwd"`
	Name            *string   `json:"name,omitempty"`
	ParentSessionPath string   `json:"parentSessionPath,omitempty"`
	Created         time.Time `json:"created"`
	Modified        time.Time `json:"modified"`
	MessageCount    int       `json:"messageCount"`
	FirstMessage    string    `json:"firstMessage"`
	AllMessagesText string    `json:"allMessagesText"`
}

// SessionListProgress 会话列表进度回调
type SessionListProgress func(loaded, total int)

// SessionManager 会话管理器
type SessionManager struct {
	sessionID    string
	sessionFile  string
	sessionDir   string
	cwd          string
	persist      bool
	flushed      bool
	fileEntries  []FileEntry
	byID         map[string]SessionEntry
	labelsByID   map[string]string
	leafID       string
}

// generateID 生成唯一ID
func generateID(byID map[string]SessionEntry) string {
	for i := 0; i < 100; i++ {
		id := uuid.New().String()[:8]
		if _, exists := byID[id]; !exists {
			return id
		}
	}
	return uuid.New().String()
}

// migrateV1ToV2 从v1迁移到v2
func migrateV1ToV2(entries []FileEntry) bool {
	var prevID string

	migrated := false
	for _, entry := range entries {
		switch e := entry.(type) {
		case *SessionHeader:
			e.Version = 2
			migrated = true
		case *SessionMessageEntry:
			e.ID = generateID(make(map[string]SessionEntry))
			e.ParentID = prevID
			prevID = e.ID
			migrated = true
		case *ThinkingLevelChangeEntry:
			e.ID = generateID(make(map[string]SessionEntry))
			e.ParentID = prevID
			prevID = e.ID
			migrated = true
		case *ModelChangeEntry:
			e.ID = generateID(make(map[string]SessionEntry))
			e.ParentID = prevID
			prevID = e.ID
			migrated = true
		case *CompactionEntry:
			e.ID = generateID(make(map[string]SessionEntry))
			e.ParentID = prevID
			prevID = e.ID
			migrated = true
		case *BranchSummaryEntry:
			e.ID = generateID(make(map[string]SessionEntry))
			e.ParentID = prevID
			prevID = e.ID
			migrated = true
		case *CustomEntry:
			e.ID = generateID(make(map[string]SessionEntry))
			e.ParentID = prevID
			prevID = e.ID
			migrated = true
		case *LabelEntry:
			e.ID = generateID(make(map[string]SessionEntry))
			e.ParentID = prevID
			prevID = e.ID
			migrated = true
		case *SessionInfoEntry:
			e.ID = generateID(make(map[string]SessionEntry))
			e.ParentID = prevID
			prevID = e.ID
			migrated = true
		case *CustomMessageEntry:
			e.ID = generateID(make(map[string]SessionEntry))
			e.ParentID = prevID
			prevID = e.ID
			migrated = true
		}
	}

	// 转换 firstKeptEntryIndex 到 firstKeptEntryId
	for _, entry := range entries {
		if compEntry, ok := entry.(*CompactionEntry); ok {
			if compEntry.FirstKeptEntryIndex != nil {
				index := *compEntry.FirstKeptEntryIndex
				if index >= 0 && index < len(entries) {
					if targetEntry, ok := entries[index].(SessionEntry); ok && targetEntry.GetType() != "session" {
						compEntry.FirstKeptEntryID = targetEntry.GetID()
					}
				}
				compEntry.FirstKeptEntryIndex = nil
			}
		}
	}

	return migrated
}

// migrateV2ToV3 从v2迁移到v3
func migrateV2ToV3(entries []FileEntry) bool {
	migrated := false
	for _, entry := range entries {
		if msgEntry, ok := entry.(*SessionMessageEntry); ok {
			if msgEntry.Message["role"] == "hookMessage" {
				msgEntry.Message["role"] = "custom"
				migrated = true
			}
		}
	}
	return migrated
}

// migrateToCurrentVersion 迁移到当前版本
func migrateToCurrentVersion(entries []FileEntry) bool {
	var header *SessionHeader
	for _, entry := range entries {
		if h, ok := entry.(*SessionHeader); ok {
			header = h
			break
		}
	}

	version := 1
	if header != nil {
		version = header.Version
	}

	if version >= CurrentSessionVersion {
		return false
	}

	migrated := false
	if version < 2 {
		migrated = migrateV1ToV2(entries) || migrated
	}
	if version < 3 {
		migrated = migrateV2ToV3(entries) || migrated
	}

	if header != nil {
		header.Version = CurrentSessionVersion
	}

	return migrated
}

// parseSessionEntries 解析会话条目
func parseSessionEntries(content string) []FileEntry {
	entries := []FileEntry{}
	lines := strings.Split(strings.TrimSpace(content), "\n")

	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}

		var entry map[string]interface{}
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue // 跳过畸形行
		}

		entryType, ok := entry["type"].(string)
		if !ok {
			continue
		}

		switch entryType {
		case "session":
			header := &SessionHeader{}
			if err := json.Unmarshal([]byte(line), header); err == nil {
				entries = append(entries, header)
			}
		case "message":
			msgEntry := &SessionMessageEntry{}
			if err := json.Unmarshal([]byte(line), msgEntry); err == nil {
				entries = append(entries, msgEntry)
			}
		case "thinking_level_change":
			thinkingEntry := &ThinkingLevelChangeEntry{}
			if err := json.Unmarshal([]byte(line), thinkingEntry); err == nil {
				entries = append(entries, thinkingEntry)
			}
		case "model_change":
			modelEntry := &ModelChangeEntry{}
			if err := json.Unmarshal([]byte(line), modelEntry); err == nil {
				entries = append(entries, modelEntry)
			}
		case "compaction":
			compactionEntry := &CompactionEntry{}
			if err := json.Unmarshal([]byte(line), compactionEntry); err == nil {
				entries = append(entries, compactionEntry)
			}
		case "branch_summary":
			branchEntry := &BranchSummaryEntry{}
			if err := json.Unmarshal([]byte(line), branchEntry); err == nil {
				entries = append(entries, branchEntry)
			}
		case "custom":
			customEntry := &CustomEntry{}
			if err := json.Unmarshal([]byte(line), customEntry); err == nil {
				entries = append(entries, customEntry)
			}
		case "label":
			labelEntry := &LabelEntry{}
			if err := json.Unmarshal([]byte(line), labelEntry); err == nil {
				entries = append(entries, labelEntry)
			}
		case "session_info":
			sessionInfoEntry := &SessionInfoEntry{}
			if err := json.Unmarshal([]byte(line), sessionInfoEntry); err == nil {
				entries = append(entries, sessionInfoEntry)
			}
		case "custom_message":
			customMsgEntry := &CustomMessageEntry{}
			if err := json.Unmarshal([]byte(line), customMsgEntry); err == nil {
				entries = append(entries, customMsgEntry)
			}
		}
	}

	return entries
}

// loadEntriesFromFile 从文件加载条目
func loadEntriesFromFile(filePath string) []FileEntry {
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return []FileEntry{}
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		return []FileEntry{}
	}

	entries := parseSessionEntries(string(content))

	// 验证会话头部
	if len(entries) == 0 {
		return entries
	}

	header, ok := entries[0].(*SessionHeader)
	if !ok || header.ID == "" {
		return []FileEntry{}
	}

	return entries
}

// getDefaultSessionDir 获取默认会话目录
func getDefaultSessionDir(cwd string) string {
	safePath := "--" + strings.ReplaceAll(strings.TrimPrefix(cwd, "/"), "/", "-") + "--"
	sessionDir := filepath.Join(os.Getenv("HOME"), ".pi", "agent", "sessions", safePath)

	if _, err := os.Stat(sessionDir); os.IsNotExist(err) {
		os.MkdirAll(sessionDir, 0755)
	}

	return sessionDir
}

// findMostRecentSession 查找最近的会话
func findMostRecentSession(sessionDir string) string {
	files, err := os.ReadDir(sessionDir)
	if err != nil {
		return ""
	}

	var sessionFiles []struct {
		Path    string
		ModTime time.Time
	}

	for _, file := range files {
		if !file.IsDir() && strings.HasSuffix(file.Name(), ".jsonl") {
			filePath := filepath.Join(sessionDir, file.Name())
			info, err := file.Info()
			if err != nil {
				continue
			}
			sessionFiles = append(sessionFiles, struct {
				Path    string
				ModTime time.Time
			}{Path: filePath, ModTime: info.ModTime()})
		}
	}

	if len(sessionFiles) == 0 {
		return ""
	}

	sort.Slice(sessionFiles, func(i, j int) bool {
		return sessionFiles[i].ModTime.After(sessionFiles[j].ModTime)
	})

	return sessionFiles[0].Path
}

// newSessionManager 创建新的会话管理器
func newSessionManager(cwd, sessionDir, sessionFile string, persist bool) *SessionManager {
	if persist && sessionDir != "" {
		if _, err := os.Stat(sessionDir); os.IsNotExist(err) {
			os.MkdirAll(sessionDir, 0755)
		}
	}

	manager := &SessionManager{
		cwd:          cwd,
		sessionDir:   sessionDir,
		persist:      persist,
		byID:         make(map[string]SessionEntry),
		labelsByID:   make(map[string]string),
		fileEntries:  []FileEntry{},
	}

	if sessionFile != "" {
		manager.setSessionFile(sessionFile)
	} else {
		manager.newSession(nil)
	}

	return manager
}

// setSessionFile 设置会话文件
func (m *SessionManager) setSessionFile(sessionFile string) {
	m.sessionFile = filepath.Clean(sessionFile)

	if _, err := os.Stat(m.sessionFile); err == nil {
		m.fileEntries = loadEntriesFromFile(m.sessionFile)

		// 如果文件为空或损坏，重新开始
		if len(m.fileEntries) == 0 {
			explicitPath := m.sessionFile
			m.newSession(nil)
			m.sessionFile = explicitPath
			m._rewriteFile()
			m.flushed = true
			return
		}

		var header *SessionHeader
		for _, entry := range m.fileEntries {
			if h, ok := entry.(*SessionHeader); ok {
				header = h
				break
			}
		}

		if header != nil {
			m.sessionID = header.ID
		} else {
			m.sessionID = uuid.New().String()
		}

		if migrateToCurrentVersion(m.fileEntries) {
			m._rewriteFile()
		}

		m._buildIndex()
		m.flushed = true
	} else {
		explicitPath := m.sessionFile
		m.newSession(nil)
		m.sessionFile = explicitPath
	}
}

// SetSessionFile 设置会话文件（公共方法）
func (m *SessionManager) SetSessionFile(sessionFile string) {
	m.setSessionFile(sessionFile)
}

// NewSession 创建新会话（公共方法）
func (m *SessionManager) NewSession(options *NewSessionOptions) string {
	return m.newSession(options)
}

// newSession 创建新会话
func (m *SessionManager) newSession(options *NewSessionOptions) string {
	m.sessionID = uuid.New().String()
	timestamp := time.Now().Format(time.RFC3339)

	header := &SessionHeader{
		Type:         "session",
		Version:      CurrentSessionVersion,
		ID:           m.sessionID,
		Timestamp:    timestamp,
		Cwd:          m.cwd,
		ParentSession: "",
	}

	if options != nil {
		header.ParentSession = options.ParentSession
	}

	m.fileEntries = []FileEntry{header}
	m.byID = make(map[string]SessionEntry)
	m.labelsByID = make(map[string]string)
	m.leafID = ""
	m.flushed = false

	if m.persist {
		fileTimestamp := strings.ReplaceAll(strings.ReplaceAll(timestamp, ":", "-"), ".", "-")
		m.sessionFile = filepath.Join(m.GetSessionDir(), fmt.Sprintf("%s_%s.jsonl", fileTimestamp, m.sessionID))
	}

	return m.sessionFile
}

// _buildIndex 构建索引
func (m *SessionManager) _buildIndex() {
	m.byID = make(map[string]SessionEntry)
	m.labelsByID = make(map[string]string)
	m.leafID = ""

	for _, entry := range m.fileEntries {
		if _, ok := entry.(*SessionHeader); ok {
			continue
		}

		sessionEntry, ok := entry.(SessionEntry)
		if !ok {
			continue
		}

		m.byID[sessionEntry.GetID()] = sessionEntry
		m.leafID = sessionEntry.GetID()

		if labelEntry, ok := entry.(*LabelEntry); ok {
			if labelEntry.Label != nil && *labelEntry.Label != "" {
				m.labelsByID[labelEntry.TargetID] = *labelEntry.Label
			} else {
				delete(m.labelsByID, labelEntry.TargetID)
			}
		}
	}
}

// _rewriteFile 重写文件
func (m *SessionManager) _rewriteFile() {
	if !m.persist || m.sessionFile == "" {
		return
	}

	var lines []string
	for _, entry := range m.fileEntries {
		data, err := json.Marshal(entry)
		if err != nil {
			continue
		}
		lines = append(lines, string(data))
	}

	content := strings.Join(lines, "\n") + "\n"
	err := os.WriteFile(m.sessionFile, []byte(content), 0644)
	if err != nil {
		// 处理错误
	}
}

// _persist 持久化条目
func (m *SessionManager) _persist(entry SessionEntry) {
	if !m.persist || m.sessionFile == "" {
		return
	}

	hasAssistant := false
	for _, e := range m.fileEntries {
		if msgEntry, ok := e.(*SessionMessageEntry); ok {
			if msgEntry.Message["role"] == "assistant" {
				hasAssistant = true
				break
			}
		}
	}

	if !hasAssistant {
		// 标记为未刷新，当助手消息到达时写入所有条目
		m.flushed = false
		return
	}

	if !m.flushed {
		// 写入所有条目
		file, err := os.OpenFile(m.sessionFile, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
		if err != nil {
			return
		}
		defer file.Close()

		for _, e := range m.fileEntries {
			data, err := json.Marshal(e)
			if err != nil {
				continue
			}
			_, err = file.Write([]byte(string(data) + "\n"))
			if err != nil {
				// 处理错误
			}
		}
		m.flushed = true
	} else {
		// 追加写入单个条目
		data, err := json.Marshal(entry)
		if err != nil {
			return
		}
		file, err := os.OpenFile(m.sessionFile, os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			return
		}
		defer file.Close()
		_, err = file.Write([]byte(string(data) + "\n"))
		if err != nil {
			// 处理错误
		}
	}
}

// _appendEntry 添加条目
func (m *SessionManager) _appendEntry(entry SessionEntry) {
	m.fileEntries = append(m.fileEntries, entry)
	m.byID[entry.GetID()] = entry
	m.leafID = entry.GetID()
	m._persist(entry)
}

// IsPersisted 检查是否持久化
func (m *SessionManager) IsPersisted() bool {
	return m.persist
}

// GetCwd 获取工作目录
func (m *SessionManager) GetCwd() string {
	return m.cwd
}

// GetSessionDir 获取会话目录
func (m *SessionManager) GetSessionDir() string {
	return m.sessionDir
}

// GetSessionID 获取会话ID
func (m *SessionManager) GetSessionID() string {
	return m.sessionID
}

// GetSessionFile 获取会话文件
func (m *SessionManager) GetSessionFile() string {
	return m.sessionFile
}

// AppendMessage 添加消息
func (m *SessionManager) AppendMessage(message ai.Message) string {
	// 将 Message 转换为 map
	msgMap := map[string]interface{}{
		"role": message.GetRole(),
	}
	
	// 尝试获取其他字段
	if msg, ok := message.(interface{ GetContent() []ai.ContentBlock }); ok {
		msgMap["content"] = msg.GetContent()
	}
	if msg, ok := message.(interface{ GetProvider() string }); ok {
		msgMap["provider"] = msg.GetProvider()
	}
	if msg, ok := message.(interface{ GetModel() string }); ok {
		msgMap["model"] = msg.GetModel()
	}
	if msg, ok := message.(interface{ GetTimestamp() int64 }); ok {
		msgMap["timestamp"] = msg.GetTimestamp()
	}
	
	entry := &SessionMessageEntry{
		SessionEntryBase: SessionEntryBase{
			Type:      "message",
			ID:        generateID(m.byID),
			ParentID:  m.leafID,
			Timestamp: time.Now().Format(time.RFC3339),
		},
		Message: msgMap,
	}

	m._appendEntry(entry)
	return entry.ID
}

// AppendThinkingLevelChange 添加思考级别变化
func (m *SessionManager) AppendThinkingLevelChange(thinkingLevel string) string {
	entry := &ThinkingLevelChangeEntry{
		SessionEntryBase: SessionEntryBase{
			Type:      "thinking_level_change",
			ID:        generateID(m.byID),
			ParentID:  m.leafID,
			Timestamp: time.Now().Format(time.RFC3339),
		},
		ThinkingLevel: thinkingLevel,
	}

	m._appendEntry(entry)
	return entry.ID
}

// AppendThinkingLevelChangeFromLevel 从 ThinkingLevel 添加思考级别变化
func (m *SessionManager) AppendThinkingLevelChangeFromLevel(thinkingLevel ThinkingLevel) string {
	return m.AppendThinkingLevelChange(string(thinkingLevel))
}

// AppendModelChange 添加模型变化
func (m *SessionManager) AppendModelChange(provider, modelID string) string {
	entry := &ModelChangeEntry{
		SessionEntryBase: SessionEntryBase{
			Type:      "model_change",
			ID:        generateID(m.byID),
			ParentID:  m.leafID,
			Timestamp: time.Now().Format(time.RFC3339),
		},
		Provider: provider,
		ModelID:  modelID,
	}

	m._appendEntry(entry)
	return entry.ID
}

// AppendCompaction 添加压缩摘要
func (m *SessionManager) AppendCompaction(summary, firstKeptEntryID string, tokensBefore int, details interface{}, fromHook bool) string {
	entry := &CompactionEntry{
		SessionEntryBase: SessionEntryBase{
			Type:      "compaction",
			ID:        generateID(m.byID),
			ParentID:  m.leafID,
			Timestamp: time.Now().Format(time.RFC3339),
		},
		Summary:          summary,
		FirstKeptEntryID: firstKeptEntryID,
		TokensBefore:     tokensBefore,
		Details:          details,
		FromHook:         fromHook,
	}

	m._appendEntry(entry)
	return entry.ID
}

// AppendCustomEntry 添加自定义条目
func (m *SessionManager) AppendCustomEntry(customType string, data interface{}) string {
	entry := &CustomEntry{
		SessionEntryBase: SessionEntryBase{
			Type:      "custom",
			ID:        generateID(m.byID),
			ParentID:  m.leafID,
			Timestamp: time.Now().Format(time.RFC3339),
		},
		CustomType: customType,
		Data:       data,
	}

	m._appendEntry(entry)
	return entry.ID
}

// AppendSessionInfo 添加会话信息
func (m *SessionManager) AppendSessionInfo(name string) string {
	entry := &SessionInfoEntry{
		SessionEntryBase: SessionEntryBase{
			Type:      "session_info",
			ID:        generateID(m.byID),
			ParentID:  m.leafID,
			Timestamp: time.Now().Format(time.RFC3339),
		},
		Name: strings.TrimSpace(name),
	}

	m._appendEntry(entry)
	return entry.ID
}

// GetSessionName 获取会话名称
func (m *SessionManager) GetSessionName() string {
	entries := m.GetEntries()
	for i := len(entries) - 1; i >= 0; i-- {
		if infoEntry, ok := entries[i].(*SessionInfoEntry); ok && infoEntry.Name != "" {
			return infoEntry.Name
		}
	}
	return ""
}

// AppendCustomMessageEntry 添加自定义消息条目
func (m *SessionManager) AppendCustomMessageEntry(customType string, content interface{}, display bool, details interface{}) string {
	entry := &CustomMessageEntry{
		SessionEntryBase: SessionEntryBase{
			Type:      "custom_message",
			ID:        generateID(m.byID),
			ParentID:  m.leafID,
			Timestamp: time.Now().Format(time.RFC3339),
		},
		CustomType: customType,
		Content:    content,
		Details:    details,
		Display:    display,
	}

	m._appendEntry(entry)
	return entry.ID
}

// AppendCustomMessageEntryWithAnyDisplay 添加自定义消息条目（display为interface{}类型）
func (m *SessionManager) AppendCustomMessageEntryWithAnyDisplay(customType string, content interface{}, display interface{}, details interface{}) string {
	displayBool := false
	if d, ok := display.(bool); ok {
		displayBool = d
	}
	return m.AppendCustomMessageEntry(customType, content, displayBool, details)
}

// GetLeafID 获取叶子ID
func (m *SessionManager) GetLeafID() string {
	return m.leafID
}

// GetLeafEntry 获取叶子条目
func (m *SessionManager) GetLeafEntry() SessionEntry {
	if m.leafID == "" {
		return nil
	}
	return m.byID[m.leafID]
}

// GetEntry 获取条目
func (m *SessionManager) GetEntry(id string) SessionEntry {
	return m.byID[id]
}

// GetChildren 获取子条目
func (m *SessionManager) GetChildren(parentID string) []SessionEntry {
	var children []SessionEntry
	for _, entry := range m.byID {
		if entry.GetParentID() == parentID {
			children = append(children, entry)
		}
	}
	return children
}

// GetLabel 获取标签
func (m *SessionManager) GetLabel(id string) string {
	return m.labelsByID[id]
}

// AppendLabelChange 添加标签变化
func (m *SessionManager) AppendLabelChange(targetID string, label string) string {
	if _, exists := m.byID[targetID]; !exists {
		panic(fmt.Sprintf("Entry %s not found", targetID))
	}

	var labelPtr *string
	if label != "" {
		labelPtr = &label
	}

	entry := &LabelEntry{
		SessionEntryBase: SessionEntryBase{
			Type:      "label",
			ID:        generateID(m.byID),
			ParentID:  m.leafID,
			Timestamp: time.Now().Format(time.RFC3339),
		},
		TargetID: targetID,
		Label:    labelPtr,
	}

	m._appendEntry(entry)

	if label != "" {
		m.labelsByID[targetID] = label
	} else {
		delete(m.labelsByID, targetID)
	}

	return entry.ID
}

// GetBranch 获取分支（从指定ID或当前叶子节点）
func (m *SessionManager) GetBranch(fromID ...string) []SessionEntry {
	startID := ""
	if len(fromID) > 0 {
		startID = fromID[0]
	}
	if startID == "" {
		startID = m.leafID
	}

	path := []SessionEntry{}
	current := m.byID[startID]
	for current != nil {
		path = append([]SessionEntry{current}, path...)
		if current.GetParentID() == "" {
			break
		}
		current = m.byID[current.GetParentID()]
	}

	return path
}

// GetBranchFrom 从指定ID获取分支（兼容旧代码）
func (m *SessionManager) GetBranchFrom(fromID string) []SessionEntry {
	return m.GetBranch(fromID)
}

// BuildSessionContext 构建会话上下文
func (m *SessionManager) BuildSessionContext() SessionContext {
	return buildSessionContext(m.GetEntries(), m.leafID, m.byID)
}

// buildSessionContext 构建会话上下文
func buildSessionContext(entries []SessionEntry, leafID string, byID map[string]SessionEntry) SessionContext {
	// 查找叶子节点
	var leaf SessionEntry
	if leafID == "" {
		if len(entries) > 0 {
			leaf = entries[len(entries)-1]
		} else {
			return SessionContext{
				Messages:      []ai.Message{},
				ThinkingLevel: "off",
				Model:         nil,
			}
		}
	} else {
		leaf = byID[leafID]
		if leaf == nil {
			if len(entries) > 0 {
				leaf = entries[len(entries)-1]
			} else {
				return SessionContext{
					Messages:      []ai.Message{},
					ThinkingLevel: "off",
					Model:         nil,
				}
			}
		}
	}

	// 从叶子到根收集路径
	path := []SessionEntry{}
	current := leaf
	for current != nil {
		path = append([]SessionEntry{current}, path...)
		if current.GetParentID() == "" {
			break
		}
		current = byID[current.GetParentID()]
	}

	// 提取设置和查找压缩
	thinkingLevel := "off"
	var model *ModelInfo
	var compaction *CompactionEntry

	for _, entry := range path {
		switch e := entry.(type) {
		case *ThinkingLevelChangeEntry:
			thinkingLevel = e.ThinkingLevel
		case *ModelChangeEntry:
			model = &ModelInfo{
				Provider: e.Provider,
				ModelID:  e.ModelID,
			}
		case *SessionMessageEntry:
			if role, ok := e.Message["role"].(string); ok && role == "assistant" {
				if provider, ok := e.Message["provider"].(string); ok {
					if modelID, ok := e.Message["model"].(string); ok {
						model = &ModelInfo{
							Provider: provider,
							ModelID:  modelID,
						}
					}
				}
			}
		case *CompactionEntry:
			compaction = e
		}
	}

	// 构建消息
	messages := []ai.Message{}

	appendMessage := func(entry SessionEntry) {
		switch e := entry.(type) {
		case *SessionMessageEntry:
			// 尝试将map转换为Message
			if msg, ok := convertMapToAIMessage(e.Message); ok {
				messages = append(messages, msg)
			}
		case *CustomMessageEntry:
			// 创建自定义消息
			customMsg := &CustomMessage{
				Role:      "custom",
				CustomType: e.CustomType,
				Content:    e.Content,
				Display:    e.Display,
				Details:    e.Details,
			}
			messages = append(messages, customMsg)
		case *BranchSummaryEntry:
			// 创建分支摘要消息
			branchMsg := &CustomMessage{
				Role:      "system",
				CustomType: "branch_summary",
				Content:    e.Summary,
				Display:    true,
			}
			messages = append(messages, branchMsg)
		}
	}

	if compaction != nil {
		// 首先添加摘要
		compactionMsg := &CustomMessage{
			Role:      "system",
			CustomType: "compaction_summary",
			Content:    compaction.Summary,
			Display:    true,
		}
		messages = append(messages, compactionMsg)

		// 查找压缩在路径中的索引
		compactionIdx := -1
		for i, entry := range path {
			if e, ok := entry.(*CompactionEntry); ok && e.ID == compaction.ID {
				compactionIdx = i
				break
			}
		}

		// 添加保留的消息（在压缩之前，从firstKeptEntryID开始）
		foundFirstKept := false
		for i := 0; i < compactionIdx; i++ {
			entry := path[i]
			if entry.GetID() == compaction.FirstKeptEntryID {
				foundFirstKept = true
			}
			if foundFirstKept {
				appendMessage(entry)
			}
		}

		// 添加压缩之后的消息
		for i := compactionIdx + 1; i < len(path); i++ {
			entry := path[i]
			appendMessage(entry)
		}
	} else {
		// 没有压缩 - 添加所有消息
		for _, entry := range path {
			appendMessage(entry)
		}
	}

	return SessionContext{
		Messages:      messages,
		ThinkingLevel: thinkingLevel,
		Model:         model,
	}
}

// convertMapToAIMessage 将map转换为Message
func convertMapToAIMessage(msgMap map[string]any) (ai.Message, bool) {
	role, ok := msgMap["role"].(string)
	if !ok {
		return nil, false
	}

	switch role {
	case "user":
		content, ok := msgMap["content"].([]ai.ContentBlock)
		if !ok {
			return nil, false
		}
		return &ai.UserMessage{
			Role:    role,
			Content: content,
		}, true
	case "assistant":
		content, ok := msgMap["content"].([]ai.ContentBlock)
		if !ok {
			return nil, false
		}
		return &ai.AssistantMessage{
			Role:    role,
			Content: content,
		}, true
	case "toolResult":
		content, ok := msgMap["content"].([]ai.ContentBlock)
		if !ok {
			return nil, false
		}
		return &ai.ToolResultMessage{
			Role:    role,
			Content: content,
		}, true
	default:
		content := msgMap["content"]
		return &CustomMessage{
			Role:    role,
			Content: content,
		}, true
	}
}

// GetHeader 获取会话头部
func (m *SessionManager) GetHeader() *SessionHeader {
	for _, entry := range m.fileEntries {
		if header, ok := entry.(*SessionHeader); ok {
			return header
		}
	}
	return nil
}

// GetEntries 获取所有会话条目
func (m *SessionManager) GetEntries() []SessionEntry {
	var entries []SessionEntry
	for _, entry := range m.fileEntries {
		if sessionEntry, ok := entry.(SessionEntry); ok {
			entries = append(entries, sessionEntry)
		}
	}
	return entries
}

// GetTree 获取会话树
func (m *SessionManager) GetTree() []SessionTreeNode {
	entries := m.GetEntries()
	nodeMap := make(map[string]*SessionTreeNode)
	var roots []SessionTreeNode

	// 创建节点
	for _, entry := range entries {
		label := m.labelsByID[entry.GetID()]
		var labelPtr *string
		if label != "" {
			labelPtr = &label
		}

		node := &SessionTreeNode{
			Entry:    entry,
			Children: []SessionTreeNode{},
			Label:    labelPtr,
		}
		nodeMap[entry.GetID()] = node
	}

	// 构建树
	for _, entry := range entries {
		node := nodeMap[entry.GetID()]
		if entry.GetParentID() == "" || entry.GetParentID() == entry.GetID() {
			roots = append(roots, *node)
		} else {
			parent := nodeMap[entry.GetParentID()]
			if parent != nil {
				parent.Children = append(parent.Children, *node)
			} else {
				// 孤儿节点 - 视为根节点
				roots = append(roots, *node)
			}
		}
	}

	// 按时间戳排序子节点
	sortChildren := func(nodes []SessionTreeNode) {
		stack := make([]*SessionTreeNode, len(nodes))
		for i, node := range nodes {
			stack[i] = &node
		}

		for len(stack) > 0 {
			node := stack[len(stack)-1]
			stack = stack[:len(stack)-1]

			sort.Slice(node.Children, func(i, j int) bool {
				t1, _ := time.Parse(time.RFC3339, node.Children[i].Entry.GetTimestamp())
				t2, _ := time.Parse(time.RFC3339, node.Children[j].Entry.GetTimestamp())
				return t1.Before(t2)
			})

			for i := range node.Children {
				stack = append(stack, &node.Children[i])
			}
		}
	}

	sortChildren(roots)

	return roots
}

// Branch 从指定条目开始新分支
func (m *SessionManager) Branch(branchFromID string) {
	if _, exists := m.byID[branchFromID]; !exists {
		panic(fmt.Sprintf("Entry %s not found", branchFromID))
	}
	m.leafID = branchFromID
}

// ResetLeaf 重置叶子指针
func (m *SessionManager) ResetLeaf() {
	m.leafID = ""
}

// BranchWithSummary 带摘要的分支
func (m *SessionManager) BranchWithSummary(branchFromID string, summary string, details interface{}, fromHook bool) string {
	if branchFromID != "" {
		if _, exists := m.byID[branchFromID]; !exists {
			panic(fmt.Sprintf("Entry %s not found", branchFromID))
		}
	}

	m.leafID = branchFromID

	entry := &BranchSummaryEntry{
		SessionEntryBase: SessionEntryBase{
			Type:      "branch_summary",
			ID:        generateID(m.byID),
			ParentID:  branchFromID,
			Timestamp: time.Now().Format(time.RFC3339),
		},
		FromID:  branchFromID,
		Summary: summary,
		Details: details,
		FromHook: fromHook,
	}

	m._appendEntry(entry)
	return entry.ID
}

// CreateBranchedSession 创建分支会话
func (m *SessionManager) CreateBranchedSession(leafID string) string {
	previousSessionFile := m.sessionFile
	path := m.GetBranchFrom(leafID)
	if len(path) == 0 {
		panic(fmt.Sprintf("Entry %s not found", leafID))
	}

	// 过滤掉标签条目
	var pathWithoutLabels []SessionEntry
	for _, entry := range path {
		if _, ok := entry.(*LabelEntry); !ok {
			pathWithoutLabels = append(pathWithoutLabels, entry)
		}
	}

	newSessionID := uuid.New().String()
	timestamp := time.Now().Format(time.RFC3339)
	fileTimestamp := strings.ReplaceAll(strings.ReplaceAll(timestamp, ":", "-"), ".", "-")
	newSessionFile := filepath.Join(m.GetSessionDir(), fmt.Sprintf("%s_%s.jsonl", fileTimestamp, newSessionID))

	header := &SessionHeader{
		Type:         "session",
		Version:      CurrentSessionVersion,
		ID:           newSessionID,
		Timestamp:    timestamp,
		Cwd:          m.cwd,
		ParentSession: "",
	}

	if m.persist && previousSessionFile != "" {
		header.ParentSession = previousSessionFile
	}

	// 收集路径中条目的标签
	pathEntryIDs := make(map[string]bool)
	for _, entry := range pathWithoutLabels {
		pathEntryIDs[entry.GetID()] = true
	}

	var labelsToWrite []struct {
		TargetID string
		Label    string
	}

	for targetID, label := range m.labelsByID {
		if pathEntryIDs[targetID] {
			labelsToWrite = append(labelsToWrite, struct {
				TargetID string
				Label    string
			}{TargetID: targetID, Label: label})
		}
	}

	if m.persist {
		// 写入新文件
		file, err := os.OpenFile(newSessionFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
		if err != nil {
			panic(fmt.Sprintf("Failed to create session file: %v", err))
		}
		defer file.Close()

		headerData, _ := json.Marshal(header)
		file.Write([]byte(string(headerData) + "\n"))

		for _, entry := range pathWithoutLabels {
			data, _ := json.Marshal(entry)
			file.Write([]byte(string(data) + "\n"))
		}

		// 写入标签条目
		lastEntryID := ""
		if len(pathWithoutLabels) > 0 {
			lastEntryID = pathWithoutLabels[len(pathWithoutLabels)-1].GetID()
		}

		parentID := lastEntryID
		var labelEntries []*LabelEntry

		for _, labelInfo := range labelsToWrite {
			labelEntry := &LabelEntry{
				SessionEntryBase: SessionEntryBase{
					Type:      "label",
					ID:        generateID(make(map[string]SessionEntry)),
					ParentID:  parentID,
					Timestamp: time.Now().Format(time.RFC3339),
				},
				TargetID: labelInfo.TargetID,
				Label:    &labelInfo.Label,
			}

			data, _ := json.Marshal(labelEntry)
			file.Write([]byte(string(data) + "\n"))

			pathEntryIDs[labelEntry.GetID()] = true
			labelEntries = append(labelEntries, labelEntry)
			parentID = labelEntry.GetID()
		}

		// 更新会话管理器状态
		m.fileEntries = []FileEntry{header}
		for _, entry := range pathWithoutLabels {
			m.fileEntries = append(m.fileEntries, entry)
		}
		for _, entry := range labelEntries {
			m.fileEntries = append(m.fileEntries, entry)
		}

		m.sessionID = newSessionID
		m.sessionFile = newSessionFile
		m.flushed = true
		m._buildIndex()

		return newSessionFile
	}

	// 内存模式
	var labelEntries []*LabelEntry
	parentID := ""
	if len(pathWithoutLabels) > 0 {
		parentID = pathWithoutLabels[len(pathWithoutLabels)-1].GetID()
	}

	for _, labelInfo := range labelsToWrite {
		labelEntry := &LabelEntry{
			SessionEntryBase: SessionEntryBase{
				Type:      "label",
				ID:        generateID(make(map[string]SessionEntry)),
				ParentID:  parentID,
				Timestamp: time.Now().Format(time.RFC3339),
			},
			TargetID: labelInfo.TargetID,
			Label:    &labelInfo.Label,
		}

		labelEntries = append(labelEntries, labelEntry)
		parentID = labelEntry.GetID()
	}

	m.fileEntries = []FileEntry{header}
	for _, entry := range pathWithoutLabels {
		m.fileEntries = append(m.fileEntries, entry)
	}
	for _, entry := range labelEntries {
		m.fileEntries = append(m.fileEntries, entry)
	}

	m.sessionID = newSessionID
	m._buildIndex()

	return ""
}

// Create 创建会话管理器
func Create(cwd, sessionDir string) *SessionManager {
	dir := sessionDir
	if dir == "" {
		dir = getDefaultSessionDir(cwd)
	}
	return newSessionManager(cwd, dir, "", true)
}

// Open 打开会话文件
func Open(path, sessionDir string) *SessionManager {
	entries := loadEntriesFromFile(path)
	var cwd string

	for _, entry := range entries {
		if header, ok := entry.(*SessionHeader); ok {
			cwd = header.Cwd
			break
		}
	}

	if cwd == "" {
		cwd, _ = os.Getwd()
	}

	dir := sessionDir
	if dir == "" {
		dir = filepath.Dir(path)
	}

	return newSessionManager(cwd, dir, path, true)
}

// ContinueRecent 继续最近的会话
func ContinueRecent(cwd, sessionDir string) *SessionManager {
	dir := sessionDir
	if dir == "" {
		dir = getDefaultSessionDir(cwd)
	}

	mostRecent := findMostRecentSession(dir)
	if mostRecent != "" {
		return newSessionManager(cwd, dir, mostRecent, true)
	}

	return newSessionManager(cwd, dir, "", true)
}

// InMemory 创建内存会话
func InMemorySessionManager(cwd string) *SessionManager {
	if cwd == "" {
		cwd, _ = os.Getwd()
	}
	return newSessionManager(cwd, "", "", false)
}

// ForkFrom 从源会话分叉
func ForkFrom(sourcePath, targetCwd, sessionDir string) *SessionManager {
	sourceEntries := loadEntriesFromFile(sourcePath)
	if len(sourceEntries) == 0 {
		panic(fmt.Sprintf("Cannot fork: source session file is empty or invalid: %s", sourcePath))
	}

	var sourceHeader *SessionHeader
	for _, entry := range sourceEntries {
		if header, ok := entry.(*SessionHeader); ok {
			sourceHeader = header
			break
		}
	}

	if sourceHeader == nil {
		panic(fmt.Sprintf("Cannot fork: source session has no header: %s", sourcePath))
	}

	dir := sessionDir
	if dir == "" {
		dir = getDefaultSessionDir(targetCwd)
	}

	if _, err := os.Stat(dir); os.IsNotExist(err) {
		os.MkdirAll(dir, 0755)
	}

	// 创建新会话文件
	newSessionID := uuid.New().String()
	timestamp := time.Now().Format(time.RFC3339)
	fileTimestamp := strings.ReplaceAll(strings.ReplaceAll(timestamp, ":", "-"), ".", "-")
	newSessionFile := filepath.Join(dir, fmt.Sprintf("%s_%s.jsonl", fileTimestamp, newSessionID))

	// 写入新头部
	newHeader := &SessionHeader{
		Type:         "session",
		Version:      CurrentSessionVersion,
		ID:           newSessionID,
		Timestamp:    timestamp,
		Cwd:          targetCwd,
		ParentSession: sourcePath,
	}

	file, err := os.OpenFile(newSessionFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		panic(fmt.Sprintf("Failed to create session file: %v", err))
	}
	defer file.Close()

	headerData, _ := json.Marshal(newHeader)
	file.Write([]byte(string(headerData) + "\n"))

	// 复制所有非头部条目
	for _, entry := range sourceEntries {
		if _, ok := entry.(*SessionHeader); !ok {
			data, _ := json.Marshal(entry)
			file.Write([]byte(string(data) + "\n"))
		}
	}

	return newSessionManager(targetCwd, dir, newSessionFile, true)
}

// List 列出会话
func List(cwd, sessionDir string, onProgress SessionListProgress) []SessionInfo {
	dir := sessionDir
	if dir == "" {
		dir = getDefaultSessionDir(cwd)
	}

	sessions := listSessionsFromDir(dir, onProgress)

	// 按修改时间排序
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].Modified.After(sessions[j].Modified)
	})

	return sessions
}

// listSessionsFromDir 从目录列出会话
func listSessionsFromDir(dir string, onProgress SessionListProgress) []SessionInfo {
	sessions := []SessionInfo{}

	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return sessions
	}

	files, err := os.ReadDir(dir)
	if err != nil {
		return sessions
	}

	var jsonlFiles []string
	for _, file := range files {
		if !file.IsDir() && strings.HasSuffix(file.Name(), ".jsonl") {
			jsonlFiles = append(jsonlFiles, filepath.Join(dir, file.Name()))
		}
	}

	total := len(jsonlFiles)
	loaded := 0

	for _, file := range jsonlFiles {
		info := buildSessionInfo(file)
		if info != nil {
			sessions = append(sessions, *info)
		}
		loaded++
		if onProgress != nil {
			onProgress(loaded, total)
		}
	}

	return sessions
}

// buildSessionInfo 构建会话信息
func buildSessionInfo(filePath string) *SessionInfo {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil
	}

	entries := parseSessionEntries(string(content))
	if len(entries) == 0 {
		return nil
	}

	header, ok := entries[0].(*SessionHeader)
	if !ok {
		return nil
	}

	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return nil
	}

	messageCount := 0
	firstMessage := ""
	var allMessages []string
	var name *string

	for _, entry := range entries {
		// 提取会话名称
		if infoEntry, ok := entry.(*SessionInfoEntry); ok && infoEntry.Name != "" {
			name = &infoEntry.Name
		}

		// 提取消息信息
		if msgEntry, ok := entry.(*SessionMessageEntry); ok {
			messageCount++

			if msgEntry.Message["role"] == "user" || msgEntry.Message["role"] == "assistant" {
				if content, ok := msgEntry.Message["content"].(string); ok {
					allMessages = append(allMessages, content)
					if firstMessage == "" && msgEntry.Message["role"] == "user" {
						firstMessage = content
					}
				}
			}
		}
	}

	cwd := header.Cwd
	parentSessionPath := header.ParentSession

	modified := getSessionModifiedDate(entries, header, fileInfo.ModTime())
	created, _ := time.Parse(time.RFC3339, header.Timestamp)

	if firstMessage == "" {
		firstMessage = "(no messages)"
	}

	return &SessionInfo{
		Path:            filePath,
		ID:              header.ID,
		Cwd:             cwd,
		Name:            name,
		ParentSessionPath: parentSessionPath,
		Created:         created,
		Modified:        modified,
		MessageCount:    messageCount,
		FirstMessage:    firstMessage,
		AllMessagesText: strings.Join(allMessages, " "),
	}
}

// getSessionModifiedDate 获取会话修改日期
func getSessionModifiedDate(entries []FileEntry, header *SessionHeader, statsMtime time.Time) time.Time {
	var lastActivityTime time.Time

	for _, entry := range entries {
		if msgEntry, ok := entry.(*SessionMessageEntry); ok {
			if msgEntry.Message["role"] == "user" || msgEntry.Message["role"] == "assistant" {
				// 尝试从消息中获取时间戳
				if timestamp, ok := msgEntry.Message["timestamp"].(float64); ok {
					t := time.Unix(int64(timestamp/1000), int64((timestamp-float64(int64(timestamp)))*1000000))
					if t.After(lastActivityTime) {
						lastActivityTime = t
					}
					continue
				}

				// 从条目时间戳获取
				if entryTimestamp, ok := entry.(SessionEntry); ok {
					t, err := time.Parse(time.RFC3339, entryTimestamp.GetTimestamp())
					if err == nil && t.After(lastActivityTime) {
						lastActivityTime = t
					}
				}
			}
		}
	}

	if !lastActivityTime.IsZero() {
		return lastActivityTime
	}

	// 从头部时间戳获取
	headerTime, err := time.Parse(time.RFC3339, header.Timestamp)
	if err == nil {
		return headerTime
	}

	return statsMtime
}
