package piagent

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/smtp"
	"net/url"
	"strings"
	"time"

	"PromAI/pkg/config"
	"PromAI/pkg/database"
	"PromAI/pkg/notify"
	"PromAI/pkg/report"

	"github.com/jay-y/pi/pkg/ai"
	agent "github.com/jay-y/pi/pkg/ai-agent"
	gomail "github.com/jordan-wright/email"
)

type PushReportTool struct {
	cfg *config.Config
	db  DB
}

func NewPushReportTool(cfg *config.Config, db DB) *PushReportTool {
	return &PushReportTool{cfg: cfg, db: db}
}

func (t *PushReportTool) GetName() string  { return "push_report" }
func (t *PushReportTool) GetLabel() string { return "推送报告" }
func (t *PushReportTool) GetDescription() string {
	return "将巡检报告或自定义内容推送到通知渠道（企业微信/钉钉/飞书/邮件）"
}

func (t *PushReportTool) GetParameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"report_id": map[string]any{
				"type":        "integer",
				"description": "报告 ID，不填则推送最近一份报告",
			},
			"channel": map[string]any{
				"type":        "string",
				"description": "通知渠道: wechat_work(企业微信), dingtalk(钉钉), feishu(飞书), email(邮件)",
				"enum":        []string{"wechat_work", "dingtalk", "feishu", "email"},
			},
			"webhook_url": map[string]any{
				"type":        "string",
				"description": "自定义机器人 webhook 地址，不填则使用系统配置的地址（支持 wechat_work/dingtalk/feishu）",
			},
			"content": map[string]any{
				"type":        "string",
				"description": "自定义推送内容，不填则推送报告摘要",
			},
		},
		"required": []string{"channel"},
	}
}

func (t *PushReportTool) Execute(ctx context.Context, params map[string]any, onUpdate func(*agent.AgentToolResult)) (*agent.AgentToolResult, error) {
	channel, _ := params["channel"].(string)
	reportID, _ := params["report_id"].(float64)
	customContent, _ := params["content"].(string)
	webhookURL, _ := params["webhook_url"].(string)

	log.Printf("[PiAgent] 工具调用: push_report channel=%s report_id=%.0f", channel, reportID)

	if customContent != "" {
		return t.pushCustomContent(ctx, channel, webhookURL, customContent)
	}

	var record ReportRecord
	if reportID > 0 {
		if t.db.First(&record, uint(reportID)).Error() != nil {
			return &agent.AgentToolResult{
				Content: []ai.ContentBlock{ai.NewTextContentBlock(fmt.Sprintf("报告 %d 不存在", uint(reportID)))},
			}, nil
		}
	} else {
		if t.db.Model(&ReportRecord{}).Order("created_at desc").First(&record).Error() != nil {
			return &agent.AgentToolResult{
				Content: []ai.ContentBlock{ai.NewTextContentBlock("暂无可用报告")},
			}, nil
		}
	}

	summary := notify.AlertSummary{
		TotalMetrics:   record.TotalMetrics,
		TotalAlerts:    record.AlertCount,
		CriticalAlerts: record.CriticalCount,
		WarningAlerts:  record.WarningCount,
		NormalMetrics:  record.TotalMetrics - record.AlertCount,
	}

	ctx = t.withReportDataFromSnapshot(ctx, record)

	projectName := t.cfg.ProjectName
	if projectName == "" {
		projectName = "PromAI"
	}

	var err error
	switch channel {
	case "wechat_work":
		cfg := t.loadWeChatWorkConfig()
		cfg.Enabled = true
		if webhookURL != "" {
			proxyURL := ""
			if strings.HasPrefix(webhookURL, "https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=") {
				botKey := strings.TrimPrefix(webhookURL, "https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=")
				err = notify.SendWeChatWorkWithWebhook(ctx, botKey, proxyURL, record.FilePath, projectName, record.DatasourceName, summary)
			} else {
				err = fmt.Errorf("企业微信 webhook 地址格式不正确，应以 https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key= 开头")
			}
		} else {
			err = notify.SendWeChatWorkWithContext(ctx, cfg, record.FilePath, projectName, record.DatasourceName, summary)
		}
	case "dingtalk":
		cfg := t.loadDingtalkConfig()
		cfg.Enabled = true
		if webhookURL != "" {
			cfg.Webhook = webhookURL
			cfg.Secret = ""
		}
		err = notify.SendDingtalkWithContext(ctx, cfg, record.FilePath, projectName, record.DatasourceName, summary)
	case "feishu":
		cfg := t.loadFeishuConfig()
		cfg.Enabled = true
		if webhookURL != "" {
			cfg.Webhook = webhookURL
			cfg.Secret = ""
			cfg.VerifySign = false
		}
		err = notify.SendFeishuWithContext(ctx, cfg, record.FilePath, projectName, record.DatasourceName, summary)
	case "email":
		cfg := t.loadEmailConfig()
		cfg.Enabled = true
		err = notify.SendEmailWithContext(ctx, cfg, record.FilePath, projectName, record.DatasourceName, summary)
	default:
		return &agent.AgentToolResult{
			Content: []ai.ContentBlock{ai.NewTextContentBlock(fmt.Sprintf("不支持的渠道: %s，可选: wechat_work, dingtalk, feishu, email", channel))},
		}, nil
	}

	if err != nil {
		log.Printf("[PiAgent] 推送报告失败: %v", err)
		return &agent.AgentToolResult{
			Content: []ai.ContentBlock{ai.NewTextContentBlock(fmt.Sprintf("推送失败: %v", err))},
		}, nil
	}

	log.Printf("[PiAgent] 推送报告成功: report_id=%d channel=%s", record.ID, channel)
	return &agent.AgentToolResult{
		Content: []ai.ContentBlock{ai.NewTextContentBlock(fmt.Sprintf("✅ 报告 #%d 已成功推送到 %s", record.ID, channel))},
	}, nil
}

func (t *PushReportTool) withReportDataFromSnapshot(ctx context.Context, record ReportRecord) context.Context {
	if record.MetricsJSON == "" {
		return ctx
	}
	var snapshot map[string]any
	if err := json.Unmarshal([]byte(record.MetricsJSON), &snapshot); err != nil {
		return ctx
	}
	tsMap := buildTypeSummaryMap(snapshot)
	if len(tsMap) == 0 {
		return ctx
	}
	metricGroups := make(map[string]*report.MetricGroup)
	for typeName, tsAny := range tsMap {
		ts, _ := tsAny.(map[string]any)
		var metrics []report.MetricData
		for i := 0; i < int(getFloat(ts, "critical")); i++ {
			metrics = append(metrics, report.MetricData{Status: "critical"})
		}
		for i := 0; i < int(getFloat(ts, "warning")); i++ {
			metrics = append(metrics, report.MetricData{Status: "warning"})
		}
		for i := 0; i < int(getFloat(ts, "normal")); i++ {
			metrics = append(metrics, report.MetricData{Status: "normal"})
		}
		if len(metrics) > 0 {
			metricGroups[typeName] = &report.MetricGroup{
				MetricsByName: map[string][]report.MetricData{"_summary": metrics},
			}
		}
	}
	if len(metricGroups) > 0 {
		ctx = context.WithValue(ctx, "report_data", report.ReportData{MetricGroups: metricGroups})
	}
	return ctx
}

func buildTypeSummaryMap(snapshot map[string]any) map[string]any {
	if tsRaw, ok := snapshot["type_summaries"]; ok {
		if tsMap, ok := tsRaw.(map[string]any); ok && len(tsMap) > 0 {
			return tsMap
		}
		if tsList, ok := tsRaw.([]any); ok && len(tsList) > 0 {
			converted := make(map[string]any, len(tsList))
			for _, item := range tsList {
				row, ok := item.(map[string]any)
				if !ok {
					continue
				}
				typeName, _ := row["type_name"].(string)
				if strings.TrimSpace(typeName) == "" {
					typeName, _ = row["type"].(string)
				}
				if strings.TrimSpace(typeName) == "" {
					continue
				}
				converted[typeName] = map[string]any{
					"critical": row["critical_count"],
					"warning":  row["warning_count"],
					"normal":   row["normal_count"],
				}
			}
			if len(converted) > 0 {
				return converted
			}
		}
	}

	metricsRaw, ok := snapshot["metrics"]
	if !ok {
		return nil
	}
	metricsList, ok := metricsRaw.([]any)
	if !ok || len(metricsList) == 0 {
		return nil
	}

	converted := make(map[string]map[string]float64)
	for _, item := range metricsList {
		row, ok := item.(map[string]any)
		if !ok {
			continue
		}
		typeName, _ := row["type_name"].(string)
		if strings.TrimSpace(typeName) == "" {
			continue
		}
		if _, exists := converted[typeName]; !exists {
			converted[typeName] = map[string]float64{
				"critical": 0,
				"warning":  0,
				"normal":   0,
			}
		}
		status, _ := row["status"].(string)
		switch status {
		case "critical":
			converted[typeName]["critical"]++
		case "warning":
			converted[typeName]["warning"]++
		default:
			converted[typeName]["normal"]++
		}
	}

	if len(converted) == 0 {
		return nil
	}

	result := make(map[string]any, len(converted))
	for typeName, counts := range converted {
		result[typeName] = map[string]any{
			"critical": counts["critical"],
			"warning":  counts["warning"],
			"normal":   counts["normal"],
		}
	}
	return result
}

func getFloat(m map[string]any, key string) float64 {
	if v, ok := m[key]; ok {
		if f, ok := v.(float64); ok {
			return f
		}
	}
	return 0
}

func (t *PushReportTool) loadNotificationChannel(channelType string) *database.NotificationChannel {
	var channel database.NotificationChannel
	if t.db.Model(&database.NotificationChannel{}).Where("channel_type = ?", channelType).First(&channel).Error() != nil {
		return nil
	}
	return &channel
}

func (t *PushReportTool) loadDingtalkConfig() notify.DingtalkConfig {
	cfg := t.cfg.Notifications.Dingtalk
	channel := t.loadNotificationChannel("dingtalk")
	if channel == nil || channel.ConfigJSON == "" {
		return cfg
	}
	var dbCfg notify.DingtalkConfig
	if err := json.Unmarshal([]byte(channel.ConfigJSON), &dbCfg); err != nil {
		log.Printf("[PiAgent] 解析钉钉通知配置失败: %v", err)
		return cfg
	}
	dbCfg.Enabled = channel.Enabled
	return dbCfg
}

func (t *PushReportTool) loadWeChatWorkConfig() notify.WeChatWorkConfig {
	cfg := t.cfg.Notifications.WeChatWork
	channel := t.loadNotificationChannel("wechat_work")
	if channel == nil || channel.ConfigJSON == "" {
		return cfg
	}
	var dbCfg notify.WeChatWorkConfig
	if err := json.Unmarshal([]byte(channel.ConfigJSON), &dbCfg); err != nil {
		log.Printf("[PiAgent] 解析企业微信通知配置失败: %v", err)
		return cfg
	}
	dbCfg.Enabled = channel.Enabled
	return dbCfg
}

func (t *PushReportTool) loadFeishuConfig() notify.FeishuConfig {
	cfg := t.cfg.Notifications.Feishu
	channel := t.loadNotificationChannel("feishu")
	if channel == nil || channel.ConfigJSON == "" {
		return cfg
	}
	var dbCfg notify.FeishuConfig
	if err := json.Unmarshal([]byte(channel.ConfigJSON), &dbCfg); err != nil {
		log.Printf("[PiAgent] 解析飞书通知配置失败: %v", err)
		return cfg
	}
	dbCfg.Enabled = channel.Enabled
	return dbCfg
}

func (t *PushReportTool) loadEmailConfig() notify.EmailConfig {
	cfg := t.cfg.Notifications.Email
	channel := t.loadNotificationChannel("email")
	if channel == nil || channel.ConfigJSON == "" {
		return cfg
	}
	var dbCfg notify.EmailConfig
	if err := json.Unmarshal([]byte(channel.ConfigJSON), &dbCfg); err != nil {
		log.Printf("[PiAgent] 解析邮件通知配置失败: %v", err)
		return cfg
	}
	dbCfg.Enabled = channel.Enabled
	return dbCfg
}

func (t *PushReportTool) pushCustomContent(ctx context.Context, channel, webhookURL, content string) (*agent.AgentToolResult, error) {
	projectName := t.cfg.ProjectName
	if projectName == "" {
		projectName = "PromAI"
	}
	switch channel {
	case "wechat_work":
		return t.pushWeChatWorkCustom(projectName, webhookURL, content)
	case "dingtalk":
		return t.pushDingtalkCustom(projectName, webhookURL, content)
	case "feishu":
		return t.pushFeishuCustom(projectName, webhookURL, content)
	case "email":
		return t.pushEmailCustom(projectName, content)
	default:
		return &agent.AgentToolResult{
			Content: []ai.ContentBlock{ai.NewTextContentBlock(fmt.Sprintf("不支持的渠道: %s", channel))},
		}, nil
	}
}

func (t *PushReportTool) pushWeChatWorkCustom(projectName, webhookURL, content string) (*agent.AgentToolResult, error) {
	webhook := webhookURL
	if webhook == "" {
		webhook = t.loadWeChatWorkConfig().Webhook
	}
	if webhook == "" {
		return &agent.AgentToolResult{Content: []ai.ContentBlock{ai.NewTextContentBlock("系统未配置企业微信 webhook")}}, nil
	}
	msg := map[string]any{
		"msgtype": "markdown",
		"markdown": map[string]any{
			"content": fmt.Sprintf("【%s】\n%s", projectName, content),
		},
	}
	return sendWebhookPOST(webhook, msg)
}

func (t *PushReportTool) pushDingtalkCustom(projectName, webhookURL, content string) (*agent.AgentToolResult, error) {
	cfg := t.loadDingtalkConfig()
	webhook := webhookURL
	if webhook == "" {
		webhook = cfg.Webhook
	}
	if webhook == "" {
		return &agent.AgentToolResult{Content: []ai.ContentBlock{ai.NewTextContentBlock("未配置钉钉 webhook，请提供 webhook_url 参数或在系统配置中设置")}}, nil
	}
	if webhookURL == "" && cfg.Secret != "" {
		timestamp := time.Now().UnixMilli()
		sign := calcDingtalkSign(timestamp, cfg.Secret)
		webhook = fmt.Sprintf("%s&timestamp=%d&sign=%s", webhook, timestamp, sign)
	}
	msg := map[string]any{
		"msgtype": "markdown",
		"markdown": map[string]string{
			"title": projectName,
			"text":  fmt.Sprintf("## %s\n\n%s", projectName, content),
		},
	}
	return sendWebhookPOST(webhook, msg)
}

func (t *PushReportTool) pushFeishuCustom(projectName, webhookURL, content string) (*agent.AgentToolResult, error) {
	cfg := t.loadFeishuConfig()
	webhook := webhookURL
	if webhook == "" {
		webhook = cfg.Webhook
	}
	if webhook == "" {
		return &agent.AgentToolResult{Content: []ai.ContentBlock{ai.NewTextContentBlock("未配置飞书 webhook，请提供 webhook_url 参数或在系统配置中设置")}}, nil
	}
	if webhookURL == "" && cfg.VerifySign && cfg.Secret != "" {
		timestamp := time.Now().UnixMilli()
		sign := calcDingtalkSign(timestamp, cfg.Secret)
		webhook = fmt.Sprintf("%s&timestamp=%d&sign=%s", webhook, timestamp, sign)
	}
	msg := map[string]any{
		"msg_type": "post",
		"content": map[string]any{
			"post": map[string]any{
				"zh_cn": map[string]any{
					"title": projectName,
					"content": []any{
						[]any{
							map[string]any{"tag": "text", "text": content},
						},
					},
				},
			},
		},
	}
	return sendWebhookPOST(webhook, msg)
}

func (t *PushReportTool) pushEmailCustom(projectName, content string) (*agent.AgentToolResult, error) {
	cfg := t.cfg.Notifications.Email
	if !cfg.Enabled {
		return &agent.AgentToolResult{Content: []ai.ContentBlock{ai.NewTextContentBlock("邮件通知未启用")}}, nil
	}
	e := gomail.NewEmail()
	e.From = cfg.From
	e.To = cfg.To
	e.Subject = projectName
	e.Text = []byte(content)
	addr := fmt.Sprintf("%s:%d", cfg.SMTPHost, cfg.SMTPPort)
	auth := smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.SMTPHost)
	tlsCfg := &tls.Config{InsecureSkipVerify: true, ServerName: cfg.SMTPHost}
	if err := e.SendWithTLS(addr, auth, tlsCfg); err != nil {
		return &agent.AgentToolResult{Content: []ai.ContentBlock{ai.NewTextContentBlock(fmt.Sprintf("邮件推送失败: %v", err))}}, nil
	}
	return &agent.AgentToolResult{Content: []ai.ContentBlock{ai.NewTextContentBlock("✅ 自定义内容已成功推送到邮件")}}, nil
}

func sendWebhookPOST(webhook string, msg map[string]any) (*agent.AgentToolResult, error) {
	jsonData, err := json.Marshal(msg)
	if err != nil {
		return &agent.AgentToolResult{Content: []ai.ContentBlock{ai.NewTextContentBlock(fmt.Sprintf("JSON编码失败: %v", err))}}, nil
	}
	req, err := http.NewRequest(http.MethodPost, webhook, bytes.NewBuffer(jsonData))
	if err != nil {
		return &agent.AgentToolResult{Content: []ai.ContentBlock{ai.NewTextContentBlock(fmt.Sprintf("创建请求失败: %v", err))}}, nil
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return &agent.AgentToolResult{Content: []ai.ContentBlock{ai.NewTextContentBlock(fmt.Sprintf("发送失败: %v", err))}}, nil
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if err := notify.ValidateWebhookResponse(webhook, resp.StatusCode, body); err != nil {
		return &agent.AgentToolResult{Content: []ai.ContentBlock{ai.NewTextContentBlock(fmt.Sprintf("推送失败: %v", err))}}, nil
	}
	return &agent.AgentToolResult{Content: []ai.ContentBlock{ai.NewTextContentBlock("✅ 自定义内容推送成功")}}, nil
}

func calcDingtalkSign(timestamp int64, secret string) string {
	stringToSign := fmt.Sprintf("%d\n%s", timestamp, secret)
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(stringToSign))
	return url.QueryEscape(base64.StdEncoding.EncodeToString(h.Sum(nil)))
}
