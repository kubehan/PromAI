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
	Expr          string  `gorm:"type:text" json:"expr"`
	Threshold     float64 `json:"threshold"`
	ThresholdType string  `gorm:"size:20;default:greater" json:"threshold_type"` // gt/ge/lt/le/eq/ne (复用 MetricConfig 的语义)
	HasThreshold  bool    `json:"has_threshold"`                                 // 是否覆盖指标自带阈值

	// 数据源选择
	DatasourceIDsRaw       string `gorm:"column:datasource_ids;type:text" json:"-"`
	DatasourceIDs          []uint `gorm:"-" json:"datasource_ids,omitempty"`
	DatasourceSelectorJSON string `gorm:"column:datasource_selector;type:text" json:"datasource_selector,omitempty"` // {"tag":"prod","project":"core","all":false}

	// 触发判定
	Severity        string `gorm:"size:20;default:warning;index" json:"severity"` // critical / warning / info
	ForDuration     string `gorm:"size:20" json:"for_duration"`                   // e.g. 5m
	KeepFiringFor   string `gorm:"size:20" json:"keep_firing_for"`                // e.g. 5m
	EvalIntervalSec int    `gorm:"default:0" json:"eval_interval_sec"`            // 0=使用全局间隔

	// 通知频率（优先级高于路由配置）
	RepeatInterval string `gorm:"size:20" json:"repeat_interval"`  // 重复通知间隔, e.g. 4h；空=继承路由
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

	RuleID       uint `gorm:"index;index:idx_ai_rule_state,priority:1" json:"rule_id"`
	DatasourceID uint `gorm:"index;index:idx_ai_ds_state,priority:1" json:"datasource_id"`

	LabelsJSON      string `gorm:"type:text" json:"labels_json"`
	AnnotationsJSON string `gorm:"type:text" json:"annotations_json"`

	State    string `gorm:"size:20;index:idx_ai_rule_state,priority:2;index:idx_ai_ds_state,priority:2;index" json:"state"` // pending / firing / resolved
	Severity string `gorm:"size:20;index" json:"severity"`

	Value     float64 `json:"value"`
	Threshold float64 `json:"threshold"`

	ActiveAt   time.Time  `json:"active_at"`
	FiredAt    *time.Time `json:"fired_at,omitempty"`
	ResolvedAt *time.Time `json:"resolved_at,omitempty"`
	LastEvalAt time.Time  `gorm:"index" json:"last_eval_at"`

	GroupKey        string `gorm:"size:64;index" json:"group_key"`
	SilencedByJSON  string `gorm:"type:text" json:"silenced_by_json"`
	InhibitedByJSON string `gorm:"type:text" json:"inhibited_by_json"`

	NotifiedCount  int        `json:"notified_count"`
	LastNotifiedAt *time.Time `json:"last_notified_at,omitempty"`

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
	EventType       string    `gorm:"size:20;index" json:"event_type"`  // pending/firing/resolved/silenced/inhibited/notified
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
	ID                  uint      `gorm:"primaryKey" json:"id"`
	ParentID            *uint     `gorm:"index" json:"parent_id,omitempty"`
	Name                string    `gorm:"size:200;not null" json:"name"`
	MatchersJSON        string    `gorm:"type:text" json:"matchers_json"`
	Continue            bool      `gorm:"column:cont" json:"continue"`
	GroupByJSON         string    `gorm:"type:text" json:"group_by_json"`
	GroupWait           string    `gorm:"size:20;default:30s" json:"group_wait"`
	GroupInterval       string    `gorm:"size:20;default:5m" json:"group_interval"`
	RepeatInterval      string    `gorm:"size:20;default:4h" json:"repeat_interval"`
	ThrottleWindow      string    `gorm:"size:20;default:''" json:"throttle_window"`
	NotifyChannelIDsRaw string    `gorm:"column:notify_channel_ids;type:text" json:"-"`
	NotifyChannelIDs    []uint    `gorm:"-" json:"notify_channel_ids,omitempty"`
	Priority            int       `gorm:"default:0;index" json:"priority"`
	SendResolved        bool      `gorm:"default:false" json:"send_resolved"`
	MaxSendCount        int       `gorm:"default:0" json:"max_send_count"`
	Enabled             bool      `gorm:"default:true" json:"enabled"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

// AlertGroup 运行时分组聚合状态
type AlertGroup struct {
	ID                   uint       `gorm:"primaryKey" json:"id"`
	GroupKey             string     `gorm:"uniqueIndex;size:64;not null" json:"group_key"`
	RuleID               uint       `gorm:"index" json:"rule_id"`
	DatasourceID         uint       `gorm:"index" json:"datasource_id"`
	RouteID              uint       `gorm:"index" json:"route_id"`
	LabelsJSON           string     `gorm:"type:text" json:"labels_json"`
	AlertCount           int        `json:"alert_count"`
	FirstSeenAt          time.Time  `json:"first_seen_at"`
	LastNotifiedAt       *time.Time `json:"last_notified_at,omitempty"`
	NextNotifyAt         *time.Time `gorm:"index" json:"next_notify_at,omitempty"`
	ResolvedNextNotifyAt *time.Time `json:"resolved_next_notify_at,omitempty"` // 恢复通知批次调度
	SendCount            int        `gorm:"default:0" json:"send_count"`
	State                string     `gorm:"size:20;index" json:"state"` // idle/pending/notified
	CreatedAt            time.Time  `json:"created_at"`
	UpdatedAt            time.Time  `json:"updated_at"`
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
	Content     string    `gorm:"type:text" json:"content"` // 实际发送的内容（markdown）
	SentAt      time.Time `gorm:"index" json:"sent_at"`
	CreatedAt   time.Time `json:"created_at"`
}

type TerminalSummary struct {
	ID              uint       `gorm:"primarykey" json:"id"`
	SessionID       string     `gorm:"uniqueIndex;size:64" json:"session_id"`
	UserID          uint       `gorm:"index" json:"user_id"`
	HostID          uint       `gorm:"index" json:"host_id"`
	IncidentID      *uint      `gorm:"index" json:"incident_id"`
	Model           string     `gorm:"size:32" json:"model"`
	Title           string     `gorm:"size:255" json:"title"`
	Summary         string     `gorm:"type:text" json:"summary"`
	CommandsJSON    string     `gorm:"type:text" json:"commands_json"`
	FindingsJSON    string     `gorm:"type:text" json:"findings_json"`
	ActionsJSON     string     `gorm:"type:text" json:"actions_json"`
	RisksJSON       string     `gorm:"type:text" json:"risks_json"`
	SuggestionsJSON string     `gorm:"type:text" json:"suggestions_json"`
	RawPrompt       string     `gorm:"type:text" json:"-"`
	RawResponse     string     `gorm:"type:text" json:"-"`
	Status          string     `gorm:"size:16" json:"status"`
	GeneratedAt     *time.Time `json:"generated_at"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

// ==================== AIOps models ====================

// Event 统一事件
type Event struct {
	ID                 uint       `gorm:"primarykey" json:"id"`
	EventID            string     `gorm:"uniqueIndex;size:64" json:"event_id"`
	Source             string     `gorm:"index;size:32" json:"source"` // alert/prometheus/manual/other
	Type               string     `gorm:"index;size:32" json:"type"`
	Severity           string     `gorm:"index;size:16" json:"severity"`
	Status             string     `gorm:"index;size:16" json:"status"` // firing/resolved/closed
	Fingerprint        string     `gorm:"index;size:64" json:"fingerprint"`
	Title              string     `gorm:"size:255" json:"title"`
	Summary            string     `gorm:"type:text" json:"summary"`
	LabelsJSON         string     `gorm:"type:text" json:"labels_json"`
	AnnotationsJSON    string     `gorm:"type:text" json:"annotations_json"`
	AlertInstanceID    *uint      `gorm:"index" json:"alert_instance_id,omitempty"`
	RuleID             *uint      `gorm:"index" json:"rule_id,omitempty"`
	DatasourceID       *uint      `gorm:"index" json:"datasource_id,omitempty"`
	IncidentID         *uint      `gorm:"index" json:"incident_id,omitempty"`
	CorrelationGroupID *uint      `gorm:"index" json:"correlation_group_id,omitempty"`
	StartsAt           time.Time  `gorm:"index" json:"starts_at"`
	EndsAt             *time.Time `json:"ends_at,omitempty"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

// Incident 故障单
type Incident struct {
	ID                 uint       `gorm:"primarykey" json:"id"`
	IncidentNo         string     `gorm:"uniqueIndex;size:32" json:"incident_no"`
	Title              string     `gorm:"size:255" json:"title"`
	Description        string     `gorm:"type:text" json:"description"`
	Severity           string     `gorm:"index;size:16" json:"severity"`
	Status             string     `gorm:"index;size:16" json:"status"` // open/acknowledged/investigating/resolved/closed
	Priority           string     `gorm:"size:16" json:"priority"`
	AssigneeID         *uint      `gorm:"index" json:"assignee_id,omitempty"`
	AssigneeName       string     `gorm:"size:64" json:"assignee_name"`
	ReporterID         *uint      `gorm:"index" json:"reporter_id,omitempty"`
	ReporterName       string     `gorm:"size:64" json:"reporter_name"`
	CorrelationGroupID *uint      `gorm:"index" json:"correlation_group_id,omitempty"`
	LabelsJSON         string     `gorm:"type:text" json:"labels_json"`
	StartsAt           time.Time  `gorm:"index" json:"starts_at"`
	AcknowledgedAt     *time.Time `json:"acknowledged_at,omitempty"`
	ResolvedAt         *time.Time `json:"resolved_at,omitempty"`
	ClosedAt           *time.Time `json:"closed_at,omitempty"`
	MTTASeconds        *int       `json:"mtta_seconds,omitempty"`
	MTTRSeconds        *int       `json:"mttr_seconds,omitempty"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

// IncidentTimeline 故障时间线（不可变）
type IncidentTimeline struct {
	ID         uint      `gorm:"primarykey" json:"id"`
	IncidentID uint      `gorm:"index" json:"incident_id"`
	Type       string    `gorm:"index;size:32" json:"type"` // event_created/ack/assign/comment/escalate/ai_rca/runbook/resolve/close
	ActorID    *uint     `gorm:"index" json:"actor_id,omitempty"`
	ActorName  string    `gorm:"size:64" json:"actor_name"`
	Title      string    `gorm:"size:255" json:"title"`
	Content    string    `gorm:"type:text" json:"content"`
	MetaJSON   string    `gorm:"type:text" json:"meta_json"`
	CreatedAt  time.Time `gorm:"index" json:"created_at"`
}

// IncidentRelation 关联关系
type IncidentRelation struct {
	ID          uint      `gorm:"primarykey" json:"id"`
	IncidentID  uint      `gorm:"index" json:"incident_id"`
	RelatedType string    `gorm:"index;size:16" json:"related_type"` // event/change/runbook/cmdb
	RelatedID   uint      `gorm:"index" json:"related_id"`
	Relation    string    `gorm:"size:32" json:"relation"`
	MetaJSON    string    `gorm:"type:text" json:"meta_json"`
	CreatedAt   time.Time `json:"created_at"`
}

// CorrelationGroup 告警关联组
type CorrelationGroup struct {
	ID         uint      `gorm:"primarykey" json:"id"`
	GroupKey   string    `gorm:"uniqueIndex;size:64" json:"group_key"`
	Strategy   string    `gorm:"size:32" json:"strategy"`
	EventCount int       `gorm:"default:0" json:"event_count"`
	IncidentID *uint     `gorm:"index" json:"incident_id,omitempty"`
	Status     string    `gorm:"index;size:16" json:"status"` // open/merged/closed
	LabelsJSON string    `gorm:"type:text" json:"labels_json"`
	FirstAt    time.Time `json:"first_at"`
	LastAt     time.Time `json:"last_at"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// CMDBItem 配置项
type CMDBItem struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	CIID      string    `gorm:"uniqueIndex;size:64" json:"ciid"`
	Name      string    `gorm:"index;size:128" json:"name"`
	Type      string    `gorm:"index;size:32" json:"type"`
	Env       string    `gorm:"index;size:16" json:"env"`
	Cluster   string    `gorm:"index;size:64" json:"cluster"`
	Owner     string    `gorm:"size:64" json:"owner"`
	Team      string    `gorm:"size:64" json:"team"`
	TagsJSON  string    `gorm:"type:text" json:"tags_json"`
	MetaJSON  string    `gorm:"type:text" json:"meta_json"`
	Enabled   bool      `gorm:"default:true" json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// TopologyEdge 拓扑依赖
type TopologyEdge struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	FromCIID  string    `gorm:"index;size:64" json:"from_ciid"`
	ToCIID    string    `gorm:"index;size:64" json:"to_ciid"`
	Type      string    `gorm:"size:32" json:"type"`
	Weight    int       `gorm:"default:1" json:"weight"`
	MetaJSON  string    `gorm:"type:text" json:"meta_json"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ChangeEvent 变更事件
type ChangeEvent struct {
	ID           uint       `gorm:"primarykey" json:"id"`
	ChangeNo     string     `gorm:"uniqueIndex;size:64" json:"change_no"`
	Source       string     `gorm:"size:32" json:"source"`
	Type         string     `gorm:"size:32" json:"type"`
	Title        string     `gorm:"size:255" json:"title"`
	Description  string     `gorm:"type:text" json:"description"`
	Status       string     `gorm:"index;size:16" json:"status"`
	AffectedJSON string     `gorm:"type:text" json:"affected_json"`
	StartedAt    time.Time  `gorm:"index" json:"started_at"`
	FinishedAt   *time.Time `json:"finished_at,omitempty"`
	IncidentID   *uint      `gorm:"index" json:"incident_id,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

// Runbook 知识库
type Runbook struct {
	ID             uint      `gorm:"primarykey" json:"id"`
	Name           string    `gorm:"index;size:255" json:"name"`
	Category       string    `gorm:"index;size:64" json:"category"`
	Content        string    `gorm:"type:text" json:"content"`
	ApplicableJSON string    `gorm:"type:text" json:"applicable_json"`
	TagsJSON       string    `gorm:"type:text" json:"tags_json"`
	UseCount       int       `gorm:"default:0" json:"use_count"`
	Enabled        bool      `gorm:"default:true" json:"enabled"`
	CreatedBy      *uint     `json:"created_by,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// TerminalFixCommand 终端命令执行/审批记录
type TerminalFixCommand struct {
	ID             uint       `gorm:"primarykey" json:"id"`
	ReqID          string     `gorm:"uniqueIndex;size:64" json:"req_id"`
	SessionID      string     `gorm:"index;size:64" json:"session_id"`
	UserID         uint       `gorm:"index" json:"user_id"`
	Username       string     `gorm:"size:64" json:"username"`
	HostID         uint       `gorm:"index" json:"host_id"`
	HostKey        string     `gorm:"size:64" json:"host_key"`
	Command        string     `gorm:"type:text" json:"command"`
	Purpose        string     `gorm:"type:text" json:"purpose"`
	RiskLevel      string     `gorm:"size:16" json:"risk_level"`
	ReviewReason   string     `gorm:"type:text" json:"review_reason"`
	Suggestion     string     `gorm:"type:text" json:"suggestion"`
	Status         string     `gorm:"index;size:16" json:"status"` // pending/approved/rejected/executed/failed
	ApproverID     *uint      `json:"approver_id,omitempty"`
	ApproverName   string     `gorm:"size:64" json:"approver_name"`
	ApproveRemark  string     `gorm:"type:text" json:"approve_remark"`
	ExecutedAt     *time.Time `json:"executed_at,omitempty"`
	ExecutedOutput string     `gorm:"type:text" json:"executed_output"`
	RejectedAt     *time.Time `json:"rejected_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

// OpsSession 自动化运维智能终端：一次排障会话
type OpsSession struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	Title      string    `gorm:"size:200" json:"title"`
	HostID     uint      `gorm:"index" json:"host_id"`
	HostKey    string    `gorm:"size:64;index" json:"host_key"`
	IncidentID *uint     `json:"incident_id"`
	Username   string    `gorm:"size:64;index" json:"username"`
	Status     string    `gorm:"size:16;index;default:running" json:"status"` // running/completed/failed
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// OpsStep 自动化运维智能终端：会话中的命令步骤（含审批状态机）
type OpsStep struct {
	ID           uint       `gorm:"primaryKey" json:"id"`
	SessionID    uint       `gorm:"index;uniqueIndex:idx_session_seq" json:"session_id"`
	Seq          int        `gorm:"uniqueIndex:idx_session_seq" json:"seq"`
	Command      string     `gorm:"type:text" json:"command"`
	Purpose      string     `gorm:"type:text" json:"purpose"`
	RiskLevel    string     `gorm:"size:16" json:"risk_level"`                   // low/medium/high
	Status       string     `gorm:"size:16;index;default:pending" json:"status"` // pending/approved/rejected/running/success/failed
	Output       string     `gorm:"type:text" json:"output"`
	Error        string     `gorm:"type:text" json:"error"`
	ExitCode     int        `json:"exit_code"`
	ApprovedBy   string     `gorm:"size:64" json:"approved_by"`
	RejectReason string     `gorm:"type:text" json:"reject_reason"`
	AutoApproved bool       `json:"auto_approved"`
	ApprovedAt   *time.Time `json:"approved_at"`
	ExecutedAt   *time.Time `json:"executed_at"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

// OpsSessionMessage 自动化运维会话中的交互消息（用户/AI/命令/结果）
type OpsSessionMessage struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	SessionID uint      `gorm:"index;uniqueIndex:idx_ops_session_msg_seq" json:"session_id"`
	Seq       int       `gorm:"uniqueIndex:idx_ops_session_msg_seq" json:"seq"`
	Role      string    `gorm:"size:24;index" json:"role"`
	Text      string    `gorm:"type:text" json:"text"`
	Command   string    `gorm:"type:text" json:"command"`
	Risk      string    `gorm:"size:16" json:"risk"`
	Status    string    `gorm:"size:16" json:"status"`
	ExitCode  int       `json:"exit_code"`
	Note      string    `gorm:"type:text" json:"note"`
	StepID    uint      `gorm:"index" json:"step_id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// OpsSessionShare 自动化运维会话分享链接（可免密或密码访问）
type OpsSessionShare struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	SessionID    uint      `gorm:"index" json:"session_id"`
	Token        string    `gorm:"size:64;uniqueIndex" json:"token"`
	PasswordHash string    `gorm:"size:128" json:"-"`
	ExpiresAt    time.Time `gorm:"index" json:"expires_at"`
	CreatedBy    string    `gorm:"size:64;index" json:"created_by"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// ==================== 主机管理 / Web 终端 ====================

// TerminalHost 受管主机（CMDB 主机属性 + SSH 登录配置）
type TerminalHost struct {
	ID                 uint       `gorm:"primarykey" json:"id"`
	Name               string     `gorm:"size:128;index" json:"name"`
	HostKey            string     `gorm:"uniqueIndex;size:128" json:"host_key"`
	Address            string     `gorm:"size:255" json:"address"`
	Port               int        `gorm:"default:22" json:"port"`
	Username           string     `gorm:"size:128" json:"username"`
	AuthType           string     `gorm:"size:16;default:password" json:"auth_type"` // password / private_key
	PasswordEnc        string     `gorm:"type:text" json:"-"`
	PrivateKeyEnc      string     `gorm:"type:text" json:"-"`
	PassphraseEnc      string     `gorm:"type:text" json:"-"`
	HostKeyFingerprint string     `gorm:"size:255" json:"host_key_fingerprint"`
	Env                string     `gorm:"size:32;index" json:"env"`
	Group              string     `gorm:"size:64;index" json:"group"`
	TagsJSON           string     `gorm:"type:text" json:"tags_json"`
	Description        string     `gorm:"type:text" json:"description"`
	Enabled            bool       `gorm:"default:true;index" json:"enabled"`
	LastStatus         string     `gorm:"size:16;index" json:"last_status"` // online / offline / unknown
	LastCheckedAt      *time.Time `json:"last_checked_at,omitempty"`
	LastError          string     `gorm:"size:500" json:"last_error"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

// TerminalHostCommand 主机自定义命令（登录后可一键执行 / 自动执行）
type TerminalHostCommand struct {
	ID          uint      `gorm:"primarykey" json:"id"`
	HostID      uint      `gorm:"index;not null" json:"host_id"`
	Name        string    `gorm:"size:128;not null" json:"name"`
	Command     string    `gorm:"type:text;not null" json:"command"`
	Description string    `gorm:"type:text" json:"description"`
	AutoRun     bool      `gorm:"default:false" json:"auto_run"`
	SortOrder   int       `gorm:"default:0" json:"sort_order"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// TerminalExecLog 主机命令执行审计日志
type TerminalExecLog struct {
	ID         uint      `gorm:"primarykey" json:"id"`
	HostID     uint      `gorm:"index" json:"host_id"`
	HostKey    string    `gorm:"size:128;index" json:"host_key"`
	Username   string    `gorm:"size:64" json:"username"`
	Source     string    `gorm:"size:16" json:"source"` // manual / quick / auto / websocket
	Command    string    `gorm:"type:text" json:"command"`
	ExitCode   int       `json:"exit_code"`
	Output     string    `gorm:"type:text" json:"output"`
	Error      string    `gorm:"size:500" json:"error"`
	DurationMS int64     `json:"duration_ms"`
	CreatedAt  time.Time `gorm:"index" json:"created_at"`
}

func AutoMigrate(db *gorm.DB) error {
	if err := db.AutoMigrate(
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
		// AIOps
		&Event{},
		&Incident{},
		&IncidentTimeline{},
		&IncidentRelation{},
		&CorrelationGroup{},
		&CMDBItem{},
		&TopologyEdge{},
		&ChangeEvent{},
		&Runbook{},
		// 主机管理 / Web 终端
		&TerminalHost{},
		&TerminalHostCommand{},
		&TerminalExecLog{},
		&TerminalSummary{},
		&TerminalFixCommand{},
		&OpsSession{},
		&OpsStep{},
		&OpsSessionMessage{},
		&OpsSessionShare{},
	); err != nil {
		return err
	}
	// 兼容早期 terminal_hosts 表：旧表使用 host 字段，迁移到 address。
	var legacyHostColumn int64
	if err := db.Raw("SELECT count(*) FROM pragma_table_info('terminal_hosts') WHERE name = 'host'").Scan(&legacyHostColumn).Error; err == nil && legacyHostColumn > 0 {
		if err := db.Exec("UPDATE terminal_hosts SET address = `host` WHERE (address IS NULL OR address = '') AND `host` IS NOT NULL AND `host` <> ''").Error; err != nil {
			return err
		}
	}
	return nil
}
