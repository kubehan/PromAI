package database

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
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
	if err := migrateTemplateIDs(DB); err != nil {
		return fmt.Errorf("failed to migrate template ids: %w", err)
	}

	log.Printf("数据库初始化成功: %s", dbPath)
	return nil
}

func SeedFromConfig(cfg *config.Config) error {
	if cfg == nil {
		return nil
	}

	if err := ensureDefaultDatasource(cfg); err != nil {
		return err
	}
	if err := ensureConfiguredDatasources(cfg); err != nil {
		return err
	}
	if err := ensureMetricCatalog(cfg); err != nil {
		return err
	}
	if err := ensureNotificationChannels(cfg); err != nil {
		return err
	}
	if err := ensureAppSettings(cfg); err != nil {
		return err
	}
	if err := InitializeTemplatesFromMetricTypes(); err != nil {
		return err
	}

	log.Printf("配置数据已同步到数据库")
	return nil
}

func ImportSQLFileIfNeeded(cfg *config.Config) error {
	if strings.ToLower(strings.TrimSpace(os.Getenv("PROMAI_IMPORT_SQL_ON_START"))) != "true" {
		return nil
	}
	if cfg != nil && len(cfg.MetricTypes) > 0 {
		log.Printf("config.yaml 已配置 metric_types，跳过 SQL 初始化")
		return nil
	}

	sqlFile := strings.TrimSpace(os.Getenv("PROMAI_IMPORT_SQL_FILE"))
	if sqlFile == "" {
		return nil
	}

	content, err := os.ReadFile(sqlFile)
	if err != nil {
		return fmt.Errorf("reading import sql file %s: %w", sqlFile, err)
	}
	sqlText := strings.TrimSpace(string(content))
	if sqlText == "" {
		log.Printf("SQL 初始化文件为空，跳过: %s", sqlFile)
		return nil
	}
	if !strings.Contains(strings.ToUpper(sqlText), "INSERT ") {
		log.Printf("SQL 初始化文件未包含 INSERT 语句，跳过: %s", sqlFile)
		return nil
	}

	if err := DB.Exec(sqlText).Error; err != nil {
		return fmt.Errorf("executing import sql file %s: %w", sqlFile, err)
	}
	log.Printf("SQL 初始化完成: %s", sqlFile)
	return nil
}

func ensureDefaultDatasource(cfg *config.Config) error {
	if cfg.PrometheusURL == "" {
		return nil
	}
	var ds DataSource
	err := DB.Where("is_default = ? OR url = ?", true, cfg.PrometheusURL).First(&ds).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return DB.Create(&DataSource{
				Name:      "默认数据源",
				URL:       cfg.PrometheusURL,
				Username:  cfg.PrometheusUsername,
				Password:  cfg.PrometheusPassword,
				IsDefault: true,
				Enabled:   true,
			}).Error
		}
		return err
	}
	updates := map[string]any{
		"name":       firstNonEmpty(ds.Name, "默认数据源"),
		"url":        cfg.PrometheusURL,
		"username":   cfg.PrometheusUsername,
		"password":   cfg.PrometheusPassword,
		"is_default": true,
	}
	return DB.Model(&ds).Updates(updates).Error
}

func ensureConfiguredDatasources(cfg *config.Config) error {
	for _, item := range cfg.DataSources {
		if item.Name == "" || item.URL == "" {
			continue
		}
		var ds DataSource
		err := DB.Where("name = ? OR url = ?", item.Name, item.URL).First(&ds).Error
		if err != nil {
			if err == gorm.ErrRecordNotFound {
				if err := DB.Create(&DataSource{
					Name:     item.Name,
					URL:      item.URL,
					Username: item.UserName,
					Password: item.Password,
					Enabled:  true,
				}).Error; err != nil {
					return err
				}
				continue
			}
			return err
		}
		if err := DB.Model(&ds).Updates(map[string]any{
			"name":     item.Name,
			"url":      item.URL,
			"username": item.UserName,
			"password": item.Password,
		}).Error; err != nil {
			return err
		}
	}
	return nil
}

func ensureMetricCatalog(cfg *config.Config) error {
	for i, mt := range cfg.MetricTypes {
		if mt.Type == "" {
			continue
		}
		var mType MetricType
		err := DB.Where("type_name = ?", mt.Type).First(&mType).Error
		if err != nil {
			if err == gorm.ErrRecordNotFound {
				mType = MetricType{TypeName: mt.Type, SortOrder: i}
				if err := DB.Create(&mType).Error; err != nil {
					return err
				}
			} else {
				return err
			}
		} else if mType.SortOrder != i {
			if err := DB.Model(&mType).Update("sort_order", i).Error; err != nil {
				return err
			}
		}

		for j, metric := range mt.Metrics {
			var existing MetricConfig
			err := DB.Where("metric_type_id = ? AND datasource_id IS NULL AND name = ?", mType.ID, metric.Name).First(&existing).Error
			payload := MetricConfig{
				MetricTypeID:    mType.ID,
				Name:            metric.Name,
				Description:     metric.Description,
				Query:           metric.Query,
				Threshold:       metric.Threshold,
				ThresholdType:   defaultThresholdType(metric.ThresholdType),
				ThresholdStatus: defaultThresholdStatus(metric.ThresholdStatus),
				Unit:            metric.Unit,
				LabelsJSON:      marshalJSON(metric.Labels),
				SortOrder:       j,
			}
			if err != nil {
				if err == gorm.ErrRecordNotFound {
					if err := DB.Create(&payload).Error; err != nil {
						return err
					}
				} else {
					return err
				}
				continue
			}
			if err := DB.Model(&existing).Updates(map[string]any{
				"description":      payload.Description,
				"query":            payload.Query,
				"threshold":        payload.Threshold,
				"threshold_type":   payload.ThresholdType,
				"threshold_status": payload.ThresholdStatus,
				"unit":             payload.Unit,
				"labels_json":      payload.LabelsJSON,
				"sort_order":       payload.SortOrder,
			}).Error; err != nil {
				return err
			}
		}
	}
	return nil
}

func ensureNotificationChannels(cfg *config.Config) error {
	channels := []NotificationChannel{
		{ChannelType: "dingtalk", Name: "钉钉通知", Enabled: cfg.Notifications.Dingtalk.Enabled, ConfigJSON: marshalJSON(cfg.Notifications.Dingtalk)},
		{ChannelType: "email", Name: "邮件通知", Enabled: cfg.Notifications.Email.Enabled, ConfigJSON: marshalJSON(cfg.Notifications.Email)},
		{ChannelType: "wechat_work", Name: "企业微信机器人", Enabled: cfg.Notifications.WeChatWork.Enabled, ConfigJSON: marshalJSON(cfg.Notifications.WeChatWork)},
		{ChannelType: "wechat_app", Name: "企业微信应用", Enabled: cfg.Notifications.WeChatApp.Enabled, ConfigJSON: marshalJSON(cfg.Notifications.WeChatApp)},
		{ChannelType: "feishu", Name: "飞书通知", Enabled: cfg.Notifications.Feishu.Enabled, ConfigJSON: marshalJSON(cfg.Notifications.Feishu)},
	}
	for _, ch := range channels {
		var existing NotificationChannel
		err := DB.Where("channel_type = ?", ch.ChannelType).First(&existing).Error
		if err != nil {
			if err == gorm.ErrRecordNotFound {
				if err := DB.Create(&ch).Error; err != nil {
					return err
				}
			} else {
				return err
			}
			continue
		}
		if err := DB.Model(&existing).Updates(map[string]any{
			"name": ch.Name,
		}).Error; err != nil {
			return err
		}
	}
	return nil
}

func ensureAppSettings(cfg *config.Config) error {
	settings := map[string]string{
		"project_name":           cfg.ProjectName,
		"cron_schedule":          cfg.CronSchedule,
		"report_cleanup_enabled": fmt.Sprintf("%v", cfg.ReportCleanup.Enabled),
		"report_cleanup_max_age": fmt.Sprintf("%d", cfg.ReportCleanup.MaxAge),
		"report_cleanup_cron":    cfg.ReportCleanup.CronSchedule,
	}
	for key, value := range settings {
		if err := upsertAppSetting(key, value); err != nil {
			return err
		}
	}
	return nil
}

func InitializeTemplatesFromMetricTypes() error {
	var metricTypes []MetricType
	if err := DB.Order("sort_order asc, id asc").Find(&metricTypes).Error; err != nil {
		return err
	}
	for _, mt := range metricTypes {
		if mt.TypeName == "" {
			continue
		}
		var tmpl InspectionTemplate
		err := DB.Where("name = ?", mt.TypeName).First(&tmpl).Error
		desc := fmt.Sprintf("基于「%s」自动创建的默认模板", mt.TypeName)
		if err != nil {
			if err == gorm.ErrRecordNotFound {
				tmpl = InspectionTemplate{Name: mt.TypeName, Description: desc}
				if err := DB.Create(&tmpl).Error; err != nil {
					return err
				}
			} else {
				return err
			}
		} else if tmpl.Description == "" {
			if err := DB.Model(&tmpl).Update("description", desc).Error; err != nil {
				return err
			}
		}

		var configs []MetricConfig
		if err := DB.Where("metric_type_id = ?", mt.ID).Order("sort_order asc, id asc").Find(&configs).Error; err != nil {
			return err
		}
		if err := ensureTemplateLinks(tmpl.ID, configs); err != nil {
			return err
		}
	}

	var global InspectionTemplate
	if err := DB.Where("name = ?", "全局模板").First(&global).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			global = InspectionTemplate{Name: "全局模板", Description: "包含全部全局指标的默认模板"}
			if err := DB.Create(&global).Error; err != nil {
				return err
			}
		} else {
			return err
		}
	}
	var allConfigs []MetricConfig
	if err := DB.Where("metric_type_id IS NOT NULL").Order("metric_type_id asc, sort_order asc, id asc").Find(&allConfigs).Error; err != nil {
		return err
	}
	return ensureTemplateLinks(global.ID, allConfigs)
}

func ensureTemplateLinks(templateID uint, configs []MetricConfig) error {
	for _, cfg := range configs {
		var link InspectionTemplateMetric
		err := DB.Where("template_id = ? AND metric_config_id = ?", templateID, cfg.ID).First(&link).Error
		if err == nil {
			continue
		}
		if err != gorm.ErrRecordNotFound {
			return err
		}
		if err := DB.Create(&InspectionTemplateMetric{
			TemplateID:     templateID,
			MetricConfigID: cfg.ID,
		}).Error; err != nil {
			return err
		}
	}
	return nil
}

func upsertAppSetting(key, value string) error {
	var setting AppSetting
	err := DB.Where("key = ?", key).First(&setting).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return DB.Create(&AppSetting{Key: key, Value: value}).Error
		}
		return err
	}
	return DB.Model(&setting).Update("value", value).Error
}

func marshalJSON(v interface{}) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func defaultThresholdType(v string) string {
	if v == "" {
		return "greater"
	}
	return v
}

func defaultThresholdStatus(v string) string {
	if v == "" {
		return "critical"
	}
	return v
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
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
