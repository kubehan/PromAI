<template>
  <div class="page-container">
    <div class="page-header">
      <h2><el-icon><Bell /></el-icon> 通知渠道</h2>
      <p>配置告警通知渠道，支持企业微信、钉钉、飞书、邮件</p>
    </div>

    <div class="section-card">
      <div class="section-header">
        <h3><el-icon :size="16" :color="getCssVar('--cyan')"><List /></el-icon> 渠道列表</h3>
        <div style="display: flex; gap: 8px; align-items: center;">
          <el-input v-model="keyword" placeholder="搜索名称" clearable style="width: 200px;" @keyup.enter="fetchData" @clear="fetchData" />
          <el-select v-model="typeFilter" placeholder="类型" clearable style="width: 140px;" @change="fetchData">
            <el-option label="企业微信机器人" value="wechat_work" />
            <el-option label="企业微信应用" value="wechat_app" />
            <el-option label="钉钉机器人" value="dingtalk" />
            <el-option label="飞书机器人" value="feishu" />
            <el-option label="邮件" value="email" />
          </el-select>
          <el-button type="primary" @click="openCreate"><el-icon><Plus /></el-icon> 新增渠道</el-button>
        </div>
      </div>
      <el-table :data="channels" v-loading="loading" stripe>
        <el-table-column type="index" label="#" width="56" />
        <el-table-column label="类型" width="130">
          <template #default="{ row }">
            <el-tag :style="{ background: channelBg(row.channel_type), color: channelColor(row.channel_type), border: 'none' }">
              {{ channelLabel(row.channel_type) }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="name" label="名称" min-width="180">
          <template #default="{ row }">
            <span style="font-weight: 600; color: var(--text-primary);">{{ row.name }}</span>
          </template>
        </el-table-column>
        <el-table-column label="状态" width="90" align="center">
          <template #default="{ row }">
            <el-switch v-model="row.enabled" @change="toggleEnabled(row)" />
          </template>
        </el-table-column>
        <el-table-column label="创建时间" width="170">
          <template #default="{ row }">{{ dayjs(row.created_at).format('MM-DD HH:mm') }}</template>
        </el-table-column>
        <el-table-column label="操作" width="220" fixed="right">
          <template #default="{ row }">
            <el-button size="small" text @click="openEdit(row)" style="color: var(--cyan);">编辑</el-button>
            <el-button size="small" text @click="handleTest(row)" style="color: var(--emerald);">测试</el-button>
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
    </div>

    <el-dialog v-model="dialogVisible" :title="editingId ? '编辑通知渠道' : '新增通知渠道'" width="620" :close-on-click-modal="false">
      <el-form ref="formRef" :model="form" :rules="rules" label-width="120px">
        <el-form-item label="渠道类型" prop="channel_type">
          <el-select v-model="form.channel_type" style="width: 100%" :disabled="!!editingId" @change="onTypeChange">
            <el-option label="企业微信机器人" value="wechat_work" />
            <el-option label="企业微信应用" value="wechat_app" />
            <el-option label="钉钉机器人" value="dingtalk" />
            <el-option label="飞书机器人" value="feishu" />
            <el-option label="邮件" value="email" />
          </el-select>
        </el-form-item>
        <el-form-item label="名称" prop="name">
          <el-input v-model="form.name" placeholder="例如：运维告警群" />
        </el-form-item>
        <el-form-item label="启用">
          <el-switch v-model="form.enabled" />
        </el-form-item>

        <template v-if="form.channel_type === 'wechat_work'">
          <el-form-item label="Webhook 地址" prop="config.webhook">
            <el-input v-model="cfg.webhook" placeholder="https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=..." />
          </el-form-item>
          <el-form-item label="代理地址">
            <el-input v-model="cfg.proxy_url" placeholder="可选，http://proxy:port" />
          </el-form-item>
        </template>

        <template v-if="form.channel_type === 'wechat_app'">
          <el-form-item label="企业 ID (CorpID)" prop="config.corpid">
            <el-input v-model="cfg.corpid" placeholder="ww..." />
          </el-form-item>
          <el-form-item label="AgentID" prop="config.agentid">
            <el-input v-model.number="cfg.agentid" placeholder="1000001" type="number" />
          </el-form-item>
          <el-form-item label="Secret" prop="config.secret">
            <el-input v-model="cfg.secret" placeholder="应用 Secret" type="password" show-password />
          </el-form-item>
          <el-form-item label="接收人">
            <el-input v-model="cfg.touser" placeholder="@all 或 企业微信 ID，多个用 | 分隔" />
          </el-form-item>
          <el-form-item label="代理地址">
            <el-input v-model="cfg.proxy_url" placeholder="可选，http://proxy:port" />
          </el-form-item>
        </template>

        <template v-if="form.channel_type === 'dingtalk'">
          <el-form-item label="Webhook 地址" prop="config.webhook">
            <el-input v-model="cfg.webhook" placeholder="https://oapi.dingtalk.com/robot/send?access_token=..." />
          </el-form-item>
          <el-form-item label="加签密钥">
            <el-input v-model="cfg.secret" placeholder="可选，安全设置中的签名密钥" type="password" show-password />
          </el-form-item>
        </template>

        <template v-if="form.channel_type === 'feishu'">
          <el-form-item label="Webhook 地址" prop="config.webhook">
            <el-input v-model="cfg.webhook" placeholder="https://open.feishu.cn/open-apis/bot/v2/hook/..." />
          </el-form-item>
          <el-form-item label="加签密钥">
            <el-input v-model="cfg.secret" placeholder="可选，安全设置中的签名密钥" type="password" show-password />
          </el-form-item>
          <el-form-item label="验签开关">
            <el-switch v-model="cfg.verify_sign" />
          </el-form-item>
        </template>

        <template v-if="form.channel_type === 'email'">
          <el-form-item label="SMTP 服务器" prop="config.smtp_host">
            <el-input v-model="cfg.smtp_host" placeholder="smtp.example.com" />
          </el-form-item>
          <el-form-item label="SMTP 端口" prop="config.smtp_port">
            <el-input v-model.number="cfg.smtp_port" placeholder="465" type="number" />
          </el-form-item>
          <el-form-item label="加密方式">
            <el-select v-model="cfg.encryption" style="width: 100%;">
              <el-option label="SSL/TLS (端口 465)" value="ssl" />
              <el-option label="STARTTLS (端口 587)" value="starttls" />
              <el-option label="无" value="none" />
            </el-select>
          </el-form-item>
          <el-form-item label="SMTP 用户名">
            <el-input v-model="cfg.username" placeholder="可选，通常与发件人邮箱相同" />
          </el-form-item>
          <el-form-item label="发件人邮箱" prop="config.from">
            <el-input v-model="cfg.from" placeholder="alert@example.com" />
          </el-form-item>
          <el-form-item label="发件人密码" prop="config.password">
            <el-input v-model="cfg.password" placeholder="SMTP 密码或授权码" type="password" show-password />
          </el-form-item>
          <el-form-item label="收件人" prop="config.to">
            <el-input v-model="cfg.to" placeholder="多个邮箱用逗号分隔" type="textarea" :rows="2" />
          </el-form-item>
        </template>
      </el-form>
      <template #footer>
        <div class="dialog-footer">
          <el-button @click="dialogVisible = false" style="color: var(--text-secondary);">取消</el-button>
          <el-button type="primary" :loading="saving" @click="handleSave">保存</el-button>
        </div>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, reactive, watch } from 'vue'
import dayjs from 'dayjs'
import { ElMessage, ElMessageBox } from 'element-plus'
import type { FormInstance } from 'element-plus'
import { getNotifications, createNotification, updateNotification, deleteNotification, testNotification } from '../api'
import type { NotificationChannel } from '../types'

function getCssVar(name: string): string {
  return getComputedStyle(document.documentElement).getPropertyValue(name).trim()
}

const loading = ref(false)
const channels = ref<NotificationChannel[]>([])
const page = ref(1)
const pageSize = ref(20)
const total = ref(0)
const keyword = ref('')
const typeFilter = ref('')

const dialogVisible = ref(false)
const saving = ref(false)
const editingId = ref<number | null>(null)
const formRef = ref<FormInstance>()
const form = reactive({ channel_type: '', name: '', enabled: true })
const cfg = reactive<any>({})
const rules = { name: [{ required: true, message: '请输入渠道名称', trigger: 'blur' }] }

function channelLabel(t: string) {
  const map: Record<string, string> = { wechat_work: '企业微信机器人', wechat_app: '企业微信应用', dingtalk: '钉钉', feishu: '飞书', email: '邮件' }
  return map[t] || t
}
function channelBg(t: string) {
  const map: Record<string, string> = { wechat_work: 'rgba(81,179,80,0.12)', wechat_app: 'rgba(81,179,80,0.12)', dingtalk: 'rgba(0,150,255,0.12)', feishu: 'rgba(83,119,241,0.12)', email: 'rgba(128,128,128,0.12)' }
  return map[t] || 'rgba(128,128,128,0.12)'
}
function channelColor(t: string) {
  const map: Record<string, string> = { wechat_work: '#51b350', wechat_app: '#51b350', dingtalk: '#0096ff', feishu: '#5377f1', email: '#888' }
  return map[t] || '#888'
}

async function fetchData() {
  loading.value = true
  try {
    const params: any = { page: page.value, page_size: pageSize.value }
    if (keyword.value) params.keyword = keyword.value
    if (typeFilter.value) params.channel_type = typeFilter.value
    const res = await getNotifications(params)
    channels.value = res.data.items
    total.value = res.data.total
  } catch (e: any) { ElMessage.error(e.message) }
  finally { loading.value = false }
}

function openCreate() {
  editingId.value = null
  form.channel_type = ''; form.name = ''; form.enabled = true
  Object.keys(cfg).forEach(k => delete cfg[k])
  dialogVisible.value = true
}

function openEdit(row: NotificationChannel) {
  editingId.value = row.id ?? null
  form.channel_type = row.channel_type; form.name = row.name; form.enabled = row.enabled ?? false
  Object.keys(cfg).forEach(k => delete cfg[k])
  if (row.config_json) {
    try {
      const parsed = JSON.parse(row.config_json)
      if (parsed.to && Array.isArray(parsed.to)) {
        parsed.to = parsed.to.join(', ')
      }
      Object.assign(cfg, parsed)
    } catch { /* ignore */ }
  }
  dialogVisible.value = true
}

function onTypeChange() {
  Object.keys(cfg).forEach(k => delete cfg[k])
}

async function handleSave() {
  const valid = await formRef.value?.validate().catch(() => false)
  if (!valid) return
  saving.value = true
  try {
    const saveCfg = { ...cfg }
    if (saveCfg.to && typeof saveCfg.to === 'string') {
      saveCfg.to = saveCfg.to.split(/[,，]\s*/).filter(Boolean)
    }
    saveCfg.enabled = form.enabled ?? true
    const payload = { ...form, config_json: JSON.stringify(saveCfg) }
    if (editingId.value) {
      await updateNotification(editingId.value, payload)
      ElMessage.success('更新成功')
    } else {
      await createNotification(payload)
      ElMessage.success('创建成功')
    }
    dialogVisible.value = false
    await fetchData()
  } catch (e: any) { ElMessage.error(e.message) }
  finally { saving.value = false }
}

async function toggleEnabled(row: NotificationChannel) {
  try {
    let cfg: any = {}
    if (row.config_json) {
      try { cfg = JSON.parse(row.config_json) } catch {}
    }
    cfg.enabled = row.enabled
    await updateNotification(row.id!, {
      enabled: row.enabled,
      config_json: JSON.stringify(cfg)
    } as any)
  } catch { ElMessage.error('更新失败') }
}

async function handleTest(row: NotificationChannel) {
  try {
    await testNotification(row.id!)
    ElMessage.success('测试消息已发送，请检查通知渠道')
  } catch (e: any) { ElMessage.error(e.message) }
}

async function handleDelete(row: NotificationChannel) {
  try {
    await ElMessageBox.confirm(`确定删除「${row.name}」？`, '确认删除', { type: 'warning', cancelButtonText: '取消', confirmButtonText: '删除' })
    await deleteNotification(row.id!); ElMessage.success('删除成功'); await fetchData()
  } catch { /* ignore */ }
}

watch(dialogVisible, v => { if (!v) editingId.value = null })

onMounted(fetchData)
</script>
