<template>
  <div class="page-container">
    <div class="page-header">
      <h2><el-icon><Monitor /></el-icon> 巡检记录</h2>
      <p>查看所有巡检任务的执行历史</p>
    </div>

    <div class="section-card">
      <div class="section-header">
        <h3><el-icon :size="16" color="#00d4ff"><List /></el-icon> 任务列表</h3>
        <div style="display: flex; gap: 8px; align-items: center;">
          <el-input v-model="keyword" placeholder="搜索目标/消息" clearable style="width: 200px;" @keyup.enter="fetchRecords" @clear="fetchRecords" />
          <el-select v-model="statusFilter" placeholder="状态" clearable style="width: 110px;" @change="fetchRecords">
            <el-option label="运行中" value="running" />
            <el-option label="已完成" value="completed" />
            <el-option label="失败" value="failed" />
          </el-select>
        </div>
      </div>
      <el-table :data="records" v-loading="loading" stripe>
        <el-table-column prop="task_id" label="任务 ID" width="200" />
        <el-table-column label="状态" width="100">
          <template #default="{ row }">
            <el-tag v-if="row.status === 'completed'" type="success" effect="dark">完成</el-tag>
            <el-tag v-else-if="row.status === 'running'" type="warning" effect="dark">运行中</el-tag>
            <el-tag v-else type="danger" effect="dark">失败</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="datasource_name" label="巡检目标" min-width="200" />
        <el-table-column prop="message" label="消息" min-width="200" show-overflow-tooltip />
        <el-table-column label="开始时间" width="170">
          <template #default="{ row }">{{ dayjs(row.started_at).format('MM-DD HH:mm:ss') }}</template>
        </el-table-column>
        <el-table-column label="完成时间" width="170">
          <template #default="{ row }">
            <span v-if="row.completed_at">{{ dayjs(row.completed_at).format('MM-DD HH:mm:ss') }}</span>
            <span v-else style="color: var(--text-tertiary);">-</span>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="120" fixed="right">
          <template #default="{ row }">
            <el-button v-if="row.report_url" size="small" text style="color: var(--cyan);" @click="viewReport(row)">查看报告</el-button>
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
          @change="fetchRecords"
        />
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import dayjs from 'dayjs'
import { ElMessage } from 'element-plus'
import { getInspectRecords } from '../api'

interface InspectRecord {
  id: number
  task_id: string
  status: string
  datasource_name: string
  message: string
  error: string
  report_url: string
  started_at: string
  completed_at: string | null
}

const loading = ref(false)
const records = ref<InspectRecord[]>([])
const page = ref(1)
const pageSize = ref(20)
const total = ref(0)
const keyword = ref('')
const statusFilter = ref('')

async function fetchRecords() {
  loading.value = true
  try {
    const params: any = { page: page.value, page_size: pageSize.value }
    if (keyword.value) params.keyword = keyword.value
    if (statusFilter.value) params.status = statusFilter.value
    const res = await getInspectRecords(params)
    records.value = res.data.items
    total.value = res.data.total
  } catch (e: any) {
    ElMessage.error(e.message)
  } finally {
    loading.value = false
  }
}

function viewReport(row: InspectRecord) {
  window.open('/api/promai/reports/' + row.report_url.split('/').pop(), '_blank')
}

onMounted(fetchRecords)
</script>
