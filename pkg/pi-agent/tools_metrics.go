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

type QueryMetricsTool struct {
	config *config.Config
	db     DB
}

func (t *QueryMetricsTool) GetName() string         { return "query_metrics" }
func (t *QueryMetricsTool) GetLabel() string        { return "查询指标" }
func (t *QueryMetricsTool) GetDescription() string  { return "查询 Prometheus 监控指标数据，返回实时指标值" }

func (t *QueryMetricsTool) GetParameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"promql": map[string]any{
				"type":        "string",
				"description": "PromQL 查询语句",
			},
			"datasource": map[string]any{
				"type":        "string",
				"description": "数据源名称或 URL，留空使用默认数据源",
			},
			"description": map[string]any{
				"type":        "string",
				"description": "查询的指标描述，用于帮助理解查询目的",
			},
		},
		"required": []string{"promql"},
	}
}

func (t *QueryMetricsTool) Execute(ctx context.Context, params map[string]any, onUpdate func(*agent.AgentToolResult)) (*agent.AgentToolResult, error) {
	promql, _ := params["promql"].(string)
	dsParam, _ := params["datasource"].(string)
	desc, _ := params["description"].(string)
	log.Printf("[PiAgent] 工具调用: query_metrics promql=%s datasource=%s desc=%s", promql, dsParam, desc)

	promURL := t.config.PrometheusURL
	promUser := t.config.PrometheusUsername
	promPass := t.config.PrometheusPassword

	if dsParam != "" {
		found := false
		if strings.HasPrefix(dsParam, "http://") || strings.HasPrefix(dsParam, "https://") {
			promURL = dsParam
			found = true
		} else {
			var dsList []DataSource
			t.db.Model(&DataSource{}).Where("enabled = ?", true).Find(&dsList)
			for _, ds := range dsList {
				if ds.Name == dsParam {
					promURL = ds.URL
					found = true
					break
				}
			}
		}
		if !found {
			return &agent.AgentToolResult{
				Content: []ai.ContentBlock{ai.NewTextContentBlock(fmt.Sprintf("未找到数据源: %s", dsParam))},
			}, nil
		}
	}

	client, err := prometheus.NewClient(promURL, promUser, promPass)
	if err != nil {
		return &agent.AgentToolResult{
			Content: []ai.ContentBlock{ai.NewTextContentBlock(fmt.Sprintf("创建 Prometheus 客户端失败: %v", err))},
		}, nil
	}

	queryCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	result, _, err := client.API.Query(queryCtx, promql, time.Now())
	if err != nil {
		return &agent.AgentToolResult{
			Content: []ai.ContentBlock{ai.NewTextContentBlock(fmt.Sprintf("查询失败: %v", err))},
		}, nil
	}

	lines := []string{}
	if desc != "" {
		lines = append(lines, fmt.Sprintf("📊 %s", desc))
	}
	lines = append(lines, fmt.Sprintf("数据源: %s", promURL))
	lines = append(lines, fmt.Sprintf("PromQL: %s", promql))
	lines = append(lines, "")

	switch v := result.(type) {
	case model.Vector:
		if len(v) == 0 {
			lines = append(lines, "查询结果为空（无数据返回）")
		}
		for _, sample := range v {
			metricParts := []string{}
			for k, val := range sample.Metric {
				metricParts = append(metricParts, fmt.Sprintf("%s=%s", k, val))
			}
			labels := strings.Join(metricParts, ", ")
			lines = append(lines, fmt.Sprintf("  • 值: %.2f  [%s]", float64(sample.Value), labels))
		}
	case model.Matrix:
		lines = append(lines, fmt.Sprintf("返回范围数据，共 %d 个时间序列", len(v)))
	case *model.Scalar:
		lines = append(lines, fmt.Sprintf("  • 值: %.2f", float64(v.Value)))
	default:
		lines = append(lines, fmt.Sprintf("返回类型: %T", result))
	}

	resultStr := strings.Join(lines, "\n")
	return &agent.AgentToolResult{
		Content: []ai.ContentBlock{ai.NewTextContentBlock(resultStr)},
	}, nil
}
