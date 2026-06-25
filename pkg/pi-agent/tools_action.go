package piagent

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"PromAI/pkg/config"
	"PromAI/pkg/metrics"
	"PromAI/pkg/prometheus"
	"PromAI/pkg/report"
	"github.com/jay-y/pi/pkg/ai"
	agent "github.com/jay-y/pi/pkg/ai-agent"
)

type InspectTaskItem struct {
	ID        string    `json:"id"`
	Status    string    `json:"status"`
	Message   string    `json:"message"`
	ReportURL string    `json:"report_url,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// InspectRecord 巡检任务记录，映射数据库 inspect_records 表
type InspectRecord struct {
	ID             uint       `gorm:"primaryKey" json:"id"`
	TaskID         string     `gorm:"size:100;index" json:"task_id"`
	Status         string     `gorm:"size:50;default:running" json:"status"`
	DatasourceID   *uint      `json:"datasource_id"`
	DatasourceName string     `gorm:"size:200" json:"datasource_name"`
	Message        string     `gorm:"size:500" json:"message"`
	Error          string     `gorm:"size:500" json:"error"`
	ReportURL      string     `gorm:"size:500" json:"report_url"`
	StartedAt      time.Time  `json:"started_at"`
	CompletedAt    *time.Time `json:"completed_at"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

func (InspectRecord) TableName() string { return "inspect_records" }

type TriggerInspectTool struct {
	config    *config.Config
	collector *metrics.Collector
	db        DB
	tasks     map[string]*InspectTaskItem
	tasksMu   sync.Mutex
	counter   int
}

func NewTriggerInspectTool(cfg *config.Config, collector *metrics.Collector, db DB) *TriggerInspectTool {
	return &TriggerInspectTool{
		config:    cfg,
		collector: collector,
		db:        db,
		tasks:     make(map[string]*InspectTaskItem),
	}
}

func (t *TriggerInspectTool) GetName() string  { return "trigger_inspect" }
func (t *TriggerInspectTool) GetLabel() string { return "触发巡检" }
func (t *TriggerInspectTool) GetDescription() string {
	return "对指定数据源手动触发一次巡检"
}

func (t *TriggerInspectTool) GetParameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"datasource": map[string]any{
				"type":        "string",
				"description": "数据源名称或 URL，留空使用默认数据源",
			},
		},
	}
}

func (t *TriggerInspectTool) Execute(ctx context.Context, params map[string]any, onUpdate func(*agent.AgentToolResult)) (*agent.AgentToolResult, error) {
	dsParam, _ := params["datasource"].(string)
	log.Printf("[PiAgent] 工具调用: trigger_inspect datasource=%s", dsParam)

	ds, promURL, promUser, promPass, dsName := resolveToolDatasource(t.config, t.db, dsParam)
	runtimeCfg, err := buildToolRuntimeMetricConfig(t.config, t.db, ds)
	if err != nil {
		return &agent.AgentToolResult{
			Content: []ai.ContentBlock{ai.NewTextContentBlock(fmt.Sprintf("加载数据源巡检配置失败: %v", err))},
		}, nil
	}
	if len(runtimeCfg.MetricTypes) == 0 {
		target := dsName
		if ds != nil {
			target = ds.Name
		}
		return &agent.AgentToolResult{
			Content: []ai.ContentBlock{ai.NewTextContentBlock(fmt.Sprintf("数据源 %s 未配置巡检模板或指标，无法执行巡检", target))},
		}, nil
	}

	client, err := prometheus.NewClient(promURL, promUser, promPass)
	if err != nil {
		return &agent.AgentToolResult{
			Content: []ai.ContentBlock{ai.NewTextContentBlock(fmt.Sprintf("创建客户端失败: %v", err))},
		}, nil
	}

	hcCtx, hcCancel := context.WithTimeout(ctx, 10*time.Second)
	if err := client.HealthCheck(hcCtx); err != nil {
		hcCancel()
		return &agent.AgentToolResult{
			Content: []ai.ContentBlock{ai.NewTextContentBlock(fmt.Sprintf("数据源 %s 不可用: %v", promURL, err))},
		}, nil
	}
	hcCancel()

	t.tasksMu.Lock()
	t.counter++
	taskID := fmt.Sprintf("ai_task_%d_%d", time.Now().Unix(), t.counter)
	task := &InspectTaskItem{
		ID:        taskID,
		Status:    "running",
		Message:   "正在执行巡检...",
		CreatedAt: time.Now(),
	}
	t.tasks[taskID] = task
	t.tasksMu.Unlock()

	// 创建持久化巡检记录
	now := time.Now()
	t.db.Create(&InspectRecord{
		TaskID: taskID,
		Status: "running",
		DatasourceID: func() *uint {
			if ds != nil {
				return &ds.ID
			}
			return nil
		}(),
		DatasourceName: dsName,
		Message:        "正在执行巡检...",
		StartedAt:      now,
		CreatedAt:      now,
	})

	go func() {
		dataCollector := metrics.NewCollectorWithURL(client.API, runtimeCfg, promURL)
		data, err := dataCollector.CollectMetrics()
		if err != nil {
			t.tasksMu.Lock()
			task.Status = "failed"
			task.Message = fmt.Sprintf("收集指标失败: %v", err)
			t.tasksMu.Unlock()
			t.db.Model(&InspectRecord{}).Where("task_id = ?", taskID).Updates(map[string]any{
				"status": "failed", "error": err.Error(), "completed_at": time.Now(),
			})
			return
		}
		data.Datasource = promURL

		reportPath, err := report.GenerateReport(*data)
		if err != nil {
			t.tasksMu.Lock()
			task.Status = "failed"
			task.Message = fmt.Sprintf("生成报告失败: %v", err)
			t.tasksMu.Unlock()
			t.db.Model(&InspectRecord{}).Where("task_id = ?", taskID).Updates(map[string]any{
				"status": "failed", "error": err.Error(), "completed_at": time.Now(),
			})
			return
		}

		// 保存报告记录到数据库
		alertCount := 0
		criticalCount := 0
		warningCount := 0
		totalMetrics := 0
		hasCritical := false
		hasWarning := false
		var abnormalDetails []map[string]any
		typeSummaries := map[string]map[string]int{}

		for typeName, group := range data.MetricGroups {
			ts := map[string]int{"critical": 0, "warning": 0, "normal": 0, "total": 0}
			for _, metrics := range group.MetricsByName {
				for _, m := range metrics {
					totalMetrics++
					ts["total"]++
					switch m.Status {
					case "critical":
						criticalCount++
						alertCount++
						hasCritical = true
						ts["critical"]++
					case "warning":
						warningCount++
						alertCount++
						hasWarning = true
						ts["warning"]++
					default:
						ts["normal"]++
					}
					if m.Status == "critical" || m.Status == "warning" {
						labels := make(map[string]string)
						for _, lbl := range m.Labels {
							if lbl.Value != "" && lbl.Name != "__name__" {
								labels[lbl.Name] = lbl.Value
							}
						}
						detail := map[string]any{
							"type":   typeName,
							"name":   m.Name,
							"value":  m.Value,
							"unit":   m.Unit,
							"status": m.Status,
							"labels": labels,
						}
						if m.ThresholdType != "" {
							detail["threshold"] = m.Threshold
							detail["threshold_type"] = m.ThresholdType
						}
						abnormalDetails = append(abnormalDetails, detail)
					}
				}
			}
			typeSummaries[typeName] = ts
		}

		status := "success"
		if hasCritical {
			status = "danger"
		} else if hasWarning {
			status = "warning"
		}

		info, _ := os.Stat(reportPath)
		fileSize := int64(0)
		if info != nil {
			fileSize = info.Size()
		}

		snapshot := map[string]any{
			"datasource_name":  data.Datasource,
			"total_metrics":    totalMetrics,
			"critical_count":   criticalCount,
			"warning_count":    warningCount,
			"abnormal_details": abnormalDetails,
			"type_summaries":   typeSummaries,
		}
		metricsJSON, _ := json.Marshal(snapshot)

		titlePrefix := runtimeCfg.ProjectName
		if titlePrefix == "" {
			titlePrefix = "巡检报告"
		}
		record := ReportRecord{
			Title:          fmt.Sprintf("%s - %s", titlePrefix, time.Now().Format("2006-01-02 15:04")),
			DatasourceName: data.Datasource,
			FilePath:       reportPath,
			FileSize:       fileSize,
			TotalMetrics:   totalMetrics,
			AlertCount:     alertCount,
			CriticalCount:  criticalCount,
			WarningCount:   warningCount,
			Status:         status,
			MetricsJSON:    string(metricsJSON),
			CreatedAt:      time.Now(),
		}
		if ds != nil {
			record.DatasourceID = &ds.ID
		}
		t.db.Create(&record)

		reportURL := "/api/promai/reports/" + reportPath[len("reports/"):]
		t.tasksMu.Lock()
		task.Status = "completed"
		task.Message = "巡检完成"
		task.ReportURL = reportURL
		t.tasksMu.Unlock()

		t.db.Model(&InspectRecord{}).Where("task_id = ?", taskID).Updates(map[string]any{
			"status": "completed", "message": "巡检完成", "report_url": reportURL, "completed_at": time.Now(),
		})
	}()

	return &agent.AgentToolResult{
		Content: []ai.ContentBlock{ai.NewTextContentBlock(
			fmt.Sprintf("✅ 巡检任务已创建\n任务ID: %s\n数据源: %s\n可使用 query_task 工具查询进度", taskID, promURL),
		)},
	}, nil
}

type QueryTaskTool struct {
	parent *TriggerInspectTool
}

func (t *QueryTaskTool) GetName() string        { return "query_task" }
func (t *QueryTaskTool) GetLabel() string       { return "查询任务" }
func (t *QueryTaskTool) GetDescription() string { return "查询巡检任务的执行进度和结果" }

func (t *QueryTaskTool) GetParameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"task_id": map[string]any{
				"type":        "string",
				"description": "任务 ID",
			},
		},
		"required": []string{"task_id"},
	}
}

func (t *QueryTaskTool) Execute(ctx context.Context, params map[string]any, onUpdate func(*agent.AgentToolResult)) (*agent.AgentToolResult, error) {
	taskID, _ := params["task_id"].(string)
	log.Printf("[PiAgent] 工具调用: query_task task_id=%s", taskID)
	if taskID == "" {
		return &agent.AgentToolResult{
			Content: []ai.ContentBlock{ai.NewTextContentBlock("请提供任务 ID")},
		}, nil
	}

	t.parent.tasksMu.Lock()
	task, ok := t.parent.tasks[taskID]
	t.parent.tasksMu.Unlock()

	if !ok {
		return &agent.AgentToolResult{
			Content: []ai.ContentBlock{ai.NewTextContentBlock(fmt.Sprintf("任务 %s 不存在", taskID))},
		}, nil
	}

	statusIcon := map[string]string{
		"running":   "⏳ 执行中",
		"completed": "✅ 已完成",
		"failed":    "❌ 失败",
	}[task.Status]

	lines := []string{
		fmt.Sprintf("📋 任务状态: %s", statusIcon),
		fmt.Sprintf("任务 ID: %s", task.ID),
		fmt.Sprintf("消息: %s", task.Message),
	}
	if task.ReportURL != "" {
		lines = append(lines, fmt.Sprintf("报告链接: [📄 点击查看报告](%s)", task.ReportURL))
	}
	lines = append(lines, fmt.Sprintf("创建: %s", task.CreatedAt.Format("2006-01-02 15:04:05")))

	// 任务完成时，从数据库读取报告分析摘要
	if task.Status == "completed" && task.ReportURL != "" {
		filename := strings.TrimPrefix(task.ReportURL, "/api/promai/reports/")
		var r ReportRecord
		if t.parent.db.Model(&ReportRecord{}).Where("file_path LIKE ?", "%"+filename).First(&r).Error() == nil {
			statusText := map[string]string{
				"success": "✅ 正常",
				"danger":  "🔴 高危",
				"warning": "🟡 告警",
			}[r.Status]
			if statusText == "" {
				statusText = r.Status
			}
			lines = append(lines, "")
			lines = append(lines, "📊 **巡检报告摘要**")
			lines = append(lines, fmt.Sprintf("   - 整体状态: %s", statusText))
			lines = append(lines, fmt.Sprintf("   - 总指标数: %d", r.TotalMetrics))
			lines = append(lines, fmt.Sprintf("   - 告警总数: %d", r.AlertCount))
			lines = append(lines, fmt.Sprintf("   - 严重告警: %d", r.CriticalCount))
			lines = append(lines, fmt.Sprintf("   - 警告告警: %d", r.WarningCount))

			// 解析异常明细
			if r.MetricsJSON != "" {
				var snapshot map[string]any
				if err := json.Unmarshal([]byte(r.MetricsJSON), &snapshot); err == nil {
					if details, ok := snapshot["abnormal_details"].([]any); ok && len(details) > 0 {
						lines = append(lines, "")
						lines = append(lines, "📋 **异常明细**")
						currentType := ""
						for _, d := range details {
							item, ok := d.(map[string]any)
							if !ok {
								continue
							}
							typeName, _ := item["type"].(string)
							name, _ := item["name"].(string)
							value, _ := item["value"].(float64)
							unit, _ := item["unit"].(string)
							status, _ := item["status"].(string)
							threshold, _ := item["threshold"].(float64)
							thresholdType, _ := item["threshold_type"].(string)

							if typeName != "" && typeName != currentType {
								currentType = typeName
								lines = append(lines, fmt.Sprintf("  *%s*", typeName))
							}

							statusEmoji := "⚠️"
							statusLabel := "警告"
							if status == "critical" {
								statusEmoji = "🔴"
								statusLabel = "严重"
							}

							thresholdStr := ""
							if thresholdType != "" {
								thresholdStr = fmt.Sprintf(" (阈值: %s %.2f)", thresholdType, threshold)
							}

							labelStr := ""
							if labels, ok := item["labels"].(map[string]any); ok {
								var parts []string
								for k, v := range labels {
									val, _ := v.(string)
									if val != "" {
										parts = append(parts, fmt.Sprintf("%s=%s", k, val))
									}
								}
								if len(parts) > 0 {
									labelStr = " [" + strings.Join(parts, ", ") + "]"
								}
							}

							lines = append(lines, fmt.Sprintf("    %s %s%s: %.2f%s%s (%s)",
								statusEmoji, name, labelStr, value, unit, thresholdStr, statusLabel))
						}
					}
				}
			}
		}
	}

	return &agent.AgentToolResult{
		Content: []ai.ContentBlock{ai.NewTextContentBlock(strings.Join(lines, "\n"))},
	}, nil
}
