<template>
  <div class="page-container">
    <div class="page-header">
      <h2>事件中心</h2>
      <p>统一事件接入、过滤与转故障</p>
    </div>
    <el-card class="section-card">
      <el-table :data="items" stripe v-loading="loading">
        <el-table-column prop="event_id" label="事件ID" width="200" />
        <el-table-column prop="title" label="标题" min-width="200">
          <template #default="{row}">
            <span>{{ row.title || row.annotations_json || '-' }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="severity" label="级别" width="100" />
        <el-table-column prop="status" label="状态" width="100" />
        <el-table-column prop="source" label="来源" width="100" />
        <el-table-column prop="created_at" label="时间" width="180" />
        <el-table-column label="操作" width="160">
          <template #default="{row}">
            <el-button size="small" text type="primary" @click="toIncident(row)" :disabled="row.incident_id">转故障</el-button>
          </template>
        </el-table-column>
      </el-table>
      <div style="margin-top:12px">
        <el-pagination background layout="total, prev, pager, next" :total="total" :page-size="pageSize" v-model:current-page="page" @current-change="load" />
      </div>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import axios from 'axios'
import { ElMessage } from 'element-plus'

const items = ref<any[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(50)
const loading = ref(false)

async function load() {
  loading.value = true
  try {
    const res = await axios.get('/api/promai/aiops/events', { params: { page: page.value, page_size: pageSize.value } })
    items.value = res.data.items || []
    total.value = res.data.total || 0
  } catch (e: any) {
    ElMessage.error(e.message || '加载失败')
  } finally {
    loading.value = false
  }
}

async function toIncident(row: any) {
  try {
    await axios.post('/api/promai/aiops/events/to-incident', { event_id: row.id, title: row.title || row.event_id, severity: row.severity || 'warning' })
    ElMessage.success('已转故障')
    load()
  } catch (e: any) {
    ElMessage.error(e.message || '操作失败')
  }
}

onMounted(load)
</script>
