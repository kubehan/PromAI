<template>
  <div class="page-container">
    <div class="page-header">
      <h2><el-icon><Setting /></el-icon> 系统设置</h2>
      <p>配置系统运行参数和计划任务</p>
    </div>

    <div class="form-section">
        <h3><el-icon :size="16" :color="getCssVar('--cyan')"><Tools /></el-icon> 基本设置</h3>
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

    <div class="form-section">
        <h3><el-icon :size="16" :color="getCssVar('--cyan')"><Brush /></el-icon> 主题设置</h3>
      <div class="theme-grid">
        <div
          v-for="t in themeOptions"
          :key="t.value"
          class="theme-card"
          :class="{ active: currentTheme === t.value }"
          @click="setTheme(t.value)"
        >
          <div class="theme-preview" :class="'theme-preview--' + t.value">
            <div class="preview-sidebar"></div>
            <div class="preview-main">
              <div class="preview-header"></div>
              <div class="preview-card"></div>
              <div class="preview-card short"></div>
            </div>
          </div>
          <div class="theme-info">
            <span class="theme-name">{{ t.icon }} {{ t.label }}</span>
            <span class="theme-desc">{{ t.desc }}</span>
          </div>
          <div v-if="currentTheme === t.value" class="theme-check">
            <el-icon color="#fff" :size="14"><Check /></el-icon>
          </div>
        </div>
      </div>
    </div>

    <div class="form-section">
      <h3><el-icon :size="16" color="#7c3aed"><MagicStick /></el-icon> AI 智能助手配置</h3>

      <el-form :model="form" label-width="160px" style="max-width: 600px;">
        <el-form-item label="启用 AI 助手">
          <el-switch v-model="aiEnabled" />
        </el-form-item>
        <el-form-item label="默认模型" v-if="modelList.length > 0">
          <el-select v-model="form.ai_default_model" style="width: 100%;">
            <el-option v-for="m in modelList" :key="m.name" :label="m.name" :value="m.name" />
          </el-select>
        </el-form-item>
      </el-form>

      <div v-for="(m, idx) in modelList" :key="idx" class="model-card glass-card">
        <div class="model-header">
          <el-icon color="#7c3aed"><MagicStick /></el-icon>
          <span class="model-title">{{ m.name || '未命名模型' }}</span>
          <el-tag v-if="m.name === form.ai_default_model" size="small" type="primary" effect="dark">默认</el-tag>
          <el-button text type="danger" size="small" @click="removeModel(idx)" style="margin-left: auto;">
            <el-icon><Delete /></el-icon>
          </el-button>
        </div>
        <el-form label-width="120px" style="margin-top: 12px;">
          <el-form-item label="名称">
            <el-input v-model="m.name" placeholder="模型标识" />
          </el-form-item>
          <el-form-item label="提供商">
            <el-select v-model="m.provider" style="width: 100%;">
              <el-option label="OpenAI" value="openai" />
              <el-option label="Anthropic" value="anthropic" />
              <el-option label="Ollama" value="ollama" />
            </el-select>
          </el-form-item>
          <el-form-item label="模型">
            <el-input v-model="m.model" placeholder="gpt-4o-mini" />
          </el-form-item>
          <el-form-item label="接口地址">
            <el-input v-model="m.base_url" placeholder="https://api.openai.com/v1" />
          </el-form-item>
          <el-form-item label="API Key">
            <el-input v-model="m.api_key" type="password" show-password placeholder="留空则不修改" />
            <div style="color: var(--text-tertiary); font-size: 12px; margin-top: 6px;">
              {{ m.api_key && m.api_key !== '********' ? '⭕ 新值待保存' : m.api_key === '********' ? '✅ 已配置' : '⭕ 未配置' }}
            </div>
          </el-form-item>
          <el-form-item label="思考级别">
            <el-select v-model="m.thinking_level" style="width: 100%;">
              <el-option label="关闭" value="off" />
              <el-option label="最低" value="minimal" />
              <el-option label="低" value="low" />
              <el-option label="中等" value="medium" />
              <el-option label="高" value="high" />
              <el-option label="最高" value="xhigh" />
            </el-select>
          </el-form-item>
          <el-form-item label="最大 Token">
            <el-input-number v-model="m.max_tokens" :min="1024" :max="128000" :step="1024" style="width: 100%;" />
          </el-form-item>
          <el-form-item label="代理地址">
            <el-input v-model="m.proxy_url" placeholder="http://proxy.example.com:8080 或 socks5://..." />
            <div style="color: var(--text-tertiary); font-size: 12px; margin-top: 6px;">可选，留空则直连</div>
          </el-form-item>
          <el-form-item label=" ">
            <div class="model-actions">
              <el-button type="success" size="small" plain :loading="m._testing" @click="testModel(idx)">
                <el-icon><MagicStick /></el-icon> 测试连接
              </el-button>
              <span v-if="m._testResult !== undefined" :class="m._testResult ? 'test-pass' : 'test-fail'" style="font-size:13px;">
                {{ m._testResult ? '✅ ' + (m._testMessage || '连接成功') : '❌ ' + (m._testError || '连接失败') }}
              </span>
            </div>
          </el-form-item>
        </el-form>
      </div>

      <el-button type="primary" size="small" plain @click="addModel" style="margin-top: 12px;">
        <el-icon><Plus /></el-icon> 添加模型
      </el-button>
    </div>

    <el-button type="primary" size="large" :loading="saving" @click="handleSave" style="height: 48px; padding: 0 40px; font-size: 15px; margin-left: 160px;">
      <el-icon><Check /></el-icon> 保存设置
    </el-button>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { getSettings, updateSettings, testAiModel } from '../api'
import { useTheme } from '../composables/useTheme'

function getCssVar(name: string): string {
  return getComputedStyle(document.documentElement).getPropertyValue(name).trim()
}

const { currentTheme, setTheme, themeOptions } = useTheme()

interface AIModelForm {
  name: string
  provider: string
  model: string
  base_url: string
  api_key: string
  thinking_level: string
  max_tokens: number
  proxy_url: string
  _testing?: boolean
  _testResult?: boolean
  _testMessage?: string
  _testError?: string
}

const saving = ref(false)
const form = ref<Record<string, string>>({
  project_name: '', cron_schedule: '', report_cleanup_cron: '0 0 * * *',
  report_cleanup_enabled: 'true', report_cleanup_max_age: '7',
  ai_enabled: 'false', ai_default_model: '',
})
const modelList = ref<AIModelForm[]>([])

const reportCleanupEnabled = computed({
  get: () => form.value.report_cleanup_enabled === 'true',
  set: (v: boolean) => { form.value.report_cleanup_enabled = String(v) },
})
const reportCleanupMaxAge = computed({
  get: () => parseInt(form.value.report_cleanup_max_age || '7'),
  set: (v: number) => { form.value.report_cleanup_max_age = String(v) },
})
const aiEnabled = computed({
  get: () => form.value.ai_enabled === 'true',
  set: (v: boolean) => { form.value.ai_enabled = String(v) },
})

function defaultModel(): AIModelForm {
  return {
    name: '', provider: 'openai', model: 'gpt-4o-mini',
    base_url: 'https://api.openai.com/v1', api_key: '',
    thinking_level: 'medium', max_tokens: 16384, proxy_url: '',
  }
}

function addModel() {
  const m = defaultModel()
  m.name = `model-${modelList.value.length + 1}`
  modelList.value.push(m)
}

function removeModel(idx: number) {
  const removed = modelList.value[idx]
  modelList.value.splice(idx, 1)
  if (form.value.ai_default_model === removed.name && modelList.value.length > 0) {
    form.value.ai_default_model = modelList.value[0].name
  }
}

async function fetchData() {
  try {
    const res = await getSettings()
    Object.assign(form.value, res.data)
    if (res.data.ai_models) {
      try { modelList.value = JSON.parse(res.data.ai_models) } catch {}
    }
    // 初始化默认模型
    if (modelList.value.length > 0 && !form.value.ai_default_model) {
      form.value.ai_default_model = modelList.value[0].name
    }
  } catch (e: any) { ElMessage.error(e.message) }
}

async function testModel(idx: number) {
  const m = modelList.value[idx]
  if (!m.name || !m.model || !m.base_url) {
    ElMessage.warning('请先填写名称、模型名称和接口地址')
    return
  }
  m._testing = true
  m._testResult = undefined
  m._testError = undefined
  try {
    const res = await testAiModel({
      name: m.name, provider: m.provider, model: m.model,
      base_url: m.base_url, api_key: m.api_key,
      thinking_level: m.thinking_level, max_tokens: m.max_tokens,
      proxy_url: m.proxy_url,
    })
    m._testResult = res.data.success
    m._testMessage = res.data.message
    m._testError = res.data.error
  } catch (e: any) {
    m._testResult = false
    m._testError = e.message
  } finally {
    m._testing = false
  }
}

async function handleSave() {
  saving.value = true
  try {
    const payload: Record<string, string> = {
      project_name: form.value.project_name, cron_schedule: form.value.cron_schedule,
      report_cleanup_cron: form.value.report_cleanup_cron,
      report_cleanup_enabled: form.value.report_cleanup_enabled,
      report_cleanup_max_age: form.value.report_cleanup_max_age,
      ai_enabled: form.value.ai_enabled,
      ai_default_model: form.value.ai_default_model,
    }
    payload.ai_models = JSON.stringify(modelList.value)
    await updateSettings(payload)
    ElMessage.success('设置已保存')
    fetchData()
  } catch (e: any) { ElMessage.error(e.message) }
  finally { saving.value = false }
}

onMounted(fetchData)
</script>

<style scoped>
.model-card {
  margin-top: 16px;
  padding: 20px;
  border-radius: 12px;
}
.model-header {
  display: flex;
  align-items: center;
  gap: 8px;
  padding-bottom: 12px;
  border-bottom: 1px solid rgba(255,255,255,0.06);
}
.model-title {
  font-weight: 600;
  font-size: 15px;
  color: var(--text-primary);
}
.model-actions {
  display: flex;
  align-items: center;
  gap: 12px;
}
.test-pass {
  color: var(--emerald, #10b981);
}
.test-fail {
  color: var(--red, #ef4444);
}

.theme-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
  gap: 16px;
  max-width: 860px;
}

.theme-card {
  position: relative;
  border: 2px solid var(--border);
  border-radius: 12px;
  overflow: hidden;
  cursor: pointer;
  transition: all 0.25s ease;
  background: var(--bg-primary);
}

.theme-card:hover {
  border-color: var(--text-tertiary);
  transform: translateY(-2px);
}

.theme-card.active {
  border-color: var(--cyan);
  box-shadow: 0 0 0 2px var(--cyan-dim);
}

.theme-preview {
  display: flex;
  height: 100px;
  overflow: hidden;
}

.theme-preview--dark .preview-sidebar { background: #080c18; }
.theme-preview--dark .preview-main { background: #0d1326; }
.theme-preview--dark .preview-header { background: #1e293b; }
.theme-preview--dark .preview-card { background: #111827; border: 1px solid rgba(56,189,248,0.12); }

.theme-preview--light .preview-sidebar { background: #f1f5f9; }
.theme-preview--light .preview-main { background: #f8fafc; }
.theme-preview--light .preview-header { background: #ffffff; }
.theme-preview--light .preview-card { background: #ffffff; border: 1px solid rgba(59,130,246,0.15); }

.theme-preview--cyber .preview-sidebar { background: #050510; }
.theme-preview--cyber .preview-main { background: #0a0a1a; }
.theme-preview--cyber .preview-header { background: #1f1f40; }
.theme-preview--cyber .preview-card { background: #0f0f24; border: 1px solid rgba(0,255,255,0.15); }

.theme-preview--minimal .preview-sidebar { background: #fafafa; }
.theme-preview--minimal .preview-main { background: #ffffff; }
.theme-preview--minimal .preview-header { background: #ffffff; }
.theme-preview--minimal .preview-card { background: #ffffff; border: 1px solid rgba(0,0,0,0.06); }

.preview-sidebar {
  width: 40px;
  flex-shrink: 0;
}

.preview-main {
  flex: 1;
  padding: 10px;
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.preview-header {
  height: 12px;
  border-radius: 4px;
}

.preview-card {
  height: 24px;
  border-radius: 4px;
}

.preview-card.short {
  width: 60%;
}

.theme-info {
  padding: 10px 12px;
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.theme-name {
  font-size: 14px;
  font-weight: 600;
  color: var(--text-primary);
}

.theme-desc {
  font-size: 12px;
  color: var(--text-tertiary);
}

.theme-check {
  position: absolute;
  top: 6px;
  right: 6px;
  width: 22px;
  height: 22px;
  border-radius: 50%;
  background: var(--cyan);
  display: flex;
  align-items: center;
  justify-content: center;
}
</style>
