package config

import "PromAI/pkg/notify"

type AuthConfig struct {
	Username   string `yaml:"username"`
	Password   string `yaml:"password"`
	JWTSecret  string `yaml:"jwt_secret"`
}

type AIConfig struct {
	Enabled      bool             `yaml:"enabled"`
	DefaultModel string           `yaml:"default_model"`
	Models       []AIModelConfig  `yaml:"models"`
}

type AIModelConfig struct {
	Name          string `yaml:"name" json:"name"`
	Provider      string `yaml:"provider" json:"provider"`
	Model         string `yaml:"model" json:"model"`
	BaseURL       string `yaml:"base_url" json:"base_url"`
	APIKey        string `yaml:"api_key" json:"api_key,omitempty"`
	ThinkingLevel string `yaml:"thinking_level" json:"thinking_level"`
	MaxTokens     int    `yaml:"max_tokens" json:"max_tokens"`
}

type Config struct {
	PrometheusURL      string       `yaml:"prometheus_url"`
	PrometheusUsername string       `yaml:"prometheus_username"`
	PrometheusPassword string       `yaml:"prometheus_password"`
	Auth               AuthConfig   `yaml:"auth"`
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
	Port string   `yaml:"port"`
	AI   AIConfig `yaml:"ai"`
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
