package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
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

type InspectTask struct {
	ID         string `json:"id"`
	Status     string `json:"status"` // running, completed, failed
	Message    string `json:"message"`
	ReportURL  string `json:"report_url,omitempty"`
	Error      string `json:"error,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

var (
	inspectTasks   = make(map[string]*InspectTask)
	inspectTasksMu sync.Mutex
	taskCounter    int
)

func newTaskID() string {
	taskCounter++
	return fmt.Sprintf("task_%d_%d", time.Now().Unix(), taskCounter)
}

type MetricHealthData struct {
	MetricName    string            `json:"metric_name"`
	TypeName      string            `json:"type_name"`
	Status        string            `json:"status"`
	Value         float64           `json:"value"`
	Unit          string            `json:"unit"`
	Threshold     float64           `json:"threshold"`
	ThresholdType string            `json:"threshold_type"`
	Labels        map[string]string `json:"labels"`
}

type DatasourceHealthSnapshot struct {
	DatasourceName string             `json:"datasource_name"`
	TotalMetrics   int                `json:"total_metrics"`
	CriticalCount  int                `json:"critical_count"`
	WarningCount   int                `json:"warning_count"`
	Metrics        []MetricHealthData `json:"metrics"`
}

func reportDataToHealth(data *report.ReportData) *DatasourceHealthSnapshot {
	var metrics []MetricHealthData
	criticals := 0
	warnings := 0
	for _, group := range data.MetricGroups {
		for metricName, metricList := range group.MetricsByName {
			for _, m := range metricList {
				if m.Status == "critical" {
					criticals++
				} else if m.Status == "warning" {
					warnings++
				}
				labels := make(map[string]string)
				for _, l := range m.Labels {
					key := l.Alias
					if key == "" {
						key = l.Name
					}
					labels[key] = l.Value
				}
				metrics = append(metrics, MetricHealthData{
					MetricName:    metricName,
					TypeName:      group.Type,
					Status:        m.Status,
					Value:         m.Value,
					Unit:          m.Unit,
					Threshold:     m.Threshold,
					ThresholdType: m.ThresholdType,
					Labels:        labels,
				})
			}
		}
	}
	return &DatasourceHealthSnapshot{
		DatasourceName: data.Datasource,
		TotalMetrics:   len(metrics),
		CriticalCount:  criticals,
		WarningCount:   warnings,
		Metrics:        metrics,
	}
}

func buildHealthSnapshot(data *report.ReportData) string {
	var metrics []MetricHealthData
	for _, group := range data.MetricGroups {
		for metricName, metricList := range group.MetricsByName {
			for _, m := range metricList {
				labels := make(map[string]string)
				for _, l := range m.Labels {
					key := l.Alias
					if key == "" {
						key = l.Name
					}
					labels[key] = l.Value
				}
				metrics = append(metrics, MetricHealthData{
					MetricName:    metricName,
					TypeName:      group.Type,
					Status:        m.Status,
					Value:         m.Value,
					Unit:          m.Unit,
					Threshold:     m.Threshold,
					ThresholdType: m.ThresholdType,
					Labels:        labels,
				})
			}
		}
	}
	snapshot := DatasourceHealthSnapshot{
		DatasourceName: data.Datasource,
		TotalMetrics:   len(metrics),
		Metrics:        metrics,
	}
	for _, m := range metrics {
		if m.Status == "critical" {
			snapshot.CriticalCount++
		} else if m.Status == "warning" {
			snapshot.WarningCount++
		}
	}
	b, _ := json.Marshal(snapshot)
	return string(b)
}

var (
	latestReports   map[string]*DatasourceHealthSnapshot
	latestReportsMu sync.RWMutex
)

func setLatestReport(key string, data *DatasourceHealthSnapshot) {
	latestReportsMu.Lock()
	defer latestReportsMu.Unlock()
	if latestReports == nil {
		latestReports = make(map[string]*DatasourceHealthSnapshot)
	}
	latestReports[key] = data
}

func getLatestReport(key string) *DatasourceHealthSnapshot {
	latestReportsMu.RLock()
	defer latestReportsMu.RUnlock()
	if latestReports == nil {
		return nil
	}
	return latestReports[key]
}

func loadLatestReports() {
	var records []database.ReportRecord
	database.DB.Where("metrics_json != ''").Order("created_at desc").Find(&records)
	seen := make(map[string]bool)
	for _, r := range records {
		key := r.DatasourceName
		if seen[key] {
			continue
		}
		seen[key] = true
		var snapshot DatasourceHealthSnapshot
		if err := json.Unmarshal([]byte(r.MetricsJSON), &snapshot); err == nil {
			setLatestReport(key, &snapshot)
		}
	}
	log.Printf("已加载 %d 个数据源的最新巡检快照", len(seen))
}

type AdminAPI struct {
	collector    *metrics.Collector
	config       *config.Config
	authUser     string
	authPass     string
	jwtSecret    string
	scheduler    *cron.Cron
	syncCronJobs map[uint]cron.EntryID
}

func NewAdminAPI(collector *metrics.Collector, cfg *config.Config, scheduler *cron.Cron) *AdminAPI {
	authUser := cfg.Auth.Username
	authPass := cfg.Auth.Password
	jwtSecret := cfg.Auth.JWTSecret

	if envUser := os.Getenv("PROMAI_AUTH_USERNAME"); envUser != "" {
		authUser = envUser
	}
	if envPass := os.Getenv("PROMAI_AUTH_PASSWORD"); envPass != "" {
		authPass = envPass
	}
	if envSecret := os.Getenv("PROMAI_JWT_SECRET"); envSecret != "" {
		jwtSecret = envSecret
	}

	if authUser == "" {
		authUser = "admin"
	}
	if authPass == "" {
		authPass = "admin"
	}
	if jwtSecret == "" {
		jwtSecret = "promai-default-secret"
	}

	return &AdminAPI{
		collector:    collector,
		config:       cfg,
		authUser:     authUser,
		authPass:     authPass,
		jwtSecret:    jwtSecret,
		scheduler:    scheduler,
		syncCronJobs: make(map[uint]cron.EntryID),
	}
}

func (a *AdminAPI) RegisterHandlers(mux *http.ServeMux) {
	// Public auth routes (no auth required)
	mux.HandleFunc("/api/v1/auth/login", a.handleLogin)

	// Auth middleware helper
	auth := a.authMiddleware

	// Protected routes
	mux.HandleFunc("/api/v1/auth/me", auth(a.handleMe))
	mux.HandleFunc("/api/v1/datasources", auth(a.handleDataSources))
	mux.HandleFunc("/api/v1/datasources/", auth(a.handleDataSourceByID))
	mux.HandleFunc("/api/v1/notifications", auth(a.handleNotifications))
	mux.HandleFunc("/api/v1/notifications/", auth(a.handleNotificationByID))
	mux.HandleFunc("/api/v1/cronjobs", auth(a.handleCronJobs))
	mux.HandleFunc("/api/v1/cronjobs/", auth(a.handleCronJobByID))
	mux.HandleFunc("/api/v1/reports", auth(a.handleReports))
	mux.HandleFunc("/api/v1/reports/", auth(a.handleReportByID))
	mux.HandleFunc("/api/v1/metrics/types", auth(a.handleMetricTypes))
	mux.HandleFunc("/api/v1/metrics/types/", auth(a.handleMetricTypeByID))
	mux.HandleFunc("/api/v1/metrics/configs", auth(a.handleMetricConfigs))
	mux.HandleFunc("/api/v1/metrics/configs/", auth(a.handleMetricConfigByID))
	mux.HandleFunc("/api/v1/metrics/validate", auth(a.handleValidatePromQL))
	mux.HandleFunc("/api/v1/templates", auth(a.handleTemplates))
	mux.HandleFunc("/api/v1/templates/", auth(a.handleTemplateByID))
	mux.HandleFunc("/api/v1/settings", auth(a.handleSettings))
	mux.HandleFunc("/api/v1/inspect", auth(a.handleInspect))
	mux.HandleFunc("/api/v1/inspect/records", auth(a.handleInspectRecords))
	mux.HandleFunc("/api/v1/inspect/task/", auth(a.handleInspectTask))
	mux.HandleFunc("/api/v1/datasources/import", auth(a.handleImportDatasource))
	mux.HandleFunc("/api/v1/notifications/test", auth(a.handleTestNotification))
	mux.HandleFunc("/api/v1/dashboard/stats", auth(a.handleDashboardStats))
	mux.HandleFunc("/api/v1/dashboard/health", auth(a.handleDashboardHealth))
	mux.HandleFunc("/api/v1/dashboard/health/trend", auth(a.handleDashboardHealthTrend))
	mux.HandleFunc("/api/v1/datasources/apply-template", auth(a.handleApplyTemplate))
	mux.HandleFunc("/api/v1/sync-sources", auth(a.handleSyncSources))
	mux.HandleFunc("/api/v1/sync-sources/", auth(a.handleSyncSourceByID))

	log.Printf("[AdminAPI] 管理接口已注册")
}

// authMiddleware 验证 JWT Token
func (a *AdminAPI) authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			writeError(w, 401, "未登录")
			return
		}
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			writeError(w, 401, "无效的认证令牌")
			return
		}
		claims, err := validateToken(parts[1], a.jwtSecret)
		if err != nil {
			writeError(w, 401, "认证令牌无效或已过期")
			return
		}
		ctx := context.WithValue(r.Context(), "username", claims.Username)
		next(w, r.WithContext(ctx))
	}
}

// handleLogin 处理登录请求
func (a *AdminAPI) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		writeError(w, 405, "不支持的请求方法")
		return
	}
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, 400, "请求体格式错误")
		return
	}
	if req.Username == "" || req.Password == "" {
		writeError(w, 400, "用户名和密码不能为空")
		return
	}
	if req.Username != a.authUser || req.Password != a.authPass {
		writeError(w, 401, "用户名或密码错误")
		return
	}
	token, err := generateToken(req.Username, a.jwtSecret)
	if err != nil {
		writeError(w, 500, "生成令牌失败")
		return
	}
	writeJSON(w, map[string]interface{}{
		"token":    token,
		"username": req.Username,
	})
}

// handleMe 返回当前登录用户信息
func (a *AdminAPI) handleMe(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		writeError(w, 405, "不支持的请求方法")
		return
	}
	username, _ := r.Context().Value("username").(string)
	writeJSON(w, map[string]interface{}{
		"username": username,
	})
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

func maskPassword(ds []database.DataSource) {
	for i := range ds {
		ds[i].Password = ""
	}
}

func (a *AdminAPI) handleDataSources(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		var ds []database.DataSource
		database.DB.Order("is_default desc, name asc").Find(&ds)
		maskPassword(ds)
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
		d.Password = ""
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
	if strings.HasSuffix(r.URL.Path, "/test") {
		a.handleTestDatasource(w, r)
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
		d.Password = ""
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
		if upd.Password == "" {
			upd.Password = d.Password
		}
		database.DB.Save(&upd)
		upd.Password = ""
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

	taskID := newTaskID()
	task := &InspectTask{
		ID:        taskID,
		Status:    "running",
		Message:   "巡检任务已创建，正在执行...",
		CreatedAt: time.Now(),
	}
	inspectTasksMu.Lock()
	inspectTasks[taskID] = task
	inspectTasksMu.Unlock()

	dsName := promURL
	if req.DatasourceID > 0 {
		var ds database.DataSource
		if database.DB.First(&ds, req.DatasourceID).Error == nil {
			dsName = ds.Name
		}
	}
	database.DB.Create(&database.InspectRecord{
		TaskID:         taskID,
		Status:         "running",
		DatasourceID:   func() *uint { if req.DatasourceID > 0 { return &req.DatasourceID }; return nil }(),
		DatasourceName: dsName,
		Message:        "巡检任务已创建，正在执行...",
		StartedAt:      time.Now(),
	})

	writeJSON(w, map[string]interface{}{
		"success": true,
		"task_id": taskID,
		"message": "巡检任务已创建",
	})

	go a.runInspect(task, promURL, promUser, promPass, req)
}

func (a *AdminAPI) runInspect(task *InspectTask, promURL, promUser, promPass string, req struct {
	DatasourceID    uint   `json:"datasource_id"`
	DatasourceURL   string `json:"datasource_url"`
	WechatBotKey    string `json:"wechat_bot_key"`
	ToUser          string `json:"touser"`
	MetricConfigIDs []uint `json:"metric_config_ids"`
}) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	done := make(chan struct{})
	var runErr error
	var data *report.ReportData
	var reportPath string

	go func() {
		defer close(done)

		client, err := prometheus.NewClient(promURL, promUser, promPass)
		if err != nil {
			runErr = fmt.Errorf("创建Prometheus客户端失败: %v", err)
			return
		}

		activeConfig := a.config
		if len(req.MetricConfigIDs) > 0 {
			var selectedConfigs []database.MetricConfig
			database.DB.Where("id IN ?", req.MetricConfigIDs).Find(&selectedConfigs)
			if len(selectedConfigs) > 0 {
				activeConfig = buildFilteredConfig(activeConfig, selectedConfigs)
			}
		} else if req.DatasourceID > 0 {
			var ds database.DataSource
			if database.DB.First(&ds, req.DatasourceID).Error == nil {
				if ds.TemplateID != nil {
					var links []database.InspectionTemplateMetric
					database.DB.Where("template_id = ?", *ds.TemplateID).Find(&links)
					if len(links) > 0 {
						cfgIDs := make([]uint, len(links))
						for i, l := range links {
							cfgIDs[i] = l.MetricConfigID
						}
						var tmplConfigs []database.MetricConfig
						database.DB.Where("id IN ?", cfgIDs).Find(&tmplConfigs)
						for i := range tmplConfigs {
							var override database.TemplateMetricOverride
							if database.DB.Where("template_id = ? AND metric_config_id = ?", *ds.TemplateID, tmplConfigs[i].ID).First(&override).Error == nil {
								override.Apply(&tmplConfigs[i])
							}
						}
						if len(tmplConfigs) > 0 {
							activeConfig = buildFilteredConfig(activeConfig, tmplConfigs)
						}
					}
				} else {
					var dsConfigs []database.MetricConfig
					database.DB.Where("(datasource_id IS NULL OR datasource_id = ?) AND metric_type_id IS NOT NULL", req.DatasourceID).Find(&dsConfigs)
					if len(dsConfigs) > 0 {
						activeConfig = buildFilteredConfig(activeConfig, dsConfigs)
					}
				}
			}
		}

		dataCollector := metrics.NewCollectorWithURL(client.API, activeConfig, promURL)
		data, runErr = dataCollector.CollectMetrics()
		if runErr != nil {
			runErr = fmt.Errorf("收集指标失败: %v", runErr)
			return
		}
		data.Datasource = promURL
		setLatestReport(promURL, reportDataToHealth(data))

		reportPath, runErr = report.GenerateReport(*data)
		if runErr != nil {
			runErr = fmt.Errorf("生成报告失败: %v", runErr)
			return
		}

		if req.WechatBotKey != "" {
			alertSummary := notify.CalculateAlertSummary(*data)
			notify.SendWeChatWorkWithWebhook(context.Background(), req.WechatBotKey, "", reportPath, a.config.ProjectName, promURL, alertSummary)
		}

		if req.DatasourceID > 0 {
			var ds database.DataSource
			if database.DB.First(&ds, req.DatasourceID).Error == nil && ds.NotifyChannels != "" {
				var channelIDs []uint
				if err := json.Unmarshal([]byte(ds.NotifyChannels), &channelIDs); err == nil && len(channelIDs) > 0 {
					var channels []database.NotificationChannel
					database.DB.Where("id IN ? AND enabled = ?", channelIDs, true).Find(&channels)
					alertSummary := notify.CalculateAlertSummary(*data)
					for _, ch := range channels {
						sendSingleNotification(ch, reportPath, data.Datasource, alertSummary, data)
					}
				}
			}
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
			MetricsJSON:    buildHealthSnapshot(data),
		})
	}()

	select {
	case <-done:
		inspectTasksMu.Lock()
		if runErr != nil {
			task.Status = "failed"
			task.Error = runErr.Error()
			database.DB.Model(&database.InspectRecord{}).Where("task_id = ?", task.ID).Updates(map[string]interface{}{
				"status": "failed", "error": runErr.Error(), "completed_at": time.Now(),
			})
		} else {
			reportURL := "/api/promai/reports/" + filepath.Base(reportPath)
			task.Status = "completed"
			task.ReportURL = reportURL
			task.Message = "巡检完成"
			database.DB.Model(&database.InspectRecord{}).Where("task_id = ?", task.ID).Updates(map[string]interface{}{
				"status": "completed", "message": "巡检完成", "report_url": reportURL, "completed_at": time.Now(),
			})
		}
		inspectTasksMu.Unlock()
	case <-ctx.Done():
		inspectTasksMu.Lock()
		task.Status = "failed"
		task.Error = "巡检超时（10分钟）"
		database.DB.Model(&database.InspectRecord{}).Where("task_id = ?", task.ID).Updates(map[string]interface{}{
			"status": "failed", "error": "巡检超时（10分钟）", "completed_at": time.Now(),
		})
		inspectTasksMu.Unlock()
	}
}

func (a *AdminAPI) handleInspectTask(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		writeError(w, 405, "不支持的请求方法")
		return
	}
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) < 5 {
		writeError(w, 400, "缺少任务 ID")
		return
	}
	taskID := parts[4]

	inspectTasksMu.Lock()
	task, ok := inspectTasks[taskID]
	inspectTasksMu.Unlock()

	if !ok {
		writeError(w, 404, "任务不存在")
		return
	}
	writeJSON(w, task)
}

func (a *AdminAPI) handleInspectRecords(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		writeError(w, 405, "不支持的请求方法")
		return
	}
	var records []database.InspectRecord
	database.DB.Order("created_at desc").Limit(100).Find(&records)
	writeJSON(w, records)
}

// buildFilteredConfig builds a config.Config from a filtered set of MetricConfig rows
func buildFilteredConfig(base *config.Config, selectedConfigs []database.MetricConfig) *config.Config {
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

	activeConfig := buildFilteredConfig(a.config, selectedConfigs)
	dataCollector := metrics.NewCollectorWithURL(client.API, activeConfig, promURL)
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

func (a *AdminAPI) handleTestDatasource(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		writeError(w, 405, "不支持的请求方法")
		return
	}
	id, err := parseParentID(r.URL.Path)
	if err != nil {
		writeError(w, 400, err.Error())
		return
	}
	var ds database.DataSource
	if database.DB.First(&ds, id).Error != nil {
		writeError(w, 404, "数据源不存在")
		return
	}
	client, err := prometheus.NewClient(ds.URL, ds.Username, ds.Password)
	if err != nil {
		writeJSON(w, map[string]interface{}{
			"success": false,
			"message": fmt.Sprintf("创建客户端失败: %v", err),
		})
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := client.HealthCheck(ctx); err != nil {
		writeJSON(w, map[string]interface{}{
			"success": false,
			"message": fmt.Sprintf("连接失败: %v", err),
		})
		return
	}
	writeJSON(w, map[string]interface{}{
		"success": true,
		"message": "连接成功",
	})
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

func (a *AdminAPI) getDatasourceMetricConfigs(ds database.DataSource) []database.MetricConfig {
	if ds.TemplateID != nil {
		var links []database.InspectionTemplateMetric
		database.DB.Where("template_id = ?", *ds.TemplateID).Find(&links)
		if len(links) > 0 {
			cfgIDs := make([]uint, len(links))
			for i, l := range links {
				cfgIDs[i] = l.MetricConfigID
			}
			var configs []database.MetricConfig
			database.DB.Where("id IN ?", cfgIDs).Find(&configs)
			for i := range configs {
				var override database.TemplateMetricOverride
				if database.DB.Where("template_id = ? AND metric_config_id = ?", *ds.TemplateID, configs[i].ID).First(&override).Error == nil {
					override.Apply(&configs[i])
				}
			}
			return configs
		}
	}
	var configs []database.MetricConfig
	database.DB.Where("(datasource_id IS NULL OR datasource_id = ?) AND metric_type_id IS NOT NULL", ds.ID).Find(&configs)
	return configs
}

func (a *AdminAPI) handleDashboardHealth(w http.ResponseWriter, r *http.Request) {
	dsIDStr := r.URL.Query().Get("datasource_id")

	var datasources []database.DataSource
	q := database.DB.Order("is_default desc, name asc")
	if dsIDStr != "" {
		q = q.Where("id = ?", dsIDStr)
	}
	q.Find(&datasources)

	type TypeSummaryItem struct {
		TypeName      string `json:"type_name"`
		TotalMetrics  int    `json:"total_metrics"`
		CriticalCount int    `json:"critical_count"`
		WarningCount  int    `json:"warning_count"`
		NormalCount   int    `json:"normal_count"`
		Alerts        int    `json:"alerts"`
	}
	type HealthMetric struct {
		MetricName    string            `json:"metric_name"`
		TypeName      string            `json:"type_name"`
		Status        string            `json:"status"`
		Value         float64           `json:"value"`
		Unit          string            `json:"unit"`
		Threshold     float64           `json:"threshold"`
		ThresholdType string            `json:"threshold_type"`
		Labels        map[string]string `json:"labels"`
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
		LastReportURL string              `json:"last_report_url"`
		Metrics       []HealthMetric      `json:"metrics"`
		TypeSummaries []TypeSummaryItem   `json:"type_summaries"`
	}

	var results []DatasourceHealth

	for _, ds := range datasources {
		var lastReport database.ReportRecord
		database.DB.Where("datasource_name = ?", ds.URL).Order("created_at desc").First(&lastReport)

		snapshot := getLatestReport(ds.URL)
		if snapshot == nil {
		lastReportURL := ""
		if lastReport.ID > 0 && lastReport.FilePath != "" {
			lastReportURL = "/api/promai/reports/" + filepath.Base(lastReport.FilePath)
		}
		results = append(results, DatasourceHealth{
			Datasource:    ds,
			Metrics:       []HealthMetric{},
			LastReportAt:  func() *time.Time { if lastReport.ID > 0 { return &lastReport.CreatedAt }; return nil }(),
			LastReportURL: lastReportURL,
			TypeSummaries: []TypeSummaryItem{},
		})
			continue
		}

		typeMetrics := make(map[string]*TypeSummaryItem)
		typeOrder := make([]string, 0)
		var metrics []HealthMetric
		alerts := 0
		for _, m := range snapshot.Metrics {
			if m.Status == "critical" || m.Status == "warning" {
				alerts++
			}
			metrics = append(metrics, HealthMetric{
				MetricName:    m.MetricName,
				TypeName:      m.TypeName,
				Status:        m.Status,
				Value:         m.Value,
				Unit:          m.Unit,
				Threshold:     m.Threshold,
				ThresholdType: m.ThresholdType,
				Labels:        m.Labels,
			})

			if _, ok := typeMetrics[m.TypeName]; !ok {
				typeMetrics[m.TypeName] = &TypeSummaryItem{TypeName: m.TypeName}
				typeOrder = append(typeOrder, m.TypeName)
			}
			item := typeMetrics[m.TypeName]
			item.TotalMetrics++
			switch m.Status {
			case "critical":
				item.CriticalCount++
				item.Alerts++
			case "warning":
				item.WarningCount++
				item.Alerts++
			default:
				item.NormalCount++
			}
		}
		typeSummaries := make([]TypeSummaryItem, 0, len(typeOrder))
		for _, tn := range typeOrder {
			typeSummaries = append(typeSummaries, *typeMetrics[tn])
		}

		healthScore := 100.0
		if snapshot.TotalMetrics > 0 {
			healthScore = float64(snapshot.TotalMetrics-alerts) / float64(snapshot.TotalMetrics) * 100
		}

		lastReportURL := ""
		if lastReport.ID > 0 && lastReport.FilePath != "" {
			lastReportURL = "/api/promai/reports/" + filepath.Base(lastReport.FilePath)
		}
		results = append(results, DatasourceHealth{
			Datasource:    ds,
			TotalMetrics:  snapshot.TotalMetrics,
			Alerts:        alerts,
			CriticalCount: snapshot.CriticalCount,
			WarningCount:  snapshot.WarningCount,
			NormalCount:   snapshot.TotalMetrics - alerts,
			HealthScore:   healthScore,
			LastReportAt:  func() *time.Time { if lastReport.ID > 0 { return &lastReport.CreatedAt }; return nil }(),
			LastReportURL: lastReportURL,
			Metrics:       metrics,
			TypeSummaries: typeSummaries,
		})
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Datasource.Name < results[j].Datasource.Name
	})

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

func (a *AdminAPI) handleDashboardHealthTrend(w http.ResponseWriter, r *http.Request) {
	daysStr := r.URL.Query().Get("days")
	days := 14
	if d, err := strconv.Atoi(daysStr); err == nil && d > 0 && d <= 90 {
		days = d
	}

	since := time.Now().AddDate(0, 0, -days)

	type DailyTrend struct {
		Date         string `json:"date"`
		TotalMetrics int    `json:"total_metrics"`
		Critical     int    `json:"critical"`
		Warning      int    `json:"warning"`
		Alerts       int    `json:"alerts"`
	}

	var records []database.ReportRecord
	database.DB.Where("created_at >= ?", since).Order("created_at asc").Find(&records)

	dailyMap := make(map[string]*DailyTrend)
	dateKeys := make([]string, 0)

	for _, rec := range records {
		date := rec.CreatedAt.Format("2006-01-02")
		if _, ok := dailyMap[date]; !ok {
			dailyMap[date] = &DailyTrend{Date: date}
			dateKeys = append(dateKeys, date)
		}
		d := dailyMap[date]
		d.TotalMetrics += rec.TotalMetrics
		d.Critical += rec.CriticalCount
		d.Warning += rec.WarningCount
		d.Alerts += rec.AlertCount
	}

	trend := make([]DailyTrend, 0, len(dateKeys))
	for _, k := range dateKeys {
		trend = append(trend, *dailyMap[k])
	}

	writeJSON(w, map[string]interface{}{
		"trend": trend,
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

// ---- Sync Source Handlers ----

func (a *AdminAPI) handleSyncSources(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		var list []database.SyncSource
		database.DB.Order("created_at desc").Find(&list)
		for i := range list {
			list[i].AuthPassword = ""
			list[i].AuthToken = ""
		}
		writeJSON(w, list)
	case "POST":
		var s database.SyncSource
		if err := json.NewDecoder(r.Body).Decode(&s); err != nil {
			writeError(w, 400, "请求体格式错误")
			return
		}
		if s.Name == "" || s.URL == "" || s.NameField == "" {
			writeError(w, 400, "名称、URL、名称字段不能为空")
			return
		}
		if s.Method == "" {
			s.Method = "GET"
		}
		if s.AuthType == "" {
			s.AuthType = "none"
		}
		database.DB.Create(&s)
		a.scheduleSyncSource(&s)
		s.AuthPassword = ""
		s.AuthToken = ""
		writeJSON(w, s)
	default:
		writeError(w, 405, "不支持的请求方法")
	}
}

func (a *AdminAPI) handleSyncSourceByID(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/sync-sources/")
	// Check for sub-routes
	if strings.HasSuffix(path, "/sync") {
		idStr := strings.TrimSuffix(path, "/sync")
		id, err := strconv.ParseUint(strings.Trim(idStr, "/"), 10, 64)
		if err != nil {
			writeError(w, 400, "无效的ID")
			return
		}
		a.handleSyncSourceTrigger(w, r, uint(id))
		return
	}
	if strings.HasSuffix(path, "/logs") {
		idStr := strings.TrimSuffix(path, "/logs")
		id, err := strconv.ParseUint(strings.Trim(idStr, "/"), 10, 64)
		if err != nil {
			writeError(w, 400, "无效的ID")
			return
		}
		a.handleSyncSourceLogs(w, r, uint(id))
		return
	}
	id, err := strconv.ParseUint(strings.Trim(path, "/"), 10, 64)
	if err != nil {
		writeError(w, 400, "无效的ID")
		return
	}
	switch r.Method {
	case "GET":
		var s database.SyncSource
		if database.DB.First(&s, id).Error != nil {
			writeError(w, 404, "同步源不存在")
			return
		}
		s.AuthPassword = ""
		s.AuthToken = ""
		writeJSON(w, s)
	case "PUT":
		var s database.SyncSource
		if database.DB.First(&s, id).Error != nil {
			writeError(w, 404, "同步源不存在")
			return
		}
		var upd database.SyncSource
		if err := json.NewDecoder(r.Body).Decode(&upd); err != nil {
			writeError(w, 400, "请求体格式错误")
			return
		}
		upd.ID = s.ID
		upd.CreatedAt = s.CreatedAt
		if upd.PasswordField == "" {
			upd.PasswordField = s.PasswordField
		}
		if upd.AuthPassword == "" {
			upd.AuthPassword = s.AuthPassword
		}
		if upd.AuthToken == "" {
			upd.AuthToken = s.AuthToken
		}
		database.DB.Save(&upd)
		a.rescheduleSyncSource(&s, &upd)
		upd.AuthPassword = ""
		upd.AuthToken = ""
		writeJSON(w, upd)
	case "DELETE":
		var s database.SyncSource
		if database.DB.First(&s, id).Error != nil {
			writeError(w, 404, "同步源不存在")
			return
		}
		a.removeSyncCron(s.ID)
		database.DB.Delete(&s)
		w.WriteHeader(204)
	default:
		writeError(w, 405, "不支持的请求方法")
	}
}

func (a *AdminAPI) handleSyncSourceTrigger(w http.ResponseWriter, r *http.Request, id uint) {
	if r.Method != "POST" {
		writeError(w, 405, "不支持的请求方法")
		return
	}
	var s database.SyncSource
	if database.DB.First(&s, id).Error != nil {
		writeError(w, 404, "同步源不存在")
		return
	}
	go a.executeSync(&s)
	writeJSON(w, map[string]string{"message": "同步任务已启动"})
}

func (a *AdminAPI) handleSyncSourceLogs(w http.ResponseWriter, r *http.Request, id uint) {
	if r.Method != "GET" {
		writeError(w, 405, "不支持的请求方法")
		return
	}
	var logs []database.SyncLog
	database.DB.Where("sync_source_id = ?", id).Order("created_at desc").Limit(50).Find(&logs)
	writeJSON(w, logs)
}

func (a *AdminAPI) executeSync(s *database.SyncSource) {
	log.Printf("[Sync] 开始同步: %s", s.Name)
	start := time.Now()

	// Build request
	req, err := http.NewRequest(s.Method, s.URL, nil)
	if err != nil {
		a.recordSyncLog(s.ID, "failed", fmt.Sprintf("创建请求失败: %v", err), 0, 0, 0, 0)
		return
	}

	// Custom headers (support -H 'Key: Value' lines or JSON)
	if s.Headers != "" {
		headers := map[string]string{}
		if err := json.Unmarshal([]byte(s.Headers), &headers); err != nil {
			// Parse -H format: one -H 'Key: Value' per line
			for _, line := range strings.Split(s.Headers, "\n") {
				line = strings.TrimSpace(line)
				if line == "" {
					continue
				}
				// Strip optional -H prefix and quotes
				line = strings.TrimPrefix(line, "-H")
				line = strings.TrimSpace(line)
				line = strings.Trim(line, "'\"")
				if k, v, ok := strings.Cut(line, ":"); ok {
					headers[strings.TrimSpace(k)] = strings.TrimSpace(v)
				}
			}
		}
		for k, v := range headers {
			req.Header.Set(k, v)
		}
	}

	// Request body
	if s.Body != "" && (s.Method == "POST" || s.Method == "PUT" || s.Method == "PATCH") {
		req.Body = io.NopCloser(strings.NewReader(s.Body))
		req.ContentLength = int64(len(s.Body))
	}

	// Auth
	switch s.AuthType {
	case "basic":
		req.SetBasicAuth(s.AuthUsername, s.AuthPassword)
	case "bearer":
		req.Header.Set("Authorization", "Bearer "+s.AuthToken)
	}

	// Set default headers
	if req.Header.Get("Content-Type") == "" && s.Body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if req.Header.Get("Accept") == "" {
		req.Header.Set("Accept", "application/json")
	}

	// Execute
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		a.recordSyncLog(s.ID, "failed", fmt.Sprintf("请求失败: %v", err), 0, 0, 0, 0)
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		a.recordSyncLog(s.ID, "failed", fmt.Sprintf("读取响应失败: %v", err), 0, 0, 0, 0)
		return
	}

	if resp.StatusCode >= 400 {
		a.recordSyncLog(s.ID, "failed", fmt.Sprintf("请求返回错误状态 %d: %s", resp.StatusCode, string(body)), 0, 0, 0, 0)
		return
	}

	// Parse JSON
	var data interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		a.recordSyncLog(s.ID, "failed", fmt.Sprintf("JSON解析失败: %v", err), 0, 0, 0, 0)
		return
	}

	// Extract data array
	items := a.extractJSONPath(data, s.DataPath)
	itemsArr, ok := items.([]interface{})
	if !ok {
		// Try treating the whole response as an array
		if arr, ok2 := data.([]interface{}); ok2 {
			itemsArr = arr
		} else {
			a.recordSyncLog(s.ID, "failed", "响应数据不是数组，请检查 data_path 配置", 0, 0, 0, 0)
			return
		}
	}

	total := len(itemsArr)
	created := 0
	updated := 0
	errCount := 0
	errMsgs := []string{}

	for _, item := range itemsArr {
		obj, ok := item.(map[string]interface{})
		if !ok {
			errCount++
			continue
		}
		name := fmt.Sprintf("%v", obj[s.NameField])
		if name == "" || name == "<nil>" {
			errCount++
			errMsgs = append(errMsgs, fmt.Sprintf("缺少名称字段 '%s'", s.NameField))
			continue
		}
		url := ""
		if s.URLTemplate != "" {
			url = s.URLTemplate
			for k, v := range obj {
				url = strings.ReplaceAll(url, "{"+k+"}", fmt.Sprintf("%v", v))
			}
		} else if s.URLField != "" {
			url = fmt.Sprintf("%v", obj[s.URLField])
		}
		username := ""
		if s.UsernameField != "" {
			username = fmt.Sprintf("%v", obj[s.UsernameField])
		}
		password := ""
		if s.PasswordField != "" {
			password = fmt.Sprintf("%v", obj[s.PasswordField])
		}

		// Find or create datasource
		var existing database.DataSource
		result := database.DB.Where("name = ?", name).First(&existing)
		if result.Error == nil {
			existing.URL = url
			existing.Username = username
			if password != "" {
				existing.Password = password
			}
			database.DB.Save(&existing)
			updated++
		} else {
			ds := database.DataSource{
				Name:     name,
				URL:      url,
				Username: username,
				Password: password,
			}
			database.DB.Create(&ds)
			created++
		}
	}

	status := "success"
	if created+updated == 0 {
		status = "failed"
	} else if errCount > 0 {
		status = "partial"
	}

	msg := fmt.Sprintf("同步完成: 总 %d 项, 新增 %d, 更新 %d, 失败 %d", total, created, updated, errCount)
	if len(errMsgs) > 0 {
		msg += "; " + strings.Join(errMsgs, "; ")
	}
	elapsed := time.Since(start)
	log.Printf("[Sync] %s (%v)", msg, elapsed)
	a.recordSyncLog(s.ID, status, msg, total, created, updated, errCount)
}

func (a *AdminAPI) extractJSONPath(data interface{}, path string) interface{} {
	if path == "" {
		return data
	}
	parts := strings.Split(path, ".")
	current := data
	for _, part := range parts {
		obj, ok := current.(map[string]interface{})
		if !ok {
			return nil
		}
		current = obj[part]
		if current == nil {
			return nil
		}
	}
	return current
}

func (a *AdminAPI) recordSyncLog(syncSourceID uint, status, message string, total, created, updated, errCount int) {
	log := database.SyncLog{
		SyncSourceID: syncSourceID,
		Status:       status,
		Message:      message,
		TotalItems:   total,
		CreatedItems: created,
		UpdatedItems: updated,
		ErrorItems:   errCount,
	}
	database.DB.Create(&log)

	// Update sync source updated_at
	database.DB.Model(&database.SyncSource{}).Where("id = ?", syncSourceID).Update("updated_at", time.Now())
}

func (a *AdminAPI) scheduleSyncSource(s *database.SyncSource) {
	if s.CronExpr == "" || !s.Enabled {
		return
	}
	if a.scheduler == nil {
		return
	}
	id, ok := a.syncCronJobs[s.ID]
	if ok {
		a.scheduler.Remove(id)
	}
	sourceID := s.ID
	entryID, err := a.scheduler.AddFunc(s.CronExpr, func() {
		var src database.SyncSource
		if database.DB.First(&src, sourceID).Error != nil {
			return
		}
		a.executeSync(&src)
	})
	if err != nil {
		log.Printf("[Sync] 调度同步源 %s 失败: %v", s.Name, err)
		return
	}
	a.syncCronJobs[s.ID] = entryID
	log.Printf("[Sync] 已调度同步源: %s (%s)", s.Name, s.CronExpr)
}

func (a *AdminAPI) rescheduleSyncSource(old, new *database.SyncSource) {
	if old.CronExpr != new.CronExpr || old.Enabled != new.Enabled {
		a.removeSyncCron(old.ID)
		a.scheduleSyncSource(new)
	}
}

func (a *AdminAPI) removeSyncCron(id uint) {
	if entryID, ok := a.syncCronJobs[id]; ok {
		if a.scheduler != nil {
			a.scheduler.Remove(entryID)
		}
		delete(a.syncCronJobs, id)
	}
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
