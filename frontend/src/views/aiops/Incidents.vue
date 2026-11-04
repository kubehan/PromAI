<template>
  <div class="page-container">
    <div class="page-header">
      <h2>故障管理</h2>
      <p>故障单全生命周期、状态流转、MTTA/MTTR</p>
    </div>
    <el-card class="section-card">
      <div style="margin-bottom:12px">
        <el-button type="primary" size="small" @click="create">新建故障单</el-button>
      </div>
      <el-table :data="items" stripe v-loading="loading">
        <el-table-column prop="incident_no" label="故障单号" width="160" />
        <el-table-column prop="title" label="标题" min-width="200" />
        <el-table-column prop="severity" label="严重度" width="100" />
        <el-table-column prop="status" label="状态" width="120" />
        <el-table-column prop="assignee_name" label="责任人" width="120" />
        <el-table-column label="MTTA/MTTR" width="160">
          <template #default="{row}">
            <span>{{ row.mtta_seconds ? row.mtta_seconds+'s' : '-' }} / {{ row.mttr_seconds ? row.mttr_seconds+'s' : '-' }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="created_at" label="创建时间" width="180" />
        <el-table-column label="操作" width="200">
          <template #default="{row}">
            <el-button size="small" text @click="ack(row)" v-if="row.status==='open'">确认</el-button>
            <el-button size="small" text @click="resolve(row)" v-if="row.status!=='resolved' && row.status!=='closed'">解决</el-button>
            <el-button size="small" text @click="close(row)" v-if="row.status==='resolved'">关闭</el-button>
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
import { ElMessage, ElMessageBox } from 'element-plus'

const items = ref<any[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(50)
const loading = ref(false)

async function load() {
  loading.value = true
  try {
    const res = await axios.get('/api/promai/aiops/incidents', { params: { page: page.value, page_size: pageSize.value } })
    items.value = res.data.items || []
    total.value = res.data.total || 0
  } catch (e: any) {
    ElMessage.error(e.message || '加载失败')
  } finally {
    loading.value = false
  }
}

async function create() {
  try {
    const { value } = await ElMessageBox.prompt('请输入故障标题', '新建故障单', { confirmButtonText: '创建', cancelButtonText: '取消' })
    if (!value) return
    await axios.post('/api/promai/aiops/incidents', { title: value, severity: 'warning' })
    ElMessage.success('创建成功')
    load()
  } catch (e: any) {
    if (e !== 'cancel') ElMessage.error(e.message || '创建失败')
  }
}

async function ack(row: any) {
  await axios.post(`/api/promai/aiops/incidents/${row.id}/ack`)
  ElMessage.success('已确认')
  load()
}
async function resolve(row: any) {
  await axios.post(`/api/promai/aiops/incidents/${row.id}/resolve`)
  ElMessage.success('已解决')
  load()
}
async function close(row: any) {
  await axios.post(`/api/promai/aiops/incidents/${row.id}/close`)
  ElMessage.success('已关闭')
  load()
}

onMounted(load)
</script>
