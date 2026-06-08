package database

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"PromAI/pkg/config"

	"gopkg.in/yaml.v2"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

func InitDB(dbPath string) error {
	var err error
	DB, err = gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		return fmt.Errorf("failed to connect database: %w", err)
	}

	sqlDB, err := DB.DB()
	if err != nil {
		return fmt.Errorf("failed to get sql.DB: %w", err)
	}
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)
	sqlDB.SetConnMaxLifetime(time.Hour)

	if err := AutoMigrate(DB); err != nil {
		return fmt.Errorf("failed to auto migrate: %w", err)
	}

	log.Printf("数据库初始化成功: %s", dbPath)
	return nil
}

func SeedFromConfig(cfg *config.Config) error {
	var count int64
	DB.Model(&DataSource{}).Count(&count)
	if count > 0 {
		log.Printf("数据库已有数据，跳过初始化导入")
		return nil
	}

	log.Printf("首次运行，从配置文件导入数据到数据库...")

	defaultDS := DataSource{
		Name:      "默认数据源",
		URL:       cfg.PrometheusURL,
		Username:  cfg.PrometheusUsername,
		Password:  cfg.PrometheusPassword,
		IsDefault: true,
	}
	DB.Create(&defaultDS)

	for _, ds := range cfg.DataSources {
		d := DataSource{
			Name:     ds.Name,
			URL:      ds.URL,
			Username: ds.UserName,
			Password: ds.Password,
		}
		DB.Create(&d)
	}

	for _, mt := range cfg.MetricTypes {
		mType := MetricType{
			TypeName:  mt.Type,
			SortOrder: 0,
		}
		DB.Create(&mType)

		for _, m := range mt.Metrics {
			labelsBytes, _ := json.Marshal(m.Labels)
			mc := MetricConfig{
				MetricTypeID:    mType.ID,
				Name:            m.Name,
				Description:     m.Description,
				Query:           m.Query,
				Threshold:       m.Threshold,
				ThresholdType:   m.ThresholdType,
				ThresholdStatus: m.ThresholdStatus,
				Unit:            m.Unit,
				LabelsJSON:      string(labelsBytes),
			}
			DB.Create(&mc)
		}
	}

	DB.Create(&NotificationChannel{
		ChannelType: "dingtalk",
		Name:        "钉钉通知",
		Enabled:     cfg.Notifications.Dingtalk.Enabled,
		ConfigJSON:  marshalJSON(cfg.Notifications.Dingtalk),
	})
	DB.Create(&NotificationChannel{
		ChannelType: "email",
		Name:        "邮件通知",
		Enabled:     cfg.Notifications.Email.Enabled,
		ConfigJSON:  marshalJSON(cfg.Notifications.Email),
	})
	DB.Create(&NotificationChannel{
		ChannelType: "wechat_work",
		Name:        "企业微信机器人",
		Enabled:     cfg.Notifications.WeChatWork.Enabled,
		ConfigJSON:  marshalJSON(cfg.Notifications.WeChatWork),
	})
	DB.Create(&NotificationChannel{
		ChannelType: "wechat_app",
		Name:        "企业微信应用",
		Enabled:     cfg.Notifications.WeChatApp.Enabled,
		ConfigJSON:  marshalJSON(cfg.Notifications.WeChatApp),
	})
	DB.Create(&NotificationChannel{
		ChannelType: "feishu",
		Name:        "飞书通知",
		Enabled:     cfg.Notifications.Feishu.Enabled,
		ConfigJSON:  marshalJSON(cfg.Notifications.Feishu),
	})

	DB.Create(&AppSetting{Key: "project_name", Value: cfg.ProjectName})
	DB.Create(&AppSetting{Key: "cron_schedule", Value: cfg.CronSchedule})

	DB.Create(&AppSetting{Key: "report_cleanup_enabled", Value: fmt.Sprintf("%v", cfg.ReportCleanup.Enabled)})
	DB.Create(&AppSetting{Key: "report_cleanup_max_age", Value: fmt.Sprintf("%d", cfg.ReportCleanup.MaxAge)})
	DB.Create(&AppSetting{Key: "report_cleanup_cron", Value: cfg.ReportCleanup.CronSchedule})

	for _, mt := range cfg.MetricTypes {
		typeName := mt.Type
		// Find the MetricType we just created
		var mType MetricType
		if err := DB.Where("type_name = ?", typeName).First(&mType).Error; err != nil {
			continue
		}
		tmpl := InspectionTemplate{
			Name:        typeName,
			Description: fmt.Sprintf("基于「%s」自动创建的默认模板", typeName),
		}
		DB.Create(&tmpl)
		// Associate all metrics in this group with the template
		var configs []MetricConfig
		DB.Where("metric_type_id = ?", mType.ID).Find(&configs)
		for _, mc := range configs {
			DB.Create(&InspectionTemplateMetric{
				TemplateID:     tmpl.ID,
				MetricConfigID: mc.ID,
			})
		}
	}

	// 创建全局模板（包含全部指标）
	var allConfigs []MetricConfig
	DB.Where("metric_type_id IS NOT NULL").Find(&allConfigs)
	if len(allConfigs) > 0 {
		globalTmpl := InspectionTemplate{
			Name:        "全局模板",
			Description: "包含全部全局指标的默认模板",
		}
		DB.Create(&globalTmpl)
		for _, mc := range allConfigs {
			DB.Create(&InspectionTemplateMetric{
				TemplateID:     globalTmpl.ID,
				MetricConfigID: mc.ID,
			})
		}
	}

	log.Printf("数据导入完成")
	return nil
}

func marshalJSON(v interface{}) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func LoadConfigFile(path string) (*config.Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}
	var cfg config.Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config file: %w", err)
	}
	if envPrometheusURL := os.Getenv("PROMETHEUS_URL"); envPrometheusURL != "" {
		cfg.PrometheusURL = envPrometheusURL
		cfg.PrometheusUsername = os.Getenv("PROMETHEUS_USERNAME")
		cfg.PrometheusPassword = os.Getenv("PROMETHEUS_PASSWORD")
	}
	return &cfg, nil
}
