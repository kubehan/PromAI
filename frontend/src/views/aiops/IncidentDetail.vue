<template>
  <div class="page-container">
    <div class="page-header">
      <div>
        <h2>故障详情</h2>
        <p>概览、时间线、AI 排查联动</p>
      </div>
      <div class="header-actions">
        <el-button @click="goBack">返回</el-button>
        <el-button type="primary" @click="toTerminal">进入终端排查</el-button>
      </div>
    </div>

    <div v-if="incident" class="detail-grid">
      <div class="card left-col">
        <div class="card-title">故障概览</div>
        <div class="kv">
          <span class="k">故障编号</span><span class="v">{{ incident.incident_no || incident.id }}</span>
        </div>
        <div class="kv"><span class="k">标题</span><span class="v">{{ incident.title }}</span></div>
        <div class="kv"><span class="k">状态</span><span class="v"><el-tag :type="statusType" size="small">{{ statusText }}</el-tag></span></div>
        <div class="kv"><span class="k">严重度</span><span class="v">{{ incident.severity || '-' }}</span></div>
        <div class="kv"><span class="k">负责人</span><span class="v">{{ incident.assignee_name || '-' }}</span></div>
        <div class="kv"><span class="k">开始时间</span><span class="v">{{ fmt(incident.starts_at) }}</span></div>
        <div class="kv"><span class="k">关闭时间</span><span class="v">{{ fmt(incident.closed_at) }}</span></div>
        <div class="kv" v-if="incident.summary"><span class="k">描述</span><span class="v pre">{{ incident.summary }}</span></div>
        <div class="card-actions">
          <el-button size="small" type="success" @click="act('ack')" :disabled="isDone">确认</el-button>
          <el-button size="small" type="warning" @click="act('resolve')">解决</el-button>
          <el-button size="small" type="danger" @click="act('close')">关闭</el-button>
        </div>
      </div>

      <div class="card right-col">
        <div class="card-title">时间线 / AI 排查联动</div>
        <div v-if="timeline.length===0" class="empty">暂无时间线记录</div>
        <div class="tl-item" v-for="t in timeline" :key="t.id">
          <div class="tl-node" :class="typeClass(t.type)"></div>
          <div class="tl-body">
            <div class="tl-head">
              <span class="tl-title">{{ t.title || t.content }}</span>
              <el-button v-if="sessionId(t)" size="small" text type="primary" @click="openSession(t)">进入终端</el-button>
            </div>
            <div class="tl-meta">{{ t.actor_name || '系统' }} · {{ fmt(t.created_at) }} · {{ typeText(t.type) }}</div>
            <div class="tl-content pre">{{ t.content }}</div>
            <div v-if="metaContent(t)" class="tl-content pre muted">{{ metaContent(t) }}</div>
          </div>
        </div>
      </div>
    </div>
    <div v-else class="loading">加载中...</div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import axios from 'axios'
import { ElMessage } from 'element-plus'

const route = useRoute()
const router = useRouter()
const id = route.params.id as string
const incident = ref<any>(null)
const timeline = ref<any[]>([])

const statusMap:any = { open:'info', new:'info', investigating:'warning', acknowledged:'warning', resolved:'success', closed:'info' }
const statusTextMap:any = { open:'进行中', new:'新建', investigating:'排查中', acknowledged:'已确认', resolved:'已解决', closed:'已关闭' }
const typeTextMap:any = { event_created:'事件创建', ack:'确认', assign:'指派', comment:'评论', escalate:'升级', ai:'AI 总结', ai_rca:'AI 根因', action:'操作', resolve:'解决', close:'关闭', runbook:'预案' }

const statusText = computed(()=> statusTextMap[incident.value?.status] || incident.value?.status || '-')
const statusType = computed(()=> statusMap[incident.value?.status] || 'info')
const isDone = computed(()=> ['resolved','closed'].includes(incident.value?.status))

const fmt = (t:string) => t ? new Date(t).toLocaleString() : '-'
const typeClass = (t:string) => (t==='ai'||t==='ai_rca') ? 'ai' : 'normal'
const typeText = (t:string)=> typeTextMap[t]||t||'-'
const sessionId = (t:any) => { try { const m = JSON.parse(t.meta_json||'{}'); return m.session_id || '' } catch(e){ return '' } }
const metaContent = (t:any) => { try { const m = JSON.parse(t.meta_json||'{}'); return Object.keys(m).length ? JSON.stringify(m) : '' } catch(e){ return '' } }

const load = async () => {
  try {
    const res = await axios.get(`/api/promai/aiops/incidents/${id}`)
    incident.value = res.data?.incident || null
    timeline.value = res.data?.timeline || []
  } catch(e:any) { ElMessage.error(e.message) }
}
const act = async (a:string) => {
  try {
    await axios.post(`/api/promai/aiops/incidents/${id}/${a}`, {})
    ElMessage.success('操作成功')
    load()
  } catch(e:any) { ElMessage.error(e.message) }
}
const goBack = () => router.push('/aiops/incidents')
const toTerminal = () => router.push('/ai-terminal')
const openSession = (t:any) => {
  const sid = sessionId(t)
  if (sid) router.push({ path:'/ai-terminal', query: { session: sid } })
}

onMounted(load)
</script>

<style scoped>
.page-container { padding: 16px; }
.page-header { display:flex; align-items:center; justify-content:space-between; margin-bottom:16px; }
.page-header h2 { margin:0; font-size:20px; }
.page-header p { margin:4px 0 0; color:#6b7280; font-size:13px; }
.header-actions { display:flex; gap:8px; }
.detail-grid { display:grid; grid-template-columns: 340px 1fr; gap:16px; }
.card { background:#fff; border:1px solid #e5e7eb; border-radius:12px; padding:16px; }
.card-title { font-weight:600; font-size:14px; margin-bottom:12px; }
.kv { display:flex; gap:8px; margin-bottom:8px; font-size:13px; }
.kv .k { width:70px; color:#6b7280; flex-shrink:0; }
.kv .v { color:#111827; word-break:break-all; }
.kv .v.pre { white-space:pre-wrap; }
.card-actions { margin-top:16px; display:flex; gap:8px; }
.tl-item { display:flex; gap:12px; position:relative; padding-bottom:18px; }
.tl-item:not(:last-child)::before { content:''; position:absolute; left:5px; top:14px; bottom:0; width:2px; background:#e5e7eb; }
.tl-node { width:12px; height:12px; border-radius:50%; background:#60a5fa; margin-top:3px; flex-shrink:0; }
.tl-node.ai { background:#a855f7; }
.tl-head { display:flex; align-items:center; justify-content:space-between; gap:8px; }
.tl-title { font-weight:600; font-size:13px; color:#111827; }
.tl-meta { font-size:11px; color:#9ca3af; margin-top:2px; }
.tl-content { font-size:13px; color:#374151; margin-top:4px; line-height:1.6; }
.tl-content.pre { white-space:pre-wrap; }
.tl-content.muted { color:#6b7280; font-family:monospace; font-size:11px; }
.empty, .loading { color:#9ca3af; font-size:13px; padding:12px; }
</style>