<template>
  <div class="page-container ai-chat-page">
    <div class="page-header">
      <h2><el-icon><MagicStick /></el-icon> AI 智能助手</h2>
      <div class="header-controls">
        <p>自然语言查询指标、分析告警、管理系统</p>
        <div class="model-selector" v-if="modelOptions.length > 0">
          <span class="model-label">当前模型：</span>
          <el-select v-model="currentModel" size="small" style="width: 180px;">
            <el-option v-for="m in modelOptions" :key="m.name" :label="m.name" :value="m.name" />
          </el-select>
        </div>
      </div>
    </div>

    <div class="chat-body">
      <div class="chat-layout">
        <div class="chat-main" ref="chatRef">
          <div v-if="messages.length === 0" class="chat-empty">
            <div class="empty-icon">
              <el-icon :size="48"><MagicStick /></el-icon>
            </div>
            <h3>PromAI 智能助手</h3>
            <p class="empty-desc">你可以问我：</p>
            <div class="suggestions">
              <div class="suggestion-chip" @click="sendSuggestion('查一下所有数据源的状态')">
                查一下所有数据源的状态
              </div>
              <div class="suggestion-chip" @click="sendSuggestion('最近有哪些巡检报告异常？')">
                最近有哪些巡检报告异常？
              </div>
              <div class="suggestion-chip" @click="sendSuggestion('帮我分析一下CPU使用率告警')">
                帮我分析一下CPU使用率告警
              </div>
              <div class="suggestion-chip" @click="sendSuggestion('查询 production 集群最近24小时的CPU使用率')">
                查询 production 集群的CPU使用率
              </div>
            </div>
          </div>

          <div v-for="(msg, idx) in messages" :key="idx" class="chat-msg" :class="msg.role">
            <div class="msg-avatar">
              <el-icon v-if="msg.role === 'user'"><User /></el-icon>
              <el-icon v-else><MagicStick /></el-icon>
            </div>
            <div class="msg-content">
              <div class="msg-sender">{{ msg.role === 'user' ? '你' : 'AI 助手' }}</div>
              <div class="msg-text" v-html="renderContent(msg.content)"></div>
              <div v-if="msg.tools && msg.tools.length" class="msg-tools">
                <div v-for="(tool, ti) in msg.tools" :key="ti" class="tool-call" :class="tool.status">
                  <el-icon v-if="tool.status === 'running'"><Loading /></el-icon>
                  <el-icon v-else-if="tool.status === 'done'"><CircleCheck /></el-icon>
                  <el-icon v-else><CircleClose /></el-icon>
                  <span>{{ tool.name }}</span>
                </div>
              </div>
            </div>
          </div>

          <div v-if="loading" class="chat-msg assistant">
            <div class="msg-avatar">
              <el-icon><MagicStick /></el-icon>
            </div>
            <div class="msg-content">
              <div class="msg-sender">AI 助手</div>
              <div class="msg-text">
                <span class="thinking-dots"><span>.</span><span>.</span><span>.</span></span>
              </div>
            </div>
          </div>
        </div>

        <div class="chat-input-bar">
          <el-input
            v-model="inputText"
            type="textarea"
            :rows="2"
            placeholder="输入你的问题，例如：查一下所有数据源的健康状态"
            :disabled="loading"
            @keydown.enter.prevent="sendMessage"
          />
          <el-button type="primary" :loading="loading" @click="sendMessage" class="send-btn">
            <el-icon><Promotion /></el-icon>
            发送
          </el-button>
        </div>
      </div>

      <div class="chat-sidebar" v-if="sessions.length > 0">
        <div class="sidebar-title">历史会话</div>
        <div v-for="s in sessions" :key="s.id" class="session-item" @click="switchSession(s.id)">
          <el-icon :size="14"><ChatDotRound /></el-icon>
          <div class="session-info">
            <span class="session-name">{{ s.model_name || shortId(s.id) }}</span>
            <span class="session-meta">{{ s.msg_count }} 条消息</span>
          </div>
          <el-button text size="small" @click.stop="deleteSession(s.id)">
            <el-icon :size="12"><Close /></el-icon>
          </el-button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, nextTick } from 'vue'
import { marked } from 'marked'
import { getAiSessions, deleteAiSession, getSettings } from '../api'

marked.setOptions({
  breaks: true,
  gfm: true,
})

interface ToolCall {
  name: string
  status: 'running' | 'done' | 'error'
}

interface ChatMessage {
  role: 'user' | 'assistant'
  content: string
  tools?: ToolCall[]
}

const inputText = ref('')
const messages = ref<ChatMessage[]>([])
const loading = ref(false)
const sessionId = ref('')
const sessions = ref<{id: string; created_at: string; updated_at: string; msg_count: number; model_name?: string}[]>([])
const currentModel = ref('')
const modelOptions = ref<{name: string}[]>([])
const chatRef = ref<HTMLElement | null>(null)

function shortId(id: string) {
  return id.length > 12 ? id.slice(0, 12) + '...' : id
}

function renderContent(text: string): string {
  if (!text) return ''
  return marked.parse(text) as string
}

async function sendMessage() {
  const text = inputText.value.trim()
  if (!text || loading.value) return
  inputText.value = ''
  messages.value.push({ role: 'user', content: text })
  loading.value = true
  scrollToBottom()

  const currentTools = ref<ToolCall[]>([])

  try {
    const resp = await fetch('/api/promai/ai/chat', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'Authorization': `Bearer ${localStorage.getItem('token')}`,
      },
      body: JSON.stringify({ message: text, session_id: sessionId.value, model_name: currentModel.value }),
    })

    if (!resp.ok) {
      throw new Error('请求失败')
    }

    const reader = resp.body?.getReader()
    if (!reader) throw new Error('无法读取响应流')

    const decoder = new TextDecoder()
    let buffer = ''

    messages.value.push({ role: 'assistant', content: '', tools: [] })
    const msgIdx = messages.value.length - 1

    while (true) {
      const { done, value } = await reader.read()
      if (done) break

      buffer += decoder.decode(value, { stream: true })
      const lines = buffer.split('\n')
      buffer = lines.pop() || ''

      for (const line of lines) {
        if (!line.startsWith('data: ')) continue
        try {
          const data = JSON.parse(line.slice(6))
          switch (data.type) {
            case 'session_id':
              sessionId.value = data.session_id
              break
            case 'text':
              messages.value[msgIdx].content += data.content
              scrollToBottom()
              break
            case 'tool_start':
              currentTools.value.push({ name: data.tool_name, status: 'running' })
              messages.value[msgIdx].tools = [...currentTools.value]
              break
            case 'tool_end':
              const tool = currentTools.value.find(t => t.name === data.tool_name)
              if (tool) tool.status = 'done'
              messages.value[msgIdx].tools = [...currentTools.value]
              break
            case 'done':
              loading.value = false
              loadSessions()
              break
            case 'error':
              messages.value[msgIdx].content += `\n\n❌ 错误: ${data.content}`
              loading.value = false
              break
          }
        } catch { /* ignore parse errors */ }
      }
    }
  } catch (e: any) {
    messages.value.push({ role: 'assistant', content: `❌ 请求失败: ${e.message}` })
  } finally {
    loading.value = false
    scrollToBottom()
  }
}

function sendSuggestion(text: string) {
  inputText.value = text
  sendMessage()
}

function scrollToBottom() {
  nextTick(() => {
    if (chatRef.value) {
      chatRef.value.scrollTop = chatRef.value.scrollHeight
    }
  })
}

async function loadSessions() {
  try {
    const res = await getAiSessions()
    sessions.value = res.data
  } catch { /* ignore */ }
}

async function switchSession(id: string) {
  sessionId.value = id
  messages.value = []
  try {
    const res = await fetch(`/api/promai/ai/sessions/${id}`, {
      headers: { 'Authorization': `Bearer ${localStorage.getItem('token')}` },
    })
    if (!res.ok) throw new Error('加载失败')
    const data = await res.json()
    for (const m of data.messages) {
      messages.value.push({ role: m.role, content: m.content })
    }
  } catch { /* ignore */ }
}

async function deleteSession(id: string) {
  try {
    await deleteAiSession(id)
    await loadSessions()
    if (sessionId.value === id) {
      sessionId.value = ''
      messages.value = []
    }
  } catch { /* ignore */ }
}

async function loadModels() {
  try {
    const res = await getSettings()
    const raw = res.data.ai_models
    if (raw) {
      const list = JSON.parse(raw)
      modelOptions.value = list
      if (list.length > 0 && !currentModel.value) {
        currentModel.value = res.data.ai_default_model || list[0].name
      }
    }
  } catch {}
}

onMounted(() => {
  loadSessions()
  loadModels()
})
</script>

<style scoped>
.ai-chat-page {
  display: flex;
  flex-direction: column;
  height: 100%;
  position: relative;
  overflow: hidden;
}

.chat-body {
  display: flex;
  flex: 1;
  gap: 16px;
  min-height: 0;
  overflow: hidden;
}

.chat-layout {
  display: flex;
  flex-direction: column;
  flex: 1;
  overflow: hidden;
  min-height: 0;
}

.chat-main {
  flex: 1;
  overflow-y: auto;
  padding: 0 0 16px 0;
  display: flex;
  flex-direction: column;
  gap: 16px;
  min-height: 0;
}

.chat-empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  flex: 1;
  color: var(--text-tertiary);
}

.empty-icon {
  width: 80px;
  height: 80px;
  border-radius: 20px;
  background: linear-gradient(135deg, var(--cyan-dim), var(--purple-dim));
  display: flex;
  align-items: center;
  justify-content: center;
  margin-bottom: 16px;
  color: var(--cyan);
}

.chat-empty h3 {
  color: var(--text-primary);
  font-size: 20px;
  margin: 0 0 8px;
}

.header-controls {
  display: flex;
  align-items: center;
  gap: 16px;
}
.header-controls p {
  margin: 0;
  color: var(--text-tertiary);
}
.model-selector {
  display: flex;
  align-items: center;
  gap: 4px;
  margin-left: auto;
}
.model-label {
  font-size: 13px;
  color: var(--text-tertiary);
  white-space: nowrap;
}

.empty-desc {
  color: var(--text-tertiary);
  margin-bottom: 16px;
}

.suggestions {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  justify-content: center;
  max-width: 600px;
}

.suggestion-chip {
  padding: 8px 16px;
  border-radius: 20px;
  background: var(--cyan-dim);
  border: 1px solid var(--border-glow);
  color: var(--cyan);
  font-size: 13px;
  cursor: pointer;
  transition: all 0.2s;
}

.suggestion-chip:hover {
  background: var(--cyan-dim);
  filter: brightness(1.2);
  border-color: var(--cyan);
  transform: translateY(-1px);
}

.chat-msg {
  display: flex;
  gap: 12px;
  padding: 0 16px;
}

.chat-msg.user {
  flex-direction: row-reverse;
}

.msg-avatar {
  width: 36px;
  height: 36px;
  border-radius: 10px;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  font-size: 16px;
}

.user .msg-avatar {
  background: linear-gradient(135deg, var(--cyan), var(--purple));
  color: white;
}

.assistant .msg-avatar {
  background: rgba(124,58,237,0.2);
  color: var(--purple);
}

.msg-content {
  max-width: 70%;
}

.user .msg-content {
  text-align: right;
}

.msg-sender {
  font-size: 12px;
  color: var(--text-tertiary);
  margin-bottom: 4px;
}

.msg-text {
  padding: 12px 16px;
  border-radius: 12px;
  font-size: 14px;
  line-height: 1.6;
  word-break: break-word;
}

.user .msg-text {
  background: linear-gradient(135deg, var(--cyan-dim), var(--purple-dim));
  color: var(--text-primary);
  border: 1px solid var(--border-glow);
}

.assistant .msg-text {
  background: var(--bg-elevated);
  border: 1px solid var(--border);
  color: var(--text-primary);
}

.msg-text :deep(p) {
  margin: 0 0 8px;
}
.msg-text :deep(p:last-child) {
  margin-bottom: 0;
}
.msg-text :deep(code) {
  background: var(--cyan-dim);
  padding: 2px 6px;
  border-radius: 4px;
  font-size: 13px;
  color: var(--cyan);
}
.msg-text :deep(pre) {
  background: var(--bg-elevated);
  border: 1px solid var(--border);
  border-radius: 8px;
  padding: 12px 16px;
  overflow-x: auto;
  margin: 8px 0;
}
.msg-text :deep(pre code) {
  background: none;
  padding: 0;
  color: inherit;
  font-size: 13px;
}
.msg-text :deep(ul),
.msg-text :deep(ol) {
  padding-left: 20px;
  margin: 4px 0;
}
.msg-text :deep(li) {
  margin: 2px 0;
}
.msg-text :deep(blockquote) {
  border-left: 3px solid var(--cyan);
  padding-left: 12px;
  margin: 8px 0;
  color: var(--text-secondary);
}
.msg-text :deep(table) {
  border-collapse: collapse;
  width: 100%;
  margin: 8px 0;
  font-size: 13px;
}
.msg-text :deep(th),
.msg-text :deep(td) {
  border: 1px solid var(--border);
  padding: 6px 10px;
  text-align: left;
}
.msg-text :deep(th) {
  background: var(--bg-elevated);
  font-weight: 600;
}
.msg-text :deep(hr) {
  border: none;
  border-top: 1px solid var(--border);
  margin: 12px 0;
}
.msg-text :deep(h1),
.msg-text :deep(h2),
.msg-text :deep(h3),
.msg-text :deep(h4) {
  margin: 12px 0 6px;
  color: var(--text-primary);
}
.msg-text :deep(a) {
  color: var(--cyan);
  text-decoration: underline;
}

.msg-tools {
  display: flex;
  gap: 6px;
  margin-top: 8px;
  flex-wrap: wrap;
}

.tool-call {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 4px 10px;
  border-radius: 6px;
  font-size: 12px;
  background: var(--bg-elevated);
  border: 1px solid var(--border);
  color: var(--text-tertiary);
}

.tool-call.running {
  color: var(--cyan);
  border-color: var(--border-glow);
}

.tool-call.done {
  color: var(--emerald);
  border-color: var(--emerald-dim);
}

.thinking-dots span {
  animation: blink 1.4s infinite;
  font-size: 24px;
  line-height: 1;
}

.thinking-dots span:nth-child(2) { animation-delay: 0.2s; }
.thinking-dots span:nth-child(3) { animation-delay: 0.4s; }

@keyframes blink {
  0%, 80%, 100% { opacity: 0; }
  40% { opacity: 1; }
}

.chat-input-bar {
  display: flex;
  gap: 12px;
  padding: 16px 0;
  border-top: 1px solid var(--border);
  align-items: flex-end;
}

.chat-input-bar .el-input {
  flex: 1;
}

.send-btn {
  height: 60px;
  min-width: 90px;
}

.chat-sidebar {
  width: 200px;
  padding: 16px;
  border-left: 1px solid var(--border);
  overflow-y: auto;
}

.sidebar-title {
  font-size: 13px;
  color: var(--text-tertiary);
  margin-bottom: 12px;
  font-weight: 600;
}

.session-item {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 10px;
  border-radius: 8px;
  cursor: pointer;
  font-size: 12px;
  color: var(--text-secondary);
  transition: all 0.2s;
}

.session-item:hover {
  background: var(--cyan-dim);
  color: var(--text-primary);
}

.session-info {
  flex: 1;
  overflow: hidden;
  min-width: 0;
}

.session-name {
  display: block;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 12px;
}

.session-meta {
  display: block;
  font-size: 11px;
  color: var(--text-tertiary);
  margin-top: 2px;
}
</style>
