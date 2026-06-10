import axios from 'axios'
import router from '../router'
import type {
  DataSource, MetricType, MetricConfig,
  NotificationChannel, CronJob, ReportRecord,
  InspectRecord, InspectRequest, DashboardStats,
  SyncSource, SyncLog
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
export const batchSetTemplate = (ids: number[], templateId: number | null) => api.patch('/datasources', { ids, action: 'set-template', template_id: templateId })
export const batchSetNotify = (ids: number[], notifyChannels: string) => api.patch('/datasources', { ids, action: 'set-notify', notify_channels: notifyChannels })
export const batchApplyTemplate = (ids: number[]) => api.patch('/datasources', { ids, action: 'apply-template' })
export const batchInspect = (ids: number[]) => api.patch('/datasources', { ids, action: 'inspect' })
export const batchSetCreds = (ids: number[], username: string, password: string) => api.patch('/datasources', { ids, action: 'set-creds', username, password })
export const importDatasources = (yaml: string) => api.post('/datasources/import', { yaml_content: yaml })
export const applyTemplate = (datasourceId: number) => api.post('/datasources/apply-template', { datasource_id: datasourceId })
export const testDataSource = (id: number) => api.post(`/datasources/${id}/test`)

// Notifications
export const getNotifications = () => api.get<NotificationChannel[]>('/notifications')
export const getNotification = (id: number) => api.get<NotificationChannel>(`/notifications/${id}`)
export const createNotification = (n: NotificationChannel) => api.post<NotificationChannel>('/notifications', n)
export const updateNotification = (id: number, n: NotificationChannel) => api.put<NotificationChannel>(`/notifications/${id}`, n)
export const deleteNotification = (id: number) => api.delete(`/notifications/${id}`)
export const testNotification = (id: number) => api.post('/notifications/test', { id })

// Cron Jobs
export const getCronJobs = () => api.get<CronJob[]>('/cronjobs')
export const getCronJob = (id: number) => api.get<CronJob>(`/cronjobs/${id}`)
export const createCronJob = (j: CronJob) => api.post<CronJob>('/cronjobs', j)
export const updateCronJob = (id: number, j: CronJob) => api.put<CronJob>(`/cronjobs/${id}`, j)
export const deleteCronJob = (id: number) => api.delete(`/cronjobs/${id}`)

// Reports
export const getReports = () => api.get<ReportRecord[]>('/report-records')
export const deleteReport = (id: number) => api.delete(`/report-records/${id}`)

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
export const getInspectRecords = () => api.get<InspectRecord[]>('/inspect/records')

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
export const getTemplates = () => api.get('/templates')
export const getTemplate = (id: number) => api.get(`/templates/${id}`)
export const createTemplate = (name: string, description?: string) => api.post('/templates', { name, description })
export const updateTemplate = (id: number, t: any) => api.put(`/templates/${id}`, t)
export const deleteTemplate = (id: number) => api.delete(`/templates/${id}`)
export const getTemplateMetrics = (id: number) => api.get(`/templates/${id}/metrics`)
export const setTemplateMetrics = (id: number, metricConfigIds: number[]) => api.post(`/templates/${id}/metrics`, { metric_config_ids: metricConfigIds })
export const saveTemplateMetricOverride = (templateId: number, configId: number, data: any) => api.put(`/templates/${templateId}/metrics/${configId}/override`, data)
export const inspectWithTemplate = (id: number, req: any) => api.post(`/templates/${id}/inspect`, req)

export default api
