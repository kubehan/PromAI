<template>
  <div class="page-container">
    <div class="page-header" style="display: flex; justify-content: space-between; align-items: flex-start;">
      <div>
        <h2><el-icon><DataAnalysis /></el-icon> 健康大屏</h2>
        <p>全景巡检健康状态 BI 看板</p>
      </div>
      <div style="display: flex; gap: 12px; align-items: center;">
        <el-select v-model="selectedDS" placeholder="全部数据源" style="width: 200px;" @change="fetchData">
          <el-option label="全部数据源" value="" />
          <el-option v-for="ds in allDatasources" :key="ds.id" :label="ds.name" :value="ds.id" />
        </el-select>
        <el-button plain @click="fetchData" :loading="loading"><el-icon><Refresh /></el-icon> 刷新</el-button>
      </div>
    </div>

    <!-- Global Health -->
    <el-row :gutter="20" style="margin-bottom: 24px;">
      <el-col :span="6">
        <div class="stat-card">
          <div class="stat-glow" style="background: #00d4ff;"></div>
          <div class="stat-icon-wrap" style="background: rgba(0,212,255,0.08);">
            <el-icon color="#00d4ff" :size="22"><DataAnalysis /></el-icon>
          </div>
          <div class="stat-value" style="color: #00d4ff;">{{ overallHealth.toFixed(1) }}%</div>
          <div class="stat-label">综合健康分</div>
        </div>
      </el-col>
      <el-col :span="6">
        <div class="stat-card">
          <div class="stat-glow" style="background: #10b981;"></div>
          <div class="stat-icon-wrap" style="background: rgba(16,185,129,0.08);">
            <el-icon color="#10b981" :size="22"><Check /></el-icon>
          </div>
          <div class="stat-value" style="color: #10b981;">{{ summary.normal_total }}</div>
          <div class="stat-label">正常指标</div>
        </div>
      </el-col>
      <el-col :span="6">
        <div class="stat-card">
          <div class="stat-glow" style="background: #f59e0b;"></div>
          <div class="stat-icon-wrap" style="background: rgba(245,158,11,0.08);">
            <el-icon color="#f59e0b" :size="22"><WarningFilled /></el-icon>
          </div>
          <div class="stat-value" style="color: #f59e0b;">{{ summary.warning_total }}</div>
          <div class="stat-label">告警指标</div>
        </div>
      </el-col>
      <el-col :span="6">
        <div class="stat-card">
          <div class="stat-glow" style="background: #ef4444;"></div>
          <div class="stat-icon-wrap" style="background: rgba(239,68,68,0.08);">
            <el-icon color="#ef4444" :size="22"><CircleCloseFilled /></el-icon>
          </div>
          <div class="stat-value" style="color: #ef4444;">{{ summary.critical_total }}</div>
          <div class="stat-label">严重告警</div>
        </div>
      </el-col>
    </el-row>

    <!-- Charts -->
    <el-row :gutter="20" style="margin-bottom: 24px;">
      <el-col :span="8">
        <div class="section-card">
          <div class="section-header">
            <h3><el-icon :size="16" color="#00d4ff"><PieChart /></el-icon> 指标状态分布</h3>
          </div>
          <div ref="pieChartRef" style="height: 280px;"></div>
        </div>
      </el-col>
      <el-col :span="16">
        <div class="section-card">
          <div class="section-header">
            <h3><el-icon :size="16" color="#00d4ff"><Histogram /></el-icon> 数据源健康对比</h3>
          </div>
          <div ref="barChartRef" style="height: 280px;"></div>
        </div>
      </el-col>
    </el-row>

    <!-- Per-datasource health cards -->
    <el-row :gutter="20" style="margin-bottom: 24px;">
      <el-col :span="8" v-for="ds in healthData" :key="ds.datasource.id">
        <div class="section-card">
          <div class="section-header">
            <h3>
              <span :style="{ width: 10, height: 10, borderRadius: '50%', background: ds.health_score >= 90 ? '#10b981' : ds.health_score >= 70 ? '#f59e0b' : '#ef4444', display: 'inline-block', boxShadow: '0 0 8px ' + (ds.health_score >= 90 ? '#10b981' : ds.health_score >= 70 ? '#f59e0b' : '#ef4444') }"></span>
              {{ ds.datasource.name }}
            </h3>
            <el-tag :style="{ background: ds.health_score >= 90 ? 'rgba(16,185,129,0.15)' : ds.health_score >= 70 ? 'rgba(245,158,11,0.15)' : 'rgba(239,68,68,0.15)', color: ds.health_score >= 90 ? '#10b981' : ds.health_score >= 70 ? '#f59e0b' : '#ef4444', border: 'none' }">
              {{ ds.health_score.toFixed(1) }}%
            </el-tag>
          </div>
          <div style="padding: 16px 20px;">
            <div style="display: flex; gap: 20px; margin-bottom: 14px;">
              <div><span style="color: var(--text-tertiary); font-size: 12px;">指标</span><br><span style="font-weight: 700; font-size: 18px;">{{ ds.total_metrics }}</span></div>
              <div><span style="color: var(--text-tertiary); font-size: 12px;">告警</span><br><span :style="{ fontWeight: 700, fontSize: 18, color: ds.alerts > 0 ? '#ef4444' : '#10b981' }">{{ ds.alerts }}</span></div>
              <div><span style="color: var(--text-tertiary); font-size: 12px;">严重</span><br><span :style="{ fontWeight: 700, fontSize: 18, color: ds.critical_count > 0 ? '#ef4444' : 'var(--text-tertiary)' }">{{ ds.critical_count }}</span></div>
            </div>
            <div style="height: 6px; background: rgba(255,255,255,0.06); border-radius: 4px; overflow: hidden;">
              <div :style="{ height: '100%', width: ds.health_score + '%', background: 'linear-gradient(90deg, #10b981, #00d4ff)', borderRadius: 4, transition: 'width 0.8s ease' }"></div>
            </div>
            <div style="margin-top: 12px;">
              <el-button size="small" text @click="switchDS(ds.datasource.id)" style="color: var(--cyan);">查看详情 →</el-button>
            </div>
          </div>
        </div>
      </el-col>
    </el-row>

    <!-- Metrics table -->
    <div class="section-card" v-if="expandedDS">
      <div class="section-header">
        <h3><el-icon :size="16" color="#00d4ff"><List /></el-icon> {{ expandedDS.datasource.name }} - 指标明细</h3>
        <el-button size="small" text @click="expandedDS = null" style="color: var(--text-tertiary);">收起</el-button>
      </div>
      <el-table :data="expandedDS.metrics" stripe size="small" max-height="400">
        <el-table-column prop="type_name" label="类型" min-width="180" />
        <el-table-column prop="metric_name" label="指标名称" min-width="160">
          <template #default="{ row }">
            <span style="font-weight: 600; color: var(--text-primary);">{{ row.metric_name }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="value" label="当前值" width="100" align="center">
          <template #default="{ row }">
            <span style="color: var(--text-tertiary);">{{ row.value }}</span>
          </template>
        </el-table-column>
        <el-table-column label="状态" width="100" align="center">
          <template #default="{ row }">
            <span :class="['status-badge', row.status]">
              {{ row.status === 'success' ? '正常' : row.status === 'critical' ? '严重' : '告警' }}
            </span>
          </template>
        </el-table-column>
        <el-table-column label="阈值" width="120" align="center">
          <template #default="{ row }">{{ row.threshold > 0 ? '> ' + row.threshold : '-' }}</template>
        </el-table-column>
        <el-table-column prop="unit" label="单位" width="80" align="center" />
      </el-table>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, nextTick, watch } from 'vue'
import { ElMessage } from 'element-plus'
import * as echarts from 'echarts'
import { getDataSources, getDashboardHealth } from '../api'
import type { DataSource } from '../types'

const loading = ref(false)
const allDatasources = ref<DataSource[]>([])
const selectedDS = ref<number | string>('')
const healthData = ref<any[]>([])
const overallHealth = ref(100)
const expandedDS = ref<any>(null)
const pieChartRef = ref<HTMLElement>()
const barChartRef = ref<HTMLElement>()
let pieChart: echarts.ECharts | null = null
let barChart: echarts.ECharts | null = null

const summary = computed(() => {
  const s = { normal_total: 0, warning_total: 0, critical_total: 0 }
  healthData.value.forEach((d: any) => {
    s.normal_total += d.normal_count
    s.warning_total += d.warning_count
    s.critical_total += d.critical_count
  })
  return s
})

function switchDS(id: number) {
  const d = healthData.value.find((h: any) => h.datasource.id === id)
  if (d) expandedDS.value = d
}

function renderCharts(data: any) {
  nextTick(() => {
    // Pie chart
    if (pieChartRef.value) {
      if (!pieChart) pieChart = echarts.init(pieChartRef.value)
      pieChart.setOption({
        backgroundColor: 'transparent',
        tooltip: { trigger: 'item', textStyle: { color: '#f1f5f9' }, backgroundColor: 'rgba(17,24,39,0.95)', borderColor: 'rgba(56,189,248,0.2)' },
        series: [{
          type: 'pie',
          radius: ['50%', '75%'],
          center: ['50%', '50%'],
          avoidLabelOverlap: true,
          padAngle: 2,
          itemStyle: { borderRadius: 6, borderColor: 'rgba(8,12,24,0.8)', borderWidth: 2 },
          label: { color: '#94a3b8', fontSize: 12 },
          labelLine: { lineStyle: { color: 'rgba(148,163,184,0.3)' } },
          data: [
            { value: summary.value.normal_total, name: '正常', itemStyle: { color: '#10b981', shadowBlur: 10, shadowColor: 'rgba(16,185,129,0.3)' } },
            { value: summary.value.warning_total, name: '告警', itemStyle: { color: '#f59e0b', shadowBlur: 10, shadowColor: 'rgba(245,158,11,0.3)' } },
            { value: summary.value.critical_total, name: '严重', itemStyle: { color: '#ef4444', shadowBlur: 10, shadowColor: 'rgba(239,68,68,0.3)' } },
          ].filter(d => d.value > 0),
        }],
      })
    }

    // Bar chart
    if (barChartRef.value) {
      if (!barChart) barChart = echarts.init(barChartRef.value)
      const names = data.map((d: any) => d.datasource.name)
      const scores = data.map((d: any) => parseFloat(d.health_score.toFixed(1)))
      barChart.setOption({
        backgroundColor: 'transparent',
        tooltip: { trigger: 'axis', textStyle: { color: '#f1f5f9' }, backgroundColor: 'rgba(17,24,39,0.95)', borderColor: 'rgba(56,189,248,0.2)' },
        grid: { left: '3%', right: '4%', bottom: '10%', top: '5%', containLabel: true },
        xAxis: {
          type: 'category', data: names, axisLabel: { color: '#94a3b8', fontSize: 11 },
          axisLine: { lineStyle: { color: 'rgba(56,189,248,0.1)' } },
          axisTick: { show: false },
        },
        yAxis: {
          type: 'value', min: 0, max: 100,
          splitLine: { lineStyle: { color: 'rgba(56,189,248,0.06)', type: 'dashed' } },
          axisLabel: { color: '#94a3b8', fontSize: 11, formatter: '{value}%' },
        },
        series: [{
          type: 'bar',
          barWidth: 32,
          borderRadius: [6, 6, 0, 0],
          data: scores.map((v: number) => ({
            value: v,
            itemStyle: {
              color: v >= 90 ? '#10b981' : v >= 70 ? '#f59e0b' : '#ef4444',
              shadowBlur: 8,
              shadowColor: v >= 90 ? 'rgba(16,185,129,0.3)' : v >= 70 ? 'rgba(245,158,11,0.3)' : 'rgba(239,68,68,0.3)',
            },
          })),
          label: {
            show: true, position: 'top', color: '#94a3b8', fontSize: 12, fontWeight: 600,
            formatter: '{c}%',
          },
        }],
      })
    }
  })
}

async function fetchData() {
  loading.value = true
  try {
    const res = await getDashboardHealth(selectedDS.value ? Number(selectedDS.value) : undefined)
    healthData.value = res.data.datasources
    overallHealth.value = res.data.overall_health
    expandedDS.value = null
    renderCharts(res.data.datasources)
  } catch (e: any) {
    ElMessage.error(e.message)
  } finally {
    loading.value = false
  }
}

watch(selectedDS, fetchData)

onMounted(async () => {
  try {
    const ds = await getDataSources()
    allDatasources.value = ds.data
  } catch { /* ignore */ }
  await fetchData()
})

onUnmounted(() => {
  pieChart?.dispose()
  barChart?.dispose()
})
</script>
