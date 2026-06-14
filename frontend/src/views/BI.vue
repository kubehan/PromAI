<template>
  <div class="page-container">
    <div class="page-header" style="display: flex; justify-content: space-between; align-items: flex-start;">
      <div>
        <h2><el-icon><DataAnalysis /></el-icon> 健康大屏</h2>
        <p>全景巡检健康状态 BI 看板</p>
      </div>
      <div style="display: flex; gap: 12px; align-items: center;">
        <el-select v-model="selectedDS" placeholder="全部数据源" filterable clearable style="width: 200px;" @change="fetchData">
          <el-option label="全部数据源" value="" />
          <el-option v-for="ds in allDatasources" :key="ds.id" :label="ds.name" :value="ds.id" />
        </el-select>
        <el-button plain @click="fetchData" :loading="loading"><el-icon><Refresh /></el-icon> 刷新</el-button>
      </div>
    </div>

    <grid-layout
      v-model:layout="layout"
      :col-num="12"
      :row-height="30"
      :margin="[12, 12]"
      :is-draggable="true"
      :is-resizable="true"
      :vertical-compact="true"
      @layout-updated="layoutUpdated"
    >
      <!-- Stat cards as individual blocks -->
      <grid-item v-for="(s, i) in overviewStats" :key="'stat-' + i"
        :x="item('stat-' + i).x" :y="item('stat-' + i).y" :w="item('stat-' + i).w" :h="item('stat-' + i).h"
        :i="'stat-' + i" drag-allow-from=".stat-card">
        <div class="stat-card" :style="{ borderLeftColor: s.color }">
          <div class="stat-title">{{ s.label }}</div>
          <div class="stat-card-body">
            <div class="stat-icon" :style="{ background: s.bg }">
              <el-icon :color="s.color" :size="20"><component :is="s.icon" /></el-icon>
            </div>
            <div class="stat-value" :style="{ color: s.color }">{{ s.value }}</div>
          </div>
        </div>
      </grid-item>

      <!-- Datasource health donut -->
      <grid-item :x="item('donut').x" :y="item('donut').y" :w="item('donut').w" :h="item('donut').h" i="donut"
        drag-allow-from=".stat-card">
        <div class="stat-card" style="  border-left-color: var(--cyan);">
          <div class="stat-title">数据源分布</div>
          <div ref="dsDonutRef" style="flex: 1; min-height: 0; width: 100%;"></div>
        </div>
      </grid-item>

      <!-- Abnormal metrics table -->
      <grid-item :x="item('abnormal').x" :y="item('abnormal').y" :w="item('abnormal').w" :h="item('abnormal').h" i="abnormal"
        drag-allow-from=".grid-drag-handle">
        <div class="section-card h-full" :style="{ borderColor: abnormalMetrics.length > 0 ? 'rgba(239,68,68,0.2)' : undefined }">
          <div class="section-header grid-drag-handle" style="cursor: grab;">
            <h3>
              <el-icon :size="16" color="#ef4444"><WarningFilled /></el-icon>
              <span style="color: #ef4444;">异常指标</span>
              <el-tag size="small" style="margin-left: 10px; background: rgba(239,68,68,0.15); color: #ef4444; border: none;" v-if="abnormalMetrics.length > 0">{{ abnormalMetrics.length }} 项</el-tag>
            </h3>
            <div style="display: flex; gap: 10px; align-items: center;" v-if="abnormalMetrics.length > 0">
              <el-select v-model="abnormalStatusFilter" placeholder="状态" size="small" style="width: 60px;" clearable>
                <el-option label="严重" value="critical" /><el-option label="告警" value="warning" />
              </el-select>
              <el-select v-model="abnormalTypeFilter" placeholder="类型" size="small" style="width: 100px;" clearable>
                <el-option v-for="t in typeOptions" :key="t" :label="t" :value="t" />
              </el-select>
              <el-input v-model="abnormalSearch" placeholder="搜索" size="small" style="width: 140px;" clearable />
              <el-button size="small" text @click="exportCSV" style="color: var(--cyan);">导出CSV</el-button>
            </div>
          </div>
          <div v-if="abnormalMetrics.length > 0" style="overflow: auto; flex: 1;">
            <el-table :data="groupedAbnormalMetrics" stripe size="small" height="100%">
              <el-table-column label="数据源" width="100"><template #default="{ row }"><span style="font-weight: 600; color: var(--text-primary); font-size: 12px;">{{ row.datasource_name }}</span></template></el-table-column>
              <el-table-column prop="type_name" label="类型" min-width="90" />
              <el-table-column prop="metric_name" label="指标名称" min-width="120"><template #default="{ row }"><span style="font-weight: 600; color: var(--text-primary); font-size: 12px;">{{ row.metric_name }}</span></template></el-table-column>
              <el-table-column label="标签" width="280"><template #default="{ row }"><div style="display: flex; flex-wrap: wrap; gap: 2px 6px;"><template v-for="(v, k) in row.labels" :key="k"><span v-if="v && v !== '-'" style="font-size: 11px; color: var(--text-tertiary);"><span style="color: var(--text-secondary);">{{ k }}:</span> {{ v }}</span></template></div></template></el-table-column>
              <el-table-column prop="value" label="值" width="70" align="center"><template #default="{ row }"><span style="color: var(--text-tertiary);">{{ typeof row.value === 'number' ? row.value.toFixed(2) : row.value }}</span></template></el-table-column>
              <el-table-column label="状态" width="60" align="center"><template #default="{ row }"><span :class="['status-badge', row.status]">{{ row.status === 'critical' ? '严重' : '告警' }}</span></template></el-table-column>
              <el-table-column label="阈值" width="80" align="center"><template #default="{ row }"><span style="color: var(--text-tertiary); font-size: 11px;">{{ row.threshold > 0 ? (row.threshold_type === 'less' ? '< ' : '> ') + row.threshold : '-' }}</span></template></el-table-column>
              <el-table-column label="时间" width="120" align="center"><template #default="{ row }"><span style="color: var(--text-tertiary); font-size: 11px;">{{ row.last_report_at ? new Date(row.last_report_at).toLocaleString() : '-' }}</span></template></el-table-column>
            </el-table>
          </div>
          <el-empty v-else description="暂无异常指标" :image-size="50" />
        </div>
      </grid-item>

      <!-- Pie Chart -->
      <grid-item :x="item('pie').x" :y="item('pie').y" :w="item('pie').w" :h="item('pie').h" i="pie"
        drag-allow-from=".grid-drag-handle">
        <div class="section-card h-full">
          <div class="section-header grid-drag-handle" style="cursor: grab;">
            <h3><el-icon :size="16" :color="getCssVar('--cyan')"><PieChart /></el-icon> 指标状态分布</h3>
          </div>
          <div ref="pieChartRef" style="flex: 1; min-height: 0;"></div>
        </div>
      </grid-item>

      <!-- Type Alert Distribution -->
      <grid-item :x="item('type-bar').x" :y="item('type-bar').y" :w="item('type-bar').w" :h="item('type-bar').h" i="type-bar"
        drag-allow-from=".grid-drag-handle">
        <div class="section-card h-full">
          <div class="section-header grid-drag-handle" style="cursor: grab;">
            <h3><el-icon :size="16" color="#f59e0b"><WarningFilled /></el-icon> 异常类型分布</h3>
            <span style="color: var(--text-tertiary); font-size: 11px;">Top-15</span>
          </div>
          <div ref="typeBarRef" style="flex: 1; min-height: 0;"></div>
        </div>
      </grid-item>

      <!-- Health Score Distribution -->
      <grid-item :x="item('distribution').x" :y="item('distribution').y" :w="item('distribution').w" :h="item('distribution').h" i="distribution"
        drag-allow-from=".grid-drag-handle">
        <div class="section-card h-full">
          <div class="section-header grid-drag-handle" style="cursor: grab;">
            <h3><el-icon :size="16" :color="getCssVar('--cyan')"><Histogram /></el-icon> 健康分分布</h3>
            <span style="color: var(--text-tertiary); font-size: 11px;">数据源维度</span>
          </div>
          <div ref="distChartRef" style="flex: 1; min-height: 0;"></div>
        </div>
      </grid-item>

      <!-- Trend chart -->
      <grid-item :x="item('trend').x" :y="item('trend').y" :w="item('trend').w" :h="item('trend').h" i="trend"
        drag-allow-from=".grid-drag-handle">
        <div class="section-card h-full">
          <div class="section-header grid-drag-handle" style="cursor: grab;">
            <h3><el-icon :size="16" :color="getCssVar('--cyan')"><TrendCharts /></el-icon> 异常趋势（近 {{ trendDays }} 天）</h3>
            <el-radio-group v-model="trendDays" size="small" @change="fetchTrend">
              <el-radio-button :value="7">7天</el-radio-button>
              <el-radio-button :value="14">14天</el-radio-button>
              <el-radio-button :value="30">30天</el-radio-button>
            </el-radio-group>
          </div>
          <div ref="trendChartRef" style="flex: 1; min-height: 0;"></div>
        </div>
      </grid-item>

      <!-- Datasource health table -->
      <grid-item :x="item('ds-list').x" :y="item('ds-list').y" :w="item('ds-list').w" :h="item('ds-list').h" i="ds-list"
        drag-allow-from=".grid-drag-handle">
        <div class="section-card h-full">
          <div class="section-header grid-drag-handle" style="cursor: grab;">
            <h3><el-icon :size="16" :color="getCssVar('--cyan')"><Connection /></el-icon> 数据源健康</h3>
            <div style="display: flex; gap: 8px; align-items: center;">
              <el-input v-model="dsSearch" placeholder="搜索" size="small" style="width: 160px;" clearable />
              <span style="color: var(--text-tertiary); font-size: 12px;">{{ filteredDatasources.length }} 个</span>
            </div>
          </div>
          <div style="overflow: auto; flex: 1; display: flex; flex-direction: column;">
            <el-table :data="dsPageData" stripe size="small" @row-click="switchDS" style="cursor: pointer;" height="100%"
              @sort-change="handleSortChange" :default-sort="{ prop: 'health_score', order: 'ascending' }">
              <el-table-column label="数据源" min-width="140">
                <template #default="{ row }">
                  <span style="display: flex; align-items: center; gap: 5px;">
                    <span :style="{ width: 7, height: 7, borderRadius: '50%', flexShrink: 0, background: row.health_score >= 90 ? '#10b981' : row.health_score >= 70 ? '#f59e0b' : '#ef4444', boxShadow: '0 0 5px ' + (row.health_score >= 90 ? 'rgba(16,185,129,0.5)' : row.health_score >= 70 ? 'rgba(245,158,11,0.5)' : 'rgba(239,68,68,0.5)') }"></span>
                    <span style="font-weight: 600; color: var(--text-primary); font-size: 12px;">{{ row.datasource.name }}</span>
                  </span>
                </template>
              </el-table-column>
              <el-table-column label="健康分" prop="health_score" sortable="custom" width="120" align="center">
                <template #default="{ row }">
                  <span :style="{ fontWeight: 700, fontSize: 12, color: row.health_score >= 90 ? '#10b981' : row.health_score >= 70 ? '#f59e0b' : '#ef4444' }">{{ row.health_score.toFixed(1) }}%</span>
                </template>
              </el-table-column>
              <el-table-column label="指标" prop="total_metrics" sortable="custom" width="60" align="center"><template #default="{ row }"><span style="color: var(--text-tertiary); font-size: 12px;">{{ row.total_metrics }}</span></template></el-table-column>
              <el-table-column label="告警" prop="warning_count" sortable="custom" width="60" align="center"><template #default="{ row }"><span :style="{ color: row.warning_count > 0 ? '#f59e0b' : 'var(--text-tertiary)', fontWeight: row.warning_count > 0 ? 700 : 400, fontSize: 12 }">{{ row.warning_count }}</span></template></el-table-column>
              <el-table-column label="严重" prop="critical_count" sortable="custom" width="60" align="center"><template #default="{ row }"><span :style="{ color: row.critical_count > 0 ? '#ef4444' : 'var(--text-tertiary)', fontWeight: row.critical_count > 0 ? 700 : 400, fontSize: 12 }">{{ row.critical_count }}</span></template></el-table-column>
              <el-table-column label="操作" width="90" align="center">
                <template #default="{ row }">
                  <el-button size="small" text @click.stop="switchDS(row)" style="color: var(--cyan); font-size: 12px;">详情</el-button>
                  <el-button size="small" text @click.stop="viewReport(row)" style="color: var(--cyan); font-size: 12px;">报告</el-button>
                </template>
              </el-table-column>
            </el-table>
            <div style="display: flex; justify-content: flex-end; padding: 8px 0 0;">
              <el-pagination v-model:current-page="dsPage" v-model:page-size="dsPageSize" :total="filteredDatasources.length" :page-sizes="[10, 15, 20, 30]" layout="total, sizes, prev, pager, next" background small />
            </div>
          </div>
        </div>
      </grid-item>

      <!-- Metrics detail -->
      <grid-item :x="item('detail').x" :y="item('detail').y" :w="item('detail').w" :h="item('detail').h" i="detail"
        drag-allow-from=".grid-drag-handle">
        <div class="section-card h-full">
          <div class="section-header grid-drag-handle" style="cursor: grab;">
            <h3><el-icon :size="16" :color="getCssVar('--cyan')"><List /></el-icon> <span v-if="expandedDS">{{ expandedDS.datasource.name }} - </span>指标明细</h3>
            <div style="display: flex; gap: 8px;" v-if="expandedDS">
              <el-button size="small" text @click="exportDetailCSV" style="color: var(--cyan);">导出CSV</el-button>
              <el-button size="small" text @click="expandedDS = null" style="color: var(--text-tertiary);">收起</el-button>
            </div>
          </div>
          <div v-if="expandedDS" style="overflow: auto; flex: 1;">
            <div style="padding: 4px 16px 0;">
              <div style="display: flex; flex-wrap: wrap; gap: 4px; margin-bottom: 8px;">
                <el-tag :type="typeFilter === '' ? '' : 'info'" :effect="typeFilter === '' ? 'dark' : 'plain'" size="small" style="cursor: pointer;" @click="typeFilter = ''">全部</el-tag>
                <el-tag v-for="t in typeOptions" :key="t" :type="typeFilter === t ? '' : 'info'" :effect="typeFilter === t ? 'dark' : 'plain'" size="small" style="cursor: pointer;" @click="typeFilter = t">{{ t }}</el-tag>
                <el-input v-model="metricSearch" placeholder="搜索" size="small" style="width: 160px; margin-left: auto;" clearable />
              </div>
              <div style="display: flex; flex-wrap: wrap; gap: 6px; margin-bottom: 10px;" v-if="expandedDS.type_summaries">
                <div v-for="s in expandedDS.type_summaries" :key="s.type_name" style="display: flex; align-items: center; gap: 5px; padding: 4px 10px; border-radius: 6px; background: rgba(255,255,255,0.03); border: 1px solid rgba(255,255,255,0.05); font-size: 12px;">
                  <span :class="['status-dot', s.alerts > 0 ? (s.critical_count > 0 ? 'critical' : 'warning') : 'normal']"></span>
                  <span style="color: var(--text-primary); font-weight: 600;">{{ s.type_name }}</span>
                  <span style="color: var(--text-tertiary);">总{{ s.total_metrics }}</span>
                  <span v-if="s.critical_count > 0" style="color: #ef4444;">严重{{ s.critical_count }}</span>
                  <span v-if="s.warning_count > 0" style="color: #f59e0b;">告警{{ s.warning_count }}</span>
                  <span v-if="s.normal_count > 0" style="color: #10b981;">正常{{ s.normal_count }}</span>
                </div>
              </div>
            </div>
            <div style="padding: 0 16px 8px;">
              <el-table :data="filteredMetrics" stripe size="small" height="100%">
                <el-table-column prop="type_name" label="类型" min-width="100" />
                <el-table-column prop="metric_name" label="指标名称" min-width="120"><template #default="{ row }"><span style="font-weight: 600; color: var(--text-primary); font-size: 12px;">{{ row.metric_name }}</span></template></el-table-column>
                <el-table-column label="标签" width="280"><template #default="{ row }"><div style="display: flex; flex-wrap: wrap; gap: 2px 6px;"><template v-for="(v, k) in row.labels" :key="k"><span v-if="v && v !== '-'" style="font-size: 11px; color: var(--text-tertiary);"><span style="color: var(--text-secondary);">{{ k }}:</span> {{ v }}</span></template></div></template></el-table-column>
                <el-table-column prop="value" label="值" width="70" align="center"><template #default="{ row }"><span style="color: var(--text-tertiary);">{{ typeof row.value === 'number' ? row.value.toFixed(2) : row.value }}</span></template></el-table-column>
                <el-table-column label="状态" width="60" align="center"><template #default="{ row }"><span :class="['status-badge', row.status]">{{ row.status === 'normal' || row.status === 'success' ? '正常' : row.status === 'critical' ? '严重' : '告警' }}</span></template></el-table-column>
                <el-table-column label="阈值" width="80" align="center"><template #default="{ row }">{{ row.threshold > 0 ? row.threshold_type === 'less' ? '< ' : '> ' : '' }}{{ row.threshold > 0 ? row.threshold : '-' }}</template></el-table-column>
                <el-table-column prop="unit" label="单位" width="50" align="center" />
              </el-table>
            </div>
          </div>
          <div v-else style="padding: 20px; text-align: center; color: var(--text-tertiary);">点击数据源「详情」查看指标明细</div>
        </div>
      </grid-item>
    </grid-layout>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, nextTick, watch } from 'vue'
// @ts-ignore
import { GridLayout, GridItem } from 'vue3-grid-layout'
import { ElMessage } from 'element-plus'
import * as echarts from 'echarts'
import { getAllDataSources, getDashboardHealth, getDashboardHealthTrend } from '../api'
import type { DataSource } from '../types'

function getCssVar(name: string): string {
  return getComputedStyle(document.documentElement).getPropertyValue(name).trim()
}

const loading = ref(false)
const allDatasources = ref<DataSource[]>([])
const selectedDS = ref<number | string>('')
const healthData = ref<any[]>([])
const overallHealth = ref(100)
const totalDatasources = ref(0)
const healthDistribution = ref<any[]>([])
const typeAlerts = ref<any[]>([])
const expandedDS = ref<any>(null)
const typeFilter = ref('')
const metricSearch = ref('')
const abnormalStatusFilter = ref('')
const abnormalTypeFilter = ref('')
const abnormalSearch = ref('')
const trendDays = ref(14)
const dsSearch = ref('')
const dsSortProp = ref('health_score')
const dsSortOrder = ref<'asc' | 'desc'>('asc')
const dsPage = ref(1)
const dsPageSize = ref(10)

const pieChartRef = ref<HTMLElement>()
const typeBarRef = ref<HTMLElement>()
const distChartRef = ref<HTMLElement>()
const trendChartRef = ref<HTMLElement>()
const dsDonutRef = ref<HTMLElement>()
let pieChart: echarts.ECharts | null = null
let typeBarChart: echarts.ECharts | null = null
let distChart: echarts.ECharts | null = null
let trendChart: echarts.ECharts | null = null
let dsDonutChart: echarts.ECharts | null = null

const defaultLayout = [
  { x: 0, y: 0, w: 2, h: 4, i: 'stat-0' },
  { x: 2, y: 0, w: 2, h: 4, i: 'stat-1' },
  { x: 4, y: 0, w: 2, h: 4, i: 'stat-2' },
  { x: 6, y: 0, w: 2, h: 4, i: 'stat-3' },
  { x: 8, y: 0, w: 2, h: 4, i: 'stat-4' },
  { x: 10, y: 0, w: 2, h: 4, i: 'donut' },
  { x: 0, y: 4, w: 12, h: 10, i: 'abnormal' },
  { x: 0, y: 14, w: 4, h: 10, i: 'pie' },
  { x: 4, y: 14, w: 4, h: 10, i: 'type-bar' },
  { x: 8, y: 14, w: 4, h: 10, i: 'distribution' },
  { x: 0, y: 24, w: 12, h: 10, i: 'trend' },
  { x: 0, y: 34, w: 12, h: 12, i: 'ds-list' },
  { x: 0, y: 46, w: 12, h: 12, i: 'detail' },
]
const savedLayout = localStorage.getItem('bi_dashboard_layout')
let parsedLayout = null
if (savedLayout) {
  try {
    const p = JSON.parse(savedLayout)
    if (Array.isArray(p) && p.length > 0 && p.every((i: any) => typeof i.i === 'string')) {
      // validate all required item IDs exist
      const ids = new Set(p.map((i: any) => i.i))
      if (defaultLayout.every((d: any) => ids.has(d.i))) {
        parsedLayout = p
      }
    }
  } catch { /* ignore corrupted data */ }
}
const layout = ref(parsedLayout || defaultLayout)

watch(layout, (val) => {
  localStorage.setItem('bi_dashboard_layout', JSON.stringify(val))
}, { deep: true })

const dsHealthCounts = computed(() => {
  let healthy = 0, warning = 0, critical = 0
  healthData.value.forEach((d: any) => {
    if (d.health_score >= 90) healthy++
    else if (d.health_score >= 70) warning++
    else critical++
  })
  return { healthy, warning, critical }
})

const overviewStats = computed(() => [
  { label: '健康分', value: overallHealth.value.toFixed(1) + '%', icon: 'DataAnalysis', color: overallHealth.value >= 90 ? getCssVar('--emerald') : overallHealth.value >= 70 ? getCssVar('--amber') : getCssVar('--red'), bg: getCssVar('--cyan-dim') },
  { label: '数据源', value: totalDatasources.value, icon: 'Connection', color: getCssVar('--cyan'), bg: getCssVar('--cyan-dim') },
  { label: '正常指标', value: summary.value.normal_total, icon: 'Check', color: getCssVar('--emerald'), bg: getCssVar('--emerald-dim') },
  { label: '告警指标', value: summary.value.warning_total, icon: 'WarningFilled', color: getCssVar('--amber'), bg: getCssVar('--amber-dim') },
  { label: '严重指标', value: summary.value.critical_total, icon: 'CircleCloseFilled', color: getCssVar('--red'), bg: getCssVar('--red-dim') },
])

const summary = computed(() => {
  const s = { normal_total: 0, warning_total: 0, critical_total: 0 }
  healthData.value.forEach((d: any) => { s.normal_total += d.normal_count; s.warning_total += d.warning_count; s.critical_total += d.critical_count })
  return s
})

const filteredDatasources = computed(() => {
  let list = [...healthData.value]
  if (dsSearch.value) {
    const q = dsSearch.value.toLowerCase()
    list = list.filter((d: any) => d.datasource.name.toLowerCase().includes(q))
  }
  const prop = dsSortProp.value as string
  const order = dsSortOrder.value
  list.sort((a: any, b: any) => {
    const va = a[prop] ?? 0, vb = b[prop] ?? 0
    return order === 'asc' ? va - vb : vb - va
  })
  return list
})

const dsPageData = computed(() => {
  const start = (dsPage.value - 1) * dsPageSize.value
  return filteredDatasources.value.slice(start, start + dsPageSize.value)
})

const typeOptions = computed(() => {
  const set = new Set<string>()
  healthData.value.forEach((d: any) => d.metrics?.forEach((m: any) => { if (m.type_name) set.add(m.type_name) }))
  return Array.from(set).sort()
})

const abnormalMetrics = computed(() => {
  const list: any[] = []
  healthData.value.forEach((d: any) => {
    d.metrics?.forEach((m: any) => {
      if (m.status === 'critical' || m.status === 'warning') {
        list.push({ ...m, datasource_name: d.datasource.name, datasource_id: d.datasource.id, last_report_at: d.last_report_at })
      }
    })
  })
  list.sort((a, b) => a.status === 'critical' && b.status !== 'critical' ? -1 : b.status === 'critical' && a.status !== 'critical' ? 1 : 0)
  return list
})

const filteredAbnormalMetrics = computed(() => {
  let list = abnormalMetrics.value
  if (abnormalStatusFilter.value) list = list.filter(m => m.status === abnormalStatusFilter.value)
  if (abnormalTypeFilter.value) list = list.filter(m => m.type_name === abnormalTypeFilter.value)
  if (abnormalSearch.value) { const q = abnormalSearch.value.toLowerCase(); list = list.filter(m => m.metric_name.toLowerCase().includes(q) || m.type_name.toLowerCase().includes(q)) }
  return list
})

const groupedAbnormalMetrics = computed(() => {
  const list = [...filteredAbnormalMetrics.value]
  list.sort((a, b) => { if (a.type_name !== b.type_name) return a.type_name.localeCompare(b.type_name); return a.status === 'critical' && b.status !== 'critical' ? -1 : b.status === 'critical' && a.status !== 'critical' ? 1 : 0 })
  return list
})

const filteredMetrics = computed(() => {
  if (!expandedDS.value?.metrics) return []
  let list = expandedDS.value.metrics
  if (typeFilter.value) list = list.filter((m: any) => m.type_name === typeFilter.value)
  if (metricSearch.value) { const q = metricSearch.value.toLowerCase(); list = list.filter((m: any) => m.metric_name.toLowerCase().includes(q) || m.type_name.toLowerCase().includes(q)) }
  return [...list].sort((a: any, b: any) => a.type_name?.localeCompare(b.type_name) || 0)
})

watch(dsSearch, () => { dsPage.value = 1 })

function item(id: string) {
  const found = layout.value.find((i: any) => i.i === id)
  return found || { x: 0, y: 0, w: 12, h: 6 }
}

function layoutUpdated(newLayout: any[]) { layout.value = newLayout }

function handleSortChange({ prop, order }: any) {
  if (prop) { dsSortProp.value = prop; dsSortOrder.value = order === 'descending' ? 'desc' : 'asc' }
}

function switchDS(row: any) {
  const d = healthData.value.find((h: any) => h.datasource.id === row.datasource?.id || h.datasource.id === row.id)
  if (d) expandedDS.value = d
  typeFilter.value = ''; metricSearch.value = ''
}

function viewReport(ds: any) {
  if (ds.last_report_url) window.open(ds.last_report_url, '_blank')
  else ElMessage.info('暂无报告')
}

function exportDetailCSV() {
  const data = filteredMetrics.value
  if (!data.length) { ElMessage.info('没有可导出的数据'); return }
  const headers = ['类型', '指标名称', '标签', '当前值', '状态', '阈值', '单位']
  const rows = data.map((row: any) => {
    const labelStr = row.labels ? Object.entries(row.labels).filter(([_, v]) => v && v !== '-').map(([k, v]) => `${k}:${v}`).join('; ') : ''
    return [row.type_name, row.metric_name, labelStr, typeof row.value === 'number' ? row.value.toFixed(2) : row.value, row.status === 'critical' ? '严重' : row.status === 'warning' ? '告警' : '正常', row.threshold > 0 ? (row.threshold_type === 'less' ? '< ' : '> ') + row.threshold : '-', row.unit || '']
  })
  downloadCSV([headers, ...rows], `${expandedDS.value?.datasource?.name || '指标明细'}_${new Date().toISOString().slice(0, 10)}.csv`)
}

function exportCSV() {
  const data = groupedAbnormalMetrics.value
  if (!data.length) { ElMessage.info('没有可导出的数据'); return }
  const headers = ['数据源', '类型', '指标名称', '标签', '当前值', '状态', '阈值', '单位', '检测时间']
  const rows = data.map((row: any) => {
    const labelStr = row.labels ? Object.entries(row.labels).filter(([_, v]) => v && v !== '-').map(([k, v]) => `${k}:${v}`).join('; ') : ''
    return [row.datasource_name, row.type_name, row.metric_name, labelStr, typeof row.value === 'number' ? row.value.toFixed(2) : row.value, row.status === 'critical' ? '严重' : '告警', row.threshold > 0 ? (row.threshold_type === 'less' ? '< ' : '> ') + row.threshold : '-', row.unit || '', row.last_report_at ? new Date(row.last_report_at).toLocaleString() : '']
  })
  downloadCSV([headers, ...rows], `异常指标_${new Date().toISOString().slice(0, 10)}.csv`)
}

function downloadCSV(rows: string[][], filename: string) {
  const csv = rows.map(r => r.map(c => `"${String(c).replace(/"/g, '""')}"`).join(',')).join('\n')
  const blob = new Blob(['\uFEFF' + csv], { type: 'text/csv;charset=utf-8;' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a'); a.href = url; a.download = filename
  document.body.appendChild(a); a.click(); document.body.removeChild(a); URL.revokeObjectURL(url)
}

function getChartTheme() {
  return {
    textPrimary: getCssVar('--text-primary'),
    textSecondary: getCssVar('--text-secondary'),
    textTertiary: getCssVar('--text-tertiary'),
    bgCard: getCssVar('--bg-card'),
    bgElevated: getCssVar('--bg-elevated'),
    bgPrimary: getCssVar('--bg-primary'),
    border: getCssVar('--border'),
    cyan: getCssVar('--cyan'),
    emerald: getCssVar('--emerald'),
    amber: getCssVar('--amber'),
    red: getCssVar('--red'),
    purple: getCssVar('--purple'),
  }
}

function renderCharts() {
  nextTick(() => {
    const t = getChartTheme()
    if (dsDonutRef.value) {
      if (!dsDonutChart) dsDonutChart = echarts.init(dsDonutRef.value)
      const { healthy, warning, critical } = dsHealthCounts.value
      const total = healthy + warning + critical
      dsDonutChart.setOption({
        backgroundColor: 'transparent',
        tooltip: { trigger: 'item', textStyle: { color: t.textPrimary }, backgroundColor: t.bgCard, borderColor: t.border },
        series: [{ type: 'pie', radius: ['45%', '70%'], center: ['50%', '50%'], avoidLabelOverlap: true, padAngle: 2, itemStyle: { borderRadius: 6, borderColor: t.bgPrimary || 'transparent', borderWidth: 2 }, label: { color: t.textSecondary, fontSize: 11, formatter: (p: any) => `${p.name}\n${p.value}` }, labelLine: { lineStyle: { color: t.textTertiary } }, data: [{ value: healthy, name: '健康', itemStyle: { color: t.emerald, shadowBlur: 10, shadowColor: 'rgba(16,185,129,0.3)' } }, { value: warning, name: '告警', itemStyle: { color: t.amber, shadowBlur: 10, shadowColor: 'rgba(245,158,11,0.3)' } }, { value: critical, name: '严重', itemStyle: { color: t.red, shadowBlur: 10, shadowColor: 'rgba(239,68,68,0.3)' } }].filter(d => d.value > 0) }],
      })
    }

    if (pieChartRef.value) {
      if (!pieChart) pieChart = echarts.init(pieChartRef.value)
      pieChart.setOption({
        backgroundColor: 'transparent',
        tooltip: { trigger: 'item', textStyle: { color: t.textPrimary }, backgroundColor: t.bgCard, borderColor: t.border },
        series: [{ type: 'pie', radius: ['45%', '70%'], center: ['50%', '50%'], avoidLabelOverlap: true, padAngle: 2, itemStyle: { borderRadius: 6, borderColor: t.bgPrimary || 'transparent', borderWidth: 2 }, label: { color: t.textSecondary, fontSize: 11 }, labelLine: { lineStyle: { color: t.textTertiary } }, data: [{ value: summary.value.normal_total, name: '正常', itemStyle: { color: t.emerald, shadowBlur: 10, shadowColor: 'rgba(16,185,129,0.3)' } }, { value: summary.value.warning_total, name: '告警', itemStyle: { color: t.amber, shadowBlur: 10, shadowColor: 'rgba(245,158,11,0.3)' } }, { value: summary.value.critical_total, name: '严重', itemStyle: { color: t.red, shadowBlur: 10, shadowColor: 'rgba(239,68,68,0.3)' } }].filter(d => d.value > 0) }],
      })
    }

    if (typeBarRef.value) {
      if (!typeBarChart) typeBarChart = echarts.init(typeBarRef.value)
      const top = (typeAlerts.value || []).slice(0, 15)
      typeBarChart.setOption({
        backgroundColor: 'transparent',
        tooltip: { trigger: 'axis', axisPointer: { type: 'shadow' }, textStyle: { color: t.textPrimary }, backgroundColor: t.bgCard, borderColor: t.border },
        legend: { data: ['严重', '告警'], textStyle: { color: t.textSecondary, fontSize: 11 }, top: 0, right: 0 },
        grid: { left: '3%', right: '4%', bottom: '8%', top: '20%', containLabel: true },
        xAxis: { type: 'value', splitLine: { lineStyle: { color: t.border, type: 'dashed' } }, axisLabel: { color: t.textSecondary, fontSize: 10 } },
        yAxis: { type: 'category', data: top.map((t: any) => t.type_name), axisLabel: { color: t.textSecondary, fontSize: 10 }, axisLine: { lineStyle: { color: t.border } }, axisTick: { show: false } },
        series: [{ name: '严重', type: 'bar', stack: 'total', barWidth: 14, itemStyle: { color: t.red }, data: top.map((t: any) => t.critical_count) }, { name: '告警', type: 'bar', stack: 'total', barWidth: 14, itemStyle: { color: t.amber, borderRadius: [0, 4, 4, 0] }, data: top.map((t: any) => t.warning_count) }],
      })
    }

    if (distChartRef.value) {
      if (!distChart) distChart = echarts.init(distChartRef.value)
      const dist = healthDistribution.value || []
      distChart.setOption({
        backgroundColor: 'transparent',
        tooltip: { trigger: 'axis', axisPointer: { type: 'shadow' }, formatter: (params: any) => { const p = params[0]; const d = dist[p.dataIndex]; return `${d.range} 分<br/>数据源: ${d.count} 个<br/>占比: ${d.pct?.toFixed(1) || 0}%` }, textStyle: { color: t.textPrimary }, backgroundColor: t.bgCard, borderColor: t.border },
        grid: { left: '3%', right: '4%', bottom: '10%', top: '15%', containLabel: true },
        xAxis: { type: 'category', data: dist.map((d: any) => d.range), axisLabel: { color: t.textSecondary, fontSize: 11 }, axisLine: { lineStyle: { color: t.border } }, axisTick: { show: false } },
        yAxis: { type: 'value', min: 0, splitLine: { lineStyle: { color: t.border, type: 'dashed' } }, axisLabel: { color: t.textSecondary, fontSize: 10 } },
        series: [{ type: 'bar', barWidth: 30, borderRadius: [6, 6, 0, 0], label: { show: true, position: 'top', color: t.textSecondary, fontSize: 11, fontWeight: 600 }, data: dist.map((d: any, i: number) => ({ value: d.count, itemStyle: { color: [t.red, '#fb923c', t.amber, '#34d399', t.emerald][i], shadowBlur: 8, shadowColor: [,, 'rgba(239,68,68,0.3)', 'rgba(251,146,60,0.3)', 'rgba(245,158,11,0.3)', 'rgba(52,211,153,0.3)', 'rgba(16,185,129,0.3)'][i] } })) }],
      })
    }
  })
}

function renderTrendChart(trend: any[]) {
  nextTick(() => {
    if (!trendChartRef.value) return
    if (!trendChart) trendChart = echarts.init(trendChartRef.value)
    const t = getChartTheme()
    trendChart.setOption({
      backgroundColor: 'transparent',
      tooltip: { trigger: 'axis', textStyle: { color: t.textPrimary }, backgroundColor: t.bgCard, borderColor: t.border },
      legend: { data: ['严重', '告警'], textStyle: { color: t.textSecondary, fontSize: 12 }, top: 0, right: 0 },
      grid: { left: '3%', right: '4%', bottom: '8%', top: '20%', containLabel: true },
      xAxis: { type: 'category', data: trend.map((d: any) => d.date), boundaryGap: false, axisLabel: { color: t.textSecondary, fontSize: 11 }, axisLine: { lineStyle: { color: t.border } }, axisTick: { show: false } },
      yAxis: { type: 'value', min: 0, splitLine: { lineStyle: { color: t.border, type: 'dashed' } }, axisLabel: { color: t.textSecondary, fontSize: 11 } },
      series: [
        { name: '严重', type: 'line', smooth: true, symbol: 'circle', symbolSize: 5, lineStyle: { color: t.red, width: 2 }, itemStyle: { color: t.red }, areaStyle: { color: 'rgba(239,68,68,0.1)' }, data: trend.map((d: any) => d.critical) },
        { name: '告警', type: 'line', smooth: true, symbol: 'circle', symbolSize: 5, lineStyle: { color: t.amber, width: 2 }, itemStyle: { color: t.amber }, areaStyle: { color: 'rgba(245,158,11,0.1)' }, data: trend.map((d: any) => d.warning) },
      ],
    })
  })
}

async function fetchTrend() {
  try { const res = await getDashboardHealthTrend(trendDays.value); renderTrendChart(res.data.trend) } catch { /* ignore */ }
}

async function fetchData() {
  loading.value = true
  try {
    const res = await getDashboardHealth(selectedDS.value ? Number(selectedDS.value) : undefined)
    healthData.value = res.data.datasources
    overallHealth.value = res.data.overall_health
    totalDatasources.value = res.data.total_datasources || res.data.datasources.length
    healthDistribution.value = res.data.health_distribution || []
    typeAlerts.value = res.data.type_alerts || []
    expandedDS.value = null
    renderCharts()
  } catch (e: any) { ElMessage.error(e.message) } finally { loading.value = false }
}

watch(selectedDS, fetchData)

onMounted(async () => {
  try { const ds = await getAllDataSources(); allDatasources.value = ds.data } catch { /* ignore */ }
  await fetchData(); await fetchTrend()
})

onUnmounted(() => {
  pieChart?.dispose(); typeBarChart?.dispose(); distChart?.dispose(); trendChart?.dispose(); dsDonutChart?.dispose()
})
</script>

<style scoped>
.h-full { height: 100%; display: flex; flex-direction: column; }
.h-full > :nth-child(2) { flex: 1; min-height: 0; }
:deep(.vue-grid-item) { overflow: hidden; }
:deep(.vue-grid-item .section-card) { margin: 0; }

.stat-card {
  height: 100%;
  background: var(--bg-card);
  border-radius: 10px;
  border-left: 3px solid var(--cyan);
  padding: 10px 12px 12px;
  display: flex;
  flex-direction: column;
  cursor: grab;
  transition: transform 0.15s, box-shadow 0.15s;
}
.stat-card:hover {
  transform: translateY(-1px);
  box-shadow: var(--shadow-card);
}
.stat-title {
  font-size: 11px;
  font-weight: 600;
  color: var(--text-primary);
  padding-bottom: 6px;
  margin-bottom: 4px;
  border-bottom: 1px solid var(--border);
  flex-shrink: 0;
}
.stat-card-body {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 10px;
}
.stat-icon {
  width: 38px;
  height: 38px;
  border-radius: 8px;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}
.stat-value {
  font-size: 24px;
  font-weight: 700;
  line-height: 1;
}
</style>
