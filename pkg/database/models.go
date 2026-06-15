package database

import (
	"time"

	"gorm.io/gorm"
)

type DataSource struct {
	ID             uint      `gorm:"primaryKey" json:"id"`
	Name           string    `gorm:"uniqueIndex;size:100;not null" json:"name"`
	URL            string    `gorm:"size:500;not null" json:"url"`
	Username       string    `gorm:"size:100" json:"username"`
	Password       string    `gorm:"size:100" json:"password"`
	IsDefault      bool      `gorm:"default:false" json:"is_default"`
	Enabled        bool      `gorm:"default:true" json:"enabled"`
	TemplateID     *uint     `json:"template_id"`
	NotifyChannels string    `gorm:"type:text" json:"notify_channels"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type MetricType struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	TypeName  string         `gorm:"uniqueIndex;size:200;not null" json:"type_name"`
	SortOrder int            `gorm:"default:0" json:"sort_order"`
	Configs   []MetricConfig `gorm:"foreignKey:MetricTypeID" json:"configs"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
}

type MetricConfig struct {
	ID              uint   `gorm:"primaryKey" json:"id"`
	MetricTypeID    uint   `gorm:"not null;index" json:"metric_type_id"`
	DatasourceID    *uint  `gorm:"index" json:"datasource_id"`
	Name            string `gorm:"size:200;not null" json:"name"`
	Description     string `gorm:"size:500" json:"description"`
	Query           string `gorm:"type:text;not null" json:"query"`
	Threshold       float64 `json:"threshold"`
	ThresholdType   string `gorm:"size:50;default:greater" json:"threshold_type"`
	ThresholdStatus string `gorm:"size:50;default:critical" json:"threshold_status"`
	Unit            string `gorm:"size:50" json:"unit"`
	LabelsJSON      string `gorm:"type:text" json:"labels_json"`
	SortOrder       int    `gorm:"default:0" json:"sort_order"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type NotificationChannel struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	ChannelType string    `gorm:"size:50;not null;index" json:"channel_type"`
	Name        string    `gorm:"size:200;not null" json:"name"`
	Enabled     bool      `gorm:"default:false" json:"enabled"`
	ConfigJSON  string    `gorm:"type:text" json:"config_json"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type CronJob struct {
	ID              uint       `gorm:"primaryKey" json:"id"`
	Name            string     `gorm:"size:200;not null" json:"name"`
	Schedule        string     `gorm:"size:100;not null" json:"schedule"`
	DatasourceID    *uint      `json:"datasource_id"`               // 旧版单数据源（向后兼容）
	DatasourceIDs   string     `gorm:"type:text" json:"datasource_ids"`     // 多数据源 JSON 数组
	AllDatasources  bool       `json:"all_datasources"`                      // 全部数据源
	Enabled         bool       `gorm:"default:true" json:"enabled"`
	NotifyChannels  string     `gorm:"type:text" json:"notify_channels"`
	LastRunAt       *time.Time `json:"last_run_at"`
	LastStatus      string     `gorm:"size:50" json:"last_status"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

type ReportRecord struct {
	ID              uint      `gorm:"primaryKey" json:"id"`
	Title           string    `gorm:"size:200" json:"title"`
	DatasourceID    *uint     `json:"datasource_id"`
	DatasourceName  string    `gorm:"size:200" json:"datasource_name"`
	FilePath        string    `gorm:"size:500" json:"file_path"`
	FileSize        int64     `json:"file_size"`
	TotalMetrics    int       `json:"total_metrics"`
	AlertCount      int       `json:"alert_count"`
	CriticalCount   int       `json:"critical_count"`
	WarningCount    int       `json:"warning_count"`
	Status          string    `gorm:"size:50" json:"status"`
	Duration        string    `gorm:"size:50" json:"duration"`
	MetricsJSON     string    `gorm:"type:text" json:"metrics_json"`
	CreatedAt       time.Time `json:"created_at"`
}

type AppSetting struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Key       string    `gorm:"uniqueIndex;size:100;not null" json:"key"`
	Value     string    `gorm:"type:text" json:"value"`
	UpdatedAt time.Time `json:"updated_at"`
}

type InspectionTemplate struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	Name        string    `gorm:"size:200;not null" json:"name"`
	Description string    `gorm:"size:500" json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type InspectionTemplateMetric struct {
	TemplateID     uint `gorm:"primaryKey;autoIncrement:false" json:"template_id"`
	MetricConfigID uint `gorm:"primaryKey;autoIncrement:false" json:"metric_config_id"`
}

type TemplateMetricOverride struct {
	ID              uint    `gorm:"primaryKey" json:"id"`
	TemplateID      uint    `gorm:"not null;index;uniqueIndex:idx_tmpl_override" json:"template_id"`
	MetricConfigID  uint    `gorm:"not null;index;uniqueIndex:idx_tmpl_override" json:"metric_config_id"`
	Query           string  `gorm:"type:text" json:"query"`
	Threshold       float64 `json:"threshold"`
	ThresholdType   string  `gorm:"size:50;default:greater" json:"threshold_type"`
	ThresholdStatus string  `gorm:"size:50;default:critical" json:"threshold_status"`
	Unit            string  `gorm:"size:50" json:"unit"`
	LabelsJSON      string  `gorm:"type:text" json:"labels_json"`
}

// InspectRecord 巡检任务记录（持久化）
type InspectRecord struct {
	ID             uint       `gorm:"primaryKey" json:"id"`
	TaskID         string     `gorm:"size:100;index" json:"task_id"`
	Status         string     `gorm:"size:50;default:running" json:"status"`
	DatasourceID   *uint      `json:"datasource_id"`
	DatasourceName string     `gorm:"size:200" json:"datasource_name"`
	Message        string     `gorm:"size:500" json:"message"`
	Error          string     `gorm:"size:500" json:"error"`
	ReportURL      string     `gorm:"size:500" json:"report_url"`
	StartedAt      time.Time  `json:"started_at"`
	CompletedAt    *time.Time `json:"completed_at"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

// Apply applies a template-level override onto a MetricConfig
func (o *TemplateMetricOverride) Apply(cfg *MetricConfig) {
	if o.ID == 0 {
		return
	}
	cfg.Query = o.Query
	cfg.Threshold = o.Threshold
	cfg.ThresholdType = o.ThresholdType
	cfg.ThresholdStatus = o.ThresholdStatus
	cfg.Unit = o.Unit
	cfg.LabelsJSON = o.LabelsJSON
}

type SyncSource struct {
	ID            uint      `gorm:"primaryKey" json:"id"`
	Name          string    `gorm:"size:200;not null" json:"name"`
	URL           string    `gorm:"size:500;not null" json:"url"`
	Method        string    `gorm:"size:10;default:GET" json:"method"`
	Headers       string    `gorm:"type:text" json:"headers"`      // JSON object
	Body          string    `gorm:"type:text" json:"body"`         // request body template
	AuthType      string    `gorm:"size:20;default:none" json:"auth_type"` // none, basic, bearer
	AuthUsername  string    `gorm:"size:100" json:"auth_username"`
	AuthPassword  string    `gorm:"size:100" json:"auth_password"`
	AuthToken     string    `gorm:"size:500" json:"auth_token"`
	DataPath      string    `gorm:"size:200" json:"data_path"`     // JSON path to data array (e.g. "data.items")
	NameField     string    `gorm:"size:100;not null" json:"name_field"`
	URLField      string    `gorm:"size:100" json:"url_field"`
	URLTemplate   string    `gorm:"size:500" json:"url_template"`  // e.g. http://{host}:{port}
	UsernameField string    `gorm:"size:100" json:"username_field"`
	PasswordField string    `gorm:"size:100" json:"password_field"`
	CronExpr      string    `gorm:"size:50" json:"cron_expr"`
	Enabled       bool      `gorm:"default:true" json:"enabled"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type SyncLog struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	SyncSourceID uint      `json:"sync_source_id"`
	Status       string    `gorm:"size:20" json:"status"` // success, partial, failed
	Message      string    `gorm:"type:text" json:"message"`
	TotalItems   int       `json:"total_items"`
	CreatedItems int       `json:"created_items"`
	UpdatedItems int       `json:"updated_items"`
	ErrorItems   int       `json:"error_items"`
	CreatedAt    time.Time `json:"created_at"`
}

type AiSession struct {
	ID        string      `gorm:"primaryKey;size:100" json:"id"`
	ModelName string      `gorm:"size:100" json:"model_name"`
	CreatedAt time.Time   `json:"created_at"`
	UpdatedAt time.Time   `json:"updated_at"`
	Messages  []AiMessage `gorm:"foreignKey:SessionID;constraint:OnDelete:CASCADE" json:"messages,omitempty"`
}

type AiMessage struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	SessionID string    `gorm:"index;size:100;not null" json:"session_id"`
	Role      string    `gorm:"size:20;not null" json:"role"`
	Content   string    `gorm:"type:text" json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

func AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&DataSource{},
		&MetricType{},
		&MetricConfig{},
		&NotificationChannel{},
		&CronJob{},
		&ReportRecord{},
		&AppSetting{},
		&InspectionTemplate{},
		&InspectionTemplateMetric{},
		&TemplateMetricOverride{},
		&InspectRecord{},
		&SyncSource{},
		&SyncLog{},
		&AiSession{},
		&AiMessage{},
	)
}
