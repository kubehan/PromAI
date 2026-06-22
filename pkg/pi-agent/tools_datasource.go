package piagent

import (
	"context"
	"fmt"
	"log"
	"strings"

	agent "github.com/jay-y/pi/pkg/ai-agent"
	"github.com/jay-y/pi/pkg/ai"
)

type ListDatasourcesTool struct {
	db DB
}

func (t *ListDatasourcesTool) GetName() string         { return "list_datasources" }
func (t *ListDatasourcesTool) GetLabel() string        { return "列出数据源" }
func (t *ListDatasourcesTool) GetDescription() string  { return "列出所有数据源及其基本信息" }

func (t *ListDatasourcesTool) GetParameters() map[string]any {
	return map[string]any{
		"type":       "object",
		"properties": map[string]any{},
	}
}

func (t *ListDatasourcesTool) Execute(ctx context.Context, params map[string]any, onUpdate func(*agent.AgentToolResult)) (*agent.AgentToolResult, error) {
	var dsList []DataSource
	t.db.Model(&DataSource{}).Order("is_default desc, enabled desc, name asc").Find(&dsList)
	log.Printf("[PiAgent] 工具调用: list_datasources -> %d 个", len(dsList))

	lines := []string{}
	lines = append(lines, fmt.Sprintf("🔌 数据源列表（共 %d 个）", len(dsList)))
	lines = append(lines, "")

	if len(dsList) == 0 {
		lines = append(lines, "暂无数据源")
	} else {
		for _, ds := range dsList {
			status := "🟢 启用"
			if !ds.Enabled {
				status = "🔴 禁用"
			}
			def := ""
			if ds.IsDefault {
				def = " [默认]"
			}
			tmpl := ""
			if ds.TemplateID != nil {
				tmpl = fmt.Sprintf(" [模板ID: %d]", *ds.TemplateID)
			}
			lines = append(lines, fmt.Sprintf("  • %s%s — %s %s%s", ds.Name, def, ds.URL, status, tmpl))
		}
	}

	return &agent.AgentToolResult{
		Content: []ai.ContentBlock{ai.NewTextContentBlock(strings.Join(lines, "\n"))},
	}, nil
}

// DB is a subset of *gorm.DB used by tools to avoid direct gorm import
type DB interface {
	Model(value any) DB
	Where(query any, args ...any) DB
	Order(value any) DB
	Count(count *int64) DB
	Find(dest any, conds ...any) DB
	First(dest any, conds ...any) DB
	Offset(offset int) DB
	Limit(limit int) DB
	Create(value any) DB
	Updates(values any) DB
	Error() error
}

type DataSource struct {
	ID             uint   `json:"id"`
	Name           string `json:"name"`
	URL            string `json:"url"`
	Username       string `json:"username"`
	Password       string `json:"password"`
	IsDefault      bool   `json:"is_default"`
	Enabled        bool   `json:"enabled"`
	TemplateID     *uint  `json:"template_id"`
	NotifyChannels string `json:"notify_channels"`
}

func (DataSource) TableName() string { return "data_sources" }
