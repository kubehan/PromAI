<template>
  <div class="page-container">
    <div class="page-header">
      <h2><el-icon><Monitor /></el-icon> 触发巡检</h2>
      <p>选择数据源和巡检指标，手动触发巡检任务</p>
    </div>

    <el-row :gutter="24">
      <el-col :span="16">
        <div class="form-section" style="margin-bottom: 0;">
          <h3><el-icon :size="16" color="#00d4ff"><Setting /></el-icon> 巡检配置</h3>
          <el-form :model="form" label-width="120px">
            <el-form-item label="选择数据源">
              <el-select v-model="form.datasource_id" placeholder="使用默认数据源" clearable filterable style="width: 100%;" @change="onDSChange">
                <el-option v-for="ds in datasources" :key="ds.id" :label="ds.name" :value="ds.id" />
              </el-select>
            </el-form-item>
            <el-form-item label="自定义 URL">
              <el-input v-model="form.datasource_url" placeholder="留空则使用默认或选择的数据源" :disabled="!!form.datasource_id" />
            </el-form-item>

            <!-- Metric selector -->
            <el-form-item label="巡检指标">
              <div style="width: 100%;">
                <div style="display: flex; justify-content: space-between; margin-bottom: 8px;">
                  <span style="color: var(--text-tertiary); font-size: 13px;">选择要巡检的指标（不选则全部）</span>
                  <div>
                    <el-button size="small" text @click="selectAllMetrics" style="color: var(--cyan);">全选</el-button>
                    <el-button size="small" text @click="deselectAllMetrics" style="color: var(--text-tertiary);">清空</el-button>
                  </div>
                </div>
                <div v-loading="metricsLoading" style="border: 1px solid var(--border); border-radius: 10px; padding: 12px; max-height: 300px; overflow-y: auto;">
                  <template v-for="mt in metricTypes" :key="mt.id">
                    <div style="margin-bottom: 8px;">
                      <el-checkbox
                        v-model="selectedTypeIds"
                        :label="mt.id"
                        :indeterminate="isIndeterminate(mt)"
                        @change="(val: any) => toggleType(mt, val)"
                        style="font-weight: 600; color: var(--text-primary);"
                      >
                        {{ mt.type_name }} ({{ mt.configs?.length || 0 }})
                      </el-checkbox>
                      <div style="padding-left: 28px; margin-top: 4px;">
                        <el-checkbox-group v-model="selectedConfigIds">
                          <el-checkbox
                            v-for="cfg in mt.configs || []"
                            :key="cfg.id"
                            :label="cfg.id"
                            style="margin-right: 12px; margin-bottom: 4px;"
                          >
                            <span style="font-size: 13px;">{{ cfg.name }}</span>
                          </el-checkbox>
                        </el-checkbox-group>
                      </div>
                    </div>
                  </template>
                  <el-empty v-if="!metricsLoading && metricTypes.length === 0" description="暂无指标配置" :image-size="40" />
                </div>
              </div>
            </el-form-item>

            <el-form-item label="企业微信 Key">
              <el-input v-model="form.wechat_bot_key" placeholder="可选：报告生成后推送到企业微信" />
            </el-form-item>
            <el-form-item label="通知用户">
              <el-input v-model="form.touser" placeholder="@all 或指定用户 ID" />
            </el-form-item>
            <el-form-item>
              <el-button type="primary" size="large" :loading="inspecting" @click="handleInspect" :disabled="inspecting" style="height: 48px; padding: 0 36px; font-size: 15px;">
                <el-icon><Lightning /></el-icon> 开始巡检
              </el-button>
            </el-form-item>
          </el-form>
        </div>
      </el-col>
      <el-col :span="8">
        <div class="section-card" style="margin-bottom: 16px;">
          <div class="section-header">
            <h3><el-icon :size="16" color="#00d4ff"><InfoFilled /></el-icon> 使用说明</h3>
          </div>
          <div style="padding: 20px; font-size: 13px; color: var(--text-tertiary); line-height: 2.4;">
            <div>• 选择已配置的数据源</div>
            <div>• 选择要巡检的指标（可选）</div>
            <div>• 可选配置企业微信推送</div>
            <div>• 报告将自动保存到历史</div>
          </div>
        </div>
        <div v-if="lastResult" class="section-card" :style="{ borderColor: lastResult.success ? 'rgba(16,185,129,0.3)' : 'rgba(239,68,68,0.3)' }">
          <div class="section-header">
            <h3><el-icon :size="16" :color="lastResult.success ? '#10b981' : '#ef4444'"><InfoFilled /></el-icon> 上次结果</h3>
          </div>
          <div style="padding: 20px;">
            <div :style="{ color: lastResult.success ? 'var(--emerald)' : 'var(--red)', fontWeight: 600, fontSize: 14, marginBottom: 12, display: 'flex', alignItems: 'center', gap: 8 }">
              <span :style="{ width: 8, height: 8, borderRadius: '50%', background: lastResult.success ? 'var(--emerald)' : 'var(--red)', boxShadow: '0 0 8px ' + (lastResult.success ? 'var(--emerald)' : 'var(--red)') }"></span>
              {{ lastResult.success ? '巡检完成' : '巡检失败' }}
            </div>
            <div v-if="lastResult.report" style="font-size: 13px; color: var(--text-tertiary); margin-bottom: 8px;">
              报告路径：<code style="font-size: 11px; color: var(--text-secondary);">{{ lastResult.report }}</code>
            </div>
            <div v-if="lastResult.url" style="margin-top: 8px;">
              <el-link type="primary" :href="lastResult.url" target="_blank" style="color: var(--cyan) !important;">打开报告 →</el-link>
            </div>
            <div v-if="lastResult.error" style="font-size: 13px; color: var(--red); margin-top: 8px;">{{ lastResult.error }}</div>
          </div>
        </div>
      </el-col>
    </el-row>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, watch } from 'vue'
import { useRoute } from 'vue-router'
import { ElMessage } from 'element-plus'
import { getAllDataSources, getMetricTypes, triggerInspect, getInspectTask } from '../api'
import type { DataSource, MetricType } from '../types'

const route = useRoute()
const inspecting = ref(false)
const metricsLoading = ref(false)
const datasources = ref<DataSource[]>([])
const metricTypes = ref<MetricType[]>([])
const selectedTypeIds = ref<number[]>([])
const selectedConfigIds = ref<number[]>([])
const form = ref({ datasource_id: undefined as number | undefined, datasource_url: '', wechat_bot_key: '', touser: '' })
const lastResult = ref<{ success: boolean; report?: string; url?: string; error?: string } | null>(null)

function isIndeterminate(mt: MetricType) {
  if (!mt.configs?.length) return false
  const cfgIds = mt.configs.map(c => c.id!)
  const selected = cfgIds.filter(id => selectedConfigIds.value.includes(id))
  return selected.length > 0 && selected.length < cfgIds.length
}

function toggleType(mt: MetricType, checked: boolean) {
  const cfgIds = mt.configs?.map(c => c.id!) || []
  if (checked) {
    cfgIds.forEach(id => { if (!selectedConfigIds.value.includes(id)) selectedConfigIds.value.push(id) })
  } else {
    selectedConfigIds.value = selectedConfigIds.value.filter(id => !cfgIds.includes(id))
  }
}

function selectAllMetrics() {
  selectedConfigIds.value = []
  metricTypes.value.forEach(mt => {
    mt.configs?.forEach(c => { if (c.id) selectedConfigIds.value.push(c.id) })
  })
}

function deselectAllMetrics() {
  selectedConfigIds.value = []
}

function onDSChange(dsId: number | undefined) {
  if (dsId) {
    loadMetricsForDS(dsId)
  }
}

async function loadMetricsForDS(dsId: number) {
  metricsLoading.value = true
  try {
    const res = await getMetricTypes()
    metricTypes.value = res.data.map(mt => ({
      ...mt,
      configs: (mt.configs || []).filter(c => c.datasource_id === null || c.datasource_id === dsId),
    })).filter(mt => mt.configs && mt.configs.length > 0)
    selectAllMetrics()
  } catch { /* ignore */ }
  finally { metricsLoading.value = false }
}

async function handleInspect() {
  inspecting.value = true; lastResult.value = null
  try {
    const res = await triggerInspect({
      datasource_id: form.value.datasource_id,
      datasource_url: form.value.datasource_url || undefined,
      wechat_bot_key: form.value.wechat_bot_key || undefined,
      touser: form.value.touser || undefined,
      metric_config_ids: selectedConfigIds.value.length > 0 ? selectedConfigIds.value : undefined,
    })
    const taskId = res.data.task_id
    if (!taskId) { ElMessage.error('巡检启动失败'); inspecting.value = false; return }

    ElMessage.info('巡检已开始，正在等待结果...')
    for (let i = 0; i < 120; i++) {
      await new Promise(r => setTimeout(r, 2000))
      const taskRes = await getInspectTask(taskId)
      const task = taskRes.data
      if (task.status === 'completed') {
        lastResult.value = { success: true, report: '', url: task.report_url || '' }
        ElMessage.success('巡检完成，报告已生成')
        inspecting.value = false
        return
      }
      if (task.status === 'failed') {
        lastResult.value = { success: false, error: task.error || '巡检失败' }
        ElMessage.error(task.error || '巡检失败')
        inspecting.value = false
        return
      }
    }
    lastResult.value = { success: false, error: '巡检超时' }
    ElMessage.error('巡检超时')
  } catch (e: any) {
    lastResult.value = { success: false, error: e.message }
    ElMessage.error(e.message)
  } finally { inspecting.value = false }
}

onMounted(async () => {
  try {
    const dsRes = await getAllDataSources()
    datasources.value = dsRes.data
    // 从路由参数预填数据源
    if (route.query.datasource_id) {
      const dsId = Number(route.query.datasource_id)
      form.value.datasource_id = dsId
      await loadMetricsForDS(dsId)
    } else {
      // 加载全部指标但仅显示全局的
      const res = await getMetricTypes()
      metricTypes.value = res.data.map(mt => ({
        ...mt,
        configs: (mt.configs || []).filter(c => c.datasource_id === null),
      })).filter(mt => mt.configs && mt.configs.length > 0)
      selectAllMetrics()
    }
  } catch { /* ignore */ }
})
</script>
