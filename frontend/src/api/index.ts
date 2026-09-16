import axios from 'axios'
import router from '../router'
import type {
  DataSource, MetricType, MetricConfig,
  NotificationChannel, CronJob, ReportRecord,
  InspectRecord, InspectRequest, DashboardStats,
  SyncSource, SyncLog, AiSkill
} from '../types'

const api = axios.create({
  baseURL: '/api/promai',
  timeout: 30000,
})

api.interceptors.request.use(config => {
  const token = localStorage.getItem('token')
  if (token) {
    config.headers.Authorization = `Bearer ${token}`
  }
  return config
})

api.interceptors.response.use(
  r => r,
  e => {
    if (e.response?.status === 401) {
      localStorage.removeItem('token')
      localStorage.removeItem('username')
      router.push('/login')
    }
    const msg = e.response?.data?.error || e.message || '请求失败'
    return Promise.reject(new Error(msg))
  }
)

// Auth
export const login = (username: string, password: string) => api.post('/auth/login', { username, password })
export const getMe = () => api.get('/auth/me')

// Data Sources
export const getDataSources = (params?: { page?: number; page_size?: number; keyword?: string; enabled?: string }) =>
  api.get<{ items: DataSource[]; total: number; page: number; page_size: number }>('/datasources', { params })

export const getAllDataSources = () =>
  api.get<DataSource[]>('/datasources/all')
export const getDataSource = (id: number) => api.get<DataSource>(`/datasources/${id}`)
export const createDataSource = (d: DataSource) => api.post<DataSource>('/datasources', d)
export const updateDataSource = (id: number, d: Partial<DataSource>) => api.put<DataSource>(`/datasources/${id}`, d)
export const deleteDataSource = (id: number) => api.delete(`/datasources/${id}`)
export const batchDeleteDataSources = (ids: number[]) => api.patch('/datasources', { ids, action: 'delete' })
export const batchToggleDataSources = (ids: number[], enabled: boolean) => api.patch('/datasources', { ids, action: 'toggle', enabled })
export const batchSetTemplate = (ids: number[], templateIds: number[]) => api.patch('/datasources', {
  ids,
  action: 'set-template',
  template_ids: templateIds,
  template_id: templateIds[0] ?? null,
})
export const batchSetNotify = (ids: number[], notifyChannels: string) => api.patch('/datasources', { ids, action: 'set-notify', notify_channels: notifyChannels })
export const batchApplyTemplate = (ids: number[]) => api.patch('/datasources', { ids, action: 'apply-template' })
export const batchInspect = (ids: number[]) => api.patch('/datasources', { ids, action: 'inspect' })
export const batchSetCreds = (ids: number[], username: string, password: string) => api.patch('/datasources', { ids, action: 'set-creds', username, password })
export const importDatasources = (yaml: string) => api.post('/datasources/import', { yaml_content: yaml })
export const applyTemplate = (datasourceId: number) => api.post('/datasources/apply-template', { datasource_id: datasourceId })
export const testDataSource = (id: number) => api.post(`/datasources/${id}/test`)

// Notifications
export const getNotifications = (params?: { page?: number; page_size?: number; keyword?: string; channel_type?: string }) =>
  api.get<{ items: NotificationChannel[]; total: number; page: number; page_size: number }>('/notifications', { params })
export const getAllNotifications = () => api.get<NotificationChannel[]>('/notifications/all')
export const getNotification = (id: number) => api.get<NotificationChannel>(`/notifications/${id}`)
export const createNotification = (n: NotificationChannel) => api.post<NotificationChannel>('/notifications', n)
export const updateNotification = (id: number, n: NotificationChannel) => api.put<NotificationChannel>(`/notifications/${id}`, n)
export const deleteNotification = (id: number) => api.delete(`/notifications/${id}`)
export const testNotification = (id: number) => api.post('/notifications/test', { id })
export interface MessageTemplate {
  style?: 'simple' | 'table' | 'card'
  title_format?: string
  show_cause?: boolean
  show_impact?: boolean
  show_value_range?: boolean
  show_hit_count?: boolean
  show_datasource?: boolean
  show_time?: boolean
  show_detail_link?: boolean
  host_format?: 'full' | 'short' | 'with_ip'
  time_format?: string
  value_precision?: number
  max_entries?: number
  max_bytes?: number
  // 简易文本模板（仅 simple 风格）：{datasource} {content} {time} 等占位符
  default_template?: string
  // 高级：自定义 Go template
  custom_markdown?: string
  custom_subject?: string
}
export interface TemplatePreviewResult {
  title: string
  markdown: string
  html: string
  plain: string
  bytes: number
  errors?: string[]
}
export const previewMessageTemplate = (template: MessageTemplate, resolved = false, mockCount = 0) =>
  api.post<TemplatePreviewResult>('/notifications/template/preview', { template, resolved, mock_count: mockCount })

// Cron Jobs
export const getCronJobs = () => api.get<CronJob[]>('/cronjobs')
export const getCronJob = (id: number) => api.get<CronJob>(`/cronjobs/${id}`)
export const createCronJob = (j: CronJob) => api.post<CronJob>('/cronjobs', j)
export const updateCronJob = (id: number, j: CronJob) => api.put<CronJob>(`/cronjobs/${id}`, j)
export const deleteCronJob = (id: number) => api.delete(`/cronjobs/${id}`)

// Reports
export const getReports = (params?: { page?: number; page_size?: number; keyword?: string; status?: string }) =>
  api.get<{ items: ReportRecord[]; total: number; page: number; page_size: number }>('/report-records', { params })
export const deleteReport = (id: number) => api.delete(`/report-records/${id}`)
export const getReportContent = (id: number) =>
  api.get<{ content: string }>(`/report-records/${id}/content`)
export const saveReportContent = (id: number, content: string) =>
  api.put<{ ok: string }>(`/report-records/${id}/content`, { content })
export const exportReportFile = (id: number, format: 'md' | 'docx') =>
  api.get(`/report-records/${id}/export?format=${format}`, { responseType: 'blob' })

// Metrics
export const getMetricTypes = (datasourceId?: number) => api.get<MetricType[]>('/metrics/types', { params: datasourceId ? { datasource_id: datasourceId } : {} })
export const createMetricType = (t: MetricType) => api.post<MetricType>('/metrics/types', t)
export const updateMetricType = (id: number, t: MetricType) => api.put<MetricType>(`/metrics/types/${id}`, t)
export const deleteMetricType = (id: number) => api.delete(`/metrics/types/${id}`)
export const createMetricConfig = (c: MetricConfig) => api.post<MetricConfig>('/metrics/configs', c)
export const updateMetricConfig = (id: number, c: MetricConfig) => api.put<MetricConfig>(`/metrics/configs/${id}`, c)
export const deleteMetricConfig = (id: number) => api.delete(`/metrics/configs/${id}`)
export const validatePromQL = (datasourceId: number | undefined, query: string) => api.post('/metrics/validate', { datasource_id: datasourceId, query })

// Settings
export const getSettings = () => api.get<Record<string, string>>('/settings')
export const updateSettings = (s: Record<string, string>) => api.put('/settings', s)

// Inspect
export const triggerInspect = (req: InspectRequest) => api.post('/inspect', req)
export const getInspectTask = (taskId: string) => api.get(`/inspect/task/${taskId}`)
export const getInspectRecords = (params?: { page?: number; page_size?: number; keyword?: string; status?: string }) =>
  api.get<{ items: InspectRecord[]; total: number; page: number; page_size: number }>('/inspect/records', { params })

// Dashboard
export const getDashboardStats = () => api.get<DashboardStats>('/dashboard/stats')
export const getDashboardHealth = (datasourceId?: number) =>
  api.get('/dashboard/health', { params: datasourceId ? { datasource_id: datasourceId } : {} })

export const getDashboardHealthTrend = (days: number = 14) =>
  api.get('/dashboard/health/trend', { params: { days } })

// Sync Sources
export const getSyncSources = () => api.get<SyncSource[]>('/sync-sources')
export const createSyncSource = (s: SyncSource) => api.post<SyncSource>('/sync-sources', s)
export const updateSyncSource = (id: number, s: SyncSource) => api.put<SyncSource>(`/sync-sources/${id}`, s)
export const deleteSyncSource = (id: number) => api.delete(`/sync-sources/${id}`)
export const triggerSync = (id: number) => api.post(`/sync-sources/${id}/sync`)
export const getSyncLogs = (id: number) => api.get<SyncLog[]>(`/sync-sources/${id}/logs`)

// Templates
export const getTemplates = (params?: { page?: number; page_size?: number; keyword?: string }) =>
  api.get<{ items: any[]; total: number; page: number; page_size: number }>('/templates', { params })
export const getAllTemplates = () => api.get<any[]>('/templates/all')
export const initTemplates = () => api.post('/templates/init')
export const getTemplate = (id: number) => api.get(`/templates/${id}`)
export const createTemplate = (name: string, description?: string) => api.post('/templates', { name, description })
export const updateTemplate = (id: number, t: any) => api.put(`/templates/${id}`, t)
export const deleteTemplate = (id: number) => api.delete(`/templates/${id}`)
export const getTemplateMetrics = (id: number) => api.get(`/templates/${id}/metrics`)
export const setTemplateMetrics = (id: number, metricConfigIds: number[]) => api.post(`/templates/${id}/metrics`, { metric_config_ids: metricConfigIds })
export const saveTemplateMetricOverride = (templateId: number, configId: number, data: any) => api.put(`/templates/${templateId}/metrics/${configId}/override`, data)
export const inspectWithTemplate = (id: number, req: any) => api.post(`/templates/${id}/inspect`, req)

// AI Agent
export const aiChat = (message: string, sessionId?: string) =>
  api.post<{session_id: string}>('/ai/chat', { message, session_id: sessionId })

export const aiChatStream = (message: string, sessionId?: string): Promise<Response> => {
  const token = localStorage.getItem('token')
  return fetch('/api/promai/ai/chat', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'Authorization': `Bearer ${token}`,
    },
    body: JSON.stringify({ message, session_id: sessionId }),
  })
}

export const getAiSessions = () => api.get('/ai/sessions')
export const deleteAiSession = (id: string) => api.delete(`/ai/sessions/${id}`)
export const testAiModel = (model: {
  name: string; provider: string; model: string; base_url: string;
  api_key: string; thinking_level: string; max_tokens: number; proxy_url?: string
}) => api.post('/ai/test-model', model)

// ===== Alerting =================================================================
import type {
  AlertRule, AlertInstance, AlertHistoryRow, AlertSilence, AlertInhibit, AlertRoute,
  AlertGroup, AlertNotifyLog, AlertStats, EvaluatorStatus, TestRuleResult, TimelineGroup,
} from '../types/alerting'

export const getAlertRules = (params?: { keyword?: string; severity?: string; enabled?: string; page?: number; page_size?: number }) =>
  api.get<{ items: AlertRule[]; total: number }>('/alert/rules', { params })
export const getAlertRule = (id: number) => api.get<AlertRule>(`/alert/rules/${id}`)
export const createAlertRule = (r: AlertRule) => api.post<AlertRule>('/alert/rules', r)
export const updateAlertRule = (id: number, r: AlertRule) => api.put<AlertRule>(`/alert/rules/${id}`, r)
export const deleteAlertRule = (id: number) => api.delete(`/alert/rules/${id}`)
export const testAlertRule = (id: number) =>
  api.post<{ rule_id: number; datasources: TestRuleResult[] }>(`/alert/rules/${id}/test`)
export const batchToggleAlertRules = (ids: number[], enabled: boolean) =>
  api.post('/alert/rules/batch-toggle', { ids, enabled })
export const batchDeleteAlertRules = (ids: number[]) =>
  api.post<{ success: boolean; count: number }>('/alert/rules/batch-delete', { ids })
export const batchEditAlertRules = (ids: number[], updates: Record<string, any>) =>
  api.post<{ success: boolean; count: number }>('/alert/rules/batch-edit', { ids, ...updates })
export const generateAlertRulesFromTemplate = (templateId: number) =>
  api.post<{ success: boolean; created: number }>('/alert/rules/generate-from-template', { template_id: templateId })

export const getAlertInstances = (params?: {
  page?: number; page_size?: number; state?: string; severity?: string;
  datasource_id?: number; rule_id?: number; fingerprint?: string;
  keyword?: string; include_masked?: string
}) => api.get<{ items: AlertInstance[]; total: number; page: number; page_size: number }>('/alert/instances', { params })
export const getAlertInstance = (fingerprint: string) =>
  api.get<{ instance: AlertInstance; history: AlertHistoryRow[]; notify_logs?: AlertNotifyLog[] }>(`/alert/instances/${fingerprint}`)
export const getAlertInstancesTrend = (fingerprints: string[], minutes = 60) =>
  api.post<Record<string, [number, number][]>>('/alert/instances/trend', { fingerprints, minutes })
export const clearAlertInstances = () =>
  api.delete('/alert/instances')

export const getAlertHistory = (params?: {
  page?: number; page_size?: number; rule_id?: number; datasource_id?: number;
  event_type?: string; severity?: string; keyword?: string; from?: string; to?: string
}) => api.get<{ items: AlertHistoryRow[]; total: number; page: number; page_size: number }>('/alert/history', { params })

export const getAlertHistoryTimeline = (params?: {
  rule_id?: number; datasource_id?: number; keyword?: string; from?: string; to?: string
}) => api.get<{ groups: TimelineGroup[] }>('/alert/history/timeline', { params })

export const getAlertSilences = (params?: { include_expired?: boolean; page?: number; page_size?: number }) =>
  api.get<{ items: AlertSilence[]; total: number }>('/alert/silences', { params })
export const getAlertSilence = (id: number) => api.get<AlertSilence>(`/alert/silences/${id}`)
export const createAlertSilence = (s: AlertSilence) => api.post<AlertSilence>('/alert/silences', s)
export const updateAlertSilence = (id: number, s: AlertSilence) => api.put<AlertSilence>(`/alert/silences/${id}`, s)
export const deleteAlertSilence = (id: number) => api.delete(`/alert/silences/${id}`)

export const getAlertInhibits = (params?: { page?: number; page_size?: number }) =>
  api.get<{ items: AlertInhibit[]; total: number }>('/alert/inhibits', { params })
export const getAlertInhibit = (id: number) => api.get<AlertInhibit>(`/alert/inhibits/${id}`)
export const createAlertInhibit = (i: AlertInhibit) => api.post<AlertInhibit>('/alert/inhibits', i)
export const updateAlertInhibit = (id: number, i: AlertInhibit) => api.put<AlertInhibit>(`/alert/inhibits/${id}`, i)
export const deleteAlertInhibit = (id: number) => api.delete(`/alert/inhibits/${id}`)

export const getAlertRoutes = () => api.get<{ items: AlertRoute[]; total: number }>('/alert/routes')
export const getAlertRoute = (id: number) => api.get<AlertRoute>(`/alert/routes/${id}`)
export const createAlertRoute = (r: AlertRoute) => api.post<AlertRoute>('/alert/routes', r)
export const updateAlertRoute = (id: number, r: AlertRoute) => api.put<AlertRoute>(`/alert/routes/${id}`, r)
export const deleteAlertRoute = (id: number) => api.delete(`/alert/routes/${id}`)

export const getAlertGroups = () => api.get<{ items: AlertGroup[]; total: number }>('/alert/groups')
export const getAlertNotifyLogs = (params?: {
  page?: number; page_size?: number; status?: string; channel_id?: number; rule_id?: number
}) => api.get<{ items: AlertNotifyLog[]; total: number; page: number; page_size: number }>('/alert/notify-logs', { params })

export const getAlertStats = () => api.get<AlertStats>('/alert/stats')
export const getAlertEvaluatorStatus = () => api.get<EvaluatorStatus>('/alert/evaluator/status')

// AI Skills (SKILL.md 规范 — 文件系统存储)
export const getAISkills = () =>
  api.get<{ items: AiSkill[]; total: number }>('/ai/skills')
export const createAISkill = (s: AiSkill) => api.post<AiSkill>('/ai/skills', s)
export const updateAISkill = (name: string, s: AiSkill) => api.post(`/ai/skills/${name}`, s)
export const deleteAISkill = (name: string) => api.delete(`/ai/skills/${name}`)

export default api
