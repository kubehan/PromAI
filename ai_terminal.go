package main

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"PromAI/pkg/aiagent/proxy"
	"PromAI/pkg/config"
	"PromAI/pkg/database"
	"PromAI/pkg/terminal"

	"gorm.io/gorm"
)

// AiTerminalAPI 自动化运维智能终端：会话 / AI 规划 / 审批执行 / 审计。
type AiTerminalAPI struct {
	db   *gorm.DB
	api  *AdminAPI
	host *HostAPI
	cfg  *config.Config
}

const (
	stepPending  = "pending"
	stepApproved = "approved"
	stepRejected = "rejected"
	stepRunning  = "running"
	stepSuccess  = "success"
	stepFailed   = "failed"
)

func registerAiTerminalAPI(mux *http.ServeMux, api *AdminAPI) {
	vault, err := terminal.VaultFromEnv(api.jwtSecret)
	if err != nil {
		log.Printf("[AiTerminal] 凭据加密初始化失败: %v", err)
		vault = nil
	}
	t := &AiTerminalAPI{
		db:   database.DB,
		api:  api,
		cfg:  api.config,
		host: &HostAPI{db: database.DB, api: api, vault: vault},
	}
	mux.HandleFunc("/api/promai/ai-terminal/", api.logRequest(api.authMiddleware(t.handle)))
	mux.HandleFunc("/promai/share/", t.handlePublicSharePage)
	log.Printf("[AiTerminal] 自动化运维智能终端接口已注册")
}

func (a *AiTerminalAPI) handle(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/promai/ai-terminal/")
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		writeError(w, 404, "接口不存在")
		return
	}
	switch parts[0] {
	case "sessions":
		a.handleSessions(w, r, parts)
	case "steps":
		a.handleSteps(w, r, parts)
	case "prompts":
		a.handlePrompts(w, r)
	case "share":
		a.handleShare(w, r, parts)
	default:
		writeError(w, 404, "接口不存在")
	}
}

const promptSystemKey = "ai_terminal_system_prompt"
const promptContinueKey = "ai_terminal_continue_prompt"

const defaultTerminalSystemPrompt = `你是运维命令助手，正在通过一个真实终端执行用户明确下达的任务。你必须只输出一个 JSON 对象，不要输出任何解释或 Markdown。JSON 结构为 {"analysis":"对当前问题/上一步结果的简要分析","steps":[{"command":"...","purpose":"...","risk":"low|medium|high"}],"done":false}。risk=low 只读查询；medium 写入或服务操作；high 破坏性/危险操作（如 rm -rf、mkfs、kill -9、dd、shutdown）。最重要的规则：只执行完成用户请求所必需的最少命令，通常一轮只给 1 条，最多 2 条；严禁执行与用户请求无关的系统检查、资源统计、性能测试、联网探测等命令（例如 free/top/df/curl/ping 等），除非用户明确要求。如果用户已明确指定要做的操作（例如删除某个压缩包），直接给出对应命令，不要做额外的环境摸底或排障发散；只有当目标不明确、确实缺少必要信息时，才允许先用一次最小范围的只读查询定位（例如 ls/find 目标目录），并在 analysis 中说明原因。每个命令的 purpose 必须直接对应用户请求。命令会在交互式终端真实执行，结果回传给你，请勿臆造输出。任务完成或无需再执行命令时，返回 done=true 且 steps 为空数组，并在 analysis 中给出结论。禁止输出 SSH/登录类命令，只给出在目标主机上执行的原始命令，命令尽量单行。`

const defaultTerminalContinuePrompt = `请根据以上执行结果继续。若用户请求已完成，请设置 done=true 并在 analysis 中说明结论；否则只给出完成该请求所必需的下一步命令，不要做无关的额外检查。`

func (a *AiTerminalAPI) settingValue(key, def string) string {
	var s database.AppSetting
	if a.db != nil && a.db.Where("key = ?", key).First(&s).Error == nil {
		if strings.TrimSpace(s.Value) != "" {
			return s.Value
		}
	}
	return def
}

func (a *AiTerminalAPI) handlePrompts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, 405, "不支持的请求方法")
		return
	}
	writeJSON(w, map[string]string{
		"system_prompt":           a.settingValue(promptSystemKey, defaultTerminalSystemPrompt),
		"continue_prompt":         a.settingValue(promptContinueKey, defaultTerminalContinuePrompt),
		"default_system_prompt":   defaultTerminalSystemPrompt,
		"default_continue_prompt": defaultTerminalContinuePrompt,
	})
}

func (a *AiTerminalAPI) handleShare(w http.ResponseWriter, r *http.Request, parts []string) {
	if len(parts) == 1 && r.Method == http.MethodPost {
		a.createShare(w, r)
		return
	}
	if len(parts) >= 2 {
		token := strings.TrimSpace(parts[1])
		if token == "" {
			writeError(w, 400, "分享 token 无效")
			return
		}
		if len(parts) == 3 && parts[2] == "markdown" && r.Method == http.MethodGet {
			a.shareMarkdown(w, r, token)
			return
		}
		if len(parts) == 3 && parts[2] == "html" && r.Method == http.MethodGet {
			a.shareHTML(w, r, token)
			return
		}
	}
	writeError(w, 405, "不支持的请求方法")
}

// ---------------- sessions ----------------

func (a *AiTerminalAPI) handleSessions(w http.ResponseWriter, r *http.Request, parts []string) {
	if len(parts) == 1 && r.Method == http.MethodGet {
		a.listSessions(w, r)
		return
	}
	if len(parts) == 1 && r.Method == http.MethodPost {
		a.createSession(w, r)
		return
	}
	if len(parts) >= 2 {
		id, err := strconv.ParseUint(parts[1], 10, 64)
		if err != nil {
			writeError(w, 400, "会话 ID 无效")
			return
		}
		sid := uint(id)
		if len(parts) >= 3 && parts[2] == "plan" && r.Method == http.MethodPost {
			a.plan(w, r, sid, false)
			return
		}
		if len(parts) >= 3 && parts[2] == "next" && r.Method == http.MethodPost {
			a.plan(w, r, sid, true)
			return
		}
		if len(parts) >= 3 && parts[2] == "messages" && r.Method == http.MethodPost {
			a.appendMessage(w, r, sid)
			return
		}
		if len(parts) == 2 && r.Method == http.MethodGet {
			a.sessionDetail(w, r, sid)
			return
		}
	}
	writeError(w, 405, "不支持的请求方法")
}

func (a *AiTerminalAPI) listSessions(w http.ResponseWriter, r *http.Request) {
	var items []database.OpsSession
	if err := a.db.Order("id desc").Limit(100).Find(&items).Error; err != nil {
		writeError(w, 500, err.Error())
		return
	}
	writeJSON(w, map[string]interface{}{"items": items, "total": len(items)})
}

func (a *AiTerminalAPI) createSession(w http.ResponseWriter, r *http.Request) {
	var req struct {
		HostID     uint   `json:"host_id"`
		IncidentID *uint  `json:"incident_id"`
		Title      string `json:"title"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, 400, "请求体格式错误")
		return
	}
	if req.HostID == 0 {
		writeError(w, 400, "请选择目标主机")
		return
	}
	var host database.TerminalHost
	if err := a.db.First(&host, req.HostID).Error; err != nil {
		writeError(w, 400, "目标主机不存在")
		return
	}
	s := database.OpsSession{
		Title:      strings.TrimSpace(req.Title),
		HostID:     host.ID,
		HostKey:    host.HostKey,
		IncidentID: req.IncidentID,
		Username:   currentUsername(r),
		Status:     "running",
	}
	if s.Title == "" {
		s.Title = "新会话"
	}
	if err := a.db.Create(&s).Error; err != nil {
		writeError(w, 500, err.Error())
		return
	}
	writeJSON(w, s)
}

func (a *AiTerminalAPI) sessionDetail(w http.ResponseWriter, r *http.Request, sid uint) {
	var session database.OpsSession
	if err := a.db.First(&session, sid).Error; err != nil {
		writeError(w, 404, "会话不存在")
		return
	}
	var steps []database.OpsStep
	var hostName string
	var host database.TerminalHost
	if err := a.db.First(&host, session.HostID).Error; err == nil {
		hostName = host.Name
	}
	a.db.Where("session_id = ?", sid).Order("seq asc, id asc").Find(&steps)
	var messages []database.OpsSessionMessage
	a.db.Where("session_id = ?", sid).Order("seq asc, id asc").Find(&messages)
	writeJSON(w, map[string]interface{}{
		"session":   session,
		"steps":     steps,
		"messages":  messages,
		"host_name": hostName,
	})
}

func (a *AiTerminalAPI) appendMessage(w http.ResponseWriter, r *http.Request, sid uint) {
	var req struct {
		Role     string `json:"role"`
		Text     string `json:"text"`
		Command  string `json:"command"`
		Risk     string `json:"risk"`
		Status   string `json:"status"`
		ExitCode int    `json:"exit_code"`
		Note     string `json:"note"`
		StepID   uint   `json:"step_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, 400, "请求体格式错误")
		return
	}
	var session database.OpsSession
	if err := a.db.First(&session, sid).Error; err != nil {
		writeError(w, 404, "会话不存在")
		return
	}
	var maxSeq int
	a.db.Model(&database.OpsSessionMessage{}).Where("session_id = ?", sid).Select("COALESCE(MAX(seq),0)").Scan(&maxSeq)
	msg := database.OpsSessionMessage{
		SessionID: sid,
		Seq:       maxSeq + 1,
		Role:      strings.TrimSpace(req.Role),
		Text:      req.Text,
		Command:   req.Command,
		Risk:      req.Risk,
		Status:    req.Status,
		ExitCode:  req.ExitCode,
		Note:      req.Note,
		StepID:    req.StepID,
	}
	if msg.Role == "" {
		msg.Role = "system"
	}
	if err := a.db.Create(&msg).Error; err != nil {
		writeError(w, 500, err.Error())
		return
	}
	writeJSON(w, msg)
}

// ---------------- steps ----------------

func (a *AiTerminalAPI) handleSteps(w http.ResponseWriter, r *http.Request, parts []string) {
	if len(parts) < 3 {
		writeError(w, 404, "接口不存在")
		return
	}
	id, err := strconv.ParseUint(parts[1], 10, 64)
	if err != nil {
		writeError(w, 400, "步骤 ID 无效")
		return
	}
	switch parts[2] {
	case "execute":
		if r.Method == http.MethodPost {
			a.executeStep(w, r, uint(id))
			return
		}
	case "approve":
		if r.Method == http.MethodPost {
			a.approveStep(w, r, uint(id))
			return
		}
	case "result":
		if r.Method == http.MethodPost {
			a.recordResult(w, r, uint(id))
			return
		}
	case "reject":
		if r.Method == http.MethodPost {
			a.rejectStep(w, r, uint(id))
			return
		}
	}
	writeError(w, 405, "不支持的请求方法")
}

// approveStep 仅做审批放行，不执行命令（命令由前端发送到终端执行）。
func (a *AiTerminalAPI) approveStep(w http.ResponseWriter, r *http.Request, stepID uint) {
	var req struct {
		Reason string `json:"reason"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	var step database.OpsStep
	if err := a.db.First(&step, stepID).Error; err != nil {
		writeError(w, 404, "步骤不存在")
		return
	}
	if step.Status != stepPending && step.Status != stepApproved {
		writeError(w, 400, "该步骤已处理")
		return
	}
	user := currentUsername(r)
	auto := step.RiskLevel == "low"
	markApproved(a.db, &step, user, req.Reason, auto)
	writeJSON(w, map[string]interface{}{"id": step.ID, "status": stepApproved, "auto_approved": auto})
}

// recordResult 记录前端在终端中执行命令后的输出与退出码。
func (a *AiTerminalAPI) recordResult(w http.ResponseWriter, r *http.Request, stepID uint) {
	var req struct {
		Output   string `json:"output"`
		ExitCode int    `json:"exit_code"`
		Error    string `json:"error"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, 400, "请求体格式错误")
		return
	}
	var step database.OpsStep
	if err := a.db.First(&step, stepID).Error; err != nil {
		writeError(w, 404, "步骤不存在")
		return
	}
	var session database.OpsSession
	if err := a.db.First(&session, step.SessionID).Error; err != nil {
		writeError(w, 500, "会话不存在")
		return
	}
	var host database.TerminalHost
	if err := a.db.First(&host, session.HostID).Error; err != nil {
		writeError(w, 500, "目标主机不存在")
		return
	}

	status := stepSuccess
	if req.ExitCode != 0 || req.Error != "" {
		status = stepFailed
	}
	_ = a.db.Model(&database.OpsStep{}).Where("id = ?", step.ID).Updates(map[string]interface{}{
		"status": status, "output": truncateText(req.Output, 20000),
		"exit_code": req.ExitCode, "error": req.Error, "executed_at": time.Now(),
	})
	_ = a.db.Create(&database.TerminalExecLog{
		HostID: host.ID, HostKey: host.HostKey, Username: currentUsername(r),
		Source: "ai", Command: step.Command, ExitCode: req.ExitCode,
		Output: truncateText(req.Output, 20000),
	}).Error
	writeJSON(w, map[string]interface{}{"id": step.ID, "status": status})
}

func (a *AiTerminalAPI) executeStep(w http.ResponseWriter, r *http.Request, stepID uint) {
	var req struct {
		Approve bool   `json:"approve"`
		Reason  string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, 400, "请求体格式错误")
		return
	}

	var step database.OpsStep
	if err := a.db.First(&step, stepID).Error; err != nil {
		writeError(w, 404, "步骤不存在")
		return
	}
	if step.Status != stepPending && step.Status != stepApproved {
		writeError(w, 400, "该步骤已处理，不能重复执行")
		return
	}

	user := currentUsername(r)
	switch step.RiskLevel {
	case "high":
		if !req.Approve || strings.TrimSpace(req.Reason) == "" {
			writeError(w, 400, "高危命令必须确认并填写审批理由")
			return
		}
		markApproved(a.db, &step, user, req.Reason, false)
	case "medium":
		if !req.Approve {
			writeError(w, 400, "该命令需要确认后执行")
			return
		}
		markApproved(a.db, &step, user, req.Reason, false)
	default: // low：自动放行
		markApproved(a.db, &step, user, "", true)
	}

	var session database.OpsSession
	if err := a.db.First(&session, step.SessionID).Error; err != nil {
		writeError(w, 500, "会话不存在")
		return
	}
	var host database.TerminalHost
	if err := a.db.First(&host, session.HostID).Error; err != nil {
		writeError(w, 500, "目标主机不存在")
		return
	}

	// 抢占执行（pending/approved → running），防止并发重复执行
	now := time.Now()
	res := a.db.Model(&database.OpsStep{}).
		Where("id = ? AND status IN ?", step.ID, []string{stepPending, stepApproved}).
		Updates(map[string]interface{}{"status": stepRunning, "approved_at": now, "executed_at": now})
	if res.RowsAffected == 0 {
		writeError(w, 409, "步骤已在执行，请勿重复提交")
		return
	}

	target, terr := a.host.targetFor(&host)
	if terr != nil {
		log.Printf("[AiTerminal] 步骤 %d 获取连接目标失败: %v", step.ID, terr)
		markFailed(a.db, &step, terr.Error())
		writeError(w, 500, terr.Error())
		return
	}
	start := time.Now()
	output, exitCode, runErr := terminal.RunCommand(target, step.Command, 120*time.Second)
	duration := time.Since(start)

	if runErr != nil {
		log.Printf("[AiTerminal] 步骤 %d 命令执行失败: cmd=%q exit=%d err=%v (耗时 %v)", step.ID, step.Command, exitCode, runErr, duration)
	}

	_ = a.db.Create(&database.TerminalExecLog{
		HostID:     host.ID,
		HostKey:    host.HostKey,
		Username:   user,
		Source:     "ai",
		Command:    step.Command,
		ExitCode:   exitCode,
		Output:     truncateText(output, 20000),
		DurationMS: duration.Milliseconds(),
	}).Error

	status := stepSuccess
	if runErr != nil {
		status = stepFailed
	}
	_ = a.db.Model(&database.OpsStep{}).Where("id = ?", step.ID).Updates(map[string]interface{}{
		"status": status, "output": truncateText(output, 20000), "exit_code": exitCode,
		"error": errString(runErr), "executed_at": time.Now(),
	})

	result := map[string]interface{}{
		"id":            step.ID,
		"status":        status,
		"exit_code":     exitCode,
		"output":        output,
		"error":         errString(runErr),
		"duration_ms":   duration.Milliseconds(),
		"approved_by":   user,
		"auto_approved": step.RiskLevel != "high" && step.RiskLevel != "medium",
	}
	writeJSON(w, result)
}

func (a *AiTerminalAPI) rejectStep(w http.ResponseWriter, r *http.Request, stepID uint) {
	var req struct {
		Reason string `json:"reason"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	var step database.OpsStep
	if err := a.db.First(&step, stepID).Error; err != nil {
		writeError(w, 404, "步骤不存在")
		return
	}
	if step.Status != stepPending && step.Status != stepApproved {
		writeError(w, 400, "该步骤已处理")
		return
	}
	now := time.Now()
	_ = a.db.Model(&database.OpsStep{}).Where("id = ?", step.ID).Updates(map[string]interface{}{
		"status": stepRejected, "reject_reason": req.Reason, "approved_at": now,
	})
	writeJSON(w, map[string]interface{}{"id": step.ID, "status": stepRejected})
}

func genShareToken() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

func hashPassword(p string) string {
	sum := sha256.Sum256([]byte(p))
	return hex.EncodeToString(sum[:])
}

func (a *AiTerminalAPI) createShare(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SessionID uint   `json:"session_id"`
		Password  string `json:"password"`
		ExpireH   int    `json:"expire_h"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, 400, "请求体格式错误")
		return
	}
	if req.SessionID == 0 {
		writeError(w, 400, "session_id 不能为空")
		return
	}
	var s database.OpsSession
	if err := a.db.First(&s, req.SessionID).Error; err != nil {
		writeError(w, 404, "会话不存在")
		return
	}
	share := database.OpsSessionShare{
		SessionID: req.SessionID,
		Token:     genShareToken(),
		ExpiresAt: time.Now().Add(24 * time.Hour),
		CreatedBy: currentUsername(r),
	}
	if req.ExpireH > 0 {
		if req.ExpireH > 24*30 {
			req.ExpireH = 24 * 30
		}
		share.ExpiresAt = time.Now().Add(time.Duration(req.ExpireH) * time.Hour)
	}
	if strings.TrimSpace(req.Password) != "" {
		share.PasswordHash = hashPassword(strings.TrimSpace(req.Password))
	}
	if err := a.db.Create(&share).Error; err != nil {
		writeError(w, 500, err.Error())
		return
	}
	base := strings.TrimRight(r.Host, "/")
	scheme := "http"
	if r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") {
		scheme = "https"
	}
	writeJSON(w, map[string]interface{}{
		"token":         share.Token,
		"password_mode": share.PasswordHash != "",
		"share_url":     fmt.Sprintf("%s://%s/promai/share/%s", scheme, base, share.Token),
		"expires_at":    share.ExpiresAt,
		"markdown_url":  fmt.Sprintf("%s://%s/api/promai/ai-terminal/share/%s/markdown", scheme, base, share.Token),
		"html_url":      fmt.Sprintf("%s://%s/api/promai/ai-terminal/share/%s/html", scheme, base, share.Token),
	})
}

func (a *AiTerminalAPI) verifyShare(r *http.Request, token string) (*database.OpsSessionShare, bool) {
	var share database.OpsSessionShare
	if err := a.db.Where("token = ?", token).First(&share).Error; err != nil {
		return nil, false
	}
	if share.PasswordHash == "" {
		if !share.ExpiresAt.IsZero() && time.Now().After(share.ExpiresAt) {
			return &share, false
		}
		return &share, true
	}
	if !share.ExpiresAt.IsZero() && time.Now().After(share.ExpiresAt) {
		return &share, false
	}
	pwd := strings.TrimSpace(r.URL.Query().Get("password"))
	if pwd != "" && hashPassword(pwd) == share.PasswordHash {
		return &share, true
	}
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(strings.ToLower(auth), "bearer ") {
		if hashPassword(strings.TrimSpace(auth[7:])) == share.PasswordHash {
			return &share, true
		}
	}
	return &share, false
}

func (a *AiTerminalAPI) buildShareMarkdown(session database.OpsSession, hostName string, messages []database.OpsSessionMessage, steps []database.OpsStep) string {
	b := &strings.Builder{}
	fmt.Fprintf(b, "# 自动化运维会话分享\n\n")
	fmt.Fprintf(b, "- 会话ID: %d\n", session.ID)
	fmt.Fprintf(b, "- 标题: %s\n", session.Title)
	fmt.Fprintf(b, "- 主机: %s\n", hostName)
	fmt.Fprintf(b, "- 发起人: %s\n", session.Username)
	fmt.Fprintf(b, "- 创建时间: %s\n\n", session.CreatedAt.Format("2006-01-02 15:04:05"))
	fmt.Fprintf(b, "## 交互过程\n\n")
	if len(messages) > 0 {
		for _, m := range messages {
			switch m.Role {
			case "user":
				fmt.Fprintf(b, "### 用户\n\n%s\n\n", m.Text)
			case "assistant":
				fmt.Fprintf(b, "### AI 分析\n\n%s\n\n", m.Text)
			case "command":
				fmt.Fprintf(b, "### 命令\n\n")
				if m.Command != "" {
					fmt.Fprintf(b, "- 命令: `%s`\n", m.Command)
				}
				if m.Risk != "" {
					fmt.Fprintf(b, "- 风险: %s\n", m.Risk)
				}
				if m.Status != "" {
					fmt.Fprintf(b, "- 状态: %s\n", m.Status)
				}
				if m.Note != "" {
					fmt.Fprintf(b, "- 说明: %s\n", m.Note)
				}
				fmt.Fprintf(b, "\n")
			case "result":
				fmt.Fprintf(b, "### 执行结果\n\n- exit: %d\n\n```\n%s\n```\n\n", m.ExitCode, truncateText(m.Text, 20000))
			default:
				if strings.TrimSpace(m.Text) != "" {
					fmt.Fprintf(b, "### 系统\n\n%s\n\n", m.Text)
				}
			}
		}
		return b.String()
	}
	for _, st := range steps {
		fmt.Fprintf(b, "### 步骤 %d\n\n", st.Seq)
		fmt.Fprintf(b, "- 命令: `%s`\n", st.Command)
		fmt.Fprintf(b, "- 作用: %s\n", st.Purpose)
		fmt.Fprintf(b, "- 风险: %s\n", st.RiskLevel)
		fmt.Fprintf(b, "- 状态: %s\n", st.Status)
		if st.RejectReason != "" {
			fmt.Fprintf(b, "- 拒绝原因: %s\n", st.RejectReason)
		}
		if st.Output != "" || st.Error != "" {
			fmt.Fprintf(b, "\n#### 执行输出\n\n```")
			fmt.Fprintf(b, "\n%s", truncateText(st.Output, 20000))
			if st.Error != "" {
				fmt.Fprintf(b, "\n[error] %s", st.Error)
			}
			fmt.Fprintf(b, "\n```\n\n")
		}
	}
	return b.String()
}

func (a *AiTerminalAPI) loadShareContent(token string) (database.OpsSession, string, []database.OpsSessionMessage, []database.OpsStep, error) {
	var share database.OpsSessionShare
	if err := a.db.Where("token = ?", token).First(&share).Error; err != nil {
		return database.OpsSession{}, "", nil, nil, err
	}
	var session database.OpsSession
	if err := a.db.First(&session, share.SessionID).Error; err != nil {
		return database.OpsSession{}, "", nil, nil, err
	}
	hostName := ""
	var host database.TerminalHost
	if a.db.First(&host, session.HostID).Error == nil {
		hostName = host.Name
	}
	var messages []database.OpsSessionMessage
	a.db.Where("session_id = ?", session.ID).Order("seq asc").Find(&messages)
	var steps []database.OpsStep
	a.db.Where("session_id = ?", session.ID).Order("seq asc").Find(&steps)
	return session, hostName, messages, steps, nil
}

func (a *AiTerminalAPI) shareMarkdown(w http.ResponseWriter, r *http.Request, token string) {
	share, ok := a.verifyShare(r, token)
	if share == nil {
		writeError(w, 404, "分享不存在")
		return
	}
	if !ok {
		writeError(w, 401, "分享密码错误或缺失")
		return
	}
	session, hostName, messages, steps, err := a.loadShareContent(token)
	if err != nil {
		writeError(w, 404, "分享不存在")
		return
	}
	md := a.buildShareMarkdown(session, hostName, messages, steps)
	w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
	_, _ = w.Write([]byte(md))
}

func (a *AiTerminalAPI) shareHTML(w http.ResponseWriter, r *http.Request, token string) {
	share, ok := a.verifyShare(r, token)
	if share == nil {
		writeError(w, 404, "分享不存在")
		return
	}
	if !ok {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<!doctype html><html><head><meta charset="utf-8"><title>会话分享访问</title><style>body{font-family:Menlo,Consolas,monospace;background:#0b1020;color:#d7e2f0;padding:24px}input,button{padding:8px;border-radius:8px;border:1px solid #334155;background:#111827;color:#e2e8f0}button{cursor:pointer;margin-left:8px}</style></head><body><h2>会话分享（密码保护）</h2><form method="get"><input type="password" name="password" placeholder="请输入分享密码"/><button type="submit">查看</button></form></body></html>`))
		return
	}
	session, hostName, messages, steps, err := a.loadShareContent(token)
	if err != nil {
		writeError(w, 404, "分享不存在")
		return
	}
	md := a.buildShareMarkdown(session, hostName, messages, steps)
	escaped := html.EscapeString(md)
	htmlDoc := "<!doctype html><html><head><meta charset=\"utf-8\"><title>会话分享</title><style>body{margin:0;background:#0b1020;color:#d7e2f0;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,Helvetica,Arial,'PingFang SC','Microsoft YaHei',sans-serif;}main{max-width:960px;margin:0 auto;padding:24px;}pre{white-space:pre-wrap;background:#111827;border:1px solid #334155;border-radius:8px;padding:12px;line-height:1.6;font-family:Menlo,Consolas,monospace;}h1,h2,h3{color:#93c5fd;}</style></head><body><main><pre>" + escaped + "</pre></main></body></html>"
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(htmlDoc))
}

func renderShareTimeline(messages []database.OpsSessionMessage, steps []database.OpsStep) string {
	b := &strings.Builder{}
	appendText := func(v string) string {
		return strings.ReplaceAll(html.EscapeString(strings.TrimSpace(v)), "\n", "<br>")
	}
	riskClass := func(v string) string {
		switch strings.ToLower(strings.TrimSpace(v)) {
		case "high":
			return "tag high"
		case "medium":
			return "tag medium"
		default:
			return "tag low"
		}
	}
	statusClass := func(v string) string {
		switch strings.ToLower(strings.TrimSpace(v)) {
		case "success":
			return "tag ok"
		case "failed":
			return "tag bad"
		case "rejected":
			return "tag off"
		case "running":
			return "tag run"
		default:
			return "tag wait"
		}
	}

	if len(messages) > 0 {
		for _, m := range messages {
			role := strings.ToLower(strings.TrimSpace(m.Role))
			label := "系统"
			klass := "system"
			switch role {
			case "user":
				label, klass = "用户", "user"
			case "assistant":
				label, klass = "AI 分析", "assistant"
			case "command":
				label, klass = "命令", "command"
			case "result":
				label, klass = "执行结果", "result"
			}
			fmt.Fprintf(b, "<article class=\"event %s\"><header><span class=\"role\">%s</span>", klass, label)
			if m.CreatedAt.Unix() > 0 {
				fmt.Fprintf(b, "<span class=\"time\">%s</span>", m.CreatedAt.Format("2006-01-02 15:04:05"))
			}
			fmt.Fprintf(b, "</header><div class=\"body\">")
			if role == "command" {
				cmd := m.Command
				if strings.TrimSpace(cmd) == "" {
					cmd = m.Text
				}
				fmt.Fprintf(b, "<pre class=\"code\">$ %s</pre>", html.EscapeString(cmd))
				fmt.Fprintf(b, "<div class=\"chips\"><span class=\"%s\">%s</span><span class=\"%s\">%s</span></div>", riskClass(m.Risk), html.EscapeString(m.Risk), statusClass(m.Status), html.EscapeString(m.Status))
				if strings.TrimSpace(m.Note) != "" {
					fmt.Fprintf(b, "<div class=\"note\">%s</div>", appendText(m.Note))
				}
			} else if role == "result" {
				fmt.Fprintf(b, "<div class=\"chips\"><span class=\"%s\">exit %d</span></div>", statusClass(func() string {
					if m.ExitCode == 0 {
						return "success"
					}
					return "failed"
				}()), m.ExitCode)
				fmt.Fprintf(b, "<pre class=\"code\">%s</pre>", html.EscapeString(m.Text))
			} else {
				fmt.Fprintf(b, "<div class=\"text\">%s</div>", appendText(m.Text))
			}
			fmt.Fprintf(b, "</div></article>")
		}
		return b.String()
	}

	for _, st := range steps {
		fmt.Fprintf(b, "<article class=\"event command\"><header><span class=\"role\">命令</span></header><div class=\"body\">")
		fmt.Fprintf(b, "<pre class=\"code\">$ %s</pre>", html.EscapeString(st.Command))
		fmt.Fprintf(b, "<div class=\"chips\"><span class=\"%s\">%s</span><span class=\"%s\">%s</span></div>", riskClass(st.RiskLevel), html.EscapeString(st.RiskLevel), statusClass(st.Status), html.EscapeString(st.Status))
		if strings.TrimSpace(st.Purpose) != "" {
			fmt.Fprintf(b, "<div class=\"note\">%s</div>", appendText(st.Purpose))
		}
		fmt.Fprintf(b, "</div></article>")
		if strings.TrimSpace(st.Output) != "" || strings.TrimSpace(st.Error) != "" {
			out := st.Output
			if strings.TrimSpace(st.Error) != "" {
				out += "\n[error] " + st.Error
			}
			fmt.Fprintf(b, "<article class=\"event result\"><header><span class=\"role\">执行结果</span></header><div class=\"body\"><div class=\"chips\"><span class=\"%s\">exit %d</span></div><pre class=\"code\">%s</pre></div></article>", statusClass(func() string {
				if st.ExitCode == 0 {
					return "success"
				}
				return "failed"
			}()), st.ExitCode, html.EscapeString(out))
		}
	}
	return b.String()
}

func (a *AiTerminalAPI) handlePublicSharePage(w http.ResponseWriter, r *http.Request) {
	token := strings.TrimSpace(strings.TrimPrefix(r.URL.Path, "/promai/share/"))
	token = strings.Trim(token, "/")
	if token == "" {
		writeError(w, 404, "分享不存在")
		return
	}
	share, ok := a.verifyShare(r, token)
	if share == nil {
		writeError(w, 404, "分享不存在")
		return
	}
	if !ok {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<!doctype html><html><head><meta charset="utf-8"><title>会话分享访问</title><style>body{font-family:Menlo,Consolas,monospace;background:#0b1020;color:#d7e2f0;padding:24px}input,button{padding:8px;border-radius:8px;border:1px solid #334155;background:#111827;color:#e2e8f0}button{cursor:pointer;margin-left:8px}</style></head><body><h2>会话分享（密码保护）</h2><form method="get"><input type="password" name="password" placeholder="请输入分享密码"/><button type="submit">查看</button></form></body></html>`))
		return
	}
	session, hostName, messages, steps, err := a.loadShareContent(token)
	if err != nil {
		writeError(w, 404, "分享不存在")
		return
	}
	md := a.buildShareMarkdown(session, hostName, messages, steps)
	if f := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("format"))); f == "md" || f == "markdown" {
		w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
		_, _ = w.Write([]byte(md))
		return
	}
	timeline := renderShareTimeline(messages, steps)
	escaped := html.EscapeString(md)
	htmlDoc := "<!doctype html><html><head><meta charset=\"utf-8\"><title>会话分享</title><style>body{margin:0;background:#0b1020;color:#d7e2f0;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,Helvetica,Arial,'PingFang SC','Microsoft YaHei',sans-serif;}main{max-width:980px;margin:0 auto;padding:20px;}h1{font-size:20px;color:#93c5fd;margin:0 0 8px;} .meta{display:grid;grid-template-columns:repeat(auto-fit,minmax(180px,1fr));gap:8px;margin:10px 0 16px;} .meta div{background:#111827;border:1px solid #334155;border-radius:10px;padding:8px 10px;font-size:12px;color:#cbd5e1;} .tabs{display:flex;gap:8px;margin:12px 0 16px;} .tab{padding:6px 12px;border:1px solid #334155;border-radius:8px;background:#111827;color:#cbd5e1;cursor:pointer;} .tab.active{background:#1d4ed8;color:#fff;border-color:#2563eb;} .panel{display:none;} .panel.active{display:block;} .timeline{display:flex;flex-direction:column;gap:10px;} .event{border:1px solid #334155;border-radius:12px;background:#111827;} .event header{display:flex;justify-content:space-between;align-items:center;padding:10px 12px;border-bottom:1px solid #1f2937;} .role{font-weight:600;color:#93c5fd;} .time{font-size:12px;color:#94a3b8;} .body{padding:10px 12px;} .text{line-height:1.7;} .code{white-space:pre-wrap;background:#0f172a;border:1px solid #334155;border-radius:8px;padding:10px;font-family:Menlo,Consolas,monospace;font-size:12px;line-height:1.6;} .chips{display:flex;flex-wrap:wrap;gap:8px;margin:8px 0;} .tag{font-size:11px;padding:2px 8px;border-radius:999px;border:1px solid #334155;background:#0f172a;color:#cbd5e1;} .tag.high{border-color:#7f1d1d;color:#fecaca;background:#2b0f12;} .tag.medium{border-color:#78350f;color:#fde68a;background:#2b1a0b;} .tag.low{border-color:#14532d;color:#86efac;background:#072015;} .tag.ok{border-color:#14532d;color:#86efac;background:#072015;} .tag.bad{border-color:#7f1d1d;color:#fecaca;background:#2b0f12;} .tag.off{border-color:#475569;color:#cbd5e1;background:#0f172a;} .tag.run{border-color:#92400e;color:#fde68a;background:#2b1a0b;} .tag.wait{border-color:#334155;color:#cbd5e1;background:#0f172a;} .note{font-size:12px;color:#9ca3af;} pre.markdown{white-space:pre-wrap;background:#111827;border:1px solid #334155;border-radius:8px;padding:12px;line-height:1.6;font-family:Menlo,Consolas,monospace;} .hint{font-size:12px;color:#94a3b8;}</style></head><body><main><h1>自动化运维会话分享</h1><div class=\"hint\">同一链接支持 HTML/Markdown 视图切换；Markdown 可追加 format=md 直接获取文本。</div><section class=\"meta\"><div><strong>会话ID</strong><br>" + strconv.FormatUint(uint64(session.ID), 10) + "</div><div><strong>标题</strong><br>" + html.EscapeString(session.Title) + "</div><div><strong>主机</strong><br>" + html.EscapeString(hostName) + "</div><div><strong>发起人</strong><br>" + html.EscapeString(session.Username) + "</div></section><div class=\"tabs\"><button class=\"tab active\" data-tab=\"html\">时间线</button><button class=\"tab\" data-tab=\"md\">Markdown</button></div><section id=\"panel-html\" class=\"panel active\"><div class=\"timeline\">" + timeline + "</div></section><section id=\"panel-md\" class=\"panel\"><pre class=\"markdown\">" + escaped + "</pre></section></main><script>const tabs=document.querySelectorAll('.tab');const panels={html:document.getElementById('panel-html'),md:document.getElementById('panel-md')};tabs.forEach(t=>t.addEventListener('click',()=>{tabs.forEach(x=>x.classList.remove('active'));t.classList.add('active');Object.values(panels).forEach(p=>p.classList.remove('active'));panels[t.dataset.tab].classList.add('active');}));</script></body></html>"
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(htmlDoc))
}

// ---------------- 规划（LLM） ----------------

type plannedStep struct {
	Command string `json:"command"`
	Purpose string `json:"purpose"`
	Risk    string `json:"risk"`
}

// agentPlan 是 AI 的分析 + 命令计划。
type agentPlan struct {
	Analysis string        `json:"analysis"`
	Steps    []plannedStep `json:"steps"`
	Done     bool          `json:"done"`
}

func (a *AiTerminalAPI) plan(w http.ResponseWriter, r *http.Request, sid uint, includeOutput bool) {
	var req struct {
		Message string `json:"message"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, 400, "请求体格式错误")
		return
	}
	msg := strings.TrimSpace(req.Message)
	if msg == "" && !includeOutput {
		writeError(w, 400, "请输入意图")
		return
	}

	var session database.OpsSession
	if err := a.db.First(&session, sid).Error; err != nil {
		writeError(w, 404, "会话不存在")
		return
	}

	plan, err := a.llmPlan(session, msg, includeOutput)
	if err != nil {
		log.Printf("[AiTerminal] 会话 %d 规划失败: %v", sid, err)
		writeError(w, 502, err.Error())
		return
	}

	var maxSeq int
	a.db.Model(&database.OpsStep{}).Where("session_id = ?", sid).Select("COALESCE(MAX(seq),0)").Scan(&maxSeq)

	created := make([]database.OpsStep, 0, len(plan.Steps))
	for i, ps := range plan.Steps {
		st := database.OpsStep{
			SessionID: sid,
			Seq:       maxSeq + i + 1,
			Command:   ps.Command,
			Purpose:   ps.Purpose,
			RiskLevel: normalizeRisk(ps.Risk, ps.Command),
			Status:    stepPending,
		}
		if err := a.db.Create(&st).Error; err != nil {
			writeError(w, 500, err.Error())
			return
		}
		created = append(created, st)
	}
	writeJSON(w, map[string]interface{}{
		"analysis": plan.Analysis,
		"done":     plan.Done,
		"steps":    created,
	})
}

func (a *AiTerminalAPI) llmPlan(session database.OpsSession, message string, includeOutput bool) (agentPlan, error) {
	var plan agentPlan
	mc := proxy.ResolveAIModel(a.cfg)
	if mc == nil {
		return plan, errors.New("未配置 AI 模型，请到系统设置配置")
	}

	var host database.TerminalHost
	hostCtx := ""
	if a.db.First(&host, session.HostID).Error == nil {
		hostCtx = fmt.Sprintf("%s (%s:%d) 用户=%s env=%s host_key=%s",
			host.Name, host.Address, host.Port, host.Username, host.Env, host.HostKey)
	}

	history := ""
	if includeOutput {
		var steps []database.OpsStep
		a.db.Where("session_id = ?", session.ID).Order("seq asc").Find(&steps)
		for _, st := range steps {
			if history != "" {
				history += "\n"
			}
			history += fmt.Sprintf("命令[%s] 风险=%s 状态=%s\n输出: %s",
				st.Command, st.RiskLevel, st.Status, truncateText(st.Output, 800))
		}
	}

	system := a.settingValue(promptSystemKey, defaultTerminalSystemPrompt)

	user := "目标主机上下文：" + hostCtx + "\n"
	if history != "" {
		user += "\n已经执行过的步骤与真实输出：\n" + history + "\n"
	}
	if message != "" {
		user += "\n用户请求：" + message
	} else {
		user += "\n" + a.settingValue(promptContinueKey, defaultTerminalContinuePrompt)
	}

	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			log.Printf("[AiTerminal] AI 计划解析失败，第 %d 次重试", attempt)
			backoff := time.Duration(attempt) * time.Second
			time.Sleep(backoff)
		}

		content, err := proxy.CallOpenAICompat(mc, system, user, "")
		if err != nil {
			log.Printf("[AiTerminal] AI 调用失败 (尝试 %d/3): %v", attempt+1, err)
			lastErr = err
			continue
		}

		var parsed agentPlan
		if parsePlan(content, &parsed) == nil {
			return parsed, nil
		}
		// 记录解析失败的原始内容（截断）便于排查
		log.Printf("[AiTerminal] JSON 解析失败 (尝试 %d/3), 原始返回(截断): %s", attempt+1, truncateText(content, 500))
		lastErr = errors.New("AI 返回格式无法解析")
	}
	if lastErr != nil {
		return plan, lastErr
	}
	return plan, errors.New("AI 返回格式无法解析，请重试")
}

func parsePlan(content string, out *agentPlan) error {
	// 策略1: 去除 markdown 代码块标记后再尝试
	cleaned := stripMarkdownFences(content)
	if cleaned != content {
		if err := extractJSON(cleaned, out); err == nil {
			return nil
		}
	}

	// 策略2: 从原始内容中提取 JSON
	if err := extractJSON(content, out); err == nil {
		return nil
	}

	// 策略3: 从清洗后的内容提取 JSON
	if cleaned != content {
		if err := extractJSON(cleaned, out); err == nil {
			return nil
		}
	}

	return errors.New("无法从 AI 返回中提取有效的 JSON 计划")
}

// stripMarkdownFences 去除 markdown 代码块标记（```json ... ``` 或 ``` ... ```）。
func stripMarkdownFences(s string) string {
	// 尝试匹配 ```json ... ``` 或 ``` ... ```
	lower := strings.TrimSpace(strings.ToLower(s))
	for _, fence := range []string{"```json", "```"} {
		if strings.HasPrefix(lower, fence) {
			// 找到开始标记后的内容
			idx := strings.Index(s, "\n")
			if idx < 0 {
				idx = len(fence)
			}
			rest := strings.TrimSpace(s[idx+1:])
			// 去掉结尾的 ```
			if end := strings.LastIndex(rest, "```"); end >= 0 {
				rest = strings.TrimSpace(rest[:end])
			}
			return rest
		}
	}
	return s
}

// extractJSON 从文本中提取首个合法的 JSON 对象。
func extractJSON(content string, out *agentPlan) error {
	start := strings.Index(content, "{")
	end := strings.LastIndex(content, "}")
	if start < 0 || end <= start {
		return errors.New("无 JSON")
	}

	// 尝试从 start 到 end 逐渐收缩，找到合法的 JSON
	candidate := content[start : end+1]
	var obj agentPlan
	if err := json.Unmarshal([]byte(candidate), &obj); err == nil {
		if strings.TrimSpace(obj.Analysis) != "" || len(obj.Steps) > 0 {
			*out = obj
			return nil
		}
		return errors.New("空计划")
	}

	// 如果直接解析失败，尝试用 balancedBraces 找更精确的边界
	balanced := balancedBraces(content, start)
	if balanced > start && balanced > end {
		candidate = content[start : balanced+1]
		if err := json.Unmarshal([]byte(candidate), &obj); err == nil {
			if strings.TrimSpace(obj.Analysis) != "" || len(obj.Steps) > 0 {
				*out = obj
				return nil
			}
			return errors.New("空计划")
		}
	}

	return fmt.Errorf("JSON 解析失败")
}

// balancedBraces 从 start 位置开始，找到匹配的 } 位置。
func balancedBraces(s string, start int) int {
	depth := 0
	inString := false
	escaped := false
	for i := start; i < len(s); i++ {
		ch := s[i]
		if escaped {
			escaped = false
			continue
		}
		if ch == '\\' && inString {
			escaped = true
			continue
		}
		if ch == '"' {
			inString = !inString
			continue
		}
		if inString {
			continue
		}
		if ch == '{' {
			depth++
		} else if ch == '}' {
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

// ---------------- 审批/状态辅助 ----------------

func markApproved(db *gorm.DB, step *database.OpsStep, user, reason string, auto bool) {
	now := time.Now()
	_ = db.Model(&database.OpsStep{}).Where("id = ?", step.ID).Updates(map[string]interface{}{
		"status": stepApproved, "approved_by": user, "reject_reason": reason,
		"auto_approved": auto, "approved_at": now,
	})
	step.ApprovedBy = user
	step.AutoApproved = auto
	step.ApprovedAt = &now
}

func markFailed(db *gorm.DB, step *database.OpsStep, errMsg string) {
	_ = db.Model(&database.OpsStep{}).Where("id = ?", step.ID).Updates(map[string]interface{}{
		"status": stepFailed, "error": errMsg, "executed_at": time.Now(),
	})
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// ---------------- 风险分级 ----------------

var riskHighPrefixes = []string{"rm -rf", "mkfs", "kill -9", "dd ", ">/dev/s", "shutdown", "reboot", "fdisk", "drop table", "delete from", "drop database", "flush"}
var riskMediumPrefixes = []string{"mkdir", "touch", "echo", "systemctl", "service ", "mv ", "cp ", "chmod", "chown", "iptables", "kill "}

func riskLevelFor(command string) string {
	cmd := strings.TrimSpace(strings.ToLower(command))
	for _, p := range riskHighPrefixes {
		if strings.HasPrefix(cmd, p) {
			return "high"
		}
	}
	for _, p := range riskMediumPrefixes {
		if strings.HasPrefix(cmd, p) {
			return "medium"
		}
	}
	return "low"
}

func normalizeRisk(llmRisk, command string) string {
	heuristic := riskLevelFor(command)
	if heuristic == "high" {
		return "high" // 安全硬顶
	}
	switch llmRisk {
	case "medium", "high":
		return llmRisk
	}
	return heuristic
}
