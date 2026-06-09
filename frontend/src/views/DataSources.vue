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
        <el-table-column label="创建时间" width="170">
          <template #default="{ row }">{{ dayjs(row.created_at).format('MM-DD HH:mm') }}</template>
        </el-table-column>
        <el-table-column label="操作" width="280" fixed="right">
          <template #default="{ row }">
            <el-button size="small" text @click="openEdit(row)" style="color: var(--cyan);">编辑</el-button>
            <el-button size="small" text @click="inspectDS(row)" style="color: var(--emerald);">巡检</el-button>
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

  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import dayjs from 'dayjs'
import { ElMessage, ElMessageBox } from 'element-plus'
import type { FormInstance } from 'element-plus'
import { getDataSources, createDataSource, updateDataSource, deleteDataSource, importDatasources, applyTemplate, getTemplates, triggerInspect } from '../api'
import type { DataSource } from '../types'

const loading = ref(false)
const saving = ref(false)
const importing = ref(false)
const datasources = ref<DataSource[]>([])
const templates = ref<any[]>([])
const dialogVisible = ref(false)
const importVisible = ref(false)
const editingId = ref<number | null>(null)
const formRef = ref<FormInstance>()
const yamlContent = ref('')
const form = ref<DataSource>({ name: '', url: '', username: '', password: '' })
const rules = {
  name: [{ required: true, message: '请输入名称', trigger: 'blur' }],
  url: [{ required: true, message: '请输入 URL', trigger: 'blur' }],
}

async function fetchData() {
  loading.value = true
  try {
    const [dsRes, tmplRes] = await Promise.all([getDataSources(), getTemplates()])
    datasources.value = dsRes.data
    templates.value = tmplRes.data
  } finally { loading.value = false }
}

function templateName(id: number | null | undefined) {
  if (!id) return ''
  const t = templates.value.find(x => x.id === id)
  return t ? t.name : `ID: ${id}`
}

function openCreate() {
  editingId.value = null
  form.value = { name: '', url: '', username: '', password: '' }
  dialogVisible.value = true
}

function openEdit(row: DataSource) {
  editingId.value = row.id!
  form.value = { ...row }
  dialogVisible.value = true
}

async function handleSave() {
  const valid = await formRef.value?.validate().catch(() => false)
  if (!valid) return
  saving.value = true
  try {
    if (editingId.value) {
      await updateDataSource(editingId.value, form.value)
      ElMessage.success('更新成功')
    } else {
      await createDataSource(form.value)
      ElMessage.success('创建成功')
    }
    dialogVisible.value = false
    await fetchData()
  } catch (e: any) {
    ElMessage.error(e.message)
  } finally { saving.value = false }
}

async function inspectDS(row: DataSource) {
  ElMessage.info(`正在对「${row.name}」执行巡检...`)
  try {
    const res = await triggerInspect({ datasource_id: row.id })
    ElMessage.success('巡检完成')
    if (res.data.url) {
      window.open(res.data.url, '_blank')
    }
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
