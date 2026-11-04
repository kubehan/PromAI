<template>
  <div class="terminal-pane">
    <div ref="containerRef" class="terminal-container"></div>
    <div v-if="status !== 'connected'" class="terminal-overlay">
      <el-icon v-if="status === 'connecting'" class="is-loading" :size="22"><Loading /></el-icon>
      <el-icon v-else :size="22"><CircleClose /></el-icon>
      <span>{{ statusText }}</span>
      <el-button v-if="status === 'closed' || status === 'error'" size="small" type="primary" plain @click="connect">
        重新连接
      </el-button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { nextTick, onActivated, onBeforeUnmount, onDeactivated, onMounted, ref, watch } from 'vue'
import { Terminal } from 'xterm'
import { FitAddon } from 'xterm-addon-fit'
import 'xterm/css/xterm.css'
import type { Host } from '../../types'

const props = defineProps<{ host: Host; active?: boolean }>()
const emit = defineEmits<{
  (e: 'status', status: string): void
  (e: 'connected'): void
}>()

const containerRef = ref<HTMLElement | null>(null)
const status = ref<'connecting' | 'connected' | 'closed' | 'error'>('connecting')
const pageActive = ref(true)
const statusText = ref('正在连接...')

let term: Terminal | null = null
let fitAddon: FitAddon | null = null
let ws: WebSocket | null = null
let resizeObserver: ResizeObserver | null = null
function updateStatus(next: typeof status.value, text: string) {
  status.value = next
  statusText.value = text
  emit('status', next)
}

function connect() {
  if (!props.host?.id) return
  closeSocket()
  updateStatus('connecting', '正在连接...')
  const token = localStorage.getItem('token') || ''
  const protocol = location.protocol === 'https:' ? 'wss:' : 'ws:'
  const cols = term?.cols || 120
  const rows = term?.rows || 30
  const url = `${protocol}//${location.host}/api/promai/hosts/${props.host.id}/terminal?token=${encodeURIComponent(token)}&cols=${cols}&rows=${rows}`
  ws = new WebSocket(url)

  ws.onopen = () => {
    updateStatus('connected', '已连接')
    emit('connected')
    term?.focus()
    sendResize()
  }
  ws.onmessage = evt => {
    try {
      const msg = JSON.parse(evt.data)
      if (msg.type === 'output') {
        term?.write(msg.data || '')
      } else if (msg.type === 'ready') {
        term?.write(`\r\n\x1b[32m● 已连接 ${msg.host || props.host.name}\x1b[0m\r\n`)
      } else if (msg.type === 'error') {
        term?.write(`\r\n\x1b[31m${msg.message || '连接失败'}\x1b[0m\r\n`)
        updateStatus('error', msg.message || '连接失败')
      } else if (msg.type === 'closed') {
        updateStatus('closed', msg.message || '会话已结束')
      }
    } catch {
      term?.write(evt.data)
    }
  }
  ws.onerror = () => {
    if (status.value === 'connected') return
    updateStatus('error', 'WebSocket 连接失败')
  }
  ws.onclose = () => {
    if (status.value !== 'error') updateStatus('closed', '连接已关闭')
  }
}

function sendResize() {
  if (ws?.readyState === WebSocket.OPEN && term) {
    ws.send(JSON.stringify({ type: 'resize', cols: term.cols, rows: term.rows }))
  }
}

function sendCommand(command: string) {
  if (ws?.readyState !== WebSocket.OPEN) return false
  ws.send(JSON.stringify({ type: 'input', data: command + '\n' }))
  term?.focus()
  return true
}

function fit() {
  try {
    fitAddon?.fit()
    sendResize()
  } catch {
    // 容器隐藏时 fit 可能失败，等下次显示再处理
  }
}

function closeSocket() {
  if (ws) {
    ws.onclose = null
    ws.close()
    ws = null
  }
}

onMounted(async () => {
  await nextTick()
  term = new Terminal({
    cursorBlink: true,
    convertEol: false,
    scrollback: 8000,
    fontFamily: 'Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace',
    fontSize: 13,
    lineHeight: 1.2,
    theme: {
      background: '#0b1020',
      foreground: '#d7e2f0',
      cursor: '#00d4ff',
      selectionBackground: '#1f4b6e',
      black: '#1b2436',
      red: '#ff6b81',
      green: '#4ade80',
      yellow: '#fbbf24',
      blue: '#60a5fa',
      magenta: '#c084fc',
      cyan: '#22d3ee',
      white: '#d7e2f0',
    },
  })
  fitAddon = new FitAddon()
  term.loadAddon(fitAddon)
  if (containerRef.value) {
    term.open(containerRef.value)
    fit()
  }
  term.onData(data => {
    if (ws?.readyState === WebSocket.OPEN) {
      ws.send(JSON.stringify({ type: 'input', data }))
    }
  })
  term.onResize(() => sendResize())
  resizeObserver = new ResizeObserver(() => {
    if (pageActive.value && props.active !== false) fit()
  })
  if (containerRef.value) resizeObserver.observe(containerRef.value)
  connect()
})

watch(
  () => props.active,
  async active => {
    if (active) {
      await nextTick()
      fit()
      term?.focus()
    }
  },
)

onActivated(async () => {
  pageActive.value = true
  if (props.active !== false) {
    await nextTick()
    fit()
    term?.focus()
  }
})

onDeactivated(() => {
  pageActive.value = false
})

onBeforeUnmount(() => {
  closeSocket()
  resizeObserver?.disconnect()
  term?.dispose()
  term = null
})

defineExpose({ sendCommand, fit, connect, status })
</script>

<style scoped>
.terminal-pane {
  position: relative;
  width: 100%;
  height: 100%;
  min-height: 320px;
  background: #0b1020;
  overflow: hidden;
}
.terminal-container {
  width: 100%;
  height: 100%;
  padding: 6px 8px;
  box-sizing: border-box;
}
.terminal-overlay {
  position: absolute;
  inset: 0;
  background: rgba(8, 13, 28, 0.72);
  color: #b8c7dd;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 10px;
  font-size: 13px;
  backdrop-filter: blur(2px);
}
.is-loading {
  animation: spin 1s linear infinite;
}
@keyframes spin {
  to { transform: rotate(360deg); }
}
</style>
