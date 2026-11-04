# AIOpsGate AI 助手架构与运行机制设计文档

> 版本：1.0　日期：2026-10-30　范围：前端 + 后端 AI 相关模块

## 1. 项目概述

AIOpsGate 是一个**运维会话网关 + AI 运维助手**平台。核心能力是让运维人员（或 AI Agent）通过受控的 SSH / K8s 会话对远程生产环境执行命令，同时用 AI 对命令做**安全审查**、对人机操作做**全量审计**。

平台把「AI 助手」拆成两条相对独立的能力线：

1. **AI 审查引擎（AIReviewer）** —— 对所有将要执行的生产命令做只读/写操作判断，是安全底座。
2. **AI 对话助手（OpenClaw)）** —— 一个可自主调用 API 排查/修复问题的 Agent，通过 SSE 流式对话交互。

二者共用同一套**会话（Session）/ PTY** 基础设施、同一个**统一 AI 配置层**、同一张**白皮书知识库（Briefing）**。

---

## 2. 总体架构

```
┌──────────────────────────────────────────────────────────────────────┐
│                          前端 (Vue3 + Vite)                          │
│  AIChat.vue │ AITerminal.vue │ SiteConnect.vue │ Dashboard │ ...    │
│          （AI对话/任务流式界面，Service Key 自助管理）                  │
└──────────────┬───────────────────────────────────────────────────────┘
               │ HTTP(REST)           │ WebSocket(SIO)      │ HTTP + SSE
               ▼                      ▼                     ▼
┌──────────────────────────────────────────────────────────────────────┐
│                        后端 (Flask, :5100)                          │
│                                                                      │
│  routes/                                                            │
│   ├─ /api/chat/*        AI 对话/SSE 代理      (chat.py)            │
│   ├─ /api/session/*     命令执行 diagnose/fix (user.py)            │
│   ├─ /api/site/*        场地连接/会话(供AI调用) (site.py)            │
│   ├─ /api/admin/*       管理、AI 设置、ServiceKey(admin.py)          │
│   └─ /api/admin/sites/* 基础连接配置  (sites.py)                     │
│                                                                      │
│  services/                                                          │
│   ├─ ai_reviewer.py     AI 命令安全审查（白名单+缓存+LLM判定）        │
│   ├─ ai_config.py       统一 AI 配置（DB→env→默认） + ai_call/ai_chat │
│   ├─ pty_manager.py     SSH 会话/进程/PTY 管理（并发+超时+回收）       │
│   ├─ k8s_manager.py     K8s 会话管理（kubeconfig）                   │
│   ├─ ws_terminal.py     人工运维终端 WS 桥接                         │
│   └─ ws_ai_terminal.py  AI 运维终端 WS 桥接（按键级审查）             │
│                                                                      │
│  模型层 models.py：Admin/Template/Session/SessionLog/AIConfig/      │
│                 AIReviewLog/FixCommand/SiteBriefing/Chat*           │
└───────┬──────────────────────────────┬──────────────────────────────┘
        │                              │
        │ REST /v1/chat/completions      │ OpenAI /chat/completions
        ▼                              ▼
┌──────────────────┐      ┌─────────────────────────────────┐
│ OpenClaw Gateway  │      │ LLM（DashScope Qwen / 其他）     │
│ :18789            │◄────►│ qwen3-coder-plus（审查用）        │
│ AIopsGate Agent    │      └─────────────────────────────────┘
│ (SOUL/TOOLS/…)     │
└──────────────────┘
```

---

## 3. 核心领域模型（AI 相关）

| 表 | 作用 | 关键字段 |
|----|------|---------|
| `admins` | 用户 / 角色权限 | `role`(super_admin/admin/operator/auditor)、`service_key`(AI调用凭证)、`ai_enabled`(本用户AI开关) |
| `templates` | 连接模板（SSH/K8s） | `connect_script`、`type`、`ai_review`(是否审查)、`allowed_commands` |
| `sessions` | 一个受管网关会话 | `token`、`type`、`mode`(ai/operator)、`site_name`、`ai_summary` |
| `session_logs` | 全量输入/输出审计（16MB） | `direction`(input/output)、`operator`、`content` |
| `ai_config` | 键值对，AI 全部配置 | `api_key`、`base_url`、`model`、`system_prompt`、`safe_whitelist`、`reviewer_context`、`openclaw_*` |
| `ai_review_logs` | 每条审查决策记录 | `command`、`approved`、`risk_level`、`reason`、`suggestion` |
| `fix_commands` | 待人工审批的修复命令 | 状态机 `pending→approved/executed/rejected` |
| `site_briefings` | 运维白皮书（双层） | `template_id`(NULL=统一配置) |
| `chat_conversations` / `chat_messages` | AI 对话历史 | `openclaw_user`(会话路由)、`status`(streaming/done) |

---

## 4. AI 会话运行机制（重点）

### 4.1 会话（Session）生命周期

`status: created → starting → active → closed/error`

- **创建**：`POST /api/site/connect`（AI 用）或管理员远程连接。AI 模式会话强制 `mode=ai`，人工运维用 `mode=operator`。
- **启动**：`sites.py/site.py` 把 `SiteConfig` 基础脚本克隆 + 追加 `sw <场地名>` 一步，通过 `manager.start_session()` 用 **pty.fork()** 派生一个交互 shell（SSH）。**异步后台启动**（HTTP 立即返回），AI 轮询 `/connect/<id>/status` 直到 `active`。
- **维护**：PTY 进程由 `SessionManager` 的**回收线程**监控——进程死亡、空闲超时、最大时长都会自动关停并生成 AI 总结。
- **关闭**：关闭 PTY + 置 `closed`，后台触发 `_generate_close_summary()`（调 LLM 汇总本次操作生成运维报告，写入 `ai_summary`）。

### 4.2 三条命令执行通道（/api/session/*）

| 接口 | 语义 | 审查 | 执行方式 |
|------|------|------|---------|
| `/execute` | 通用命令 | 会话开启 AI 审查时 | 通过后立即执行 |
| `/diagnose` | 只读排查 | 强制 `readonly=True` 严格只读 | 通过后立即执行 |
| `/fix` | 写/修复命令 | `analyze()` 只做描述不下放行结论 | **进审批队列**，人工批准后执行 |
| `/batch` | 多命令（≤50） | 每条各自过审查 | 通过后立即执行 |

`_require_ai_mode()` 把关：**operator 模式的会话禁止通过 API 执行命令**，全部需走终端人工操作。

### 4.3 AI 命令审查引擎（ai_reviewer.py）——三级快慢通道

审查一次命令的决策路径（`review()`）：

```
        command
          │
   ┌──────▼───────────────┐
   │ ① SAFE_WHITELIST 命中 │──是──► 直接 approve（0 次 LLM 调用）
   └──────┬───────────────┘
          │ 否
   ┌──────▼───────────────┐
   │ ② 结果缓存命中 (TTL300s)│──是──► 返回缓存（0 次 LLM 调用）
   └──────┬───────────────┘
          │ 否
   ┌──────▼────────────────────────────────┐
   │ ③ 调 LLM：system_prompt(rule)+command  │
   │    readonly 时前置「严格只读」约束       │
   │    temperature=0.1  → 解析 JSON        │
   └──────┬────────────────────────────────┘
          │
     approved / risk_level / reason / suggestion
```

**安全设计要点：**
- **白名单优先**，命中即跳过调用，保障两类只读命令延迟极低（`df -h`、`kubectl get` 等）。
- **缓存** 命中同命令 5 分钟内复用，降低成本和时延。
- **令牌化评分拆解**：`approved`（安全性）与 `command_suggestion`（操作规范建议）**完全解耦**——只读命令即使跑错主机也必 approve，只是给建议。
- **降级为放行**：LLM 超时/网络/解析失败时统一 `approved=True, risk_level=unknown`，避免故障时阻断一切运维；但仍做 `error/timeout` 日志。
- `_reviewer_context`（运维审查上下文/操作规范）会作为环境约束追加进 system prompt，实现「不同环境不同规则」。

### 4.4 修复命令人工审批闭环

```
AI: POST /api/session/fix {command}
  → FixCommand(status=pending) + ai_summary(作用) + ai_risk_level
  → 运维人员在管理后台看到「待审批」列表
  → 人工 approve → manager.execute() 立即执行 → 记录 output → status=executed
  → 人工 reject → status=rejected + 备注
```

所有状态流转写入 `session_logs`，全程可审计。

### 4.5 白皮书知识（Briefing）注入

```
GET /api/session/briefing
  → 合并：统一配置(template_id=NULL) + 连接专属补充(template_id=会话) + reviewer_context
  → 返回 markdown（或 format=json）
```

AI Agent 排查前**必读**（在 AGENTS.md/TOOLS.md 中强制约定），借此获得该场地架构、部署、正常参考值、操作规范。

---

## 5. AI 对话助手（OpenClaw 集成）

### 5.1 会话隔离与路由

- 每个用户有唯一 `openclaw_user = aiopsgate-<admin_id>-v<N>`，OpenClaw 据此划分各自独立上下文，互不干扰。
- `v<N>` 是**会话版本号**（存于 ai_config，内存缓存），用于「新对话 / Stop / 切换会话」——每操作一次递增版本，从而**重置该用户的 Agent 上下文**。

### 5.2 流式对话（SSE）架构

`routes/chat.py` 采用「后台线程 + 内存广播通道」模型，保证**浏览器断开不杀死生成线程**、刷新后能续接：

```
POST /api/chat/send
  → 存 user msg + 建 assistant msg(status=streaming)
  → 创建 StreamChannel + 后台线程

后台线程 _background_generate
  → 拼 [AIOpsGate Config] 系统注入(base_url + service_key)
  → POST OpenClaw /v1/chat/completions (stream=true, user=openclaw_user)
  → 逐 chunk 写入 StreamChannel.publish("chunk", delta)
  → 结束时 publish("done", full)

GET /api/chat/stream/<msg_id>  (EventSource, ?token=)
  → ch.subscribe()：立即回放已发生的全部历史（history replay）
  → 实时接收新 chunk
  → 浏览器断开 → 仅 unsubscribe；线程照跑
```

- `StreamChannel`：**一生产者多消费者**广播，含 `_history` 供迟到订阅者回放。
- 每 20 个 chunk 把增量写回 DB（`_update_msg`），完成时置 `done`。
- 前端标注 `streaming` 状态 + 打字机动画；刷新后若最后一条仍 `streaming` 会自动重连。

### 5.3 关键配置注入

每次对话，后端把 `[AIOpsGate Config]`（`base_url` + 当前用户的 `service_key`）作为 **system 消息**注入给 Agent，Agent 依据 `TOOLS.md` 用 `X-Service-Key` 调用 `/api/site|session/*` 自查自修。`service_key` 存于用户 `admins.service_key`，管理员可自助生成/撤销。

### 5.4 停止机制（stop）

`POST /api/chat/stop/<msg_id>`：
1. 通过 `_cancel_stream` 关闭底层 HTTP 响应（SSE 立即断，前端先停）；
2. 主动向 OpenClaw 发 `user=/stop` 的 dot 请求，试图中断 Agent 当前回合；
3. 会话版本 `v+1` 重置上下文，防止后续输入被「上游未死线程」污染。

### 5.5 Agent 行为定义（agent-workspace/）

| 文件 | 作用 |
|------|------|
| `AGENTS.md` | 启动流程、核心工作流（连接→等就绪→读白皮→诊断→修复→汇报）、工作原则 |
| `SOUL.md` | 角色行为（主动排查、透明汇报、排查思路、输出控制/base64 技巧） |
| `TOOLS.md` | 完整 API 手册（curl 示例、认证方式、readonly/fix 区别） |
| `IDENTITY.md` / `USER.md` / `HEARTBEAT.md` | 身份、用户画像、心跳任务（默认空） |

这些文件由 OpenClaw 在 `~/.openclaw/openclaw.json` 注册到 `aiopsgate` agent 的 workspace 路径，作为 Agent 的「人格 + 工具说明」。

---

## 6. PTY 会话管理（会话基础设施）

### 6.1 pty.fork() 模型

每个 Session 对应一个 `pty.fork()` 派生的 `/bin/bash -c 'ssh ...'` 子进程（含多步交互：密码/跳板/`sw 场地`）。父进程持有 fd + pid，负责读写。

### 6.2 命令串行化与脏会话恢复

`manager.execute()`:
1. 若上一条命令把会话搞脏（`_needs_recovery`）→ `_recover()`（Ctrl-C + 回车 + drain）；
2. `_drain_buffer()` 丢弃历史残留输出；
3. 写入 `command\n`，读直到识别到 shell 提示符（`_looks_like_prompt`）或超时；
4. 超时未回提示符 → 标记脏 + 下一次自动恢复；
5. `_clean_output()` 剥离回显与提示符，返回纯净输出。

> 同一 fd 用 `threading.Lock` 串行化，避免 AI REST 与 WS 终端并发写冲突。

### 6.3 AI 终端 WS 桥（按键级审查）

路径：`/api/admin/sessions/<id>/start` → `ws_ai_terminal.py` 的 `/_ai-terminal` 命名空间。

- 普通按键（echo/Tab/方向键）**即时透传**到 PTY。
- 按 Enter 时提取当前命令 → `_is_safe_readonly()` 命中则直接放行（零延迟），否则 `reviewing=True` 后台送 AI 审查，**按住 Enter 不放**并显示「🔍 AI 审查中...」。
- 审查通过 → 补发被按住的回车；被拒 → 发 `Ctrl-U` 清空该行 + 终端红字提示拒绝原因/风险/建议。
- 审查全程走 app_context + 后台线程。

### 6.4 超时与回收

`SessionManager` 独立守护线程每 30s `_reap_loop()`：进程死 / 空闲超时 / 超最长时长 → 清理 PTY + 关会话 + 触发 AI 总结。

---

## 7. 安全与审计设计

1. **双层认证**：
   - 管理后台：JWT（`admin_required`）+ RBAC（`permission_required`，角色→权限集合）。
   - 命令执行：`X-Session-Token`（`session_token_required`）；AI 调用：`X-Service-Key` 用户级密钥。
2. **操作者溯源**：`session_token_required` 按 service_key > X-Operator > JWT > token 前缀 逐级解析 `g.operator`，写入每条日志。
3. **只读强约束**：`diagnose` 走 `readonly=True`，system prompt 前置「严格只读」，写命令必拒。
4. **人工兜底**：`fix` 必须人工审批才能落地，AI 无权直接改造环境。
5. **全量审计**：`session_logs`(16MB/条)/`ai_review_logs`/`fix_commands` 记录每一步，Dashboard + 审查统计页可视化。
6. **密钥隔离**：OpenClaw Gateway Token 只存后端；`connect_script` 中密码在 API 响应中 `******` 脱敏。
7. **AI 汇总审计**：会话关闭自动生成运维总结报告，形成「人工/机器操作」可回溯档案。

---

## 8. AI 配置统一层

`services/ai_config.py`：**唯一事实来源**。

- 加载优先级：**DB(AIConfig) > 环境变量 > 硬编码默认**。
- 提供 `get_ai_config()`（拿到 api_key/base_url/model/timeout）、`ai_call()`（解析 JSON 返回）、`ai_chat()`（返回纯文本）。
- 审查用的 `system_prompt`、`safe_whitelist`、`reviewer_context` 存 DB，管理员可在「AI 审查配置」页在线调整并即时生效（`reload_config()`）。

---

## 9. 前端关键交互

| 视图 | 职责 |
|------|------|
| `AIChat.vue` | 就绪检测（OpenClaw 连通 + ServiceKey 有无）→ 引导配置 → 对话界面；SSE 流式渲染、对话历史、新对话/停止 |
| `AISettings.vue` | 命令审查的 AI 服务配置、System Prompt、审查上下文、安全白名单、审查统计与日志 |
| `AITerminal.vue` | xterm.js + Socket.IO 连接 `/_ai-terminal`，实时显示 AI 审查结果，时长/结束运维 |
| `SiteConnect.vue` | 输入场地名+原因快速建连（入口，供人工选择） |
| `Dashboard.vue` / `Sessions.vue` | 会话与场地状态、AI 审查/待审批概览 |

前端通过 `axios`（自动带 JWT）与 SSE（`EventSource?token=`）调用后端，Socket.IO 走 WebSocket。

---

## 10. 关键设计取舍与扩展点

| 设计点 | 取舍说明 |
|--------|---------|
| 命令审查三级快慢通道 | 白名单/缓存牺牲一定严格性换实时性，纯 LLM 作为兜底判定 |
| 审查失败默认放行 | 保证容灾可用性优先于严格阻断（原因记日志） |
| 后台线程 + StreamChannel | 彻底解耦「生成」与「订阅」，故障不丢流、刷新可续接 |
| 会话版本 v 重置上下文 | 简单可靠地实现「新的上下文」，替代复杂会话清理 |
| pty.fork 单进程/会话 | 天然串行、易回收；代价是横向扩展到多机需额外设计 |
| AI 完成整改需人工审批 | 安全红线，机器只诊断/提议，不直接生产改动 |

**未来扩展方向**：多机 PTY 调度、命令审查基于工具的 Prompt 生态化、二次复核（人审+AI 审）、白皮书自动生成/版本回滚、多 Agent 协同。

---

## 11. Golang 复刻方案（Go 版执行引擎）

> 目标：用 Go 1.21+ 复刻「终端命令执行」核心链路，保持与原系统相同的
> **会话模型、命令审查快慢通道、脏会话恢复、WS 双向桥接、审计日志**等行为，
> 同时利用 Go 语言特性（goroutine / context / 标准库）简化并发与生命周期管理。

### 11.1 技术选型映射（Python → Go）

| 能力 | Python 原实现 | Go 复刻选型 |
|------|--------------|------------|
| Web 框架 | Flask + Flask-SocketIO | Gin/chi（REST）+ nhooyr.io/websocket 或 gorilla/websocket |
| 会话进程 | `pty.fork()` + `os.read/write` | `github.com/creack/pty`（PTY）+ `os/exec` |
| 并发 | `threading.Thread` + `threading.Lock` | goroutine + `sync.Mutex` |
| 多路复用 | `select.select([fd])` | goroutine + blocking `Read` 于独立 goroutine |
| 流式（SSE） | 后台线程 + `StreamChannel` | goroutine + `chan` + `http.ResponseWriter`(Flush) |
| DB | SQLAlchemy + MySQL | GORM + MySQL |
| 定时回收 | 守护线程 | `time.Ticker` goroutine + `context` |
| 认证 | PyJWT | `github.com/golang-jwt/jwt/v5` |

### 11.2 目录结构

```
golang-backend/
├── cmd/gateway/main.go            # 入口（路由、中间件、启动）
├── internal/
│   ├── model/                     # 数据模型（Admin/Session/SessionLog/...）
│   ├── auth/                      # JWT + service_key + session_token
│   ├── config/                    # 统一 AI 配置（DB→env→默认）
│   ├── ai/
│   │   ├── reviewer.go            # AI 审查引擎（白名单+缓存+LLM）
│   │   ├── config.go              # 审查上下文 / system prompt 拼装
│   │   └── call.go                # LLM HTTP 调用（DashScope Qwen）
│   ├── session/
│   │   ├── manager.go             # 会话管理器（核心：pty + 串行锁 + 回收）
│   │   ├── execute.go             # 命令执行（脏恢复/提示符识别/输出清洗）
│   │   └── pty.go                 # pty 包装（fd/pid/窗口大小）
│   ├── transport/
│   │   ├── ws_terminal.go         # 人工终端 WS 桥（按键转发+日志）
│   │   └── ws_ai_terminal.go      # AI 终端 WS 桥（Enter 拦截审查）
│   └── chat/
│       ├── proxy.go               # OpenClaw 流式代理（SSE）
│       └── channel.go             # StreamChannel（chan 广播+历史回放）
└── routes/                        # Gin handler（/api/chat /api/session /api/site ...）
```

### 11.3 会话管理器（核心，对应 pty_manager.py）

```go
type Session struct {
    mu        sync.Mutex      // 同一 PTY 的所有读写串行化（对应 Python thread lock）
    fd        *os.File        // pty 主端 fd
    cmd       *exec.Cmd       // 子进程（bash -c "ssh ..."）
    alive     bool
    needsRec  bool            // 上次命令是否把会话搞脏（对应 _needs_recovery）
    lastAct   time.Time       // 最近活动时间（回收用）
    readyPtn  *regexp.Regexp  // 提示符识别（对应 _PROMPT_RE）
}

func (m *Manager) StartSession(ctx, sessionID, connectScript) (string, error) {
    // 解析 connect_script：command + steps（多步交互）
    // pty.Start(exec.Command("/bin/bash","-c",command))   // 对应 pty.fork()
    // 按 steps 匹配 expect → 发送 send（密码/跳板/`sw 场地`）
    // 读初始输出，注册到 map[SessionID]*Session
}

func (m *Manager) Execute(ctx, sessionID, command string, timeout time.Duration) (string, error) {
    s := m.get(sessionID)
    s.mu.Lock(); defer s.mu.Unlock()        // 串行化

    if s.needsRec { m.recover(s) }          // 脏会话：Ctrl-C+回车+清缓冲
    m.drain(s)                              // 丢弃残留输出

    io.WriteString(s.fd, command+"\n")

    out, ok := m.readUntilPrompt(ctx, s, timeout) // select→读→直到提示符或ctx取消
    if !ok { s.needsRec = true }            // 超时未到提示符→标记脏
    return m.cleanOutput(out, command), nil
}
```

**`readUntilPrompt` 用 context 替代 Python 的轮询 select + 超时比较：**

```go
func (m *Manager) readUntilPrompt(ctx context.Context, s *Session, timeout time.Duration) (string, bool) {
    dctx, cancel := context.WithTimeout(ctx, timeout)
    defer cancel()
    buf := make([]byte, 65536)
    for {
        n, err := s.fd.Read(buf)        // 阻塞读，天然替代 select.select([fd])
        if err != nil { break }          // 对应 OSError/进程退出
        chunk := string(buf[:n])
        acc += chunk
        last := lastLineAfterAnsi(acc)
        if m.looksLikePrompt(last) { return acc, true }   // 提示符识别
        select {
        case <-dctx.Done(): return acc, false             // 超时/取消
        default:
        }
    }
    return acc, false
}
```

### 11.4 命令审查引擎（对应 ai_reviewer.py 三级快慢通道）

```go
type Reviewer struct {
    mu           sync.RWMutex
    apiKey, base string
    model        string
    timeout      time.Duration
    systemPrompt string
    reviewerCtx  string
    cache        map[string]cacheItem   // command md5 → (result, ts)
    whitelist    map[string]struct{}    // 自定义白名单
}

func (r *Reviewer) Review(ctx context.Context, cmd, context string,
                          sessionID int, readonly bool) Result {
    if r.whitelisted(cmd)      { return Result{Approved:true, Risk:"safe", Whitelisted:true} }
    if !readonly {
        if it, ok := r.cached(cmd); ok { return it.result }      // 缓存 TTL 5min
    }
    // LLM 判定（对应 ai_reviewer.review 的 API 调用）
    return r.llmReview(ctx, cmd, context, readonly)
}
```

- `whitelisted`：精确匹配 + SSH 纯登录规则（`ssh <host>`，带远端命令需审查），对应 `_is_whitelisted`。
- `readonly` 时绕过缓存 + 前置「严格只读」system prompt 前缀。
- 失败降级：LLM 超时/网络/解析错误 → `Approved:true, Risk:"unknown"` 并记 error 标记，保证容灾可用。
- 结果写入 `AIReviewLog`，`analyze()`（fix 用，只描述不下结论）单独实现。

### 11.5 人工终端 WS 桥（对应 ws_terminal.py）

```go
// 每会话一个 goroutine 读 PTY → 写 WebSocket；同时做 current_line 追踪 / 日志采集
func (h *WSTerminal) readLoop(ctx, sess, ws) {
    buf := make([]byte, 32768)
    for {
        n, err := sess.fd.Read(buf)                 // 阻塞读
        if err != nil { break }
        ws.WriteJSON(OutputEvent{Data: string(buf[:n])}) // → 前端 xterm
        h.updateCurrentLine(sess, string(buf[:n]))       // ANSI剥离+行追踪
        if enter { cmd := extractCommand(sess.currentLine) } // 提取并记 input 日志
        if looksLikePrompt(sess.currentLine) { sess.recording = false } // 回提示符→停止采集
    }
}
```

- **按键转发**：`ws.ReadJSON` → `io.WriteString(sess.fd, data)`；检测 `\r/\n` 置 enter 标志。
- **窗口大小**：`unix.IoctlSetInt(fd, unix.TIOCSWINSZ, ...)` 对应 `fcntl.ioctl(TIOCSWINSZ)`。
- **current_line 追踪**：`updateCurrentLine` 逐字符处理 `\n`/`\r`/`\x08`/`\x07`/可打印，等价 `_update_current_line`。
- **命令提取**：正则 `[\$#>]\s+(.*)` + `] [$#>]? (.*)`，等价 `_extract_command`。

### 11.6 AI 终端 WS 桥（对应 ws_ai_terminal.py，Enter 拦截审查）

```go
func (h *WsAiTerminal) onInput(ctx, bridge, data) {
    if data contains '\r' or '\n' {
        cmd := extractCommand(bridge.currentLine)
        if cmd == "" || isSafeReadonly(cmd) {
            // 直接放行 Enter（回退安全只读零延迟）
            io.WriteString(bridge.fd, data); recordLog(cmd); return
        }
        bridge.reviewing = true
        ws.WriteJSON(OutputEvent{Data: "[🔍 AI 审查中...]"})
        go func() {                                   // 后台审查（对应 threading.Thread）
            res := reviewer.Review(ctx, cmd, buildContext(), ...)
            bridge.mu.Lock(); defer bridge.mu.Unlock()
            if res.Approved {
                io.WriteString(bridge.fd, bridge.heldEnter)   // 补发被按住的回车
            } else {
                io.WriteString(bridge.fd, "\x15")             // Ctrl-U 清行
                ws.WriteJSON(OutputEvent{Data: "🛡️ AI 审查拒绝: ..."})
            }
            bridge.reviewing = false
        }()
        return
    }
    io.WriteString(bridge.fd, data)   // 普通按键立即转发
}
```

- `isSafeReadonly`：本地安全词/前缀集合，等价 `_is_safe_readonly`。
- 后台审查失败时仍补发回车避免终端卡死（等价原文 `except:` 兜底）。

### 11.7 会话回收（对应 `_reap_loop`）

```go
func (m *Manager) startReaper() {
    go func() {
        tick := time.NewTicker(30 * time.Second)
        defer tick.Stop()
        for {
            select {
            case <-m.ctx.Done(): return
            case <-tick.C:
                m.reap()      // 进程死/空闲超时/超最长时长 → 清理+关会话+触发AI总结
            }
        }
    }()
}
```

### 11.8 SSE 流式对话（对应 StreamChannel）

用 **channel 广播 + 历史切片回放**实现同样的「一生产者多消费者、断开不丢、刷新可续」：

```go
type StreamChannel struct {
    mu          sync.Mutex
    subs        []chan Item
    history     []Item          // 迟到订阅者回放
    done        bool
    final       string
}
func (c *StreamChannel) Subscribe() <-chan Item {
    c.mu.Lock(); defer c.mu.Unlock()
    ch := make(chan Item, 64)
    for _, it := range c.history { ch <- it }   // 历史回放
    if c.done { ch <- Item{Type:"done", Data:c.final}; close(ch); return ch }
    c.subs = append(c.subs, ch)
    return ch
}
func (c *StreamChannel) Publish(t, d string) { ... }  // 写 history + 广播全部订阅者 }
```

HTTP handler 用 `SetWriter(nopFlusher)`/`fmt.Fprintln(w, "data: ..."); w.(http.Flusher).Flush()` 支持 SSE 逐块推送。

### 11.9 认证映射

| 原机制 | Go 复刻 |
|--------|--------|
| `admin_required`（JWT） | Gin 中间件，`jwt.Parse` 校验 |
| `permission_required`（RBAC） | 中间件查 `Admin.Role` 权限集合 |
| `session_token_required` | 中间件查 Session + 超时判定 + `g.operator` 解析（service_key > X-Operator > JWT > token） |
| `service_or_admin_required` | 先查 `X-Service-Key`，回退 JWT |

### 11.10 与原系统一致的边界行为（复刻验收清单）

1. **只读约束**：`diagnose` 强制 `readonly=true` 严格只读；`operator` 模式禁止 API 执行。
2. **修复审批**：`fix` 只做 `analyze`，进 `pending` 队列，人工 approve 才 `execute`。
3. **提示符识别**：`[user@host ~]$` / `...#` / `...>$` 判定命令结束（需做 ANSI 剥离）。
4. **输出清洗**：剥离开启的行与末尾提示符，避免污染 AI 结果。
5. **失败降级放行**：审查服务不可用时 `Approved=true` 且记日志（与 Python 一致）。
6. **审计完整性**：每条 input/output/AI 决策/审批都落 `session_logs` / `ai_review_logs` / `fix_commands`。
7. **WS 桥接不杀会话**：浏览器断开仅 `detach`，PTY 继续后台运行。

### 11.11 Go 复刻的额外优势

- **真并发**：Python 依赖 GIL、同一 PTY 用锁互斥；Go 的 goroutine + channel 更易做**每会话独立读写协程**，且天然支持多 worker 横向扩展（会话存 Redis + 分布式锁）。
- **生命周期**：`context` 贯穿「请求→审查→执行」，超时/取消/后台生成可统一由 `context.Done()` 控制，替代 Python 里手动 flag（`_cancel_events`、`_is_cancelled`）。
- **编译期类型**：命令审查结果、SSE 事件、WS 消息均可用 struct + JSON tag，避免 Python 频繁 `json.loads` 与 dict 键错。
- **部署**：单二进制，`Dockerfile` / systemd / K8s 皆轻量；无需 Python 运行时。

> 复刻顺序建议：① 模型+认证+配置层 → ② session/manager+execute（命令执行保真）→
> ③ ai/reviewer（三级通道）→ ④ transport/ws_*（双向桥接）→ ⑤ chat/proxy+channel（SSE 对话）。
> 每一步用「11.10 验收清单」对照原系统回归。