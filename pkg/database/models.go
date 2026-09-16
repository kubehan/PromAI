package database

import (
	"time"

	"gorm.io/gorm"
)

type DataSource struct {
	ID                  uint       `gorm:"primaryKey" json:"id"`
	Name                string     `gorm:"uniqueIndex;size:100;not null" json:"name"`
	URL                 string     `gorm:"size:500;not null" json:"url"`
	Username            string     `gorm:"size:100" json:"username"`
	Password            string     `gorm:"size:100" json:"password"`
	IsDefault           bool       `gorm:"default:false" json:"is_default"`
	Enabled             bool       `gorm:"default:true" json:"enabled"`
	TemplateID          *uint      `json:"template_id"`
	TemplateIDsRaw      string     `gorm:"column:template_ids;type:text" json:"-"`
	TemplateIDs         []uint     `gorm:"-" json:"template_ids,omitempty"`
	ProjectName         string     `gorm:"size:200" json:"project_name"`
	ConnectionStatus    string     `gorm:"size:50" json:"connection_status"`
	ConnectionCheckedAt *time.Time `json:"connection_checked_at,omitempty"`
	NotifyChannels      string     `gorm:"type:text" json:"notify_channels"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
	HealthStatus        string     `gorm:"-" json:"health_status"`
	ReportStatus        string     `gorm:"-" json:"report_status,omitempty"`
	LastReportAt        *time.Time `gorm:"-" json:"last_report_at,omitempty"`
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
	ID              uint      `gorm:"primaryKey" json:"id"`
	MetricTypeID    uint      `gorm:"not null;index" json:"metric_type_id"`
	DatasourceID    *uint     `gorm:"index" json:"datasource_id"`
	Name            string    `gorm:"size:200;not null" json:"name"`
	Description     string    `gorm:"size:500" json:"description"`
	Query           string    `gorm:"type:text;not null" json:"query"`
	Threshold       float64   `json:"threshold"`
	ThresholdType   string    `gorm:"size:50;default:greater" json:"threshold_type"`
	ThresholdStatus string    `gorm:"size:50;default:critical" json:"threshold_status"`
	Unit            string    `gorm:"size:50" json:"unit"`
	LabelsJSON      string    `gorm:"type:text" json:"labels_json"`
	SortOrder       int       `gorm:"default:0" json:"sort_order"`
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
	ID             uint       `gorm:"primaryKey" json:"id"`
	Name           string     `gorm:"size:200;not null" json:"name"`
	Schedule       string     `gorm:"size:100;not null" json:"schedule"`
	DatasourceID   *uint      `json:"datasource_id"`                   // 旧版单数据源（向后兼容）
	DatasourceIDs  string     `gorm:"type:text" json:"datasource_ids"` // 多数据源 JSON 数组
	AllDatasources bool       `json:"all_datasources"`                 // 全部数据源
	Enabled        bool       `gorm:"default:true" json:"enabled"`
	NotifyChannels string     `gorm:"type:text" json:"notify_channels"`
	LastRunAt      *time.Time `json:"last_run_at"`
	LastStatus     string     `gorm:"size:50" json:"last_status"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

type ReportRecord struct {
	ID             uint      `gorm:"primaryKey" json:"id"`
	Title          string    `gorm:"size:200" json:"title"`
	DatasourceID   *uint     `json:"datasource_id"`
	DatasourceName string    `gorm:"size:200" json:"datasource_name"`
	FilePath       string    `gorm:"size:500" json:"file_path"`
	FileSize       int64     `json:"file_size"`
	TotalMetrics   int       `json:"total_metrics"`
	AlertCount     int       `json:"alert_count"`
	CriticalCount  int       `json:"critical_count"`
	WarningCount   int       `json:"warning_count"`
	Status         string    `gorm:"size:50" json:"status"`
	Duration       string    `gorm:"size:50" json:"duration"`
	MetricsJSON    string    `gorm:"type:text" json:"metrics_json"`
	Content        string    `gorm:"type:text" json:"content"` // 在线编辑的报告 Markdown 内容（为空时自动生成）
	CreatedAt      time.Time `json:"created_at"`
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
	Headers       string    `gorm:"type:text" json:"headers"`              // JSON object
	Body          string    `gorm:"type:text" json:"body"`                 // request body template
	AuthType      string    `gorm:"size:20;default:none" json:"auth_type"` // none, basic, bearer
	AuthUsername  string    `gorm:"size:100" json:"auth_username"`
	AuthPassword  string    `gorm:"size:100" json:"auth_password"`
	AuthToken     string    `gorm:"size:500" json:"auth_token"`
	DataPath      string    `gorm:"size:200" json:"data_path"` // JSON path to data array (e.g. "data.items")
	NameField     string    `gorm:"size:100;not null" json:"name_field"`
	URLField      string    `gorm:"size:100" json:"url_field"`
	URLTemplate   string    `gorm:"size:500" json:"url_template"` // e.g. http://{host}:{port}
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

// SkillUsage 记录每次 AI 会话中 Skill 的曝光（被注入系统提示词）
// 用于统计 Skill 使用趋势
type SkillUsage struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	SkillName string    `gorm:"index:idx_skill_usage_name_day;size:100;not null" json:"skill_name"`
	SessionID string    `gorm:"index;size:100;not null" json:"session_id"`
	Day       string    `gorm:"index:idx_skill_usage_name_day;size:10;not null" json:"day"` // YYYY-MM-DD
	CreatedAt time.Time `json:"created_at"`
}

func (SkillUsage) TableName() string { return "skill_usage" }

// ===== Alerting 子系统模型 =====================================================
//
// AlertRule       告警规则定义（可复用 MetricConfig，也可自定义 PromQL）
// AlertInstance   实时告警实例（按 fingerprint 去重）
// AlertHistory    告警历史归档（状态变迁追加，便于回溯/统计）
// AlertSilence    静默规则（按 label matcher 静默匹配的活跃告警）
// AlertInhibit    抑制规则（高优先级告警抑制低优先级告警）
// AlertRoute      通知路由树（扁平表，parent_id 0 表示根）
// AlertGroup      运行时分组聚合状态（dispatcher 调度通知的最小单位）
// AlertNotifyLog  通知发送日志（成功/失败/降级）
// AlertMatcher    通用 matcher（=, !=, =~, !~）以 JSON 嵌入到字符串字段
//
// 字段约定：所有 JSON 字段统一存为 string（type:text），由上层 Marshal/Unmarshal。
// =================================================================================

// AlertRule 告警规则
//   - SourceType=metric  时使用 MetricConfigID 引用现有指标，无需重复维护 PromQL
//   - SourceType=custom  时使用 Expr/Threshold/ThresholdType 字段，自定义 PromQL
//   - 数据源选择：DatasourceIDs（显式列表）和 DatasourceSelector（按 tag/project 批量）二选一
type AlertRule struct {
	ID          uint   `gorm:"primaryKey" json:"id"`
	Name        string `gorm:"size:200;not null;index" json:"name"`
	Description string `gorm:"size:500" json:"description"`

	// 规则来源
	SourceType     string `gorm:"size:20;default:metric;index" json:"source_type"` // metric / custom
	MetricConfigID *uint  `gorm:"index" json:"metric_config_id,omitempty"`

	// 关联巡检模版（可选）
	TemplateID *uint `gorm:"index" json:"template_id,omitempty"`

	// 自定义 PromQL（SourceType=custom 时使用，metric 时可作为覆盖）
	Expr            string  `gorm:"type:text" json:"expr"`
	Threshold       float64 `json:"threshold"`
	ThresholdType   string  `gorm:"size:20;default:greater" json:"threshold_type"` // gt/ge/lt/le/eq/ne (复用 MetricConfig 的语义)
	HasThreshold    bool    `json:"has_threshold"`                                 // 是否覆盖指标自带阈值

	// 数据源选择
	DatasourceIDsRaw     string `gorm:"column:datasource_ids;type:text" json:"-"`
	DatasourceIDs        []uint `gorm:"-" json:"datasource_ids,omitempty"`
	DatasourceSelectorJSON string `gorm:"column:datasource_selector;type:text" json:"datasource_selector,omitempty"` // {"tag":"prod","project":"core","all":false}

	// 触发判定
	Severity        string `gorm:"size:20;default:warning;index" json:"severity"` // critical / warning / info
	ForDuration     string `gorm:"size:20" json:"for_duration"`                   // e.g. 5m
	KeepFiringFor   string `gorm:"size:20" json:"keep_firing_for"`                // e.g. 5m
	EvalIntervalSec int    `gorm:"default:0" json:"eval_interval_sec"`            // 0=使用全局间隔

	// 通知频率（优先级高于路由配置）
	RepeatInterval string `gorm:"size:20" json:"repeat_interval"` // 重复通知间隔, e.g. 4h；空=继承路由
	MaxSendCount   int    `gorm:"default:0" json:"max_send_count"` // 最大发送次数；0=继承路由

	// 元数据
	LabelsJSON      string `gorm:"type:text" json:"labels_json"`      // {"team":"infra"}
	AnnotationsJSON string `gorm:"type:text" json:"annotations_json"` // {"summary":"...","description":"...","runbook_url":"..."}

	// 告警原因 & 影响范围（用户在创建规则时填写，发通知时附上）
	Cause  string `gorm:"type:text" json:"cause"`
	Impact string `gorm:"type:text" json:"impact"`

	// 通知路径（二选一；route_id 优先）
	RouteID             *uint  `gorm:"index" json:"route_id,omitempty"`
	NotifyChannelIDsRaw string `gorm:"column:notify_channel_ids;type:text" json:"-"`
	NotifyChannelIDs    []uint `gorm:"-" json:"notify_channel_ids,omitempty"`

	Enabled   bool      `gorm:"default:true;index" json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// AlertInstance 当前活跃 / 待恢复的告警实例（热表）
type AlertInstance struct {
	ID          uint   `gorm:"primaryKey" json:"id"`
	Fingerprint string `gorm:"uniqueIndex;size:64;not null" json:"fingerprint"`

	RuleID       uint  `gorm:"index;index:idx_ai_rule_state,priority:1" json:"rule_id"`
	DatasourceID uint  `gorm:"index;index:idx_ai_ds_state,priority:1" json:"datasource_id"`

	LabelsJSON      string `gorm:"type:text" json:"labels_json"`
	AnnotationsJSON string `gorm:"type:text" json:"annotations_json"`

	State    string `gorm:"size:20;index:idx_ai_rule_state,priority:2;index:idx_ai_ds_state,priority:2;index" json:"state"` // pending / firing / resolved
	Severity string `gorm:"size:20;index" json:"severity"`

	Value     float64 `json:"value"`
	Threshold float64 `json:"threshold"`

	ActiveAt    time.Time  `json:"active_at"`
	FiredAt     *time.Time `json:"fired_at,omitempty"`
	ResolvedAt  *time.Time `json:"resolved_at,omitempty"`
	LastEvalAt  time.Time  `gorm:"index" json:"last_eval_at"`

	GroupKey       string `gorm:"size:64;index" json:"group_key"`
	SilencedByJSON string `gorm:"type:text" json:"silenced_by_json"`
	InhibitedByJSON string `gorm:"type:text" json:"inhibited_by_json"`

	NotifiedCount   int        `json:"notified_count"`
	LastNotifiedAt  *time.Time `json:"last_notified_at,omitempty"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// AlertHistory 告警状态变迁追加表，单条事件不可变
type AlertHistory struct {
	ID              uint      `gorm:"primaryKey" json:"id"`
	Fingerprint     string    `gorm:"index;size:64" json:"fingerprint"`
	RuleID          uint      `gorm:"index" json:"rule_id"`
	RuleName        string    `gorm:"size:200" json:"rule_name"`
	DatasourceID    uint      `gorm:"index" json:"datasource_id"`
	DatasourceName  string    `gorm:"size:200" json:"datasource_name"`
	State           string    `gorm:"size:20;index" json:"state"`
	Severity        string    `gorm:"size:20;index" json:"severity"`
	Value           float64   `json:"value"`
	Threshold       float64   `json:"threshold"`
	LabelsJSON      string    `gorm:"type:text" json:"labels_json"`
	AnnotationsJSON string    `gorm:"type:text" json:"annotations_json"`
	EventType       string    `gorm:"size:20;index" json:"event_type"` // pending/firing/resolved/silenced/inhibited/notified
	NotifyChannels  string    `gorm:"type:text" json:"notify_channels"` // 通知渠道 JSON: [{"id":1,"type":"wechat_work","name":"企业微信"}]
	NotifyResult    string    `gorm:"size:20" json:"notify_result"`     // 通知结果: success/failed/throttled
	OccurredAt      time.Time `gorm:"index" json:"occurred_at"`
	CreatedAt       time.Time `json:"created_at"`
}

// AlertSilence 静默规则
type AlertSilence struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	Comment      string    `gorm:"size:500;not null" json:"comment"`
	CreatedBy    string    `gorm:"size:100" json:"created_by"`
	MatchersJSON string    `gorm:"type:text;not null" json:"matchers_json"`
	StartsAt     time.Time `gorm:"index" json:"starts_at"`
	EndsAt       time.Time `gorm:"index" json:"ends_at"`
	Enabled      bool      `gorm:"default:true;index" json:"enabled"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// AlertInhibit 抑制规则
type AlertInhibit struct {
	ID                 uint      `gorm:"primaryKey" json:"id"`
	Name               string    `gorm:"size:200;not null" json:"name"`
	SourceMatchersJSON string    `gorm:"type:text;not null" json:"source_matchers_json"`
	TargetMatchersJSON string    `gorm:"type:text;not null" json:"target_matchers_json"`
	EqualLabelsJSON    string    `gorm:"type:text" json:"equal_labels_json"` // ["cluster","instance"]
	Enabled            bool      `gorm:"default:true;index" json:"enabled"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

// AlertRoute 通知路由（扁平表模拟树）
type AlertRoute struct {
	ID                  uint   `gorm:"primaryKey" json:"id"`
	ParentID            *uint  `gorm:"index" json:"parent_id,omitempty"`
	Name                string `gorm:"size:200;not null" json:"name"`
	MatchersJSON        string `gorm:"type:text" json:"matchers_json"`
	Continue            bool   `gorm:"column:cont" json:"continue"`
	GroupByJSON         string `gorm:"type:text" json:"group_by_json"`
	GroupWait           string `gorm:"size:20;default:30s" json:"group_wait"`
	GroupInterval       string `gorm:"size:20;default:5m" json:"group_interval"`
	RepeatInterval      string `gorm:"size:20;default:4h" json:"repeat_interval"`
	ThrottleWindow      string `gorm:"size:20;default:''" json:"throttle_window"`
	NotifyChannelIDsRaw string `gorm:"column:notify_channel_ids;type:text" json:"-"`
	NotifyChannelIDs    []uint `gorm:"-" json:"notify_channel_ids,omitempty"`
	Priority            int    `gorm:"default:0;index" json:"priority"`
	SendResolved        bool   `gorm:"default:false" json:"send_resolved"`
	MaxSendCount        int    `gorm:"default:0" json:"max_send_count"`
	Enabled             bool   `gorm:"default:true" json:"enabled"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

// AlertGroup 运行时分组聚合状态
type AlertGroup struct {
	ID           uint   `gorm:"primaryKey" json:"id"`
	GroupKey     string `gorm:"uniqueIndex;size:64;not null" json:"group_key"`
	RuleID       uint   `gorm:"index" json:"rule_id"`
	DatasourceID uint   `gorm:"index" json:"datasource_id"`
	RouteID      uint   `gorm:"index" json:"route_id"`
	LabelsJSON   string `gorm:"type:text" json:"labels_json"`
	AlertCount          int        `json:"alert_count"`
	FirstSeenAt         time.Time  `json:"first_seen_at"`
	LastNotifiedAt      *time.Time `json:"last_notified_at,omitempty"`
	NextNotifyAt        *time.Time `gorm:"index" json:"next_notify_at,omitempty"`
	ResolvedNextNotifyAt *time.Time `json:"resolved_next_notify_at,omitempty"` // 恢复通知批次调度
	SendCount           int        `gorm:"default:0" json:"send_count"`
	State               string     `gorm:"size:20;index" json:"state"` // idle/pending/notified
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
}

// AlertNotifyLog 通知发送日志
type AlertNotifyLog struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	GroupKey    string    `gorm:"size:64;index" json:"group_key"`
	RuleID      uint      `gorm:"index" json:"rule_id"`
	ChannelID   uint      `gorm:"index" json:"channel_id"`
	ChannelType string    `gorm:"size:50" json:"channel_type"`
	Status      string    `gorm:"size:20;index" json:"status"` // success / failed / throttled
	Error       string    `gorm:"size:500" json:"error"`
	PayloadHash string    `gorm:"size:64;index" json:"payload_hash"`
	AlertCount  int       `json:"alert_count"`
	Content     string    `gorm:"type:text" json:"content"`              // 实际发送的内容（markdown）
	SentAt      time.Time `gorm:"index" json:"sent_at"`
	CreatedAt   time.Time `json:"created_at"`
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
		&SkillUsage{},
		// alerting
		&AlertRule{},
		&AlertInstance{},
		&AlertHistory{},
		&AlertSilence{},
		&AlertInhibit{},
		&AlertRoute{},
		&AlertGroup{},
		&AlertNotifyLog{},
	)
}
