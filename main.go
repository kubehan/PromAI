package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"PromAI/pkg/config"
	"PromAI/pkg/database"
	"PromAI/pkg/metrics"
	"PromAI/pkg/notify"
	piagent "PromAI/pkg/pi-agent"
	"PromAI/pkg/prometheus"
	"PromAI/pkg/report"
	"PromAI/pkg/status"
	"PromAI/pkg/taskmanager"
	"PromAI/pkg/utils"

	"github.com/robfig/cron/v3"
	"gopkg.in/yaml.v2"
)

// loadConfig 加载配置文件
func loadConfig(path string) (*config.Config, error) {
	data, err := os.ReadFile(path) // 读取配置文件
	if err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	var config config.Config // 定义配置结构体
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("parsing config file: %w", err)
	} // 解析配置文件
	// 从环境变量中获取 PrometheusURL
	if envPrometheusURL := os.Getenv("PROMETHEUS_URL"); envPrometheusURL != "" {
		log.Printf("使用环境变量中的 Prometheus URL: %s", envPrometheusURL)
		config.PrometheusURL = envPrometheusURL
		config.PrometheusUsername = os.Getenv("PROMETHEUS_USERNAME")
		config.PrometheusPassword = os.Getenv("PROMETHEUS_PASSWORD")
	} else {
		log.Printf("使用配置文件中的 Prometheus URL: %s", config.PrometheusURL)
	}
	return &config, nil // 返回配置结构体
}

// setup 初始化应用程序
func setup(configPath string) (*prometheus.Client, *config.Config, error) {
	config, err := loadConfig(configPath)
	if err != nil {
		return nil, nil, fmt.Errorf("loading config: %w", err)
	}

	client, err := prometheus.NewClient(config.PrometheusURL, config.PrometheusUsername, config.PrometheusPassword)
	if err != nil {
		return nil, nil, fmt.Errorf("initializing Prometheus client: %w", err)
	}
	return client, config, nil
}

func main() {
	// 设置命令行参数
	configPath := flag.String("config", "config/config.yaml", "配置文件路径")
	port := flag.String("port", ":8091", "服务端口")
	flag.Parse()

	// 初始化应用程序
	client, config, err := setup(*configPath)
	if err != nil {
		log.Fatalf("Failed to setup application: %v", err)
	}

	// 创建指标收集器
	collector := metrics.NewCollector(client.API, config)

	// 设置全局端口
	utils.SetGlobalPort(strings.TrimPrefix(*port, ":"))

	// 初始化 SQLite 数据库
	dbPath := "promai.db"
	if envDB := os.Getenv("PROMAI_DB_PATH"); envDB != "" {
		dbPath = envDB
	}
	if err := database.InitDB(dbPath); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	// 从配置文件导入初始数据到数据库
	if err := database.SeedFromConfig(config); err != nil {
		log.Printf("Warning: failed to seed database: %v", err)
	}

	// 从数据库加载历史巡检快照到内存
	loadLatestReports()

	// 创建全局定时调度器
	globalScheduler = cron.New(cron.WithParser(cron.NewParser(cron.SecondOptional | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)))

	// 设置 HTTP 路由
	setupRoutes(collector, config, globalScheduler)

	// 启动全局定时调度器（从配置文件和数据库加载定时任务）
	startGlobalScheduler(config, collector)

	// 配置报告清理
	if config.ReportCleanup.Enabled {
		cleanupSchedule := config.ReportCleanup.CronSchedule
		if cleanupSchedule == "" {
			cleanupSchedule = "0 2 * * *"
		}
		if cleanupSchedule != "" {
			c := cron.New()
			_, err := c.AddFunc(cleanupSchedule, func() {
				if err := report.CleanupReports(config.ReportCleanup.MaxAge); err != nil {
					log.Printf("报告清理失败: %v", err)
					return
				}
				log.Printf("报告清理成功")
			})
			if err != nil {
				log.Printf("设置清理定时任务失败: %v", err)
			} else {
				c.Start()
				log.Printf("报告清理定时任务已启动，执行周期: %s", cleanupSchedule)
			}
		}
	}

	// 启动 HTTP 服务器
	log.Printf("==========================================")
	log.Printf("PromAI 系统监控平台启动成功！")
	log.Printf("==========================================")
	log.Printf("服务端口: %s", *port)
	log.Printf("")
	log.Printf("访问地址:")
	log.Printf("  首页: http://localhost%s/api/promai", *port)
	log.Printf("  管理后台: http://localhost%s/api/promai/admin", *port)
	log.Printf("  巡检进度: http://localhost%s/api/promai/progress", *port)
	log.Printf("  历史报告: http://localhost%s/api/promai/reports/history", *port)
	log.Printf("")
	log.Printf("API接口:")
	log.Printf("  生成报告: GET http://localhost%s/api/promai/getreport", *port)
	log.Printf("  报告列表: GET http://localhost%s/api/promai/reports/list", *port)
	log.Printf("  状态页面: GET http://localhost%s/api/promai/status", *port)
	log.Printf("  静态文件: http://localhost%s/api/promai/reports/", *port)
	log.Printf("")
	log.Printf("数据源配置:")
	log.Printf("  默认数据源: %s", config.PrometheusURL)
	if len(config.DataSources) > 0 {
		log.Printf("  额外数据源:")
		for _, ds := range config.DataSources {
			log.Printf("    - %s: %s", ds.Name, ds.URL)
		}
	}
	log.Printf("")
	log.Printf("定时任务:")
	if config.CronSchedule != "" {
		log.Printf("  巡检任务: %s", config.CronSchedule)
	} else {
		log.Printf("  巡检任务: 未配置")
	}
	if config.ReportCleanup.Enabled {
		log.Printf("  报告清理: %s", config.ReportCleanup.CronSchedule)
	} else {
		log.Printf("  报告清理: 未启用")
	}
	log.Printf("")
	log.Printf("通知配置:")
	if config.Notifications.Dingtalk.Enabled {
		log.Printf("  钉钉通知: 已启用")
	}
	if config.Notifications.Email.Enabled {
		log.Printf("  邮件通知: 已启用")
	}
	if config.Notifications.WeChatWork.Enabled {
		log.Printf("  企业微信: 已启用")
	}
	if config.Notifications.Feishu.Enabled {
		log.Printf("  飞书通知: 已启用")
	}
	if config.Notifications.WeChatApp.Enabled {
		log.Printf("  企业微信应用: 已启用")
	}
	log.Printf("==========================================")
	log.Printf("服务器正在运行...")

	if err := http.ListenAndServe(*port, nil); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

// 全局定时调度器，由 admin API 动态管理
var globalScheduler *cron.Cron
var cronTaskCounter int64

// setupRoutes 设置 HTTP 路由
func setupRoutes(collector *metrics.Collector, config *config.Config, scheduler *cron.Cron) {
	// 设置首页路由
	http.HandleFunc("/api/promai/", indexHandler)
	http.HandleFunc("/api/promai/index", indexHandler)

	// 设置报告生成路由
	http.HandleFunc("/api/promai/getreport", makeReportHandler(collector, config))

	// 设置报告列表API
	http.HandleFunc("/api/promai/reports/list", reportsListHandler)

	// 设置最近活动API
	http.HandleFunc("/api/promai/activities", recentActivitiesHandler)

	// 设置静态文件服务
	http.Handle("/api/promai/reports/", http.StripPrefix("/api/promai/reports/", http.FileServer(http.Dir("reports"))))

	// 设置进度页面路由
	http.HandleFunc("/api/promai/progress", progressHandler)

	// 设置历史报告页面路由
	http.HandleFunc("/api/promai/reports/history", reportsHandler)

	// 设置状态页面路由
	http.HandleFunc("/api/promai/status", makeStatusHandler(collector.Client, config))

	// 设置任务管理相关API
	http.HandleFunc("/api/promai/tasks", tasksHandler)
	http.HandleFunc("/api/promai/tasks/", taskDetailHandler)

	// 将首页替换为后台登录页面（前端 SPA）
	if _, err := os.Stat("frontend/dist"); err == nil {
		distDir := "frontend/dist"
		http.Handle("/promai/assets/", http.StripPrefix("/promai/", http.FileServer(http.Dir(distDir))))
		http.Handle("/promai/favicon.svg", http.StripPrefix("/promai/", http.FileServer(http.Dir(distDir))))
		http.HandleFunc("/promai", func(w http.ResponseWriter, r *http.Request) {
			http.ServeFile(w, r, distDir+"/index.html")
		})
		http.HandleFunc("/promai/", func(w http.ResponseWriter, r *http.Request) {
			path := distDir + r.URL.Path[len("/promai/"):]
			if _, err := os.Stat(path); err == nil {
				http.ServeFile(w, r, path)
			} else {
				http.ServeFile(w, r, distDir+"/index.html")
			}
		})
		log.Printf("  前端构建产物已加载 (frontend/dist)")
	}

	// 注册管理 API 路由
	adminAPI := NewAdminAPI(collector, config, scheduler)
	adminAPI.RegisterHandlers(http.DefaultServeMux)

	// 注册 AI Agent 路由
	aiAgent := piagent.NewAgentHandler(config, collector, database.DB, config.Auth.JWTSecret)
	aiAgent.RegisterRoutes(http.DefaultServeMux, adminAPI.authMiddleware)

}

// startGlobalScheduler 启动全局定时调度器
func startGlobalScheduler(config *config.Config, collector *metrics.Collector) {
	// 清除旧任务，防止泄漏
	globalScheduler.Stop()
	for globalScheduler.Entries() != nil {
		globalScheduler = cron.New(cron.WithParser(cron.NewParser(cron.SecondOptional | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)))
		break
	}

	// 从数据库加载定时任务
	var dbJobs []database.CronJob
	database.DB.Where("enabled = ?", true).Find(&dbJobs)
	for _, job := range dbJobs {
		j := job
		_, err := globalScheduler.AddFunc(j.Schedule, func() {
			log.Printf("[Cron] 执行定时任务: %s", j.Name)
			doInspection(config, collector, j)
		})
		if err != nil {
			log.Printf("[Cron] 调度任务 %s 失败 (%s): %v", j.Name, j.Schedule, err)
		} else {
			log.Printf("[Cron] 已调度任务: %s (%s)", j.Name, j.Schedule)
		}
	}

	// 保留原有配置文件的定时任务（兼容）
	if config.CronSchedule != "" {
		_, err := globalScheduler.AddFunc(config.CronSchedule, func() {
			log.Printf("[Cron] 执行定时巡检任务(配置文件)...")
			data, err := collector.CollectMetrics()
			if err != nil {
				log.Printf("[Cron] 收集指标失败: %v", err)
				return
			}
			reportFilePath, err := report.GenerateReport(*data)
			if err != nil {
				log.Printf("[Cron] 生成报告失败: %v", err)
				return
			}
			saveReportRecord(data, reportFilePath)
			sendNotifications(config, reportFilePath, data)
			log.Printf("[Cron] 定时巡检任务完成: %s", reportFilePath)
		})
		if err != nil {
			log.Printf("[Cron] 调度配置文件任务失败: %v", err)
		} else {
			log.Printf("[Cron] 已调度配置文件任务: %s", config.CronSchedule)
		}
	}

	globalScheduler.Start()
	log.Printf("[Cron] 定时调度器已启动")
}

// executeInspectionWithProgress 带进度更新的巡检执行
func executeInspectionWithProgress(collector *metrics.Collector, config *config.Config, prometheusURL string, taskID string) (*report.ReportData, error) {
	// 开始执行巡检
	taskmanager.GlobalTaskManager.UpdateTaskProgress(taskID, 25, "收集系统资源数据")

	// 收集指标数据
	data, err := collector.CollectMetrics()
	if err != nil {
		taskmanager.GlobalTaskManager.FailStep(taskID, "收集系统资源数据", err.Error())
		return nil, fmt.Errorf("collecting metrics: %w", err)
	}
	taskmanager.GlobalTaskManager.CompleteStep(taskID, "收集系统资源数据")
	taskmanager.GlobalTaskManager.UpdateTaskProgress(taskID, 50, "收集服务状态")

	// 设置数据源信息
	data.Datasource = prometheusURL

	taskmanager.GlobalTaskManager.CompleteStep(taskID, "收集服务状态")
	taskmanager.GlobalTaskManager.UpdateTaskProgress(taskID, 75, "分析告警信息")

	// 分析告警信息
	taskmanager.GlobalTaskManager.CompleteStep(taskID, "分析告警信息")
	taskmanager.GlobalTaskManager.UpdateTaskProgress(taskID, 90, "生成巡检报告")

	// 生成报告
	reportFilePath, err := report.GenerateReport(*data)
	if err != nil {
		taskmanager.GlobalTaskManager.FailStep(taskID, "生成巡检报告", err.Error())
		return nil, fmt.Errorf("generating report: %w", err)
	}
	data.ReportUrl = reportFilePath

	// 完成任务
	taskmanager.GlobalTaskManager.CompleteTask(taskID, reportFilePath)

	return data, nil
}

// makeReportHandler 创建报告处理器
func makeReportHandler(collector *metrics.Collector, config *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 记录访问日志
		log.Printf("[API] %s %s from %s", r.Method, r.URL.Path, r.RemoteAddr)
		log.Printf("[DEBUG] 完整URL: %s", r.URL.String())
		log.Printf("[DEBUG] 完整Query: %s", r.URL.Query())
		// 获取并记录机器人key参数
		wechatBotKey := r.URL.Query().Get("wechat_bot_key")
		if wechatBotKey != "" {
			log.Printf("[DEBUG] 获取到企业微信机器人key: %s", wechatBotKey)
		} else {
			log.Printf("[DEBUG] 未传入企业微信机器人key参数,使用默认值")
		}

		// 获取datasource参数 - 使用多种方法确保获取到正确的值
		datasource := r.URL.Query().Get("datasource")
		prometheusURL := ""
		prometheusUsername := ""
		prometheusPassword := ""

		// 额外的调试信息
		log.Printf("[DEBUG] 收到的完整查询参数RawQuery: %s", r.URL.RawQuery)
		log.Printf("[DEBUG] 收到的URL.Query()结果: %v", r.URL.Query())
		log.Printf("[DEBUG] 收到的datasource参数: '%s'", datasource)
		log.Printf("[DEBUG] 收到的wechat_bot_key参数: '%s'", r.URL.Query().Get("wechat_bot_key"))

		// 确保datasource在任何位置都能被识别
		if datasource == "" && r.URL.RawQuery != "" {
			// 手动解析查询字符串，防止URL.Query()出现问题
			queryParams := strings.Split(r.URL.RawQuery, "&")
			log.Printf("[DEBUG] 手动解析的查询参数:")
			for _, param := range queryParams {
				log.Printf("[DEBUG]   %s", param)
				if strings.HasPrefix(param, "datasource=") {
					// 提取datasource值
					if parts := strings.SplitN(param, "=", 2); len(parts) == 2 {
						datasource = parts[1]
						log.Printf("[DEBUG] 手动解析找到datasource: '%s'", datasource)
						break
					}
				}
			}
		}

		if datasource != "" {
			log.Printf("[DEBUG] datasource参数不为空，开始处理...")
			// 检查是否是URL格式（包含http://或https://）
			if strings.HasPrefix(datasource, "http://") || strings.HasPrefix(datasource, "https://") {
				prometheusURL = datasource
				log.Printf("[DEBUG] 检测到URL格式，使用自定义PrometheusURL: %s", prometheusURL)
			} else {
				log.Printf("[DEBUG] 检测到名称格式，查找配置的数据源...")
				// 查找配置的数据源
				for _, ds := range config.DataSources {
					log.Printf("[DEBUG] 检查数据源: %s -> %s", ds.Name, ds.URL)
					if ds.Name == datasource {
						prometheusURL = ds.URL
						prometheusUsername = ds.UserName
						prometheusPassword = ds.Password
						log.Printf("[DEBUG] 找到匹配的数据源: %s -> %s", ds.Name, ds.URL)
						break
					}
				}
				if prometheusURL == "" {
					log.Printf("[DEBUG] 未找到配置的数据源: %s", datasource)
					http.Error(w, fmt.Sprintf("Datasource '%s' not found", datasource), http.StatusBadRequest)
					return
				}
			}
		} else {
			// 使用默认的Prometheus URL
			prometheusURL = config.PrometheusURL
			log.Printf("[DEBUG] datasource参数为空，使用默认Prometheus URL: %s", prometheusURL)
		}
		log.Printf("[DEBUG] 最终使用的Prometheus URL: %s", prometheusURL)

		// 获取taskID参数（可选）
		taskID := r.URL.Query().Get("taskid")

		// 如果没有提供taskID，自动生成一个（确保所有逻辑都使用带进度更新的执行方式）
		if taskID == "" {
			log.Printf("[DEBUG] 未传入taskid，自动生成taskid")
			// 使用任务管理器创建任务来生成唯一的taskid
			defaultTask := taskmanager.GlobalTaskManager.CreateTask("手动巡检", prometheusURL)
			taskID = defaultTask.ID
			log.Printf("[DEBUG] 自动生成的taskid: %s", taskID)
		}

		// 如果指定了datasource参数，创建新的collector
		var dataCollector *metrics.Collector
		if datasource != "" {
			// 创建新的Prometheus客户端
			log.Printf("[DEBUG] 创建自定义Prometheus客户端，URL: %s", prometheusURL)
			client, err := prometheus.NewClient(prometheusURL, prometheusUsername, prometheusPassword)
			if err != nil {
				http.Error(w, fmt.Sprintf("Failed to create Prometheus client for datasource '%s': %v", datasource, err), http.StatusInternalServerError)
				return
			}
			dataCollector = metrics.NewCollectorWithURL(client.API, config, prometheusURL)
			log.Printf("[DEBUG] 自定义collector创建完成，数据源: %s", prometheusURL)
		} else {
			dataCollector = collector
			log.Printf("[DEBUG] 使用全局collector")
		}

		var data *report.ReportData
		var err error

		// 现在总是使用带进度更新的执行方式（自动生成的taskid或传入的taskid）
		log.Printf("[DEBUG] 开始执行巡检，taskid: %s, datasource: %s", taskID, prometheusURL)
		data, err = executeInspectionWithProgress(dataCollector, config, prometheusURL, taskID)
		if err != nil {
			http.Error(w, "Failed to collect metrics", http.StatusInternalServerError)
			log.Printf("Error collecting metrics: %v", err)
			return
		}

		reportFilePath := data.ReportUrl

		// 创建包含HTTP请求和报告数据的context，用于动态URL生成和分类汇总
		ctx := context.WithValue(r.Context(), "http_request", r)
		ctx = context.WithValue(ctx, "report_data", *data)

		// 手动触发时也发送通知
		sendNotificationsWithContext(ctx, config, reportFilePath, data)

		// 如果是自动生成的taskid，可以可选地清理任务记录（可选）
		if strings.HasPrefix(taskID, "manual_") {
			log.Printf("[DEBUG] 清理自动生成的任务记录: %s", taskID)
			// taskmanager.GlobalTaskManager.RemoveTask(taskID) // 可选：移除任务记录
		}

		// 去掉 reports/ 前缀，因为静态文件服务已经映射到 reports 目录
		reportFileName := strings.TrimPrefix(reportFilePath, "reports/")
		http.Redirect(w, r, "/api/promai/reports/"+reportFileName, http.StatusSeeOther)
	}
}

// sendNotifications 发送所有通知（兼容版本）
func sendNotifications(config *config.Config, reportFilePath string, reportData *report.ReportData) {
	// 创建包含报告数据的上下文
	ctx := context.WithValue(context.Background(), "report_data", *reportData)
	sendNotificationsWithContext(ctx, config, reportFilePath, reportData)
}

// sendJobNotifications sends notifications to the channels configured in a cron job
func sendJobNotifications(job database.CronJob, reportFilePath string, reportData *report.ReportData) {
	if job.NotifyChannels == "" {
		return
	}
	var channelIDs []uint
	if err := json.Unmarshal([]byte(job.NotifyChannels), &channelIDs); err != nil || len(channelIDs) == 0 {
		return
	}
	var channels []database.NotificationChannel
	database.DB.Where("id IN ? AND enabled = ?", channelIDs, true).Find(&channels)
	if len(channels) == 0 {
		return
	}
	alertSummary := notify.CalculateAlertSummary(*reportData)
	for _, ch := range channels {
		sendSingleNotification(ch, reportFilePath, reportData.Datasource, alertSummary, reportData)
	}
}

func sendSingleNotification(ch database.NotificationChannel, reportFilePath, datasource string, summary notify.AlertSummary, reportData *report.ReportData) {
	ctx := context.Background()
	if reportData != nil {
		ctx = context.WithValue(ctx, "report_data", *reportData)
	}
	switch ch.ChannelType {
	case "dingtalk":
		var cfg notify.DingtalkConfig
		if json.Unmarshal([]byte(ch.ConfigJSON), &cfg) == nil {
			notify.SendDingtalkWithContext(ctx, cfg, reportFilePath, "PromAI", datasource, summary)
		}
	case "email":
		var cfg notify.EmailConfig
		if json.Unmarshal([]byte(ch.ConfigJSON), &cfg) == nil {
			notify.SendEmailWithContext(ctx, cfg, reportFilePath, "PromAI", datasource, summary)
		}
	case "wechat_work":
		var cfg notify.WeChatWorkConfig
		if json.Unmarshal([]byte(ch.ConfigJSON), &cfg) == nil {
			notify.SendWeChatWorkWithContext(ctx, cfg, reportFilePath, "PromAI", datasource, summary)
		}
	case "wechat_app":
		var cfg notify.WeChatAppConfig
		if json.Unmarshal([]byte(ch.ConfigJSON), &cfg) == nil {
			notify.SendWeChatAppWithContext(ctx, cfg, reportFilePath, "PromAI", datasource, summary)
		}
	case "feishu":
		var cfg notify.FeishuConfig
		if json.Unmarshal([]byte(ch.ConfigJSON), &cfg) == nil {
			notify.SendFeishuWithContext(ctx, cfg, reportFilePath, "PromAI", datasource, summary)
		}
	}
}

// sendNotificationsWithContext 发送所有通知（支持动态URL）
func sendNotificationsWithContext(ctx context.Context, config *config.Config, reportFilePath string, reportData *report.ReportData) {
	// 计算告警汇总
	alertSummary := notify.CalculateAlertSummary(*reportData)

	log.Printf("告警汇总: 总指标=%d, 异常=%d, 严重=%d, 警告=%d, 正常=%d",
		alertSummary.TotalMetrics, alertSummary.TotalAlerts, alertSummary.CriticalAlerts,
		alertSummary.WarningAlerts, alertSummary.NormalMetrics)

	if config.Notifications.Dingtalk.Enabled {
		log.Printf("发送钉钉消息")
		if err := notify.SendDingtalkWithContext(ctx, config.Notifications.Dingtalk, reportFilePath, config.ProjectName, reportData.Datasource, alertSummary); err != nil {
			log.Printf("发送钉钉消息失败: %v", err)
		}
	}

	if config.Notifications.Email.Enabled {
		log.Printf("发送邮件")
		notify.SendEmailWithContext(ctx, config.Notifications.Email, reportFilePath, config.ProjectName, reportData.Datasource, alertSummary)
	}

	if config.Notifications.WeChatWork.Enabled {
		log.Printf("发送企业微信消息")
		if err := notify.SendWeChatWorkWithContext(ctx, config.Notifications.WeChatWork, reportFilePath, config.ProjectName, reportData.Datasource, alertSummary); err != nil {
			log.Printf("发送企业微信消息失败: %v", err)
		}
	}

	if config.Notifications.Feishu.Enabled {
		log.Printf("发送飞书消息")
		if err := notify.SendFeishuWithContext(ctx, config.Notifications.Feishu, reportFilePath, config.ProjectName, reportData.Datasource, alertSummary); err != nil {
			log.Printf("发送飞书消息失败: %v", err)
		}
	}

	// 企业微信应用通知 - 支持动态touser参数
	if config.Notifications.WeChatApp.Enabled {
		log.Printf("发送企业微信应用消息")

		// 创建配置副本，以便动态修改touser
		appConfig := config.Notifications.WeChatApp

		// 检查是否有动态传入的touser参数
		if r, ok := ctx.Value("http_request").(*http.Request); ok {
			dynamicToUser := r.URL.Query().Get("touser")
			if dynamicToUser != "" {
				log.Printf("[NOTIFICATION] 检测到动态touser参数: %s，将覆盖配置文件设置", dynamicToUser)
				appConfig.ToUser = dynamicToUser
			}
		}

		if err := notify.SendWeChatAppWithContext(ctx, appConfig, reportFilePath, config.ProjectName, reportData.Datasource, alertSummary); err != nil {
			log.Printf("发送企业微信应用消息失败: %v", err)
		}
	}

	// 检查是否有动态传入的企业微信机器人key
	if r, ok := ctx.Value("http_request").(*http.Request); ok {
		wechatBotKey := r.URL.Query().Get("wechat_bot_key")
		if wechatBotKey != "" {
			log.Printf("[NOTIFICATION] 检测到动态企业微信机器人key: %s", wechatBotKey)
			log.Printf("[NOTIFICATION] 开始发送企业微信通知...")

			// 从配置文件获取代理地址
			proxyURL := ""
			if config.Notifications.WeChatWork.ProxyURL != "" {
				proxyURL = config.Notifications.WeChatWork.ProxyURL
			}
			log.Printf("[NOTIFICATION] 使用代理地址: %s", proxyURL)

			if err := notify.SendWeChatWorkWithWebhook(ctx, wechatBotKey, proxyURL, reportFilePath, config.ProjectName, reportData.Datasource, alertSummary); err != nil {
				log.Printf("[NOTIFICATION] 发送企业微信消息失败: %v", err)
			} else {
				log.Printf("[NOTIFICATION] 企业微信通知发送成功")
			}
		} else {
			log.Printf("[NOTIFICATION] 未传入企业微信机器人key，跳过企业微信通知")
		}
	}
}

// makeStatusHandler 创建状态页面处理器
func makeStatusHandler(client metrics.PrometheusAPI, config *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 记录访问日志
		log.Printf("[API] %s %s from %s", r.Method, r.URL.Path, r.RemoteAddr)

		// 获取datasource参数
		datasource := r.URL.Query().Get("datasource")
		prometheusURL := ""
		prometheusUsername := ""
		prometheusPassword := ""
		var prometheusClient metrics.PrometheusAPI

		if datasource != "" {
			// 检查是否是URL格式（包含http://或https://）
			if strings.HasPrefix(datasource, "http://") || strings.HasPrefix(datasource, "https://") {
				prometheusURL = datasource
			} else {
				// 查找配置的数据源
				for _, ds := range config.DataSources {
					if ds.Name == datasource {
						prometheusURL = ds.URL
						prometheusUsername = ds.UserName
						prometheusPassword = ds.Password
						break
					}
				}
				if prometheusURL == "" {
					http.Error(w, fmt.Sprintf("Datasource '%s' not found", datasource), http.StatusBadRequest)
					return
				}
			}

			// 创建新的Prometheus客户端
			newClient, err := prometheus.NewClient(prometheusURL, prometheusUsername, prometheusPassword)
			if err != nil {
				http.Error(w, fmt.Sprintf("Failed to create Prometheus client for datasource '%s': %v", datasource, err), http.StatusInternalServerError)
				return
			}
			prometheusClient = newClient.API
		} else {
			prometheusClient = client
			prometheusURL = config.PrometheusURL
		}

		log.Printf("状态接口使用Prometheus URL: %s", prometheusURL)
		data, err := status.CollectMetricStatus(prometheusClient, config, prometheusURL)
		if err != nil {
			http.Error(w, "Failed to collect status data", http.StatusInternalServerError)
			log.Printf("Error collecting status data: %v", err)
			return
		}

		// 创建模板函数映射
		funcMap := template.FuncMap{
			"now": time.Now,
			"date": func(format string, t time.Time) string {
				return t.Format(format)
			},
		}

		tmpl := template.New("status.html").Funcs(funcMap)
		tmpl, err = tmpl.ParseFiles("templates/status.html")
		if err != nil {
			http.Error(w, "Failed to parse template", http.StatusInternalServerError)
			log.Printf("Error parsing template: %v", err)
			return
		}

		if err := tmpl.Execute(w, data); err != nil {
			http.Error(w, "Failed to render template", http.StatusInternalServerError)
			log.Printf("Error rendering template: %v", err)
			return
		}
	}
}

// indexHandler 首页处理器
func indexHandler(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/promai/", http.StatusFound)
}

// progressHandler 进度页面处理器
func progressHandler(w http.ResponseWriter, r *http.Request) {
	tmpl, err := template.ParseFiles("templates/progress.html")
	if err != nil {
		http.Error(w, "Failed to parse template", http.StatusInternalServerError)
		log.Printf("Error parsing progress template: %v", err)
		return
	}

	if err := tmpl.Execute(w, nil); err != nil {
		http.Error(w, "Failed to render template", http.StatusInternalServerError)
		log.Printf("Error rendering progress template: %v", err)
		return
	}
}

// reportsHandler 历史报告页面处理器
func reportsHandler(w http.ResponseWriter, r *http.Request) {
	tmpl, err := template.ParseFiles("templates/reports.html")
	if err != nil {
		http.Error(w, "Failed to parse template", http.StatusInternalServerError)
		log.Printf("Error parsing reports template: %v", err)
		return
	}

	if err := tmpl.Execute(w, nil); err != nil {
		http.Error(w, "Failed to render template", http.StatusInternalServerError)
		log.Printf("Error rendering reports template: %v", err)
		return
	}
}

// reportsListHandler 报告列表API处理器
func reportsListHandler(w http.ResponseWriter, r *http.Request) {
	// 记录访问日志
	log.Printf("[API] %s %s from %s", r.Method, r.URL.Path, r.RemoteAddr)

	// 读取reports目录下的所有HTML文件
	files, err := os.ReadDir("reports")
	if err != nil {
		log.Printf("Error reading reports directory: %v", err)
		http.Error(w, "Failed to read reports directory", http.StatusInternalServerError)
		return
	}

	log.Printf("Found %d files in reports directory", len(files))

	type ReportInfo struct {
		ID         string `json:"id"`
		Title      string `json:"title"`
		Time       string `json:"time"`
		Size       string `json:"size"`
		Duration   string `json:"duration"`
		Datasource string `json:"datasource"`
		Stats      struct {
			Total    int `json:"total"`
			Alerts   int `json:"alerts"`
			Critical int `json:"critical"`
			Warning  int `json:"warning"`
		} `json:"stats"`
		Status string `json:"status"`
		URL    string `json:"url"`
	}

	var reports []ReportInfo
	htmlFileCount := 0

	for _, file := range files {
		if !file.IsDir() && strings.HasSuffix(file.Name(), ".html") {
			htmlFileCount++
			info, err := file.Info()
			if err != nil {
				continue
			}

			// 解析文件名获取时间信息
			// 例如: inspection_report_20250926_103846.html
			name := file.Name()
			id := strings.TrimSuffix(name, ".html")

			// 从文件名中提取时间
			parts := strings.Split(name, "_")
			if len(parts) >= 4 {
				dateStr := parts[2]
				timeStr := strings.TrimSuffix(parts[3], ".html")
				if len(dateStr) == 8 && len(timeStr) == 6 {
					formattedTime := fmt.Sprintf("%s-%s-%s %s:%s:%s",
						dateStr[:4], dateStr[4:6], dateStr[6:8],
						timeStr[:2], timeStr[2:4], timeStr[4:6])

					// 尝试从报告文件中提取数据源信息
					datasource := "默认数据源"

					// 读取报告文件的前几行来查找数据源信息
					if content, err := os.ReadFile("reports/" + name); err == nil {
						// 在HTML内容中搜索数据源信息 - 查找URL格式
						contentStr := string(content)

						// 方法1: 使用正则表达式提取数据源
						re := regexp.MustCompile(`<strong>数据源:</strong>\s*(https?://[^\s<]+)`)
						if matches := re.FindStringSubmatch(contentStr); len(matches) > 1 {
							urlStr := matches[1]
							// 从URL中提取有意义的名称
							if strings.Contains(urlStr, "prometheus") && strings.HasPrefix(urlStr, "http") {
								// 解析URL
								if u, err := url.Parse(urlStr); err == nil {
									// 提取主机名（不带端口）
									host := u.Hostname()
									// 对于prometheus URL，提取prometheus后面的完整域名
									if strings.Contains(host, "prometheus.") {
										parts := strings.Split(host, "prometheus.")
										if len(parts) > 1 {
											datasource = parts[1]
										}
									} else {
										// 对于非prometheus URL，使用完整域名
										datasource = host
									}
								} else {
									// 如果解析失败，回退到使用完整URL
									datasource = urlStr
								}
							} else {
								// 从URL中提取主机名
								if u, err := url.Parse(urlStr); err == nil {
									hostParts := strings.Split(u.Hostname(), ".")
									if len(hostParts) > 0 {
										datasource = hostParts[0]
									}
								}
							}
						}
					}

					// 从任务管理器获取任务信息以计算耗时
					task, exists := taskmanager.GlobalTaskManager.GetTask(id)
					var startTime, endTime time.Time

					if exists && task != nil {
						startTime = task.StartTime
						endTime = task.EndTime
					} else {
						// 如果任务不存在，使用文件修改时间作为结束时间
						endTime = info.ModTime()
						// 尝试从文件名中提取开始时间（如果文件名包含时间戳）
						if fileTime, err := time.Parse("20060102_150405", strings.Split(name, "_")[0]); err == nil {
							startTime = fileTime
						}
					}

					report := ReportInfo{
						ID:    id,
						Title: fmt.Sprintf("系统巡检报告 - %s", datasource),
						Time:  formattedTime,
						Size:  formatFileSize(info.Size()),
						URL:   "/api/promai/reports/" + name,
					}

					// 计算实际耗时
					if !startTime.IsZero() && !endTime.IsZero() {
						duration := endTime.Sub(startTime)
						if duration < time.Minute {
							report.Duration = fmt.Sprintf("%d秒", int(duration.Seconds()))
						} else if duration < time.Hour {
							report.Duration = fmt.Sprintf("%.1f分钟", duration.Minutes())
						} else {
							report.Duration = fmt.Sprintf("%.1f小时", duration.Hours())
						}
					} else {
						report.Duration = "2分钟"
					}
					report.Stats.Total = 150
					report.Stats.Alerts = 0
					report.Stats.Critical = 0
					report.Stats.Warning = 0
					report.Status = "success"
					report.Datasource = datasource
					reports = append(reports, report)
				}
			}
		}
	}

	log.Printf("Processed %d HTML files, created %d report entries", htmlFileCount, len(reports))

	// 按时间倒序排序
	sort.Slice(reports, func(i, j int) bool {
		return reports[i].Time > reports[j].Time
	})

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(reports); err != nil {
		log.Printf("Error encoding reports: %v", err)
	}
}

// formatFileSize 格式化文件大小
func formatFileSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// ActivityItem 表示一个活动项
type ActivityItem struct {
	ID         string    `json:"id"`
	Type       string    `json:"type"` // success, warning, error, info
	Title      string    `json:"title"`
	Message    string    `json:"message"`
	Time       time.Time `json:"time"`
	Icon       string    `json:"icon"`
	Source     string    `json:"source"` // task, report, alert
	Datasource string    `json:"datasource"`
}

// recentActivitiesHandler 处理最近活动API
func recentActivitiesHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("[API] %s %s from %s", r.Method, r.URL.Path, r.RemoteAddr)

	w.Header().Set("Content-Type", "application/json")

	var activities []ActivityItem

	// 获取最近的报告
	if files, err := os.ReadDir("reports"); err == nil {
		for _, file := range files {
			if !file.IsDir() && strings.HasSuffix(file.Name(), ".html") {

				// 只取最近5个报告
				if len(activities) >= 5 {
					break
				}

				// 解析文件名获取时间
				name := file.Name()
				if strings.HasPrefix(name, "inspection_report_") {
					parts := strings.Split(name, "_")
					if len(parts) >= 4 {
						timeStr := strings.TrimSuffix(parts[3], ".html")
						if reportTime, err := time.Parse("20060102_150405", timeStr); err == nil {
							// 提取数据源
							datasource := "未知"
							if content, err := os.ReadFile("reports/" + name); err == nil {
								re := regexp.MustCompile(`<strong>数据源:</strong>\s*(https?://[^\s<]+)`)
								if matches := re.FindStringSubmatch(string(content)); len(matches) > 1 {
									urlStr := matches[1]
									if strings.Contains(urlStr, "prometheus") && strings.HasPrefix(urlStr, "http") {
										if u, err := url.Parse(urlStr); err == nil {
											host := u.Hostname()
											if strings.Contains(host, "prometheus.") {
												parts := strings.Split(host, "prometheus.")
												if len(parts) > 1 {
													datasource = parts[1]
												}
											} else {
												datasource = host
											}
										}
									}
								}
							}

							activities = append(activities, ActivityItem{
								ID:         "report_" + reportTime.Format("20060102_150405"),
								Type:       "success",
								Title:      "巡检报告生成",
								Message:    fmt.Sprintf("成功生成 %s 的巡检报告", datasource),
								Time:       reportTime,
								Icon:       "✓",
								Source:     "report",
								Datasource: datasource,
							})
						}
					}
				}
			}
		}
	}

	// 获取最近的任务
	tasks := taskmanager.GlobalTaskManager.GetAllTasks()
	for _, task := range tasks {
		// 只取最近的任务
		if len(activities) >= 10 {
			break
		}

		// 根据任务状态生成活动
		switch task.Status {
		case taskmanager.StatusCompleted:
			activities = append(activities, ActivityItem{
				ID:         "task_" + task.ID,
				Type:       "success",
				Title:      "巡检任务完成",
				Message:    fmt.Sprintf("%s 巡检任务已完成", task.Datasource),
				Time:       task.EndTime,
				Icon:       "✓",
				Source:     "task",
				Datasource: task.Datasource,
			})
		case taskmanager.StatusFailed:
			activities = append(activities, ActivityItem{
				ID:         "task_" + task.ID,
				Type:       "error",
				Title:      "巡检任务失败",
				Message:    fmt.Sprintf("%s 巡检任务执行失败", task.Datasource),
				Time:       task.EndTime,
				Icon:       "✗",
				Source:     "task",
				Datasource: task.Datasource,
			})
		case taskmanager.StatusRunning:
			activities = append(activities, ActivityItem{
				ID:         "task_" + task.ID,
				Type:       "info",
				Title:      "巡检任务执行中",
				Message:    fmt.Sprintf("%s 正在执行巡检", task.Datasource),
				Time:       task.StartTime,
				Icon:       "⏳",
				Source:     "task",
				Datasource: task.Datasource,
			})
		}
	}

	// 按时间倒序排序
	sort.Slice(activities, func(i, j int) bool {
		return activities[i].Time.After(activities[j].Time)
	})

	// 只返回最近10条
	if len(activities) > 10 {
		activities = activities[:10]
	}

	if err := json.NewEncoder(w).Encode(activities); err != nil {
		log.Printf("Error encoding activities: %v", err)
	}
}

// adminHandler 管理后台页面处理器
func adminHandler(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "templates/admin.html")
}

// doInspection 执行巡检并保存报告记录（并发执行所有数据源）
func doInspection(cfg *config.Config, collector *metrics.Collector, job database.CronJob) {
	dsIDs := resolveJobDatasourceIDs(job)
	if len(dsIDs) == 0 {
		doSingleInspection(cfg, collector, job, nil)
		return
	}

	var mu sync.Mutex
	var anySuccess bool
	var wg sync.WaitGroup
	for _, dsID := range dsIDs {
		wg.Add(1)
		id := dsID
		go func() {
			defer wg.Done()
			if doSingleInspection(cfg, collector, job, &id) {
				mu.Lock()
				anySuccess = true
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	status := "success"
	if !anySuccess {
		status = "failed"
	}
	database.DB.Model(&job).Updates(map[string]interface{}{"last_run_at": time.Now(), "last_status": status})
}

// doSingleInspection 对单个数据源执行巡检，创建 InspectRecord 并保存报告
func doSingleInspection(cfg *config.Config, collector *metrics.Collector, job database.CronJob, dsIDPtr *uint) bool {
	taskID := fmt.Sprintf("cron_%d_%d_%d", job.ID, time.Now().Unix(), atomic.AddInt64(&cronTaskCounter, 1))

	if dsIDPtr != nil {
		dsID := *dsIDPtr
		var ds database.DataSource
		if database.DB.First(&ds, dsID).Error != nil {
			log.Printf("[Cron] 数据源 %d 不存在", dsID)
			database.DB.Create(&database.InspectRecord{
				TaskID: taskID, Status: "failed",
				DatasourceID: &dsID, DatasourceName: fmt.Sprintf("数据源 %d", dsID),
				Message: "数据源不存在", Error: "数据源不存在",
				StartedAt: time.Now(), CompletedAt: &[]time.Time{time.Now()}[0],
			})
			return false
		}

		client, err := prometheus.NewClient(ds.URL, ds.Username, ds.Password)
		if err != nil {
			log.Printf("[Cron] 创建数据源 %s 客户端失败: %v", ds.Name, err)
			database.DB.Create(&database.InspectRecord{
				TaskID: taskID, Status: "failed", DatasourceID: &dsID,
				DatasourceName: ds.Name, Message: "创建客户端失败", Error: err.Error(),
				StartedAt: time.Now(), CompletedAt: &[]time.Time{time.Now()}[0],
			})
			return false
		}

		// 先检查连通性，不可用则立即终止
		hcCtx, hcCancel := context.WithTimeout(context.Background(), 10*time.Second)
		if err := client.HealthCheck(hcCtx); err != nil {
			hcCancel()
			log.Printf("[Cron] 数据源 %s 不可用: %v，跳过巡检", ds.Name, err)
			database.DB.Create(&database.InspectRecord{
				TaskID: taskID, Status: "failed", DatasourceID: &dsID,
				DatasourceName: ds.Name, Message: "数据源不可用", Error: err.Error(),
				StartedAt: time.Now(), CompletedAt: &[]time.Time{time.Now()}[0],
			})
			return false
		}
		hcCancel()

		activeCfg := cfg
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
					activeCfg = buildFilteredConfig(cfg, tmplConfigs)
				}
			}
		}

		database.DB.Create(&database.InspectRecord{
			TaskID: taskID, Status: "running", DatasourceID: &dsID,
			DatasourceName: ds.Name, Message: "正在执行巡检...",
			StartedAt: time.Now(),
		})

		dsCollector := metrics.NewCollectorWithURL(client.API, activeCfg, ds.URL)
		data, err := dsCollector.CollectMetrics()
		if err != nil {
			log.Printf("[Cron] 数据源 %s 收集指标失败: %v", ds.Name, err)
			database.DB.Model(&database.InspectRecord{}).Where("task_id = ?", taskID).Updates(map[string]interface{}{
				"status": "failed", "error": err.Error(), "completed_at": time.Now(),
			})
			return false
		}
		data.Datasource = ds.URL
		setLatestReport(data.Datasource, reportDataToHealth(data))

		reportFilePath, err := report.GenerateReport(*data)
		if err != nil {
			log.Printf("[Cron] 数据源 %s 生成报告失败: %v", ds.Name, err)
			database.DB.Model(&database.InspectRecord{}).Where("task_id = ?", taskID).Updates(map[string]interface{}{
				"status": "failed", "error": err.Error(), "completed_at": time.Now(),
			})
			return false
		}
		saveReportRecord(data, reportFilePath)
		sendNotifications(cfg, reportFilePath, data)
		sendJobNotifications(job, reportFilePath, data)
		reportURL := "/api/promai/reports/" + filepath.Base(reportFilePath)
		database.DB.Model(&database.InspectRecord{}).Where("task_id = ?", taskID).Updates(map[string]interface{}{
			"status": "completed", "message": "巡检完成", "report_url": reportURL, "completed_at": time.Now(),
		})
		log.Printf("[Cron] 数据源 %s 巡检完成: %s", ds.Name, reportFilePath)
		return true
	}

	// No datasource specified — use default collector
	// 先检查默认数据源连通性
	hcCtx, hcCancel := context.WithTimeout(context.Background(), 10*time.Second)
	client, err := prometheus.NewClient(cfg.PrometheusURL, cfg.PrometheusUsername, cfg.PrometheusPassword)
	if err != nil {
		hcCancel()
		log.Printf("[Cron] 创建默认数据源客户端失败: %v", err)
		database.DB.Create(&database.InspectRecord{
			TaskID: taskID, Status: "failed",
			DatasourceName: cfg.PrometheusURL, Message: "创建客户端失败", Error: err.Error(),
			StartedAt: time.Now(), CompletedAt: &[]time.Time{time.Now()}[0],
		})
		return false
	}
	if err := client.HealthCheck(hcCtx); err != nil {
		hcCancel()
		log.Printf("[Cron] 默认数据源 %s 不可用: %v，跳过巡检", cfg.PrometheusURL, err)
		database.DB.Create(&database.InspectRecord{
			TaskID: taskID, Status: "failed",
			DatasourceName: cfg.PrometheusURL, Message: "数据源不可用", Error: err.Error(),
			StartedAt: time.Now(), CompletedAt: &[]time.Time{time.Now()}[0],
		})
		return false
	}
	hcCancel()

	database.DB.Create(&database.InspectRecord{
		TaskID: taskID, Status: "running",
		DatasourceName: cfg.PrometheusURL, Message: "正在执行巡检...",
		StartedAt: time.Now(),
	})
	data, err := collector.CollectMetrics()
	if err != nil {
		log.Printf("[Cron] 收集指标失败: %v", err)
		database.DB.Model(&database.InspectRecord{}).Where("task_id = ?", taskID).Updates(map[string]interface{}{
			"status": "failed", "error": err.Error(), "completed_at": time.Now(),
		})
		database.DB.Model(&job).Updates(map[string]interface{}{"last_run_at": time.Now(), "last_status": "failed"})
		return false
	}
	data.Datasource = cfg.PrometheusURL
	setLatestReport(data.Datasource, reportDataToHealth(data))
	reportFilePath, err := report.GenerateReport(*data)
	if err != nil {
		log.Printf("[Cron] 生成报告失败: %v", err)
		database.DB.Model(&database.InspectRecord{}).Where("task_id = ?", taskID).Updates(map[string]interface{}{
			"status": "failed", "error": err.Error(), "completed_at": time.Now(),
		})
		return false
	}
	saveReportRecord(data, reportFilePath)
	sendNotifications(cfg, reportFilePath, data)
	sendJobNotifications(job, reportFilePath, data)
	reportURL := "/api/promai/reports/" + filepath.Base(reportFilePath)
	database.DB.Model(&database.InspectRecord{}).Where("task_id = ?", taskID).Updates(map[string]interface{}{
		"status": "completed", "message": "巡检完成", "report_url": reportURL, "completed_at": time.Now(),
	})
	database.DB.Model(&job).Updates(map[string]interface{}{"last_run_at": time.Now(), "last_status": "success"})
	log.Printf("[Cron] 定时任务 %s 完成: %s", job.Name, reportFilePath)
	return true
}

func resolveJobDatasourceIDs(job database.CronJob) []uint {
	if job.AllDatasources {
		var dsList []database.DataSource
		database.DB.Where("enabled = ?", true).Find(&dsList)
		ids := make([]uint, len(dsList))
		for i, ds := range dsList {
			ids[i] = ds.ID
		}
		return ids
	}
	if job.DatasourceIDs != "" {
		var ids []uint
		if err := json.Unmarshal([]byte(job.DatasourceIDs), &ids); err == nil && len(ids) > 0 {
			return ids
		}
	}
	if job.DatasourceID != nil {
		return []uint{*job.DatasourceID}
	}
	return nil
}

// saveReportRecord 保存报告记录到数据库
func saveReportRecord(data *report.ReportData, reportPath string) {
	alertCount := 0
	criticalCount := 0
	warningCount := 0
	totalMetrics := 0
	hasCritical := false
	hasWarning := false

	for _, group := range data.MetricGroups {
		for _, metrics := range group.MetricsByName {
			for _, m := range metrics {
				totalMetrics++
				switch m.Status {
				case "critical":
					criticalCount++
					alertCount++
					hasCritical = true
				case "warning":
					warningCount++
					alertCount++
					hasWarning = true
				}
			}
		}
	}

	status := "success"
	if hasCritical {
		status = "danger"
	} else if hasWarning {
		status = "warning"
	}

	info, _ := os.Stat(reportPath)
	database.DB.Create(&database.ReportRecord{
		Title:          fmt.Sprintf("巡检报告 - %s", time.Now().Format("2006-01-02 15:04")),
		DatasourceName: data.Datasource,
		FilePath:       reportPath,
		FileSize:       func() int64 { if info != nil { return info.Size() }; return 0 }(),
		TotalMetrics:   totalMetrics,
		AlertCount:     alertCount,
		CriticalCount:  criticalCount,
		WarningCount:   warningCount,
		Status:         status,
		MetricsJSON:    buildHealthSnapshot(data),
	})
}

// tasksHandler 处理任务列表API
func tasksHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("[API] %s %s from %s", r.Method, r.URL.Path, r.RemoteAddr)

	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case "GET":
		// 获取所有任务
		tasks := taskmanager.GlobalTaskManager.GetAllTasks()
		json.NewEncoder(w).Encode(tasks)

	case "POST":
		// 创建新任务
		var req struct {
			Name       string `json:"name"`
			Datasource string `json:"datasource"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		if req.Name == "" {
			req.Name = "系统巡检任务"
		}

		task := taskmanager.GlobalTaskManager.CreateTask(req.Name, req.Datasource)
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(task)

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// taskDetailHandler 处理单个任务详情API
func taskDetailHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("[API] %s %s from %s", r.Method, r.URL.Path, r.RemoteAddr)

	// 从路径中提取任务ID
	path := strings.TrimPrefix(r.URL.Path, "/api/promai/tasks/")
	parts := strings.Split(path, "/")
	if len(parts) == 0 || parts[0] == "" {
		http.Error(w, "Task ID required", http.StatusBadRequest)
		return
	}
	taskID := parts[0]

	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case "GET":
		// 获取任务详情
		if task, exists := taskmanager.GlobalTaskManager.GetTask(taskID); exists {
			json.NewEncoder(w).Encode(task)
		} else {
			http.Error(w, "Task not found", http.StatusNotFound)
		}

	case "DELETE":
		// 取消任务
		taskmanager.GlobalTaskManager.CancelTask(taskID)
		w.WriteHeader(http.StatusOK)

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}
