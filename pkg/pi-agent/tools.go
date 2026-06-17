package piagent

import (
	"PromAI/pkg/config"
	"PromAI/pkg/metrics"
	agent "github.com/jay-y/pi/pkg/ai-agent"
)

func CreateAllTools(cfg *config.Config, collector *metrics.Collector, db DB) []agent.AgentTool {
	inspectTool := NewTriggerInspectTool(cfg, collector, db)

	configs := []agent.AgentToolConfig{
		&QueryMetricsTool{config: cfg, db: db},
		&AnalyzeAlertTool{config: cfg, db: db},
		&ListReportsTool{db: db},
		&GetReportDetailTool{db: db},
		&ListDatasourcesTool{db: db},
		inspectTool,
		&QueryTaskTool{parent: inspectTool},
		NewPushReportTool(cfg, db),
	}

	tools := make([]agent.AgentTool, len(configs))
	for i, c := range configs {
		tools[i] = agent.NewAgentTool(c)
	}
	return tools
}
