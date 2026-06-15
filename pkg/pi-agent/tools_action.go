package piagent

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"PromAI/pkg/config"
	"PromAI/pkg/metrics"
	"PromAI/pkg/prometheus"
	"PromAI/pkg/report"
	agent "github.com/jay-y/pi/pkg/ai-agent"
	"github.com/jay-y/pi/pkg/ai"
)

type InspectTaskItem struct {
	ID        string    `json:"id"`
	Status    string    `json:"status"`
	Message   string    `json:"message"`
	ReportURL string    `json:"report_url,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

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

func (t *TriggerInspectTool) GetName() string         { return "trigger_inspect" }
func (t *TriggerInspectTool) GetLabel() string        { return "触发巡检" }
func (t *TriggerInspectTool) GetDescription() string  { return "对指定数据源手动触发一次巡检" }

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

	promURL := t.config.PrometheusURL
	promUser := t.config.PrometheusUsername
	promPass := t.config.PrometheusPassword

	if dsParam != "" {
		if strings.HasPrefix(dsParam, "http://") || strings.HasPrefix(dsParam, "https://") {
			promURL = dsParam
		} else {
			var dsList []DataSource
			t.db.Model(&DataSource{}).Where("enabled = ?", true).Find(&dsList)
			for _, ds := range dsList {
				if ds.Name == dsParam {
					promURL = ds.URL
					break
				}
			}
		}
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

	go func() {
		dataCollector := metrics.NewCollectorWithURL(client.API, t.config, promURL)
		data, err := dataCollector.CollectMetrics()
		if err != nil {
			t.tasksMu.Lock()
			task.Status = "failed"
			task.Message = fmt.Sprintf("收集指标失败: %v", err)
			t.tasksMu.Unlock()
			return
		}
		data.Datasource = promURL

		reportPath, err := report.GenerateReport(*data)
		if err != nil {
			t.tasksMu.Lock()
			task.Status = "failed"
			task.Message = fmt.Sprintf("生成报告失败: %v", err)
			t.tasksMu.Unlock()
			return
		}

		t.tasksMu.Lock()
		task.Status = "completed"
		task.Message = "巡检完成"
		task.ReportURL = "/api/promai/reports/" + reportPath[len("reports/"):]
		t.tasksMu.Unlock()
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

func (t *QueryTaskTool) GetName() string         { return "query_task" }
func (t *QueryTaskTool) GetLabel() string        { return "查询任务" }
func (t *QueryTaskTool) GetDescription() string  { return "查询巡检任务的执行进度和结果" }

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
		lines = append(lines, fmt.Sprintf("报告: %s", task.ReportURL))
	}
	lines = append(lines, fmt.Sprintf("创建: %s", task.CreatedAt.Format("2006-01-02 15:04:05")))

	return &agent.AgentToolResult{
		Content: []ai.ContentBlock{ai.NewTextContentBlock(strings.Join(lines, "\n"))},
	}, nil
}
