package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"PromAI/pkg/database"
	"PromAI/pkg/terminal"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"gorm.io/gorm"
)

// HostAPI 提供主机管理（CMDB）、SSH 连通性测试、自定义命令执行和 Web 终端。
type HostAPI struct {
	db    *gorm.DB
	api   *AdminAPI
	vault *terminal.Vault
}

type hostView struct {
	database.TerminalHost
	HasPassword   bool                           `json:"has_password"`
	HasPrivateKey bool                           `json:"has_private_key"`
	Commands      []database.TerminalHostCommand `json:"commands,omitempty"`
}

type hostPayload struct {
	Name               string `json:"name"`
	HostKey            string `json:"host_key"`
	Address            string `json:"address"`
	Port               int    `json:"port"`
	Username           string `json:"username"`
	AuthType           string `json:"auth_type"`
	Password           string `json:"password"`
	PrivateKey         string `json:"private_key"`
	Passphrase         string `json:"passphrase"`
	HostKeyFingerprint string `json:"host_key_fingerprint"`
	Env                string `json:"env"`
	Group              string `json:"group"`
	TagsJSON           string `json:"tags_json"`
	Description        string `json:"description"`
	Enabled            *bool  `json:"enabled"`
	ClearPassword      bool   `json:"clear_password"`
	ClearPrivateKey    bool   `json:"clear_private_key"`
}

type hostCommandPayload struct {
	Name        string `json:"name"`
	Command     string `json:"command"`
	Description string `json:"description"`
	AutoRun     bool   `json:"auto_run"`
	SortOrder   int    `json:"sort_order"`
}

func registerHostAPI(mux *http.ServeMux, api *AdminAPI) {
	vault, err := terminal.VaultFromEnv(api.jwtSecret)
	if err != nil {
		log.Printf("[Host] 凭据加密初始化失败: %v", err)
		return
	}
	h := &HostAPI{db: database.DB, api: api, vault: vault}
	h.ensureAllDefaultHostCommands()
	mux.HandleFunc("/api/promai/hosts", api.logRequest(h.withAuth(h.handleHosts)))
	mux.HandleFunc("/api/promai/hosts/", api.logRequest(h.withAuth(h.handleHostByID)))
	log.Printf("[Host] 主机管理与 Web 终端接口已注册")
}

// withAuth 普通请求沿用 Bearer JWT；WebSocket 浏览器无法设置自定义 Header，
// 因此仅对终端端点允许通过 query token 鉴权。
func (h *HostAPI) withAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if isTerminalWebSocket(r) {
			token := r.URL.Query().Get("token")
			if token == "" {
				writeError(w, 401, "未登录")
				return
			}
			claims, err := validateToken(token, h.api.jwtSecret)
			if err != nil {
				writeError(w, 401, "认证令牌无效或已过期")
				return
			}
			ctx := context.WithValue(r.Context(), "username", claims.Username)
			next(w, r.WithContext(ctx))
			return
		}
		h.api.authMiddleware(next)(w, r)
	}
}

func isTerminalWebSocket(r *http.Request) bool {
	return r.Method == http.MethodGet && strings.HasSuffix(strings.TrimRight(r.URL.Path, "/"), "/terminal") &&
		strings.EqualFold(r.Header.Get("Upgrade"), "websocket")
}

func currentUsername(r *http.Request) string {
	if v, ok := r.Context().Value("username").(string); ok {
		return v
	}
	return ""
}

func (h *HostAPI) handleHosts(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.listHosts(w, r)
	case http.MethodPost:
		h.createHost(w, r)
	default:
		writeError(w, 405, "不支持的请求方法")
	}
}

func (h *HostAPI) listHosts(w http.ResponseWriter, r *http.Request) {
	keyword := strings.TrimSpace(r.URL.Query().Get("keyword"))
	env := strings.TrimSpace(r.URL.Query().Get("env"))
	group := strings.TrimSpace(r.URL.Query().Get("group"))

	q := h.db.Model(&database.TerminalHost{})
	if keyword != "" {
		like := "%" + keyword + "%"
		q = q.Where("name LIKE ? OR host_key LIKE ? OR address LIKE ? OR tags_json LIKE ?", like, like, like, like)
	}
	if env != "" {
		q = q.Where("env = ?", env)
	}
	if group != "" {
		q = q.Where(`"group" = ?`, group)
	}
	var hosts []database.TerminalHost
	if err := q.Order("name asc, id asc").Find(&hosts).Error; err != nil {
		writeError(w, 500, err.Error())
		return
	}

	commands := h.commandsForHosts(hosts)
	items := make([]hostView, 0, len(hosts))
	for i := range hosts {
		items = append(items, toHostView(hosts[i], commands[hosts[i].ID]))
	}
	writeJSON(w, map[string]interface{}{"items": items, "total": len(items)})
}

func (h *HostAPI) createHost(w http.ResponseWriter, r *http.Request) {
	var payload hostPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, 400, "请求体格式错误")
		return
	}
	if err := validateHostPayload(&payload); err != nil {
		writeError(w, 400, err.Error())
		return
	}
	hostKey := normalizeHostKey(payload.HostKey)
	if hostKey == "" {
		hostKey = normalizeHostKey(payload.Name)
	}
	if hostKey == "" {
		hostKey = "host-" + strings.ReplaceAll(uuid.NewString()[:8], "-", "")
	}
	var count int64
	h.db.Model(&database.TerminalHost{}).Where("host_key = ?", hostKey).Count(&count)
	if count > 0 {
		hostKey = hostKey + "-" + strings.ReplaceAll(uuid.NewString()[:8], "-", "")
	}

	enabled := true
	if payload.Enabled != nil {
		enabled = *payload.Enabled
	}
	host := database.TerminalHost{
		Name:               strings.TrimSpace(payload.Name),
		HostKey:            hostKey,
		Address:            strings.TrimSpace(payload.Address),
		Port:               payload.Port,
		Username:           strings.TrimSpace(payload.Username),
		AuthType:           normalizeAuthType(payload.AuthType),
		HostKeyFingerprint: strings.TrimSpace(payload.HostKeyFingerprint),
		Env:                strings.TrimSpace(payload.Env),
		Group:              strings.TrimSpace(payload.Group),
		TagsJSON:           strings.TrimSpace(payload.TagsJSON),
		Description:        strings.TrimSpace(payload.Description),
		Enabled:            enabled,
		LastStatus:         "unknown",
	}
	if err := h.applySecrets(&host, payload); err != nil {
		writeError(w, 400, err.Error())
		return
	}
	if err := h.db.Create(&host).Error; err != nil {
		writeError(w, 500, err.Error())
		return
	}
	if err := h.ensureDefaultHostCommands(host.ID); err != nil {
		log.Printf("[Host] 创建默认初始化指令失败 host=%d: %v", host.ID, err)
	}
	commands := h.commandsForHosts([]database.TerminalHost{host})[host.ID]
	writeJSON(w, toHostView(host, commands))
}

func (h *HostAPI) handleHostByID(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/promai/hosts/")
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		http.NotFound(w, r)
		return
	}
	id, err := strconv.ParseUint(parts[0], 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	hostID := uint(id)

	if len(parts) >= 2 && parts[1] == "terminal" && r.Method == http.MethodGet {
		h.handleTerminalWS(w, r, hostID)
		return
	}

	host, err := h.loadHost(hostID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			writeError(w, 404, "主机不存在")
			return
		}
		writeError(w, 500, err.Error())
		return
	}

	if len(parts) == 1 {
		switch r.Method {
		case http.MethodGet:
			commands := h.commandsForHosts([]database.TerminalHost{*host})[host.ID]
			writeJSON(w, toHostView(*host, commands))
		case http.MethodPut:
			h.updateHost(w, r, host)
		case http.MethodDelete:
			h.deleteHost(w, host)
		default:
			writeError(w, 405, "不支持的请求方法")
		}
		return
	}

	switch parts[1] {
	case "initial-commands":
		if r.Method != http.MethodPut && r.Method != http.MethodPost {
			writeError(w, 405, "不支持的请求方法")
			return
		}
		h.setInitialCommands(w, r, host)
	case "test":
		if r.Method != http.MethodPost {
			writeError(w, 405, "不支持的请求方法")
			return
		}
		h.testHost(w, host)
	case "exec":
		if r.Method != http.MethodPost {
			writeError(w, 405, "不支持的请求方法")
			return
		}
		h.execHost(w, r, host)
	case "commands":
		h.handleHostCommands(w, r, host, parts[2:])
	case "logs":
		if r.Method != http.MethodGet {
			writeError(w, 405, "不支持的请求方法")
			return
		}
		h.listExecLogs(w, r, host)
	default:
		http.NotFound(w, r)
	}
}

func (h *HostAPI) updateHost(w http.ResponseWriter, r *http.Request, host *database.TerminalHost) {
	var payload hostPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, 400, "请求体格式错误")
		return
	}
	if err := validateHostPayload(&payload); err != nil {
		writeError(w, 400, err.Error())
		return
	}
	hostKey := normalizeHostKey(payload.HostKey)
	if hostKey == "" {
		hostKey = host.HostKey
	}
	var count int64
	h.db.Model(&database.TerminalHost{}).Where("host_key = ? AND id <> ?", hostKey, host.ID).Count(&count)
	if count > 0 {
		writeError(w, 400, "主机标识已存在")
		return
	}
	host.Name = strings.TrimSpace(payload.Name)
	host.HostKey = hostKey
	host.Address = strings.TrimSpace(payload.Address)
	host.Port = payload.Port
	host.Username = strings.TrimSpace(payload.Username)
	host.AuthType = normalizeAuthType(payload.AuthType)
	host.HostKeyFingerprint = strings.TrimSpace(payload.HostKeyFingerprint)
	host.Env = strings.TrimSpace(payload.Env)
	host.Group = strings.TrimSpace(payload.Group)
	host.TagsJSON = strings.TrimSpace(payload.TagsJSON)
	host.Description = strings.TrimSpace(payload.Description)
	if payload.Enabled != nil {
		host.Enabled = *payload.Enabled
	}
	if err := h.applySecrets(host, payload); err != nil {
		writeError(w, 400, err.Error())
		return
	}
	if err := h.db.Save(host).Error; err != nil {
		writeError(w, 500, err.Error())
		return
	}
	commands := h.commandsForHosts([]database.TerminalHost{*host})[host.ID]
	writeJSON(w, toHostView(*host, commands))
}

func (h *HostAPI) deleteHost(w http.ResponseWriter, host *database.TerminalHost) {
	err := h.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("host_id = ?", host.ID).Delete(&database.TerminalHostCommand{}).Error; err != nil {
			return err
		}
		return tx.Delete(&database.TerminalHost{}, host.ID).Error
	})
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	writeJSON(w, map[string]interface{}{"success": true})
}

func (h *HostAPI) testHost(w http.ResponseWriter, host *database.TerminalHost) {
	start := time.Now()
	target, err := h.targetFor(host)
	if err != nil {
		h.updateHostStatus(host, "offline", err.Error())
		writeJSON(w, map[string]interface{}{"success": false, "message": err.Error(), "duration_ms": time.Since(start).Milliseconds()})
		return
	}
	client, err := terminal.Dial(target)
	if err != nil {
		h.updateHostStatus(host, "offline", err.Error())
		writeJSON(w, map[string]interface{}{"success": false, "message": err.Error(), "duration_ms": time.Since(start).Milliseconds()})
		return
	}
	_ = client.Close()
	h.updateHostStatus(host, "online", "")
	writeJSON(w, map[string]interface{}{"success": true, "message": "SSH 连接成功", "duration_ms": time.Since(start).Milliseconds()})
}

func (h *HostAPI) updateHostStatus(host *database.TerminalHost, status, lastErr string) {
	now := time.Now()
	host.LastStatus = status
	host.LastCheckedAt = &now
	host.LastError = lastErr
	_ = h.db.Model(host).Updates(map[string]interface{}{
		"last_status":     status,
		"last_checked_at": &now,
		"last_error":      lastErr,
	}).Error
}

// trySaveHostFingerprint 尝试获取并保存主机 SSH 指纹（TOFU 模式）。
// 仅在主机未配置指纹时调用，异步执行不影响主流程。
func (h *HostAPI) trySaveHostFingerprint(host *database.TerminalHost) {
	fp, err := terminal.FetchHostKeyFingerprint(host.Address, host.Port, 10*time.Second)
	if err != nil {
		log.Printf("[Host] TOFU: 获取主机 %s 指纹失败: %v", host.Name, err)
		return
	}
	log.Printf("[Host] TOFU: 主机 %s 指纹已自动保存 %s", host.Name, fp)
	_ = h.db.Model(host).Update("host_key_fingerprint", fp).Error
}

func (h *HostAPI) execHost(w http.ResponseWriter, r *http.Request, host *database.TerminalHost) {
	var req struct {
		Command    string `json:"command"`
		TimeoutSec int    `json:"timeout_sec"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, 400, "请求体格式错误")
		return
	}
	h.runCommand(w, r, host, req.Command, req.TimeoutSec, "manual")
}

func (h *HostAPI) runCommand(w http.ResponseWriter, r *http.Request, host *database.TerminalHost, command string, timeoutSec int, source string) {
	command = strings.TrimSpace(command)
	if command == "" {
		writeError(w, 400, "命令不能为空")
		return
	}
	target, err := h.targetFor(host)
	if err != nil {
		writeError(w, 400, err.Error())
		return
	}
	if timeoutSec <= 0 {
		timeoutSec = 60
	}
	if timeoutSec > 1800 {
		timeoutSec = 1800
	}
	start := time.Now()
	output, exitCode, runErr := terminal.RunCommand(target, command, time.Duration(timeoutSec)*time.Second)
	duration := time.Since(start)

	entry := database.TerminalExecLog{
		HostID:     host.ID,
		HostKey:    host.HostKey,
		Username:   currentUsername(r),
		Source:     source,
		Command:    command,
		ExitCode:   exitCode,
		Output:     truncateText(output, 20000),
		DurationMS: duration.Milliseconds(),
	}
	if runErr != nil {
		entry.Error = runErr.Error()
	}
	_ = h.db.Create(&entry).Error

	result := map[string]interface{}{
		"success":     runErr == nil,
		"output":      output,
		"exit_code":   exitCode,
		"duration_ms": duration.Milliseconds(),
	}
	if runErr != nil {
		result["error"] = runErr.Error()
	}
	writeJSON(w, result)
}

func (h *HostAPI) listExecLogs(w http.ResponseWriter, r *http.Request, host *database.TerminalHost) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	var logs []database.TerminalExecLog
	if err := h.db.Where("host_id = ?", host.ID).Order("id desc").Limit(limit).Find(&logs).Error; err != nil {
		writeError(w, 500, err.Error())
		return
	}
	writeJSON(w, map[string]interface{}{"items": logs, "total": len(logs)})
}

func (h *HostAPI) handleHostCommands(w http.ResponseWriter, r *http.Request, host *database.TerminalHost, rest []string) {
	if len(rest) == 0 {
		switch r.Method {
		case http.MethodGet:
			commands := h.commandsForHosts([]database.TerminalHost{*host})[host.ID]
			writeJSON(w, map[string]interface{}{"items": commands, "total": len(commands)})
		case http.MethodPost:
			h.createHostCommand(w, r, host)
		default:
			writeError(w, 405, "不支持的请求方法")
		}
		return
	}
	cmdID, err := strconv.ParseUint(rest[0], 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	var command database.TerminalHostCommand
	if err := h.db.Where("id = ? AND host_id = ?", uint(cmdID), host.ID).First(&command).Error; err != nil {
		writeError(w, 404, "命令不存在")
		return
	}
	if len(rest) >= 2 && rest[1] == "run" {
		if r.Method != http.MethodPost {
			writeError(w, 405, "不支持的请求方法")
			return
		}
		h.runCommand(w, r, host, command.Command, 60, "quick")
		return
	}
	switch r.Method {
	case http.MethodPut:
		var payload hostCommandPayload
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			writeError(w, 400, "请求体格式错误")
			return
		}
		if err := validateCommandPayload(&payload); err != nil {
			writeError(w, 400, err.Error())
			return
		}
		command.Name = strings.TrimSpace(payload.Name)
		command.Command = strings.TrimSpace(payload.Command)
		command.Description = strings.TrimSpace(payload.Description)
		command.AutoRun = payload.AutoRun
		command.SortOrder = payload.SortOrder
		if err := h.db.Save(&command).Error; err != nil {
			writeError(w, 500, err.Error())
			return
		}
		writeJSON(w, command)
	case http.MethodDelete:
		if err := h.db.Delete(&command).Error; err != nil {
			writeError(w, 500, err.Error())
			return
		}
		writeJSON(w, map[string]interface{}{"success": true})
	default:
		writeError(w, 405, "不支持的请求方法")
	}
}

func (h *HostAPI) createHostCommand(w http.ResponseWriter, r *http.Request, host *database.TerminalHost) {
	var payload hostCommandPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, 400, "请求体格式错误")
		return
	}
	if err := validateCommandPayload(&payload); err != nil {
		writeError(w, 400, err.Error())
		return
	}
	command := database.TerminalHostCommand{
		HostID:      host.ID,
		Name:        strings.TrimSpace(payload.Name),
		Command:     strings.TrimSpace(payload.Command),
		Description: strings.TrimSpace(payload.Description),
		AutoRun:     payload.AutoRun,
		SortOrder:   payload.SortOrder,
	}
	if err := h.db.Create(&command).Error; err != nil {
		writeError(w, 500, err.Error())
		return
	}
	writeJSON(w, command)
}

func (h *HostAPI) handleTerminalWS(w http.ResponseWriter, r *http.Request, hostID uint) {
	host, err := h.loadHost(hostID)
	if err != nil {
		writeError(w, 404, "主机不存在")
		return
	}
	if !host.Enabled {
		writeError(w, 403, "主机已禁用")
		return
	}
	target, err := h.targetFor(host)
	if err != nil {
		writeError(w, 400, err.Error())
		return
	}

	cols, _ := strconv.Atoi(r.URL.Query().Get("cols"))
	rows, _ := strconv.Atoi(r.URL.Query().Get("rows"))
	upgrader := websocket.Upgrader{
		ReadBufferSize:  8192,
		WriteBufferSize: 8192,
		CheckOrigin:     func(*http.Request) bool { return true },
	}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[Host] WebSocket 升级失败: %v", err)
		return
	}
	defer conn.Close()

	shell, err := terminal.OpenShell(target, cols, rows)
	if err != nil {
		_ = conn.WriteJSON(map[string]interface{}{"type": "error", "message": err.Error()})
		h.updateHostStatus(host, "offline", err.Error())
		return
	}
	h.updateHostStatus(host, "online", "")
	defer shell.Close()

	// TOFU: 首次连接自动保存主机指纹
	if strings.TrimSpace(host.HostKeyFingerprint) == "" {
		go h.trySaveHostFingerprint(host)
	}

	writer := &wsJSONWriter{conn: conn}
	_ = writer.send(map[string]interface{}{
		"type":     "ready",
		"host":     host.Name,
		"host_key": host.HostKey,
		"host_id":  host.ID,
	})

	username := currentUsername(r)
	activity := make(chan struct{}, 1)
	outDone := make(chan struct{})
	go func() {
		defer close(outDone)
		buf := make([]byte, 8192)
		for {
			n, readErr := shell.Read(buf)
			if n > 0 {
				select {
				case activity <- struct{}{}:
				default:
				}
				if err := writer.send(map[string]interface{}{"type": "output", "data": string(buf[:n])}); err != nil {
					return
				}
			}
			if readErr != nil {
				_ = writer.send(map[string]interface{}{"type": "closed", "message": "会话已结束"})
				return
			}
		}
	}()

	// 初始化命令逐行下发：每行等待终端输出稳定后再发送，兼容堡垒机菜单、
	// 登录提示等交互流程，避免一次性写入导致后续命令被丢弃。
	autoCommands := h.autoRunCommands(host.ID)
	if len(autoCommands) > 0 {
		go func() {
			time.Sleep(300 * time.Millisecond)
			for _, cmd := range autoCommands {
				for _, line := range splitCommandLines(cmd.Command) {
					waitOutputIdle(activity, 600*time.Millisecond, 8*time.Second)
					if _, err := shell.Write([]byte(line + "\n")); err != nil {
						return
					}
					_ = h.db.Create(&database.TerminalExecLog{
						HostID:   host.ID,
						HostKey:  host.HostKey,
						Username: username,
						Source:   "auto",
						Command:  line,
					}).Error
				}
			}
		}()
	}

	for {
		var msg struct {
			Type string `json:"type"`
			Data string `json:"data"`
			Cols int    `json:"cols"`
			Rows int    `json:"rows"`
		}
		if err := conn.ReadJSON(&msg); err != nil {
			break
		}
		switch msg.Type {
		case "input":
			if msg.Data != "" {
				if _, err := shell.Write([]byte(msg.Data)); err != nil {
					return
				}
			}
		case "resize":
			_ = shell.Resize(msg.Cols, msg.Rows)
		case "ping":
			_ = writer.send(map[string]interface{}{"type": "pong"})
		}
	}
	_ = shell.Close()
	<-outDone
}

func splitCommandLines(command string) []string {
	command = strings.ReplaceAll(command, "\r\n", "\n")
	command = strings.ReplaceAll(command, "\r", "\n")
	lines := make([]string, 0)
	for _, line := range strings.Split(command, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

// waitOutputIdle 等到终端连续 idle 时间没有输出，最多等待 max。
func waitOutputIdle(activity <-chan struct{}, idle, max time.Duration) {
	idleTimer := time.NewTimer(idle)
	maxTimer := time.NewTimer(max)
	defer idleTimer.Stop()
	defer maxTimer.Stop()
	for {
		select {
		case <-activity:
			if !idleTimer.Stop() {
				select {
				case <-idleTimer.C:
				default:
				}
			}
			idleTimer.Reset(idle)
		case <-idleTimer.C:
			return
		case <-maxTimer.C:
			return
		}
	}
}

type wsJSONWriter struct {
	conn *websocket.Conn
	mu   sync.Mutex
}

func (w *wsJSONWriter) send(v interface{}) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.conn.WriteJSON(v)
}

func (h *HostAPI) loadHost(id uint) (*database.TerminalHost, error) {
	var host database.TerminalHost
	if err := h.db.First(&host, id).Error; err != nil {
		return nil, err
	}
	return &host, nil
}

func (h *HostAPI) targetFor(host *database.TerminalHost) (terminal.SSHTarget, error) {
	password, err := h.vault.Decrypt(host.PasswordEnc)
	if err != nil {
		return terminal.SSHTarget{}, err
	}
	privateKey, err := h.vault.Decrypt(host.PrivateKeyEnc)
	if err != nil {
		return terminal.SSHTarget{}, err
	}
	passphrase, err := h.vault.Decrypt(host.PassphraseEnc)
	if err != nil {
		return terminal.SSHTarget{}, err
	}
	return terminal.SSHTarget{
		Address:            host.Address,
		Port:               host.Port,
		Username:           host.Username,
		AuthType:           host.AuthType,
		Password:           password,
		PrivateKey:         privateKey,
		Passphrase:         passphrase,
		HostKeyFingerprint: host.HostKeyFingerprint,
		Timeout:            10 * time.Second,
	}, nil
}

func (h *HostAPI) applySecrets(host *database.TerminalHost, payload hostPayload) error {
	authType := normalizeAuthType(payload.AuthType)
	if payload.ClearPassword {
		host.PasswordEnc = ""
	}
	if payload.ClearPrivateKey {
		host.PrivateKeyEnc = ""
	}
	if strings.TrimSpace(payload.Password) != "" {
		enc, err := h.vault.Encrypt(payload.Password)
		if err != nil {
			return err
		}
		host.PasswordEnc = enc
	}
	if strings.TrimSpace(payload.PrivateKey) != "" {
		enc, err := h.vault.Encrypt(payload.PrivateKey)
		if err != nil {
			return err
		}
		host.PrivateKeyEnc = enc
	}
	if strings.TrimSpace(payload.Passphrase) != "" {
		enc, err := h.vault.Encrypt(payload.Passphrase)
		if err != nil {
			return err
		}
		host.PassphraseEnc = enc
	}
	if authType == "private_key" && host.PrivateKeyEnc == "" {
		return fmt.Errorf("密钥登录需要填写 SSH 私钥")
	}
	if authType == "password" && host.PasswordEnc == "" {
		return fmt.Errorf("密码登录需要填写 SSH 密码")
	}
	return nil
}

var defaultHostCommandTemplates = []struct {
	Name        string
	Command     string
	Description string
}{
	{
		Name:        "查看主机名称",
		Command:     "hostname",
		Description: "打印目标主机的 hostname",
	},
	{
		Name:        "查看主机 IP 地址",
		Command:     "hostname -I 2>/dev/null || ip -4 addr show scope global | awk '/inet /{print $2}'",
		Description: "打印目标主机的 IPv4 地址",
	},
}

// ensureDefaultHostCommands 初始化执行指令：每个主机至少保留“查看主机名称”和“查看 IP”。
// 只有主机没有任何自定义命令时才写入，避免覆盖用户配置。
func (h *HostAPI) ensureDefaultHostCommands(hostID uint) error {
	var count int64
	if err := h.db.Model(&database.TerminalHostCommand{}).Where("host_id = ?", hostID).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	commands := make([]database.TerminalHostCommand, 0, len(defaultHostCommandTemplates))
	for i, item := range defaultHostCommandTemplates {
		commands = append(commands, database.TerminalHostCommand{
			HostID:      hostID,
			Name:        item.Name,
			Command:     item.Command,
			Description: item.Description,
			AutoRun:     true,
			SortOrder:   i,
		})
	}
	return h.db.Create(&commands).Error
}

func (h *HostAPI) ensureAllDefaultHostCommands() {
	var hosts []database.TerminalHost
	if err := h.db.Select("id").Find(&hosts).Error; err != nil {
		return
	}
	for _, host := range hosts {
		if err := h.ensureDefaultHostCommands(host.ID); err != nil {
			log.Printf("[Host] 初始化默认指令失败 host=%d: %v", host.ID, err)
		}
	}
}

// setInitialCommands 用多行文本整体替换主机的初始化命令。
// 每行一条命令，SSH 登录成功后由终端逐行发送并自动换行。
func (h *HostAPI) setInitialCommands(w http.ResponseWriter, r *http.Request, host *database.TerminalHost) {
	var req struct {
		Commands []string `json:"commands"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, 400, "请求体格式错误")
		return
	}
	lines := make([]string, 0, len(req.Commands))
	for _, raw := range req.Commands {
		for _, line := range strings.Split(raw, "\n") {
			line = strings.TrimSpace(line)
			if line != "" {
				lines = append(lines, line)
			}
		}
	}
	if len(lines) > 100 {
		writeError(w, 400, "初始化命令不能超过 100 条")
		return
	}
	for _, line := range lines {
		if len(line) > 2000 {
			writeError(w, 400, "单条初始化命令不能超过 2000 字符")
			return
		}
	}

	var existing []database.TerminalHostCommand
	if err := h.db.Where("host_id = ?", host.ID).Order("sort_order asc, id asc").Find(&existing).Error; err != nil {
		writeError(w, 500, err.Error())
		return
	}
	err := h.db.Transaction(func(tx *gorm.DB) error {
		for i, line := range lines {
			name := initialCommandName(line)
			if i < len(existing) && existing[i].Command == line && existing[i].Name != "" {
				name = existing[i].Name
			}
			if i < len(existing) {
				cmd := existing[i]
				cmd.Name = name
				cmd.Command = line
				cmd.AutoRun = true
				cmd.SortOrder = i
				if err := tx.Save(&cmd).Error; err != nil {
					return err
				}
				continue
			}
			cmd := database.TerminalHostCommand{
				HostID:    host.ID,
				Name:      name,
				Command:   line,
				AutoRun:   true,
				SortOrder: i,
			}
			if err := tx.Create(&cmd).Error; err != nil {
				return err
			}
		}
		if len(existing) > len(lines) {
			ids := make([]uint, 0, len(existing)-len(lines))
			for _, cmd := range existing[len(lines):] {
				ids = append(ids, cmd.ID)
			}
			if err := tx.Where("host_id = ? AND id IN ?", host.ID, ids).Delete(&database.TerminalHostCommand{}).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	commands := h.commandsForHosts([]database.TerminalHost{*host})[host.ID]
	writeJSON(w, map[string]interface{}{"items": commands, "total": len(commands)})
}

func initialCommandName(command string) string {
	command = strings.TrimSpace(command)
	if len(command) > 64 {
		return command[:61] + "..."
	}
	return command
}

func (h *HostAPI) commandsForHosts(hosts []database.TerminalHost) map[uint][]database.TerminalHostCommand {
	result := make(map[uint][]database.TerminalHostCommand)
	if len(hosts) == 0 {
		return result
	}
	ids := make([]uint, 0, len(hosts))
	for _, host := range hosts {
		ids = append(ids, host.ID)
	}
	var commands []database.TerminalHostCommand
	if err := h.db.Where("host_id IN ?", ids).Order("sort_order asc, id asc").Find(&commands).Error; err != nil {
		return result
	}
	for _, cmd := range commands {
		result[cmd.HostID] = append(result[cmd.HostID], cmd)
	}
	return result
}

func (h *HostAPI) autoRunCommands(hostID uint) []database.TerminalHostCommand {
	var commands []database.TerminalHostCommand
	_ = h.db.Where("host_id = ? AND auto_run = ?", hostID, true).Order("sort_order asc, id asc").Find(&commands).Error
	return commands
}

func toHostView(host database.TerminalHost, commands []database.TerminalHostCommand) hostView {
	return hostView{
		TerminalHost:  host,
		HasPassword:   host.PasswordEnc != "",
		HasPrivateKey: host.PrivateKeyEnc != "",
		Commands:      commands,
	}
}

func validateHostPayload(payload *hostPayload) error {
	if strings.TrimSpace(payload.Name) == "" {
		return fmt.Errorf("主机名称不能为空")
	}
	if strings.TrimSpace(payload.Address) == "" {
		return fmt.Errorf("主机地址不能为空")
	}
	if strings.TrimSpace(payload.Username) == "" {
		return fmt.Errorf("SSH 用户名不能为空")
	}
	if payload.Port <= 0 {
		payload.Port = 22
	}
	if payload.Port > 65535 {
		return fmt.Errorf("端口范围无效")
	}
	payload.AuthType = normalizeAuthType(payload.AuthType)
	return nil
}

func validateCommandPayload(payload *hostCommandPayload) error {
	if strings.TrimSpace(payload.Name) == "" {
		return fmt.Errorf("命令名称不能为空")
	}
	if strings.TrimSpace(payload.Command) == "" {
		return fmt.Errorf("命令内容不能为空")
	}
	return nil
}

func normalizeAuthType(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "private_key", "key":
		return "private_key"
	default:
		return "password"
	}
}

func normalizeHostKey(v string) string {
	v = strings.ToLower(strings.TrimSpace(v))
	var b strings.Builder
	lastDash := false
	for _, r := range v {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' {
			b.WriteRune(r)
			lastDash = r == '-'
			continue
		}
		if !lastDash {
			b.WriteRune('-')
			lastDash = true
		}
	}
	return strings.Trim(b.String(), "-.")
}

func truncateText(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "\n...[已截断]"
}
