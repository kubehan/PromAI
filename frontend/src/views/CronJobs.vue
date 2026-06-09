<template>
  <div class="page-container">
    <div class="page-header">
      <h2><el-icon><Clock /></el-icon> 定时任务</h2>
      <p>配置定时巡检任务，支持选择数据源</p>
    </div>

    <div class="section-card">
      <div class="section-header">
        <h3><el-icon :size="16" color="#00d4ff"><List /></el-icon> 任务列表</h3>
        <el-button type="primary" @click="openCreate"><el-icon><Plus /></el-icon> 新增任务</el-button>
      </div>
      <el-table :data="jobs" v-loading="loading" stripe>
        <el-table-column type="index" label="#" width="56" />
        <el-table-column prop="name" label="任务名称" min-width="180">
          <template #default="{ row }">
            <span style="font-weight: 600; color: var(--text-primary);">{{ row.name }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="schedule" label="调度表达式" width="150">
          <template #default="{ row }">
            <el-tag size="small" style="background: rgba(0,212,255,0.1); color: var(--cyan); border: none; font-family: monospace;">{{ row.schedule }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="数据源" width="180">
          <template #default="{ row }">
            <span style="color: var(--text-secondary);">{{ dsName(row.datasource_id) }}</span>
          </template>
        </el-table-column>
        <el-table-column label="通知渠道" min-width="160">
          <template #default="{ row }">
            <span v-if="notifNames(row.notify_channels)" style="color: var(--text-tertiary); font-size: 12px;">{{ notifNames(row.notify_channels) }}</span>
            <span v-else style="color: var(--text-tertiary);">-</span>
          </template>
        </el-table-column>
        <el-table-column label="状态" width="90" align="center">
          <template #default="{ row }">
            <el-switch v-model="row.enabled" @change="toggleEnabled(row)" />
          </template>
        </el-table-column>
        <el-table-column label="上次执行" width="200">
          <template #default="{ row }">
            <span style="color: var(--text-tertiary); font-size: 13px;" v-if="row.last_run_at">{{ dayjs(row.last_run_at).format('MM-DD HH:mm') }}</span>
            <span v-else style="color: var(--text-tertiary);">-</span>
            <el-tag v-if="row.last_status" :style="{ background: row.last_status === 'success' ? 'rgba(16,185,129,0.1)' : 'rgba(239,68,68,0.1)', color: row.last_status === 'success' ? '#10b981' : '#ef4444', border: 'none', marginLeft: '6px' }" size="small">
              {{ row.last_status === 'success' ? '成功' : '失败' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="160" fixed="right">
          <template #default="{ row }">
            <el-button size="small" text @click="openEdit(row)" style="color: var(--cyan);">编辑</el-button>
            <el-button size="small" text @click="handleDelete(row)" style="color: var(--red);">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
    </div>

    <el-dialog v-model="dialogVisible" :title="editingId ? '编辑定时任务' : '新增定时任务'" width="540" :close-on-click-modal="false">
      <el-form ref="formRef" :model="form" :rules="rules" label-width="120px">
        <el-form-item label="任务名称" prop="name">
          <el-input v-model="form.name" placeholder="例如：每日巡检" />
        </el-form-item>
        <el-form-item label="调度表达式" prop="schedule">
          <el-input v-model="form.schedule" placeholder="Cron 表达式，如 00 08,17 * * *">
            <template #append>
              <el-tooltip content="分 时 日 月 周" placement="top">
                <el-icon><QuestionFilled /></el-icon>
              </el-tooltip>
            </template>
          </el-input>
        </el-form-item>
        <el-form-item label="数据源">
          <el-select v-model="form.datasource_id" placeholder="默认数据源" clearable style="width: 100%">
            <el-option v-for="ds in datasources" :key="ds.id" :label="ds.name" :value="ds.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="通知渠道">
          <el-select v-model="notifChannelIds" placeholder="选择通知渠道" multiple clearable style="width: 100%">
            <el-option v-for="nc in notifications" :key="nc.id" :label="nc.name" :value="nc.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="启用">
          <el-switch v-model="form.enabled" />
        </el-form-item>
      </el-form>
      <template #footer>
        <div class="dialog-footer">
          <el-button @click="dialogVisible = false" style="color: var(--text-secondary);">取消</el-button>
          <el-button type="primary" :loading="saving" @click="handleSave">保存</el-button>
        </div>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, watch } from 'vue'
import dayjs from 'dayjs'
import { ElMessage, ElMessageBox } from 'element-plus'
import type { FormInstance } from 'element-plus'
import { getCronJobs, createCronJob, updateCronJob, deleteCronJob, getDataSources, getNotifications } from '../api'
import type { CronJob, DataSource, NotificationChannel } from '../types'

const loading = ref(false)
const saving = ref(false)
const jobs = ref<CronJob[]>([])
const datasources = ref<DataSource[]>([])
const notifications = ref<NotificationChannel[]>([])
const dialogVisible = ref(false)
const editingId = ref<number | null>(null)
const formRef = ref<FormInstance>()
const notifChannelIds = ref<number[]>([])
const form = ref<CronJob>({ name: '', schedule: '', datasource_id: null, enabled: true })
const rules = {
  name: [{ required: true, message: '请输入任务名称', trigger: 'blur' }],
  schedule: [{ required: true, message: '请输入调度表达式', trigger: 'blur' }],
}

function dsName(id: number | null | undefined) {
  if (!id) return '默认数据源'
  const d = datasources.value.find(x => x.id === id)
  return d ? d.name : `ID: ${id}`
}

function notifNames(channels: string | undefined | null) {
  if (!channels) return ''
  let ids: number[] = []
  try { ids = JSON.parse(channels) } catch { ids = [] }
  return ids.map(id => notifications.value.find(n => n.id === id)?.name).filter(Boolean).join(', ')
}

// Sync form.notify_channels with notifChannelIds
watch(notifChannelIds, (ids) => {
  form.value.notify_channels = ids.length > 0 ? JSON.stringify(ids) : ''
})

async function fetchData() {
  loading.value = true
  try {
    const [jr, dr, nr] = await Promise.all([getCronJobs(), getDataSources(), getNotifications()])
    jobs.value = jr.data; datasources.value = dr.data.items; notifications.value = nr.data
  } finally { loading.value = false }
}

function openCreate() {
  editingId.value = null
  form.value = { name: '', schedule: '', datasource_id: null, enabled: true }
  notifChannelIds.value = []
  dialogVisible.value = true
}
function openEdit(row: CronJob) {
  editingId.value = row.id!
  form.value = { ...row }
  try { notifChannelIds.value = row.notify_channels ? JSON.parse(row.notify_channels) : [] }
  catch { notifChannelIds.value = [] }
  dialogVisible.value = true
}

async function handleSave() {
  const valid = await formRef.value?.validate().catch(() => false)
  if (!valid) return
  saving.value = true
  try {
    if (editingId.value) { await updateCronJob(editingId.value, form.value); ElMessage.success('更新成功') }
    else { await createCronJob(form.value); ElMessage.success('创建成功') }
    dialogVisible.value = false; await fetchData()
  } catch (e: any) { ElMessage.error(e.message) } finally { saving.value = false }
}

async function handleDelete(row: CronJob) {
  try {
    await ElMessageBox.confirm(`确定删除定时任务「${row.name}」？`, '确认删除', { type: 'warning', cancelButtonText: '取消', confirmButtonText: '删除' })
    await deleteCronJob(row.id!); ElMessage.success('删除成功'); await fetchData()
  } catch { /* ignore */ }
}

async function toggleEnabled(row: CronJob) {
  try { await updateCronJob(row.id!, row); ElMessage.success(row.enabled ? '已启用' : '已禁用') }
  catch { row.enabled = !row.enabled }
}

onMounted(fetchData)
</script>
