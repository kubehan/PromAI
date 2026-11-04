import { createRouter, createWebHistory, type RouteRecordRaw } from 'vue-router'

const routes: RouteRecordRaw[] = [
  {
    path: '/login',
    name: 'Login',
    component: () => import('../views/Login.vue'),
    meta: { title: '登录', noAuth: true },
  },
  {
    path: '/',
    redirect: '/login',
  },
  {
    path: '/dashboard',
    name: 'Dashboard',
    component: () => import('../views/Dashboard.vue'),
    meta: { title: '控制台', icon: 'Odometer' },
  },
  {
    path: '/bi',
    name: 'BI',
    component: () => import('../views/BI.vue'),
    meta: { title: '健康大屏', icon: 'DataAnalysis' },
  },
  {
    path: '/datasources',
    name: 'DataSources',
    component: () => import('../views/DataSources.vue'),
    meta: { title: '数据源管理', icon: 'Connection' },
  },
  {
    path: '/notifications',
    name: 'Notifications',
    component: () => import('../views/Notifications.vue'),
    meta: { title: '通知渠道', icon: 'Bell' },
  },
  {
    path: '/cronjobs',
    name: 'CronJobs',
    component: () => import('../views/CronJobs.vue'),
    meta: { title: '定时任务', icon: 'Clock' },
  },
  {
    path: '/reports',
    name: 'Reports',
    component: () => import('../views/Reports.vue'),
    meta: { title: '报告管理', icon: 'Document' },
  },
  {
    path: '/metrics',
    name: 'Metrics',
    component: () => import('../views/Metrics.vue'),
    meta: { title: '指标配置', icon: 'TrendCharts' },
  },
  {
    path: '/settings',
    name: 'Settings',
    component: () => import('../views/Settings.vue'),
    meta: { title: '系统设置', icon: 'Setting' },
  },
  {
    path: '/inspection',
    name: 'Inspection',
    component: () => import('../views/Inspection.vue'),
    meta: { title: '触发巡检', icon: 'Monitor' },
  },
  {
    path: '/inspect-records',
    name: 'InspectRecords',
    component: () => import('../views/InspectRecords.vue'),
    meta: { title: '巡检记录', icon: 'List' },
  },
  {
    path: '/templates',
    name: 'Templates',
    component: () => import('../views/InspectionTemplates.vue'),
    meta: { title: '巡检模板', icon: 'Collection' },
  },
  {
    path: '/ai',
    name: 'AiChat',
    component: () => import('../views/AiChat.vue'),
    meta: { title: 'AI 助手', icon: 'MagicStick' },
  },
  {
    path: '/skills',
    name: 'Skills',
    component: () => import('../views/Skills.vue'),
    meta: { title: 'Skill 管理', icon: 'Tools' },
  },
  {
    path: '/hosts',
    name: 'Hosts',
    component: () => import('../views/Hosts.vue'),
    meta: { title: '主机管理', icon: 'Connection' },
  },
  {
    path: '/terminal',
    name: 'HostTerminal',
    component: () => import('../views/HostTerminal.vue'),
    meta: { title: 'Web 终端', icon: 'Monitor' },
  },
  {
    path: '/ai-terminal',
    redirect: to => ({ path: '/terminal', query: to.query }),
  },
  // ===== Alerting =================================================================
  {
    path: '/alerts',
    name: 'AlertInstances',
    component: () => import('../views/AlertInstances.vue'),
    meta: { title: '实时告警', icon: 'Warning' },
  },
  {
    path: '/alert-rules',
    name: 'AlertRules',
    component: () => import('../views/AlertRules.vue'),
    meta: { title: '告警规则', icon: 'AlarmClock' },
  },
  {
    path: '/alert-silences',
    name: 'AlertSilences',
    component: () => import('../views/AlertSilences.vue'),
    meta: { title: '告警静默', icon: 'Mute' },
  },
  {
    path: '/alert-inhibits',
    name: 'AlertInhibits',
    component: () => import('../views/AlertInhibits.vue'),
    meta: { title: '抑制规则', icon: 'Filter' },
  },
  {
    path: '/alert-routes',
    name: 'AlertRoutes',
    component: () => import('../views/AlertRoutes.vue'),
    meta: { title: '通知路由', icon: 'Share' },
  },
  {
    path: '/alert-history',
    name: 'AlertHistory',
    component: () => import('../views/AlertHistory.vue'),
    meta: { title: '告警历史', icon: 'Histogram' },
  },
  {
    path: '/user-manual',
    name: 'UserManual',
    component: () => import('../views/UserManual.vue'),
    meta: { title: '使用手册', icon: 'Reading' },
  },
  {
    path: '/aiops/dashboard',
    name: 'AIOpsDashboard',
    component: () => import('../views/aiops/Dashboard.vue'),
    meta: { title: 'AIOps 驾驶舱', icon: 'DataLine' },
  },
  {
    path: '/aiops/events',
    name: 'AIOpsEvents',
    component: () => import('../views/aiops/Events.vue'),
    meta: { title: '事件中心', icon: 'Bell' },
  },
  {
    path: '/aiops/incidents',
    name: 'AIOpsIncidents',
    component: () => import('../views/aiops/Incidents.vue'),
    meta: { title: '故障管理', icon: 'Warning' },
  },
  {
    path: '/aiops/incidents/:id',
    name: 'AIOpsIncidentDetail',
    component: () => import('../views/aiops/IncidentDetail.vue'),
    meta: { title: '故障详情', icon: 'Warning' },
  },
  {
    path: '/aiops/topology',
    name: 'AIOpsTopology',
    component: () => import('../views/aiops/Topology.vue'),
    meta: { title: '服务拓扑', icon: 'Share' },
  },
  {
    path: '/aiops/cmdb',
    name: 'AIOpsCMDB',
    component: () => import('../views/aiops/CMDB.vue'),
    meta: { title: 'CMDB', icon: 'List' },
  },
  {
    path: '/aiops/changes',
    name: 'AIOpsChanges',
    component: () => import('../views/aiops/Changes.vue'),
    meta: { title: '变更事件', icon: 'Refresh' },
  },
  {
    path: '/aiops/runbooks',
    name: 'AIOpsRunbooks',
    component: () => import('../views/aiops/Runbooks.vue'),
    meta: { title: '知识库', icon: 'Reading' },
  },
  {
    path: '/aiops/oncall',
    name: 'AIOpsOnCall',
    component: () => import('../views/aiops/OnCall.vue'),
    meta: { title: '值班管理', icon: 'User' },
  },
  {
    path: '/aiops/slo',
    name: 'AIOpsSLO',
    component: () => import('../views/aiops/SLO.vue'),
    meta: { title: 'SLO', icon: 'DataAnalysis' },
  },
]

const router = createRouter({
  history: createWebHistory('/promai/'),
  routes,
})

router.beforeEach((to, _from, next) => {
  const token = localStorage.getItem('token')
  if (to.meta.noAuth) {
    next()
    return
  }
  if (!token) {
    next('/login')
    return
  }
  next()
})

export default router
