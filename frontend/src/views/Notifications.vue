<template>
  <div class="page-container">
    <div class="page-header">
      <h2><el-icon><Bell /></el-icon> 通知渠道</h2>
      <p>配置告警通知渠道，支持企业微信、钉钉、飞书、邮件</p>
    </div>

    <div class="section-card">
      <div class="section-header">
        <h3><el-icon :size="16" color="#00d4ff"><List /></el-icon> 渠道列表</h3>
        <el-button type="primary" @click="openCreate"><el-icon><Plus /></el-icon> 新增渠道</el-button>
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

        <!-- 企业微信机器人 -->
        <template v-if="form.channel_type === 'wechat_work'">
          <el-form-item label="Webhook 地址" prop="config.webhook">
            <el-input v-model="cfg.webhook" placeholder="https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=..." />
          </el-form-item>
          <el-form-item label="代理地址">
            <el-input v-model="cfg.proxy_url" placeholder="可选，http://proxy:port" />
          </el-form-item>
        </template>

        <!-- 企业微信应用 -->
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

        <!-- 钉钉机器人 -->
        <template v-if="form.channel_type === 'dingtalk'">
          <el-form-item label="Webhook 地址" prop="config.webhook">
            <el-input v-model="cfg.webhook" placeholder="https://oapi.dingtalk.com/robot/send?access_token=..." />
          </el-form-item>
          <el-form-item label="加签密钥">
            <el-input v-model="cfg.secret" placeholder="可选，安全设置中的签名密钥" type="password" show-password />
          </el-form-item>
        </template>

        <!-- 飞书机器人 -->
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

        <!-- 邮件 -->
        <template v-if="form.channel_type === 'email'">
          <el-form-item label="SMTP 服务器" prop="config.smtp_host">
            <el-input v-model="cfg.smtp_host" placeholder="smtp.example.com" />
          </el-form-item>
          <el-form-item label="SMTP 端口" prop="config.smtp_port">
            <el-input v-model.number="cfg.smtp_port" placeholder="465" type="number" />
          </el-form-item>
          <el-form-item label="用户名" prop="config.username">
            <el-input v-model="cfg.username" placeholder="user@example.com" />
          </el-form-item>
          <el-form-item label="密码" prop="config.password">
            <el-input v-model="cfg.password" placeholder="SMTP 密码/授权码" type="password" show-password />
          </el-form-item>
          <el-form-item label="发件地址" prop="config.from">
            <el-input v-model="cfg.from" placeholder="user@example.com" />
          </el-form-item>
          <el-form-item label="收件地址" prop="config.to">
            <el-input v-model="cfg.to" placeholder="user1@example.com, user2@example.com" />
            <div style="font-size: 11px; color: var(--text-tertiary); margin-top: 2px;">多个邮箱用逗号分隔</div>
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
import { ref, onMounted, reactive } from 'vue'
import dayjs from 'dayjs'
import { ElMessage, ElMessageBox } from 'element-plus'
import type { FormInstance } from 'element-plus'
import { getNotifications, createNotification, updateNotification, deleteNotification, testNotification } from '../api'
import type { NotificationChannel } from '../types'

const loading = ref(false)
const saving = ref(false)
const channels = ref<NotificationChannel[]>([])
const dialogVisible = ref(false)
const editingId = ref<number | null>(null)
const formRef = ref<FormInstance>()

const form = ref<NotificationChannel>({ channel_type: 'wechat_work', name: '', enabled: true, config_json: '{}' })

const cfg = reactive<Record<string, any>>({
  webhook: '', secret: '', proxy_url: '',
  corpid: '', agentid: 0, touser: '',
  smtp_host: '', smtp_port: 465, username: '', password: '', from: '', to: '',
  verify_sign: false,
})

const rules = {
  channel_type: [{ required: true, message: '请选择渠道类型', trigger: 'change' }],
  name: [{ required: true, message: '请输入名称', trigger: 'blur' }],
}

function channelLabel(t: string) {
  const m: Record<string,string> = { wechat_work: '企业微信', wechat_app: '企业微信应用', dingtalk: '钉钉', feishu: '飞书', email: '邮件' }
  return m[t] || t
}
function channelBg(t: string) {
  const m: Record<string,string> = { wechat_work: 'rgba(16,185,129,0.1)', wechat_app: 'rgba(16,185,129,0.1)', dingtalk: 'rgba(0,212,255,0.1)', feishu: 'rgba(124,58,237,0.1)', email: 'rgba(99,102,241,0.1)' }
  return m[t] || 'rgba(99,102,241,0.1)'
}
function channelColor(t: string) {
  const m: Record<string,string> = { wechat_work: '#10b981', wechat_app: '#10b981', dingtalk: '#00d4ff', feishu: '#7c3aed', email: '#818cf8' }
  return m[t] || '#818cf8'
}

function defaultConfig(t: string): Record<string, any> {
  switch (t) {
    case 'wechat_work': return { webhook: '', proxy_url: '' }
    case 'wechat_app': return { corpid: '', agentid: 0, secret: '', touser: '@all', proxy_url: '' }
    case 'dingtalk': return { webhook: '', secret: '' }
    case 'feishu': return { webhook: '', secret: '', verify_sign: false }
    case 'email': return { smtp_host: '', smtp_port: 465, username: '', password: '', from: '', to: '' }
    default: return {}
  }
}

function loadConfig(json: string, t: string) {
  let parsed: Record<string, any> = {}
  try { parsed = JSON.parse(json) } catch { /* ignore */ }
  const lowerMap: Record<string, any> = {}
  for (const k of Object.keys(parsed)) lowerMap[k.toLowerCase()] = parsed[k]

  const defs = defaultConfig(t)
  for (const key of Object.keys(defs)) {
    let val: any = lowerMap[key.toLowerCase()] !== undefined ? lowerMap[key.toLowerCase()] : defs[key]
    if (key === 'to' && Array.isArray(val)) {
      const arr: string[] = val
      val = arr.join(', ')
    }

    (cfg as any)[key] = val
  }
}

function syncConfigToJson() {
  const defs = defaultConfig(form.value.channel_type)
  const obj: Record<string, any> = {}
  for (const key of Object.keys(defs)) {
    let val = (cfg as any)[key]
    if (key === 'to' && typeof val === 'string') {
      val = val.split(',').map((s: string) => s.trim()).filter(Boolean)
    }
    obj[key] = val
  }
  form.value.config_json = JSON.stringify(obj)
}

function onTypeChange() {
  loadConfig('{}', form.value.channel_type)
}

async function fetchData() {
  loading.value = true
  try { const res = await getNotifications(); channels.value = res.data } finally { loading.value = false }
}

function openCreate() {
  editingId.value = null
  form.value = { channel_type: 'wechat_work', name: '', enabled: true, config_json: '{}' }
  loadConfig('{}', 'wechat_work')
  dialogVisible.value = true
}

function openEdit(row: NotificationChannel) {
  editingId.value = row.id!
  form.value = { ...row }
  loadConfig(row.config_json || '{}', row.channel_type)
  dialogVisible.value = true
}

async function handleSave() {
  const valid = await formRef.value?.validate().catch(() => false)
  if (!valid) return
  saving.value = true
  try {
    syncConfigToJson()
    if (editingId.value) { await updateNotification(editingId.value, form.value); ElMessage.success('更新成功') }
    else { await createNotification(form.value); ElMessage.success('创建成功') }
    dialogVisible.value = false; await fetchData()
  } catch (e: any) { ElMessage.error(e.message) } finally { saving.value = false }
}

async function handleDelete(row: NotificationChannel) {
  try {
    await ElMessageBox.confirm(`确定删除「${row.name}」？`, '确认删除', { type: 'warning', cancelButtonText: '取消', confirmButtonText: '删除' })
    await deleteNotification(row.id!); ElMessage.success('删除成功'); await fetchData()
  } catch { /* ignore */ }
}

async function handleTest(row: NotificationChannel) {
  try { await testNotification(row.id!); ElMessage.success('测试通知已发送，请检查渠道') }
  catch (e: any) { ElMessage.error(e.message) }
}

async function toggleEnabled(row: NotificationChannel) {
  try { await updateNotification(row.id!, row); ElMessage.success(row.enabled ? '已启用' : '已禁用') }
  catch { row.enabled = !row.enabled }
}

onMounted(fetchData)
</script>
