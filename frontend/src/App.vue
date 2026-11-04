<template>
  <template v-if="showLayout">
    <div class="app-shell">
      <div class="bg-grid"></div>
      <el-container class="app-layout">
        <el-aside :width="sidebarWidth" class="app-sidebar">
          <div class="sidebar-header">
            <div class="logo">
              <div class="logo-icon">
                <svg width="32" height="32" viewBox="0 0 32 32" fill="none">
                  <rect width="32" height="32" rx="8" fill="url(#logo-grad)"/>
                  <path d="M16 8c-4.4 0-8 3.6-8 8s3.6 8 8 8 8-3.6 8-8-3.6-8-8-8zm0 14c-3.3 0-6-2.7-6-6s2.7-6 6-6 6 2.7 6 6-2.7 6-6 6z" fill="white" opacity="0.9"/>
                  <path d="M16 12l-2.5 4h5L16 12z" fill="white"/>
                  <defs>
                    <linearGradient id="logo-grad" x1="0" y1="0" x2="32" y2="32">
                      <stop offset="0%" stop-color="#00d4ff"/>
                      <stop offset="100%" stop-color="#7c3aed"/>
                    </linearGradient>
                  </defs>
                </svg>
              </div>
              <div class="logo-text-group" v-show="!collapsed">
                <span class="logo-text">PromAI</span>
                <span class="logo-sub">运维监控平台</span>
              </div>
            </div>
            <el-button text class="collapse-btn" @click="toggleCollapse">
              <el-icon :size="18" class="collapse-icon">
                <Fold v-if="!collapsed" /><Expand v-else />
              </el-icon>
            </el-button>
          </div>

          <div class="sidebar-nav">
            <el-menu
              :default-active="currentRoute"
              :collapse="collapsed"
              :collapse-transition="false"
              background-color="transparent"
              router
            >
              <el-menu-item index="/dashboard">
                <el-icon><Odometer /></el-icon>
                <span>控制台</span>
              </el-menu-item>
              <el-menu-item index="/bi">
                <el-icon><DataAnalysis /></el-icon>
                <span>健康大屏</span>
              </el-menu-item>
              <el-menu-item index="/datasources">
                <el-icon><Connection /></el-icon>
                <template #title>
                  <span style="flex:1;">数据源管理</span>
                  <el-tooltip content="此页面支持一键触发巡检" placement="right">
                    <el-badge value="巡检" class="inspect-badge" />
                  </el-tooltip>
                </template>
              </el-menu-item>
              <el-menu-item index="/notifications">
                <el-icon><Bell /></el-icon>
                <span>通知渠道</span>
              </el-menu-item>
              <el-menu-item index="/cronjobs">
                <el-icon><Clock /></el-icon>
                <span>定时任务</span>
              </el-menu-item>
              <el-menu-item index="/reports">
                <el-icon><Document /></el-icon>
                <span>报告管理</span>
              </el-menu-item>
              <el-sub-menu index="metrics-group">
                <template #title>
                  <el-icon><TrendCharts /></el-icon>
                  <span>指标配置</span>
                </template>
                <el-menu-item index="/metrics">
                  <el-icon><List /></el-icon>
                  <span>指标列表</span>
                </el-menu-item>
                <el-menu-item index="/templates">
                  <el-icon><Collection /></el-icon>
                  <span>巡检模板</span>
                </el-menu-item>
              </el-sub-menu>
              <el-sub-menu index="alert-group">
                <template #title>
                  <el-icon><Warning /></el-icon>
                  <span>告警管理</span>
                </template>
                <el-menu-item index="/alerts">
                  <el-icon><Warning /></el-icon>
                  <template #title>
                    <span style="flex:1;">实时告警</span>
                    <el-badge :value="alertFiringCount" :hidden="alertFiringCount===0" class="alert-badge" />
                  </template>
                </el-menu-item>
                <el-menu-item index="/alert-rules">
                  <el-icon><AlarmClock /></el-icon>
                  <span>告警规则</span>
                </el-menu-item>
                <el-menu-item index="/alert-silences">
                  <el-icon><Mute /></el-icon>
                  <span>告警静默</span>
                </el-menu-item>
                <el-menu-item index="/alert-inhibits">
                  <el-icon><Filter /></el-icon>
                  <span>抑制规则</span>
                </el-menu-item>
                <el-menu-item index="/alert-routes">
                  <el-icon><Share /></el-icon>
                  <span>通知路由</span>
                </el-menu-item>
                <el-menu-item index="/alert-history">
                  <el-icon><Histogram /></el-icon>
                  <span>告警历史</span>
                </el-menu-item>
              </el-sub-menu>
              <el-menu-item index="/ai">
                <el-icon><MagicStick /></el-icon>
                <span>AI 助手</span>
              </el-menu-item>
              <el-menu-item index="/skills">
                <el-icon><Tools /></el-icon>
                <span>Skill 管理</span>
              </el-menu-item>
              <el-menu-item index="/settings">
                <el-icon><Setting /></el-icon>
                <span>系统设置</span>
              </el-menu-item>
            <el-menu-item index="/inspection">
              <el-icon><Monitor /></el-icon>
              <template #title>触发巡检</template>
            </el-menu-item>
            <el-menu-item index="/inspect-records">
              <el-icon><List /></el-icon>
              <span>巡检记录</span>
            </el-menu-item>
              <el-menu-item index="/hosts">
                <el-icon><Connection /></el-icon>
                <span>主机管理</span>
              </el-menu-item>
              <el-menu-item index="/terminal">
                <el-icon><Monitor /></el-icon>
                <span>Web 终端</span>
              </el-menu-item>
              <el-menu-item v-if="embedEnabled" index="/embedded">
                <el-icon><Platform /></el-icon>
                <span>嵌入式页面</span>
              </el-menu-item>
              <el-menu-item index="/aiops/dashboard">
                <el-icon><DataAnalysis /></el-icon>
                <span>智能运维</span>
              </el-menu-item>
              <el-menu-item index="/aiops/events">
                <el-icon><Bell /></el-icon>
                <span>事件中心</span>
              </el-menu-item>
              <el-menu-item index="/aiops/incidents">
                <el-icon><Warning /></el-icon>
                <span>故障管理</span>
              </el-menu-item>
              <el-menu-item index="/user-manual">
                <el-icon><Reading /></el-icon>
                <span>使用手册</span>
              </el-menu-item>
            </el-menu>
          </div>

          <div class="sidebar-footer" v-show="!collapsed">
            <div class="status-dot"></div>
            <span>系统运行中</span>
          </div>
        </el-aside>

        <el-container>
          <el-header class="app-header">
            <div class="header-left">
              <el-breadcrumb separator="/">
                <el-breadcrumb-item :to="{ path: '/dashboard' }">PromAI</el-breadcrumb-item>
                <el-breadcrumb-item v-if="currentMeta?.title">{{ currentMeta.title }}</el-breadcrumb-item>
              </el-breadcrumb>
            </div>
            <div class="header-right">
              <el-dropdown trigger="click" @command="handleThemeChange">
                <el-button text class="header-btn">
                  <el-icon :size="18"><Brush /></el-icon>
                  <span style="margin-left: 4px; font-size: 13px;">主题</span>
                </el-button>
                <template #dropdown>
                  <el-dropdown-menu>
                    <el-dropdown-item v-for="t in themeOptions" :key="t.value" :command="t.value" :class="{ active: currentTheme === t.value }">
                      <span style="margin-right: 8px;">{{ t.icon }}</span>
                      <span>{{ t.label }}</span>
                      <el-icon v-if="currentTheme === t.value" style="margin-left: auto; color: var(--cyan);"><Check /></el-icon>
                    </el-dropdown-item>
                  </el-dropdown-menu>
                </template>
              </el-dropdown>
              <div class="header-user">
                <el-icon><User /></el-icon>
                <span class="user-name">{{ username }}</span>
              </div>
              <el-button text class="header-btn" @click="handleLogout">
                <el-icon><SwitchButton /></el-icon>
                <span style="margin-left: 4px; font-size: 13px;">退出</span>
              </el-button>
            </div>
          </el-header>

          <el-main class="app-main">
            <router-view v-slot="{ Component }">
              <transition name="page-fade" mode="out-in">
                <keep-alive :include="['HostTerminal']">
                  <component :is="Component" />
                </keep-alive>
              </transition>
            </router-view>
          </el-main>
        </el-container>
      </el-container>
    </div>
  </template>
  <template v-else>
    <router-view />
  </template>
</template>

<script setup lang="ts">
import { computed, ref, onMounted, onUnmounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useTheme } from './composables/useTheme'
import { getAlertStats, getEmbed } from './api'

const route = useRoute()
const router = useRouter()
const collapsed = ref(false)
const sidebarWidth = computed(() => collapsed.value ? '68px' : '260px')
const currentRoute = computed(() => route.path)
const currentMeta = computed(() => route.meta)
const showLayout = computed(() => route.path !== '/login')
const username = ref(localStorage.getItem('username') || 'Admin')

const { currentTheme, setTheme, themeOptions } = useTheme()
const alertFiringCount = ref(0)
const embedEnabled = ref(false)
let pollTimer: ReturnType<typeof setInterval> | null = null
async function pollAlertCount() {
  try { const r = await getAlertStats(); const s = r.data.by_state; alertFiringCount.value = s.find(x => x.State === 'firing')?.Count || 0 } catch {}
}
async function loadEmbedFlag() {
  if (!localStorage.getItem('token')) { embedEnabled.value = false; return }
  try { const r = await getEmbed(); embedEnabled.value = !!r.data.enabled } catch { embedEnabled.value = false }
}
onMounted(() => { loadEmbedFlag(); pollAlertCount(); pollTimer = setInterval(() => { loadEmbedFlag(); pollAlertCount() }, 15000) })
onUnmounted(() => { if (pollTimer) clearInterval(pollTimer) })

function toggleCollapse() {
  collapsed.value = !collapsed.value
}

function handleThemeChange(name: string) {
  setTheme(name as any)
}

function handleLogout() {
  localStorage.removeItem('token')
  localStorage.removeItem('username')
  router.push('/login')
}
</script>

<style scoped>
.app-shell {
  height: 100vh;
  position: relative;
  overflow: hidden;
}

.bg-grid {
  position: fixed;
  inset: 0;
  background-image:
    linear-gradient(var(--grid-line) 1px, transparent 1px),
    linear-gradient(90deg, var(--grid-line) 1px, transparent 1px);
  background-size: 48px 48px;
  pointer-events: none;
  z-index: 0;
}

.app-layout {
  height: 100vh;
  position: relative;
  z-index: 1;
}

.app-sidebar {
  background: var(--bg-secondary);
  border-right: 1px solid var(--border);
  transition: width 0.3s cubic-bezier(0.4, 0, 0.2, 1);
  overflow: hidden;
  display: flex;
  flex-direction: column;
  position: relative;
}

.app-sidebar::after {
  content: '';
  position: absolute;
  top: 0;
  right: -1px;
  width: 1px;
  height: 100%;
  background: linear-gradient(180deg, transparent, var(--cyan), var(--purple), transparent);
  opacity: 0.15;
}

.sidebar-header {
  height: 64px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 16px 0 20px;
  border-bottom: 1px solid var(--border);
  flex-shrink: 0;
}

.logo {
  display: flex;
  align-items: center;
  gap: 12px;
  overflow: hidden;
}

.logo-icon {
  flex-shrink: 0;
}

.logo-text-group {
  display: flex;
  flex-direction: column;
  white-space: nowrap;
}

.logo-text {
  font-size: 18px;
  font-weight: 800;
  background: linear-gradient(135deg, var(--cyan), var(--purple));
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  background-clip: text;
  letter-spacing: -0.5px;
}

.collapse-icon {
  color: var(--text-tertiary);
}

.logo-sub {
  font-size: 10px;
  color: var(--text-tertiary);
  letter-spacing: 2px;
  text-transform: uppercase;
  font-weight: 500;
}

.collapse-btn {
  flex-shrink: 0;
  padding: 4px;
}

.sidebar-nav {
  flex: 1;
  overflow-y: auto;
  padding: 12px 0;
}

.sidebar-nav::-webkit-scrollbar { width: 2px; }

.el-menu {
  border-right: none !important;
}

.el-menu-item {
  margin: 2px 10px;
  border-radius: 10px;
  height: 42px;
  line-height: 42px;
  font-size: 13px;
  font-weight: 500;
  transition: all 0.2s ease;
  color: var(--text-secondary) !important;
}

.el-menu-item:hover {
  background: var(--cyan-dim) !important;
  color: var(--text-primary) !important;
}

.el-menu-item.is-active {
  background: var(--cyan-dim) !important;
  color: var(--cyan) !important;
  font-weight: 600;
}

:deep(.el-sub-menu__title) {
  color: var(--text-secondary) !important;
}

:deep(.el-sub-menu__title:hover) {
  color: var(--text-primary) !important;
  background: var(--cyan-dim) !important;
}

:deep(.el-sub-menu.is-active .el-sub-menu__title) {
  color: var(--cyan) !important;
}

.el-menu-item.is-active::before {
  content: '';
  position: absolute;
  left: 0;
  top: 50%;
  transform: translateY(-50%);
  width: 3px;
  height: 20px;
  background: var(--cyan);
  border-radius: 0 4px 4px 0;
  box-shadow: 0 0 12px var(--cyan-dim);
}

.sidebar-footer {
  border-top: 1px solid var(--border);
  padding: 14px 20px;
  display: flex;
  align-items: center;
  gap: 10px;
  font-size: 12px;
  color: var(--text-tertiary);
  flex-shrink: 0;
}

.status-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: var(--emerald);
  box-shadow: 0 0 12px var(--emerald-dim);
  animation: pulse-dot 2s ease-in-out infinite;
}

@keyframes pulse-dot {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.5; }
}

.app-header {
  height: 64px;
  background: var(--bg-elevated);
  backdrop-filter: blur(20px);
  -webkit-backdrop-filter: blur(20px);
  border-bottom: 1px solid var(--border);
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 32px;
  position: relative;
  z-index: 10;
}

.app-header::after {
  content: '';
  position: absolute;
  bottom: 0;
  left: 5%;
  width: 90%;
  height: 1px;
  background: linear-gradient(90deg, transparent, var(--border), transparent);
}

.header-left {
  display: flex;
  align-items: center;
  gap: 16px;
}

.header-left .el-breadcrumb { font-size: 14px; }

.header-right {
  display: flex;
  align-items: center;
  gap: 8px;
}

.header-btn {
  color: var(--text-secondary) !important;
  font-size: 13px;
}

.header-user {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 4px 12px;
  color: var(--text-secondary);
  font-size: 13px;
}

.user-name {
  font-weight: 500;
}

.app-main {
  background: transparent;
  padding: 0;
  overflow: hidden;
  position: relative;
}

.page-fade-enter-active,
.page-fade-leave-active {
  transition: opacity 0.2s ease, transform 0.2s ease;
}

.page-fade-enter-from {
  opacity: 0;
  transform: translateY(8px);
}

.page-fade-leave-to {
  opacity: 0;
  transform: translateY(-8px);
}
.alert-badge { --el-badge-bg-color: var(--red); margin-top: -2px; }
.alert-badge :deep(.el-badge__content) { font-size: 11px; padding: 0 5px; height: 18px; line-height: 18px; border: none; }
.inspect-badge { --el-badge-bg-color: var(--cyan); margin-top: -2px; margin-right: 12px; }
.inspect-badge :deep(.el-badge__content) { font-size: 10px; padding: 0 5px; height: 16px; line-height: 16px; border: none; }
</style>
