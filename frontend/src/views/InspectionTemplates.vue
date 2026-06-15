<template>
  <div class="page-container">
    <div class="page-header">
      <h2><el-icon><Collection /></el-icon> 巡检模板</h2>
      <p>创建和管理巡检模板，快速选择指标组合进行巡检</p>
    </div>

    <div class="section-card">
      <div class="section-header">
        <h3><el-icon :size="16" :color="getCssVar('--cyan')"><List /></el-icon> 模板列表</h3>
        <div style="display: flex; gap: 8px; align-items: center;">
          <el-input v-model="keyword" placeholder="搜索模板名称" clearable style="width: 200px;" @keyup.enter="fetchData" @clear="fetchData" />
          <el-button type="primary" @click="openCreate"><el-icon><Plus /></el-icon> 新建模板</el-button>
        </div>
      </div>
      <el-table :data="templates" v-loading="loading" stripe>
        <el-table-column type="index" label="#" width="56" />
        <el-table-column prop="name" label="模板名称" min-width="200">
          <template #default="{ row }">
            <span style="font-weight: 600; color: var(--text-primary);">{{ row.name }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="description" label="描述" min-width="260">
          <template #default="{ row }">
            <span style="color: var(--text-tertiary);">{{ row.description || '-' }}</span>
          </template>
        </el-table-column>
        <el-table-column label="指标数" width="90" align="center">
          <template #default="{ row }">
            <span style="color: var(--cyan); font-weight: 700;">{{ row.metric_count }}</span>
          </template>
        </el-table-column>
        <el-table-column label="创建时间" width="170">
          <template #default="{ row }">{{ dayjs(row.created_at).format('YYYY-MM-DD HH:mm') }}</template>
        </el-table-column>
        <el-table-column label="操作" width="340" fixed="right">
          <template #default="{ row }">
            <el-button size="small" text @click="viewMetrics(row)" style="color: var(--text-secondary);">查看指标</el-button>
            <el-button size="small" text @click="editMetrics(row)" style="color: var(--cyan);">配置指标</el-button>
            <el-dropdown trigger="click" @command="(cmd: string) => handleMore(row, cmd)">
              <el-button size="small" text style="color: var(--text-tertiary);">
                更多<el-icon><ArrowDown /></el-icon>
              </el-button>
              <template #dropdown>
                <el-dropdown-menu>
                  <el-dropdown-item command="edit">编辑信息</el-dropdown-item>
                  <el-dropdown-item command="delete" style="color: var(--red);">删除</el-dropdown-item>
                </el-dropdown-menu>
              </template>
            </el-dropdown>
          </template>
        </el-table-column>
      </el-table>
      <div v-if="total > pageSize" style="display: flex; justify-content: flex-end; margin-top: 16px; padding: 0 24px 16px;">
        <el-pagination
          v-model:current-page="page"
          v-model:page-size="pageSize"
          :page-sizes="[10, 20, 50, 100]"
          :total="total"
          layout="total, sizes, prev, pager, next"
          background
          @change="fetchData"
        />
      </div>
      <el-empty v-if="!loading && templates.length === 0" description="暂无巡检模板" :image-size="60" />
    </div>

    <el-dialog v-model="createDialog" :title="editingId ? '编辑模板' : '新建模板'" width="480" :close-on-click-modal="false">
      <el-form ref="formRef" :model="form" :rules="rules" label-width="100px">
        <el-form-item label="模板名称" prop="name">
          <el-input v-model="form.name" placeholder="例如：基础巡检模板" />
        </el-form-item>
        <el-form-item label="描述">
          <el-input v-model="form.description" placeholder="可选" type="textarea" :rows="2" />
        </el-form-item>
      </el-form>
      <template #footer>
        <div class="dialog-footer">
          <el-button @click="createDialog = false" style="color: var(--text-secondary);">取消</el-button>
          <el-button type="primary" :loading="saving" @click="handleSave">保存</el-button>
        </div>
      </template>
    </el-dialog>

    <el-dialog v-model="metricsDialog" title="配置模板指标" width="640" :close-on-click-modal="false" top="3vh">
      <div v-loading="metricsLoading">
        <div style="margin-bottom: 12px; display: flex; justify-content: space-between; align-items: center;">
          <span style="color: var(--text-tertiary); font-size: 13px;">
            为「<strong style="color: var(--text-primary);">{{ editingTemplate?.name }}</strong>」选择指标
          </span>
          <div>
            <el-button size="small" text @click="selectAll" style="color: var(--cyan);">全选</el-button>
            <el-button size="small" text @click="deselectAll" style="color: var(--text-tertiary);">清空</el-button>
          </div>
        </div>
        <div style="max-height: 400px; overflow-y: auto; border: 1px solid var(--border); border-radius: 10px; padding: 12px;">
          <div v-for="mt in metricTypes" :key="mt.id" style="margin-bottom: 10px;">
            <div style="font-weight: 600; color: var(--text-primary); margin-bottom: 4px; font-size: 14px;">
              <el-checkbox
                :model-value="isTypeAllSelected(mt)"
                :indeterminate="isTypeIndeterminate(mt)"
                @change="(val: boolean) => toggleType(mt, val)"
              >
                {{ mt.type_name }} ({{ (mt.configs || []).length }})
              </el-checkbox>
            </div>
            <div style="padding-left: 28px;">
              <el-checkbox-group v-model="selectedConfigIds">
                <el-checkbox v-for="cfg in mt.configs || []" :key="cfg.id" :label="cfg.id" style="margin-bottom: 2px;">
                  <span style="font-size: 13px;">{{ cfg.name }}</span>
                </el-checkbox>
              </el-checkbox-group>
            </div>
          </div>
          <el-empty v-if="metricTypes.length === 0" description="暂无指标配置" :image-size="40" />
        </div>
      </div>
      <template #footer>
        <div class="dialog-footer">
          <el-button @click="metricsDialog = false" style="color: var(--text-secondary);">取消</el-button>
          <el-button type="primary" :loading="savingMetrics" @click="handleSaveMetrics">保存配置</el-button>
        </div>
      </template>
    </el-dialog>

    <el-dialog v-model="detailDialog" :title="'模板指标 — ' + (detailTemplate?.name || '')" width="900" :close-on-click-modal="false" top="3vh">
      <div v-loading="detailLoading">
        <el-table :data="detailMetrics" stripe size="small">
          <el-table-column type="index" label="#" width="50" />
          <el-table-column label="类型" width="180">
            <template #default="{ row }">
              <el-tag size="small" style="background: rgba(99,102,241,0.12); color: #818cf8; border: none;">
                {{ row.type_name }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="name" label="指标名称" min-width="150">
            <template #default="{ row }">
              <span style="font-weight: 600; color: var(--text-primary);">{{ row.name }}</span>
            </template>
          </el-table-column>
          <el-table-column prop="query" label="PromQL" min-width="260">
            <template #default="{ row }">
              <code style="font-size: 11px; background: rgba(0,0,0,0.3); padding: 2px 8px; border-radius: 6px; color: var(--text-tertiary); display: block; max-width: 350px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap;">{{ row.query }}</code>
            </template>
          </el-table-column>
          <el-table-column label="阈值" width="120">
            <template #default="{ row }">
              <span v-if="row.threshold" style="color: var(--text-secondary); font-size: 12px;">
                {{ thresholdOpLabel(row.threshold_type) }} {{ row.threshold }}{{ row.unit }}
              </span>
              <span v-else style="color: var(--text-tertiary);">-</span>
            </template>
          </el-table-column>
          <el-table-column label="级别" width="70" align="center">
            <template #default="{ row }">
              <el-tag v-if="row.threshold_status" size="small" :style="statusStyle(row.threshold_status)" effect="dark">
                {{ statusLabel(row.threshold_status) }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column label="标签数" width="70" align="center">
            <template #default="{ row }">
              <span style="color: var(--text-tertiary);">{{ row.labels_json ? Object.keys(JSON.parse(row.labels_json)).length : 0 }}</span>
            </template>
          </el-table-column>
          <el-table-column label="操作" width="80" fixed="right">
            <template #default="{ row }">
              <el-button size="small" text @click="openEditMetricInTmpl(row)" style="color: var(--cyan);">编辑</el-button>
            </template>
          </el-table-column>
        </el-table>
        <el-empty v-if="!detailLoading && detailMetrics.length === 0" description="该模板尚未配置指标" :image-size="50" />
      </div>
    </el-dialog>

    <el-dialog v-model="metricEditDialog" :title="'编辑指标 — ' + (editingMetric?.name || '')" width="720" :close-on-click-modal="false" top="3vh">
      <el-form ref="metricEditFormRef" :model="metricEditForm" :rules="metricEditRules" label-width="110px">
        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item label="指标名称" prop="name">
              <el-input v-model="metricEditForm.name" disabled>
                <template #suffix><el-tooltip content="全局属性，不可修改" placement="top"><el-icon style="color: var(--text-tertiary);"><InfoFilled /></el-icon></el-tooltip></template>
              </el-input>
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="描述">
              <el-input v-model="metricEditForm.description" disabled />
            </el-form-item>
          </el-col>
        </el-row>
        <el-form-item label="PromQL" prop="query">
          <el-input v-model="metricEditForm.query" type="textarea" :rows="2" />
        </el-form-item>
        <el-row :gutter="16">
          <el-col :span="6">
            <el-form-item label="阈值">
              <el-input v-model.number="metricEditForm.threshold" type="number" step="0.1" style="width: 100%;" placeholder="请输入阈值" />
            </el-form-item>
          </el-col>
          <el-col :span="9">
            <el-form-item label="条件">
              <el-select v-model="metricEditForm.threshold_type" style="width: 100%;">
                <el-option label="大于 >" value="greater" />
                <el-option label="大于等于 >=" value="greater_equal" />
                <el-option label="小于 <" value="less" />
                <el-option label="小于等于 <=" value="less_equal" />
                <el-option label="等于 =" value="equal" />
                <el-option label="不等于 !=" value="not_equal" />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="9">
            <el-form-item label="级别">
              <el-select v-model="metricEditForm.threshold_status" style="width: 100%;">
                <el-option label="严重 critical" value="critical" />
                <el-option label="警告 warning" value="warning" />
                <el-option label="提醒 info" value="info" />
                <el-option label="正常 normal" value="normal" />
              </el-select>
            </el-form-item>
          </el-col>
        </el-row>
        <el-form-item label="单位">
          <el-input v-model="metricEditForm.unit" placeholder="%, MB, ms" />
        </el-form-item>
        <el-form-item label="标签映射">
          <el-input v-model="metricEditLabelsJson" type="textarea" :rows="2" placeholder='{"instance":"实例地址","job":"任务名"}' />
        </el-form-item>
      </el-form>
      <template #footer>
        <div class="dialog-footer">
          <el-button @click="metricEditDialog = false" style="color: var(--text-secondary);">取消</el-button>
          <el-button type="primary" :loading="savingMetricEdit" @click="handleSaveMetricEdit">保存</el-button>
        </div>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import dayjs from 'dayjs'
import { ElMessage, ElMessageBox } from 'element-plus'
import type { FormInstance } from 'element-plus'
import { getTemplates, createTemplate, updateTemplate, deleteTemplate, getTemplate, getTemplateMetrics, setTemplateMetrics, getMetricTypes, saveTemplateMetricOverride } from '../api'
import type { MetricType } from '../types'

function getCssVar(name: string): string {
  return getComputedStyle(document.documentElement).getPropertyValue(name).trim()
}

const loading = ref(false)
const saving = ref(false)
const metricsLoading = ref(false)
const savingMetrics = ref(false)
const templates = ref<any[]>([])
const page = ref(1)
const pageSize = ref(20)
const total = ref(0)
const keyword = ref('')
const metricTypes = ref<MetricType[]>([])
const selectedConfigIds = ref<number[]>([])
const editingTemplate = ref<any>(null)

const createDialog = ref(false)
const metricsDialog = ref(false)
const detailDialog = ref(false)
const editingId = ref<number | null>(null)
const formRef = ref<FormInstance>()
const form = ref({ name: '', description: '' })
const rules = { name: [{ required: true, message: '请输入模板名称', trigger: 'blur' }] }

// Detail view state
const detailTemplate = ref<any>(null)
const detailLoading = ref(false)
const detailMetrics = ref<any[]>([])

async function fetchData() {
  loading.value = true
  try {
    const params: any = { page: page.value, page_size: pageSize.value }
    if (keyword.value) params.keyword = keyword.value
    const res = await getTemplates(params)
    templates.value = res.data.items
    total.value = res.data.total
  } finally { loading.value = false }
}

function openCreate() {
  editingId.value = null
  form.value = { name: '', description: '' }
  createDialog.value = true
}

async function handleSave() {
  const valid = await formRef.value?.validate().catch(() => false)
  if (!valid) return
  saving.value = true
  try {
    if (editingId.value) {
      await updateTemplate(editingId.value, form.value)
      ElMessage.success('更新成功')
    } else {
      await createTemplate(form.value.name, form.value.description)
      ElMessage.success('创建成功')
    }
    createDialog.value = false
    await fetchData()
  } catch (e: any) { ElMessage.error(e.message) }
  finally { saving.value = false }
}

async function handleMore(row: any, cmd: string) {
  switch (cmd) {
    case 'edit':
      editingId.value = row.id
      form.value = { name: row.name, description: row.description || '' }
      createDialog.value = true
      break
    case 'delete':
      try {
        await ElMessageBox.confirm(`确定删除模板「${row.name}」？`, '确认删除', { type: 'warning' })
        await deleteTemplate(row.id)
        ElMessage.success('删除成功')
        await fetchData()
      } catch { /* ignore */ }
      break
  }
}

async function editMetrics(row: any) {
  editingTemplate.value = row
  metricsDialog.value = true
  metricsLoading.value = true
  selectedConfigIds.value = []
  try {
    const [mtRes, tmRes] = await Promise.all([getMetricTypes(), getTemplateMetrics(row.id)])
    metricTypes.value = mtRes.data
    selectedConfigIds.value = (tmRes.data || []).map((c: any) => c.id)
  } catch (e: any) { ElMessage.error(e.message) }
  finally { metricsLoading.value = false }
}

function isTypeAllSelected(mt: MetricType) {
  if (!mt.configs?.length) return false
  return mt.configs.every(c => selectedConfigIds.value.includes(c.id!))
}

function isTypeIndeterminate(mt: MetricType) {
  if (!mt.configs?.length) return false
  const some = mt.configs.some(c => selectedConfigIds.value.includes(c.id!))
  return some && !isTypeAllSelected(mt)
}

function toggleType(mt: MetricType, checked: boolean) {
  const ids = mt.configs?.map(c => c.id!) || []
  if (checked) {
    ids.forEach(id => { if (!selectedConfigIds.value.includes(id)) selectedConfigIds.value.push(id) })
  } else {
    selectedConfigIds.value = selectedConfigIds.value.filter(id => !ids.includes(id))
  }
}

function selectAll() {
  selectedConfigIds.value = []
  metricTypes.value.forEach(mt => mt.configs?.forEach(c => { if (c.id) selectedConfigIds.value.push(c.id) }))
}
function deselectAll() { selectedConfigIds.value = [] }

async function handleSaveMetrics() {
  if (!editingTemplate.value?.id) return
  savingMetrics.value = true
  try {
    await setTemplateMetrics(editingTemplate.value.id, selectedConfigIds.value)
    ElMessage.success('模板指标更新成功')
    metricsDialog.value = false
    await fetchData()
  } catch (e: any) { ElMessage.error(e.message) }
  finally { savingMetrics.value = false }
}

async function viewMetrics(row: any) {
  detailTemplate.value = row
  detailDialog.value = true
  detailLoading.value = true
  detailMetrics.value = []
  try {
    const [tmRes, mtRes] = await Promise.all([getTemplateMetrics(row.id), getMetricTypes()])
    const configs = tmRes.data || []
    const typeMap: Record<number, string> = {}
    ;(mtRes.data || []).forEach((mt: any) => { if (mt.id) typeMap[mt.id] = mt.type_name })
    detailMetrics.value = configs.map((c: any) => ({ ...c, type_name: typeMap[c.metric_type_id] || '未知' }))
  } catch (e: any) { ElMessage.error(e.message) }
  finally { detailLoading.value = false }
}

// Metric edit within template detail
const metricEditDialog = ref(false)
const editingMetric = ref<any>(null)
const metricEditFormRef = ref<FormInstance>()
const metricEditForm = ref<any>({})
const metricEditLabelsJson = ref('')
const savingMetricEdit = ref(false)
const metricEditRules = {
  name: [{ required: true, message: '请输入指标名称', trigger: 'blur' }],
  query: [{ required: true, message: '请输入 PromQL', trigger: 'blur' }],
}

function openEditMetricInTmpl(row: any) {
  editingMetric.value = row
  metricEditForm.value = { ...row }
  try { metricEditLabelsJson.value = row.labels_json ? JSON.stringify(JSON.parse(row.labels_json), null, 2) : '' }
  catch { metricEditLabelsJson.value = row.labels_json || '' }
  metricEditDialog.value = true
}

async function handleSaveMetricEdit() {
  const valid = await metricEditFormRef.value?.validate().catch(() => false)
  if (!valid) return
  if (metricEditLabelsJson.value) {
    try { JSON.parse(metricEditLabelsJson.value); metricEditForm.value.labels_json = metricEditLabelsJson.value }
    catch { ElMessage.warning('标签映射 JSON 格式不正确'); return }
  } else { metricEditForm.value.labels_json = '' }
  savingMetricEdit.value = true
  try {
    await saveTemplateMetricOverride(detailTemplate.value.id, editingMetric.value.id, {
      query: metricEditForm.value.query,
      threshold: metricEditForm.value.threshold,
      threshold_type: metricEditForm.value.threshold_type,
      threshold_status: metricEditForm.value.threshold_status,
      unit: metricEditForm.value.unit,
      labels_json: metricEditForm.value.labels_json,
    })
    ElMessage.success('指标更新成功')
    metricEditDialog.value = false
    await viewMetrics(detailTemplate.value)
  } catch (e: any) { ElMessage.error(e.message) }
  finally { savingMetricEdit.value = false }
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

onMounted(fetchData)
</script>
