package metrics

import (
	"context"
	"fmt"
	"html/template"
	"log"
	"math"
	"time"

	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"

	"PromAI/pkg/config"
	"PromAI/pkg/prometheus"
	"PromAI/pkg/report"
)

// Collector 处理指标收集
type Collector struct {
	Client        PrometheusAPI
	config        *config.Config
	prometheusURL string
}

type PrometheusAPI interface {
	Query(ctx context.Context, query string, ts time.Time, opts ...v1.Option) (model.Value, v1.Warnings, error)
	QueryRange(ctx context.Context, query string, r v1.Range, opts ...v1.Option) (model.Value, v1.Warnings, error)
}

// NewCollector 创建新的收集器
func NewCollector(client PrometheusAPI, config *config.Config) *Collector {
	return &Collector{
		Client:        client,
		config:        config,
		prometheusURL: config.PrometheusURL,
	}
}

// NewCollectorWithURL 创建带有指定URL的收集器
func NewCollectorWithURL(client PrometheusAPI, config *config.Config, prometheusURL string) *Collector {
	return &Collector{
		Client:        client,
		config:        config,
		prometheusURL: prometheusURL,
	}
}

// UpdatePrometheusURL 更新Prometheus URL和客户端
func (c *Collector) UpdatePrometheusURL(url,username,password string) error {
	client, err := prometheus.NewClient(url,username,password)
	if err != nil {
		return fmt.Errorf("creating prometheus client: %w", err)
	}
	c.Client = client.API
	c.prometheusURL = url
	return nil
}

// CollectMetrics 收集指标数据
func (c *Collector) CollectMetrics() (*report.ReportData, error) {
	log.Printf("[DEBUG] 开始收集指标，使用数据源: %s", c.prometheusURL)
	ctx := context.Background()

	data := &report.ReportData{
		Timestamp:    time.Now(),
		MetricGroups: make(map[string]*report.MetricGroup),
		ChartData:    make(map[string]template.JS),
		Project:      c.config.ProjectName,
		Datasource:   c.prometheusURL, //在CollectMetrics函数开始时设置默认数据源
	}

	for _, metricType := range c.config.MetricTypes {
		group := &report.MetricGroup{
			Type:          metricType.Type,
			MetricsByName: make(map[string][]report.MetricData),
		}
		data.MetricGroups[metricType.Type] = group

		for _, metric := range metricType.Metrics {
			log.Printf("[DEBUG] 查询指标 %s, 查询语句: %s, 数据源: %s", metric.Name, metric.Query, c.prometheusURL)
			result, _, err := c.Client.Query(ctx, metric.Query, time.Now())
			if err != nil {
				log.Printf("警告: 查询指标 %s 失败: %v", metric.Name, err)
				continue
			}
			log.Printf("指标 [%s] 查询结果: %+v", metric.Name, result)

			switch v := result.(type) {
			case model.Vector:
				metrics := make([]report.MetricData, 0, len(v))
				for _, sample := range v {
					log.Printf("指标 [%s] 原始数据: %+v, 值: %+v", metric.Name, sample.Metric, sample.Value)

					availableLabels := make(map[string]string)
					for labelName, labelValue := range sample.Metric {
						availableLabels[string(labelName)] = string(labelValue)
					}

					labels := make([]report.LabelData, 0, len(metric.Labels))
					for configLabel, configAlias := range metric.Labels {
						labelValue := "-"
						if rawValue, exists := availableLabels[configLabel]; exists && rawValue != "" {
							labelValue = rawValue
						} else {
							log.Printf("警告: 指标 [%s] 标签 [%s] 缺失或为空", metric.Name, configLabel)
						}

						labels = append(labels, report.LabelData{
							Name:  configLabel,
							Alias: configAlias,
							Value: labelValue,
						})
					}

					value := float64(sample.Value)

					// 检查值是否有效（非NaN且有限）
					if math.IsNaN(value) || math.IsInf(value, 0) {
						log.Printf("警告: 指标 [%s] 返回无效值 (NaN/Inf): %v, 跳过该条记录", metric.Name, value)
						continue
					}

					metricData := report.MetricData{
						Name:        metric.Name,
						Description: metric.Description,
						Value:       value,
						Threshold:   metric.Threshold,
						Unit:        metric.Unit,
						Status:      getStatus(value, metric.Threshold, metric.ThresholdType, metric.ThresholdStatus),
						StatusText:  report.GetStatusText(getStatus(value, metric.Threshold, metric.ThresholdType, metric.ThresholdStatus)),
						Timestamp:   time.Now(),
						Labels:      labels,
					}

					if err := validateMetricData(metricData, metric.Labels); err != nil {
						log.Printf("警告: 指标 [%s] 数据验证失败: %v", metric.Name, err)
						continue
					}

					metrics = append(metrics, metricData)
				}
				group.MetricsByName[metric.Name] = metrics
			}
		}
	}
	return data, nil
}

// validateMetricData 验证指标数据的完整性
func validateMetricData(data report.MetricData, configLabels map[string]string) error {
	if len(data.Labels) != len(configLabels) {
		return fmt.Errorf("标签数量不匹配: 期望 %d, 实际 %d",
			len(configLabels), len(data.Labels))
	}

	labelMap := make(map[string]bool)
	for _, label := range data.Labels {
		if _, exists := configLabels[label.Name]; !exists {
			return fmt.Errorf("发现未配置的标签: %s", label.Name)
		}
		if label.Value == "" {
			return fmt.Errorf("标签 %s 值为空", label.Name)
		}
		labelMap[label.Name] = true
	}

	return nil
}

// getStatus 获取状态 - 支持threshold_status配置
func getStatus(value, threshold float64, thresholdType, thresholdStatus string) string {
	if thresholdType == "" {
		thresholdType = "greater"
	}
	if thresholdStatus == "" {
		thresholdStatus = "critical" // 默认阈值触发时为严重
	}

	// 判断是否触发阈值条件
	triggered := false
	switch thresholdType {
	case "greater":
		triggered = value > threshold
	case "greater_equal":
		triggered = value >= threshold
	case "less":
		triggered = value < threshold
	case "less_equal":
		triggered = value <= threshold
	case "equal":
		triggered = value == threshold
	}

	if triggered {
		// 阈值条件触发，返回配置的状态
		return thresholdStatus
	}

	// 未触发阈值条件，判断是否接近阈值（警告状态）
	warningTriggered := false
	switch thresholdType {
	case "greater":
		warningTriggered = value >= threshold*0.9
	case "greater_equal":
		warningTriggered = value >= threshold*0.9
	case "less":
		warningTriggered = value <= threshold*0.9
	case "less_equal":
		warningTriggered = value <= threshold*0.9
	case "equal":
		warningTriggered = math.Abs(value-threshold) <= threshold*0.2
	}

	if warningTriggered {
		return "warning"
	}

	// 既未触发阈值也未接近阈值，根据threshold_status决定默认状态
	if thresholdStatus == "critical" {
		return "normal"
	} else {
		return "critical"
	}
}

// validateLabels 验证标签数据的完整性
func validateLabels(labels []report.LabelData) bool {
	for _, label := range labels {
		if label.Value == "" || label.Value == "-" {
			return false
		}
	}
	return true
}
