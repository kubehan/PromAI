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
