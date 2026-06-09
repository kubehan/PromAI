<template>
  <div class="page-container">
    <div class="page-header">
      <h2><el-icon><Connection /></el-icon> 数据源管理</h2>
      <p>管理 Prometheus 数据源连接配置</p>
    </div>

    <div class="section-card">
      <div class="section-header">
        <h3><el-icon :size="16" color="#00d4ff"><List /></el-icon> 数据源列表</h3>
        <div class="action-bar">
          <el-button plain @click="importVisible = true">
            <el-icon><Upload /></el-icon> YAML 导入
          </el-button>
          <el-button plain @click="syncDialogVisible = true">
            <el-icon><Refresh /></el-icon> 数据源同步
          </el-button>
          <el-button type="primary" @click="openCreate">
            <el-icon><Plus /></el-icon> 新增数据源
          </el-button>
        </div>
      </div>
      <el-table :data="datasources" v-loading="loading" stripe>
        <el-table-column type="index" label="#" width="56" />
        <el-table-column prop="name" label="名称" min-width="180">
          <template #default="{ row }">
            <div style="display: flex; align-items: center; gap: 8px;">
              <span style="font-weight: 600; color: var(--text-primary);">{{ row.name }}</span>
              <el-tag v-if="row.is_default" size="small" effect="dark" style="background: rgba(0,212,255,0.15); color: var(--cyan); border: none;">默认</el-tag>
            </div>
          </template>
        </el-table-column>
        <el-table-column prop="url" label="URL" min-width="320">
          <template #default="{ row }">
            <code style="font-size: 12px; color: var(--text-tertiary);">{{ row.url }}</code>
          </template>
        </el-table-column>
        <el-table-column prop="username" label="用户名" width="120">
          <template #default="{ row }">
            <span v-if="row.username" style="color: var(--text-secondary);">{{ row.username }}</span>
            <span v-else style="color: var(--text-tertiary);">-</span>
          </template>
        </el-table-column>
        <el-table-column label="巡检模板" width="160">
          <template #default="{ row }">
            <span v-if="row.template_id" style="color: var(--text-secondary); font-size: 13px;">{{ templateName(row.template_id) }}</span>
            <span v-else style="color: var(--text-tertiary); font-size: 13px;">-</span>
          </template>
        </el-table-column>
        <el-table-column label="通知渠道" width="180">
          <template #default="{ row }">
            <span v-if="row.notify_channels" style="color: var(--text-secondary); font-size: 13px;">{{ channelNames(row.notify_channels) }}</span>
            <span v-else style="color: var(--text-tertiary); font-size: 13px;">-</span>
          </template>
        </el-table-column>
        <el-table-column label="创建时间" width="170">
          <template #default="{ row }">{{ dayjs(row.created_at).format('MM-DD HH:mm') }}</template>
        </el-table-column>
        <el-table-column label="操作" width="360" fixed="right">
          <template #default="{ row }">
            <el-button size="small" text @click="openEdit(row)" style="color: var(--cyan);">编辑</el-button>
            <el-button size="small" text @click="testConnectivity(row)" style="color: var(--emerald);">测试连接</el-button>
            <el-button size="small" text @click="inspectDS(row)" style="color: var(--orange);">巡检</el-button>
            <el-dropdown trigger="click" @command="(cmd: string) => handleMore(row, cmd)">
              <el-button size="small" text style="color: var(--text-tertiary);">
                更多<el-icon><ArrowDown /></el-icon>
              </el-button>
              <template #dropdown>
                <el-dropdown-menu>
                  <el-dropdown-item command="template"><el-icon><CopyDocument /></el-icon> 导入全局指标</el-dropdown-item>
                  <el-dropdown-item command="delete" style="color: var(--red);"><el-icon><Delete /></el-icon> 删除</el-dropdown-item>
                </el-dropdown-menu>
              </template>
            </el-dropdown>
          </template>
        </el-table-column>
      </el-table>
    </div>

    <el-dialog v-model="dialogVisible" :title="editingId ? '编辑数据源' : '新增数据源'" width="520" :close-on-click-modal="false">
      <el-form ref="formRef" :model="form" :rules="rules" label-width="100px">
        <el-form-item label="名称" prop="name">
          <el-input v-model="form.name" placeholder="例如：生产环境 Prometheus" />
        </el-form-item>
        <el-form-item label="URL" prop="url">
          <el-input v-model="form.url" placeholder="http://prometheus:9090" />
        </el-form-item>
        <el-form-item label="用户名">
          <el-input v-model="form.username" placeholder="可选" />
        </el-form-item>
        <el-form-item label="密码">
          <el-input v-model="form.password" type="password" placeholder="可选" show-password />
        </el-form-item>
        <el-form-item label="巡检模板">
          <el-select v-model="form.template_id" placeholder="不绑定模板（使用指标列表中的配置）" clearable style="width: 100%;">
            <el-option v-for="t in templates" :key="t.id" :label="t.name + ' (' + t.metric_count + ' 指标)'" :value="t.id" />
          </el-select>
          <div style="font-size: 11px; color: var(--text-tertiary); margin-top: 2px;">绑定的模板在巡检时优先使用，未绑定时使用「导入全局指标」生成的配置</div>
        </el-form-item>
        <el-form-item label="通知渠道">
          <el-select v-model="selectedChannels" multiple placeholder="不发送通知" clearable style="width: 100%;">
            <el-option v-for="ch in notifChannels" :key="ch.id" :label="ch.name + ' (' + ch.channel_type + ')'" :value="ch.id" />
          </el-select>
          <div style="font-size: 11px; color: var(--text-tertiary); margin-top: 2px;">点击「巡检」时将自动推送报告到选中的通知渠道</div>
        </el-form-item>
      </el-form>
      <template #footer>
        <div class="dialog-footer">
          <el-button @click="dialogVisible = false" style="color: var(--text-secondary);">取消</el-button>
          <el-button type="primary" :loading="saving" @click="handleSave">保存</el-button>
        </div>
      </template>
    </el-dialog>

    <el-dialog v-model="importVisible" title="YAML 批量导入" width="560">
      <el-alert title="YAML 格式示例" type="info" :closable="false" style="margin-bottom: 16px;">
        <template #default>
          <pre style="font-size: 12px; line-height: 1.6; margin: 8px 0 0; color: var(--text-tertiary);">
- name: 生产环境
  url: http://prometheus-prod:9090
  username: admin
  password: xxx
- name: 测试环境
 url: http://prometheus-test:9090
          </pre>
        </template>
      </el-alert>
      <el-input v-model="yamlContent" type="textarea" :rows="10" placeholder="请输入 YAML 格式的数据源配置..." />
      <template #footer>
        <div class="dialog-footer">
          <el-button @click="importVisible = false" style="color: var(--text-secondary);">取消</el-button>
          <el-button type="primary" :loading="importing" @click="handleImport">导入</el-button>
        </div>
      </template>
    </el-dialog>

    <!-- Sync Source Dialog -->
    <el-dialog v-model="syncDialogVisible" title="数据源同步" width="800" :close-on-click-modal="false">
      <div style="display: flex; gap: 16px; flex-direction: column;">
        <!-- Sync Source List -->
        <div style="display: flex; gap: 8px; align-items: center; flex-wrap: wrap;">
          <template v-for="ss in syncSources" :key="ss.id">
            <el-tag
              style="cursor: pointer;"
              :type="selectedSyncId === ss.id ? '' : 'info'"
              :effect="selectedSyncId === ss.id ? 'dark' : 'plain'"
              closable
              @click="selectSync(ss.id!)"
              @close="handleDeleteSync(ss.id!)"
            >
              {{ ss.name }}
            </el-tag>
          </template>
          <el-button size="small" circle @click="editSync(null)">
            <el-icon><Plus /></el-icon>
          </el-button>
          <el-button v-if="selectedSyncId" size="small" :loading="syncing" @click="handleTriggerSync">
            <el-icon><Refresh /></el-icon> 立即同步
          </el-button>
        </div>

        <!-- Edit Form -->
        <el-form v-if="syncForm" :model="syncForm" label-width="120" size="small">
          <el-form-item label="名称">
            <el-input v-model="syncForm.name" placeholder="同步源名称" />
          </el-form-item>
          <el-form-item label="请求 URL">
            <el-input v-model="syncForm.url" placeholder="https://example.com/api/endpoints" />
          </el-form-item>
          <el-row :gutter="12">
            <el-col :span="6">
              <el-form-item label="方法">
                <el-select v-model="syncForm.method" style="width: 100%;">
                  <el-option label="GET" value="GET" />
                  <el-option label="POST" value="POST" />
                </el-select>
              </el-form-item>
            </el-col>
            <el-col :span="18">
              <el-form-item label="请求头">
                <el-input v-model="syncForm.headers" placeholder='{"Authorization":"Bearer xxx"}' />
              </el-form-item>
            </el-col>
          </el-row>
          <el-form-item v-if="syncForm.method === 'POST'" label="请求体">
            <el-input v-model="syncForm.body" type="textarea" :rows="3" placeholder='{"key":"value"}' />
          </el-form-item>
          <el-form-item label="认证方式">
            <el-select v-model="syncForm.auth_type" style="width: 200px;">
              <el-option label="无" value="none" />
              <el-option label="Basic Auth" value="basic" />
              <el-option label="Bearer Token" value="bearer" />
            </el-select>
          </el-form-item>
          <template v-if="syncForm.auth_type === 'basic'">
            <el-row :gutter="12">
              <el-col :span="12">
                <el-form-item label="用户名">
                  <el-input v-model="syncForm.auth_username" />
                </el-form-item>
              </el-col>
              <el-col :span="12">
                <el-form-item label="密码">
                  <el-input v-model="syncForm.auth_password" type="password" />
                </el-form-item>
              </el-col>
            </el-row>
          </template>
          <el-form-item v-if="syncForm.auth_type === 'bearer'" label="Token">
            <el-input v-model="syncForm.auth_token" type="password" />
          </el-form-item>
          <el-divider content-position="left">字段映射</el-divider>
          <el-form-item label="数据路径">
            <el-input v-model="syncForm.data_path" placeholder='例如: data.items（留空表示直接使用根数组）' />
          </el-form-item>
          <el-row :gutter="12">
            <el-col :span="12">
              <el-form-item label="名称字段" required>
                <el-input v-model="syncForm.name_field" placeholder="name" />
              </el-form-item>
            </el-col>
            <el-col :span="12">
              <el-form-item label="URL 字段">
                <el-input v-model="syncForm.url_field" placeholder="url / endpoint" />
              </el-form-item>
            </el-col>
          </el-row>
          <el-row :gutter="12">
            <el-col :span="12">
              <el-form-item label="用户名字段">
                <el-input v-model="syncForm.username_field" placeholder="username" />
              </el-form-item>
            </el-col>
            <el-col :span="12">
              <el-form-item label="密码字段">
                <el-input v-model="syncForm.password_field" placeholder="password" />
              </el-form-item>
            </el-col>
          </el-row>
          <el-divider content-position="left">定时同步</el-divider>
          <el-row :gutter="12">
            <el-col :span="8">
              <el-form-item label="Cron 表达式">
                <el-input v-model="syncForm.cron_expr" placeholder="0 */30 * * * *" />
              </el-form-item>
            </el-col>
            <el-col :span="8">
              <el-form-item label="启用">
                <el-switch v-model="syncForm.enabled" />
              </el-form-item>
            </el-col>
          </el-row>
        </el-form>
        <div v-if="syncForm" style="display: flex; gap: 8px; justify-content: flex-end;">
          <el-button @click="syncForm = null">取消编辑</el-button>
          <el-button type="primary" :loading="savingSync" @click="handleSaveSync">保存</el-button>
        </div>

        <!-- Sync Logs -->
        <div v-if="selectedSyncId" style="margin-top: 8px;">
          <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 8px;">
            <span style="font-weight: 600; font-size: 13px; color: var(--text-primary);">同步日志</span>
            <el-button size="small" text @click="loadLogs(selectedSyncId)">
              <el-icon><Refresh /></el-icon> 刷新
            </el-button>
          </div>
          <el-timeline v-if="logs.length > 0">
            <el-timeline-item
              v-for="log in logs" :key="log.id"
              :timestamp="log.created_at"
              :type="log.status === 'success' ? 'success' : log.status === 'partial' ? 'warning' : 'danger'"
            >
              <span style="font-size: 13px;">{{ log.message }}</span>
            </el-timeline-item>
          </el-timeline>
          <div v-else style="text-align: center; color: var(--text-tertiary); font-size: 13px; padding: 16px;">暂无同步日志</div>
        </div>
      </div>
    </el-dialog>

  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, watch } from 'vue'
import dayjs from 'dayjs'
import { ElMessage, ElMessageBox } from 'element-plus'
import type { FormInstance } from 'element-plus'
import { getDataSources, createDataSource, updateDataSource, deleteDataSource, importDatasources, applyTemplate, getTemplates, getNotifications, triggerInspect, getInspectTask, testDataSource, getSyncSources, createSyncSource, updateSyncSource, deleteSyncSource, triggerSync, getSyncLogs } from '../api'
import type { DataSource, SyncSource } from '../types'

const loading = ref(false)
const saving = ref(false)
const importing = ref(false)
const datasources = ref<DataSource[]>([])
const templates = ref<any[]>([])
const notifChannels = ref<any[]>([])
const dialogVisible = ref(false)
const importVisible = ref(false)
const editingId = ref<number | null>(null)
const formRef = ref<FormInstance>()
const yamlContent = ref('')
const form = ref<DataSource>({ name: '', url: '', username: '', password: '' })
const selectedChannels = ref<number[]>([])
const rules = {
  name: [{ required: true, message: '请输入名称', trigger: 'blur' }],
  url: [{ required: true, message: '请输入 URL', trigger: 'blur' }],
}

// Sync source state
const syncDialogVisible = ref(false)
const syncSources = ref<SyncSource[]>([])
const selectedSyncId = ref<number | null>(null)
const syncForm = ref<SyncSource | null>(null)
const savingSync = ref(false)
const syncing = ref(false)
const logs = ref<any[]>([])

function selectSync(id: number) {
  selectedSyncId.value = id
  const ss = syncSources.value.find(s => s.id === id)
  syncForm.value = ss ? { ...ss, auth_password: '', auth_token: '' } : null
  loadLogs(id)
}

function editSync(ss: SyncSource | null) {
  if (ss) {
    selectedSyncId.value = ss.id!
    syncForm.value = { ...ss, auth_password: '', auth_token: '' }
    loadLogs(ss.id!)
  } else {
    selectedSyncId.value = null
    syncForm.value = { name: '', url: '', method: 'GET', headers: '', body: '', auth_type: 'none', auth_username: '', auth_password: '', auth_token: '', data_path: '', name_field: 'name', url_field: '', username_field: '', password_field: '', cron_expr: '', enabled: true }
  }
}

async function loadSyncSources() {
  try {
    const res = await getSyncSources()
    syncSources.value = res.data
  } catch { /* ignore */ }
}

async function loadLogs(id: number) {
  try {
    const res = await getSyncLogs(id)
    logs.value = res.data
  } catch { /* ignore */ }
}

async function handleSaveSync() {
  if (!syncForm.value) return
  if (!syncForm.value.name || !syncForm.value.url || !syncForm.value.name_field) {
    ElMessage.warning('名称、URL、名称字段不能为空')
    return
  }
  savingSync.value = true
  try {
    if (syncForm.value.id) {
      await updateSyncSource(syncForm.value.id, syncForm.value)
      ElMessage.success('更新成功')
    } else {
      const res = await createSyncSource(syncForm.value)
      selectedSyncId.value = res.data.id!
      ElMessage.success('创建成功')
    }
    await loadSyncSources()
  } catch (e: any) {
    ElMessage.error(e.message)
  } finally {
    savingSync.value = false
  }
}

async function handleDeleteSync(id: number) {
  try {
    await ElMessageBox.confirm('确定删除此同步源？', '确认')
    await deleteSyncSource(id)
    if (selectedSyncId.value === id) {
      selectedSyncId.value = null
      syncForm.value = null
      logs.value = []
    }
    await loadSyncSources()
    ElMessage.success('已删除')
  } catch { /* ignore */ }
}

async function handleTriggerSync() {
  if (!selectedSyncId.value) return
  syncing.value = true
  try {
    await triggerSync(selectedSyncId.value)
    ElMessage.success('同步任务已启动，请稍后查看日志')
    setTimeout(() => loadLogs(selectedSyncId.value!), 3000)
  } catch (e: any) {
    ElMessage.error(e.message)
  } finally {
    syncing.value = false
  }
}

watch(syncDialogVisible, (v) => {
  if (v) {
    loadSyncSources()
    selectedSyncId.value = null
    syncForm.value = null
    logs.value = []
  }
})

async function fetchData() {
  loading.value = true
  try {
    const [dsRes, tmplRes, notifRes] = await Promise.all([getDataSources(), getTemplates(), getNotifications()])
    datasources.value = dsRes.data
    templates.value = tmplRes.data
    notifChannels.value = notifRes.data
  } finally { loading.value = false }
}

function channelNames(json: string) {
  try {
    const ids = JSON.parse(json) as number[]
    return ids.map(id => notifChannels.value.find(c => c.id === id)?.name || `ID: ${id}`).join(', ')
  } catch { return '' }
}

function templateName(id: number | null | undefined) {
  if (!id) return ''
  const t = templates.value.find(x => x.id === id)
  return t ? t.name : `ID: ${id}`
}

function openCreate() {
  editingId.value = null
  form.value = { name: '', url: '', username: '', password: '' }
  selectedChannels.value = []
  dialogVisible.value = true
}

function openEdit(row: DataSource) {
  editingId.value = row.id!
  form.value = { ...row, password: '' }
  selectedChannels.value = row.notify_channels ? JSON.parse(row.notify_channels) : []
  dialogVisible.value = true
}

async function handleSave() {
  const valid = await formRef.value?.validate().catch(() => false)
  if (!valid) return
  saving.value = true
  try {
    const payload = {
      ...form.value,
      notify_channels: selectedChannels.value.length ? JSON.stringify(selectedChannels.value) : '',
    }
    if (editingId.value) {
      await updateDataSource(editingId.value, payload)
      ElMessage.success('更新成功')
    } else {
      await createDataSource(payload)
      ElMessage.success('创建成功')
    }
    dialogVisible.value = false
    await fetchData()
  } catch (e: any) {
    ElMessage.error(e.message)
  } finally { saving.value = false }
}

async function testConnectivity(row: DataSource) {
  try {
    const res = await testDataSource(row.id!)
    if (res.data.success) {
      ElMessage.success('连接成功')
    } else {
      ElMessage.error(res.data.message || '连接失败')
    }
  } catch (e: any) {
    ElMessage.error(e.message)
  }
}

async function inspectDS(row: DataSource) {
  try {
    const res = await triggerInspect({ datasource_id: row.id })
    const taskId = res.data.task_id
    if (!taskId) { ElMessage.error('巡检启动失败'); return }

    ElMessage.info(`巡检已开始，正在等待结果...`)
    for (let i = 0; i < 120; i++) {
      await new Promise(r => setTimeout(r, 2000))
      const taskRes = await getInspectTask(taskId)
      const task = taskRes.data
      if (task.status === 'completed') {
        ElMessage.success('巡检完成')
        if (task.report_url) window.open(task.report_url, '_blank')
        return
      }
      if (task.status === 'failed') {
        ElMessage.error(task.error || '巡检失败')
        return
      }
    }
    ElMessage.error('巡检超时')
  } catch (e: any) {
    ElMessage.error(e.message)
  }
}

async function handleMore(row: DataSource, cmd: string) {
  switch (cmd) {
    case 'delete':
      try {
        await ElMessageBox.confirm(`确定删除数据源「${row.name}」？`, '确认删除', { type: 'warning', cancelButtonText: '取消', confirmButtonText: '删除' })
        await deleteDataSource(row.id!)
        ElMessage.success('删除成功')
        await fetchData()
      } catch { /* ignore */ }
      break
    case 'template':
      try {
        const res = await applyTemplate(row.id!)
        ElMessage.success(res.data.message || '模板应用成功')
        await fetchData()
      } catch (e: any) { ElMessage.error(e.message) }
      break
  }
}

async function handleImport() {
  if (!yamlContent.value.trim()) { ElMessage.warning('请输入 YAML 内容'); return }
  importing.value = true
  try {
    const res = await importDatasources(yamlContent.value)
    ElMessage.success(res.data.message || '导入成功')
    importVisible.value = false
    yamlContent.value = ''
    await fetchData()
  } catch (e: any) { ElMessage.error(e.message) }
  finally { importing.value = false }
}

onMounted(fetchData)
</script>
