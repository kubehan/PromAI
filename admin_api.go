package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"PromAI/pkg/config"
	"PromAI/pkg/database"
	"PromAI/pkg/metrics"
	"PromAI/pkg/notify"
	"PromAI/pkg/prometheus"
	"PromAI/pkg/report"

	"github.com/prometheus/common/model"
	"github.com/robfig/cron/v3"
)

type AdminAPI struct {
	collector *metrics.Collector
	config    *config.Config
}

func NewAdminAPI(collector *metrics.Collector, cfg *config.Config) *AdminAPI {
	return &AdminAPI{collector: collector, config: cfg}
}

func (a *AdminAPI) RegisterHandlers(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/datasources", a.handleDataSources)
	mux.HandleFunc("/api/v1/datasources/", a.handleDataSourceByID)
	mux.HandleFunc("/api/v1/notifications", a.handleNotifications)
	mux.HandleFunc("/api/v1/notifications/", a.handleNotificationByID)
	mux.HandleFunc("/api/v1/cronjobs", a.handleCronJobs)
	mux.HandleFunc("/api/v1/cronjobs/", a.handleCronJobByID)
	mux.HandleFunc("/api/v1/reports", a.handleReports)
	mux.HandleFunc("/api/v1/reports/", a.handleReportByID)
	mux.HandleFunc("/api/v1/metrics/types", a.handleMetricTypes)
	mux.HandleFunc("/api/v1/metrics/types/", a.handleMetricTypeByID)
	mux.HandleFunc("/api/v1/metrics/configs", a.handleMetricConfigs)
	mux.HandleFunc("/api/v1/metrics/configs/", a.handleMetricConfigByID)
	mux.HandleFunc("/api/v1/metrics/validate", a.handleValidatePromQL)
	mux.HandleFunc("/api/v1/templates", a.handleTemplates)
	mux.HandleFunc("/api/v1/templates/", a.handleTemplateByID)
	mux.HandleFunc("/api/v1/settings", a.handleSettings)
	mux.HandleFunc("/api/v1/inspect", a.handleInspect)
	mux.HandleFunc("/api/v1/datasources/import", a.handleImportDatasource)
	mux.HandleFunc("/api/v1/notifications/test", a.handleTestNotification)
	mux.HandleFunc("/api/v1/dashboard/stats", a.handleDashboardStats)
	mux.HandleFunc("/api/v1/dashboard/health", a.handleDashboardHealth)
	mux.HandleFunc("/api/v1/datasources/apply-template", a.handleApplyTemplate)

	log.Printf("[AdminAPI] 管理接口已注册")
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

func getLastPathID(path string) (uint, error) {
	parts := strings.Split(strings.TrimRight(path, "/"), "/")
	if len(parts) == 0 {
		return 0, fmt.Errorf("no path segments")
	}
	idStr := parts[len(parts)-1]
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid id: %s", idStr)
	}
	return uint(id), nil
}

func parseParentID(path string) (uint, error) {
	parts := strings.Split(strings.TrimRight(path, "/"), "/")
	if len(parts) < 2 {
		return 0, fmt.Errorf("no parent id")
	}
	idStr := parts[len(parts)-2]
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid parent id: %s", idStr)
	}
	return uint(id), nil
}

func (a *AdminAPI) handleDataSources(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		var ds []database.DataSource
		database.DB.Order("is_default desc, name asc").Find(&ds)
		writeJSON(w, ds)
	case "POST":
		var d database.DataSource
		if err := json.NewDecoder(r.Body).Decode(&d); err != nil {
			writeError(w, 400, "请求体格式错误")
			return
		}
		if d.Name == "" || d.URL == "" {
			writeError(w, 400, "名称和URL不能为空")
			return
		}
		database.DB.Create(&d)
		w.WriteHeader(201)
		writeJSON(w, d)
	default:
		writeError(w, 405, "不支持的请求方法")
	}
}

func (a *AdminAPI) handleDataSourceByID(w http.ResponseWriter, r *http.Request) {
	// Check for sub-routes first
	if strings.HasSuffix(r.URL.Path, "/bind-metrics") {
		a.handleBindMetrics(w, r)
		return
	}
	id, err := getLastPathID(r.URL.Path)
	if err != nil {
		writeError(w, 400, err.Error())
		return
	}
	switch r.Method {
	case "GET":
		var d database.DataSource
		if database.DB.First(&d, id).Error != nil {
			writeError(w, 404, "数据源不存在")
			return
		}
		writeJSON(w, d)
	case "PUT":
		var d database.DataSource
		if database.DB.First(&d, id).Error != nil {
			writeError(w, 404, "数据源不存在")
			return
		}
		var upd database.DataSource
		if err := json.NewDecoder(r.Body).Decode(&upd); err != nil {
			writeError(w, 400, "请求体格式错误")
			return
		}
		upd.ID = d.ID
		upd.CreatedAt = d.CreatedAt
		database.DB.Save(&upd)
		writeJSON(w, upd)
	case "DELETE":
		database.DB.Delete(&database.DataSource{}, id)
		w.WriteHeader(204)
	default:
		writeError(w, 405, "不支持的请求方法")
	}
}

func (a *AdminAPI) handleImportDatasource(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		writeError(w, 405, "不支持的请求方法")
		return
	}
	var req struct {
		YAMLContent string `json:"yaml_content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.YAMLContent == "" {
		writeError(w, 400, "请提供YAML内容")
		return
	}

	imported := 0
	lines := strings.Split(req.YAMLContent, "\n")
	var currentDS *database.DataSource
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "- name:") {
			if currentDS != nil && currentDS.Name != "" && currentDS.URL != "" {
				database.DB.Create(&currentDS)
				imported++
			}
			currentDS = &database.DataSource{}
			currentDS.Name = strings.TrimSpace(strings.TrimPrefix(trimmed, "- name:"))
		} else if currentDS != nil {
			if strings.HasPrefix(trimmed, "url:") {
				currentDS.URL = strings.TrimSpace(strings.TrimPrefix(trimmed, "url:"))
			} else if strings.HasPrefix(trimmed, "username:") {
				currentDS.Username = strings.TrimSpace(strings.TrimPrefix(trimmed, "username:"))
			} else if strings.HasPrefix(trimmed, "password:") {
				currentDS.Password = strings.TrimSpace(strings.TrimPrefix(trimmed, "password:"))
			}
		}
	}
	if currentDS != nil && currentDS.Name != "" && currentDS.URL != "" {
		database.DB.Create(&currentDS)
		imported++
	}

	writeJSON(w, map[string]interface{}{"imported": imported, "message": fmt.Sprintf("成功导入 %d 个数据源", imported)})
}

func (a *AdminAPI) handleApplyTemplate(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		writeError(w, 405, "不支持的请求方法")
		return
	}
	var req struct {
		DatasourceID uint `json:"datasource_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.DatasourceID == 0 {
		writeError(w, 400, "请提供数据源ID")
		return
	}

	var ds database.DataSource
	if database.DB.First(&ds, req.DatasourceID).Error != nil {
		writeError(w, 404, "数据源不存在")
		return
	}

	// 查找全局模板
	var globalTmpl database.InspectionTemplate
	if err := database.DB.Where("name = ?", "全局模板").First(&globalTmpl).Error; err != nil {
		writeError(w, 500, "全局模板不存在，请重新初始化数据库")
		return
	}

	// 设置数据源的 template_id 为全局模板
	database.DB.Model(&ds).Update("template_id", globalTmpl.ID)

	var count int64
	database.DB.Model(&database.InspectionTemplateMetric{}).Where("template_id = ?", globalTmpl.ID).Count(&count)

	writeJSON(w, map[string]interface{}{
		"message": fmt.Sprintf("已为数据源「%s」绑定全局模板（%d 个指标）", ds.Name, count),
	})
}

func (a *AdminAPI) handleNotifications(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		var nc []database.NotificationChannel
		database.DB.Order("channel_type asc").Find(&nc)
		writeJSON(w, nc)
	case "POST":
		var n database.NotificationChannel
		if err := json.NewDecoder(r.Body).Decode(&n); err != nil {
			writeError(w, 400, "请求体格式错误")
			return
		}
		if n.ChannelType == "" || n.Name == "" {
			writeError(w, 400, "渠道类型和名称不能为空")
			return
		}
		database.DB.Create(&n)
		w.WriteHeader(201)
		writeJSON(w, n)
	default:
		writeError(w, 405, "不支持的请求方法")
	}
}

func (a *AdminAPI) handleNotificationByID(w http.ResponseWriter, r *http.Request) {
	if strings.HasSuffix(r.URL.Path, "/test") {
		a.handleTestNotification(w, r)
		return
	}
	id, err := getLastPathID(r.URL.Path)
	if err != nil {
		writeError(w, 400, err.Error())
		return
	}
	switch r.Method {
	case "GET":
		var n database.NotificationChannel
		if database.DB.First(&n, id).Error != nil {
			writeError(w, 404, "通知渠道不存在")
			return
		}
		writeJSON(w, n)
	case "PUT":
		var n database.NotificationChannel
		if database.DB.First(&n, id).Error != nil {
			writeError(w, 404, "通知渠道不存在")
			return
		}
		var upd database.NotificationChannel
		if err := json.NewDecoder(r.Body).Decode(&upd); err != nil {
			writeError(w, 400, "请求体格式错误")
			return
		}
		upd.ID = n.ID
		upd.CreatedAt = n.CreatedAt
		database.DB.Save(&upd)
		writeJSON(w, upd)
	case "DELETE":
		database.DB.Delete(&database.NotificationChannel{}, id)
		w.WriteHeader(204)
	default:
		writeError(w, 405, "不支持的请求方法")
	}
}

func (a *AdminAPI) handleCronJobs(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		var jobs []database.CronJob
		database.DB.Order("created_at desc").Find(&jobs)
		writeJSON(w, jobs)
	case "POST":
		var j database.CronJob
		if err := json.NewDecoder(r.Body).Decode(&j); err != nil {
			writeError(w, 400, "请求体格式错误")
			return
		}
		if j.Name == "" || j.Schedule == "" {
			writeError(w, 400, "名称和调度表达式不能为空")
			return
		}
		if _, err := cron.ParseStandard(j.Schedule); err != nil {
			writeError(w, 400, "调度表达式格式无效: "+err.Error())
			return
		}
		database.DB.Create(&j)
		startGlobalScheduler(a.config, a.collector)
		w.WriteHeader(201)
		writeJSON(w, j)
	default:
		writeError(w, 405, "不支持的请求方法")
	}
}

func (a *AdminAPI) handleCronJobByID(w http.ResponseWriter, r *http.Request) {
	id, err := getLastPathID(r.URL.Path)
	if err != nil {
		writeError(w, 400, err.Error())
		return
	}
	switch r.Method {
	case "GET":
		var j database.CronJob
		if database.DB.First(&j, id).Error != nil {
			writeError(w, 404, "定时任务不存在")
			return
		}
		writeJSON(w, j)
	case "PUT":
		var j database.CronJob
		if database.DB.First(&j, id).Error != nil {
			writeError(w, 404, "定时任务不存在")
			return
		}
		var upd database.CronJob
		if err := json.NewDecoder(r.Body).Decode(&upd); err != nil {
			writeError(w, 400, "请求体格式错误")
			return
		}
		upd.ID = j.ID
		upd.CreatedAt = j.CreatedAt
		database.DB.Save(&upd)
		startGlobalScheduler(a.config, a.collector)
		writeJSON(w, upd)
	case "DELETE":
		database.DB.Delete(&database.CronJob{}, id)
		startGlobalScheduler(a.config, a.collector)
		w.WriteHeader(204)
	default:
		writeError(w, 405, "不支持的请求方法")
	}
}

func (a *AdminAPI) handleReports(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		var reports []database.ReportRecord
		database.DB.Order("created_at desc").Find(&reports)
		writeJSON(w, reports)
	default:
		writeError(w, 405, "不支持的请求方法")
	}
}

func (a *AdminAPI) handleReportByID(w http.ResponseWriter, r *http.Request) {
	id, err := getLastPathID(r.URL.Path)
	if err != nil {
		writeError(w, 400, err.Error())
		return
	}
	switch r.Method {
	case "DELETE":
		var rec database.ReportRecord
		if database.DB.First(&rec, id).Error != nil {
			writeError(w, 404, "报告不存在")
			return
		}
		os.Remove(rec.FilePath)
		database.DB.Delete(&database.ReportRecord{}, id)
		w.WriteHeader(204)
	default:
		writeError(w, 405, "不支持的请求方法")
	}
}

func (a *AdminAPI) handleMetricTypes(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		filterDS := r.URL.Query().Get("datasource_id")

		// 如果按数据源筛选，检查数据源是否绑定了巡检模板
		var templateConfigIDs []uint
		var tmplID *uint
		if filterDS != "" {
			var ds database.DataSource
			if database.DB.First(&ds, filterDS).Error == nil && ds.TemplateID != nil {
				tmplID = ds.TemplateID
				var links []database.InspectionTemplateMetric
				database.DB.Where("template_id = ?", *ds.TemplateID).Find(&links)
				for _, l := range links {
					templateConfigIDs = append(templateConfigIDs, l.MetricConfigID)
				}
			}
		}

		var mTypes []database.MetricType
		database.DB.Order("sort_order asc, id asc").Find(&mTypes)
		for i := range mTypes {
			configs := []database.MetricConfig{}
			q := database.DB.Where("metric_type_id = ?", mTypes[i].ID)
			if len(templateConfigIDs) > 0 {
				q = q.Where("id IN ?", templateConfigIDs)
			} else if filterDS != "" {
				q = q.Where("(datasource_id IS NULL OR datasource_id = ?)", filterDS)
			}
			q.Order("sort_order asc").Find(&configs)
			// 合并模板 override（如果有）
			if tmplID != nil {
				for j := range configs {
					var override database.TemplateMetricOverride
					if database.DB.Where("template_id = ? AND metric_config_id = ?", *tmplID, configs[j].ID).First(&override).Error == nil {
						override.Apply(&configs[j])
					}
				}
			}
			mTypes[i].Configs = configs
		}
		writeJSON(w, mTypes)
	case "POST":
		var mt database.MetricType
		if err := json.NewDecoder(r.Body).Decode(&mt); err != nil {
			writeError(w, 400, "请求体格式错误")
			return
		}
		if mt.TypeName == "" {
			writeError(w, 400, "类型名称不能为空")
			return
		}
		database.DB.Create(&mt)
		w.WriteHeader(201)
		writeJSON(w, mt)
	default:
		writeError(w, 405, "不支持的请求方法")
	}
}

func (a *AdminAPI) handleMetricTypeByID(w http.ResponseWriter, r *http.Request) {
	id, err := getLastPathID(r.URL.Path)
	if err != nil {
		writeError(w, 400, err.Error())
		return
	}
	switch r.Method {
	case "PUT":
		var mt database.MetricType
		if database.DB.First(&mt, id).Error != nil {
			writeError(w, 404, "指标类型不存在")
			return
		}
		var upd database.MetricType
		if err := json.NewDecoder(r.Body).Decode(&upd); err != nil {
			writeError(w, 400, "请求体格式错误")
			return
		}
		upd.ID = mt.ID
		upd.CreatedAt = mt.CreatedAt
		database.DB.Save(&upd)
		writeJSON(w, upd)
	case "DELETE":
		database.DB.Where("metric_type_id = ?", id).Delete(&database.MetricConfig{})
		database.DB.Delete(&database.MetricType{}, id)
		w.WriteHeader(204)
	default:
		writeError(w, 405, "不支持的请求方法")
	}
}

func (a *AdminAPI) handleMetricConfigs(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "POST":
		var mc database.MetricConfig
		if err := json.NewDecoder(r.Body).Decode(&mc); err != nil {
			writeError(w, 400, "请求体格式错误")
			return
		}
		if mc.Name == "" || mc.Query == "" {
			writeError(w, 400, "名称和查询语句不能为空")
			return
		}
		database.DB.Create(&mc)
		w.WriteHeader(201)
		writeJSON(w, mc)
	default:
		writeError(w, 405, "不支持的请求方法")
	}
}

func (a *AdminAPI) handleMetricConfigByID(w http.ResponseWriter, r *http.Request) {
	id, err := getLastPathID(r.URL.Path)
	if err != nil {
		writeError(w, 400, err.Error())
		return
	}
	switch r.Method {
	case "PUT":
		var mc database.MetricConfig
		if database.DB.First(&mc, id).Error != nil {
			writeError(w, 404, "指标配置不存在")
			return
		}
		var upd database.MetricConfig
		if err := json.NewDecoder(r.Body).Decode(&upd); err != nil {
			writeError(w, 400, "请求体格式错误")
			return
		}
		upd.ID = mc.ID
		upd.CreatedAt = mc.CreatedAt
		database.DB.Save(&upd)
		writeJSON(w, upd)
	case "DELETE":
		database.DB.Delete(&database.MetricConfig{}, id)
		w.WriteHeader(204)
	default:
		writeError(w, 405, "不支持的请求方法")
	}
}

func (a *AdminAPI) handleValidatePromQL(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		writeError(w, 405, "不支持的请求方法")
		return
	}
	var req struct {
		DatasourceID uint   `json:"datasource_id"`
		Query        string `json:"query"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Query == "" {
		writeError(w, 400, "请提供 PromQL 查询语句")
		return
	}

	promURL := a.config.PrometheusURL
	promUser := a.config.PrometheusUsername
	promPass := a.config.PrometheusPassword

	if req.DatasourceID > 0 {
		var ds database.DataSource
		if database.DB.First(&ds, req.DatasourceID).Error == nil {
			promURL = ds.URL
			promUser = ds.Username
			promPass = ds.Password
		}
	}

	client, err := prometheus.NewClient(promURL, promUser, promPass)
	if err != nil {
		writeError(w, 500, fmt.Sprintf("创建 Prometheus 客户端失败: %v", err))
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, _, err := client.API.Query(ctx, req.Query, time.Now())
	if err != nil {
		writeJSON(w, map[string]interface{}{
			"valid":   false,
			"error":   err.Error(),
			"message": "PromQL 语法错误或查询失败",
		})
		return
	}

	switch v := result.(type) {
	case model.Vector:
		var samples []map[string]interface{}
		labelSet := make(map[string]bool)

		for _, sample := range v {
			labels := make(map[string]string)
			for k, val := range sample.Metric {
				labels[string(k)] = string(val)
				labelSet[string(k)] = true
			}
			samples = append(samples, map[string]interface{}{
				"labels": labels,
				"value":  float64(sample.Value),
			})
		}

		// Collect all unique label names
		var labels []string
		for l := range labelSet {
			labels = append(labels, l)
		}

		writeJSON(w, map[string]interface{}{
			"valid":     true,
			"type":      "vector",
			"labels":    labels,
			"count":     len(samples),
			"samples":   samples[:min(len(samples), 10)],
		})
	case model.Matrix:
		writeJSON(w, map[string]interface{}{
			"valid":   true,
			"type":    "matrix",
			"message": "查询返回范围数据（Matrix），建议使用即时查询",
		})
	case *model.Scalar:
		writeJSON(w, map[string]interface{}{
			"valid": true,
			"type":  "scalar",
			"value": float64(v.Value),
		})
	default:
		writeJSON(w, map[string]interface{}{
			"valid":   true,
			"type":    "unknown",
			"message": fmt.Sprintf("未知返回类型: %T", result),
		})
	}
}

func (a *AdminAPI) handleSettings(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		var settings []database.AppSetting
		database.DB.Find(&settings)
		m := make(map[string]string)
		for _, s := range settings {
			m[s.Key] = s.Value
		}
		writeJSON(w, m)
	case "PUT":
		var updates map[string]string
		if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
			writeError(w, 400, "请求体格式错误")
			return
		}
	for k, v := range updates {
		var s database.AppSetting
		if database.DB.Where("key = ?", k).First(&s).Error != nil {
			database.DB.Create(&database.AppSetting{Key: k, Value: v})
		} else {
			database.DB.Model(&s).Update("value", v)
		}
		// 同步到内存中的配置
		if k == "cron_schedule" && a.config != nil {
			a.config.CronSchedule = v
		}
	}
	// 如果更新了定时调度，重启调度器
	if _, ok := updates["cron_schedule"]; ok {
		startGlobalScheduler(a.config, a.collector)
	}
	writeJSON(w, updates)
	default:
		writeError(w, 405, "不支持的请求方法")
	}
}

func (a *AdminAPI) handleInspect(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		writeError(w, 405, "不支持的请求方法")
		return
	}
	var req struct {
		DatasourceID    uint   `json:"datasource_id"`
		DatasourceURL   string `json:"datasource_url"`
		WechatBotKey    string `json:"wechat_bot_key"`
		ToUser          string `json:"touser"`
		MetricConfigIDs []uint `json:"metric_config_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, 400, "请求体格式错误")
		return
	}

	promURL := a.config.PrometheusURL
	promUser := a.config.PrometheusUsername
	promPass := a.config.PrometheusPassword

	if req.DatasourceURL != "" {
		promURL = req.DatasourceURL
	} else if req.DatasourceID > 0 {
		var ds database.DataSource
		if database.DB.First(&ds, req.DatasourceID).Error == nil {
			promURL = ds.URL
			promUser = ds.Username
			promPass = ds.Password
		}
	}

	client, err := prometheus.NewClient(promURL, promUser, promPass)
	if err != nil {
		writeError(w, 500, fmt.Sprintf("创建Prometheus客户端失败: %v", err))
		return
	}

	// 优先使用显式指定的指标; 其次 datasource 绑定的模板; 再其次绑定的指标; 最后全部指标
	activeConfig := a.config
	if len(req.MetricConfigIDs) > 0 {
		var selectedConfigs []database.MetricConfig
		database.DB.Where("id IN ?", req.MetricConfigIDs).Find(&selectedConfigs)
		if len(selectedConfigs) > 0 {
			activeConfig = a.buildFilteredConfig(activeConfig, selectedConfigs)
		}
	} else if req.DatasourceID > 0 {
		var ds database.DataSource
		if database.DB.First(&ds, req.DatasourceID).Error == nil {
			if ds.TemplateID != nil {
				// 使用数据源绑定的模板
				var links []database.InspectionTemplateMetric
				database.DB.Where("template_id = ?", *ds.TemplateID).Find(&links)
				if len(links) > 0 {
					cfgIDs := make([]uint, len(links))
					for i, l := range links { cfgIDs[i] = l.MetricConfigID }
					var tmplConfigs []database.MetricConfig
					database.DB.Where("id IN ?", cfgIDs).Find(&tmplConfigs)
					// Apply template metric overrides
					for i := range tmplConfigs {
						var override database.TemplateMetricOverride
						if database.DB.Where("template_id = ? AND metric_config_id = ?", *ds.TemplateID, tmplConfigs[i].ID).First(&override).Error == nil {
							override.Apply(&tmplConfigs[i])
						}
					}
					if len(tmplConfigs) > 0 {
						activeConfig = a.buildFilteredConfig(activeConfig, tmplConfigs)
					}
				}
			} else {
				// 使用数据源绑定的指标
				var dsConfigs []database.MetricConfig
				database.DB.Where("(datasource_id IS NULL OR datasource_id = ?) AND metric_type_id IS NOT NULL", req.DatasourceID).Find(&dsConfigs)
				if len(dsConfigs) > 0 {
					activeConfig = a.buildFilteredConfig(activeConfig, dsConfigs)
				}
			}
		}
	}

	dataCollector := metrics.NewCollector(client.API, activeConfig)
	data, err := dataCollector.CollectMetrics()
	if err != nil {
		writeError(w, 500, fmt.Sprintf("收集指标失败: %v", err))
		return
	}
	data.Datasource = promURL

	reportPath, err := report.GenerateReport(*data)
	if err != nil {
		writeError(w, 500, fmt.Sprintf("生成报告失败: %v", err))
		return
	}

	if req.WechatBotKey != "" {
		alertSummary := notify.CalculateAlertSummary(*data)
		notify.SendWeChatWorkWithWebhook(r.Context(), req.WechatBotKey, "", reportPath, a.config.ProjectName, promURL, alertSummary)
	}

	database.DB.Create(&database.ReportRecord{
		Title:          fmt.Sprintf("巡检报告 - %s", time.Now().Format("2006-01-02 15:04")),
		DatasourceName: promURL,
		FilePath:       reportPath,
		FileSize:       getFileSize(reportPath),
		TotalMetrics:   countMetrics(data),
		AlertCount:     countAlerts(data),
		CriticalCount:  countCritical(data),
		WarningCount:   countWarning(data),
		Status:         getReportStatus(data),
	})

	writeJSON(w, map[string]interface{}{
		"success": true,
		"report":  reportPath,
		"url":     "/api/promai/reports/" + filepath.Base(reportPath),
	})
}

// buildFilteredConfig builds a config.Config from a filtered set of MetricConfig rows
func (a *AdminAPI) buildFilteredConfig(base *config.Config, selectedConfigs []database.MetricConfig) *config.Config {
	typeMap := make(map[uint][]database.MetricConfig)
	for _, c := range selectedConfigs {
		typeMap[c.MetricTypeID] = append(typeMap[c.MetricTypeID], c)
	}
	var filteredTypes []config.MetricType
	for mtID, configs := range typeMap {
		var mt database.MetricType
		if database.DB.First(&mt, mtID).Error != nil {
			continue
		}
		mtCfg := config.MetricType{Type: mt.TypeName}
		for _, c := range configs {
			var labels map[string]string
			if c.LabelsJSON != "" {
				json.Unmarshal([]byte(c.LabelsJSON), &labels)
			}
			mtCfg.Metrics = append(mtCfg.Metrics, config.MetricConfig{
				Name:            c.Name,
				Description:     c.Description,
				Query:           c.Query,
				Threshold:       c.Threshold,
				Unit:            c.Unit,
				Labels:          labels,
				ThresholdType:   c.ThresholdType,
				ThresholdStatus: c.ThresholdStatus,
			})
		}
		filteredTypes = append(filteredTypes, mtCfg)
	}
	cfg := &config.Config{}
	*cfg = *base
	cfg.MetricTypes = filteredTypes
	return cfg
}

func (a *AdminAPI) handleBindMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		writeError(w, 405, "不支持的请求方法")
		return
	}
	id, err := parseParentID(r.URL.Path)
	if err != nil {
		writeError(w, 400, err.Error())
		return
	}
	var req struct {
		MetricConfigIDs []uint `json:"metric_config_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, 400, "请求体格式错误")
		return
	}
	var ds database.DataSource
	if database.DB.First(&ds, id).Error != nil {
		writeError(w, 404, "数据源不存在")
		return
	}
	// Unbind all currently bound, then bind selected
	database.DB.Model(&database.MetricConfig{}).Where("datasource_id = ?", id).Update("datasource_id", nil)
	for _, cfgID := range req.MetricConfigIDs {
		database.DB.Model(&database.MetricConfig{}).Where("id = ?", cfgID).Update("datasource_id", id)
	}
	writeJSON(w, map[string]interface{}{"message": "指标绑定成功"})
}

func (a *AdminAPI) handleTemplates(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		var templates []database.InspectionTemplate
		database.DB.Order("created_at desc").Find(&templates)
		// Attach metric config counts
		type result struct {
			database.InspectionTemplate
			MetricCount int `json:"metric_count"`
		}
		var results []result
		for _, t := range templates {
			var count int64
			database.DB.Model(&database.InspectionTemplateMetric{}).Where("template_id = ?", t.ID).Count(&count)
			results = append(results, result{InspectionTemplate: t, MetricCount: int(count)})
		}
		writeJSON(w, results)
	case "POST":
		var req struct {
			Name        string `json:"name"`
			Description string `json:"description"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
			writeError(w, 400, "请提供模板名称")
			return
		}
		t := database.InspectionTemplate{Name: req.Name, Description: req.Description}
		database.DB.Create(&t)
		w.WriteHeader(201)
		writeJSON(w, t)
	default:
		writeError(w, 405, "不支持的请求方法")
	}
}

func (a *AdminAPI) handleTemplateByID(w http.ResponseWriter, r *http.Request) {
	// Handle sub-routes like /templates/:id/metrics
	if strings.HasSuffix(r.URL.Path, "/metrics") {
		a.handleTemplateMetrics(w, r)
		return
	}
	if strings.Contains(r.URL.Path, "/metrics/") && strings.HasSuffix(r.URL.Path, "/override") {
		a.handleTemplateMetricOverride(w, r)
		return
	}
	if strings.HasSuffix(r.URL.Path, "/inspect") {
		a.handleTemplateInspect(w, r)
		return
	}

	id, err := getLastPathID(r.URL.Path)
	if err != nil {
		writeError(w, 400, err.Error())
		return
	}
	switch r.Method {
	case "GET":
		var t database.InspectionTemplate
		if database.DB.First(&t, id).Error != nil {
			writeError(w, 404, "模板不存在")
			return
		}
		// Get metric config IDs
		var links []database.InspectionTemplateMetric
		database.DB.Where("template_id = ?", id).Find(&links)
		configIDs := make([]uint, len(links))
		for i, l := range links {
			configIDs[i] = l.MetricConfigID
		}
		// Get full configs
		var configs []database.MetricConfig
		if len(configIDs) > 0 {
			database.DB.Where("id IN ?", configIDs).Find(&configs)
		}
		writeJSON(w, map[string]interface{}{
			"template": t,
			"configs":  configs,
		})
	case "PUT":
		var t database.InspectionTemplate
		if database.DB.First(&t, id).Error != nil {
			writeError(w, 404, "模板不存在")
			return
		}
		var upd database.InspectionTemplate
		if err := json.NewDecoder(r.Body).Decode(&upd); err != nil {
			writeError(w, 400, "请求体格式错误")
			return
		}
		upd.ID = t.ID
		upd.CreatedAt = t.CreatedAt
		database.DB.Save(&upd)
		writeJSON(w, upd)
	case "DELETE":
		database.DB.Where("template_id = ?", id).Delete(&database.InspectionTemplateMetric{})
		database.DB.Delete(&database.InspectionTemplate{}, id)
		w.WriteHeader(204)
	default:
		writeError(w, 405, "不支持的请求方法")
	}
}

func (a *AdminAPI) handleTemplateMetrics(w http.ResponseWriter, r *http.Request) {
	id, err := parseParentID(r.URL.Path)
	if err != nil {
		writeError(w, 400, err.Error())
		return
	}
	var t database.InspectionTemplate
	if database.DB.First(&t, id).Error != nil {
		writeError(w, 404, "模板不存在")
		return
	}
	switch r.Method {
	case "GET":
		var links []database.InspectionTemplateMetric
		database.DB.Where("template_id = ?", id).Find(&links)
		configIDs := make([]uint, len(links))
		for i, l := range links {
			configIDs[i] = l.MetricConfigID
		}
		configs := []database.MetricConfig{}
		if len(configIDs) > 0 {
			database.DB.Where("id IN ?", configIDs).Find(&configs)
		}
		// Merge overrides
		for i := range configs {
			var override database.TemplateMetricOverride
			if database.DB.Where("template_id = ? AND metric_config_id = ?", id, configs[i].ID).First(&override).Error == nil {
				override.Apply(&configs[i])
			}
		}
		writeJSON(w, configs)
	case "POST":
		var req struct {
			MetricConfigIDs []uint `json:"metric_config_ids"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, 400, "请求体格式错误")
			return
		}
		// Replace all metrics for this template
		database.DB.Where("template_id = ?", id).Delete(&database.InspectionTemplateMetric{})
		for _, cfgID := range req.MetricConfigIDs {
			database.DB.Create(&database.InspectionTemplateMetric{TemplateID: id, MetricConfigID: cfgID})
		}
		writeJSON(w, map[string]interface{}{"message": "模板指标已更新", "count": len(req.MetricConfigIDs)})
	default:
		writeError(w, 405, "不支持的请求方法")
	}
}

func (a *AdminAPI) handleTemplateMetricOverride(w http.ResponseWriter, r *http.Request) {
	if r.Method != "PUT" {
		writeError(w, 405, "不支持的请求方法")
		return
	}
	// Path: /api/v1/templates/{templateId}/metrics/{configId}/override
	parts := strings.Split(strings.TrimSuffix(strings.TrimSuffix(r.URL.Path, "/override"), "/"), "/")
	if len(parts) < 7 {
		writeError(w, 400, "路径格式错误")
		return
	}
	tmplID, err := strconv.ParseUint(parts[4], 10, 64)
	if err != nil {
		writeError(w, 400, "模板ID格式错误")
		return
	}
	configID, err := strconv.ParseUint(parts[6], 10, 64)
	if err != nil {
		writeError(w, 400, "指标配置ID格式错误")
		return
	}

	var req struct {
		Query           string  `json:"query"`
		Threshold       float64 `json:"threshold"`
		ThresholdType   string  `json:"threshold_type"`
		ThresholdStatus string  `json:"threshold_status"`
		Unit            string  `json:"unit"`
		LabelsJSON      string  `json:"labels_json"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, 400, "请求体格式错误")
		return
	}

	var override database.TemplateMetricOverride
	result := database.DB.Where("template_id = ? AND metric_config_id = ?", uint(tmplID), uint(configID)).First(&override)
	override.TemplateID = uint(tmplID)
	override.MetricConfigID = uint(configID)
	override.Query = req.Query
	override.Threshold = req.Threshold
	override.ThresholdType = req.ThresholdType
	override.ThresholdStatus = req.ThresholdStatus
	override.Unit = req.Unit
	override.LabelsJSON = req.LabelsJSON

	if result.Error != nil {
		database.DB.Create(&override)
	} else {
		database.DB.Save(&override)
	}

	writeJSON(w, map[string]interface{}{"message": "模板指标覆盖已保存"})
}

func (a *AdminAPI) handleTemplateInspect(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		writeError(w, 405, "不支持的请求方法")
		return
	}
	id, err := parseParentID(r.URL.Path)
	if err != nil {
		writeError(w, 400, err.Error())
		return
	}
	var t database.InspectionTemplate
	if database.DB.First(&t, id).Error != nil {
		writeError(w, 404, "模板不存在")
		return
	}
	var req struct {
		DatasourceID  uint   `json:"datasource_id"`
		DatasourceURL string `json:"datasource_url"`
		WechatBotKey  string `json:"wechat_bot_key"`
		ToUser        string `json:"touser"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, 400, "请求体格式错误")
		return
	}

	// Get metric configs from template
	var links []database.InspectionTemplateMetric
	database.DB.Where("template_id = ?", id).Find(&links)
	if len(links) == 0 {
		writeError(w, 400, "模板中没有指标配置")
		return
	}
	configIDs := make([]uint, len(links))
	for i, l := range links {
		configIDs[i] = l.MetricConfigID
	}
	var selectedConfigs []database.MetricConfig
	database.DB.Where("id IN ?", configIDs).Find(&selectedConfigs)
	if len(selectedConfigs) == 0 {
		writeError(w, 400, "模板中无有效指标")
		return
	}

	// Apply template metric overrides
	for i := range selectedConfigs {
		var override database.TemplateMetricOverride
		if database.DB.Where("template_id = ? AND metric_config_id = ?", id, selectedConfigs[i].ID).First(&override).Error == nil {
			override.Apply(&selectedConfigs[i])
		}
	}

	// Build prometheus URL
	promURL := a.config.PrometheusURL
	promUser := a.config.PrometheusUsername
	promPass := a.config.PrometheusPassword
	if req.DatasourceURL != "" {
		promURL = req.DatasourceURL
	} else if req.DatasourceID > 0 {
		var ds database.DataSource
		if database.DB.First(&ds, req.DatasourceID).Error == nil {
			promURL = ds.URL
			promUser = ds.Username
			promPass = ds.Password
		}
	}

	client, err := prometheus.NewClient(promURL, promUser, promPass)
	if err != nil {
		writeError(w, 500, fmt.Sprintf("创建Prometheus客户端失败: %v", err))
		return
	}

	activeConfig := a.buildFilteredConfig(a.config, selectedConfigs)
	dataCollector := metrics.NewCollector(client.API, activeConfig)
	data, err := dataCollector.CollectMetrics()
	if err != nil {
		writeError(w, 500, fmt.Sprintf("收集指标失败: %v", err))
		return
	}
	data.Datasource = promURL

	reportPath, err := report.GenerateReport(*data)
	if err != nil {
		writeError(w, 500, fmt.Sprintf("生成报告失败: %v", err))
		return
	}
	saveReportRecord(data, reportPath)

	writeJSON(w, map[string]interface{}{
		"success": true,
		"report":  reportPath,
		"url":     "/api/promai/reports/" + filepath.Base(reportPath),
	})
}

func (a *AdminAPI) handleTestNotification(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		writeError(w, 405, "不支持的请求方法")
		return
	}
	var req struct {
		ID uint `json:"id"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	var nc database.NotificationChannel
	if req.ID > 0 {
		if database.DB.First(&nc, req.ID).Error != nil {
			writeError(w, 404, "通知渠道不存在")
			return
		}
	} else {
		if err := json.NewDecoder(r.Body).Decode(&nc); err != nil {
			writeError(w, 400, "请求体格式错误")
			return
		}
	}

	testSummary := notify.AlertSummary{
		TotalMetrics:  3,
		TotalAlerts:   1,
		CriticalAlerts: 0,
		WarningAlerts: 1,
		NormalMetrics: 2,
	}

	switch nc.ChannelType {
	case "dingtalk":
		var cfg notify.DingtalkConfig
		json.Unmarshal([]byte(nc.ConfigJSON), &cfg)
		cfg.Enabled = true
		notify.SendDingtalk(cfg, "", "PromAI", "测试数据源", testSummary)
	case "email":
		var cfg notify.EmailConfig
		json.Unmarshal([]byte(nc.ConfigJSON), &cfg)
		cfg.Enabled = true
		notify.SendEmail(cfg, "", "PromAI", "测试数据源", testSummary)
	case "wechat_work":
		var cfg notify.WeChatWorkConfig
		json.Unmarshal([]byte(nc.ConfigJSON), &cfg)
		cfg.Enabled = true
		notify.SendWeChatWork(cfg, "", "PromAI", "测试数据源", testSummary)
	case "wechat_app":
		var cfg notify.WeChatAppConfig
		json.Unmarshal([]byte(nc.ConfigJSON), &cfg)
		cfg.Enabled = true
		notify.SendWeChatApp(cfg, "", "PromAI", "测试数据源", testSummary)
	case "feishu":
		var cfg notify.FeishuConfig
		json.Unmarshal([]byte(nc.ConfigJSON), &cfg)
		cfg.Enabled = true
		notify.SendFeishu(cfg, "", "PromAI", "测试数据源", testSummary)
	}

	writeJSON(w, map[string]string{"message": "测试通知已发送"})
}

func (a *AdminAPI) handleDashboardStats(w http.ResponseWriter, r *http.Request) {
	var dsCount, cronCount, reportCount, notifCount int64
	database.DB.Model(&database.DataSource{}).Count(&dsCount)
	database.DB.Model(&database.CronJob{}).Count(&cronCount)
	database.DB.Model(&database.ReportRecord{}).Count(&reportCount)
	database.DB.Model(&database.NotificationChannel{}).Count(&notifCount)

	var recentReports []database.ReportRecord
	database.DB.Order("created_at desc").Limit(5).Find(&recentReports)

	writeJSON(w, map[string]interface{}{
		"total_datasources":  dsCount,
		"total_cronjobs":     cronCount,
		"total_reports":      reportCount,
		"total_notifications": notifCount,
		"recent_reports":     recentReports,
	})
}

func (a *AdminAPI) handleDashboardHealth(w http.ResponseWriter, r *http.Request) {
	dsIDStr := r.URL.Query().Get("datasource_id")

	var datasources []database.DataSource
	q := database.DB.Order("is_default desc, name asc")
	if dsIDStr != "" {
		q = q.Where("id = ?", dsIDStr)
	}
	q.Find(&datasources)

	type HealthMetric struct {
		MetricTypeID uint   `json:"metric_type_id"`
		MetricName   string `json:"metric_name"`
		TypeName     string `json:"type_name"`
		Status       string `json:"status"`
		Value        string `json:"value"`
		Unit         string `json:"unit"`
		Threshold    float64 `json:"threshold"`
	}
	type DatasourceHealth struct {
		Datasource    database.DataSource `json:"datasource"`
		TotalMetrics  int                 `json:"total_metrics"`
		Alerts        int                 `json:"alerts"`
		CriticalCount int                 `json:"critical_count"`
		WarningCount  int                 `json:"warning_count"`
		NormalCount   int                 `json:"normal_count"`
		HealthScore   float64             `json:"health_score"`
		LastReportAt  *time.Time          `json:"last_report_at"`
		Metrics       []HealthMetric      `json:"metrics"`
	}

	var results []DatasourceHealth
	for _, ds := range datasources {
		total := 0
		alerts := 0
		criticals := 0
		warnings := 0
		var metrics []HealthMetric

		var configs []database.MetricConfig
		database.DB.Where("(datasource_id IS NULL OR datasource_id = ?)", ds.ID).Find(&configs)

		for _, cfg := range configs {
			total++
			var mt database.MetricType
			database.DB.First(&mt, cfg.MetricTypeID)
			status := "success"
			if cfg.Threshold > 0 {
				status = "warning"
				if cfg.ThresholdStatus == "critical" {
					status = "critical"
				}
			}
			if status == "critical" { criticals++; alerts++ }
			if status == "warning" { warnings++; alerts++ }

			metrics = append(metrics, HealthMetric{
				MetricTypeID: cfg.MetricTypeID,
				MetricName:   cfg.Name,
				TypeName:     mt.TypeName,
				Status:       status,
				Value:        "-",
				Unit:         cfg.Unit,
				Threshold:    cfg.Threshold,
			})
		}

		healthScore := 100.0
		if total > 0 {
			healthScore = float64(total-alerts) / float64(total) * 100
		}

		var lastReport database.ReportRecord
		database.DB.Where("datasource_name LIKE ?", "%"+ds.URL+"%").Or("datasource_id = ?", ds.ID).Order("created_at desc").First(&lastReport)

		results = append(results, DatasourceHealth{
			Datasource:    ds,
			TotalMetrics:  total,
			Alerts:        alerts,
			CriticalCount: criticals,
			WarningCount:  warnings,
			NormalCount:   total - alerts,
			HealthScore:   healthScore,
			LastReportAt:  func() *time.Time { if lastReport.ID > 0 { return &lastReport.CreatedAt }; return nil }(),
			Metrics:       metrics,
		})
	}

	// Global summary
	var totalAll, alertAll, criticalAll, warningAll int
	for _, r := range results {
		totalAll += r.TotalMetrics
		alertAll += r.Alerts
		criticalAll += r.CriticalCount
		warningAll += r.WarningCount
	}
	overallHealth := 100.0
	if totalAll > 0 {
		overallHealth = float64(totalAll-alertAll) / float64(totalAll) * 100
	}

	writeJSON(w, map[string]interface{}{
		"datasources":     results,
		"overall_health":  overallHealth,
		"total_metrics":   totalAll,
		"total_alerts":    alertAll,
		"critical_total":  criticalAll,
		"warning_total":   warningAll,
		"normal_total":    totalAll - alertAll,
	})
}

func getFileSize(path string) int64 {
	info, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return info.Size()
}

func countMetrics(data *report.ReportData) int {
	count := 0
	for _, group := range data.MetricGroups {
		for _, metrics := range group.MetricsByName {
			count += len(metrics)
		}
	}
	return count
}

func countAlerts(data *report.ReportData) int {
	return countCritical(data) + countWarning(data)
}

func countCritical(data *report.ReportData) int {
	count := 0
	for _, group := range data.MetricGroups {
		for _, metrics := range group.MetricsByName {
			for _, m := range metrics {
				if m.Status == "critical" {
					count++
				}
			}
		}
	}
	return count
}

func countWarning(data *report.ReportData) int {
	count := 0
	for _, group := range data.MetricGroups {
		for _, metrics := range group.MetricsByName {
			for _, m := range metrics {
				if m.Status == "warning" {
					count++
				}
			}
		}
	}
	return count
}

func getReportStatus(data *report.ReportData) string {
	for _, group := range data.MetricGroups {
		for _, metrics := range group.MetricsByName {
			for _, m := range metrics {
				if m.Status == "critical" {
					return "danger"
				}
			}
		}
	}
	if countWarning(data) > 0 {
		return "warning"
	}
	return "success"
}
