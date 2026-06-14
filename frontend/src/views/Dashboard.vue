<template>
  <div class="page-container">
    <div class="page-header">
      <h2><el-icon><Odometer /></el-icon> 控制台</h2>
      <p>系统运行状态与数据概览</p>
    </div>

    <el-row :gutter="20" style="margin-bottom: 24px;">
      <el-col :span="6" v-for="(stat, idx) in stats" :key="stat.label">
        <div class="stat-card fade-in-up" :style="{ '--delay': `${idx * 0.06}s` }">
          <div class="stat-glow" :style="{ background: stat.glow }"></div>
          <div class="stat-icon-wrap" :style="{ background: stat.bg }">
            <el-icon :color="stat.color" :size="22"><component :is="stat.icon" /></el-icon>
          </div>
          <div class="stat-value" :style="{ color: stat.color }">{{ stat.value }}</div>
          <div class="stat-label">{{ stat.label }}</div>
        </div>
      </el-col>
    </el-row>

    <el-row :gutter="20">
      <el-col :span="16">
        <div class="section-card">
          <div class="section-header">
            <h3><el-icon :size="16" :color="getCssVar('--cyan')"><Document /></el-icon> 最近报告</h3>
            <el-button size="small" text @click="$router.push('/reports')" style="color: var(--cyan);">
              查看全部 <el-icon><ArrowRight /></el-icon>
            </el-button>
          </div>
          <el-table :data="recentReports" v-loading="loading" stripe style="width: 100%">
            <el-table-column prop="title" label="报告名称" min-width="200">
              <template #default="{ row }">
                <span style="font-weight: 600; color: var(--text-primary);">{{ row.title }}</span>
              </template>
            </el-table-column>
            <el-table-column prop="datasource_name" label="数据源" min-width="160" />
            <el-table-column label="指标/告警" width="140" align="center">
              <template #default="{ row }">
                <span style="color: var(--cyan); font-weight: 700;">{{ row.total_metrics }}</span>
                <span style="color: var(--text-tertiary);"> / </span>
                <span v-if="row.critical_count > 0" style="color: var(--red); font-weight: 700;">{{ row.alert_count }}</span>
                <span v-else-if="row.warning_count > 0" style="color: var(--amber); font-weight: 700;">{{ row.alert_count }}</span>
                <span v-else style="color: var(--emerald); font-weight: 700;">0</span>
              </template>
            </el-table-column>
            <el-table-column label="状态" width="110" align="center">
              <template #default="{ row }">
                <span :class="['status-badge', row.status]">
                  {{ row.status === 'success' ? '正常' : row.status === 'danger' ? '高危' : '告警' }}
                </span>
              </template>
            </el-table-column>
            <el-table-column label="时间" width="170">
              <template #default="{ row }">
                <span style="color: var(--text-tertiary); font-size: 13px;">{{ dayjs(row.created_at).format('YYYY-MM-DD HH:mm') }}</span>
              </template>
            </el-table-column>
          </el-table>
          <el-empty v-if="!loading && recentReports.length === 0" description="暂无报告数据" :image-size="60" />
        </div>
      </el-col>
      <el-col :span="8">
        <div class="section-card">
          <div class="section-header">
            <h3><el-icon :size="16" :color="getCssVar('--cyan')"><Lightning /></el-icon> 快速操作</h3>
          </div>
          <div style="padding: 20px;">
            <el-button type="primary" class="quick-action-btn" @click="$router.push('/inspection')" style="margin-bottom: 12px;">
              <el-icon><Monitor /></el-icon> 触发巡检
            </el-button>
            <el-button plain class="quick-action-btn" @click="$router.push('/datasources')" style="margin-bottom: 12px;">
              <el-icon><Connection /></el-icon> 管理数据源
            </el-button>
            <el-button plain class="quick-action-btn" @click="$router.push('/notifications')">
              <el-icon><Bell /></el-icon> 配置通知
            </el-button>
          </div>
        </div>
        <div class="section-card" style="margin-top: 20px;">
          <div class="section-header">
            <h3><el-icon :size="16" :color="getCssVar('--emerald')"><InfoFilled /></el-icon> 系统信息</h3>
          </div>
          <div style="padding: 20px; font-size: 13px; color: var(--text-tertiary); line-height: 2.2;">
            <div style="display: flex; justify-content: space-between;">
              <span>运行状态</span>
              <span style="color: var(--emerald); font-weight: 600;">● 正常</span>
            </div>
            <div style="display: flex; justify-content: space-between;">
              <span>数据源数量</span>
              <span style="color: var(--cyan);">{{ stats[0].value }}</span>
            </div>
            <div style="display: flex; justify-content: space-between;">
              <span>定时任务</span>
              <span style="color: var(--cyan);">{{ stats[1].value }}</span>
            </div>
            <div style="display: flex; justify-content: space-between;">
              <span>数据库</span>
              <span style="color: var(--text-secondary);">SQLite</span>
            </div>
          </div>
        </div>
      </el-col>
    </el-row>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import dayjs from 'dayjs'
import { getAllDataSources, getCronJobs, getAllNotifications, getReports } from '../api'
import type { ReportRecord } from '../types'
import { useTheme } from '../composables/useTheme'

function getCssVar(name: string): string {
  return getComputedStyle(document.documentElement).getPropertyValue(name).trim()
}

const { currentTheme } = useTheme()
const loading = ref(false)
const recentReports = ref<ReportRecord[]>([])
const statValues = ref([0, 0, 0, 0])
const stats = computed(() => {
  // read currentTheme to re-evaluate on theme change
  void currentTheme.value
  return [
    { label: '数据源', value: statValues.value[0], icon: 'Connection', color: getCssVar('--cyan'), glow: getCssVar('--cyan'), bg: getCssVar('--cyan-dim') },
    { label: '定时任务', value: statValues.value[1], icon: 'Clock', color: getCssVar('--emerald'), glow: getCssVar('--emerald'), bg: getCssVar('--emerald-dim') },
    { label: '通知渠道', value: statValues.value[2], icon: 'Bell', color: getCssVar('--amber'), glow: getCssVar('--amber'), bg: getCssVar('--amber-dim') },
    { label: '历史报告', value: statValues.value[3], icon: 'Document', color: getCssVar('--purple'), glow: getCssVar('--purple'), bg: getCssVar('--purple-dim') },
  ]
})

onMounted(async () => {
  loading.value = true
  try {
    const [ds, cron, notif, reps] = await Promise.all([
      getAllDataSources(), getCronJobs(), getAllNotifications(), getReports(),
    ])
    statValues.value[0] = ds.data.length
    statValues.value[1] = cron.data.length
    statValues.value[2] = notif.data.length
    statValues.value[3] = reps.data.total
    recentReports.value = reps.data.items.slice(0, 5)
  } catch { /* ignore */ } finally {
    loading.value = false
  }
})
</script>
