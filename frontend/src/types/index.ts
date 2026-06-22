export interface DataSource {
  id?: number
  name: string
  url: string
  username?: string
  password?: string
  is_default?: boolean
  enabled?: boolean
  template_id?: number | null
  notify_channels?: string
  created_at?: string
  health_status?: string
}

export interface MetricConfig {
  id?: number
  metric_type_id: number
  datasource_id?: number | null
  name: string
  description?: string
  query: string
  threshold?: number
  threshold_type?: string
  threshold_status?: string
  unit?: string
  labels_json?: string
  sort_order?: number
  created_at?: string
  updated_at?: string
}

export interface MetricType {
  id?: number
  type_name: string
  sort_order?: number
  configs?: MetricConfig[]
  created_at?: string
  updated_at?: string
}

export interface NotificationChannel {
  id?: number
  channel_type: string
  name: string
  enabled?: boolean
  config_json?: string
  created_at?: string
  updated_at?: string
}

export interface CronJob {
  id?: number
  name: string
  schedule: string
  datasource_id?: number | null
  datasource_ids?: string
  all_datasources?: boolean
  enabled?: boolean
  notify_channels?: string
  last_run_at?: string | null
  last_status?: string
  created_at?: string
  updated_at?: string
}

export interface ReportRecord {
  id?: number
  title: string
  datasource_id?: number | null
  datasource_name: string
  file_path: string
  file_size: number
  total_metrics: number
  alert_count: number
  critical_count: number
  warning_count: number
  status: string
  duration?: string
  created_at?: string
}

export interface DashboardStats {
  total_datasources: number
  total_cronjobs: number
  total_reports: number
  total_notifications: number
  recent_reports: ReportRecord[]
}

export interface InspectRecord {
  id: number
  task_id: string
  status: string
  datasource_name: string
  message: string
  error: string
  report_url: string
  started_at: string
  completed_at: string | null
}

export interface SyncSource {
  id?: number
  name: string
  url: string
  method?: string
  headers?: string
  body?: string
  auth_type?: string
  auth_username?: string
  auth_password?: string
  auth_token?: string
  data_path?: string
  name_field: string
  url_field?: string
  url_template?: string
  username_field?: string
  password_field?: string
  cron_expr?: string
  enabled?: boolean
  created_at?: string
  updated_at?: string
}

export interface SyncLog {
  id: number
  sync_source_id: number
  status: string
  message: string
  total_items: number
  created_items: number
  updated_items: number
  error_items: number
  created_at: string
}

export interface InspectRequest {
  datasource_id?: number
  datasource_url?: string
  wechat_bot_key?: string
  touser?: string
  metric_config_ids?: number[]
}
