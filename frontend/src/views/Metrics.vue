<template>
  <div class="page-container">
    <div class="page-header">
      <h2><el-icon><TrendCharts /></el-icon> 指标配置</h2>
      <p>管理 PromQL 指标查询和告警阈值，可全局配置或按数据源自定义</p>
    </div>

    <div class="section-card">
      <div class="section-header">
        <h3><el-icon :size="16" color="#00d4ff"><List /></el-icon> 指标列表</h3>
        <div class="action-bar">
          <el-select v-model="filterDS" placeholder="全部数据源" clearable filterable style="width: 160px;" @change="fetchData">
            <el-option label="全局指标" :value="0" />
            <el-option v-for="ds in datasources" :key="ds.id" :label="ds.name" :value="ds.id" />
          </el-select>
          <el-button plain @click="fetchData"><el-icon><Refresh /></el-icon></el-button>
          <el-button plain @click="typeDialog = true"><el-icon><Collection /></el-icon> 类型管理</el-button>
          <el-button type="primary" @click="openCreateConfig">
            <el-icon><Plus /></el-icon> 新增指标
          </el-button>
        </div>
      </div>
      <el-table :data="flatMetrics" v-loading="loading" stripe size="default">
        <el-table-column type="index" label="#" width="50" />
        <el-table-column label="类型" width="200">
          <template #default="{ row }">
            <el-tag size="small" style="background: rgba(99,102,241,0.12); color: #818cf8; border: none;">
              {{ row.type_name }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="name" label="指标名称" min-width="160">
          <template #default="{ row }">
            <span style="font-weight: 600; color: var(--text-primary);">{{ row.name }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="query" label="PromQL" min-width="280">
          <template #default="{ row }">
            <code style="font-size: 11px; background: rgba(0,0,0,0.3); padding: 2px 8px; border-radius: 6px; color: var(--text-tertiary); display: block; max-width: 400px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap;">{{ row.query }}</code>
          </template>
        </el-table-column>
        <el-table-column label="阈值" width="140">
          <template #default="{ row }">
            <span v-if="row.threshold" style="color: var(--text-secondary); font-size: 13px;">
              {{ thresholdOpLabel(row.threshold_type) }} {{ row.threshold }}{{ row.unit }}
            </span>
            <span v-else style="color: var(--text-tertiary);">-</span>
          </template>
        </el-table-column>
        <el-table-column label="级别" width="90" align="center">
          <template #default="{ row }">
            <el-tag v-if="row.threshold_status" size="small" :style="statusStyle(row.threshold_status)" effect="dark">
              {{ statusLabel(row.threshold_status) }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="标签数" width="80" align="center">
          <template #default="{ row }">
            <span style="color: var(--text-tertiary);">{{ row.labels_json ? Object.keys(JSON.parse(row.labels_json)).length : 0 }}</span>
          </template>
        </el-table-column>
        <el-table-column label="数据源" width="140">
          <template #default="{ row }">
            <span v-if="row.datasource_id" style="color: var(--text-secondary); font-size: 13px;">{{ dsName(row.datasource_id) }}</span>
            <el-tag v-else size="small" style="background: rgba(0,212,255,0.1); color: var(--cyan); border: none;">全局</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="140" fixed="right">
          <template #default="{ row }">
            <el-button size="small" text @click="openEditConfig(row)" style="color: var(--cyan);">编辑</el-button>
            <el-button size="small" text @click="handleDeleteConfig(row)" style="color: var(--red);">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
      <el-empty v-if="!loading && flatMetrics.length === 0" description="暂无指标配置" :image-size="60" />
    </div>

    <el-dialog v-model="configDialog" :title="editingConfigId ? '编辑指标' : '新增指标'" width="720" :close-on-click-modal="false" top="3vh">
      <el-form ref="configFormRef" :model="configForm" :rules="configRules" label-width="110px">
        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item label="指标类型" prop="metric_type_id">
              <el-select v-model="configForm.metric_type_id" style="width: 100%;" :disabled="!!editingConfigId" filterable>
                <el-option v-for="mt in metricTypes" :key="mt.id" :label="mt.type_name" :value="mt.id" />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="数据源">
              <el-select v-model="configForm.datasource_id" placeholder="全局指标" clearable filterable style="width: 100%;">
                <el-option v-for="ds in datasources" :key="ds.id" :label="ds.name" :value="ds.id" />
              </el-select>
            </el-form-item>
          </el-col>
        </el-row>
        <el-form-item label="指标名称" prop="name">
          <el-input v-model="configForm.name" placeholder="CPU 使用率" />
        </el-form-item>
        <el-form-item label="PromQL" prop="query">
          <el-input v-model="configForm.query" type="textarea" :rows="2" placeholder="avg(rate(node_cpu_seconds_total[5m])) * 100" />
        </el-form-item>
        <div style="display: flex; justify-content: flex-end; margin-top: -16px; margin-bottom: 12px;">
          <el-button size="small" :loading="validating" @click="handleValidate" style="color: var(--cyan);">
            <el-icon><Connection /></el-icon> 验证语法
          </el-button>
        </div>

        <div v-if="validationResult" :class="['validation-panel', validationResult.valid ? 'valid' : 'invalid']">
          <div v-if="validationResult.valid">
            <div style="display: flex; gap: 16px; margin-bottom: 8px; font-size: 13px;">
              <span style="color: var(--emerald);">语法正确</span>
              <span style="color: var(--text-tertiary);">类型: {{ validationResult.type }}</span>
              <span v-if="validationResult.count !== undefined" style="color: var(--text-tertiary);">样本数: {{ validationResult.count }}</span>
              <span v-if="validationResult.value !== undefined" style="color: var(--text-tertiary);">值: {{ validationResult.value }}</span>
            </div>
            <div v-if="validationResult.labels && validationResult.labels.length" style="margin-bottom: 6px;">
              <span style="font-size: 12px; color: var(--text-tertiary);">返回标签: </span>
              <el-tag v-for="l in validationResult.labels" :key="l" size="small" style="background: rgba(99,102,241,0.1); color: #818cf8; border: none; margin-right: 4px; margin-bottom: 2px;">{{ l }}</el-tag>
            </div>
            <div v-if="validationResult.samples && validationResult.samples.length" style="max-height: 120px; overflow-y: auto;">
              <div v-for="(s, i) in validationResult.samples" :key="i" style="font-size: 12px; padding: 2px 0; border-bottom: 1px solid var(--border); display: flex; gap: 8px;">
                <span style="color: var(--text-tertiary); min-width: 50px;">#{{ i+1 }}</span>
                <span style="color: var(--text-secondary);">{{ formatLabels(s.labels) }}</span>
                <span style="color: var(--cyan); font-weight: 600; margin-left: auto;">{{ typeof s.value === 'number' ? s.value.toFixed(2) : s.value }}</span>
              </div>
            </div>
          </div>
          <div v-else style="color: var(--red); font-size: 13px;">
            <el-icon><WarningFilled /></el-icon> {{ validationResult.error || validationResult.message }}
          </div>
        </div>

        <el-form-item label="描述">
          <el-input v-model="configForm.description" placeholder="可选" />
        </el-form-item>
        <el-row :gutter="16">
          <el-col :span="7">
            <el-form-item label="阈值">
              <el-input v-model.number="configForm.threshold" type="number" step="0.5" style="width: 100%;" placeholder="请输入阈值" />
            </el-form-item>
          </el-col>
          <el-col :span="8">
            <el-form-item label="条件">
              <el-select v-model="configForm.threshold_type" style="width: 100%;">
                <el-option label="大于 >" value="greater" />
                <el-option label="大于等于 >=" value="greater_equal" />
                <el-option label="小于 <" value="less" />
                <el-option label="小于等于 <=" value="less_equal" />
                <el-option label="等于 =" value="equal" />
                <el-option label="不等于 !=" value="not_equal" />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="8">
            <el-form-item label="级别">
              <el-select v-model="configForm.threshold_status" style="width: 100%;">
                <el-option label="严重 critical" value="critical" />
                <el-option label="警告 warning" value="warning" />
                <el-option label="提醒 info" value="info" />
                <el-option label="正常 normal" value="normal" />
              </el-select>
            </el-form-item>
          </el-col>
        </el-row>
        <el-form-item label="单位">
          <el-input v-model="configForm.unit" placeholder="%, MB, ms" />
        </el-form-item>
        <el-form-item label="标签映射">
          <el-input v-model="labelsJsonStr" type="textarea" :rows="2" placeholder='{"instance":"实例地址","job":"任务名"}' />
          <div style="font-size: 11px; color: var(--text-tertiary); margin-top: 2px;">Prometheus 标签名到显示别名的 JSON 映射</div>
        </el-form-item>
      </el-form>
      <template #footer>
        <div class="dialog-footer">
          <el-button @click="configDialog = false" style="color: var(--text-secondary);">取消</el-button>
          <el-button type="primary" :loading="savingConfig" @click="handleSaveConfig">保存</el-button>
        </div>
      </template>
    </el-dialog>

    <el-dialog v-model="typeDialog" title="指标类型管理" width="560" :close-on-click-modal="false">
      <div v-loading="typeLoading">
        <div style="margin-bottom: 12px; display: flex; justify-content: flex-end;">
          <el-button size="small" type="primary" @click="openCreateType">
            <el-icon><Plus /></el-icon> 新建类型
          </el-button>
        </div>
        <el-table :data="metricTypes" stripe size="small">
          <el-table-column type="index" label="#" width="50" />
          <el-table-column prop="type_name" label="类型名称" min-width="200" />
          <el-table-column label="指标数" width="100" align="center">
            <template #default="{ row }">
              <span style="color: var(--cyan);">{{ (row.configs || []).length }}</span>
            </template>
          </el-table-column>
          <el-table-column label="操作" width="180" fixed="right">
            <template #default="{ row }">
              <el-button size="small" text @click="openEditType(row)" style="color: var(--cyan);">编辑</el-button>
              <el-button size="small" text @click="handleDeleteType(row)" style="color: var(--red);">删除</el-button>
            </template>
          </el-table-column>
        </el-table>
        <el-empty v-if="!typeLoading && metricTypes.length === 0" description="暂无类型" :image-size="50" />
      </div>
    </el-dialog>

    <el-dialog v-model="typeFormDialog" :title="editingTypeId ? '编辑类型' : '新建类型'" width="400" :close-on-click-modal="false">
      <el-form ref="typeFormRef" :model="typeForm" :rules="typeRules" label-width="80px">
        <el-form-item label="类型名称" prop="type_name">
          <el-input v-model="typeForm.type_name" placeholder="例如：L8-应用层：自定义业务监控" />
        </el-form-item>
      </el-form>
      <template #footer>
        <div class="dialog-footer">
          <el-button @click="typeFormDialog = false" style="color: var(--text-secondary);">取消</el-button>
          <el-button type="primary" :loading="savingType" @click="handleSaveType">保存</el-button>
        </div>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import type { FormInstance } from 'element-plus'
import { getMetricTypes, createMetricConfig, updateMetricConfig, deleteMetricConfig, getAllDataSources, validatePromQL, createMetricType, updateMetricType, deleteMetricType } from '../api'
import type { MetricType, MetricConfig, DataSource } from '../types'

interface FlatMetric extends MetricConfig {
  type_name: string
}

const loading = ref(false)
const savingConfig = ref(false)
const validating = ref(false)
const datasources = ref<DataSource[]>([])
const metricTypes = ref<MetricType[]>([])
const flatMetrics = ref<FlatMetric[]>([])
const filterDS = ref<number | ''>('')

const configDialog = ref(false)
const editingConfigId = ref<number | null>(null)
const configFormRef = ref<FormInstance>()
const configForm = ref<MetricConfig>({ metric_type_id: 0, datasource_id: undefined, name: '', query: '', description: '', threshold: 0, threshold_type: 'greater', threshold_status: 'critical', unit: '', labels_json: '' })
const labelsJsonStr = ref('')
const validationResult = ref<any>(null)
const configRules = {
  metric_type_id: [{ required: true, message: '请选择指标类型', trigger: 'change' }],
  name: [{ required: true, message: '请输入指标名称', trigger: 'blur' }],
  query: [{ required: true, message: '请输入 PromQL', trigger: 'blur' }],
}

const thresholdOpMap: Record<string, string> = {
  greater: '>', greater_equal: '>=', less: '<', less_equal: '<=', equal: '=', not_equal: '!=',
}
const statusStyleMap: Record<string, any> = {
  critical: { background: 'rgba(239,68,68,0.15)', color: '#ef4444', border: 'none' },
  warning: { background: 'rgba(245,158,11,0.15)', color: '#f59e0b', border: 'none' },
  info: { background: 'rgba(59,130,246,0.15)', color: '#3b82f6', border: 'none' },
  normal: { background: 'rgba(16,185,129,0.15)', color: '#10b981', border: 'none' },
}
const statusLabelMap: Record<string, string> = {
  critical: '严重', warning: '警告', info: '提醒', normal: '正常',
}

function thresholdOpLabel(t: string) { return thresholdOpMap[t] || t }
function statusStyle(s: string) { return statusStyleMap[s] || statusStyleMap.critical }
function statusLabel(s: string) { return statusLabelMap[s] || s }

// Sync labels_json with textarea string
watch(labelsJsonStr, (val) => {
  try {
    JSON.parse(val)
    configForm.value.labels_json = val
  } catch { /* keep previous valid value */ }
})

function formatLabels(labels: Record<string, string>) {
  return Object.entries(labels).map(([k, v]) => `${k}="${v}"`).join(', ')
}

function dsName(id: number | null | undefined) {
  if (!id) return ''
  const d = datasources.value.find(x => x.id === id)
  return d ? d.name : ''
}

async function fetchData() {
  loading.value = true
  try {
    const dsFilter = filterDS.value !== '' ? Number(filterDS.value) : null
    const [mtRes, dsRes] = await Promise.all([
      dsFilter !== null ? getMetricTypes(dsFilter) : getMetricTypes(),
      getAllDataSources()
    ])
    metricTypes.value = mtRes.data
    datasources.value = dsRes.data

    const flat: FlatMetric[] = []
    for (const mt of metricTypes.value) {
      for (const cfg of mt.configs || []) {
        flat.push({ ...cfg, type_name: mt.type_name })
      }
    }
    flatMetrics.value = flat
  } finally { loading.value = false }
}

function openCreateConfig() {
  editingConfigId.value = null
  configForm.value = { metric_type_id: metricTypes.value[0]?.id || 0, datasource_id: undefined, name: '', query: '', description: '', threshold: 0, threshold_type: 'greater', threshold_status: 'critical', unit: '', labels_json: '' }
  labelsJsonStr.value = ''
  validationResult.value = null
  configDialog.value = true
}

function openEditConfig(row: FlatMetric) {
  editingConfigId.value = row.id!
  configForm.value = { ...row }
  // Pretty-print labels_json
  try { labelsJsonStr.value = row.labels_json ? JSON.stringify(JSON.parse(row.labels_json), null, 2) : '' }
  catch { labelsJsonStr.value = row.labels_json || '' }
  validationResult.value = null
  configDialog.value = true
}

async function handleSaveConfig() {
  const valid = await configFormRef.value?.validate().catch(() => false)
  if (!valid) return
  // Validate labels_json
  if (labelsJsonStr.value) {
    try {
      JSON.parse(labelsJsonStr.value)
      configForm.value.labels_json = labelsJsonStr.value
    } catch {
      ElMessage.warning('标签映射 JSON 格式不正确')
      return
    }
  } else {
    configForm.value.labels_json = ''
  }
  savingConfig.value = true
  try {
    if (editingConfigId.value) {
      await updateMetricConfig(editingConfigId.value, configForm.value)
      ElMessage.success('更新成功')
    } else {
      await createMetricConfig(configForm.value)
      ElMessage.success('创建成功')
    }
    configDialog.value = false
    await fetchData()
  } catch (e: any) { ElMessage.error(e.message) }
  finally { savingConfig.value = false }
}

async function handleDeleteConfig(row: FlatMetric) {
  try {
    await ElMessageBox.confirm(`确定删除指标「${row.name}」？`, '确认删除', { type: 'warning', cancelButtonText: '取消', confirmButtonText: '删除' })
    await deleteMetricConfig(row.id!)
    ElMessage.success('删除成功')
    await fetchData()
  } catch { /* ignore */ }
}

async function handleValidate() {
  if (!configForm.value.query.trim()) { ElMessage.warning('请先输入 PromQL'); return }
  validating.value = true
  validationResult.value = null
  try {
    const res = await validatePromQL(configForm.value.datasource_id || undefined, configForm.value.query)
    validationResult.value = res.data
    if (!res.data.valid) {
      ElMessage.warning(res.data.error || '语法错误')
    }
  } catch (e: any) {
    validationResult.value = { valid: false, error: e.message }
    ElMessage.error(e.message)
  } finally { validating.value = false }
}

// Type management
const typeDialog = ref(false)
const typeLoading = ref(false)
const typeFormDialog = ref(false)
const editingTypeId = ref<number | null>(null)
const typeFormRef = ref<FormInstance>()
const typeForm = ref({ type_name: '' })
const typeRules = { type_name: [{ required: true, message: '请输入类型名称', trigger: 'blur' }] }
const savingType = ref(false)

function openCreateType() {
  editingTypeId.value = null
  typeForm.value = { type_name: '' }
  typeFormDialog.value = true
}

function openEditType(row: any) {
  editingTypeId.value = row.id
  typeForm.value = { type_name: row.type_name }
  typeFormDialog.value = true
}

async function handleSaveType() {
  const valid = await typeFormRef.value?.validate().catch(() => false)
  if (!valid) return
  savingType.value = true
  try {
    if (editingTypeId.value) {
      await updateMetricType(editingTypeId.value, typeForm.value)
      ElMessage.success('更新成功')
    } else {
      await createMetricType(typeForm.value)
      ElMessage.success('创建成功')
    }
    typeFormDialog.value = false
    await fetchData()
    typeLoading.value = false
  } catch (e: any) { ElMessage.error(e.message) }
  finally { savingType.value = false }
}

async function handleDeleteType(row: any) {
  try {
    await ElMessageBox.confirm(`确定删除类型「${row.type_name}」？其下的所有指标将一并删除。`, '确认删除', { type: 'warning', confirmButtonText: '删除', cancelButtonText: '取消' })
    await deleteMetricType(row.id)
    ElMessage.success('删除成功')
    await fetchData()
  } catch { /* ignore */ }
}

onMounted(fetchData)
</script>

<style scoped>
.validation-panel {
  margin: 4px 0 12px 24px;
  padding: 10px 14px;
  border-radius: 8px;
  border: 1px solid;
  font-size: 13px;
}
.validation-panel.valid {
  background: rgba(16,185,129,0.05);
  border-color: rgba(16,185,129,0.2);
}
.validation-panel.invalid {
  background: rgba(239,68,68,0.05);
  border-color: rgba(239,68,68,0.2);
}
</style>
