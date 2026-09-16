<template>
  <div class="page-container">
    <div class="page-header">
      <h2><el-icon><Document /></el-icon> 报告管理</h2>
      <p>查看、在线编辑和导出历史巡检报告</p>
    </div>

    <div class="section-card">
      <div class="section-header">
        <h3><el-icon :size="16" :color="getCssVar('--cyan')"><List /></el-icon> 历史报告</h3>
        <div style="display: flex; gap: 8px; align-items: center;">
          <el-input v-model="keyword" placeholder="搜索标题/数据源" clearable style="width: 200px;" @keyup.enter="fetchData" @clear="fetchData" />
          <el-select v-model="statusFilter" placeholder="状态" clearable style="width: 110px;" @change="fetchData">
            <el-option label="正常" value="success" />
            <el-option label="告警" value="warning" />
            <el-option label="高危" value="danger" />
          </el-select>
          <el-button plain @click="fetchData" :loading="loading"><el-icon><Refresh /></el-icon> 刷新</el-button>
        </div>
      </div>
      <el-table :data="reports" v-loading="loading" stripe>
        <el-table-column type="index" label="#" width="56" />
        <el-table-column prop="title" label="报告名称" min-width="220">
          <template #default="{ row }">
            <span style="font-weight: 600; color: var(--text-primary);">{{ row.title }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="datasource_name" label="数据源" min-width="180" />
        <el-table-column label="指标" width="80" align="center">
          <template #default="{ row }">
            <span style="color: var(--cyan); font-weight: 700;">{{ row.total_metrics }}</span>
          </template>
        </el-table-column>
        <el-table-column label="告警" width="80" align="center">
          <template #default="{ row }">
            <span v-if="row.alert_count > 0" style="color: var(--red); font-weight: 700;">{{ row.alert_count }}</span>
            <span v-else style="color: var(--emerald);">0</span>
          </template>
        </el-table-column>
        <el-table-column label="状态" width="100" align="center">
          <template #default="{ row }">
            <span :class="['status-badge', row.status]">
              {{ row.status === 'success' ? '正常' : row.status === 'danger' ? '高危' : '告警' }}
            </span>
          </template>
        </el-table-column>
        <el-table-column label="大小" width="100" align="right">
          <template #default="{ row }">
            <span style="color: var(--text-tertiary);">{{ (row.file_size / 1024).toFixed(1) }} KB</span>
          </template>
        </el-table-column>
        <el-table-column label="时间" width="170">
          <template #default="{ row }">{{ dayjs(row.created_at).format('YYYY-MM-DD HH:mm') }}</template>
        </el-table-column>
        <el-table-column label="操作" width="300" fixed="right">
          <template #default="{ row }">
            <el-button size="small" text @click="viewReport(row)" style="color: var(--cyan);">查看</el-button>
            <el-button size="small" text @click="editReport(row)" style="color: var(--cyan);">编辑</el-button>
            <el-dropdown split-button size="small" type="primary" plain @command="(cmd: string) => handleExport(row, cmd)">
              <el-icon><Download /></el-icon>&nbsp;导出
              <template #dropdown>
                <el-dropdown-menu>
                  <el-dropdown-item command="docx"><el-icon><Document /></el-icon> Word 文档 (.docx)</el-dropdown-item>
                  <el-dropdown-item command="md"><el-icon><Tickets /></el-icon> Markdown (.md)</el-dropdown-item>
                </el-dropdown-menu>
              </template>
            </el-dropdown>
            <el-button size="small" text @click="handleDelete(row)" style="color: var(--red);">删除</el-button>
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
      <el-empty v-if="!loading && reports.length === 0" description="暂无报告" :image-size="60" />
    </div>

    <el-dialog
      v-model="editVisible"
      title="编辑报告内容（Markdown）"
      width="92%"
      top="5vh"
      destroy-on-close
      @closed="editClosed"
    >
      <div style="display: flex; gap: 12px; height: 66vh;">
        <div style="flex: 1; display: flex; flex-direction: column; min-width: 0;">
          <div class="editor-toolbar">
            <span>编辑</span>
            <span style="color: var(--text-tertiary); margin-left: 8px;">支持 Markdown 语法（标题 / 表格 / 列表 / 加粗等）</span>
          </div>
          <el-input
            v-model="editingContent"
            type="textarea"
            :autosize="false"
            resize="none"
            spellcheck="false"
            class="md-editor-input"
            style="flex: 1;"
          />
        </div>
        <div style="flex: 1; display: flex; flex-direction: column; min-width: 0; border: 1px solid var(--border-color); border-radius: 6px; overflow: auto;">
          <div class="editor-toolbar"><span>预览</span></div>
          <div class="md-preview" v-html="renderPreview"></div>
        </div>
      </div>
      <template #footer>
        <div style="display: flex; align-items: center; justify-content: space-between;">
          <span style="color: var(--text-tertiary); font-size: 12px;">
            编辑内容将用于 MD / Word 导出，导出前记得先保存。
          </span>
          <div>
            <el-button @click="editVisible = false">取消</el-button>
            <el-button type="primary" :loading="saving" @click="saveReport">
              <el-icon><Check /></el-icon> 保存内容
            </el-button>
          </div>
        </div>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import dayjs from 'dayjs'
import { ElMessage, ElMessageBox } from 'element-plus'
import { marked } from 'marked'
import { getReports, deleteReport, getReportContent, saveReportContent, exportReportFile } from '../api'
import type { ReportRecord } from '../types'

marked.setOptions({ breaks: true, gfm: true })

function getCssVar(name: string): string {
  return getComputedStyle(document.documentElement).getPropertyValue(name).trim()
}

const loading = ref(false)
const reports = ref<ReportRecord[]>([])
const page = ref(1)
const pageSize = ref(20)
const total = ref(0)
const keyword = ref('')
const statusFilter = ref('')

async function fetchData() {
  loading.value = true
  try {
    const params: any = { page: page.value, page_size: pageSize.value }
    if (keyword.value) params.keyword = keyword.value
    if (statusFilter.value) params.status = statusFilter.value
    const res = await getReports(params)
    reports.value = res.data.items
    total.value = res.data.total
  } catch (e: any) { ElMessage.error(e.message) }
  finally { loading.value = false }
}

function viewReport(row: ReportRecord) {
  const filename = row.file_path?.replace(/^reports\//, '')
  window.open('/api/promai/reports/' + filename, '_blank')
}

async function handleDelete(row: ReportRecord) {
  try {
    await ElMessageBox.confirm(`确定删除「${row.title}」？`, '确认删除', { type: 'warning', cancelButtonText: '取消', confirmButtonText: '删除' })
    await deleteReport(row.id!); ElMessage.success('删除成功'); await fetchData()
  } catch { /* ignore */ }
}

async function handleExport(row: ReportRecord, format: string) {
  try {
    const res = await exportReportFile(row.id!, format as 'md' | 'docx')
    const blob = res.data as Blob
    const ext = format === 'docx' ? '.docx' : '.md'
    const base = (row.title || 'inspection_report').replace(/[\\/:*?"<>|]/g, '_')
    downloadBlob(blob, base + ext)
    ElMessage.success(`已导出 ${format === 'docx' ? 'Word' : 'Markdown'} 文件`)
  } catch (e: any) {
    ElMessage.error(e.message)
  }
}

function downloadBlob(blob: Blob, filename: string) {
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = filename
  document.body.appendChild(a)
  a.click()
  document.body.removeChild(a)
  URL.revokeObjectURL(url)
}

// ===== 在线编辑 =====
const editVisible = ref(false)
const editingContent = ref('')
const saving = ref(false)
const currentEditing = ref<ReportRecord | null>(null)

const renderPreview = ref('')

async function editReport(row: ReportRecord) {
  currentEditing.value = row
  editVisible.value = true
  editingContent.value = '加载中...'
  renderPreview.value = ''
  try {
    const res = await getReportContent(row.id!)
    editingContent.value = res.data.content
    renderPreview.value = marked.parse(editingContent.value) as string
  } catch (e: any) {
    editVisible.value = false
    ElMessage.error(e.message)
  }
}

function editClosed() {
  currentEditing.value = null
  editingContent.value = ''
  renderPreview.value = ''
}

async function saveReport() {
  if (!currentEditing.value) return
  saving.value = true
  try {
    await saveReportContent(currentEditing.value.id!, editingContent.value)
    ElMessage.success('报告内容已保存')
    editVisible.value = false
  } catch (e: any) {
    ElMessage.error(e.message)
  } finally {
    saving.value = false
  }
}

onMounted(fetchData)
</script>

<style scoped>
.editor-toolbar {
  height: 34px;
  line-height: 34px;
  padding: 0 12px;
  font-size: 13px;
  font-weight: 600;
  background: var(--bg-elevated, #f5f7fb);
  border-bottom: 1px solid var(--border-color);
  border-radius: 6px 6px 0 0;
  flex: none;
}
.md-editor-input :deep(.el-textarea__inner) {
  height: 100% !important;
  font-family: 'SFMono-Regular', Consolas, 'Liberation Mono', Menlo, monospace;
  font-size: 13px;
  line-height: 1.6;
  border-radius: 0 0 0 6px;
}
.md-preview {
  flex: 1;
  padding: 12px 16px;
  overflow: auto;
  line-height: 1.7;
}
.md-preview :deep(h1) { font-size: 22px; margin: 8px 0 12px; }
.md-preview :deep(h2) { font-size: 18px; margin: 12px 0 8px; }
.md-preview :deep(h3) { font-size: 15px; margin: 10px 0 6px; }
.md-preview :deep(p) { margin: 6px 0; }
.md-preview :deep(table) { border-collapse: collapse; width: 100%; margin: 8px 0; }
.md-preview :deep(th), .md-preview :deep(td) { border: 1px solid #dcdfe6; padding: 6px 10px; text-align: left; }
.md-preview :deep(th) { background: #f5f7fa; font-weight: 600; }
.md-preview :deep(li) { margin: 2px 0; }
.md-preview :deep(blockquote) { border-left: 3px solid var(--cyan); margin: 8px 0; padding: 4px 12px; color: var(--text-tertiary); background: #f7f8fa; }
.md-preview :deep(pre) { background: #f6f8fa; padding: 10px 12px; border-radius: 6px; overflow: auto; }
.md-preview :deep(code) { background: #f6f8fa; padding: 2px 5px; border-radius: 3px; font-size: 90%; }
.md-preview :deep(pre code) { background: none; padding: 0; }
</style>