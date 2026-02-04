package config

import "PromAI/pkg/notify"

type Config struct {
	PrometheusURL      string       `yaml:"prometheus_url"`
	PrometheusUsername string       `yaml:"prometheus_username"`
	PrometheusPassword string       `yaml:"prometheus_password"`
	DataSources        []DataSource `yaml:"data_sources"`
	MetricTypes        []MetricType `yaml:"metric_types"`
	ProjectName        string       `yaml:"project_name"`
	CronSchedule       string       `yaml:"cron_schedule"`
	ReportCleanup      struct {
		Enabled      bool   `yaml:"enabled"`
		MaxAge       int    `yaml:"max_age"`
		CronSchedule string `yaml:"cron_schedule"`
	} `yaml:"report_cleanup"`
	Notifications struct {
		Dingtalk   notify.DingtalkConfig   `yaml:"dingtalk"`
		Email      notify.EmailConfig      `yaml:"email"`
		WeChatWork notify.WeChatWorkConfig `yaml:"wechat_work"`
		WeChatApp  notify.WeChatAppConfig  `yaml:"wechat_app"`
		Feishu     notify.FeishuConfig     `yaml:"feishu"`
	} `yaml:"notifications"`
	Port string `yaml:"port"`
}

type DataSource struct {
	Name     string `yaml:"name"`
	URL      string `yaml:"url"`
	UserName string `yaml:"username"`
	Password string `yaml:"password"`
}

type MetricType struct {
	Type    string         `yaml:"type"`
	Metrics []MetricConfig `yaml:"metrics"`
}

type MetricConfig struct {
	Name            string            `yaml:"name"`
	Description     string            `yaml:"description"`
	Query           string            `yaml:"query"`
	Threshold       float64           `yaml:"threshold"`
	Unit            string            `yaml:"unit"`
	Labels          map[string]string `yaml:"labels"`
	ThresholdType   string            `yaml:"threshold_type"`
	ThresholdStatus string            `yaml:"threshold_status"`
}
