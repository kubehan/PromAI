package piagent

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"path/filepath"
	"strings"
	"time"

	"github.com/jay-y/pi/pkg/ai"
	agent "github.com/jay-y/pi/pkg/ai-agent"
)

type ListReportsTool struct {
	db DB
}

func (t *ListReportsTool) GetName() string  { return "list_reports" }
func (t *ListReportsTool) GetLabel() string { return "查询巡检报告" }
func (t *ListReportsTool) GetDescription() string {
	return "查询历史巡检报告列表，支持按数据源、状态、时间筛选"
}

func (t *ListReportsTool) GetParameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"datasource": map[string]any{
				"type":        "string",
				"description": "数据源名称",
			},
			"status": map[string]any{
				"type":        "string",
				"description": "报告状态: success(正常), danger(高危), warning(告警)",
			},
			"page": map[string]any{
				"type":        "integer",
				"description": "页码，默认 1",
			},
			"page_size": map[string]any{
				"type":        "integer",
				"description": "每页数量，默认 5",
			},
		},
	}
}

func (t *ListReportsTool) Execute(ctx context.Context, params map[string]any, onUpdate func(*agent.AgentToolResult)) (*agent.AgentToolResult, error) {
	ds, _ := params["datasource"].(string)
	status, _ := params["status"].(string)
	page, _ := params["page"].(float64)
	pageSize, _ := params["page_size"].(float64)
	log.Printf("[PiAgent] 工具调用: list_reports datasource=%s status=%s page=%.0f", ds, status, page)
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 5
	}
	normalizedStatus := normalizeReportStatus(status)
	dsKeywords := t.resolveDatasourceKeywords(ds)

	var records []ReportRecord
	t.db.Model(&ReportRecord{}).Order("created_at desc").Find(&records)
	records = filterReportRecords(records, dsKeywords, normalizedStatus)
	records = dedupeReportRecords(records)
	total := int64(len(records))
	start := int((page - 1) * pageSize)
	end := start + int(pageSize)
	if start > len(records) {
		start = len(records)
	}
	if end > len(records) {
		end = len(records)
	}
	pageRecords := records[start:end]

	lines := []string{}
	lines = append(lines, fmt.Sprintf("📋 巡检报告列表（共 %d 条，当前第 %.0f 页）", total, page))
	lines = append(lines, "")

	if len(pageRecords) == 0 && normalizedStatus == "" {
		var inspectRecords []InspectRecord
		t.db.Model(&InspectRecord{}).Order("created_at desc").Find(&inspectRecords)
		inspectRecords = filterInspectRecords(inspectRecords, dsKeywords)
		inspectRecords = dedupeInspectRecords(inspectRecords)
		if len(inspectRecords) == 0 {
			lines = append(lines, "暂无报告")
		} else {
			lines = append(lines, "report_records 暂无匹配结果，已回退到历史巡检任务记录：")
			lines = append(lines, "")
			for _, r := range inspectRecords[:minInt(len(inspectRecords), int(pageSize))] {
				lines = append(lines, fmt.Sprintf("  📄 %s", fallbackInspectTitle(r)))
				lines = append(lines, fmt.Sprintf("     数据源: %s | 状态: %s", r.DatasourceName, humanizeReportStatus(r.Status)))
				lines = append(lines, fmt.Sprintf("     时间: %s", r.CreatedAt.Format("2006-01-02 15:04:05")))
				lines = append(lines, fmt.Sprintf("     链接: [查看报告](%s)", r.ReportURL))
				lines = append(lines, "")
			}
		}
	} else {
		for _, r := range pageRecords {
			lines = append(lines, fmt.Sprintf("  📄 %s", r.Title))
			lines = append(lines, fmt.Sprintf("     数据源: %s | 状态: %s", r.DatasourceName, humanizeReportStatus(r.Status)))
			lines = append(lines, fmt.Sprintf("     指标: %d | 告警: %d (严重: %d, 警告: %d)",
				r.TotalMetrics, r.AlertCount, r.CriticalCount, r.WarningCount))
			lines = append(lines, fmt.Sprintf("     时间: %s", r.CreatedAt.Format("2006-01-02 15:04:05")))
			lines = append(lines, fmt.Sprintf("     ID: %d", r.ID))
			if link := buildReportLink(r.FilePath); link != "" {
				lines = append(lines, fmt.Sprintf("     链接: [查看报告](%s)", link))
			}
			lines = append(lines, "")
		}
	}

	return &agent.AgentToolResult{
		Content: []ai.ContentBlock{ai.NewTextContentBlock(strings.Join(lines, "\n"))},
	}, nil
}

type GetReportDetailTool struct {
	db DB
}

func (t *GetReportDetailTool) GetName() string  { return "get_report_detail" }
func (t *GetReportDetailTool) GetLabel() string { return "读取报告详情" }
func (t *GetReportDetailTool) GetDescription() string {
	return "读取特定巡检报告的详细指标数据"
}

func (t *GetReportDetailTool) GetParameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"report_id": map[string]any{
				"type":        "integer",
				"description": "报告 ID",
			},
		},
		"required": []string{"report_id"},
	}
}

func (t *GetReportDetailTool) Execute(ctx context.Context, params map[string]any, onUpdate func(*agent.AgentToolResult)) (*agent.AgentToolResult, error) {
	id, _ := params["report_id"].(float64)
	log.Printf("[PiAgent] 工具调用: get_report_detail report_id=%.0f", id)
	if id < 1 {
		return &agent.AgentToolResult{
			Content: []ai.ContentBlock{ai.NewTextContentBlock("请提供有效的报告 ID")},
		}, nil
	}

	var record ReportRecord
	if t.db.First(&record, uint(id)).Error() != nil {
		return &agent.AgentToolResult{
			Content: []ai.ContentBlock{ai.NewTextContentBlock(fmt.Sprintf("报告 %d 不存在", uint(id)))},
		}, nil
	}

	lines := []string{}
	lines = append(lines, fmt.Sprintf("📄 报告详情: %s", record.Title))
	lines = append(lines, fmt.Sprintf("数据源: %s", record.DatasourceName))
	lines = append(lines, fmt.Sprintf("状态: %s", humanizeReportStatus(record.Status)))
	lines = append(lines, fmt.Sprintf("总指标: %d", record.TotalMetrics))
	lines = append(lines, fmt.Sprintf("告警数: %d (严重: %d, 警告: %d)", record.AlertCount, record.CriticalCount, record.WarningCount))
	lines = append(lines, fmt.Sprintf("生成时间: %s", record.CreatedAt.Format("2006-01-02 15:04:05")))
	if link := buildReportLink(record.FilePath); link != "" {
		lines = append(lines, fmt.Sprintf("报告链接: [查看报告](%s)", link))
	}

	if record.MetricsJSON != "" {
		lines = append(lines, "")
		lines = append(lines, "📊 指标快照:")
		var snapshot map[string]any
		if err := json.Unmarshal([]byte(record.MetricsJSON), &snapshot); err == nil {
			if metrics, ok := snapshot["metrics"].([]any); ok {
				criticalMetrics := []string{}
				warningMetrics := []string{}
				for _, m := range metrics {
					if mm, ok := m.(map[string]any); ok {
						name, _ := mm["metric_name"].(string)
						status, _ := mm["status"].(string)
						val, _ := mm["value"].(float64)
						unit, _ := mm["unit"].(string)
						line := fmt.Sprintf("    • %s = %.2f%s", name, val, unit)
						switch status {
						case "critical":
							criticalMetrics = append(criticalMetrics, "🔴 "+line)
						case "warning":
							warningMetrics = append(warningMetrics, "🟡 "+line)
						}
					}
				}
				if len(criticalMetrics) > 0 {
					lines = append(lines, "  严重告警指标:")
					lines = append(lines, criticalMetrics...)
				}
				if len(warningMetrics) > 0 {
					lines = append(lines, "  警告指标:")
					lines = append(lines, warningMetrics...)
				}
			}
		}
	}

	return &agent.AgentToolResult{
		Content: []ai.ContentBlock{ai.NewTextContentBlock(strings.Join(lines, "\n"))},
	}, nil
}

type ReportRecord struct {
	ID             uint      `json:"id"`
	Title          string    `json:"title"`
	DatasourceID   *uint     `json:"datasource_id"`
	DatasourceName string    `json:"datasource_name"`
	FilePath       string    `json:"file_path"`
	FileSize       int64     `json:"file_size"`
	TotalMetrics   int       `json:"total_metrics"`
	AlertCount     int       `json:"alert_count"`
	CriticalCount  int       `json:"critical_count"`
	WarningCount   int       `json:"warning_count"`
	Status         string    `json:"status"`
	MetricsJSON    string    `json:"metrics_json"`
	CreatedAt      time.Time `json:"created_at"`
}

func (ReportRecord) TableName() string { return "report_records" }

func (t *ListReportsTool) resolveDatasourceKeywords(keyword string) []string {
	keyword = strings.TrimSpace(keyword)
	if keyword == "" {
		return nil
	}
	result := []string{strings.ToLower(keyword)}
	var datasources []DataSource
	t.db.Model(&DataSource{}).Find(&datasources)
	for _, ds := range datasources {
		if strings.Contains(strings.ToLower(ds.Name), strings.ToLower(keyword)) || strings.Contains(strings.ToLower(ds.URL), strings.ToLower(keyword)) {
			result = append(result, strings.ToLower(ds.Name), strings.ToLower(ds.URL))
		}
	}
	return uniqueStrings(result)
}

func filterReportRecords(records []ReportRecord, dsKeywords []string, status string) []ReportRecord {
	result := make([]ReportRecord, 0, len(records))
	for _, record := range records {
		if status != "" && normalizeReportStatus(record.Status) != status {
			continue
		}
		if len(dsKeywords) > 0 {
			target := strings.ToLower(record.DatasourceName + " " + record.FilePath + " " + record.Title)
			matched := false
			for _, keyword := range dsKeywords {
				if strings.Contains(target, keyword) {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}
		result = append(result, record)
	}
	return result
}

func filterInspectRecords(records []InspectRecord, dsKeywords []string) []InspectRecord {
	result := make([]InspectRecord, 0, len(records))
	for _, record := range records {
		if record.ReportURL == "" {
			continue
		}
		status := strings.ToLower(record.Status)
		if status != "completed" && status != "success" {
			continue
		}
		if len(dsKeywords) > 0 {
			target := strings.ToLower(record.DatasourceName + " " + record.ReportURL)
			matched := false
			for _, keyword := range dsKeywords {
				if strings.Contains(target, keyword) {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}
		result = append(result, record)
	}
	return result
}

func dedupeReportRecords(records []ReportRecord) []ReportRecord {
	result := make([]ReportRecord, 0, len(records))
	seen := make(map[string]struct{}, len(records))
	for _, record := range records {
		key := filepath.Base(record.FilePath)
		if key == "." || key == "" {
			key = fmt.Sprintf("report-%d", record.ID)
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, record)
	}
	return result
}

func dedupeInspectRecords(records []InspectRecord) []InspectRecord {
	result := make([]InspectRecord, 0, len(records))
	seen := make(map[string]struct{}, len(records))
	for _, record := range records {
		key := filepath.Base(record.ReportURL)
		if key == "." || key == "" {
			key = fmt.Sprintf("inspect-%d", record.ID)
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, record)
	}
	return result
}

func buildReportLink(filePath string) string {
	name := filepath.Base(strings.TrimSpace(filePath))
	if name == "" || name == "." {
		return ""
	}
	return "/api/promai/reports/" + name
}

func normalizeReportStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "", "all":
		return ""
	case "success", "normal", "completed":
		return "success"
	case "warning", "warn":
		return "warning"
	case "danger", "critical", "error", "failed":
		return "danger"
	default:
		return strings.ToLower(strings.TrimSpace(status))
	}
}

func humanizeReportStatus(status string) string {
	switch normalizeReportStatus(status) {
	case "success":
		return "✅ 正常"
	case "warning":
		return "🟡 告警"
	case "danger":
		return "🔴 高危"
	default:
		return status
	}
}

func fallbackInspectTitle(record InspectRecord) string {
	name := filepath.Base(record.ReportURL)
	if name == "" || name == "." {
		return "巡检报告"
	}
	return name
}

func uniqueStrings(items []string) []string {
	seen := make(map[string]struct{}, len(items))
	result := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		result = append(result, item)
	}
	return result
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
