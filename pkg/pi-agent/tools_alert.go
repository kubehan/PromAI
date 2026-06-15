package piagent

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"PromAI/pkg/config"
	"PromAI/pkg/prometheus"
	agent "github.com/jay-y/pi/pkg/ai-agent"
	"github.com/jay-y/pi/pkg/ai"
	"github.com/prometheus/common/model"
)

type AnalyzeAlertTool struct {
	config *config.Config
	db     DB
}

func (t *AnalyzeAlertTool) GetName() string         { return "analyze_alert" }
func (t *AnalyzeAlertTool) GetLabel() string        { return "分析告警" }
func (t *AnalyzeAlertTool) GetDescription() string  { return "分析告警根因，查询关联指标进行综合判断" }

func (t *AnalyzeAlertTool) GetParameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"metric_name": map[string]any{
				"type":        "string",
				"description": "告警指标名称，例如：CPU性能状态监控",
			},
			"datasource": map[string]any{
				"type":        "string",
				"description": "数据源名称",
			},
			"instance": map[string]any{
				"type":        "string",
				"description": "实例地址，限定分析范围",
			},
		},
		"required": []string{"metric_name"},
	}
}

func (t *AnalyzeAlertTool) Execute(ctx context.Context, params map[string]any, onUpdate func(*agent.AgentToolResult)) (*agent.AgentToolResult, error) {
	metricName, _ := params["metric_name"].(string)
	dsParam, _ := params["datasource"].(string)
	instance, _ := params["instance"].(string)
	log.Printf("[PiAgent] 工具调用: analyze_alert metric=%s datasource=%s instance=%s", metricName, dsParam, instance)

	promURL := t.config.PrometheusURL
	promUser := t.config.PrometheusUsername
	promPass := t.config.PrometheusPassword

	if dsParam != "" {
		var dsList []DataSource
		t.db.Model(&DataSource{}).Where("enabled = ?", true).Find(&dsList)
		for _, ds := range dsList {
			if ds.Name == dsParam {
				promURL = ds.URL
				promUser = ds.Username
				promPass = ds.Password
				break
			}
		}
	}

	client, err := prometheus.NewClient(promURL, promUser, promPass)
	if err != nil {
		return &agent.AgentToolResult{
			Content: []ai.ContentBlock{ai.NewTextContentBlock(fmt.Sprintf("创建客户端失败: %v", err))},
		}, nil
	}

	queryCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	lines := []string{}
	lines = append(lines, fmt.Sprintf("🔍 告警分析: %s", metricName))
	lines = append(lines, fmt.Sprintf("数据源: %s", promURL))
	if instance != "" {
		lines = append(lines, fmt.Sprintf("实例: %s", instance))
	}
	lines = append(lines, "")

	relatedQueries := map[string]string{
		"CPU 使用率": "100 - avg by(instance) (irate(node_cpu_seconds_total{mode='idle'}[5m]) * 100)",
		"内存使用率": "100 - (node_memory_MemAvailable_bytes * 100 / node_memory_MemTotal_bytes)",
		"磁盘使用率": "100 - (node_filesystem_avail_bytes * 100 / node_filesystem_size_bytes)",
	}
	if instance != "" {
		for k, q := range relatedQueries {
			relatedQueries[k] = q + fmt.Sprintf(`{instance="%s"}`, instance)
		}
	}

	lines = append(lines, "📈 关联指标（当前值）:")
	for name, q := range relatedQueries {
		result, _, err := client.API.Query(queryCtx, q, time.Now())
		if err != nil {
			lines = append(lines, fmt.Sprintf("  • %s: 查询失败", name))
			continue
		}
		if vec, ok := result.(model.Vector); ok && len(vec) > 0 {
			avg := 0.0
			for _, s := range vec {
				avg += float64(s.Value)
			}
			avg /= float64(len(vec))
			lines = append(lines, fmt.Sprintf("  • %s: %.1f%% (样本数: %d)", name, avg, len(vec)))
		} else {
			lines = append(lines, fmt.Sprintf("  • %s: 无数据", name))
		}
	}

	lines = append(lines, "")
	lines = append(lines, "📋 最近相关巡检记录:")
	var records []ReportRecord
	t.db.Model(&ReportRecord{}).
		Where("datasource_name LIKE ? AND (critical_count > 0 OR warning_count > 0)", "%"+promURL+"%").
		Order("created_at desc").
		Limit(3).
		Find(&records)
	if len(records) == 0 {
		lines = append(lines, "  暂无近期异常记录")
	} else {
		for _, r := range records {
			dt := ""
			if !r.CreatedAt.IsZero() {
				dt = r.CreatedAt.Format("01-02 15:04")
			}
			lines = append(lines, fmt.Sprintf("  • %s — 严重: %d, 警告: %d", dt, r.CriticalCount, r.WarningCount))
		}
	}

	lines = append(lines, "")
	lines = append(lines, "💡 分析建议:")
	lines = append(lines, "  1. 检查关联指标趋势，确认是否为资源瓶颈")
	lines = append(lines, "  2. 对比历史报告，观察指标变化趋势")
	lines = append(lines, "  3. 如需进一步排查，可以触发一次新的巡检")

	return &agent.AgentToolResult{
		Content: []ai.ContentBlock{ai.NewTextContentBlock(strings.Join(lines, "\n"))},
	}, nil
}
