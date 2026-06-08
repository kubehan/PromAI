<template>
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
            <el-icon :size="18" color="rgba(255,255,255,0.4)">
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
            text-color="rgba(255,255,255,0.5)"
            active-text-color="#00d4ff"
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
              <span>数据源管理</span>
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
            <el-menu-item index="/settings">
              <el-icon><Setting /></el-icon>
              <span>系统设置</span>
            </el-menu-item>
            <el-menu-item index="/inspection">
              <el-icon><Monitor /></el-icon>
              <template #title>触发巡检</template>
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
            <div class="header-status">
              <span class="status-indicator"></span>
              <span class="status-label">All Systems Normal</span>
            </div>
          </div>
        </el-header>

        <el-main class="app-main">
          <router-view v-slot="{ Component }">
            <transition name="page-fade" mode="out-in">
              <component :is="Component" />
            </transition>
          </router-view>
        </el-main>
      </el-container>
    </el-container>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRoute } from 'vue-router'

const route = useRoute()
const collapsed = ref(false)
const sidebarWidth = computed(() => collapsed.value ? '68px' : '260px')
const currentRoute = computed(() => route.path)
const currentMeta = computed(() => route.meta)

function toggleCollapse() {
  collapsed.value = !collapsed.value
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
    linear-gradient(rgba(56, 189, 248, 0.03) 1px, transparent 1px),
    linear-gradient(90deg, rgba(56, 189, 248, 0.03) 1px, transparent 1px);
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
  background: linear-gradient(180deg, rgba(13, 19, 38, 0.98) 0%, rgba(8, 12, 24, 0.98) 100%);
  border-right: 1px solid rgba(56, 189, 248, 0.08);
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
  border-bottom: 1px solid rgba(56, 189, 248, 0.06);
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
  background: linear-gradient(135deg, #00d4ff, #7c3aed);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  background-clip: text;
  letter-spacing: -0.5px;
}

.logo-sub {
  font-size: 10px;
  color: rgba(255,255,255,0.3);
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
}

.el-menu-item:hover {
  background: rgba(56, 189, 248, 0.08) !important;
  color: rgba(255,255,255,0.8) !important;
}

.el-menu-item.is-active {
  background: rgba(0, 212, 255, 0.1) !important;
  color: #00d4ff !important;
  font-weight: 600;
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
  box-shadow: 0 0 12px rgba(0, 212, 255, 0.5);
}

.sidebar-footer {
  border-top: 1px solid rgba(56, 189, 248, 0.06);
  padding: 14px 20px;
  display: flex;
  align-items: center;
  gap: 10px;
  font-size: 12px;
  color: rgba(255,255,255,0.3);
  flex-shrink: 0;
}

.status-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: var(--emerald);
  box-shadow: 0 0 12px rgba(16, 185, 129, 0.6);
  animation: pulse-dot 2s ease-in-out infinite;
}

@keyframes pulse-dot {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.5; }
}

.app-header {
  height: 64px;
  background: rgba(13, 19, 38, 0.8);
  backdrop-filter: blur(20px);
  -webkit-backdrop-filter: blur(20px);
  border-bottom: 1px solid rgba(56, 189, 248, 0.06);
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
  background: linear-gradient(90deg, transparent, rgba(0, 212, 255, 0.08), transparent);
}

.header-left {
  display: flex;
  align-items: center;
  gap: 16px;
}

.header-left .el-breadcrumb { font-size: 14px; }

.header-status {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 6px 16px;
  border-radius: 20px;
  background: rgba(16, 185, 129, 0.08);
  border: 1px solid rgba(16, 185, 129, 0.15);
}

.status-indicator {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: var(--emerald);
  box-shadow: 0 0 12px rgba(16, 185, 129, 0.6);
}

.status-label {
  font-size: 12px;
  font-weight: 600;
  color: var(--emerald);
  letter-spacing: 0.5px;
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
</style>
