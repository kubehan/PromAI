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
      <!-- Global Health Stats -->
      <grid-item :x="item('stats').x" :y="item('stats').y" :w="item('stats').w" :h="item('stats').h" i="stats"
        drag-allow-from=".grid-drag-handle">
        <div class="section-card h-full">
          <div class="section-header grid-drag-handle" style="cursor: grab;">
            <h3><el-icon :size="16" color="#00d4ff"><DataAnalysis /></el-icon> 概览统计</h3>
          </div>
          <div style="display: flex; gap: 12px; padding: 8px 18px 18px;">
            <div class="stat-card" style="flex: 1;">
              <div class="stat-glow" style="background: #00d4ff;"></div>
              <div class="stat-icon-wrap" style="background: rgba(0,212,255,0.08);">
                <el-icon color="#00d4ff" :size="20"><DataAnalysis /></el-icon>
              </div>
              <div class="stat-value" :style="{ color: overallHealth >= 90 ? '#10b981' : overallHealth >= 70 ? '#f59e0b' : '#ef4444', fontSize: 18 }">{{ overallHealth.toFixed(1) }}%</div>
              <div class="stat-label" style="font-size: 11px;">综合健康分</div>
            </div>
            <div class="stat-card" style="flex: 1;">
              <div class="stat-glow" style="background: #10b981;"></div>
              <div class="stat-icon-wrap" style="background: rgba(16,185,129,0.08);">
                <el-icon color="#10b981" :size="20"><Check /></el-icon>
              </div>
              <div class="stat-value" style="color: #10b981; font-size: 18px;">{{ summary.normal_total }}</div>
              <div class="stat-label" style="font-size: 11px;">正常指标</div>
            </div>
            <div class="stat-card" style="flex: 1;">
              <div class="stat-glow" style="background: #f59e0b;"></div>
              <div class="stat-icon-wrap" style="background: rgba(245,158,11,0.08);">
                <el-icon color="#f59e0b" :size="20"><WarningFilled /></el-icon>
              </div>
              <div class="stat-value" style="color: #f59e0b; font-size: 18px;">{{ summary.warning_total }}</div>
              <div class="stat-label" style="font-size: 11px;">告警指标</div>
            </div>
            <div class="stat-card" style="flex: 1;">
              <div class="stat-glow" style="background: #ef4444;"></div>
              <div class="stat-icon-wrap" style="background: rgba(239,68,68,0.08);">
                <el-icon color="#ef4444" :size="20"><CircleCloseFilled /></el-icon>
              </div>
              <div class="stat-value" style="color: #ef4444; font-size: 18px;">{{ summary.critical_total }}</div>
              <div class="stat-label" style="font-size: 11px;">严重告警</div>
            </div>
          </div>
        </div>
      </grid-item>

      <!-- Global Abnormal Metrics Table -->
      <grid-item :x="item('abnormal').x" :y="item('abnormal').y" :w="item('abnormal').w" :h="item('abnormal').h" i="abnormal"
        drag-allow-from=".grid-drag-handle">
        <div class="section-card h-full" :style="{ borderColor: abnormalMetrics.length > 0 ? 'rgba(239,68,68,0.2)' : undefined }">
          <div class="section-header grid-drag-handle" style="cursor: grab;">
            <h3>
              <el-icon :size="16" color="#ef4444"><WarningFilled /></el-icon>
              <span style="color: #ef4444;">异常指标</span>
              <el-tag size="small" style="margin-left: 8px; background: rgba(239,68,68,0.15); color: #ef4444; border: none;" v-if="abnormalMetrics.length > 0">{{ abnormalMetrics.length }} 项</el-tag>
            </h3>
            <div style="display: flex; gap: 8px; align-items: center;" v-if="abnormalMetrics.length > 0">
              <el-select v-model="abnormalStatusFilter" placeholder="状态" size="small" style="width: 100px;" clearable>
                <el-option label="严重" value="critical" />
                <el-option label="告警" value="warning" />
              </el-select>
              <el-select v-model="abnormalTypeFilter" placeholder="类型" size="small" style="width: 140px;" clearable>
                <el-option v-for="t in typeOptions" :key="t" :label="t" :value="t" />
              </el-select>
              <el-input v-model="abnormalSearch" placeholder="搜索指标" size="small" style="width: 180px;" clearable />
              <el-button size="small" text @click="exportCSV" style="color: var(--cyan);"><el-icon><Download /></el-icon> 导出 CSV</el-button>
            </div>
          </div>
          <div v-if="abnormalMetrics.length > 0" style="overflow: auto; flex: 1;">
            <el-table :data="groupedAbnormalMetrics" stripe size="small" max-height="400">
              <el-table-column label="数据源" width="110">
                <template #default="{ row }">
                  <span style="font-weight: 600; color: var(--text-primary); font-size: 12px;">{{ row.datasource_name }}</span>
                </template>
              </el-table-column>
              <el-table-column prop="type_name" label="类型" min-width="120" />
              <el-table-column prop="metric_name" label="指标名称" min-width="140">
                <template #default="{ row }">
                  <span style="font-weight: 600; color: var(--text-primary); font-size: 12px;">{{ row.metric_name }}</span>
                </template>
              </el-table-column>
              <el-table-column label="标签" width="360">
                <template #default="{ row }">
                  <div style="display: flex; flex-wrap: wrap; gap: 2px 8px;">
                    <template v-for="(v, k) in row.labels" :key="k"><span v-if="v && v !== '-'" style="font-size: 12px; color: var(--text-tertiary); word-break: break-all;">
                      <span style="color: var(--text-secondary);">{{ k }}:</span> {{ v }}
                    </span></template>
                  </div>
                </template>
              </el-table-column>
              <el-table-column prop="value" label="当前值" width="90" align="center">
                <template #default="{ row }">
                  <span style="color: var(--text-tertiary);">{{ typeof row.value === 'number' ? row.value.toFixed(2) : row.value }}</span>
                </template>
              </el-table-column>
              <el-table-column label="状态" width="70" align="center">
                <template #default="{ row }">
                  <span :class="['status-badge', row.status]">
                    {{ row.status === 'critical' ? '严重' : '告警' }}
                  </span>
                </template>
              </el-table-column>
              <el-table-column label="阈值" width="100" align="center">
                <template #default="{ row }">
                  <span style="color: var(--text-tertiary); font-size: 12px;">{{ row.threshold > 0 ? (row.threshold_type === 'less' ? '< ' : '> ') + row.threshold : '-' }}</span>
                </template>
              </el-table-column>
              <el-table-column prop="unit" label="单位" width="60" align="center" />
              <el-table-column label="检测时间" width="140" align="center">
                <template #default="{ row }">
                  <span style="color: var(--text-tertiary); font-size: 12px;">{{ row.last_report_at ? new Date(row.last_report_at).toLocaleString() : '-' }}</span>
                </template>
              </el-table-column>
            </el-table>
          </div>
          <div v-else style="padding: 24px; text-align: center; color: var(--text-tertiary);">
            暂无异常指标
          </div>
        </div>
      </grid-item>

      <!-- Pie Chart -->
      <grid-item :x="item('pie').x" :y="item('pie').y" :w="item('pie').w" :h="item('pie').h" i="pie"
        drag-allow-from=".grid-drag-handle">
        <div class="section-card h-full">
          <div class="section-header grid-drag-handle" style="cursor: grab;">
            <h3><el-icon :size="16" color="#00d4ff"><PieChart /></el-icon> 指标状态分布</h3>
          </div>
          <div ref="pieChartRef" style="flex: 1; min-height: 0;"></div>
        </div>
      </grid-item>

      <!-- Abnormal Bar Chart -->
      <grid-item :x="item('abnormal-bar').x" :y="item('abnormal-bar').y" :w="item('abnormal-bar').w" :h="item('abnormal-bar').h" i="abnormal-bar"
        drag-allow-from=".grid-drag-handle">
        <div class="section-card h-full">
          <div class="section-header grid-drag-handle" style="cursor: grab;">
            <h3><el-icon :size="16" color="#f59e0b"><WarningFilled /></el-icon> 数据源异常分布</h3>
          </div>
          <div ref="abnormalBarRef" style="flex: 1; min-height: 0;"></div>
        </div>
      </grid-item>

      <!-- Health Bar Chart -->
      <grid-item :x="item('health-bar').x" :y="item('health-bar').y" :w="item('health-bar').w" :h="item('health-bar').h" i="health-bar"
        drag-allow-from=".grid-drag-handle">
        <div class="section-card h-full">
          <div class="section-header grid-drag-handle" style="cursor: grab;">
            <h3><el-icon :size="16" color="#00d4ff"><Histogram /></el-icon> 数据源健康对比</h3>
          </div>
          <div ref="barChartRef" style="flex: 1; min-height: 0;"></div>
        </div>
      </grid-item>

      <!-- Trend chart -->
      <grid-item :x="item('trend').x" :y="item('trend').y" :w="item('trend').w" :h="item('trend').h" i="trend"
        drag-allow-from=".grid-drag-handle">
        <div class="section-card h-full">
          <div class="section-header grid-drag-handle" style="cursor: grab;">
            <h3><el-icon :size="16" color="#00d4ff"><TrendCharts /></el-icon> 异常趋势（近 {{ trendDays }} 天）</h3>
            <el-radio-group v-model="trendDays" size="small" @change="fetchTrend">
              <el-radio-button :value="7">7天</el-radio-button>
              <el-radio-button :value="14">14天</el-radio-button>
              <el-radio-button :value="30">30天</el-radio-button>
            </el-radio-group>
          </div>
          <div ref="trendChartRef" style="flex: 1; min-height: 0;"></div>
        </div>
      </grid-item>

      <!-- Per-datasource health cards -->
      <grid-item :x="item('ds-cards').x" :y="item('ds-cards').y" :w="item('ds-cards').w" :h="item('ds-cards').h" i="ds-cards"
        drag-allow-from=".grid-drag-handle">
        <div class="section-card h-full">
          <div class="section-header grid-drag-handle" style="cursor: grab;">
            <h3><el-icon :size="16" color="#00d4ff"><Grid /></el-icon> 数据源健康</h3>
          </div>
          <div style="overflow: auto; flex: 1; padding: 8px 18px 18px;">
            <el-row :gutter="16">
              <el-col :span="8" v-for="ds in sortedHealthData" :key="ds.datasource.id" style="margin-bottom: 12px;">
                <div class="section-card" :style="cardBorderStyle(ds)">
                  <div class="section-header">
                    <h3>
                      <span :style="{ width: 10, height: 10, borderRadius: '50%', background: ds.health_score >= 90 ? '#10b981' : ds.health_score >= 70 ? '#f59e0b' : '#ef4444', display: 'inline-block', boxShadow: '0 0 8px ' + (ds.health_score >= 90 ? '#10b981' : ds.health_score >= 70 ? '#f59e0b' : '#ef4444') }"></span>
                      {{ ds.datasource.name }}
                    </h3>
                    <el-tag :style="{ background: ds.health_score >= 90 ? 'rgba(16,185,129,0.15)' : ds.health_score >= 70 ? 'rgba(245,158,11,0.15)' : 'rgba(239,68,68,0.15)', color: ds.health_score >= 90 ? '#10b981' : ds.health_score >= 70 ? '#f59e0b' : '#ef4444', border: 'none' }">
                      {{ ds.health_score.toFixed(1) }}%
                    </el-tag>
                  </div>
                  <div style="padding: 14px 18px;">
                    <div style="display: flex; gap: 16px; margin-bottom: 10px;">
                      <div><span style="color: var(--text-tertiary); font-size: 11px;">指标</span><br><span style="font-weight: 700; font-size: 16px;">{{ ds.total_metrics }}</span></div>
                      <div><span style="color: var(--text-tertiary); font-size: 11px;">告警</span><br><span :style="{ fontWeight: 700, fontSize: 16, color: ds.alerts > 0 ? '#f59e0b' : '#10b981' }">{{ ds.warning_count }}</span></div>
                      <div><span style="color: var(--text-tertiary); font-size: 11px;">严重</span><br><span :style="{ fontWeight: 700, fontSize: 16, color: ds.critical_count > 0 ? '#ef4444' : 'var(--text-tertiary)' }">{{ ds.critical_count }}</span></div>
                    </div>
                    <div style="height: 4px; background: rgba(255,255,255,0.06); border-radius: 4px; overflow: hidden; margin-bottom: 10px;">
                      <div :style="{ height: '100%', width: ds.health_score + '%', background: 'linear-gradient(90deg, #10b981, #00d4ff)', borderRadius: 4, transition: 'width 0.8s ease' }"></div>
                    </div>
                    <div v-if="dsTopAbnormal(ds).length > 0" style="margin-bottom: 8px;">
                      <div v-for="m in dsTopAbnormal(ds)" :key="m.metric_name + m.labels?.instance"
                        style="display: flex; align-items: center; gap: 6px; padding: 3px 0; font-size: 12px; line-height: 1.4;">
                        <span :style="{ width: 6, height: 6, borderRadius: '50%', flexShrink: 0, background: m.status === 'critical' ? '#ef4444' : '#f59e0b', boxShadow: '0 0 4px ' + (m.status === 'critical' ? '#ef4444' : '#f59e0b') }"></span>
                        <span style="color: var(--text-primary); overflow: hidden; text-overflow: ellipsis; word-break: break-all; max-width: 180px;" :title="m.metric_name">
                          {{ m.metric_name }}
                        </span>
                        <span style="color: var(--text-tertiary); flex-shrink: 0;">{{ typeof m.value === 'number' ? m.value.toFixed(1) : m.value }}</span>
                      </div>
                    </div>
                    <div style="display: flex; gap: 6px; margin-top: 4px;">
                      <el-button size="small" text @click="switchDS(ds.datasource.id)" style="color: var(--cyan); font-size: 12px;">查看详情</el-button>
                      <el-button size="small" text @click.stop="viewReport(ds)" style="color: var(--cyan); font-size: 12px;">查看报告 →</el-button>
                    </div>
                  </div>
                </div>
              </el-col>
            </el-row>
          </div>
        </div>
      </grid-item>

      <!-- Metrics detail table -->
      <grid-item :x="item('detail').x" :y="item('detail').y" :w="item('detail').w" :h="item('detail').h" i="detail"
        drag-allow-from=".grid-drag-handle">
        <div class="section-card h-full">
          <div class="section-header grid-drag-handle" style="cursor: grab;">
            <h3><el-icon :size="16" color="#00d4ff"><List /></el-icon> <span v-if="expandedDS">{{ expandedDS.datasource.name }} - </span>指标明细</h3>
            <div style="display: flex; gap: 8px;" v-if="expandedDS">
              <el-button size="small" text @click="exportDetailCSV" style="color: var(--cyan);"><el-icon><Download /></el-icon> 导出 CSV</el-button>
              <el-button size="small" text @click="expandedDS = null" style="color: var(--text-tertiary);">收起</el-button>
            </div>
          </div>
          <div v-if="expandedDS" style="overflow: auto; flex: 1;">
            <div style="padding: 8px 18px 0;">
              <div style="display: flex; flex-wrap: wrap; gap: 6px; margin-bottom: 12px;">
                <el-tag :type="typeFilter === '' ? '' : 'info'" :effect="typeFilter === '' ? 'dark' : 'plain'" style="cursor: pointer;" @click="typeFilter = ''">全部</el-tag>
                <el-tag v-for="t in typeOptions" :key="t" :type="typeFilter === t ? '' : 'info'" :effect="typeFilter === t ? 'dark' : 'plain'" style="cursor: pointer;" @click="typeFilter = t">{{ t }}</el-tag>
                <el-input v-model="metricSearch" placeholder="搜索指标名称" size="small" style="width: 200px; margin-left: auto;" clearable />
              </div>
              <div style="display: flex; flex-wrap: wrap; gap: 8px; margin-bottom: 16px;" v-if="expandedDS.type_summaries">
                <div v-for="s in expandedDS.type_summaries" :key="s.type_name"
                  style="display: flex; align-items: center; gap: 8px; padding: 8px 14px; border-radius: 8px; background: rgba(255,255,255,0.04); border: 1px solid rgba(255,255,255,0.06); font-size: 13px;">
                  <span :class="['status-dot', s.alerts > 0 ? (s.critical_count > 0 ? 'critical' : 'warning') : 'normal']"></span>
                  <span style="color: var(--text-primary); font-weight: 600;">{{ s.type_name }}</span>
                  <span style="color: var(--text-tertiary);">总{{ s.total_metrics }}</span>
                  <span v-if="s.critical_count > 0" style="color: #ef4444;">严重{{ s.critical_count }}</span>
                  <span v-if="s.warning_count > 0" style="color: #f59e0b;">告警{{ s.warning_count }}</span>
                  <span v-if="s.normal_count > 0" style="color: #10b981;">正常{{ s.normal_count }}</span>
                </div>
              </div>
            </div>
            <div style="padding: 0 18px 18px;">
              <el-table :data="filteredMetrics" stripe size="small" max-height="400">
                <el-table-column prop="type_name" label="类型" min-width="120" />
                <el-table-column prop="metric_name" label="指标名称" min-width="140">
                  <template #default="{ row }">
                    <span style="font-weight: 600; color: var(--text-primary); font-size: 12px;">{{ row.metric_name }}</span>
                  </template>
                </el-table-column>
                <el-table-column label="标签" width="360">
                  <template #default="{ row }">
                    <div style="display: flex; flex-wrap: wrap; gap: 2px 8px;">
                      <template v-for="(v, k) in row.labels" :key="k"><span v-if="v && v !== '-'" style="font-size: 12px; color: var(--text-tertiary); word-break: break-all;">
                        <span style="color: var(--text-secondary);">{{ k }}:</span> {{ v }}
                      </span></template>
                    </div>
                  </template>
                </el-table-column>
                <el-table-column prop="value" label="当前值" width="90" align="center">
                  <template #default="{ row }">
                    <span style="color: var(--text-tertiary);">{{ typeof row.value === 'number' ? row.value.toFixed(2) : row.value }}</span>
                  </template>
                </el-table-column>
                <el-table-column label="状态" width="70" align="center">
                  <template #default="{ row }">
                    <span :class="['status-badge', row.status]">
                      {{ row.status === 'normal' || row.status === 'success' ? '正常' : row.status === 'critical' ? '严重' : '告警' }}
                    </span>
                  </template>
                </el-table-column>
                <el-table-column label="阈值" width="100" align="center">
                  <template #default="{ row }">{{ row.threshold > 0 ? row.threshold_type === 'less' ? '< ' : '> ' : '' }}{{ row.threshold > 0 ? row.threshold : '-' }}</template>
                </el-table-column>
                <el-table-column prop="unit" label="单位" width="60" align="center" />
              </el-table>
            </div>
          </div>
          <div v-else style="padding: 24px; text-align: center; color: var(--text-tertiary);">
            请点击数据源卡片上的「查看详情」查看指标明细
          </div>
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
import { getDataSources, getDashboardHealth, getDashboardHealthTrend } from '../api'
import type { DataSource } from '../types'

const loading = ref(false)
const allDatasources = ref<DataSource[]>([])
const selectedDS = ref<number | string>('')
const healthData = ref<any[]>([])
const overallHealth = ref(100)
const expandedDS = ref<any>(null)
const typeFilter = ref('')
const metricSearch = ref('')
const abnormalStatusFilter = ref('')
const abnormalTypeFilter = ref('')
const abnormalSearch = ref('')
const trendData = ref<any[]>([])
const trendDays = ref(14)

const defaultLayout = [
  { x: 0, y: 0, w: 12, h: 2, i: 'stats' },
  { x: 0, y: 2, w: 12, h: 10, i: 'abnormal' },
  { x: 0, y: 12, w: 4, h: 8, i: 'pie' },
  { x: 4, y: 12, w: 4, h: 8, i: 'abnormal-bar' },
  { x: 8, y: 12, w: 4, h: 8, i: 'health-bar' },
  { x: 0, y: 20, w: 12, h: 8, i: 'trend' },
  { x: 0, y: 28, w: 12, h: 12, i: 'ds-cards' },
  { x: 0, y: 40, w: 12, h: 10, i: 'detail' },
]
const savedLayout = localStorage.getItem('bi_dashboard_layout')
const layout = ref(savedLayout ? JSON.parse(savedLayout) : defaultLayout)

watch(layout, (val) => {
  localStorage.setItem('bi_dashboard_layout', JSON.stringify(val))
}, { deep: true })
const pieChartRef = ref<HTMLElement>()
const barChartRef = ref<HTMLElement>()
const abnormalBarRef = ref<HTMLElement>()
const trendChartRef = ref<HTMLElement>()
let pieChart: echarts.ECharts | null = null
let barChart: echarts.ECharts | null = null
let abnormalBarChart: echarts.ECharts | null = null
let trendChart: echarts.ECharts | null = null

const summary = computed(() => {
  const s = { normal_total: 0, warning_total: 0, critical_total: 0 }
  healthData.value.forEach((d: any) => {
    s.normal_total += d.normal_count
    s.warning_total += d.warning_count
    s.critical_total += d.critical_count
  })
  return s
})

const sortedHealthData = computed(() => {
  return [...healthData.value].sort((a, b) => a.health_score - b.health_score)
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
  if (abnormalSearch.value) {
    const q = abnormalSearch.value.toLowerCase()
    list = list.filter(m => m.metric_name.toLowerCase().includes(q) || m.type_name.toLowerCase().includes(q))
  }
  return list
})

const groupedAbnormalMetrics = computed(() => {
  const list = [...filteredAbnormalMetrics.value]
  list.sort((a, b) => {
    if (a.type_name !== b.type_name) return a.type_name.localeCompare(b.type_name)
    return a.status === 'critical' && b.status !== 'critical' ? -1 : b.status === 'critical' && a.status !== 'critical' ? 1 : 0
  })
  return list
})

const filteredMetrics = computed(() => {
  if (!expandedDS.value?.metrics) return []
  let list = expandedDS.value.metrics
  if (typeFilter.value) list = list.filter((m: any) => m.type_name === typeFilter.value)
  if (metricSearch.value) {
    const q = metricSearch.value.toLowerCase()
    list = list.filter((m: any) => m.metric_name.toLowerCase().includes(q) || m.type_name.toLowerCase().includes(q))
  }
  return [...list].sort((a: any, b: any) => a.type_name?.localeCompare(b.type_name) || 0)
})

function cardBorderStyle(ds: any) {
  if (ds.critical_count > 0) return { borderColor: 'rgba(239,68,68,0.3)' }
  if (ds.warning_count > 0) return { borderColor: 'rgba(245,158,11,0.3)' }
  return {}
}

function item(id: string) {
  const found = layout.value.find((i: any) => i.i === id)
  return found || { x: 0, y: 0, w: 12, h: 6 }
}

function layoutUpdated(newLayout: any[]) {
  layout.value = newLayout
}

function dsTopAbnormal(ds: any): any[] {
  if (!ds.metrics) return []
  const abnormal = ds.metrics.filter((m: any) => m.status === 'critical' || m.status === 'warning')
  abnormal.sort((a: any, b: any) => a.status === 'critical' && b.status !== 'critical' ? -1 : b.status === 'critical' && a.status !== 'critical' ? 1 : 0)
  return abnormal.slice(0, 3)
}

function switchDS(id: number) {
  const d = healthData.value.find((h: any) => h.datasource.id === id)
  if (d) expandedDS.value = d
  typeFilter.value = ''
  metricSearch.value = ''
}

function exportDetailCSV() {
  const data = filteredMetrics.value
  if (!data.length) {
    ElMessage.info('没有可导出的数据')
    return
  }

  const headers = ['类型', '指标名称', '标签', '当前值', '状态', '阈值', '单位']

  const rows = data.map((row: any) => {
    const labelStr = row.labels
      ? Object.entries(row.labels)
          .filter(([_, v]) => v && v !== '-')
          .map(([k, v]) => `${k}:${v}`)
          .join('; ')
      : ''
    return [
      row.type_name,
      row.metric_name,
      labelStr,
      typeof row.value === 'number' ? row.value.toFixed(2) : row.value,
      row.status === 'critical' ? '严重' : row.status === 'warning' ? '告警' : '正常',
      row.threshold > 0 ? (row.threshold_type === 'less' ? '< ' : '> ') + row.threshold : '-',
      row.unit || '',
    ]
  })

  const csvContent = [headers, ...rows]
    .map((row: string[]) => row.map((cell: string) => `"${String(cell).replace(/"/g, '""')}"`).join(','))
    .join('\n')

  const blob = new Blob(['\uFEFF' + csvContent], { type: 'text/csv;charset=utf-8;' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = `${expandedDS.value?.datasource?.name || '指标明细'}_${new Date().toISOString().slice(0, 10)}.csv`
  document.body.appendChild(a)
  a.click()
  document.body.removeChild(a)
  URL.revokeObjectURL(url)
}

function exportCSV() {
  const data = groupedAbnormalMetrics.value
  if (!data.length) {
    ElMessage.info('没有可导出的数据')
    return
  }

  const headers = ['数据源', '类型', '指标名称', '标签', '当前值', '状态', '阈值', '单位', '检测时间']

  const rows = data.map((row: any) => {
    const labelStr = row.labels
      ? Object.entries(row.labels)
          .filter(([_, v]) => v && v !== '-')
          .map(([k, v]) => `${k}:${v}`)
          .join('; ')
      : ''
    return [
      row.datasource_name,
      row.type_name,
      row.metric_name,
      labelStr,
      typeof row.value === 'number' ? row.value.toFixed(2) : row.value,
      row.status === 'critical' ? '严重' : '告警',
      row.threshold > 0 ? (row.threshold_type === 'less' ? '< ' : '> ') + row.threshold : '-',
      row.unit || '',
      row.last_report_at ? new Date(row.last_report_at).toLocaleString() : '',
    ]
  })

  const csvContent = [headers, ...rows]
    .map(row => row.map(cell => `"${String(cell).replace(/"/g, '""')}"`).join(','))
    .join('\n')

  const blob = new Blob(['\uFEFF' + csvContent], { type: 'text/csv;charset=utf-8;' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = `异常指标_${new Date().toISOString().slice(0, 10)}.csv`
  document.body.appendChild(a)
  a.click()
  document.body.removeChild(a)
  URL.revokeObjectURL(url)
}

function viewReport(ds: any) {
  if (ds.last_report_url) {
    window.open(ds.last_report_url, '_blank')
  } else {
    ElMessage.info('暂无报告')
  }
}

function renderCharts(data: any) {
  nextTick(() => {
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

    if (abnormalBarRef.value) {
      if (!abnormalBarChart) abnormalBarChart = echarts.init(abnormalBarRef.value)
      const names = data.map((d: any) => d.datasource.name)
      abnormalBarChart.setOption({
        backgroundColor: 'transparent',
        tooltip: { trigger: 'axis', textStyle: { color: '#f1f5f9' }, backgroundColor: 'rgba(17,24,39,0.95)', borderColor: 'rgba(56,189,248,0.2)' },
        legend: { data: ['严重', '告警'], textStyle: { color: '#94a3b8', fontSize: 11 }, top: 0, right: 0 },
        grid: { left: '3%', right: '4%', bottom: '8%', top: '18%', containLabel: true },
        xAxis: {
          type: 'category', data: names, axisLabel: { color: '#94a3b8', fontSize: 10 },
          axisLine: { lineStyle: { color: 'rgba(56,189,248,0.1)' } },
          axisTick: { show: false },
        },
        yAxis: {
          type: 'value',
          splitLine: { lineStyle: { color: 'rgba(56,189,248,0.06)', type: 'dashed' } },
          axisLabel: { color: '#94a3b8', fontSize: 10 },
        },
        series: [
          {
            name: '严重', type: 'bar', stack: 'total', barWidth: 24,
            itemStyle: { color: '#ef4444', borderRadius: 0 },
            data: data.map((d: any) => d.critical_count),
          },
          {
            name: '告警', type: 'bar', stack: 'total', barWidth: 24,
            itemStyle: { color: '#f59e0b', borderRadius: [4, 4, 0, 0] },
            data: data.map((d: any) => d.warning_count),
          },
        ],
      })
    }

    if (barChartRef.value) {
      if (!barChart) barChart = echarts.init(barChartRef.value)
      const names = data.map((d: any) => d.datasource.name)
      const scores = data.map((d: any) => parseFloat(d.health_score.toFixed(1)))
      barChart.setOption({
        backgroundColor: 'transparent',
        tooltip: { trigger: 'axis', textStyle: { color: '#f1f5f9' }, backgroundColor: 'rgba(17,24,39,0.95)', borderColor: 'rgba(56,189,248,0.2)' },
        grid: { left: '3%', right: '4%', bottom: '8%', top: '5%', containLabel: true },
        xAxis: {
          type: 'category', data: names, axisLabel: { color: '#94a3b8', fontSize: 10 },
          axisLine: { lineStyle: { color: 'rgba(56,189,248,0.1)' } },
          axisTick: { show: false },
        },
        yAxis: {
          type: 'value', min: 0, max: 100,
          splitLine: { lineStyle: { color: 'rgba(56,189,248,0.06)', type: 'dashed' } },
          axisLabel: { color: '#94a3b8', fontSize: 10, formatter: '{value}%' },
        },
        series: [{
          type: 'bar',
          barWidth: 24,
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
            show: true, position: 'top', color: '#94a3b8', fontSize: 10, fontWeight: 600,
            formatter: '{c}%',
          },
        }],
      })
    }
  })
}

function renderTrendChart(trend: any[]) {
  nextTick(() => {
    if (!trendChartRef.value) return
    if (!trendChart) trendChart = echarts.init(trendChartRef.value)
    const dates = trend.map((d: any) => d.date)
    const criticals = trend.map((d: any) => d.critical)
    const warnings = trend.map((d: any) => d.warning)
    trendChart.setOption({
      backgroundColor: 'transparent',
      tooltip: {
        trigger: 'axis',
        textStyle: { color: '#f1f5f9' },
        backgroundColor: 'rgba(17,24,39,0.95)',
        borderColor: 'rgba(56,189,248,0.2)',
      },
      legend: {
        data: ['严重', '告警'],
        textStyle: { color: '#94a3b8', fontSize: 12 },
        top: 0, right: 0,
      },
      grid: { left: '3%', right: '4%', bottom: '8%', top: '18%', containLabel: true },
      xAxis: {
        type: 'category',
        data: dates,
        boundaryGap: false,
        axisLabel: { color: '#94a3b8', fontSize: 11 },
        axisLine: { lineStyle: { color: 'rgba(56,189,248,0.1)' } },
        axisTick: { show: false },
      },
      yAxis: {
        type: 'value',
        min: 0,
        splitLine: { lineStyle: { color: 'rgba(56,189,248,0.06)', type: 'dashed' } },
        axisLabel: { color: '#94a3b8', fontSize: 11 },
      },
      series: [
        {
          name: '严重',
          type: 'line',
          smooth: true,
          symbol: 'circle',
          symbolSize: 6,
          lineStyle: { color: '#ef4444', width: 2 },
          itemStyle: { color: '#ef4444' },
          areaStyle: { color: 'rgba(239,68,68,0.1)' },
          data: criticals,
        },
        {
          name: '告警',
          type: 'line',
          smooth: true,
          symbol: 'circle',
          symbolSize: 6,
          lineStyle: { color: '#f59e0b', width: 2 },
          itemStyle: { color: '#f59e0b' },
          areaStyle: { color: 'rgba(245,158,11,0.1)' },
          data: warnings,
        },
      ],
    })
  })
}

async function fetchTrend() {
  try {
    const res = await getDashboardHealthTrend(trendDays.value)
    trendData.value = res.data.trend
    renderTrendChart(res.data.trend)
  } catch { /* ignore */ }
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
    allDatasources.value = ds.data.items
  } catch { /* ignore */ }
  await fetchData()
  await fetchTrend()
})

onUnmounted(() => {
  pieChart?.dispose()
  barChart?.dispose()
  abnormalBarChart?.dispose()
  trendChart?.dispose()
})
</script>

<style scoped>
.h-full {
  height: 100%;
  display: flex;
  flex-direction: column;
}
.h-full > :nth-child(2) {
  flex: 1;
  min-height: 0;
}
:deep(.vue-grid-item) {
  overflow: hidden;
}
:deep(.vue-grid-item .section-card) {
  margin: 0;
}
</style>

