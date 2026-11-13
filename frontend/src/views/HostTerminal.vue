<template>
  <div class="terminal-page">
    <aside v-show="!sidebarCollapsed" class="host-sidebar" :style="{ width: sidebarWidth + 'px' }">
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
    <div
      v-show="!sidebarCollapsed"
      class="resize-handle v"
      title="拖动调整主机列表宽度"
      @pointerdown="startResize('sidebar', $event)"
    ></div>

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
          <el-button text size="small" @click="toggleAiPosition">{{ aiOnLeft ? '会话在右侧' : '会话在左侧' }}</el-button>
          <el-button text size="small" :type="aiOpen ? 'primary' : ''" @click="toggleAi">
            <el-icon><Cpu /></el-icon>
            AI 助手
          </el-button>
          <el-button text size="small" @click="toggleFullscreen">
            <el-icon><FullScreen /></el-icon>
            {{ isFullscreen ? '退出全屏' : '全屏' }}
          </el-button>
        </div>
      </div>

      <div class="terminal-body-wrap">
        <template v-if="aiOpen && activeHost && aiOnLeft">
          <aside class="ai-panel" :style="{ width: aiWidth + 'px' }">
            <div class="ai-header">
              <span><el-icon><Cpu /></el-icon> AI 助手 · {{ activeHost.name }}</span>
              <div class="ai-header-actions">
                <el-select v-model="selectedSessionId" size="small" placeholder="历史会话" class="ai-session-select" @change="switchSession">
                  <el-option v-for="s in sessionList" :key="s.id" :label="(s.title||'会话#'+s.id)+' · '+ (s.created_at||'').slice(0,10)" :value="s.id"/>
                </el-select>
                <el-button text size="small" @click="newAiSession">新建</el-button>
                <el-button text size="small" @click="exportSession">导出</el-button>
                <el-button text size="small" @click="shareAiSession">分享</el-button>
                <el-icon class="ai-close" title="关闭" @click="toggleAi"><Close /></el-icon>
              </div>
            </div>

            <div ref="aiChatRef" class="ai-chat">
              <div v-if="!aiMessages.length" class="ai-hint">
                <el-icon :size="34"><Cpu /></el-icon>
                <p>描述问题，AI 会分析并自动把放行的命令发送到终端执行，读取结果后继续分析。</p>
                <p class="ai-hint-sub">描述问题，AI 会分析并自动把放行的命令发送到终端执行，读取结果后继续分析。</p>
                <div class="ai-suggests">
                  <button class="ai-suggest" @click="useSuggestion('梳理该机器部署的服务及运行监听的端口，访问方式，占用磁盘大小')">服务梳理</button>
                  <button class="ai-suggest" @click="useSuggestion('查询磁盘使用情况')">磁盘使用</button>
                  <button class="ai-suggest" @click="useSuggestion('查询内存和 CPU 负载')">资源负载</button>
                  <button class="ai-suggest" @click="useSuggestion('查看网络连接和监听端口')">网络端口</button>
                  <button class="ai-suggest" @click="useSuggestion('查看系统进程列表，按内存排序')">进程列表</button>
                  <button class="ai-suggest" @click="useSuggestion('查看系统日志最近 20 条关键记录')">系统日志</button>
                  <button class="ai-suggest" @click="useSuggestion('查看最近登录记录和登录失败记录')">登录审计</button>
                  <button class="ai-suggest" @click="useSuggestion('查看定时任务列表')">定时任务</button>
                </div>
              </div>

              <div v-for="m in aiMessages" :key="m.id" class="ai-msg" :class="'role-' + m.role">
                <template v-if="m.role === 'user'">
                  <div class="bubble user-bubble">{{ m.text }}</div>
                </template>

                <template v-else-if="m.role === 'assistant'">
                  <div class="msg-role">AI 分析</div>
                  <div class="bubble assistant-bubble" :class="{ collapsible: isLongText(m.text) && !m.expanded }">{{ m.text }}</div>
                  <button v-if="isLongText(m.text)" class="expand-toggle" @click="toggleExpand(m)">
                    {{ m.expanded ? '收起' : '展开全文' }}
                  </button>
                </template>

                <template v-else-if="m.role === 'command'">
                  <div class="cmd-card" :class="m.status">
                    <div class="cmd-line">
                      <span class="cmd-sign">$</span>
                      <span class="cmd-text" :class="{ collapsible: isLongText(m.command, 3, 120) && !m.expanded }">{{ m.command }}</span>
                    </div>
                    <button v-if="isLongText(m.command, 3, 120)" class="expand-toggle" @click="toggleExpand(m)">
                      {{ m.expanded ? '收起命令' : '展开命令' }}
                    </button>
                    <div class="cmd-meta">
                      <el-tag size="small" :type="riskType(m.risk)" effect="plain">{{ riskText(m.risk) }}</el-tag>
                      <span class="cmd-status" :class="m.status">{{ cmdStatusText(m.status) }}</span>
                    </div>
                    <div v-if="m.note" class="cmd-note" :class="{ collapsible: isLongText(m.note, 3, 160) && !m.expanded }">{{ m.note }}</div>
                    <div v-if="m.status === 'failed'" class="cmd-retry">
                      <template v-if="retryId === m.id">
                        <el-input v-model="retryCmd" size="small" class="retry-input"/>
                        <el-button size="small" type="primary" @click="doRetry(m)">执行</el-button>
                        <el-button size="small" @click="retryId=0">取消</el-button>
                      </template>
                      <el-button v-else size="small" type="warning" plain @click="startRetry(m)">编辑并重试</el-button>
                    </div>
                    <div v-if="m.stepId && m.status === 'pending' && m.risk === 'high'" class="cmd-approve">
                      <el-button size="small" type="primary" @click="approveMsg(m)">批准执行</el-button>
                      <el-button size="small" type="danger" plain @click="rejectMsg(m)">拒绝</el-button>
                    </div>
                  </div>
                </template>

                <template v-else-if="m.role === 'result'">
                  <div class="result-card">
                    <div class="result-head">
                      <span>执行结果</span>
                      <span class="result-exit" :class="m.exitCode === 0 ? 'ok' : 'bad'">exit {{ m.exitCode }}</span>
                    </div>
                    <pre class="result-out" :class="{ collapsible: isLongText(m.text, 10, 600) && !m.expanded }">{{ m.text || '（无输出）' }}</pre>
                    <button v-if="isLongText(m.text, 10, 600)" class="expand-toggle" @click="toggleExpand(m)">
                      {{ m.expanded ? '收起输出' : '展开输出' }}
                    </button>
                  </div>
                </template>

                <template v-else>
                  <div class="system-bubble">{{ m.text }}</div>
                </template>
              </div>

              <div v-if="aiThinking" class="ai-msg role-assistant">
                <div class="msg-role">AI 分析</div>
                <div class="bubble assistant-bubble thinking">
                  <el-icon class="is-loading"><Loading /></el-icon>
                  <span>{{ aiThinkingText }}</span>
                </div>
              </div>
            </div>

            <div class="ai-input">
              <div class="ai-quick-actions">
                <button class="ai-quick" @click="useSuggestion('查询磁盘使用情况')">磁盘</button>
                <button class="ai-quick" @click="useSuggestion('查询内存使用情况')">内存</button>
                <button class="ai-quick" @click="useSuggestion('查询 CPU 负载情况')">CPU</button>
                <button class="ai-quick" @click="useSuggestion('查看网络连接状态')">网络</button>
                <button class="ai-quick" @click="useSuggestion('查看系统进程列表')">进程</button>
                <button class="ai-quick" @click="useSuggestion('查看系统服务和运行状态')">服务</button>
              </div>
              <el-input
                v-model="aiIntent"
                type="textarea"
                :rows="2"
                resize="none"
                :disabled="aiBusy"
                placeholder="描述问题或继续追问，Enter 发送，Shift+Enter 换行"
                @keydown.enter.exact.prevent="sendAi"
              />
              <div class="ai-input-actions">
                <span class="ai-tip">低危自动执行 · 中高危需确认</span>
                <div class="ai-input-btns">
                  <el-button v-if="aiBusy" size="small" type="danger" plain @click="cancelAgent">停止</el-button>
                  <el-button size="small" type="primary" :loading="aiBusy" :disabled="aiBusy" @click="sendAi">发送</el-button>
                </div>
              </div>
            </div>
          </aside>

          <div
            class="resize-handle v"
            title="拖动调整 AI 会话区与终端宽度"
            @pointerdown="startResize('ai', $event)"
          ></div>
        </template>

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

        <div
          v-if="aiOpen && activeHost && !aiOnLeft"
          class="resize-handle v"
          title="拖动调整 AI 会话区与终端宽度"
          @pointerdown="startResize('ai', $event)"
        ></div>

        <aside v-if="aiOpen && activeHost && !aiOnLeft" class="ai-panel" :style="{ width: aiWidth + 'px' }">
          <div class="ai-header">
            <span><el-icon><Cpu /></el-icon> AI 助手 · {{ activeHost.name }}</span>
            <div class="ai-header-actions">
              <el-select v-model="selectedSessionId" size="small" placeholder="历史会话" class="ai-session-select" @change="switchSession">
                <el-option v-for="s in sessionList" :key="s.id" :label="(s.title||'会话#'+s.id)+' · '+ (s.created_at||'').slice(0,10)" :value="s.id"/>
              </el-select>
              <el-button text size="small" @click="newAiSession">新建</el-button>
              <el-button text size="small" @click="exportSession">导出</el-button>
              <el-button text size="small" @click="shareAiSession">分享</el-button>
              <el-icon class="ai-close" title="关闭" @click="toggleAi"><Close /></el-icon>
            </div>
          </div>

          <div ref="aiChatRef" class="ai-chat">
            <div v-if="!aiMessages.length" class="ai-hint">
              <el-icon :size="34"><Cpu /></el-icon>
              <p>描述问题，AI 会分析并自动把放行的命令发送到终端执行，读取结果后继续分析。</p>
              <p class="ai-hint-sub">描述问题，AI 会分析并自动把放行的命令发送到终端执行，读取结果后继续分析。</p>
              <div class="ai-suggests">
                <button class="ai-suggest" @click="useSuggestion('先检查系统负载和磁盘占用，再给我结论建议')">资源体检</button>
                <button class="ai-suggest" @click="useSuggestion('排查最近 15 分钟错误日志并总结风险')">日志排障</button>
                <button class="ai-suggest" @click="useSuggestion('帮我做一次安全检查，只输出需要处理的问题')">安全检查</button>
              </div>
            </div>

            <div v-for="m in aiMessages" :key="m.id" class="ai-msg" :class="'role-' + m.role">
              <template v-if="m.role === 'user'">
                <div class="bubble user-bubble">{{ m.text }}</div>
              </template>

              <template v-else-if="m.role === 'assistant'">
                <div class="msg-role">AI 分析</div>
                <div class="bubble assistant-bubble" :class="{ collapsible: isLongText(m.text) && !m.expanded }">{{ m.text }}</div>
                <button v-if="isLongText(m.text)" class="expand-toggle" @click="toggleExpand(m)">
                  {{ m.expanded ? '收起' : '展开全文' }}
                </button>
              </template>

              <template v-else-if="m.role === 'command'">
                <div class="cmd-card" :class="m.status">
                  <div class="cmd-line">
                    <span class="cmd-sign">$</span>
                    <span class="cmd-text" :class="{ collapsible: isLongText(m.command, 3, 120) && !m.expanded }">{{ m.command }}</span>
                  </div>
                  <button v-if="isLongText(m.command, 3, 120)" class="expand-toggle" @click="toggleExpand(m)">
                    {{ m.expanded ? '收起命令' : '展开命令' }}
                  </button>
                  <div class="cmd-meta">
                    <el-tag size="small" :type="riskType(m.risk)" effect="plain">{{ riskText(m.risk) }}</el-tag>
                    <span class="cmd-status" :class="m.status">{{ cmdStatusText(m.status) }}</span>
                  </div>
                  <div v-if="m.note" class="cmd-note" :class="{ collapsible: isLongText(m.note, 3, 160) && !m.expanded }">{{ m.note }}</div>
                  <div v-if="m.status === 'failed'" class="cmd-retry">
                    <template v-if="retryId === m.id">
                      <el-input v-model="retryCmd" size="small" class="retry-input"/>
                      <el-button size="small" type="primary" @click="doRetry(m)">执行</el-button>
                      <el-button size="small" @click="retryId=0">取消</el-button>
                    </template>
                    <el-button v-else size="small" type="warning" plain @click="startRetry(m)">编辑并重试</el-button>
                  </div>
                  <div v-if="m.stepId && m.status === 'pending' && m.risk === 'high'" class="cmd-approve">
                    <el-button size="small" type="primary" @click="approveMsg(m)">批准执行</el-button>
                    <el-button size="small" type="danger" plain @click="rejectMsg(m)">拒绝</el-button>
                  </div>
                </div>
              </template>

              <template v-else-if="m.role === 'result'">
                <div class="result-card">
                  <div class="result-head">
                    <span>执行结果</span>
                    <span class="result-exit" :class="m.exitCode === 0 ? 'ok' : 'bad'">exit {{ m.exitCode }}</span>
                  </div>
                  <pre class="result-out" :class="{ collapsible: isLongText(m.text, 10, 600) && !m.expanded }">{{ m.text || '（无输出）' }}</pre>
                  <button v-if="isLongText(m.text, 10, 600)" class="expand-toggle" @click="toggleExpand(m)">
                    {{ m.expanded ? '收起输出' : '展开输出' }}
                  </button>
                </div>
              </template>

              <template v-else>
                <div class="system-bubble">{{ m.text }}</div>
              </template>
            </div>

            <div v-if="aiThinking" class="ai-msg role-assistant">
              <div class="msg-role">AI 分析</div>
              <div class="bubble assistant-bubble thinking">
                <el-icon class="is-loading"><Loading /></el-icon>
                <span>{{ aiThinkingText }}</span>
              </div>
            </div>
          </div>

          <div class="ai-input">
            <el-input
              v-model="aiIntent"
              type="textarea"
              :rows="2"
              resize="none"
              :disabled="aiBusy"
              placeholder="描述问题或继续追问，Enter 发送，Shift+Enter 换行"
              @keydown.enter.exact.prevent="sendAi"
            />
            <div class="ai-input-actions">
              <span class="ai-tip">低危自动执行 · 中高危需确认</span>
              <div class="ai-input-btns">
                <el-button v-if="aiBusy" size="small" type="danger" plain @click="cancelAgent">停止</el-button>
                <el-button size="small" type="primary" :loading="aiBusy" :disabled="aiBusy" @click="sendAi">发送</el-button>
              </div>
            </div>
          </div>
        </aside>
      </div>

      <div
        v-if="activeHost"
        class="resize-handle h"
        title="拖动调整审批列表高度"
        @pointerdown="startResize('approval', $event)"
      ></div>

      <div class="approval-bar" v-if="activeHost" :style="{ height: approvalHeight + 'px' }">
        <div class="approval-head">
          <div class="approval-title">
            <el-icon><DocumentChecked /></el-icon>
            <span>审批列表</span>
            <el-badge v-if="pendingApprovals.length" :value="pendingApprovals.length" type="warning" />
          </div>
          <div class="approval-head-actions">
            <span class="approval-tip">低危命令自动放行执行；中高危命令按编号提交审批引擎</span>
            <el-button
              v-if="pendingApprovals.length"
              size="small"
              type="primary"
              plain
              @click="approveAll"
            >全部批准</el-button>
          </div>
        </div>
        <div class="approval-list">
          <div
            v-for="item in pendingApprovals"
            :key="item.id"
            class="approval-item"
            :class="item.risk_level"
          >
            <span class="approval-no">编号 {{ item.id }}</span>
            <div class="approval-main">
              <div class="approval-cmd"><span class="cmd-sign">$</span>{{ item.command }}</div>
              <div class="approval-purpose">作用：{{ item.purpose || '—' }}</div>
            </div>
            <el-tag size="small" :type="riskType(item.risk_level)" effect="plain">
              {{ riskText(item.risk_level) }}
            </el-tag>
            <div class="approval-actions">
              <el-button size="small" type="primary" @click="approveApproval(item)">批准执行</el-button>
              <el-button size="small" type="danger" plain @click="rejectApproval(item)">拒绝</el-button>
            </div>
          </div>
          <div v-if="approvalHistory.length" class="approval-subhead">
            <span>历史审批</span>
            <span class="approval-subcount">{{ approvalHistory.length }}</span>
          </div>
          <div
            v-for="item in approvalHistory"
            :key="`h-${item.id}`"
            class="approval-item history"
            :class="item.risk"
          >
            <span class="approval-no">编号 {{ item.id }}</span>
            <div class="approval-main">
              <div class="approval-cmd"><span class="cmd-sign">$</span>{{ item.command }}</div>
              <div class="approval-purpose">作用：{{ item.purpose || '—' }}</div>
            </div>
            <el-tag size="small" :type="riskType(item.risk)" effect="plain">
              {{ riskText(item.risk) }}
            </el-tag>
            <el-tag v-if="item.auto" size="small" type="info" effect="plain">自动放行</el-tag>
            <el-tag size="small" :type="historyStatusType(item.status)" effect="plain">
              {{ historyStatusText(item.status) }}
            </el-tag>
            <span v-if="item.exitCode !== undefined && item.exitCode !== 0" class="approval-exit">
              exit {{ item.exitCode }}
            </span>
          </div>
          <div v-if="!pendingApprovals.length && !approvalHistory.length" class="approval-empty">
            暂无审批记录。低危命令会自动放行执行；中高危命令会在此等待审批。
          </div>
        </div>
      </div>

      <el-dialog v-model="shareDialogVisible" width="460px" class="share-dialog" :show-close="false" align-center>
        <template #header>
          <div class="share-title">
            <el-icon><Share /></el-icon>
            <span>会话分享</span>
          </div>
        </template>
        <div class="share-body">
          <div class="share-row">
            <label>有效期（小时）</label>
            <el-input-number v-model="shareForm.expireH" :min="1" :max="720" :step="1" />
          </div>
          <div class="share-row">
            <label><el-icon><Lock /></el-icon> 访问密码（可选）</label>
            <el-input v-model="shareForm.password" type="password" show-password placeholder="留空为免密分享" />
          </div>
        </div>
        <template #footer>
          <div class="share-actions">
            <el-button @click="shareDialogVisible = false">取消</el-button>
            <el-button type="primary" :loading="shareLoading" @click="submitShare">确定并复制链接</el-button>
          </div>
        </template>
      </el-dialog>
    </section>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onActivated, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { Share, Lock } from '@element-plus/icons-vue'
import TerminalPane from '../components/terminal/TerminalPane.vue'
import {
  getHosts, createAiSession, listAiSessions, getAiSession,
  planAiSteps, nextAiSteps, approveAiStep, recordAiStep, rejectAiStep, createAiSessionShare, appendAiSessionMessage,
} from '../api'
import type { Host, OpsStep, OpsSession } from '../types'

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
const sidebarCollapsed = ref(localStorage.getItem('terminal-sidebar-collapsed') === '1')
const terminalMainRef = ref<HTMLElement | null>(null)
const isFullscreen = ref(false)

function clamp(v: number, min: number, max: number) {
  return Math.min(max, Math.max(min, v))
}
function savedSize(key: string, def: number, min: number, max: number) {
  const n = parseInt(localStorage.getItem(key) || '', 10)
  return Number.isFinite(n) ? clamp(n, min, max) : def
}
const sidebarWidth = ref(savedSize('terminal-sidebar-width', 250, 160, 480))
const aiWidth = ref(savedSize('terminal-ai-width', 380, 280, 900))
const approvalHeight = ref(savedSize('terminal-approval-height', 200, 90, 520))
const paneRefs = new Map<string, any>()
let sessionSeq = 0

const aiOpen = ref(localStorage.getItem('terminal-ai-open') === '1')
const aiOnLeft = ref(localStorage.getItem('terminal-ai-left') === '1')
const aiBusy = ref(false)
const aiThinking = ref(false)
let aiCancelFlag = false
const sessionList = ref<OpsSession[]>([])
const selectedSessionId = ref<number | null>(null)
const retryId = ref(0)
const retryCmd = ref('')
const defaultTitle = document.title
const aiThinkingText = ref('正在分析…')
const aiIntent = ref('')
const aiSessionId = ref<number | null>(null)
const aiMessages = ref<ChatMsg[]>([])
const aiChatRef = ref<HTMLElement | null>(null)
let aiHostId: number | null = null
let chatSeq = 0

interface ApprovalDecision {
  approve: boolean
  reason?: string
}
interface ApprovalWaiter {
  promise: Promise<ApprovalDecision>
  resolve: (d: ApprovalDecision) => void
}
const pendingApprovals = ref<OpsStep[]>([])
const approvalWaiters = new Map<number, ApprovalWaiter>()

interface ApprovalRecord {
  id: number
  seq: number
  command: string
  purpose: string
  risk: string
  status: string
  auto: boolean
  output?: string
  exitCode?: number
}
const approvalHistory = ref<ApprovalRecord[]>([])
const stepMsgIds = new Map<number, number>()
const stepCache = new Map<number, OpsStep>()
const restoringMessages = ref(false)
const shareDialogVisible = ref(false)
const shareLoading = ref(false)
const shareForm = ref({ password: '', expireH: 24 })

interface ChatMsg {
  id: number
  role: 'user' | 'assistant' | 'command' | 'result' | 'system'
  text: string
  command?: string
  risk?: string
  status?: string
  exitCode?: number
  note?: string
  stepId?: number
  expanded?: boolean
}

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
      return
    }
  }
  sessionSeq += 1
  const key = `${host.id}-${Date.now()}-${sessionSeq}`
  tabs.value.push({ key, host, name: nextTabName(host), status: 'connecting' })
  activeKey.value = key
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

function fitActive() {
  activePane()?.fit()
}

function reconnectActive() {
  activePane()?.connect()
}

function openNewWindow() {
  if (activeHost.value) openHost(activeHost.value, true)
}

function toggleAi() {
  aiOpen.value = !aiOpen.value
  if (aiOpen.value && activeHost.value?.id) loadAiForHost(activeHost.value.id)
}

function toggleAiPosition() {
  aiOnLeft.value = !aiOnLeft.value
  localStorage.setItem('terminal-ai-left', aiOnLeft.value ? '1' : '0')
  nextTick(() => fitActive())
}

async function newAiSession() {
  const host = activeHost.value
  if (!host?.id) return
  clearApprovals()
  aiSessionId.value = null
  aiMessages.value = []
  aiIntent.value = ''
  try {
    const { data } = await createAiSession(host.id, `${host.name} 智能运维`)
    aiSessionId.value = data.id
    aiHostId = host.id
    ElMessage.success('已新建会话')
  } catch (e: any) {
    ElMessage.error(e.message || '新建会话失败')
  }
}

async function shareAiSession() {
  shareDialogVisible.value = true
}

async function refreshSessionList(hostId: number) {
  try {
    const { data } = await listAiSessions()
    sessionList.value = (data.items || []).filter(x => x.host_id === hostId)
  } catch { /* ignore */ }
}

async function switchSession(id: number) {
  if (!id) return
  aiCancelFlag = true
  clearApprovals()
  aiSessionId.value = id
  aiHostId = activeHost.value?.id || null
  aiMessages.value = []
  try {
    const { data } = await getAiSession(id)
    if (data.messages?.length) {
      restoringMessages.value = true
      try {
        for (const m of data.messages) {
          const msg = pushMsg(m.role as any, m.text || '', {
            command: m.command, risk: m.risk, status: m.status,
            exitCode: m.exit_code, note: m.note, stepId: m.step_id,
          })
          if (m.step_id) stepMsgIds.set(m.step_id, msg.id)
          if (m.status === 'pending' && m.step_id) {
            const st = { id: m.step_id, command: m.command || '', risk_level: m.risk || 'medium', purpose: m.note || '', seq: 0, status: 'pending', session_id: 0 } as OpsStep
            if (!pendingApprovals.value.some(x => x.id === st.id)) pendingApprovals.value.push(st)
            stepCache.set(st.id, st)
          }
        }
      } finally { restoringMessages.value = false }
      return
    }
    for (const st of data.steps || []) {
      if (st.status === 'pending' || st.status === 'approved') {
        pendingApprovals.value.push(st)
        ensureStepMsg(st, 'pending')
        continue
      }
      recordHistory(st, st.status, st.auto_approved, st.output || st.error, st.exit_code)
      ensureStepMsg(st, st.status === 'failed' ? 'failed' : st.status === 'rejected' ? 'rejected' : 'success')
      if (st.output || st.error) pushMsg('result', st.output || st.error || '', { exitCode: st.exit_code ?? 0 })
    }
  } catch { /* ignore */ }
}

async function exportSession() {
  if (!aiSessionId.value) { ElMessage.warning('当前没有会话'); return }
  try {
    const { data } = await getAiSession(aiSessionId.value)
    const lines: string[] = []
    lines.push('# ' + (data.session.title || 'AI 运维会话'))
    lines.push('')
    lines.push('- 会话ID: ' + data.session.id)
    lines.push('- 主机: ' + data.host_name)
    lines.push('- 时间: ' + data.session.created_at)
    lines.push('')
    if (data.messages?.length) {
      for (const m of data.messages) {
        if (m.role === 'user') lines.push('### 用户\n\n' + m.text + '\n')
        else if (m.role === 'assistant') lines.push('### AI 分析\n\n' + m.text + '\n')
        else if (m.role === 'command') lines.push('### 命令\n- ' + (m.command||m.text) + '\n- 风险: ' + m.risk + ' 状态: ' + m.status + '\n')
        else if (m.role === 'result') lines.push('### 执行结果\n- exit: ' + m.exit_code + '\n\n```\n' + m.text + '\n```\n')
        else lines.push('### ' + m.role + '\n\n' + m.text + '\n')
      }
    } else {
      for (const st of data.steps || []) {
        lines.push('### 步骤 ' + st.seq)
        lines.push('- 命令: `' + st.command + '`')
        lines.push('- 风险: ' + st.risk_level + '  状态: ' + st.status)
        if (st.output) lines.push('\n```\n' + st.output + '\n```')
        lines.push('')
      }
    }
    const blob = new Blob([lines.join('\n')], { type: 'text/markdown' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url; a.download = 'session-' + data.session.id + '-' + data.host_name + '.md'; a.click()
    URL.revokeObjectURL(url)
    ElMessage.success('已下载会话报告')
  } catch (e: any) { ElMessage.error(e.message || '导出失败') }
}

async function submitShare() {
  const host = activeHost.value
  if (!host?.id) {
    ElMessage.warning('请先选择主机')
    return
  }
  let sid = aiSessionId.value
  if (!sid) {
    sid = await ensureAiSession()
  }
  if (!sid) {
    ElMessage.error('会话不存在')
    return
  }
  shareLoading.value = true
  try {
    const pwd = (shareForm.value.password || '').trim()
    const expireH = Math.max(1, Math.min(24 * 30, Number(shareForm.value.expireH) || 24))
    const { data } = await createAiSessionShare(sid, pwd, expireH)
    const finalShareURL = data.password_mode
      ? `${data.share_url}?password=${encodeURIComponent(pwd)}`
      : data.share_url
    const copyPayload = data.password_mode ? `${finalShareURL}\n密码: ${pwd}` : finalShareURL
    if (await copyText(copyPayload)) {
      ElMessage.success('分享链接已复制')
      shareDialogVisible.value = false
    } else {
      window.prompt('复制分享链接', copyPayload)
      ElMessage.warning('浏览器限制自动复制，已弹出手动复制框')
    }
  } catch (e: any) {
    ElMessage.error(e.message || '创建分享失败')
  } finally {
    shareLoading.value = false
  }
}

function nextMsgId() {
  chatSeq += 1
  return chatSeq
}

function isLongText(text?: string, lines = 6, charLimit = 420) {
  if (!text) return false
  if (text.length > charLimit) return true
  return text.split('\n').length > lines
}

function toggleExpand(m: ChatMsg) {
  m.expanded = !m.expanded
}

function useSuggestion(text: string) {
  aiIntent.value = text
}

function pushMsg(role: ChatMsg['role'], text: string, extra: Partial<ChatMsg> = {}) {
  const msg: ChatMsg = { id: nextMsgId(), role, text, ...extra }
  aiMessages.value.push(msg)
  if (aiSessionId.value && !restoringMessages.value) {
    void appendAiSessionMessage(aiSessionId.value, {
      role: msg.role,
      text: msg.text,
      command: msg.command,
      risk: msg.risk,
      status: msg.status,
      exit_code: msg.exitCode ?? 0,
      note: msg.note,
      step_id: msg.stepId,
    }).catch(() => undefined)
  }
  scrollChat()
  return msg
}

function updateMsg(id: number, patch: Partial<ChatMsg>) {
  const msg = aiMessages.value.find(m => m.id === id)
  if (msg) Object.assign(msg, patch)
}

function ensureStepMsg(st: OpsStep, status: string) {
  stepCache.set(st.id, st)
  const exists = stepMsgIds.get(st.id)
  if (exists) {
    updateMsg(exists, {
      command: st.command,
      risk: st.risk_level,
      note: st.purpose,
      status,
      stepId: st.id,
    })
    return exists
  }
  const msg = pushMsg('command', '', {
    command: st.command,
    risk: st.risk_level,
    status,
    note: st.purpose,
    stepId: st.id,
  })
  stepMsgIds.set(st.id, msg.id)
  return msg.id
}

function setStepMsgStatus(stepId: number, status: string) {
  const id = stepMsgIds.get(stepId)
  if (id) updateMsg(id, { status })
}

function findStep(stepId: number) {
  return pendingApprovals.value.find(x => x.id === stepId) || stepCache.get(stepId) || null
}

function approveMsg(m: ChatMsg) {
  if (!m.stepId) return
  const st = findStep(m.stepId)
  if (st) approveApproval(st)
}

function rejectMsg(m: ChatMsg) {
  if (!m.stepId) return
  const st = findStep(m.stepId)
  if (st) rejectApproval(st)
}

function scrollChat() {
  nextTick(() => {
    const el = aiChatRef.value
    if (el) el.scrollTop = el.scrollHeight
  })
}

async function loadAiForHost(hostId: number) {
  if (aiHostId === hostId) return
  clearApprovals()
  aiHostId = hostId
  aiSessionId.value = null
  aiMessages.value = []
  try {
    const { data } = await listAiSessions()
    const s = (data.items || []).find(x => x.host_id === hostId)
    if (!s) return
    aiSessionId.value = s.id
    selectedSessionId.value = s.id
    await refreshSessionList(hostId)
    const { data: d } = await getAiSession(s.id)
    if (d.messages?.length) {
      restoringMessages.value = true
      try {
        for (const m of d.messages) {
          const msg = pushMsg(m.role as any, m.text || '', {
            command: m.command,
            risk: m.risk,
            status: m.status,
            exitCode: m.exit_code,
            note: m.note,
            stepId: m.step_id,
          })
          if (m.step_id) stepMsgIds.set(m.step_id, msg.id)
        }
      } finally {
        restoringMessages.value = false
      }
      return
    }
    for (const st of d.steps || []) {
      if (st.status === 'pending' || st.status === 'approved') {
        pendingApprovals.value.push(st)
        ensureStepMsg(st, 'pending')
        continue
      }
      recordHistory(st, st.status, st.auto_approved, st.output || st.error, st.exit_code)
      ensureStepMsg(st, st.status === 'failed' ? 'failed' : st.status === 'rejected' ? 'rejected' : 'success')
      if (st.output || st.error) {
        pushMsg('result', st.output || st.error || '', { exitCode: st.exit_code ?? 0 })
      }
    }
  } catch {
    /* ignore */
  }
}

async function ensureAiSession(): Promise<number | null> {
  if (aiSessionId.value) return aiSessionId.value
  const host = activeHost.value
  if (!host?.id) return null
  const { data } = await createAiSession(host.id, `${host.name} 智能运维`)
  aiSessionId.value = data.id
  aiHostId = host.id
  return data.id
}

async function sendAi() {
  const msg = aiIntent.value.trim()
  if (!msg || aiBusy.value) return
  if (!activeHost.value) {
    ElMessage.warning('请先选择主机')
    return
  }
  const pane = activePane()
  if (!pane) {
    ElMessage.warning('请先打开主机终端')
    return
  }
  aiIntent.value = ''
  pushMsg('user', msg)
  aiCancelFlag = false
  aiBusy.value = true
  try {
    const id = await ensureAiSession()
    if (!id) return
    const usedStream = false // SSE PiAgent 模式暂时禁用，使用稳定的plan/next路径
    if (!usedStream) await runAgent(id, msg, pane)
  } catch (e: any) {
    pushMsg('system', e.message || 'AI 执行失败')
  } finally {
    aiBusy.value = false
    aiThinking.value = false
    aiThinkingText.value = '正在分析…'
    scrollChat()
  }
}

async function streamAgent(sessionId: number, message: string, _pane: any): Promise<boolean> {
  const token = localStorage.getItem('token')
  if (!token) return false
  try {
    const resp = await fetch(`/api/promai/ai-terminal/sessions/${sessionId}/stream`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
      body: JSON.stringify({ message }),
    })
    if (!resp.ok || !resp.body) return false
    const reader = resp.body.getReader()
    const decoder = new TextDecoder()
    let buffer = ''
    let streaming = true
    while (streaming && !aiCancelFlag) {
      const { done, value } = await reader.read()
      if (done) break
      buffer += decoder.decode(value, { stream: true })
      const parts = buffer.split('\n\n')
      buffer = parts.pop() || ''
      for (const part of parts) {
        const line = part.replace(/^data: /, '').trim()
        if (!line) continue
        try {
          const evt = JSON.parse(line)
          switch (evt.type) {
            case 'text':
              pushMsg('assistant', evt.content)
              break
            case 'command':
              pushMsg('command', '', { command: evt.command || 'run_on_host', risk: 'low', status: evt.status || 'running' })
              break
            case 'result':
              pushMsg('result', evt.content, { exitCode: evt.status === 'success' ? 0 : 1 })
              break
            case 'done':
              streaming = false
              break
            case 'error':
              pushMsg('system', evt.content || 'Stream error')
              streaming = false
              break
          }
        } catch { /* skip malformed */ }
      }
    }
    reader.cancel()
    return true
  } catch {
    return false
  }
}

async function runAgent(sessionId: number, firstMessage: string, pane: any) {
  let message = firstMessage
  let useNext = false
  let rounds = 0
  let touchedFlow = false
  const MAX_ROUNDS = 50
  const historyStart = approvalHistory.value.length
  const pendingStart = pendingApprovals.value.length

  while (rounds < MAX_ROUNDS) {
    if (aiCancelFlag) { pushMsg('system', '已停止'); break }
    rounds += 1
    aiThinking.value = true
    aiThinkingText.value = rounds === 1 ? '正在分析问题…' : '正在读取结果并继续分析…'
    let res
    try {
      res = useNext ? await nextAiSteps(sessionId, message) : await planAiSteps(sessionId, message)
    } finally {
      aiThinking.value = false
    }
    const { analysis, done, steps } = res.data

    if (analysis && analysis.trim()) pushMsg('assistant', analysis.trim())
    if (!steps || steps.length === 0) break
    touchedFlow = true

    const manualSteps = steps.filter(s => s.risk_level !== 'low')
    if (manualSteps.length) queueApprovals(manualSteps)
    let executed = false
    for (const st of steps) {
      ensureStepMsg(st, st.risk_level === 'low' ? 'running' : 'pending')
      const auto = st.risk_level === 'low'
      let reason = ''
      if (!auto) {
        const decision = await waitApproval(st)
        if (!decision.approve) {
          try {
            await rejectAiStep(st.id, decision.reason || '用户拒绝')
          } catch { /* ignore */ }
          recordHistory(st, 'rejected', false)
          setStepMsgStatus(st.id, 'rejected')
          continue
        }
        setStepMsgStatus(st.id, 'running')
        reason = decision.reason || ''
      }

      try {
        await approveAiStep(st.id, reason)
      } catch (e: any) {
        setStepMsgStatus(st.id, 'failed')
        pushMsg('system', e.message || '审批失败')
        continue
      }
      await executeOnTerminal(st, pane, auto)
      executed = true
    }

    if (done) break
    if (!executed) break
    useNext = true
    message = ''
  }
  if (touchedFlow || approvalHistory.value.length !== historyStart || pendingApprovals.value.length !== pendingStart) {
    requestFinalConclusion()
  }
}

function cancelAgent() { aiCancelFlag = true; aiBusy.value = false; aiThinking.value = false; pushMsg("system","用户停止了自动化运维") }

function requestFinalConclusion() {
  const hasPending = pendingApprovals.value.length > 0
  if (hasPending) {
    pushMsg('assistant', '当前还有待审批命令，流程已暂停。请完成审批后我会继续执行并输出最终结论建议。')
    return
  }
  const success = approvalHistory.value.filter(x => x.status === 'success').length
  const failed = approvalHistory.value.filter(x => x.status === 'failed').length
  const rejected = approvalHistory.value.filter(x => x.status === 'rejected').length
  pushMsg('assistant', `本轮自动运维已完成。执行成功 ${success} 条，失败 ${failed} 条，拒绝 ${rejected} 条。建议先关注失败/拒绝步骤，再按业务风险决定是否继续追加命令。`)
}

async function executeOnTerminal(st: OpsStep, pane: any, auto = false) {
  const msgId = ensureStepMsg(st, 'running')
  let output = ''
  let exitCode = 0
  try {
    const r = await pane.execCommand(st.command, 120000)
    output = r.output
    exitCode = r.exitCode
  } catch (e: any) {
    output = e.message || '命令执行失败'
    exitCode = -1
  }
  const ok = exitCode === 0
  updateMsg(msgId, { status: ok ? 'success' : 'failed' })
  pushMsg('result', output, { exitCode })
  recordHistory(st, ok ? 'success' : 'failed', auto, output, exitCode)
  try {
    await recordAiStep(st.id, output, exitCode, ok ? '' : `exit ${exitCode}`)
  } catch { /* ignore */ }
}

function recordHistory(st: OpsStep, status: string, auto: boolean, output?: string, exitCode?: number) {
  const record: ApprovalRecord = {
    id: st.id,
    seq: st.seq,
    command: st.command,
    purpose: st.purpose,
    risk: st.risk_level,
    status,
    auto,
    output,
    exitCode,
  }
  const idx = approvalHistory.value.findIndex(x => x.id === st.id)
  if (idx >= 0) approvalHistory.value[idx] = record
  else approvalHistory.value.unshift(record)
}

function ensureWaiter(st: OpsStep): ApprovalWaiter {
  let w = approvalWaiters.get(st.id)
  if (!w) {
    let resolve!: (d: ApprovalDecision) => void
    const promise = new Promise<ApprovalDecision>(res => {
      resolve = res
    })
    w = { promise, resolve }
    approvalWaiters.set(st.id, w)
  }
  return w
}

function queueApprovals(steps: OpsStep[]) {
  for (const st of steps) {
    stepCache.set(st.id, st)
    if (!pendingApprovals.value.some(x => x.id === st.id)) pendingApprovals.value.push(st)
    ensureStepMsg(st, 'pending')
    ensureWaiter(st)
  }
}

function waitApproval(st: OpsStep): Promise<ApprovalDecision> {
  return ensureWaiter(st).promise
}

function settleApproval(id: number, d: ApprovalDecision) {
  const w = approvalWaiters.get(id)
  if (w) {
    approvalWaiters.delete(id)
    w.resolve(d)
  }
  pendingApprovals.value = pendingApprovals.value.filter(x => x.id !== id)
  const st = stepCache.get(id)
  if (st && d.approve) setStepMsgStatus(id, 'running')
}

function clearApprovals() {
  for (const id of [...approvalWaiters.keys()]) {
    settleApproval(id, { approve: false, reason: '会话已切换' })
  }
  pendingApprovals.value = []
  approvalHistory.value = []
  stepMsgIds.clear()
  stepCache.clear()
}

function approveApproval(st: OpsStep) {
  if (approvalWaiters.has(st.id)) {
    settleApproval(st.id, { approve: true })
    return
  }
  void runStandalone(st, true)
}

function rejectApproval(st: OpsStep) {
  if (approvalWaiters.has(st.id)) {
    settleApproval(st.id, { approve: false, reason: '用户拒绝' })
    return
  }
  void runStandalone(st, false)
}

function approveAll() {
  for (const st of [...pendingApprovals.value]) {
    if (approvalWaiters.has(st.id)) settleApproval(st.id, { approve: true })
    else void runStandalone(st, true)
  }
}

async function runStandalone(st: OpsStep, approve: boolean) {
  const pane = activePane()
  if (!approve) {
    try {
      await rejectAiStep(st.id, '用户拒绝')
      pendingApprovals.value = pendingApprovals.value.filter(x => x.id !== st.id)
      recordHistory(st, 'rejected', false)
      ensureStepMsg(st, 'rejected')
    } catch (e: any) {
      ElMessage.error(e.message || '操作失败')
    }
    return
  }
  if (!pane) {
    ElMessage.warning('终端尚未连接，无法执行')
    return
  }
  try {
    await approveAiStep(st.id, '')
    pendingApprovals.value = pendingApprovals.value.filter(x => x.id !== st.id)
    await executeOnTerminal(st, pane, false)
  } catch (e: any) {
    ElMessage.error(e.message || '执行失败')
  }
}

function startRetry(m: ChatMsg) { retryId.value = m.id; retryCmd.value = m.command || '' }
async function doRetry(m: ChatMsg) {
  const pane = activePane()
  if (!pane) { ElMessage.warning('终端未连接'); return }
  const cmd = retryCmd.value.trim() || m.command || ''
  retryId.value = 0
  const cmdMsg = pushMsg('command', '', { command: cmd, risk: m.risk, status: 'running', note: '手动重试' })
  let output = '', exitCode = 0
  try { const r = await pane.execCommand(cmd, 120000); output = r.output; exitCode = r.exitCode }
  catch (e: any) { output = e.message || '执行失败'; exitCode = -1 }
  const ok = exitCode === 0
  updateMsg(cmdMsg.id, { status: ok ? 'success' : 'failed' })
  pushMsg('result', output, { exitCode })
}

function riskType(r?: string) {
  return r === 'high' ? 'danger' : r === 'medium' ? 'warning' : 'success'
}
function riskText(r?: string) {
  return r === 'high' ? '高危' : r === 'medium' ? '中危' : '低危'
}
function cmdStatusText(s?: string) {
  return ({ running: '执行中…', success: '已执行', failed: '执行失败', rejected: '已拒绝', pending: '待执行' } as any)[s || ''] || ''
}
function historyStatusType(s: string) {
  return s === 'success' ? 'success' : s === 'failed' ? 'danger' : s === 'rejected' ? 'info' : 'warning'
}
function historyStatusText(s: string) {
  return ({ success: '已执行', failed: '失败', rejected: '已拒绝', running: '执行中' } as any)[s] || s
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

async function copyText(text: string) {
  try {
    window.focus()
    if (document.hasFocus() && navigator.clipboard?.writeText) {
      await navigator.clipboard.writeText(text)
      return true
    }
  } catch {
    // fall through
  }
  try {
    fallbackCopy(text)
    return true
  } catch {
    return false
  }
}

async function copyHost(host: Host) {
  const text = `${host.username}@${host.address}:${host.port}`
  if (await copyText(text)) {
    ElMessage.success(`已复制：${text}`)
  } else {
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

type ResizeKind = 'sidebar' | 'ai' | 'approval'
let resizeState: { kind: ResizeKind; startX: number; startY: number; start: number } | null = null
let resizePointer: { el: HTMLElement; id: number } | null = null

function startResize(kind: ResizeKind, e: PointerEvent) {
  e.preventDefault()
  const el = e.currentTarget as HTMLElement | null
  if (el && typeof el.setPointerCapture === 'function') {
    try {
      el.setPointerCapture(e.pointerId)
      resizePointer = { el, id: e.pointerId }
    } catch {
      resizePointer = null
    }
  }
  const start = kind === 'sidebar' ? sidebarWidth.value : kind === 'ai' ? aiWidth.value : approvalHeight.value
  resizeState = { kind, startX: e.clientX, startY: e.clientY, start }
  document.body.style.userSelect = 'none'
  document.body.style.cursor = kind === 'approval' ? 'row-resize' : 'col-resize'
  window.addEventListener('pointermove', onResizeMove)
  window.addEventListener('pointerup', stopResize)
  window.addEventListener('pointercancel', stopResize)
}

function onResizeMove(e: PointerEvent) {
  if (!resizeState) return
  const { kind, startX, startY, start } = resizeState
  if (kind === 'sidebar') {
    sidebarWidth.value = clamp(start + (e.clientX - startX), 160, 480)
  } else if (kind === 'ai') {
    const max = Math.max(320, window.innerWidth - sidebarWidth.value - 260)
    const delta = e.clientX - startX
    const next = aiOnLeft.value ? (start + delta) : (start - delta)
    aiWidth.value = clamp(next, 280, max)
  } else {
    approvalHeight.value = clamp(start - (e.clientY - startY), 90, Math.min(520, window.innerHeight * 0.6))
  }
}

function stopResize() {
  if (!resizeState) return
  resizeState = null
  window.removeEventListener('pointermove', onResizeMove)
  window.removeEventListener('pointerup', stopResize)
  window.removeEventListener('pointercancel', stopResize)
  if (resizePointer && typeof resizePointer.el.releasePointerCapture === 'function') {
    try { resizePointer.el.releasePointerCapture(resizePointer.id) } catch { /* ignore */ }
  }
  resizePointer = null
  document.body.style.userSelect = ''
  document.body.style.cursor = ''
  localStorage.setItem('terminal-sidebar-width', String(sidebarWidth.value))
  localStorage.setItem('terminal-ai-width', String(aiWidth.value))
  localStorage.setItem('terminal-approval-height', String(approvalHeight.value))
  nextTick(() => fitActive())
}

function openRouteHost() {
  const qid = Number(route.query.host)
  if (!qid) return
  const target = hosts.value.find(h => h.id === qid)
  if (target) openHost(target)
}

async function openRouteSession() {
  if (!aiOpen.value) return
  const sid = Number(route.query.session)
  if (!sid) return
  if (aiSessionId.value === sid && aiMessages.value.length) return
  try {
    const { data } = await getAiSession(sid)
    aiHostId = data.session.host_id
    aiSessionId.value = data.session.id
    clearApprovals()
    aiMessages.value = []
    if (data.messages?.length) {
      restoringMessages.value = true
      try {
        for (const m of data.messages) {
          const msg = pushMsg(m.role as any, m.text || '', {
            command: m.command,
            risk: m.risk,
            status: m.status,
            exitCode: m.exit_code,
            note: m.note,
            stepId: m.step_id,
          })
          if (m.step_id) stepMsgIds.set(m.step_id, msg.id)
        }
      } finally {
        restoringMessages.value = false
      }
    } else {
      for (const st of data.steps || []) {
        if (st.status === 'pending' || st.status === 'approved') {
          pendingApprovals.value.push(st)
          ensureStepMsg(st, 'pending')
          continue
        }
        recordHistory(st, st.status, st.auto_approved, st.output || st.error, st.exit_code)
        ensureStepMsg(st, st.status === 'failed' ? 'failed' : st.status === 'rejected' ? 'rejected' : 'success')
        if (st.output || st.error) {
          pushMsg('result', st.output || st.error || '', { exitCode: st.exit_code ?? 0 })
        }
      }
    }
  } catch {
    /* ignore */
  }
}

watch(sidebarCollapsed, collapsed => {
  localStorage.setItem('terminal-sidebar-collapsed', collapsed ? '1' : '0')
  nextTick(() => fitActive())
})

watch(aiOpen, open => {
  localStorage.setItem('terminal-ai-open', open ? '1' : '0')
  nextTick(() => fitActive())
})

watch(
  () => activeHost.value?.id,
  id => {
    if (aiOpen.value && id) loadAiForHost(id)
  },
)

watch(
  () => route.query.host,
  () => {
    if (route.path === '/terminal') openRouteHost()
  },
)

watch(
  () => route.query.session,
  () => {
    if (route.path === '/terminal') void openRouteSession()
  },
)

watch(pendingApprovals, (list) => {
  if (list.length === 0) { document.title = defaultTitle; return }
  document.title = '[' + list.length + ' 条待审批] ' + defaultTitle
})
document.addEventListener('visibilitychange', () => {
  if (!document.hidden && pendingApprovals.value.length) {
    document.title = '[' + pendingApprovals.value.length + ' 条待审批] ' + defaultTitle
  }
})

onMounted(async () => {
  document.addEventListener('fullscreenchange', handleFullscreenChange)
  await loadHosts()
  openRouteHost()
  await openRouteSession()
})

onBeforeUnmount(() => {
  document.removeEventListener('fullscreenchange', handleFullscreenChange)
  stopResize()
})

onActivated(async () => {
  openRouteHost()
  await openRouteSession()
  await nextTick()
  fitActive()
})
</script>

<style scoped>
.terminal-page {
  display: flex;
  height: calc(100vh - 108px);
  min-height: 520px;
  gap: 8px;
}
.host-sidebar {
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

.resize-handle {
  flex: none;
  border-radius: 6px;
  position: relative;
  transition: background .15s, box-shadow .15s;
  border: 1px solid rgba(96, 165, 250, .22);
}
.resize-handle.v {
  width: 10px;
  cursor: col-resize;
  background: linear-gradient(180deg, rgba(96, 165, 250, .22), rgba(96, 165, 250, .1));
}
.resize-handle.h {
  height: 10px;
  cursor: row-resize;
  background: linear-gradient(90deg, rgba(96, 165, 250, .22), rgba(96, 165, 250, .1));
}
.resize-handle:hover {
  background: rgba(64, 158, 255, .55);
  box-shadow: 0 0 0 1px rgba(96, 165, 250, .45), 0 0 10px rgba(96, 165, 250, .35);
}

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

.terminal-body-wrap {
  flex: 1; min-height: 0; display: flex; gap: 8px;
}
.terminal-body-wrap.reverse { flex-direction: row-reverse; }
.terminal-body {
  flex: 1; min-width: 0; min-height: 0; position: relative;
  border: 1px solid var(--border-color, rgba(255, 255, 255, .08));
  border-radius: 10px; overflow: hidden; background: #0b1020;
}
.terminal-empty {
  height: 100%; display: flex; flex-direction: column; align-items: center;
  justify-content: center; gap: 12px; color: #6b7a90;
}
.terminal-empty p { margin: 0; font-size: 13px; }

.approval-bar {
  background: var(--bg-card, rgba(255, 255, 255, .03));
  border: 1px solid var(--border-color, rgba(255, 255, 255, .08));
  border-radius: 10px; padding: 8px 10px; display: flex; flex-direction: column; gap: 8px;
  min-height: 90px;
}
.approval-head { display: flex; align-items: center; justify-content: space-between; gap: 10px; }
.approval-title { display: flex; align-items: center; gap: 6px; font-size: 13px; font-weight: 600; }
.approval-title .el-badge { line-height: 1; }
.approval-head-actions { display: flex; align-items: center; gap: 10px; }
.approval-tip { font-size: 11px; opacity: .55; }
.approval-list { flex: 1; overflow-y: auto; display: flex; flex-direction: column; gap: 6px; }
.approval-subhead {
  display: flex; align-items: center; gap: 8px; font-size: 11px; opacity: .6;
  margin-top: 2px; padding-top: 6px; border-top: 1px dashed var(--border-color, rgba(255, 255, 255, .08));
}
.approval-subcount { font-family: Menlo, Monaco, Consolas, monospace; }
.approval-item {
  display: flex; align-items: center; gap: 10px;
  background: rgba(255, 255, 255, .03); border: 1px solid var(--border-color, rgba(255, 255, 255, .08));
  border-left: 3px solid #22c55e; border-radius: 8px; padding: 7px 10px;
}
.approval-item.history { opacity: .72; padding: 5px 10px; }
.approval-item.medium, .approval-item.history.medium { border-left-color: #f59e0b; }
.approval-item.high, .approval-item.history.high { border-left-color: #ef4444; }
.approval-no {
  flex: none; font-size: 11px; font-family: Menlo, Monaco, Consolas, monospace;
  color: #22d3ee; background: rgba(34, 211, 238, .1); border-radius: 5px; padding: 2px 6px;
}
.approval-main { flex: 1; min-width: 0; }
.approval-cmd {
  font-family: Menlo, Monaco, Consolas, monospace; font-size: 12px; color: #e2e9f3;
  white-space: nowrap; overflow: hidden; text-overflow: ellipsis;
}
.approval-cmd .cmd-sign { color: #22d3ee; margin-right: 4px; }
.approval-purpose { font-size: 11px; opacity: .6; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.approval-actions { display: flex; gap: 6px; flex: none; }
.approval-exit { font-size: 11px; font-family: Menlo, Monaco, Consolas, monospace; color: #ef4444; }
.approval-empty { font-size: 12px; opacity: .5; padding: 10px 4px; text-align: center; }

.ai-panel {
  flex: none; display: flex; flex-direction: column; gap: 8px;
  background:
    radial-gradient(120% 60% at 0% 0%, rgba(59, 130, 246, .14) 0%, rgba(59, 130, 246, 0) 46%),
    radial-gradient(120% 60% at 100% 0%, rgba(16, 185, 129, .1) 0%, rgba(16, 185, 129, 0) 44%),
    linear-gradient(180deg, rgba(15, 23, 42, .96), rgba(7, 11, 22, .98));
  border: 1px solid rgba(96, 165, 250, .22);
  border-radius: 14px; padding: 10px; overflow: hidden;
  box-shadow: 0 10px 24px rgba(2, 6, 23, .5), inset 0 1px 0 rgba(148, 163, 184, .08);
}
.ai-header { display: flex; align-items: center; justify-content: space-between; font-size: 13px; font-weight: 700; color: #dbeafe; }
.ai-header > span { display: flex; align-items: center; gap: 6px; }
.ai-header-actions { display: flex; align-items: center; gap: 4px; }
.ai-close { cursor: pointer; opacity: .6; }
.ai-close:hover { opacity: 1; color: #ef4444; }

.ai-chat {
  flex: 1; min-height: 0; overflow-y: auto; display: flex; flex-direction: column; gap: 10px; padding: 8px;
  border: 1px solid rgba(148, 163, 184, .12); border-radius: 12px; background: rgba(2, 6, 23, .45);
}
.ai-hint { margin: auto 0; text-align: center; color: #94a3b8; font-size: 12px; padding: 20px 8px; }
.ai-hint p { margin: 8px 0 0; }
.ai-hint-sub { opacity: .7; }
.ai-suggests { margin-top: 14px; display: flex; gap: 8px; justify-content: center; flex-wrap: wrap; }
.ai-suggest {
  border: 1px solid rgba(96, 165, 250, .28);
  background: rgba(30, 41, 59, .78);
  color: #bfdbfe;
  border-radius: 999px;
  padding: 4px 10px;
  font-size: 11px;
  cursor: pointer;
}
.ai-suggest:hover { background: rgba(30, 64, 175, .45); }

.ai-msg { display: flex; flex-direction: column; gap: 4px; }
.ai-msg.role-user { align-items: flex-end; }
.msg-role { font-size: 11px; opacity: .55; }
.bubble { font-size: 12px; line-height: 1.65; padding: 9px 11px; border-radius: 10px; white-space: pre-wrap; word-break: break-word; max-width: 100%; }
.user-bubble {
  background: linear-gradient(135deg, rgba(29, 78, 216, .35), rgba(37, 99, 235, .22));
  border: 1px solid rgba(96, 165, 250, .35);
  color: #dbeafe;
}
.assistant-bubble {
  background: linear-gradient(180deg, rgba(15, 23, 42, .9), rgba(17, 24, 39, .9));
  color: #e2e8f0;
  border: 1px solid rgba(148, 163, 184, .18);
}
.assistant-bubble.thinking { display: flex; align-items: center; gap: 8px; color: #8fb4d8; }
.collapsible {
  display: -webkit-box;
  -webkit-line-clamp: 6;
  -webkit-box-orient: vertical;
  overflow: hidden;
}
.cmd-text.collapsible,
.cmd-note.collapsible { -webkit-line-clamp: 3; }
.result-out.collapsible { max-height: 220px; overflow: hidden; }
.expand-toggle {
  background: transparent;
  border: 0;
  color: #60a5fa;
  padding: 2px 0 0;
  font-size: 11px;
  text-align: left;
  cursor: pointer;
}

.cmd-card { background: linear-gradient(180deg, rgba(15, 23, 42, .95), rgba(15, 23, 42, .8)); border: 1px solid rgba(96, 165, 250, .26); border-radius: 10px; padding: 9px; }
.cmd-card.running { border-color: #f59e0b; }
.cmd-card.failed { border-color: #ef4444; }
.cmd-card.rejected { border-color: #6b7280; opacity: .75; }
.cmd-line { display: flex; gap: 6px; font-family: Menlo, Monaco, Consolas, monospace; font-size: 12px; color: #e2e9f3; }
.cmd-sign { color: #22d3ee; }
.cmd-text { word-break: break-all; }
.cmd-meta { display: flex; align-items: center; gap: 8px; margin-top: 6px; }
.cmd-status { font-size: 11px; }
.cmd-status.running { color: #f59e0b; }
.cmd-status.success { color: #22c55e; }
.cmd-status.failed { color: #ef4444; }
.cmd-status.rejected { color: #9ca3af; }
.cmd-note { margin-top: 4px; font-size: 11px; opacity: .65; }
.cmd-approve { margin-top: 8px; display: flex; gap: 6px; }

.result-card { background: linear-gradient(180deg, rgba(2, 6, 23, .88), rgba(15, 23, 42, .75)); border: 1px solid rgba(148, 163, 184, .18); border-radius: 10px; padding: 8px; }
.result-head { display: flex; align-items: center; justify-content: space-between; font-size: 11px; opacity: .7; margin-bottom: 6px; }
.result-exit.ok { color: #22c55e; opacity: 1; }
.result-exit.bad { color: #ef4444; opacity: 1; }
.result-out { margin: 0; color: #d7e2f0; font-size: 11px; white-space: pre-wrap; word-break: break-all; overflow: auto; }
.system-bubble { font-size: 11px; color: #f59e0b; background: rgba(245, 158, 11, .08); border-radius: 6px; padding: 6px 8px; white-space: pre-wrap; }

.ai-input { display: flex; flex-direction: column; gap: 6px; border-top: 1px solid rgba(148, 163, 184, .2); padding-top: 8px; }
.ai-quick-actions { display: flex; gap: 8px; flex-wrap: wrap; }
.ai-quick {
  border: 1px solid rgba(96, 165, 250, .26);
  background: rgba(30, 41, 59, .7);
  color: #bfdbfe;
  border-radius: 999px;
  font-size: 11px;
  padding: 4px 10px;
  cursor: pointer;
}
.ai-quick:hover { background: rgba(30, 64, 175, .45); }
.ai-input-actions { display: flex; align-items: center; justify-content: space-between; }
.ai-tip { font-size: 11px; color: #93c5fd; opacity: .9; }
.is-loading { animation: spin 1s linear infinite; }

.ai-session-select { width: 110px; }
.ai-session-select :deep(.el-input__wrapper) { background: rgba(30,41,59,.7); border-color: rgba(96,165,250,.22); box-shadow: none; }
.ai-session-select :deep(.el-input__inner) { color: #bfdbfe; font-size: 11px; }
.ai-input-btns { display: flex; gap: 6px; align-items: center; }
.cmd-retry { display: flex; align-items: center; gap: 6px; margin-top: 6px; }
.retry-input { flex: 1; }
.retry-input :deep(.el-input__wrapper) { background: rgba(15,23,42,.9); padding: 2px 6px; }
@keyframes spin { to { transform: rotate(360deg); } }

:deep(.share-dialog .el-dialog) {
  border-radius: 16px;
  border: 1px solid rgba(96, 165, 250, .28);
  background: linear-gradient(180deg, #0f172a 0%, #0b1020 100%);
  box-shadow: 0 16px 46px rgba(2, 6, 23, .65);
}
.share-title {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 16px;
  font-weight: 700;
  color: #dbeafe;
}
.share-body {
  display: flex;
  flex-direction: column;
  gap: 14px;
  padding: 4px 2px;
}
.share-row {
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.share-row label {
  font-size: 12px;
  color: #93c5fd;
  display: flex;
  align-items: center;
  gap: 6px;
}
.share-actions { display: flex; justify-content: flex-end; gap: 8px; }
</style>
