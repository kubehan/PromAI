<template>
  <div class="page-container">
    <div class="page-header">
      <div>
        <h2>主机管理</h2>
        <p>维护服务器资产与 SSH 登录方式，支持连通性检测、初始化命令和 Web 终端</p>
      </div>
      <div class="header-actions">
        <el-button plain @click="router.push('/terminal')">
          <el-icon><Monitor /></el-icon>
          Web 终端
        </el-button>
        <el-button type="primary" @click="openCreate">
          <el-icon><Plus /></el-icon>
          新增主机
        </el-button>
      </div>
    </div>

    <div class="toolbar">
      <el-input v-model="keyword" clearable placeholder="搜索名称 / 地址 / 标签" style="width: 260px" @keyup.enter="loadHosts">
        <template #prefix><el-icon><Search /></el-icon></template>
      </el-input>
      <el-select v-model="envFilter" clearable placeholder="环境" style="width: 140px" @change="loadHosts">
        <el-option v-for="env in envOptions" :key="env" :label="env" :value="env" />
      </el-select>
      <el-button @click="loadHosts">刷新</el-button>
      <span class="host-count">共 {{ hosts.length }} 台主机</span>
    </div>

    <el-table :data="pagedHosts" v-loading="loading" stripe class="host-table">
      <el-table-column label="主机" min-width="220">
        <template #default="{ row }">
          <div class="host-name-cell">
            <span class="status-dot" :class="row.last_status || 'unknown'"></span>
            <div>
              <div class="host-name">{{ row.name }}</div>
              <div class="host-key">{{ row.host_key }}</div>
            </div>
          </div>
        </template>
      </el-table-column>
      <el-table-column label="地址" min-width="190">
        <template #default="{ row }">
          <span class="mono">{{ row.address }}:{{ row.port }}</span>
        </template>
      </el-table-column>
      <el-table-column prop="username" label="登录用户" width="120" />
      <el-table-column label="认证方式" width="110">
        <template #default="{ row }">
          <el-tag size="small" effect="plain">{{ row.auth_type === 'private_key' ? '密钥' : '密码' }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="环境 / 分组" width="170">
        <template #default="{ row }">
          <el-tag v-if="row.env" size="small" type="warning" effect="plain" style="margin-right: 6px">{{ row.env }}</el-tag>
          <span class="muted">{{ row.group || '-' }}</span>
        </template>
      </el-table-column>
      <el-table-column label="状态" width="100">
        <template #default="{ row }">
          <el-tag size="small" :type="statusType(row.last_status)">{{ statusText(row.last_status) }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="操作" width="320" fixed="right">
        <template #default="{ row }">
          <div class="op-actions">
            <el-button size="small" type="primary" @click="openTerminal(row)">连接</el-button>
            <el-button size="small" @click="handleTest(row)">测试</el-button>
            <el-button size="small" @click="openLogs(row)">日志</el-button>
            <el-button size="small" @click="openEdit(row)">编辑</el-button>
            <el-button size="small" type="danger" @click="handleDelete(row)">删除</el-button>
          </div>
        </template>
      </el-table-column>
    </el-table>

    <div class="pager-bar">
      <el-pagination
        background
        layout="total, sizes, prev, pager, next, jumper"
        :total="hosts.length"
        :current-page="page"
        :page-size="pageSize"
        :page-sizes="[10, 20, 50, 100]"
        @current-change="onPageChange"
        @size-change="onSizeChange"
      />
    </div>

    <el-dialog v-model="dialogVisible" :title="form.id ? '编辑主机' : '新增主机'" width="720px" destroy-on-close>
      <el-form :model="form" :rules="rules" ref="formRef" label-width="110px">
        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item label="主机名称" prop="name">
              <el-input v-model="form.name" placeholder="例如：prod-web-01" />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="主机标识" prop="host_key">
              <el-input v-model="form.host_key" placeholder="留空自动生成" />
            </el-form-item>
          </el-col>
          <el-col :span="16">
            <el-form-item label="地址" prop="address">
              <el-input v-model="form.address" placeholder="IP 或域名" />
            </el-form-item>
          </el-col>
          <el-col :span="8">
            <el-form-item label="端口" prop="port">
              <el-input-number v-model="form.port" :min="1" :max="65535" controls-position="right" style="width: 100%" />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="登录用户" prop="username">
              <el-input v-model="form.username" placeholder="root / ubuntu" />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="认证方式">
              <el-radio-group v-model="form.auth_type">
                <el-radio-button value="password">密码</el-radio-button>
                <el-radio-button value="private_key">私钥</el-radio-button>
              </el-radio-group>
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="环境">
              <el-input v-model="form.env" placeholder="prod / staging / test" />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="分组">
              <el-input v-model="form.group" placeholder="业务分组" />
            </el-form-item>
          </el-col>
        </el-row>

        <el-form-item v-if="form.auth_type === 'password'" label="SSH 密码">
          <el-input v-model="form.password" type="password" show-password
            :placeholder="form.id && form.has_password ? '已保存密码，留空保持不变' : '请输入 SSH 密码'" />
        </el-form-item>
        <template v-else>
          <el-form-item label="SSH 私钥">
            <el-input v-model="form.private_key" type="textarea" :rows="4"
              :placeholder="form.id && form.has_private_key ? '已保存私钥，留空保持不变' : '粘贴 PEM / OpenSSH 私钥'" />
          </el-form-item>
          <el-form-item label="私钥口令">
            <el-input v-model="form.passphrase" type="password" show-password placeholder="私钥有口令时填写" />
          </el-form-item>
        </template>

        <el-form-item label="初始化命令">
          <el-input
            v-model="initialCommandsText"
            type="textarea"
            :rows="6"
            placeholder="每行一条命令，例如：&#10;hostname&#10;hostname -I&#10;cd /home/wei.han/tools"
          />
          <div class="form-tip">SSH 登录成功后自动逐行执行，每条命令自动换行；默认执行 hostname 和 hostname -I。</div>
        </el-form-item>

        <el-form-item label="主机指纹">
          <el-input v-model="form.host_key_fingerprint" placeholder="可选，SHA256:...，留空自动接受主机密钥" />
        </el-form-item>
        <el-form-item label="标签">
          <el-input v-model="form.tags_json" placeholder='JSON，例如 {"role":"web","idc":"sh"}' />
        </el-form-item>
        <el-form-item label="描述">
          <el-input v-model="form.description" type="textarea" :rows="2" placeholder="用途、负责人等" />
        </el-form-item>
        <el-form-item label="启用">
          <el-switch v-model="form.enabled" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="handleSave">保存</el-button>
      </template>
    </el-dialog>

    <el-drawer v-model="logsVisible" :title="`执行日志 - ${logHost?.name || ''}`" size="620px">
      <el-timeline v-if="logs.length">
        <el-timeline-item v-for="log in logs" :key="log.id" :timestamp="formatTime(log.created_at)" placement="top">
          <div class="log-item">
            <div class="log-command mono">{{ log.command }}</div>
            <div class="muted">{{ log.username || '-' }} · {{ log.source }} · 退出码 {{ log.exit_code }} · {{ log.duration_ms }}ms</div>
            <pre v-if="log.output" class="log-output">{{ log.output }}</pre>
          </div>
        </el-timeline-item>
      </el-timeline>
      <el-empty v-else description="暂无执行日志" />
    </el-drawer>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox, type FormInstance, type FormRules } from 'element-plus'
import {
  getHosts, createHost, updateHost, deleteHost, testHost,
  getHostCommands, setHostInitialCommands, getHostLogs,
} from '../api'
import type { Host, HostExecLog } from '../types'

const DEFAULT_INITIAL_COMMANDS = 'hostname\nhostname -I'

const router = useRouter()
const hosts = ref<Host[]>([])
const loading = ref(false)
const keyword = ref('')
const envFilter = ref('')
const envOptions = ref<string[]>(['prod', 'staging', 'test', 'dev'])
const page = ref(1)
const pageSize = ref(20)

const pagedHosts = computed(() => {
  const start = (page.value - 1) * pageSize.value
  return hosts.value.slice(start, start + pageSize.value)
})

function onPageChange(p: number) { page.value = p }
function onSizeChange(size: number) {
  pageSize.value = size
  page.value = 1
}

const dialogVisible = ref(false)
const saving = ref(false)
const formRef = ref<FormInstance>()
const form = reactive<Host>(emptyHost())
const initialCommandsText = ref(DEFAULT_INITIAL_COMMANDS)
const rules: FormRules = {
  name: [{ required: true, message: '请输入主机名称', trigger: 'blur' }],
  address: [{ required: true, message: '请输入主机地址', trigger: 'blur' }],
  username: [{ required: true, message: '请输入登录用户', trigger: 'blur' }],
}

const logsVisible = ref(false)
const logHost = ref<Host | null>(null)
const logs = ref<HostExecLog[]>([])

function emptyHost(): Host {
  return {
    name: '', host_key: '', address: '', port: 22, username: '',
    auth_type: 'password', host_key_fingerprint: '', env: '', group: '',
    tags_json: '', description: '', enabled: true, password: '', private_key: '', passphrase: '',
  }
}

function parseInitialCommands() {
  return initialCommandsText.value
    .split(/\r?\n/)
    .map(line => line.trim())
    .filter(Boolean)
}

async function loadHosts() {
  loading.value = true
  page.value = 1
  try {
    const { data } = await getHosts({ keyword: keyword.value || undefined, env: envFilter.value || undefined })
    hosts.value = data.items || []
    const envs = new Set(envOptions.value)
    hosts.value.forEach(h => { if (h.env) envs.add(h.env) })
    envOptions.value = Array.from(envs)
  } catch (e: any) {
    ElMessage.error(e.message || '加载主机失败')
  } finally {
    loading.value = false
  }
}

function openCreate() {
  Object.assign(form, emptyHost())
  initialCommandsText.value = DEFAULT_INITIAL_COMMANDS
  dialogVisible.value = true
}

function openEdit(row: Host) {
  Object.assign(form, emptyHost(), {
    ...row,
    password: '',
    private_key: '',
    passphrase: '',
    clear_password: false,
    clear_private_key: false,
  })
  initialCommandsText.value = (row.commands || []).map(c => c.command).join('\n') || DEFAULT_INITIAL_COMMANDS
  dialogVisible.value = true
  if (row.id) loadInitialCommands(row.id)
}

async function loadInitialCommands(hostId: number) {
  try {
    const { data } = await getHostCommands(hostId)
    initialCommandsText.value = (data.items || []).map(c => c.command).join('\n')
  } catch {
    // 保留列表数据中的命令，加载失败不阻塞编辑
  }
}

async function handleSave() {
  const valid = await formRef.value?.validate().catch(() => false)
  if (!valid) return
  const isNew = !form.id
  saving.value = true
  try {
    let hostId = form.id
    if (hostId) {
      const { data } = await updateHost(hostId, { ...form })
      const idx = hosts.value.findIndex(h => h.id === hostId)
      if (idx >= 0) hosts.value[idx] = data
    } else {
      const { data } = await createHost({ ...form })
      hostId = data.id
      form.id = data.id
      hosts.value.push(data)
    }
    if (hostId) {
      const lines = parseInitialCommands()
      const targetLines = lines.length ? lines : (isNew ? DEFAULT_INITIAL_COMMANDS.split('\n') : [])
      await setHostInitialCommands(hostId, targetLines)
    }
    ElMessage.success(isNew ? '主机已新增' : '主机已更新')
    dialogVisible.value = false
    await loadHosts()
  } catch (e: any) {
    ElMessage.error(e.message || '保存失败')
  } finally {
    saving.value = false
  }
}

async function handleDelete(row: Host) {
  if (!row.id) return
  try {
    await ElMessageBox.confirm(`确定删除主机「${row.name}」吗？关联的初始化命令会一并删除。`, '删除确认', { type: 'warning' })
    await deleteHost(row.id)
    hosts.value = hosts.value.filter(h => h.id !== row.id)
    const maxPage = Math.max(1, Math.ceil(hosts.value.length / pageSize.value))
    if (page.value > maxPage) page.value = maxPage
    ElMessage.success('已删除')
  } catch (e: any) {
    if (e !== 'cancel') ElMessage.error(e.message || '删除失败')
  }
}

async function handleTest(row: Host) {
  if (!row.id) return
  try {
    const { data } = await testHost(row.id)
    if (data.success) {
      row.last_status = 'online'
      ElMessage.success(`连接成功（${data.duration_ms}ms）`)
    } else {
      row.last_status = 'offline'
      ElMessage.error(data.message || '连接失败')
    }
  } catch (e: any) {
    ElMessage.error(e.message || '测试失败')
  }
}

function openTerminal(row: Host) {
  if (!row.id) return
  router.push({ path: '/terminal', query: { host: String(row.id) } })
}

async function openLogs(row: Host) {
  if (!row.id) return
  logHost.value = row
  logsVisible.value = true
  try {
    const { data } = await getHostLogs(row.id, 100)
    logs.value = data.items || []
  } catch (e: any) {
    ElMessage.error(e.message || '加载日志失败')
  }
}

function statusType(status?: string) {
  if (status === 'online') return 'success'
  if (status === 'offline') return 'danger'
  return 'info'
}

function statusText(status?: string) {
  if (status === 'online') return '在线'
  if (status === 'offline') return '离线'
  return '未知'
}

function formatTime(v?: string | null) {
  if (!v) return ''
  return new Date(v).toLocaleString()
}

onMounted(loadHosts)
</script>

<style scoped>
.page-header { display: flex; justify-content: space-between; align-items: flex-start; gap: 16px; }
.header-actions { display: flex; gap: 8px; }
.toolbar { display: flex; align-items: center; gap: 10px; margin: 18px 0 12px; flex-wrap: wrap; }
.host-count { margin-left: auto; color: var(--text-secondary, #8b9bb4); font-size: 13px; }
.host-table { width: 100%; font-size: 14px; -webkit-font-smoothing: antialiased; }
.host-table :deep(.el-table__cell) { color: var(--text-primary, #e8eef7); }
.host-table :deep(th.el-table__cell) { color: #9fb3c8; font-weight: 600; }
.host-table :deep(.el-table__inner-wrapper) { border-radius: 10px; overflow: hidden; }
.host-name-cell { display: flex; align-items: center; gap: 10px; }
.op-actions { display: flex; align-items: center; gap: 6px; flex-wrap: nowrap; }
.op-actions .el-button + .el-button { margin-left: 0; }
.pager-bar { display: flex; justify-content: flex-end; margin-top: 14px; }
.status-dot { width: 8px; height: 8px; border-radius: 50%; background: #6b7280; flex: none; }
.status-dot.online { background: #22c55e; box-shadow: 0 0 8px rgba(34, 197, 94, .6); }
.status-dot.offline { background: #ef4444; }
.host-name { font-weight: 600; color: #eef4fb; }
.host-key, .muted { color: #b6c6db; font-size: 13px; letter-spacing: .2px; }
.mono { font-family: Menlo, Monaco, Consolas, monospace; font-size: 13px; color: #c3d2e6; }
.form-tip { margin-top: 6px; color: var(--text-secondary, #8b9bb4); font-size: 12px; line-height: 1.5; }
.log-item { display: flex; flex-direction: column; gap: 6px; }
.log-command { color: #22d3ee; word-break: break-all; }
.log-output {
  background: #0b1020; color: #d7e2f0; padding: 12px; border-radius: 8px;
  max-height: 180px; overflow: auto; margin-top: 4px; font-family: Menlo, Monaco, Consolas, monospace;
  font-size: 12px; white-space: pre-wrap; word-break: break-all;
}
</style>
