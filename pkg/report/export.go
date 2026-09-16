package report

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// ExportMetric 描述单条巡检指标数据（用于生成可导出的报告内容）
type ExportMetric struct {
	MetricName    string
	TypeName      string
	Status        string
	Value         float64
	Unit          string
	Threshold     float64
	ThresholdType string
	Labels        map[string]string
}

// ExportReport 描述生成可导出报告（Markdown / Word）所需的全部信息
type ExportReport struct {
	Title      string
	Datasource string
	CreatedAt  time.Time
	Status     string
	Content    string // 用户在线编辑保存的 Markdown 内容（为空时自动生成默认模板）
	Metrics    []ExportMetric
}

func formatValue(v float64) string {
	f := fmt.Sprintf("%.2f", v)
	f = strings.TrimRight(f, "0")
	f = strings.TrimRight(f, ".")
	if f == "" || f == "-" {
		return "0"
	}
	return f
}

func formatThreshold(m ExportMetric) string {
	val := formatValue(m.Threshold)
	op := ""
	switch m.ThresholdType {
	case "gt", ">":
		op = ">"
	case "lt", "<":
		op = "<"
	case "gte", ">=":
		op = ">="
	case "lte", "<=":
		op = "<="
	default:
		op = m.ThresholdType
	}
	if op != "" {
		val = op + " " + val
	}
	if m.Unit != "" {
		val = val + " " + m.Unit
	}
	return strings.TrimSpace(val)
}

func formatLabel(m ExportMetric) string {
	keys := make([]string, 0, len(m.Labels))
	for k := range m.Labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=%s", k, m.Labels[k]))
	}
	if len(parts) == 0 {
		return "-"
	}
	return strings.Join(parts, ", ")
}

// GenerateDefaultMarkdown 根据报告数据生成默认的 Markdown 报告内容（可在线编辑后保存）
func GenerateDefaultMarkdown(r ExportReport) string {
	title := strings.TrimSpace(r.Title)
	if title == "" {
		title = "系统巡检报告"
	}
	createdAt := r.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now()
	}

	total := len(r.Metrics)
	critical, warning, normal := 0, 0, 0
	for _, m := range r.Metrics {
		switch m.Status {
		case "critical":
			critical++
		case "warning":
			warning++
		default:
			normal++
		}
	}
	statusText := map[string]string{
		"success": "正常",
		"warning": "告警",
		"danger":  "高危",
	}[r.Status]
	if statusText == "" {
		if critical > 0 {
			statusText = "高危"
		} else if warning > 0 {
			statusText = "告警"
		} else {
			statusText = "正常"
		}
	}

	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n", title)
	b.WriteString("## 一、报告信息\n\n")
	b.WriteString("| 项目 | 内容 |\n")
	b.WriteString("| --- | --- |\n")
	fmt.Fprintf(&b, "| 报告名称 | %s |\n", title)
	fmt.Fprintf(&b, "| 数据源 | %s |\n", r.Datasource)
	fmt.Fprintf(&b, "| 巡检时间 | %s |\n", createdAt.Format("2006-01-02 15:04:05"))
	fmt.Fprintf(&b, "| 巡检状态 | %s |\n", statusText)

	b.WriteString("\n## 二、巡检摘要\n\n")
	b.WriteString("| 指标 | 数量 |\n")
	b.WriteString("| --- | --- |\n")
	fmt.Fprintf(&b, "| 总指标数 | %d |\n", total)
	fmt.Fprintf(&b, "| 严重异常 | %d |\n", critical)
	fmt.Fprintf(&b, "| 告警 | %d |\n", warning)
	fmt.Fprintf(&b, "| 正常 | %d |\n", normal)

	b.WriteString("\n## 三、巡检明细\n")

	// 按类型分组，保持稳定输出顺序
	types := make([]string, 0)
	metricsByType := make(map[string][]ExportMetric)
	for _, m := range r.Metrics {
		if _, ok := metricsByType[m.TypeName]; !ok {
			types = append(types, m.TypeName)
		}
		metricsByType[m.TypeName] = append(metricsByType[m.TypeName], m)
	}
	sort.Strings(types)

	for _, t := range types {
		ms := metricsByType[t]
		fmt.Fprintf(&b, "\n### %s\n\n", t)
		b.WriteString("| 指标 | 标签 | 当前值 | 阈值 | 状态 |\n")
		b.WriteString("| --- | --- | --- | --- | --- |\n")
		for _, m := range ms {
			status := GetStatusText(m.Status)
			value := formatValue(m.Value)
			if m.Unit != "" {
				value = value + " " + m.Unit
			}
			fmt.Fprintf(&b, "| %s | %s | %s | %s | %s |\n",
				m.MetricName, formatLabel(m), value, formatThreshold(m), status)
		}
	}

	b.WriteString("\n## 四、巡检结论\n\n")
	b.WriteString("（请在此处填写巡检结论、问题说明及处置建议）\n")
	b.WriteString("\n## 五、备注\n\n")
	b.WriteString("（补充说明）\n")
	return b.String()
}

// BuildMarkdown 返回最终导出的 Markdown 内容：
// 若存在用户编辑保存的内容则直接使用，否则生成默认模板。
func BuildMarkdown(r ExportReport) string {
	content := strings.TrimSpace(r.Content)
	if content != "" {
		return content
	}
	return GenerateDefaultMarkdown(r)
}