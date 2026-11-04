<template>
  <div class="page-container">
    <div class="page-header">
      <h2>AIOps 驾驶舱</h2>
      <p>事件、故障概览</p>
    </div>
    <el-row :gutter="20">
      <el-col :span="6">
        <el-card class="stat-card"><div class="stat-label">活跃事件</div><div class="stat-value">{{ stats.active_events || 0 }}</div></el-card>
      </el-col>
      <el-col :span="6">
        <el-card class="stat-card"><div class="stat-label">未闭环故障</div><div class="stat-value">{{ stats.open_incidents || 0 }}</div></el-card>
      </el-col>
    </el-row>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import axios from 'axios'

const stats = ref<any>({})

onMounted(async () => {
  try {
    const res = await axios.get('/api/promai/aiops/stats/overview')
    stats.value = res.data || {}
  } catch {}
})
</script>
