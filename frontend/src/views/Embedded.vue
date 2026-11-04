<template>
  <div class="page-container">
    <div class="page-header">
      <div>
        <h2><el-icon><Platform /></el-icon> {{ cfg?.title || '嵌入式页面' }}</h2>
        <p>将 Grafana、Prometheus、业务系统等任意 URL 内嵌到 PromAI 中统一访问</p>
      </div>
      <div class="header-actions">
        <el-button type="primary" @click="addLink">
          <el-icon><Plus /></el-icon>
          添加链接
        </el-button>
        <el-button type="success" plain :loading="saving" @click="persist">保存</el-button>
      </div>
    </div>

    <el-alert
      v-if="cfg && !cfg.enabled"
      type="warning"
      :closable="false"
      title="嵌入式页面未启用"
      description="请在「系统设置 → 嵌入式页面」打开启用开关，侧边栏将显示本页面。"
      show-icon
    />

    <div class="embed-grid">
      <aside v-show="!listCollapsed" class="embed-list">
        <div class="list-title">链接列表</div>
        <div
          v-for="item in cfg?.items"
          :key="item.id"
          class="embed-item"
          :class="{ active: item.id === activeId }"
          @click="load(item)"
        >
          <el-icon class="item-icon"><Link /></el-icon>
          <span class="item-title" :title="item.url">{{ item.title || item.url }}</span>
          <div class="item-actions">
            <el-icon class="act" title="编辑" @click.stop="editLink(item)"><Edit /></el-icon>
            <el-icon class="act danger" title="删除" @click.stop="deleteLink(item)"><Delete /></el-icon>
          </div>
        </div>
        <el-empty v-if="!cfg?.items?.length" description="暂无链接，点右上角添加" :image-size="60" />
      </aside>

      <section ref="mainRef" class="embed-main">
        <template v-if="active">
          <div class="embed-toolbar">
            <el-button text size="small" :title="listCollapsed ? '展开链接列表' : '收起链接列表'" @click="listCollapsed = !listCollapsed">
              <el-icon><Fold v-if="!listCollapsed" /><Expand v-else /></el-icon>
            </el-button>
            <span class="page-name">{{ active.title }}</span>
            <div class="toolbar-actions">
              <el-button text size="small" @click="openExternal(active.url)">
                <el-icon><Position /></el-icon>
                新窗口打开
              </el-button>
              <el-button text size="small" @click="toggleFullscreen">
                <el-icon><FullScreen /></el-icon>
                {{ isFullscreen ? '退出全屏' : '全屏' }}
              </el-button>
            </div>
          </div>
          <iframe
            :src="active.url"
            class="embed-frame"
            allowfullscreen
            allow="clipboard-read; clipboard-write; fullscreen; autoplay; camera; microphone; display-capture"
          ></iframe>
        </template>
        <el-empty v-else description="从左侧选择一个链接内嵌" />
      </section>
    </div>

    <el-dialog :title="editing?.id ? '编辑链接' : '添加链接'" v-model="dialogVisible" width="520px" destroy-on-close>
      <el-form label-width="80px">
        <el-form-item label="名称">
          <el-input v-model="editForm.title" placeholder="例如：Grafana / Prometheus" />
        </el-form-item>
        <el-form-item label="地址">
          <el-input v-model="editForm.url" placeholder="https://grafana.example.com/d/xxx" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" @click="saveItem">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onBeforeUnmount, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { getEmbed, saveEmbed } from '../api'
import type { EmbedConfig, EmbedItem } from '../types'

const cfg = ref<EmbedConfig | null>(null)
const activeId = ref<string>('')
const saving = ref(false)

const active = computed(() => cfg.value?.items.find(i => i.id === activeId.value) || null)

const listCollapsed = ref(false)
const mainRef = ref<HTMLElement | null>(null)
const isFullscreen = ref(false)

async function toggleFullscreen() {
  const el = mainRef.value
  if (!el) return
  try {
    if (document.fullscreenElement) {
      await document.exitFullscreen()
    } else {
      await el.requestFullscreen()
    }
  } catch {
    // 忽略浏览器不支持等情况
  }
}

function handleFullscreenChange() {
  isFullscreen.value = document.fullscreenElement === mainRef.value
}

async function fetchData() {
  try {
    const { data } = await getEmbed()
    cfg.value = data
    if (!activeId.value && data.items.length) activeId.value = data.items[0].id
  } catch (e: any) { ElMessage.error(e.message || '加载嵌入式页面失败') }
}

async function persist() {
  if (!cfg.value) return
  saving.value = true
  try {
    const { data } = await saveEmbed(cfg.value)
    cfg.value = data
    ElMessage.success('已保存')
  } catch (e: any) { ElMessage.error(e.message || '保存失败') }
  finally { saving.value = false }
}

function load(item: EmbedItem) {
  activeId.value = item.id
}

function openExternal(url: string) {
  window.open(url, '_blank', 'noopener')
}

const dialogVisible = ref(false)
const editing = ref<EmbedItem | null>(null)
const editForm = ref<{ title: string; url: string }>({ title: '', url: '' })

function addLink() {
  editing.value = null
  editForm.value = { title: '', url: '' }
  dialogVisible.value = true
}

function editLink(item: EmbedItem) {
  editing.value = item
  editForm.value = { title: item.title, url: item.url }
  dialogVisible.value = true
}

async function saveItem() {
  if (!editForm.value.url) { ElMessage.warning('请输入地址'); return }
  if (!cfg.value) return
  if (editing.value) {
    editing.value.title = editForm.value.title
    editing.value.url = editForm.value.url
  } else {
    const item: EmbedItem = {
      id: '',
      title: editForm.value.title || editForm.value.url,
      url: editForm.value.url,
      sort: cfg.value.items.length + 1,
    }
    cfg.value.items.push(item)
    activeId.value = item.id
  }
  dialogVisible.value = false
  await persist()
  await fetchData()
}

async function deleteLink(item: EmbedItem) {
  if (!cfg.value) return
  try {
    await ElMessageBox.confirm(`确定删除「${item.title || item.url}」吗？`, '删除确认', { type: 'warning' })
    cfg.value.items = cfg.value.items.filter(i => i.id !== item.id)
    if (activeId.value === item.id) activeId.value = cfg.value.items[0]?.id || ''
    await persist()
  } catch (e: any) {
    if (e !== 'cancel') ElMessage.error(e.message || '删除失败')
  }
}

onMounted(() => {
  document.addEventListener('fullscreenchange', handleFullscreenChange)
  fetchData()
})
onBeforeUnmount(() => {
  document.removeEventListener('fullscreenchange', handleFullscreenChange)
})
</script>

<style scoped>
.page-header { display: flex; justify-content: space-between; align-items: flex-start; gap: 16px; flex-wrap: wrap; }
.header-actions { display: flex; align-items: center; gap: 10px; }
.enable-label { font-size: 13px; opacity: .7; }
.embed-grid {
  display: flex; gap: 14px; margin-top: 18px; height: calc(100vh - 240px); min-height: 420px;
}
.embed-list {
  width: 260px; flex: none; overflow-y: auto;
  background: var(--bg-card, rgba(255, 255, 255, .03));
  border: 1px solid var(--border-color, rgba(255, 255, 255, .08));
  border-radius: 10px; padding: 10px;
}
.list-title { font-size: 13px; opacity: .7; margin: 4px 4px 10px; }
.embed-item {
  display: flex; align-items: center; gap: 8px; padding: 9px 10px; border-radius: 8px;
  cursor: pointer; transition: background .15s;
}
.embed-item:hover { background: rgba(64, 158, 255, .08); }
.embed-item.active { background: rgba(64, 158, 255, .16); }
.item-icon { color: #22d3ee; }
.item-title { flex: 1; min-width: 0; font-size: 13px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.item-actions { display: flex; gap: 4px; opacity: 0; transition: opacity .15s; }
.embed-item:hover .item-actions { opacity: .8; }
.act { font-size: 14px; padding: 2px; border-radius: 4px; }
.act:hover { color: #60a5fa; background: rgba(64, 158, 255, .16); }
.act.danger:hover { color: #ef4444; background: rgba(239, 68, 68, .15); }
.embed-main {
  flex: 1; min-width: 0; display: flex; flex-direction: column;
  background: var(--bg-card, rgba(255, 255, 255, .03));
  border: 1px solid var(--border-color, rgba(255, 255, 255, .08));
  border-radius: 10px; overflow: hidden;
}
.embed-toolbar {
  display: flex; align-items: center; justify-content: space-between;
  padding: 8px 12px; border-bottom: 1px solid var(--border-color, rgba(255, 255, 255, .08));
}
.toolbar-actions { display: flex; align-items: center; gap: 4px; }
.embed-main:fullscreen { width: 100vw; height: 100vh; box-sizing: border-box; background: #0b1020; }
.page-name { font-size: 13px; font-weight: 600; }
.embed-frame { flex: 1; width: 100%; border: none; background: #0b1020; }
</style>