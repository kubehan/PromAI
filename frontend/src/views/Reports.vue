<template>
  <div class="page-container">
    <div class="page-header">
      <h2><el-icon><Document /></el-icon> 报告管理</h2>
      <p>查看和管理历史巡检报告</p>
    </div>

    <div class="section-card">
      <div class="section-header">
        <h3><el-icon :size="16" color="#00d4ff"><List /></el-icon> 历史报告</h3>
        <el-button plain @click="fetchData" :loading="loading"><el-icon><Refresh /></el-icon> 刷新</el-button>
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
        <el-table-column label="操作" width="140" fixed="right">
          <template #default="{ row }">
            <el-button size="small" text @click="viewReport(row)" style="color: var(--cyan);">查看</el-button>
            <el-button size="small" text @click="handleDelete(row)" style="color: var(--red);">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
      <el-empty v-if="!loading && reports.length === 0" description="暂无报告" :image-size="60" />
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import dayjs from 'dayjs'
import { ElMessage, ElMessageBox } from 'element-plus'
import { getReports, deleteReport } from '../api'
import type { ReportRecord } from '../types'

const loading = ref(false)
const reports = ref<ReportRecord[]>([])

async function fetchData() {
  loading.value = true
  try { const res = await getReports(); reports.value = res.data }
  catch (e: any) { ElMessage.error(e.message) }
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

onMounted(fetchData)
</script>
