package proxy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"PromAI/pkg/aiagent/manager"
	"PromAI/pkg/config"
	"PromAI/pkg/database"
)

type Proxy struct {
	cfg *config.Config
	mgr *manager.Manager
}

func New(cfg *config.Config, mgr *manager.Manager) *Proxy {
	return &Proxy{cfg: cfg, mgr: mgr}
}

func (p *Proxy) buildTarget() (*url.URL, error) {
	base := p.mgr.Status()
	u, _ := base["baseUrl"].(string)
	if u == "" {
		ac := p.cfg.AI.Agent
		addr := ac.ListenAddr
		if addr == "" {
			addr = "127.0.0.1"
		}
		port := ac.ListenPort
		if port == 0 {
			port = 4096
		}
		u = fmt.Sprintf("http://%s:%d", addr, port)
	}
	return url.Parse(u)
}

func (p *Proxy) setAgentAuth(req *http.Request) {
	ac := p.cfg.AI.Agent
	if ac.Username != "" && ac.Password != "" {
		req.SetBasicAuth(ac.Username, ac.Password)
		return
	}
	req.SetBasicAuth("opencode", "")
}

func (p *Proxy) proxyReq(w http.ResponseWriter, r *http.Request) {
	target, err := p.buildTarget()
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	proxy := httputil.NewSingleHostReverseProxy(target)
	orig := proxy.Director
	proxy.Director = func(req *http.Request) {
		orig(req)
		p.setAgentAuth(req)
	}
	proxy.ServeHTTP(w, r)
}

func (p *Proxy) Health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	h := p.mgr.Health()
	json.NewEncoder(w).Encode(map[string]interface{}{"healthy": h, "status": p.mgr.Status()})
}

func (p *Proxy) ProxyAny(w http.ResponseWriter, r *http.Request) {
	if !p.mgr.IsEnabled() {
		http.Error(w, "ai agent not enabled", http.StatusServiceUnavailable)
		return
	}
	if !p.mgr.Health() {
		http.Error(w, "ai agent not ready", http.StatusServiceUnavailable)
		return
	}
	p.proxyReq(w, r)
}

func (p *Proxy) CreateSession(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	target, err := p.buildTarget()
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	client := &http.Client{Timeout: 5 * time.Second}
	body := bytes.NewBufferString("{}")
	req, err := http.NewRequest(http.MethodPost, target.String()+"/session", body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	p.setAgentAuth(req)
	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	w.Header().Set("Content-Type", resp.Header.Get("Content-Type"))
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

func (p *Proxy) PostMessage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) < 4 {
		http.Error(w, "bad path", http.StatusBadRequest)
		return
	}
	sid := parts[3]
	var msg map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&msg); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	target, err := p.buildTarget()
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	body, _ := json.Marshal(msg)
	req, err := http.NewRequest(http.MethodPost, target.String()+"/session/"+sid+"/message", bytes.NewReader(body))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	p.setAgentAuth(req)
	client := &http.Client{Timeout: 300 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	w.Header().Set("Content-Type", resp.Header.Get("Content-Type"))
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

// Models 转发 opencode /config/providers 并拍平成可用的模型列表。
func (p *Proxy) Models(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if !p.mgr.IsEnabled() || !p.mgr.Health() {
		json.NewEncoder(w).Encode(map[string]interface{}{"items": []interface{}{}, "error": "ai agent not ready"})
		return
	}
	target, err := p.buildTarget()
	if err != nil {
		json.NewEncoder(w).Encode(map[string]interface{}{"items": []interface{}{}, "error": err.Error()})
		return
	}
	req, _ := http.NewRequest(http.MethodGet, target.String()+"/config/providers", nil)
	p.setAgentAuth(req)
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		json.NewEncoder(w).Encode(map[string]interface{}{"items": []interface{}{}, "error": err.Error()})
		return
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	var parsed struct {
		Providers []struct {
			ID     string `json:"id"`
			Name   string `json:"name"`
			Models []struct {
				ID       string `json:"id"`
				Name     string `json:"name"`
				Label    string `json:"label"`
				Provider string `json:"provider"`
			} `json:"models"`
		} `json:"providers"`
	}
	var items []map[string]interface{}
	if err := json.Unmarshal(b, &parsed); err == nil {
		seen := map[string]bool{}
		for _, pr := range parsed.Providers {
			for _, m := range pr.Models {
				id := m.ID
				if id == "" {
					id = m.Provider + "/" + m.Name
				}
				if seen[id] {
					continue
				}
				seen[id] = true
				label := m.Label
				if label == "" {
					label = m.Name
				}
				items = append(items, map[string]interface{}{
					"id":       id,
					"label":    label,
					"provider": pr.Name,
				})
			}
		}
	}
	json.NewEncoder(w).Encode(map[string]interface{}{"items": items})
}

func (p *Proxy) Events(w http.ResponseWriter, r *http.Request) {
	if !p.mgr.IsEnabled() || !p.mgr.Health() {
		http.Error(w, "ai agent not ready", http.StatusServiceUnavailable)
		return
	}
	target, err := p.buildTarget()
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	req, _ := http.NewRequest(http.MethodGet, target.String()+r.URL.Path, nil)
	p.setAgentAuth(req)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	io.CopyBuffer(w, resp.Body, make([]byte, 4096))
}

// resolveAIModel 选择系统配置的模型(与 AI 助手/AI 总结一致)。
func resolveAIModel(cfg *config.Config) *config.AIModelConfig {
	if cfg == nil {
		return nil
	}
	ai := &cfg.AI
	if name := ai.DefaultModel; name != "" {
		for i := range ai.Models {
			if ai.Models[i].Name == name {
				return &ai.Models[i]
			}
		}
	}
	if len(ai.Models) > 0 {
		return &ai.Models[0]
	}
	return nil
}

// ResolveAIModel 导出给外部包使用（统一 AI 配置层）
func ResolveAIModel(cfg *config.Config) *config.AIModelConfig { return resolveAIModel(cfg) }

// CallOpenAICompat 导出给外部包使用
func CallOpenAICompat(mc *config.AIModelConfig, system, user, modelOverride string) (string, error) {
	return callOpenAICompat(mc, system, user, modelOverride)
}

func callOpenAICompat(mc *config.AIModelConfig, system, user, modelOverride string) (string, error) {
	if mc == nil {
		return "", fmt.Errorf("未配置 AI 模型")
	}
	base := strings.TrimRight(mc.BaseURL, "/")
	if base == "" {
		return "", fmt.Errorf("模型 BaseURL 未配置")
	}
	model := mc.Model
	// 允许前端切到其他 provider@model
	if modelOverride != "" && strings.HasPrefix(modelOverride, mc.Provider) {
		if idx := strings.LastIndex(modelOverride, "@"); idx >= 0 {
			model = modelOverride[idx+1:]
		}
	}
	payload := map[string]interface{}{
		"model": model,
		"messages": []map[string]string{
			{"role": "system", "content": system},
			{"role": "user", "content": user},
		},
		"temperature": 0.3,
	}
	if mc.MaxTokens > 0 {
		payload["max_tokens"] = mc.MaxTokens
	}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequest(http.MethodPost, base+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if mc.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+mc.APIKey)
	}
	client := &http.Client{Timeout: 300 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("模型接口返回 %d: %s", resp.StatusCode, string(b))
	}
	var out struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(b, &out); err != nil {
		return "", err
	}
	if out.Error != nil {
		return "", fmt.Errorf("%s", out.Error.Message)
	}
	if len(out.Choices) > 0 {
		return strings.TrimSpace(out.Choices[0].Message.Content), nil
	}
	return "", fmt.Errorf("模型接口无返回值")
}

// SysChat 使用系统配置的模型(非流式)进行对话，注入目标主机上下文，
// 返回文本与可提取的命令。用于"AI 助手"模式，不依赖 opencode。
func (p *Proxy) SysChat(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Message string `json:"message"`
		Model   string `json:"model,omitempty"`
		HostID  uint   `json:"hostId,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "请求体格式错误", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(req.Message) == "" {
		http.Error(w, "message required", http.StatusBadRequest)
		return
	}
	mc := resolveAIModel(p.cfg)
	if mc == nil {
		http.Error(w, "未配置 AI 模型", http.StatusBadRequest)
		return
	}
	userMsg := req.Message
	if req.HostID > 0 && database.DB != nil {
		var host database.TerminalHost
		if database.DB.First(&host, req.HostID).Error == nil {
			ctx := fmt.Sprintf(
				"[PromAI受管主机上下文]\nhost_key=%s\nhost=%s\nport=%d\nusername=%s\nenv=%s\n\n【强制约束】\n- 目标主机是唯一执行环境：%s@%s:%d\n- 你需要将所有需在目标主机上执行的命令输出在 ```bash``` 代码块中，一行一个\n- 不要给出 SSH 命令，直接给出要在目标主机上执行的原始命令\n\n用户请求：%s",
				host.HostKey, host.Address, host.Port, host.Username, host.Env,
				host.Username, host.Address, host.Port, userMsg,
			)
			userMsg = ctx
		}
	}
	system := "你是运维智能体，负责排查与操作目标主机。命令必须放在 ```bash ``` 代码块中，便于前端一键下发到真实终端执行。"
	content, err := callOpenAICompat(mc, system, userMsg, req.Model)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	json.NewEncoder(w).Encode(map[string]interface{}{
		"content": content,
		"model":   mc.Name,
		"engine":  "system",
	})
}
