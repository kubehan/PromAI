<template>
  <div class="page-container">
    <div class="page-header">
      <h2><el-icon><Setting /></el-icon> 系统设置</h2>
      <p>配置系统运行参数和计划任务</p>
    </div>

    <div class="form-section">
      <h3><el-icon :size="16" color="#00d4ff"><Tools /></el-icon> 基本设置</h3>
      <el-form :model="form" label-width="160px" style="max-width: 600px;">
        <el-form-item label="项目名称">
          <el-input v-model="form.project_name" placeholder="巡检报告" />
        </el-form-item>
        <el-form-item label="定时巡检表达式">
          <el-input v-model="form.cron_schedule" placeholder="Cron 表达式">
            <template #append>
              <el-tooltip content="分 时 日 月 周" placement="top">
                <el-icon><QuestionFilled /></el-icon>
              </el-tooltip>
            </template>
          </el-input>
          <div style="color: var(--text-tertiary); font-size: 12px; margin-top: 6px;">默认：00 08,17 * * *（每天早上 8 点和下午 5 点）</div>
        </el-form-item>
      </el-form>
    </div>

    <div class="form-section">
      <h3><el-icon :size="16" color="#f59e0b"><Delete /></el-icon> 报告清理设置</h3>
      <el-form :model="form" label-width="160px" style="max-width: 600px;">
        <el-form-item label="启用自动清理">
          <el-switch v-model="reportCleanupEnabled" />
        </el-form-item>
        <el-form-item label="清理周期">
          <el-input v-model="form.report_cleanup_cron" placeholder="0 0 * * *" />
          <div style="color: var(--text-tertiary); font-size: 12px; margin-top: 6px;">默认：0 0 * * *（每天凌晨）</div>
        </el-form-item>
        <el-form-item label="保留天数">
          <el-input-number v-model="reportCleanupMaxAge" :min="1" :max="365" style="width: 100%;" />
        </el-form-item>
      </el-form>
    </div>

    <el-button type="primary" size="large" :loading="saving" @click="handleSave" style="height: 48px; padding: 0 40px; font-size: 15px; margin-left: 160px;">
      <el-icon><Check /></el-icon> 保存设置
    </el-button>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { getSettings, updateSettings } from '../api'

const saving = ref(false)
const form = ref<Record<string, string>>({
  project_name: '', cron_schedule: '', report_cleanup_cron: '0 0 * * *',
  report_cleanup_enabled: 'true', report_cleanup_max_age: '7',
})

const reportCleanupEnabled = computed({
  get: () => form.value.report_cleanup_enabled === 'true',
  set: (v: boolean) => { form.value.report_cleanup_enabled = String(v) },
})
const reportCleanupMaxAge = computed({
  get: () => parseInt(form.value.report_cleanup_max_age || '7'),
  set: (v: number) => { form.value.report_cleanup_max_age = String(v) },
})

async function fetchData() {
  try { const res = await getSettings(); Object.assign(form.value, res.data) }
  catch (e: any) { ElMessage.error(e.message) }
}

async function handleSave() {
  saving.value = true
  try {
    await updateSettings({
      project_name: form.value.project_name, cron_schedule: form.value.cron_schedule,
      report_cleanup_cron: form.value.report_cleanup_cron,
      report_cleanup_enabled: form.value.report_cleanup_enabled,
      report_cleanup_max_age: form.value.report_cleanup_max_age,
    })
    ElMessage.success('设置已保存')
  } catch (e: any) { ElMessage.error(e.message) }
  finally { saving.value = false }
}

onMounted(fetchData)
</script>
