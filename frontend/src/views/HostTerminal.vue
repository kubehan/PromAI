<template>
  <div class="terminal-page">
    <aside v-show="!sidebarCollapsed" class="host-sidebar">
      <div class="sidebar-header">
        <span>主机列表</span>
        <div class="sidebar-actions">
          <el-button text size="small" title="刷新" @click="loadHosts"><el-icon><Refresh /></el-icon></el-button>
          <el-button text size="small" title="收起主机列表" @click="sidebarCollapsed = true"><el-icon><Fold /></el-icon></el-button>
        </div>
      </div>
      <el-input v-model="keyword" size="small" clearable placeholder="搜索主机" class="host-search">
        <template #prefix><el-icon><Search /></el-icon></template>
      </el-input>
      <div class="host-list" v-loading="hostsLoading">
        <div
          v-for="host in filteredHosts"
          :key="host.id"
          class="host-item"
          :class="{ active: tabs.some(t => t.host.id === host.id && t.key === activeKey) }"
          @click="openHost(host)"
        >
          <span class="status-dot" :class="host.last_status || 'unknown'"></span>
          <div class="host-meta">
            <div class="host-name">{{ host.name }}</div>
            <div class="host-addr">{{ host.username }}@{{ host.address }}:{{ host.port }}</div>
          </div>
          <el-tag v-if="host.env" size="small" effect="plain" type="warning">{{ host.env }}</el-tag>
          <div class="host-actions">
            <el-icon class="host-action" title="复制连接信息" @click.stop="copyHost(host)"><CopyDocument /></el-icon>
            <el-icon class="host-action" title="打开新窗口" @click.stop="openHost(host, true)"><Plus /></el-icon>
          </div>
        </div>
        <el-empty v-if="!filteredHosts.length" description="暂无主机" :image-size="70" />
      </div>
      <div class="sidebar-footer">
        <el-button text type="primary" size="small" @click="router.push('/hosts')">
          <el-icon><Setting /></el-icon>
          管理主机
        </el-button>
      </div>
    </aside>

    <section ref="terminalMainRef" class="terminal-main">
      <div class="tab-bar">
        <el-button v-if="sidebarCollapsed" text size="small" class="sidebar-toggle" title="展开主机列表" @click="sidebarCollapsed = false">
          <el-icon><Expand /></el-icon>
        </el-button>
        <div class="tabs">
          <div
            v-for="tab in tabs"
            :key="tab.key"
            class="tab-item"
            :class="{ active: tab.key === activeKey }"
            @click="activeKey = tab.key"
          >
            <span class="status-dot" :class="tab.status || tab.host.last_status || 'unknown'"></span>
            <span class="tab-name">{{ tab.name }}</span>
            <el-icon class="tab-close" @click.stop="closeTab(tab.key)"><Close /></el-icon>
          </div>
        </div>
        <div class="terminal-actions" v-if="activeTab && activeHost">
          <span class="conn-state" :class="activeTab.status">
            {{ statusLabel(activeTab.status) }}
          </span>
          <el-button text size="small" @click="fitActive">适配窗口</el-button>
          <el-button text size="small" @click="openNewWindow">新窗口</el-button>
          <el-button text size="small" @click="reconnectActive">重连</el-button>
          <el-button text size="small" @click="toggleFullscreen">
            <el-icon><FullScreen /></el-icon>
            {{ isFullscreen ? '退出全屏' : '全屏' }}
          </el-button>
        </div>
      </div>

      <div class="terminal-body">
        <div v-if="!tabs.length" class="terminal-empty">
          <el-icon :size="46"><Monitor /></el-icon>
          <p>从左侧选择一台主机开始 SSH 会话</p>
          <el-button v-if="hosts.length" type="primary" @click="openHost(hosts[0])">连接 {{ hosts[0].name }}</el-button>
        </div>
        <TerminalPane
          v-for="tab in tabs"
          v-show="tab.key === activeKey"
          :key="tab.key"
          :host="tab.host"
          :active="tab.key === activeKey"
          :ref="(el: any) => setPaneRef(tab.key, el)"
          @status="status => setTabStatus(tab, status)"
        />
      </div>

      <div class="quick-bar" v-if="activeHost">
        <div class="quick-commands">
          <span class="quick-label">快捷命令</span>
          <el-tag
            v-for="cmd in quickCommands"
            :key="cmd.id"
            class="quick-tag"
            effect="plain"
            @click="runQuickCommand(cmd.command)"
          >{{ cmd.name }}</el-tag>
          <el-button v-if="!quickCommands.length" text size="small" @click="router.push('/hosts')">
            前往配置自定义命令
          </el-button>
        </div>
        <div class="command-input">
          <el-input
            v-model="commandInput"
            placeholder="输入要在当前主机执行的命令，Enter 发送"
            size="small"
            @keyup.enter="sendInputCommand"
          >
            <template #prefix><span class="prompt">$</span></template>
          </el-input>
          <el-button size="small" type="primary" @click="sendInputCommand">发送</el-button>
        </div>
      </div>
    </section>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onActivated, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import TerminalPane from '../components/terminal/TerminalPane.vue'
import { getHosts, getHostCommands } from '../api'
import type { Host, HostCommand } from '../types'

defineOptions({ name: 'HostTerminal' })

interface TerminalTab {
  key: string
  host: Host
  name: string
  status: string
}

const route = useRoute()
const router = useRouter()
const hosts = ref<Host[]>([])
const hostsLoading = ref(false)
const keyword = ref('')
const tabs = ref<TerminalTab[]>([])
const activeKey = ref<string | null>(null)
const quickCommands = ref<HostCommand[]>([])
const commandInput = ref('')
const sidebarCollapsed = ref(localStorage.getItem('terminal-sidebar-collapsed') === '1')
const terminalMainRef = ref<HTMLElement | null>(null)
const isFullscreen = ref(false)
const paneRefs = new Map<string, any>()
let sessionSeq = 0

const filteredHosts = computed(() => {
  const kw = keyword.value.trim().toLowerCase()
  if (!kw) return hosts.value
  return hosts.value.filter(h =>
    [h.name, h.host_key, h.address, h.group, h.env].some(v => (v || '').toLowerCase().includes(kw)),
  )
})

const activeTab = computed(() => tabs.value.find(t => t.key === activeKey.value) || null)
const activeHost = computed(() => activeTab.value?.host || null)

function setPaneRef(key: string, el: any) {
  if (el) paneRefs.set(key, el)
  else paneRefs.delete(key)
}

function setTabStatus(tab: TerminalTab, status: string) {
  tab.status = status
  const host = hosts.value.find(h => h.id === tab.host.id)
  if (host && (status === 'connected' || status === 'closed' || status === 'error')) {
    host.last_status = status === 'connected' ? 'online' : status === 'closed' ? 'offline' : host.last_status
  }
}

function statusLabel(status?: string) {
  if (status === 'connected') return '已连接'
  if (status === 'connecting') return '连接中'
  if (status === 'error') return '连接失败'
  if (status === 'closed') return '已断开'
  return '未连接'
}

async function loadHosts() {
  hostsLoading.value = true
  try {
    const { data } = await getHosts()
    hosts.value = data.items || []
  } catch (e: any) {
    ElMessage.error(e.message || '加载主机失败')
  } finally {
    hostsLoading.value = false
  }
}

async function loadQuickCommands(hostId: number) {
  try {
    const { data } = await getHostCommands(hostId)
    quickCommands.value = data.items || []
  } catch {
    quickCommands.value = []
  }
}

function nextTabName(host: Host) {
  const count = tabs.value.filter(t => t.host.id === host.id).length
  return count > 0 ? `${host.name} #${count + 1}` : host.name
}

function openHost(host: Host, forceNew = false) {
  if (!host.id) return
  if (!forceNew) {
    const existing = tabs.value.find(t => t.host.id === host.id)
    if (existing) {
      activeKey.value = existing.key
      loadQuickCommands(host.id)
      return
    }
  }
  sessionSeq += 1
  const key = `${host.id}-${Date.now()}-${sessionSeq}`
  tabs.value.push({ key, host, name: nextTabName(host), status: 'connecting' })
  activeKey.value = key
  loadQuickCommands(host.id)
}

function closeTab(key: string) {
  const idx = tabs.value.findIndex(t => t.key === key)
  if (idx < 0) return
  tabs.value.splice(idx, 1)
  paneRefs.delete(key)
  if (activeKey.value === key) {
    const next = tabs.value[Math.max(0, idx - 1)]
    activeKey.value = next?.key || null
  }
}

function activePane() {
  return activeKey.value ? paneRefs.get(activeKey.value) : null
}

function runQuickCommand(command: string) {
  if (!activePane()?.sendCommand(command)) {
    ElMessage.warning('终端尚未连接')
  }
}

function sendInputCommand() {
  const cmd = commandInput.value.trim()
  if (!cmd) return
  if (!activePane()?.sendCommand(cmd)) {
    ElMessage.warning('终端尚未连接')
    return
  }
  commandInput.value = ''
}

function fitActive() {
  activePane()?.fit()
}

function reconnectActive() {
  activePane()?.connect()
}

function openNewWindow() {
  if (activeHost.value) openHost(activeHost.value, true)
}

function fallbackCopy(text: string) {
  const textarea = document.createElement('textarea')
  textarea.value = text
  textarea.style.position = 'fixed'
  textarea.style.opacity = '0'
  document.body.appendChild(textarea)
  textarea.select()
  const ok = document.execCommand('copy')
  document.body.removeChild(textarea)
  if (!ok) throw new Error('copy failed')
}

async function copyHost(host: Host) {
  const text = `${host.username}@${host.address}:${host.port}`
  try {
    if (navigator.clipboard?.writeText) {
      await navigator.clipboard.writeText(text)
    } else {
      fallbackCopy(text)
    }
    ElMessage.success(`已复制：${text}`)
  } catch {
    ElMessage.error('复制失败，请手动选择复制')
  }
}

async function toggleFullscreen() {
  const el = terminalMainRef.value
  if (!el) return
  try {
    if (document.fullscreenElement) {
      await document.exitFullscreen()
    } else {
      await el.requestFullscreen()
    }
  } catch {
    ElMessage.error('当前浏览器不支持全屏')
  }
}

function handleFullscreenChange() {
  isFullscreen.value = document.fullscreenElement === terminalMainRef.value
  nextTick(() => fitActive())
}

function openRouteHost() {
  const qid = Number(route.query.host)
  if (!qid) return
  const target = hosts.value.find(h => h.id === qid)
  if (target) openHost(target)
}

watch(activeKey, key => {
  const tab = tabs.value.find(t => t.key === key)
  if (tab?.host.id) loadQuickCommands(tab.host.id)
})

watch(sidebarCollapsed, collapsed => {
  localStorage.setItem('terminal-sidebar-collapsed', collapsed ? '1' : '0')
  nextTick(() => fitActive())
})

watch(
  () => route.query.host,
  () => {
    if (route.path === '/terminal') openRouteHost()
  },
)

onMounted(async () => {
  document.addEventListener('fullscreenchange', handleFullscreenChange)
  await loadHosts()
  openRouteHost()
})

onBeforeUnmount(() => {
  document.removeEventListener('fullscreenchange', handleFullscreenChange)
})

onActivated(async () => {
  openRouteHost()
  await nextTick()
  fitActive()
  if (activeHost.value?.id) loadQuickCommands(activeHost.value.id)
})
</script>

<style scoped>
.terminal-page {
  display: flex;
  height: calc(100vh - 108px);
  min-height: 520px;
  gap: 12px;
}
.host-sidebar {
  width: 250px;
  flex: none;
  display: flex;
  flex-direction: column;
  background: var(--bg-card, rgba(255, 255, 255, .03));
  border: 1px solid var(--border-color, rgba(255, 255, 255, .08));
  border-radius: 10px;
  overflow: hidden;
}
.sidebar-header {
  display: flex; align-items: center; justify-content: space-between;
  padding: 12px 14px 8px; font-weight: 600; font-size: 13px;
}
.sidebar-actions { display: flex; align-items: center; gap: 2px; }
.host-search { padding: 0 12px 8px; }
.host-list { flex: 1; overflow-y: auto; padding: 4px 8px; }
.host-item {
  display: flex; align-items: center; gap: 9px; padding: 9px 10px;
  border-radius: 8px; cursor: pointer; transition: background .15s;
}
.host-item:hover { background: rgba(64, 158, 255, .08); }
.host-item.active { background: rgba(64, 158, 255, .16); }
.host-actions { display: flex; align-items: center; gap: 2px; opacity: 0; transition: opacity .15s; }
.host-item:hover .host-actions { opacity: .7; }
.host-action { font-size: 14px; padding: 2px; border-radius: 4px; }
.host-action:hover { opacity: 1; color: #60a5fa; background: rgba(64, 158, 255, .16); }
.host-meta { flex: 1; min-width: 0; }
.host-name { font-size: 13px; font-weight: 500; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.host-addr { font-size: 11px; opacity: .55; font-family: Menlo, Monaco, Consolas, monospace; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.status-dot { width: 7px; height: 7px; border-radius: 50%; background: #6b7280; flex: none; }
.status-dot.online, .status-dot.connected { background: #22c55e; box-shadow: 0 0 7px rgba(34, 197, 94, .55); }
.status-dot.offline, .status-dot.error { background: #ef4444; }
.status-dot.connecting { background: #f59e0b; }
.sidebar-footer { padding: 8px 12px; border-top: 1px solid var(--border-color, rgba(255, 255, 255, .08)); }

.terminal-main { flex: 1; min-width: 0; display: flex; flex-direction: column; gap: 8px; }
.terminal-main:fullscreen { width: 100vw; height: 100vh; padding: 10px; box-sizing: border-box; background: #070b16; }
.sidebar-toggle { flex: none; }
.tab-bar {
  display: flex; align-items: center; gap: 10px;
  background: var(--bg-card, rgba(255, 255, 255, .03));
  border: 1px solid var(--border-color, rgba(255, 255, 255, .08));
  border-radius: 10px; padding: 6px 8px;
}
.tabs { display: flex; gap: 6px; overflow-x: auto; flex: 1; }
.tab-item {
  display: flex; align-items: center; gap: 6px; padding: 5px 9px;
  border-radius: 6px; font-size: 12px; cursor: pointer; white-space: nowrap;
  background: rgba(255, 255, 255, .04);
}
.tab-item.active { background: rgba(64, 158, 255, .2); color: #60a5fa; }
.tab-close { font-size: 12px; opacity: .55; }
.tab-close:hover { opacity: 1; color: #ef4444; }
.terminal-actions { display: flex; align-items: center; gap: 2px; flex: none; }
.conn-state { font-size: 12px; opacity: .7; margin-right: 6px; }
.conn-state.connected { color: #22c55e; opacity: 1; }
.conn-state.error { color: #ef4444; opacity: 1; }

.terminal-body {
  flex: 1; min-height: 0; position: relative;
  border: 1px solid var(--border-color, rgba(255, 255, 255, .08));
  border-radius: 10px; overflow: hidden; background: #0b1020;
}
.terminal-empty {
  height: 100%; display: flex; flex-direction: column; align-items: center;
  justify-content: center; gap: 12px; color: #6b7a90;
}
.terminal-empty p { margin: 0; font-size: 13px; }

.quick-bar {
  background: var(--bg-card, rgba(255, 255, 255, .03));
  border: 1px solid var(--border-color, rgba(255, 255, 255, .08));
  border-radius: 10px; padding: 8px 10px; display: flex; flex-direction: column; gap: 8px;
}
.quick-commands { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; }
.quick-label { font-size: 12px; opacity: .6; }
.quick-tag { cursor: pointer; }
.command-input { display: flex; gap: 8px; }
.prompt { color: #22d3ee; font-family: Menlo, Monaco, Consolas, monospace; }
</style>
