package report

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"math"
	"os"
	"sort"
	"time"
)

type LabelData struct {
	Name  string // 原始标签名
	Alias string // 显示的别名
	Value string // 标签值
}
type GroupStats struct {
	MaxValue      float64
	MinValue      float64
	Average       float64
	AlertCount    int // 告警数量
	CriticalCount int // 严重告警数量
	WarningCount  int // 警告数量
	TotalCount    int // 总指标数
}
type MetricData struct {
	Instance      string
	Name          string
	Description   string
	Value         float64
	Threshold     float64
	ThresholdType string
	Unit          string
	Status        string
	StatusText    string
	Timestamp     time.Time
	Labels        []LabelData // 改用结构化的标签数据
}

type MetricGroup struct {
	Type          string
	MetricsByName map[string][]MetricData
	Stats         GroupStats // 替换原来的 Average
}
type ReportData struct {
	Timestamp    time.Time
	MetricGroups map[string]*MetricGroup
	ChartData    map[string]template.JS
	Project      string
	Datasource   string
	ReportUrl    string
}

func GetStatusText(status string) string {
	switch status {
	case "critical":
		return "严重"
	case "warning":
		return "警告"
	default:
		return "正常"
	}
}

func GenerateReport(data ReportData) (string, error) {
	// 计算每个组的统计信息
	for _, group := range data.MetricGroups {
		stats := GroupStats{
			MinValue: math.MaxFloat64,
		}

		for _, metrics := range group.MetricsByName {
			for _, metric := range metrics {
				// 检查metric.Value是否为有效数值
				if !math.IsNaN(metric.Value) && !math.IsInf(metric.Value, 0) {
					// 更新最大最小值
					stats.MaxValue = math.Max(stats.MaxValue, metric.Value)
					stats.MinValue = math.Min(stats.MinValue, metric.Value)
					stats.TotalCount++
				}

				// 累加值用于计算平均值
				// stats.Average += metric.Value

				// 统计告警数量
				switch metric.Status {
				case "warning":
					stats.WarningCount++
					stats.AlertCount++
				case "critical":
					stats.CriticalCount++
					stats.AlertCount++
				}
			}
		}

		// 处理没有有效数据的情况
		if stats.TotalCount == 0 {
			stats.MaxValue = 0
			stats.MinValue = 0
		} else {
			// 确保最小值没有被设置为math.MaxFloat64（当所有值都相同时）
			if stats.MinValue == math.MaxFloat64 {
				stats.MinValue = stats.MaxValue
			}
		}

		// 计算平均值 平均值无意义，先暂时取消
		// if stats.TotalCount > 0 {
		// 	stats.Average = stats.Average / float64(stats.TotalCount)
		// }
		group.Stats = stats
	}

	// 处理图表数据
	allLabels := make(map[string]bool)      // 用于存储所有唯一的标签值
	chartData := make(map[string][]float64) // 用于存储图表数据
	// 收集所有唯一的标签值和准备图表数据
	labelValuesByMetric := make(map[string]map[string]bool) // 按指标存储唯一标签值

	// 第一次遍历收集每个指标的唯一标签值
	for _, group := range data.MetricGroups {
		for metricName, metrics := range group.MetricsByName {
			metricKey := fmt.Sprintf("%s_%s", group.Type, metricName)
			labelValuesByMetric[metricKey] = make(map[string]bool)
			// log.Println("指标组：", group.Type, "指标：", metricName, "指标键：", metricKey)
			for _, metric := range metrics {
				for _, label := range metric.Labels {
					labelValuesByMetric[metricKey][label.Value] = true
					// log.Println("指标组：", group.Type, "指标：", metricName, "指标键：", metricKey, "标签值：", label.Value)
					allLabels[label.Value] = true

				}
			}
		}
	}

	// 第二次遍历按标签值顺序生成图表数据
	for _, group := range data.MetricGroups {
		for metricName, metrics := range group.MetricsByName {
			metricKey := fmt.Sprintf("%s_%s", group.Type, metricName)
			metricValues := make(map[string]float64)
			// log.Println("指标类型：", group.Type, "指标名称：", metricName, "指标Key：", metricKey)

			// 初始化所有标签值对应的指标值为0
			for labelValue := range labelValuesByMetric[metricKey] {

				metricValues[labelValue] = 0

				log.Println("标签值：", labelValue, "指标值：", metricValues[labelValue])
			}

			// 填充实际的指标值
			for _, metric := range metrics {
				if len(metric.Labels) > 0 {
					metricValues[metric.Labels[0].Value] = metric.Value
				}
			}

			// 按标签值顺序添加到图表数据
			chartData[metricKey] = make([]float64, 0)
			for labelValue := range labelValuesByMetric[metricKey] {
				chartData[metricKey] = append(chartData[metricKey], metricValues[labelValue])
			}
			// log.Println("图表数据：", metricKey, "图表数据值：", chartData[metricKey])
		}
	}

	// 转换标签为数组并排序
	labels := make([]string, 0, len(allLabels))
	for label := range allLabels {
		labels = append(labels, label)
	}
	sort.Strings(labels)

	// 转换为JSON
	labelsJSON, _ := json.Marshal(labels)
	data.ChartData["labels"] = template.JS(labelsJSON)
	// log.Println("标签：", labels)
	// 为每个指标生成图表数据
	for key, values := range chartData {
		valuesJSON, _ := json.Marshal(values)
		data.ChartData[key] = template.JS(valuesJSON)
	}

	// 生成报告
	tmpl, err := template.ParseFiles("templates/report.html")
	if err != nil {
		return "", fmt.Errorf("parsing template: %w", err)
	}

	// 创建输出文件
	filename := fmt.Sprintf("reports/inspection_report_%s.html", time.Now().Format("20060102_150405"))
	file, err := os.Create(filename)
	if err != nil {
		return "", fmt.Errorf("creating output file: %w", err)
	}
	defer file.Close()

	// 执行模板
	if err := tmpl.Execute(file, data); err != nil {
		return "", fmt.Errorf("executing template: %w", err)
	}

	// log.Println("Report generated successfully:", filename)
	log.Printf("项目[%s]报告生成成功: %s", data.Project, filename)

	return filename, nil // 添加返回语句
}
