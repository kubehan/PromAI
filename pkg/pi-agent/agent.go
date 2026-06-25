package piagent

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"PromAI/pkg/config"
	"PromAI/pkg/database"
	"PromAI/pkg/metrics"

	"github.com/jay-y/pi/pkg/ai"
	agent "github.com/jay-y/pi/pkg/ai-agent"
	"gorm.io/gorm"
)

type sessionData struct {
	agent     *agent.Agent
	modelName string
}

type AgentHandler struct {
	config    *config.Config
	collector *metrics.Collector
	db        *gorm.DB
	jwtSecret string

	mu       sync.Mutex
	sessions map[string]*sessionData
	tools    []agent.AgentTool
}

func NewAgentHandler(cfg *config.Config, collector *metrics.Collector, db *gorm.DB, jwtSecret string) *AgentHandler {
	ai.RegisterBuiltInApiProviders()

	h := &AgentHandler{
		config:    cfg,
		collector: collector,
		db:        db,
		jwtSecret: jwtSecret,
		sessions:  make(map[string]*sessionData),
	}
	h.tools = CreateAllTools(cfg, collector, &gormDBWrapper{db: db})
	h.loadAIConfigFromDB()
	return h
}

func (h *AgentHandler) getModel(name string) *config.AIModelConfig {
	for i := range h.config.AI.Models {
		if h.config.AI.Models[i].Name == name {
			return &h.config.AI.Models[i]
		}
	}
	return nil
}

func (h *AgentHandler) getDefaultModel() *config.AIModelConfig {
	if name := h.config.AI.DefaultModel; name != "" {
		if m := h.getModel(name); m != nil {
			return m
		}
	}
	if len(h.config.AI.Models) > 0 {
		return &h.config.AI.Models[0]
	}
	return nil
}

func (h *AgentHandler) loadAIConfigFromDB() {
	var settings []struct {
		Key   string
		Value string
	}
	h.db.Table("app_settings").Where("key LIKE ?", "ai_%").Find(&settings)

	var enabled *bool
	var defaultModel string
	var modelsJSON string

	for _, s := range settings {
		switch s.Key {
		case "ai_enabled":
			v := s.Value == "true"
			enabled = &v
		case "ai_default_model":
			defaultModel = s.Value
		case "ai_models":
			modelsJSON = s.Value
		case "ai_provider", "ai_model", "ai_base_url", "ai_api_key", "ai_thinking_level", "ai_max_tokens":
		}
	}

	if modelsJSON != "" {
		var models []config.AIModelConfig
		if err := json.Unmarshal([]byte(modelsJSON), &models); err == nil {
			for i := range models {
				if strings.HasPrefix(models[i].APIKey, "enc:") {
					decrypted, err := DecryptAPIKey(strings.TrimPrefix(models[i].APIKey, "enc:"), h.jwtSecret)
					if err == nil {
						models[i].APIKey = decrypted
					} else {
						log.Printf("[PiAgent] 解密模型 %s API Key 失败: %v", models[i].Name, err)
					}
				}
			}
			if len(models) > 0 {
				h.config.AI.Models = models
			}
		}
	} else {
		legacy := h.migrateLegacySettings(settings)
		if legacy != nil {
			h.config.AI.Models = legacy
		}
	}

	if defaultModel != "" {
		h.config.AI.DefaultModel = defaultModel
		if h.getModel(defaultModel) == nil && len(h.config.AI.Models) > 0 {
			h.config.AI.DefaultModel = h.config.AI.Models[0].Name
		}
	}

	if len(h.config.AI.Models) == 0 {
		h.config.AI.DefaultModel = ""
	}

	if enabled != nil {
		h.config.AI.Enabled = *enabled
	}
}

func (h *AgentHandler) migrateLegacySettings(settings []struct{ Key, Value string }) []config.AIModelConfig {
	m := config.AIModelConfig{Name: "default"}
	hasData := false
	for _, s := range settings {
		switch s.Key {
		case "ai_provider":
			m.Provider = s.Value
			hasData = true
		case "ai_model":
			m.Model = s.Value
			hasData = true
		case "ai_base_url":
			m.BaseURL = s.Value
			hasData = true
		case "ai_api_key":
			if s.Value != "" && s.Value != "********" {
				if strings.HasPrefix(s.Value, "enc:") {
					decrypted, err := DecryptAPIKey(strings.TrimPrefix(s.Value, "enc:"), h.jwtSecret)
					if err == nil {
						m.APIKey = decrypted
					}
				} else {
					m.APIKey = s.Value
				}
				hasData = true
			}
		case "ai_thinking_level":
			m.ThinkingLevel = s.Value
			hasData = true
		case "ai_max_tokens":
			fmt.Sscanf(s.Value, "%d", &m.MaxTokens)
			hasData = true
		}
	}
	if !hasData {
		return nil
	}
	if m.Provider == "" {
		m.Provider = h.config.AI.Models[0].Provider
	}
	if m.Model == "" {
		m.Model = h.config.AI.Models[0].Model
	}
	if m.BaseURL == "" {
		m.BaseURL = h.config.AI.Models[0].BaseURL
	}
	if m.ThinkingLevel == "" {
		m.ThinkingLevel = h.config.AI.Models[0].ThinkingLevel
	}
	if m.MaxTokens == 0 {
		m.MaxTokens = h.config.AI.Models[0].MaxTokens
	}
	return []config.AIModelConfig{m}
}

func (h *AgentHandler) RegisterRoutes(mux *http.ServeMux, auth func(http.HandlerFunc) http.HandlerFunc) {
	mux.HandleFunc("/api/promai/ai/chat", auth(h.handleChat))
	mux.HandleFunc("/api/promai/ai/sessions", auth(h.handleSessions))
	mux.HandleFunc("/api/promai/ai/sessions/", auth(h.handleSessionByID))
	mux.HandleFunc("/api/promai/ai/test-model", auth(h.handleTestModel))
	log.Printf("[PiAgent] AI Agent 路由已注册 (%d 个模型配置)", len(h.config.AI.Models))
}

func (h *AgentHandler) getOrCreateSession(sessionID, modelName string) (*agent.Agent, string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if sessionID != "" {
		if sd, ok := h.sessions[sessionID]; ok {
			if modelName != "" && sd.modelName != modelName {
				log.Printf("[PiAgent] 模型变更 %s -> %s，重建会话 %s", sd.modelName, modelName, sessionID)
				delete(h.sessions, sessionID)
			} else {
				return sd.agent, sessionID
			}
		}
		if restored := h.restoreSessionLocked(sessionID, modelName); restored != nil {
			return restored, sessionID
		}
	}

	ag, resolvedModelName := h.newAgent(modelName)
	if ag == nil {
		return nil, ""
	}

	newID := fmt.Sprintf("sess_%d", time.Now().UnixNano())
	h.sessions[newID] = &sessionData{agent: ag, modelName: resolvedModelName}

	log.Printf("[PiAgent] 创建会话 %s (模型: %s)", newID, resolvedModelName)
	return ag, newID
}

func (h *AgentHandler) newAgent(modelName string) (*agent.Agent, string) {
	mc := h.getDefaultModel()
	if modelName != "" {
		if m := h.getModel(modelName); m != nil {
			mc = m
		}
	}
	if mc == nil {
		return nil, ""
	}

	llmAPIKey := mc.APIKey
	if llmAPIKey == "" {
		llmAPIKey = os.Getenv("LLM_API_KEY")
	}

	if len(llmAPIKey) > 0 {
		log.Printf("[DEBUG] API key len=%d prefix=%s suffix=%s", len(llmAPIKey), llmAPIKey[:4], llmAPIKey[len(llmAPIKey)-4:])
	} else {
		log.Printf("[DEBUG] API key is EMPTY")
	}

	systemPrompt := BuildSystemPrompt(h.config, h.db)

	level := ai.ThinkingLevelOff
	switch mc.ThinkingLevel {
	case "minimal":
		level = ai.ThinkingLevelMinimal
	case "low":
		level = ai.ThinkingLevelLow
	case "medium":
		level = ai.ThinkingLevelMedium
	case "high":
		level = ai.ThinkingLevelHigh
	case "xhigh":
		level = ai.ThinkingLevelXHigh
	}

	ag := agent.NewAgent(agent.AgentOptions{
		GetApiKey: func(provider string) (string, error) {
			return llmAPIKey, nil
		},
	})
	ag.SetModel(&ai.BaseModel{
		ID:            mc.Model,
		Name:          fmt.Sprintf("%s/%s", mc.Provider, mc.Model),
		API:           ai.ApiOpenAICompletions,
		Provider:      ai.ProviderOpenAI,
		BaseURL:       mc.BaseURL,
		APIKey:        llmAPIKey,
		Reasoning:     mc.ThinkingLevel != "off",
		ContextWindow: 128000,
		MaxTokens:     mc.MaxTokens,
		ProxyURL:      mc.ProxyURL,
	})
	ag.SetSystemPrompt(systemPrompt)
	ag.SetThinkingLevel(level)
	ag.SetTools(h.tools)
	return ag, mc.Name
}

func (h *AgentHandler) restoreSessionLocked(sessionID, modelName string) *agent.Agent {
	var session database.AiSession
	if err := h.db.Where("id = ?", sessionID).First(&session).Error; err != nil {
		return nil
	}

	resumeModelName := strings.TrimSpace(modelName)
	if resumeModelName == "" {
		resumeModelName = strings.TrimSpace(session.ModelName)
	}

	ag, resolvedModelName := h.newAgent(resumeModelName)
	if ag == nil {
		return nil
	}

	var messages []database.AiMessage
	h.db.Where("session_id = ?", sessionID).Order("created_at asc, id asc").Find(&messages)
	for _, m := range messages {
		switch m.Role {
		case "user":
			ag.AppendMessage(ai.NewUserMessage(m.Content))
		case "assistant":
			msg := ai.NewAssistantMessage(ai.ApiOpenAICompletions, ai.ProviderOpenAI, resolvedModelName)
			msg.Content = append(msg.Content, ai.NewTextContentBlock(m.Content))
			ag.AppendMessage(msg)
		}
	}

	h.sessions[sessionID] = &sessionData{agent: ag, modelName: resolvedModelName}
	log.Printf("[PiAgent] 恢复历史会话 %s (模型: %s, 消息数: %d)", sessionID, resolvedModelName, len(messages))
	return ag
}

func (h *AgentHandler) buildSessionTitle(sessionID string) string {
	var firstUserMessage database.AiMessage
	if err := h.db.Where("session_id = ? AND role = ?", sessionID, "user").Order("created_at asc").First(&firstUserMessage).Error; err != nil {
		return ""
	}
	return compactSessionTitle(firstUserMessage.Content, 28)
}

func compactSessionTitle(text string, limit int) string {
	text = strings.TrimSpace(strings.ReplaceAll(text, "\n", " "))
	text = strings.Join(strings.Fields(text), " ")
	if text == "" {
		return ""
	}
	runes := []rune(text)
	if limit <= 0 || len(runes) <= limit {
		return text
	}
	return string(runes[:limit]) + "..."
}

func (h *AgentHandler) handleChat(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		writeHTTPError(w, 405, "仅支持 POST 方法")
		return
	}

	var req ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeHTTPError(w, 400, "请求体格式错误")
		return
	}
	if req.Message == "" {
		writeHTTPError(w, 400, "消息不能为空")
		return
	}

	log.Printf("[PiAgent] Chat 请求: session=%s model=%s msg_len=%d", req.SessionID, req.ModelName, len(req.Message))

	ag, sessionID := h.getOrCreateSession(req.SessionID, req.ModelName)
	if ag == nil {
		log.Printf("[PiAgent] 未配置 AI 模型，拒绝 Chat")
		writeHTTPError(w, 400, "未配置 AI 模型")
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeHTTPError(w, 500, "不支持 SSE 推送")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	writeSSE(w, map[string]string{"type": "session_id", "session_id": sessionID})
	flusher.Flush()

	// Ensure session record exists in DB
	h.db.Where("id = ?", sessionID).First(&database.AiSession{})
	modelLabel := req.ModelName
	if modelLabel == "" {
		if sd, ok := h.sessions[sessionID]; ok {
			modelLabel = sd.modelName
		}
	}
	if modelLabel == "" && h.getDefaultModel() != nil {
		modelLabel = h.getDefaultModel().Name
	}
	h.db.Create(&database.AiSession{
		ID:        sessionID,
		ModelName: modelLabel,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	})

	// Save user message
	h.db.Create(&database.AiMessage{
		SessionID: sessionID,
		Role:      "user",
		Content:   req.Message,
		CreatedAt: time.Now(),
	})

	var assistantContent strings.Builder

	unsub := ag.Subscribe(func(event agent.AgentEvent) {
		switch e := event.(type) {
		case *agent.AgentEventMessageUpdate:
			if delta, ok := e.AssistantMessageEvent.(*ai.AssistantMessageEventTextDelta); ok {
				cleaned := stripThinkTags(delta.Delta)
				assistantContent.WriteString(cleaned)
				writeSSE(w, map[string]string{"type": "text", "content": cleaned})
				flusher.Flush()
			}
		case *agent.AgentEventTurnEnd:
			if msg, ok := e.Message.(*ai.AssistantMessage); ok {
				if msg.ErrorMessage != "" {
					writeSSE(w, map[string]string{"type": "error", "content": msg.ErrorMessage})
					flusher.Flush()
					break
				}
				// Fallback: some models don't stream text deltas, emit full text from turn end
				for _, block := range msg.Content {
					if tb, ok := block.(*ai.TextContentBlock); ok && tb.Text != "" {
						cleaned := stripThinkTags(tb.Text)
						assistantContent.WriteString(cleaned)
						writeSSE(w, map[string]string{"type": "text", "content": cleaned})
						flusher.Flush()
					}
				}
			}
		case *agent.AgentEventToolExecutionStart:
			writeSSE(w, map[string]string{"type": "tool_start", "tool_name": e.ToolName})
			flusher.Flush()
		case *agent.AgentEventToolExecutionEnd:
			writeSSE(w, map[string]string{"type": "tool_end", "tool_name": e.ToolName})
			flusher.Flush()
		case *agent.AgentEventEnd:
			writeSSE(w, map[string]string{"type": "done"})
			flusher.Flush()
			// Save assistant message
			if content := strings.TrimSpace(assistantContent.String()); content != "" {
				h.db.Create(&database.AiMessage{
					SessionID: sessionID,
					Role:      "assistant",
					Content:   content,
					CreatedAt: time.Now(),
				})
				h.db.Model(&database.AiSession{}).Where("id = ?", sessionID).Update("updated_at", time.Now())
			}
		}
	})
	defer unsub()

	if err := ag.Prompt(r.Context(), req.Message); err != nil {
		log.Printf("[PiAgent] Prompt 错误: %v", err)
		writeSSE(w, map[string]string{"type": "error", "content": err.Error()})
		flusher.Flush()
	}
}

func (h *AgentHandler) handleSessions(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		writeHTTPError(w, 405, "仅支持 GET 方法")
		return
	}

	var sessions []database.AiSession
	h.db.Order("updated_at desc").Find(&sessions)

	list := make([]SessionInfo, 0, len(sessions))
	for _, s := range sessions {
		var count int64
		h.db.Model(&database.AiMessage{}).Where("session_id = ?", s.ID).Count(&count)
		list = append(list, SessionInfo{
			ID:        s.ID,
			CreatedAt: s.CreatedAt,
			UpdatedAt: s.UpdatedAt,
			MsgCount:  count,
			ModelName: s.ModelName,
			Title:     h.buildSessionTitle(s.ID),
		})
	}

	log.Printf("[PiAgent] 列出会话: %d 个", len(list))
	writeHTTPJSON(w, list)
}

func (h *AgentHandler) handleSessionByID(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.TrimRight(r.URL.Path, "/"), "/")
	if len(parts) < 1 {
		writeHTTPError(w, 400, "缺少会话 ID")
		return
	}
	sessionID := parts[len(parts)-1]

	switch r.Method {
	case "GET":
		var session database.AiSession
		if err := h.db.Where("id = ?", sessionID).First(&session).Error; err != nil {
			writeHTTPError(w, 404, "会话不存在")
			return
		}
		var messages []database.AiMessage
		h.db.Where("session_id = ?", sessionID).Order("created_at asc").Find(&messages)

		detail := SessionDetail{
			SessionInfo: SessionInfo{
				ID:        session.ID,
				CreatedAt: session.CreatedAt,
				UpdatedAt: session.UpdatedAt,
				ModelName: session.ModelName,
				Title:     h.buildSessionTitle(session.ID),
			},
		}
		detail.MsgCount = int64(len(messages))
		for _, m := range messages {
			detail.Messages = append(detail.Messages, SessionMessage{
				ID:        m.ID,
				Role:      m.Role,
				Content:   m.Content,
				CreatedAt: m.CreatedAt,
			})
		}
		writeHTTPJSON(w, detail)

	case "DELETE":
		h.mu.Lock()
		delete(h.sessions, sessionID)
		h.mu.Unlock()

		result := h.db.Where("id = ?", sessionID).Delete(&database.AiSession{})
		if result.RowsAffected == 0 {
			writeHTTPError(w, 404, "会话不存在")
			return
		}
		log.Printf("[PiAgent] 删除会话 %s", sessionID)
		writeHTTPJSON(w, map[string]bool{"success": true})

	default:
		writeHTTPError(w, 405, "仅支持 GET/DELETE 方法")
	}
}

func (h *AgentHandler) handleTestModel(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		writeHTTPError(w, 405, "仅支持 POST 方法")
		return
	}

	var req TestModelRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeHTTPError(w, 400, "请求体格式错误")
		return
	}
	if req.Model == "" || req.BaseURL == "" {
		writeHTTPJSON(w, map[string]any{"success": false, "error": "模型名称和接口地址不能为空"})
		return
	}

	log.Printf("[PiAgent] 测试模型: name=%s provider=%s model=%s base_url=%s", req.Name, req.Provider, req.Model, req.BaseURL)

	apiKey := req.APIKey
	if apiKey == "********" {
		if mc := h.getModel(req.Name); mc != nil {
			apiKey = mc.APIKey
		}
	}

	llmAPIKey := apiKey
	if llmAPIKey == "" {
		llmAPIKey = os.Getenv("LLM_API_KEY")
	}

	// 用传入的配置创建一个临时 Agent 触发真实会话
	ag := agent.NewAgent(agent.AgentOptions{
		GetApiKey: func(provider string) (string, error) {
			return llmAPIKey, nil
		},
	})
	ag.SetModel(&ai.BaseModel{
		ID:            req.Model,
		Name:          fmt.Sprintf("%s/%s", req.Provider, req.Model),
		API:           ai.ApiOpenAICompletions,
		Provider:      ai.ProviderOpenAI,
		BaseURL:       req.BaseURL,
		APIKey:        llmAPIKey,
		Reasoning:     req.ThinkingLevel != "off",
		ContextWindow: 128000,
		MaxTokens:     req.MaxTokens,
		ProxyURL:      req.ProxyURL,
	})
	ag.SetSystemPrompt("You are a helpful assistant.")
	level := ai.ThinkingLevelOff
	switch req.ThinkingLevel {
	case "minimal":
		level = ai.ThinkingLevelMinimal
	case "low":
		level = ai.ThinkingLevelLow
	case "medium":
		level = ai.ThinkingLevelMedium
	case "high":
		level = ai.ThinkingLevelHigh
	case "xhigh":
		level = ai.ThinkingLevelXHigh
	}
	ag.SetThinkingLevel(level)

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	var responseText string
	var testError string
	done := make(chan struct{})

	unsub := ag.Subscribe(func(event agent.AgentEvent) {
		switch e := event.(type) {
		case *agent.AgentEventMessageUpdate:
			if delta, ok := e.AssistantMessageEvent.(*ai.AssistantMessageEventTextDelta); ok {
				responseText += delta.Delta
			}
		case *agent.AgentEventTurnEnd:
			if msg, ok := e.Message.(*ai.AssistantMessage); ok {
				if msg.ErrorMessage != "" {
					testError = msg.ErrorMessage
				}
				for _, block := range msg.Content {
					if tb, ok := block.(*ai.TextContentBlock); ok {
						responseText += stripThinkTags(tb.Text)
					}
				}
			}
		case *agent.AgentEventEnd:
			close(done)
		}
	})
	defer unsub()

	if err := ag.Prompt(ctx, "Hello, please respond with just 'ok'."); err != nil {
		log.Printf("[PiAgent] 测试模型 Prompt 错误: %v", err)
		writeHTTPJSON(w, map[string]any{"success": false, "error": fmt.Sprintf("启动会话失败: %v", err)})
		return
	}

	select {
	case <-done:
	case <-ctx.Done():
		log.Printf("[PiAgent] 测试模型超时: %s", req.Name)
		writeHTTPJSON(w, map[string]any{"success": false, "error": "测试超时（30秒）"})
		return
	}

	if testError != "" {
		log.Printf("[PiAgent] 测试模型失败: %s -> %s", req.Name, testError)
		writeHTTPJSON(w, map[string]any{"success": false, "error": testError})
		return
	}

	if responseText == "" {
		log.Printf("[PiAgent] 测试模型无返回: %s", req.Name)
		writeHTTPJSON(w, map[string]any{"success": false, "error": "模型无返回"})
		return
	}

	log.Printf("[PiAgent] 测试模型成功: %s (返回 %d 字符)", req.Name, len(responseText))
	writeHTTPJSON(w, map[string]any{"success": true, "message": responseText})
}

func writeSSE(w http.ResponseWriter, data any) {
	jsonData, _ := json.Marshal(data)
	fmt.Fprintf(w, "data: %s\n\n", jsonData)
}

func writeHTTPJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func writeHTTPError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

func stripThinkTags(s string) string {
	var result strings.Builder
	for {
		start := strings.Index(s, "<think>")
		if start == -1 {
			result.WriteString(s)
			break
		}
		result.WriteString(s[:start])
		end := strings.Index(s[start:], "</think>")
		if end == -1 {
			break
		}
		s = s[start+end+len("</think>"):]
	}
	return strings.TrimSpace(result.String())
}
