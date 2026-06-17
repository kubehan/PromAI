package notify

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
	"path/filepath"
	"strings"
	"sync"
	"time"

	"PromAI/pkg/report"
	"PromAI/pkg/utils"

	"github.com/jordan-wright/email"
)

type DingtalkConfig struct {
	Enabled   bool   `yaml:"enabled" json:"enabled"`
	Webhook   string `yaml:"webhook" json:"webhook"`
	Secret    string `yaml:"secret" json:"secret"`
	ReportURL string `yaml:"report_url" json:"report_url"`
}

type EmailConfig struct {
	Enabled   bool     `yaml:"enabled" json:"enabled"`
	SMTPHost  string   `yaml:"smtp_host" json:"smtp_host"`
	SMTPPort  int      `yaml:"smtp_port" json:"smtp_port"`
	Username  string   `yaml:"username" json:"username"`
	Password  string   `yaml:"password" json:"password"`
	From      string   `yaml:"from" json:"from"`
	To        []string `yaml:"to" json:"to"`
	ReportURL string   `yaml:"report_url" json:"report_url"`
}

type WeChatWorkConfig struct {
	Enabled   bool   `yaml:"enabled" json:"enabled"`
	Webhook   string `yaml:"webhook" json:"webhook"`
	ProxyURL  string `yaml:"proxy_url" json:"proxy_url"`
	ReportURL string `yaml:"report_url" json:"report_url"`
}

type WeChatAppConfig struct {
	Enabled   bool   `yaml:"enabled" json:"enabled"`
	CorpID    string `yaml:"corpid" json:"corpid"`
	AgentID   int    `yaml:"agentid" json:"agentid"`
	Secret    string `yaml:"secret" json:"secret"`
	ToUser    string `yaml:"touser" json:"touser"` // 接收人企业微信ID，多个用|分隔，如"user1|user2"，默认"@all"发送给所有人
	ProxyURL  string `yaml:"proxy_url" json:"proxy_url"`
	ReportURL string `yaml:"report_url" json:"report_url"` // 新增 ReportURL 字段
}

type FeishuConfig struct {
	Enabled    bool   `yaml:"enabled" json:"enabled"`
	Webhook    string `yaml:"webhook" json:"webhook"`
	Secret     string `yaml:"secret" json:"secret"`
	ReportURL  string `yaml:"report_url" json:"report_url"`
	Timeout    int    `yaml:"timeout" json:"timeout"`
	VerifySign bool   `yaml:"verify_sign" json:"verify_sign"`
}

type AlertSummary struct {
	TotalAlerts    int
	CriticalAlerts int
	WarningAlerts  int
	NormalMetrics  int
	TotalMetrics   int
}

type TypeAlertSummary struct {
	Type          string
	TotalMetrics  int
	CriticalCount int
	WarningCount  int
	NormalCount   int
}

// calculateAlertSummary 从报告数据中计算告警汇总
func CalculateAlertSummary(data report.ReportData) AlertSummary {
	summary := AlertSummary{}

	for _, group := range data.MetricGroups {
		for _, metrics := range group.MetricsByName {
			for _, metric := range metrics {
				summary.TotalMetrics++

				switch metric.Status {
				case "critical":
					summary.CriticalAlerts++
					summary.TotalAlerts++
				case "warning":
					summary.WarningAlerts++
					summary.TotalAlerts++
				default:
					summary.NormalMetrics++
				}
			}
		}
	}

	return summary
}

// CalculateTypeAlertSummary 按照metric_types.type分类计算告警汇总
func CalculateTypeAlertSummary(data report.ReportData) []TypeAlertSummary {
	typeSummaries := make(map[string]*TypeAlertSummary)

	for typeName, group := range data.MetricGroups {
		summary := &TypeAlertSummary{
			Type: typeName,
		}

		for _, metrics := range group.MetricsByName {
			for _, metric := range metrics {
				summary.TotalMetrics++

				switch metric.Status {
				case "critical":
					summary.CriticalCount++
				case "warning":
					summary.WarningCount++
				default:
					summary.NormalCount++
				}
			}
		}

		typeSummaries[typeName] = summary
	}

	// 转换为切片并按照类型名称排序
	result := make([]TypeAlertSummary, 0, len(typeSummaries))
	for _, summary := range typeSummaries {
		result = append(result, *summary)
	}

	// 按照类型名称排序
	for i := 0; i < len(result)-1; i++ {
		for j := i + 1; j < len(result); j++ {
			if result[i].Type > result[j].Type {
				result[i], result[j] = result[j], result[i]
			}
		}
	}

	return result
}

func SendFeishu(config FeishuConfig, reportPath string, projectName string, Datasource string, alertSummary AlertSummary) error {
	return SendFeishuWithContext(context.Background(), config, reportPath, projectName, Datasource, alertSummary)
}

func SendFeishuWithContext(ctx context.Context, config FeishuConfig, reportPath string, projectName string, Datasource string, alertSummary AlertSummary) error {
	if !config.Enabled {
		log.Printf("飞书通知未启用")
		return nil
	}
	log.Printf("开始发送飞书通知...")

	var typeSummaries []TypeAlertSummary
	if data, ok := ctx.Value("report_data").(report.ReportData); ok {
		typeSummaries = CalculateTypeAlertSummary(data)
		log.Printf("从报告数据中计算出分类汇总，共%d个分类", len(typeSummaries))
	} else {
		log.Printf("未找到报告数据，使用空分类汇总")
		typeSummaries = []TypeAlertSummary{}
	}
	// 生成报告链接
	reportFileName := filepath.Base(reportPath)
	var reportLink string
	if r, ok := ctx.Value("http_request").(*http.Request); ok {
		reportLink = utils.GetReportURL(r, reportFileName)
		log.Printf("使用动态URL生成报告链接: %s", reportLink)
	} else {
		reportLink = fmt.Sprintf("%s/api/promai/reports/%s", config.ReportURL, reportFileName)
		log.Printf("使用配置的静态URL生成报告链接: %s", reportLink)
	}

	// 告警状态
	alertStatus := "✅ 正常"
	if alertSummary.TotalAlerts > 0 {
		alertStatus = "⚠️ 异常"
	}

	// 构建文本内容
	typeSummaryText := ""
	if len(typeSummaries) > 0 {
		for _, s := range typeSummaries {
		status := "✅"
		if s.TotalMetrics == 0 {
			status = "⚪"
		} else if s.CriticalCount > 0 {
			status = "❌"
		} else if s.WarningCount > 0 {
			status = "⚠️"
		}
		typeSummaryText += fmt.Sprintf("%s%s：总%d个，异常%d个（严重%d，警告%d），正常%d个\n",
			status, s.Type, s.TotalMetrics, s.CriticalCount+s.WarningCount, s.CriticalCount, s.WarningCount, s.NormalCount)
		}
	} else {
		typeSummaryText = "暂无分类数据\n"
	}

	// 组装完整文本
	text := fmt.Sprintf("【巡检报告】%s\n项目：%s\n数据源：%s\n时间：%s\n\n分类巡检结果：\n%s\n总体统计：总%d个，异常%d个（严重%d，警告%d），正常%d个\n\n报告文件：%s\n报告链接：%s\n",
		alertStatus,
		projectName,
		Datasource,
		time.Now().Format("2006-01-02 15:04:05"),
		typeSummaryText,
		alertSummary.TotalMetrics,
		alertSummary.TotalAlerts,
		alertSummary.CriticalAlerts,
		alertSummary.WarningAlerts,
		alertSummary.NormalMetrics,
		reportFileName,
		reportLink,
	)

	// 组装消息体（Feishu 富文本消息）
	messageContent := map[string]interface{}{
		"msg_type": "post",
		"content": map[string]interface{}{
			"post": map[string]interface{}{
				"zh_cn": map[string]interface{}{
					"title": "巡检报告",
					"content": []interface{}{
						[]interface{}{
							map[string]interface{}{
								"tag":  "text",
								"text": text,
							},
						},
					},
				},
			},
		},
	}

	jsonData, err := json.Marshal(messageContent)
	if err != nil {
		log.Printf("JSON编码失败: %v", err)
		return fmt.Errorf("JSON编码失败: %v", err)
	}

	// 处理签名：如果启用了签名验证并且提供了 secret，则按飞书签名规则追加 timestamp/sign
	webhook := config.Webhook
	if config.VerifySign {
		if config.Secret == "" {
			log.Printf("启用签名校验，但未配置 secret，继续使用原始 webhook 发送")
		} else {
			timestamp := time.Now().UnixMilli()
			sign := calculateDingtalkSign(timestamp, config.Secret)
			// 飞书 webhook 在末尾追加 timestamp & sign（与钉钉类似）
			webhook = fmt.Sprintf("%s&timestamp=%d&sign=%s", config.Webhook, timestamp, sign)
		}
	}

	// HTTP 客户端，支持超时配置
	client := &http.Client{}
	if config.Timeout > 0 {
		client.Timeout = time.Duration(config.Timeout) * time.Second
	} else {
		client.Timeout = 5 * time.Second
	}

	req, err := http.NewRequestWithContext(ctx, "POST", webhook, bytes.NewBuffer(jsonData))
	if err != nil {
		log.Printf("创建请求失败: %v", err)
		return fmt.Errorf("创建请求失败: %v", err)
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")

	log.Printf("准备发送请求到 webhook: %s", webhook)
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("发送请求失败: %v", err)
		return fmt.Errorf("发送请求失败: %v", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	log.Printf("飞书响应状态码: %d, 响应内容: %s", resp.StatusCode, string(respBody))

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("飞书发送失败，状态码: %d", resp.StatusCode)
	}

	log.Printf("飞书通知发送成功")
	return nil
}

// config/config.yaml 中 dingtalk 配置
// notifications:
//   dingtalk:
//     enabled: true
//     webhook: "https://oapi.dingtalk.com/robot/send?access_token=29f727c8c973e5fb8d8339968d059393a4b4bb0bdcd667d592996035a8c0e135"
//     secret: "SEC75fd20834b42064b86c1aa97930738befeb2fe214044649397752212c5894848"

// SendDingtalk 发送钉钉通知（兼容版本）
func SendDingtalk(config DingtalkConfig, reportPath string, projectName string, Datasource string, alertSummary AlertSummary) error {
	return SendDingtalkWithContext(context.Background(), config, reportPath, projectName, Datasource, alertSummary)
}

// SendDingtalkWithContext 发送钉钉通知（支持动态URL）
// SendDingtalkWithContext 发送钉钉通知（适配钉钉Markdown换行规则）
func SendDingtalkWithContext(ctx context.Context, config DingtalkConfig, reportPath string, projectName string, Datasource string, alertSummary AlertSummary) error {
	if !config.Enabled {
		log.Printf("钉钉通知未启用")
		return nil
	}
	log.Printf("开始发送钉钉通知...")

	// 计算时间戳和签名
	timestamp := time.Now().UnixMilli()
	sign := calculateDingtalkSign(timestamp, config.Secret)
	webhook := fmt.Sprintf("%s&timestamp=%d&sign=%s", config.Webhook, timestamp, sign)

	log.Printf("准备发送请求到 webhook: %s", webhook)

	// 获取分类巡检汇总数据
	var typeSummaries []TypeAlertSummary
	if data, ok := ctx.Value("report_data").(report.ReportData); ok {
		typeSummaries = CalculateTypeAlertSummary(data)
		log.Printf("从报告数据中计算出分类汇总，共%d个分类", len(typeSummaries))
	} else {
		log.Printf("未找到报告数据，使用空分类汇总")
		typeSummaries = []TypeAlertSummary{}
	}

	// 构建分类巡检结果文本（用钉钉支持的<br/>换行，每个条目单独一行）
	typeSummaryText := ""
	if len(typeSummaries) > 0 {
		for _, summary := range typeSummaries {
			var typeStatus string
			if summary.TotalMetrics == 0 {
				typeStatus = "⚪" // 无指标
			} else if summary.CriticalCount > 0 {
				typeStatus = "❌" // 严重异常
			} else if summary.WarningCount > 0 {
				typeStatus = "⚠️" // 警告
			} else {
				typeStatus = "✅" // 正常
			}
			// 关键：用<br/>替代\n，适配钉钉Markdown换行
			typeSummaryText += fmt.Sprintf("%s%s：总%d个，异常%d个（严重%d，警告%d），正常%d个<br/>",
				typeStatus, summary.Type, summary.TotalMetrics,
				summary.CriticalCount+summary.WarningCount, summary.CriticalCount, summary.WarningCount, summary.NormalCount)
		}
	} else {
		typeSummaryText = "暂无分类数据<br/>"
	}

	// 生成报告的访问链接
	reportFileName := filepath.Base(reportPath)
	var reportLink string
	if r, ok := ctx.Value("http_request").(*http.Request); ok {
		reportLink = utils.GetReportURL(r, reportFileName)
		log.Printf("使用动态URL生成报告链接: %s", reportLink)
	} else {
		reportLink = fmt.Sprintf("%s/api/promai/reports/%s", config.ReportURL, reportFileName)
		log.Printf("使用配置的静态URL生成报告链接: %s", reportLink)
	}

	// 告警状态
	alertStatus := "✅ 正常"
	if alertSummary.TotalAlerts > 0 {
		alertStatus = "⚠️ 异常"
	}

	// 构建钉钉专属Markdown模板（移除>缩进，用<br/>换行）
	messageContent := map[string]interface{}{
		"msgtype": "markdown",
		"markdown": map[string]string{
			"title": "巡检报告",
			"text": fmt.Sprintf("## 🔍 巡检报告 %s\n\n"+
				"### ⌚ 巡检时间\n"+
				"%s\n\n"+
				"### 📊 分类巡检结果\n"+
				"%s\n\n"+
				"### 📈 整体统计\n"+
				"**总指标数**：%d个<br/>"+
				"**异常指标**：%d个（严重%d个，警告%d个）<br/>"+
				"**正常指标**：%d个\n\n"+
				"### 📋 点击查看完整报告\n"+
				"**文件名**：`%s`<br/>"+
				"**访问链接**：[点击查看报告](%s)\n\n"+
				"---\n"+
				"💡 请登录环境查看完整报告内容<br/>"+
				"⏰ 生成时间：%s",
				alertStatus,
				time.Now().Format("2006-01-02 15:04:05"),
				typeSummaryText, // 带<br/>的分类文本
				alertSummary.TotalMetrics,
				alertSummary.TotalAlerts,
				alertSummary.CriticalAlerts,
				alertSummary.WarningAlerts,
				alertSummary.NormalMetrics,
				reportFileName,
				reportLink,
				time.Now().Format("2006-01-02 15:04:05")),
		},
	}

	// 发送请求
	jsonData, err := json.Marshal(messageContent)
	if err != nil {
		log.Printf("JSON编码失败: %v", err)
		return fmt.Errorf("JSON编码失败: %v", err)
	}

	req, err := http.NewRequest("POST", webhook, bytes.NewBuffer(jsonData))
	if err != nil {
		log.Printf("创建请求失败: %v", err)
		return fmt.Errorf("创建请求失败: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("发送请求失败: %v", err)
		return fmt.Errorf("发送请求失败: %v", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	log.Printf("钉钉响应状态码: %d, 响应内容: %s", resp.StatusCode, string(respBody))

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("钉钉发送失败，状态码: %d", resp.StatusCode)
	}

	log.Printf("钉钉通知发送成功")
	return nil
}

// SendEmail 发送邮件通知（兼容版本）
func SendEmail(config EmailConfig, reportPath string, projectName string, Datasource string, alertSummary AlertSummary) error {
	return SendEmailWithContext(context.Background(), config, reportPath, projectName, Datasource, alertSummary)
}

// SendEmailWithContext 发送邮件通知（支持动态URL）
func SendEmailWithContext(ctx context.Context, config EmailConfig, reportPath string, projectName string, Datasource string, alertSummary AlertSummary) error {
	if !config.Enabled {
		log.Printf("邮件通知未启用")
		return nil
	}

	log.Printf("开始发送邮件通知...")
	log.Printf("SMTP服务器: %s:%d", config.SMTPHost, config.SMTPPort)
	log.Printf("发件人: %s", config.From)
	log.Printf("收件人: %v", config.To)

	e := email.NewEmail()
	e.From = config.From
	e.To = config.To
	e.Subject = "巡检报告"

	// 生成报告的访问链接
	reportFileName := filepath.Base(reportPath)

	// 尝试从context中获取HTTP请求对象，用于动态URL生成
	var reportLink string
	if r, ok := ctx.Value("http_request").(*http.Request); ok {
		// 打印调试信息
		log.Printf("调试信息: r.Host = %s", r.Host)
		log.Printf("调试信息: X-Forwarded-Host = %s", r.Header.Get("X-Forwarded-Host"))
		log.Printf("调试信息: X-Forwarded-Proto = %s", r.Header.Get("X-Forwarded-Proto"))
		log.Printf("调试信息: TLS = %v", r.TLS != nil)

		// 使用动态URL生成
		reportLink = utils.GetReportURL(r, reportFileName)
		log.Printf("使用动态URL生成报告链接: %s", reportLink)
		log.Printf("最终生成的 reportLink = %s", reportLink)
	} else {
		// 回退到配置的静态URL
		reportLink = fmt.Sprintf("%s/api/promai/reports/%s", config.ReportURL, reportFileName)
		log.Printf("使用配置的静态URL生成报告链接: %s", reportLink)
		log.Printf("最终生成的 reportLink = %s", reportLink)
	}

	// 尝试从context中获取报告数据，用于分类汇总
	var typeSummaries []TypeAlertSummary
	if data, ok := ctx.Value("report_data").(report.ReportData); ok {
		typeSummaries = CalculateTypeAlertSummary(data)
		log.Printf("从报告数据中计算出分类汇总")
	} else {
		log.Printf("未找到报告数据，使用空分类汇总")
		typeSummaries = []TypeAlertSummary{}
	}

	// 构建分类汇总HTML
	typeSummaryHTML := ""
	for _, summary := range typeSummaries {
		typeStatus := "✅"
		typeColor := "#28a745"
		if summary.TotalMetrics == 0 {
			typeStatus = "⚪"
			typeColor = "#6c757d"
		} else if summary.CriticalCount > 0 {
			typeStatus = "❌"
			typeColor = "#dc3545"
		} else if summary.WarningCount > 0 {
			typeStatus = "⚠️"
			typeColor = "#ffc107"
		}
		typeSummaryHTML += fmt.Sprintf(`
                <tr>
                    <td style="padding: 8px; border-bottom: 1px solid #dee2e6;"><span style="color: %s;">%s</span> <strong>%s</strong></td>
                    <td style="padding: 8px; border-bottom: 1px solid #dee2e6; text-align: center;">%d</td>
                    <td style="padding: 8px; border-bottom: 1px solid #dee2e6; text-align: center; color: #dc3545;">%d</td>
                    <td style="padding: 8px; border-bottom: 1px solid #dee2e6; text-align: center; color: #dc3545;">%d</td>
                    <td style="padding: 8px; border-bottom: 1px solid #dee2e6; text-align: center; color: #ffc107;">%d</td>
                    <td style="padding: 8px; border-bottom: 1px solid #dee2e6; text-align: center; color: #28a745;">%d</td>
                </tr>`,
			typeColor, typeStatus, summary.Type,
			summary.TotalMetrics,
			summary.CriticalCount+summary.WarningCount,
			summary.CriticalCount,
			summary.WarningCount,
			summary.NormalCount)
	}

	// 添加更丰富的邮件内容
	alertStatus := "✅ 正常"
	statusColor := "#28a745"
	if alertSummary.TotalAlerts > 0 {
		alertStatus = "⚠️ 异常"
		statusColor = "#ffc107"
	}
	if alertSummary.CriticalAlerts > 0 {
		statusColor = "#dc3545"
	}

	e.HTML = []byte(fmt.Sprintf(`
        <h2 style="color: %s;">🔍 %s 巡检报告已生成 %s</h2>
        
        <div style="background-color: #f8f9fa; padding: 15px; border-radius: 5px; margin: 15px 0;">
            <h3 style="color: #495057; margin-top: 0;">📊 分类巡检结果</h3>
            <table style="border-collapse: collapse; width: 100%%;">
                <thead>
                    <tr style="background-color: #e9ecef;">
                        <th style="padding: 8px; border-bottom: 2px solid #dee2e6; text-align: left;">分类</th>
                        <th style="padding: 8px; border-bottom: 2px solid #dee2e6; text-align: center;">总数</th>
                        <th style="padding: 8px; border-bottom: 2px solid #dee2e6; text-align: center;">异常</th>
                        <th style="padding: 8px; border-bottom: 2px solid #dee2e6; text-align: center; color: #dc3545;">严重</th>
                        <th style="padding: 8px; border-bottom: 2px solid #dee2e6; text-align: center; color: #ffc107;">警告</th>
                        <th style="padding: 8px; border-bottom: 2px solid #dee2e6; text-align: center; color: #28a745;">正常</th>
                    </tr>
                </thead>
                <tbody>%s
                </tbody>
            </table>
        </div>
        
        <div style="background-color: #f8f9fa; padding: 15px; border-radius: 5px; margin: 15px 0;">
            <h3 style="color: #495057; margin-top: 0;">🚨 告警汇总</h3>
            <table style="border-collapse: collapse; width: 100%%;">
                <tr>
                    <td style="padding: 8px; border-bottom: 1px solid #dee2e6;"><strong>总体状态：</strong></td>
                    <td style="padding: 8px; border-bottom: 1px solid #dee2e6; color: %s;">%s</td>
                </tr>
                <tr>
                    <td style="padding: 8px; border-bottom: 1px solid #dee2e6;"><strong>总指标数：</strong></td>
                    <td style="padding: 8px; border-bottom: 1px solid #dee2e6;">%d</td>
                </tr>
                <tr>
                    <td style="padding: 8px; border-bottom: 1px solid #dee2e6;"><strong>异常指标：</strong></td>
                    <td style="padding: 8px; border-bottom: 1px solid #dee2e6; color: #dc3545;">%d</td>
                </tr>
                <tr>
                    <td style="padding: 8px; border-bottom: 1px solid #dee2e6; padding-left: 20px;"><strong>🔴 严重告警：</strong></td>
                    <td style="padding: 8px; border-bottom: 1px solid #dee2e6; color: #dc3545;">%d</td>
                </tr>
                <tr>
                    <td style="padding: 8px; border-bottom: 1px solid #dee2e6; padding-left: 20px;"><strong>🟡 警告告警：</strong></td>
                    <td style="padding: 8px; border-bottom: 1px solid #dee2e6; color: #ffc107;">%d</td>
                </tr>
                <tr>
                    <td style="padding: 8px;"><strong>正常指标：</strong></td>
                    <td style="padding: 8px; color: #28a745;">%d</td>
                </tr>
            </table>
        </div>
        
        <div style="background-color: #e9ecef; padding: 15px; border-radius: 5px;">
            <h3 style="color: #495057; margin-top: 0;">📄 报告详情</h3>
            <p><strong>生成时间：</strong>%s</p>
            <p><strong>报告文件：</strong>%s</p>
            <p><strong>在线查看：</strong><a href="%s" style="color: #007bff;">点击查看报告</a></p>
        </div>
        
        <p style="margin-top: 20px; color: #6c757d;"><strong>请登录环境查看完整报告内容!</strong></p>
    `,
		statusColor,
		projectName,
		alertStatus,
		typeSummaryHTML,
		statusColor,
		alertStatus,
		alertSummary.TotalMetrics,
		alertSummary.TotalAlerts,
		alertSummary.CriticalAlerts,
		alertSummary.WarningAlerts,
		alertSummary.NormalMetrics,
		time.Now().Format("2006-01-02 15:04:05"),
		reportFileName,
		reportLink))

	// 添加附件（仅当有报告文件时）
	if reportPath != "" {
		if _, err := e.AttachFile(reportPath); err != nil {
			log.Printf("添加附件失败: %v", err)
			return fmt.Errorf("添加附件失败: %v", err)
		}
	}

	// 发送邮件（使用TLS）
	addr := fmt.Sprintf("%s:%d", config.SMTPHost, config.SMTPPort)
	auth := smtp.PlainAuth("", config.Username, config.Password, config.SMTPHost)

	tlsConfig := &tls.Config{
		InsecureSkipVerify: true,
		ServerName:         config.SMTPHost,
	}

	log.Printf("正在发送邮件...")
	if err := e.SendWithTLS(addr, auth, tlsConfig); err != nil {
		log.Printf("发送邮件失败: %v", err)
		log.Printf("SMTP配置信息:")
		log.Printf("- 服务器: %s", config.SMTPHost)
		log.Printf("- 端口: %d", config.SMTPPort)
		log.Printf("- 用户名: %s", config.Username)
		return fmt.Errorf("发送邮件失败: %v", err)
	}

	log.Printf("邮件发送成功")
	return nil
}

// calculateDingtalkSign 计算钉钉签名
func calculateDingtalkSign(timestamp int64, secret string) string {
	stringToSign := fmt.Sprintf("%d\n%s", timestamp, secret)
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(stringToSign))
	return url.QueryEscape(base64.StdEncoding.EncodeToString(h.Sum(nil)))
}

// SendWeChatWork 发送企业微信通知（兼容版本）
func SendWeChatWork(config WeChatWorkConfig, reportPath string, projectName string, Datasource string, alertSummary AlertSummary) error {
	return SendWeChatWorkWithContext(context.Background(), config, reportPath, projectName, Datasource, alertSummary)
}

// SendWeChatWorkWithWebhook 发送企业微信通知（支持动态机器人key）
func SendWeChatWorkWithWebhook(ctx context.Context, botKey string, proxyURL string, reportPath string, projectName string, Datasource string, alertSummary AlertSummary) error {
	if botKey == "" {
		log.Printf("企业微信机器人key为空")
		return nil
	}

	// 构建完整的webhook URL
	webhookURL := fmt.Sprintf("https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=%s", botKey)
	log.Printf("开始发送企业微信通知，使用机器人key: %s", botKey)

	// 尝试从context中获取报告数据，用于分类汇总
	var typeSummaries []TypeAlertSummary
	if data, ok := ctx.Value("report_data").(report.ReportData); ok {
		typeSummaries = CalculateTypeAlertSummary(data)
		log.Printf("从报告数据中计算出分类汇总")
	} else {
		log.Printf("未找到报告数据，使用空分类汇总")
		typeSummaries = []TypeAlertSummary{}
	}

	// 生成报告的访问链接
	reportFileName := filepath.Base(reportPath)

	// 尝试从context中获取HTTP请求对象，用于动态URL生成
	var reportLink string
	if r, ok := ctx.Value("http_request").(*http.Request); ok {
		// 打印调试信息
		log.Printf("调试信息: r.Host = %s", r.Host)
		log.Printf("调试信息: X-Forwarded-Host = %s", r.Header.Get("X-Forwarded-Host"))
		log.Printf("调试信息: X-Forwarded-Proto = %s", r.Header.Get("X-Forwarded-Proto"))
		log.Printf("调试信息: TLS = %v", r.TLS != nil)

		// 使用动态URL生成
		reportLink = utils.GetReportURL(r, reportFileName)
		log.Printf("使用动态URL生成报告链接: %s", reportLink)
		log.Printf("最终生成的 reportLink = %s", reportLink)
	} else {
		// 回退到配置的静态URL（如果传入的webhookURL中包含配置信息）
		reportLink = fmt.Sprintf("%s/api/promai/reports/%s", "https://alert.intra.kubehan.cn", reportFileName)
		log.Printf("使用默认静态URL生成报告链接: %s", reportLink)
	}

	// 构建消息内容
	alertStatus := "✅ 正常"
	if alertSummary.TotalAlerts > 0 {
		alertStatus = "⚠️ 异常"
	}

	// 构建分类汇总部分
	typeSummaryText := ""
	for _, summary := range typeSummaries {
		typeStatus := "✅"
		if summary.TotalMetrics == 0 {
			typeStatus = "⚪"
		} else if summary.CriticalCount > 0 {
			typeStatus = "❌"
		} else if summary.WarningCount > 0 {
			typeStatus = "⚠️"
		}
		typeSummaryText += fmt.Sprintf("**%s%s**：总%d个，异常%d个（严重%d，警告%d），正常%d个\n",
			typeStatus, summary.Type, summary.TotalMetrics,
			summary.CriticalCount+summary.WarningCount, summary.CriticalCount, summary.WarningCount, summary.NormalCount)
	}

	messageContent := map[string]interface{}{
		"msgtype": "markdown",
		"markdown": map[string]interface{}{
			"content": fmt.Sprintf("【监测报告】`%s`巡检结果 %s\n\n"+
				"### ⏰ 巡检时间\n"+
				"%s\n\n"+
				"### 📊 分类巡检结果\n"+
				"%s\n"+
				"### 📈 整体统计\n"+
				"**总指标数**：%d个\n"+
				"**异常指标**：%d个（严重%d个，警告%d个）\n"+
				"**正常指标**：%d个\n\n"+
				"📋[点击查看完整报告](%s)\n\n"+
				"⏰ 生成时间：%s",
				Datasource,
				alertStatus,
				time.Now().Format("2006-01-02 15:04:05"),
				typeSummaryText,
				alertSummary.TotalMetrics,
				alertSummary.TotalAlerts,
				alertSummary.CriticalAlerts,
				alertSummary.WarningAlerts,
				alertSummary.NormalMetrics,
				reportLink,
				time.Now().Format("2006-01-02 15:04:05")),
		},
	}

	jsonData, err := json.Marshal(messageContent)
	if err != nil {
		log.Printf("JSON编码失败: %v", err)
		return fmt.Errorf("JSON编码失败: %v", err)
	}

	// 创建HTTP客户端
	client := &http.Client{}

	// 如果配置了代理，设置代理
	if proxyURL != "" {
		log.Printf("使用代理服务器: %s", proxyURL)
		proxyURLParsed, err := url.Parse(proxyURL)
		if err != nil {
			log.Printf("解析代理URL失败: %v", err)
			return fmt.Errorf("解析代理URL失败: %v", err)
		}

		transport := &http.Transport{
			Proxy: http.ProxyURL(proxyURLParsed),
		}
		client.Transport = transport
	}

	// 发送请求
	req, err := http.NewRequest("POST", webhookURL, bytes.NewBuffer(jsonData))
	if err != nil {
		log.Printf("创建请求失败: %v", err)
		return fmt.Errorf("创建请求失败: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	log.Printf("准备发送请求到 webhook: %s", webhookURL)
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("发送请求失败: %v", err)
		return fmt.Errorf("发送请求失败: %v", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	log.Printf("企业微信响应状态码: %d, 响应内容: %s", resp.StatusCode, string(respBody))

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("企业微信发送失败，状态码: %d", resp.StatusCode)
	}

	log.Printf("企业微信通知发送成功")
	return nil
}

// SendWeChatWorkWithContext 发送企业微信通知（支持动态URL）
func SendWeChatWorkWithContext(ctx context.Context, config WeChatWorkConfig, reportPath string, projectName string, Datasource string, alertSummary AlertSummary) error {
	if !config.Enabled {
		log.Printf("企业微信通知未启用")
		return nil
	}
	log.Printf("开始发送企业微信通知...")

	// 尝试从context中获取报告数据，用于分类汇总
	var typeSummaries []TypeAlertSummary
	if data, ok := ctx.Value("report_data").(report.ReportData); ok {
		typeSummaries = CalculateTypeAlertSummary(data)
		log.Printf("从报告数据中计算出分类汇总")
	} else {
		log.Printf("未找到报告数据，使用空分类汇总")
		typeSummaries = []TypeAlertSummary{}
	}

	// 生成报告的访问链接
	reportFileName := filepath.Base(reportPath)

	// 尝试从context中获取HTTP请求对象，用于动态URL生成
	var reportLink string
	if r, ok := ctx.Value("http_request").(*http.Request); ok {
		// 打印调试信息
		log.Printf("调试信息: r.Host = %s", r.Host)
		log.Printf("调试信息: X-Forwarded-Host = %s", r.Header.Get("X-Forwarded-Host"))
		log.Printf("调试信息: X-Forwarded-Proto = %s", r.Header.Get("X-Forwarded-Proto"))
		log.Printf("调试信息: TLS = %v", r.TLS != nil)

		// 使用动态URL生成
		reportLink = utils.GetReportURL(r, reportFileName)
		log.Printf("使用动态URL生成报告链接: %s", reportLink)
		log.Printf("最终生成的 reportLink = %s", reportLink)
	} else {
		// 回退到配置的静态URL
		reportLink = fmt.Sprintf("%s/api/promai/reports/%s", config.ReportURL, reportFileName)
		log.Printf("使用配置的静态URL生成报告链接: %s", reportLink)
		log.Printf("最终生成的 reportLink = %s", reportLink)
	}

	// 构建消息内容
	alertStatus := "✅ 正常"
	if alertSummary.TotalAlerts > 0 {
		alertStatus = "⚠️ 异常"
	}

	// 构建分类汇总部分
	typeSummaryText := ""
	for _, summary := range typeSummaries {
		typeStatus := "✅"
		if summary.TotalMetrics == 0 {
			typeStatus = "⚪"
		} else if summary.CriticalCount > 0 {
			typeStatus = "❌"
		} else if summary.WarningCount > 0 {
			typeStatus = "⚠️"
		}
		typeSummaryText += fmt.Sprintf("**%s%s**：总%d个，异常%d个（严重%d，警告%d），正常%d个\n",
			typeStatus, summary.Type, summary.TotalMetrics,
			summary.CriticalCount+summary.WarningCount, summary.CriticalCount, summary.WarningCount, summary.NormalCount)
	}

	messageContent := map[string]interface{}{
		"msgtype": "markdown",
		"markdown": map[string]interface{}{
			"content": fmt.Sprintf("【监测报告】`%s`巡检结果 %s\n\n"+
				"### ⏰ 巡检时间\n"+
				"%s\n\n"+
				"### 📊 分类巡检结果\n"+
				"%s\n"+
				"### 📈 整体统计\n"+
				"**总指标数**：%d个\n"+
				"**异常指标**：%d个（严重%d个，警告%d个）\n"+
				"**正常指标**：%d个\n\n"+
				"📋[点击查看完整报告](%s)\n\n"+
				"⏰ 生成时间：%s",
				Datasource,
				alertStatus,
				time.Now().Format("2006-01-02 15:04:05"),
				typeSummaryText,
				alertSummary.TotalMetrics,
				alertSummary.TotalAlerts,
				alertSummary.CriticalAlerts,
				alertSummary.WarningAlerts,
				alertSummary.NormalMetrics,
				reportLink,
				time.Now().Format("2006-01-02 15:04:05")),
		},
	}

	jsonData, err := json.Marshal(messageContent)
	if err != nil {
		log.Printf("JSON编码失败: %v", err)
		return fmt.Errorf("JSON编码失败: %v", err)
	}

	// 创建HTTP客户端
	client := &http.Client{}

	// 如果配置了代理，设置代理
	if config.ProxyURL != "" {
		log.Printf("使用代理服务器: %s", config.ProxyURL)
		proxyURL, err := url.Parse(config.ProxyURL)
		if err != nil {
			log.Printf("解析代理URL失败: %v", err)
			return fmt.Errorf("解析代理URL失败: %v", err)
		}

		transport := &http.Transport{
			Proxy: http.ProxyURL(proxyURL),
		}
		client.Transport = transport
	}

	// 发送请求
	req, err := http.NewRequest("POST", config.Webhook, bytes.NewBuffer(jsonData))
	if err != nil {
		log.Printf("创建请求失败: %v", err)
		return fmt.Errorf("创建请求失败: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	log.Printf("准备发送请求到 webhook: %s", config.Webhook)
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("发送请求失败: %v", err)
		return fmt.Errorf("发送请求失败: %v", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	log.Printf("企业微信响应状态码: %d, 响应内容: %s", resp.StatusCode, string(respBody))

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("企业微信发送失败，状态码: %d", resp.StatusCode)
	}

	log.Printf("企业微信通知发送成功")
	return nil
}

// EmailToUserIDCache 邮箱到UserID的缓存
type EmailToUserIDCache struct {
	cache  map[string]string // email -> userid
	mutex  sync.RWMutex
	expiry time.Time
}

var emailCache = &EmailToUserIDCache{
	cache: make(map[string]string),
}

// convertEmailToUserID 将邮箱转换为UserID
// 支持格式：
// - kubehan@company.com -> 查询API获取UserID
// - kubehan -> 直接返回（假设是UserID）
// - @all -> 直接返回
func convertEmailToUserID(ctx context.Context, config WeChatAppConfig, toUser string) (string, error) {
	// 如果是 @all，直接返回
	if toUser == "@all" {
		return toUser, nil
	}

	// 分割多个用户
	users := strings.Split(toUser, "|")
	var convertedUsers []string

	for _, user := range users {
		user = strings.TrimSpace(user)

		// 如果包含@，认为是邮箱，需要转换
		if strings.Contains(user, "@") {
			userid, err := getUserIDByEmail(ctx, config, user)
			if err != nil {
				log.Printf("警告: 邮箱 %s 转换为UserID失败: %v", user, err)
				// 转换失败，使用原值（企业微信会返回错误）
				convertedUsers = append(convertedUsers, user)
			} else {
				convertedUsers = append(convertedUsers, userid)
			}
		} else {
			// 不包含@，假设已经是UserID
			convertedUsers = append(convertedUsers, user)
		}
	}

	return strings.Join(convertedUsers, "|"), nil
}

// getUserIDByEmail 通过邮箱获取UserID（使用企业微信官方API）
func getUserIDByEmail(ctx context.Context, config WeChatAppConfig, email string) (string, error) {
	// 检查缓存
	emailCache.mutex.RLock()
	if time.Now().Before(emailCache.expiry) {
		if userid, ok := emailCache.cache[email]; ok {
			emailCache.mutex.RUnlock()
			log.Printf("从缓存获取邮箱 %s 对应的UserID: %s", email, userid)
			return userid, nil
		}
	}
	emailCache.mutex.RUnlock()

	// 获取access_token
	accessToken, err := getWeChatAccessToken(ctx, config)
	if err != nil {
		return "", err
	}

	// 使用企业微信官方API：通过邮箱获取UserID
	// API文档: https://developer.work.weixin.qq.com/document/path/95892
	apiURL := fmt.Sprintf("https://qyapi.weixin.qq.com/cgi-bin/user/get_userid_by_email?access_token=%s", accessToken)

	// 构建请求体
	requestBody := map[string]interface{}{
		"email":      email,
		"email_type": 1, // 1-企业邮箱
	}
	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return "", fmt.Errorf("构建请求失败: %v", err)
	}

	client := &http.Client{}
	if config.ProxyURL != "" {
		proxyURL, _ := url.Parse(config.ProxyURL)
		client.Transport = &http.Transport{
			Proxy: http.ProxyURL(proxyURL),
		}
	}

	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
		UserID  string `json:"userid"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	if result.ErrCode != 0 {
		return "", fmt.Errorf("获取UserID失败(errcode=%d): %s", result.ErrCode, result.ErrMsg)
	}

	if result.UserID == "" {
		return "", fmt.Errorf("未找到邮箱 %s 对应的用户", email)
	}

	// 更新缓存
	emailCache.mutex.Lock()
	emailCache.cache[email] = result.UserID
	emailCache.expiry = time.Now().Add(1 * time.Hour)
	emailCache.mutex.Unlock()

	log.Printf("邮箱 %s 对应的UserID: %s", email, result.UserID)
	return result.UserID, nil
}

// WeChatAccessTokenResponse 企业微信获取Token响应
type WeChatAccessTokenResponse struct {
	ErrCode     int    `json:"errcode"`
	ErrMsg      string `json:"errmsg"`
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
}

// getWeChatAccessToken 获取企业微信应用 access_token
func getWeChatAccessToken(ctx context.Context, config WeChatAppConfig) (string, error) {
	apiURL := fmt.Sprintf("https://qyapi.weixin.qq.com/cgi-bin/gettoken?corpid=%s&corpsecret=%s", config.CorpID, config.Secret)

	client := &http.Client{}

	// 如果配置了代理，设置代理
	if config.ProxyURL != "" {
		proxyURL, err := url.Parse(config.ProxyURL)
		if err != nil {
			return "", fmt.Errorf("解析代理URL失败: %v", err)
		}
		client.Transport = &http.Transport{
			Proxy: http.ProxyURL(proxyURL),
		}
	}

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return "", fmt.Errorf("创建请求失败: %v", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("获取access_token失败: %v", err)
	}
	defer resp.Body.Close()

	var result WeChatAccessTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("解析响应失败: %v", err)
	}

	if result.ErrCode != 0 {
		return "", fmt.Errorf("获取access_token错误: %s", result.ErrMsg)
	}

	return result.AccessToken, nil
}

// SendWeChatApp 发送企业微信应用通知（兼容版本）
func SendWeChatApp(config WeChatAppConfig, reportPath string, projectName string, Datasource string, alertSummary AlertSummary) error {
	return SendWeChatAppWithContext(context.Background(), config, reportPath, projectName, Datasource, alertSummary)
}

// SendWeChatAppWithContext 发送企业微信应用通知
func SendWeChatAppWithContext(ctx context.Context, config WeChatAppConfig, reportPath string, projectName string, Datasource string, alertSummary AlertSummary) error {
	if !config.Enabled {
		log.Printf("企业微信应用通知未启用")
		return nil
	}
	log.Printf("开始发送企业微信应用通知...")

	// 1. 获取 Access Token
	accessToken, err := getWeChatAccessToken(ctx, config)
	if err != nil {
		log.Printf("获取企业微信应用Token失败: %v", err)
		return err
	}

	// 尝试从context中获取报告数据，用于分类汇总
	var typeSummaries []TypeAlertSummary
	if data, ok := ctx.Value("report_data").(report.ReportData); ok {
		typeSummaries = CalculateTypeAlertSummary(data)
		log.Printf("从报告数据中计算出分类汇总")
	} else {
		log.Printf("未找到报告数据，使用空分类汇总")
		typeSummaries = []TypeAlertSummary{}
	}

	// 生成报告的访问链接
	reportFileName := filepath.Base(reportPath)

	// 尝试从context中获取HTTP请求对象，用于动态URL生成
	var reportLink string
	if r, ok := ctx.Value("http_request").(*http.Request); ok {
		reportLink = utils.GetReportURL(r, reportFileName)
		log.Printf("使用动态URL生成报告链接: %s", reportLink)
	} else {
		// 回退到配置的静态URL
		reportLink = fmt.Sprintf("%s/api/promai/reports/%s", config.ReportURL, reportFileName)
		log.Printf("使用配置的静态URL生成报告链接: %s", reportLink)
	}

	// 构建消息内容
	alertStatus := "✅ 正常"
	if alertSummary.TotalAlerts > 0 {
		alertStatus = "⚠️ 异常"
	}

	// 构建分类汇总部分
	typeSummaryText := ""
	for _, summary := range typeSummaries {
		typeStatus := "✅"
		if summary.TotalMetrics == 0 {
			typeStatus = "⚪"
		} else if summary.CriticalCount > 0 {
			typeStatus = "❌"
		} else if summary.WarningCount > 0 {
			typeStatus = "⚠️"
		}
		typeSummaryText += fmt.Sprintf("**%s%s**：总%d个，异常%d个（严重%d，警告%d），正常%d个\n",
			typeStatus, summary.Type, summary.TotalMetrics,
			summary.CriticalCount+summary.WarningCount, summary.CriticalCount, summary.WarningCount, summary.NormalCount)
	}

	content := fmt.Sprintf("【监测报告】`%s`巡检结果 %s\n\n"+
		"### ⏰ 巡检时间\n"+
		"%s\n\n"+
		"### 📊 分类巡检结果\n"+
		"%s\n"+
		"### 📈 整体统计\n"+
		"**总指标数**：%d个\n"+
		"**异常指标**：%d个（严重%d个，警告%d个）\n"+
		"**正常指标**：%d个\n\n"+
		"📋[点击查看完整报告](%s)\n\n"+
		"⏰ 生成时间：%s",
		Datasource,
		alertStatus,
		time.Now().Format("2006-01-02 15:04:05"),
		typeSummaryText,
		alertSummary.TotalMetrics,
		alertSummary.TotalAlerts,
		alertSummary.CriticalAlerts,
		alertSummary.WarningAlerts,
		alertSummary.NormalMetrics,
		reportLink,
		time.Now().Format("2006-01-02 15:04:05"))

	// 确定接收人：如果配置了ToUser则使用，否则默认发送给所有人
	toUser := "@all"
	if config.ToUser != "" {
		toUser = config.ToUser
		log.Printf("使用配置的接收人: %s", toUser)
	} else {
		log.Printf("未配置接收人，默认发送给所有人")
	}

	// 转换邮箱为UserID
	convertedToUser, err := convertEmailToUserID(ctx, config, toUser)
	if err != nil {
		log.Printf("警告: 转换邮箱为UserID时出错: %v，使用原值", err)
		convertedToUser = toUser
	} else if convertedToUser != toUser {
		log.Printf("邮箱转换: %s -> %s", toUser, convertedToUser)
	}

	messageContent := map[string]interface{}{
		"touser":  convertedToUser,
		"msgtype": "markdown",
		"agentid": config.AgentID,
		"markdown": map[string]interface{}{
			"content": content,
		},
		"enable_duplicate_check":   0,
		"duplicate_check_interval": 1800,
	}

	jsonData, err := json.Marshal(messageContent)
	if err != nil {
		log.Printf("JSON编码失败: %v", err)
		return fmt.Errorf("JSON编码失败: %v", err)
	}

	// 发送消息
	sendURL := fmt.Sprintf("https://qyapi.weixin.qq.com/cgi-bin/message/send?access_token=%s", accessToken)

	client := &http.Client{}
	if config.ProxyURL != "" {
		proxyURL, _ := url.Parse(config.ProxyURL)
		client.Transport = &http.Transport{
			Proxy: http.ProxyURL(proxyURL),
		}
	}

	req, err := http.NewRequestWithContext(ctx, "POST", sendURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("创建请求失败: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	log.Printf("准备发送请求到企业微信API")
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("发送请求失败: %v", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	log.Printf("企业微信应用响应状态码: %d, 响应内容: %s", resp.StatusCode, string(respBody))

	var respJson map[string]interface{}
	json.Unmarshal(respBody, &respJson)

	if errCode, ok := respJson["errcode"].(float64); ok && errCode != 0 {
		return fmt.Errorf("企业微信应用发送失败: %v", respJson["errmsg"])
	}

	log.Printf("企业微信应用通知发送成功")
	return nil
}
