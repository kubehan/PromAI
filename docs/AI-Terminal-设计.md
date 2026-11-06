# 自动化运维智能终端 — 设计文档

> 版本：v0.2（评审稿） · 状态：待评审
> 项目：PromAI（Go + Vue 单体）

## 1. 背景与目标

在 PromAI 内提供一个**人工在环路项式**的智能运维终端：选择目标主机（可关联故障），用自然语言描述意图，AI 生成可审批的命令步骤，人工审批后执行，AI 读取结果循环推进直至完成，全程审计，并可沉淀为 Runbook。

设计原则：复用现有模块（SSH 执行、LLM/聊天、日志、主机管理），不引入重型框架，先自组装实现 MVP。

## 2. 现状盘点

| 能力 | 现状 | 复用点 |
|---|---|---|
| 主机管理 | `TerminalHost` CRUD + 凭据 AES 加密 | 选择目标主机 |
| SSH 执行 | `execHostCommand`（非交互、超时 + `TerminalExecLog`） | 命令执行底座 |
| 交互终端 | `TerminalPane` + WebSocket PTY | 手动兜底 |
| 执行日志 | `TerminalExecLog` | 审计 |
| AI 聊天 | `pkg/pi-agent`（SSE + 监控工具） | LLM 调用 / 流式 |
| 审批 | `TerminalFixCommand` 模型（已建未实现） | 审批追溯落点（可选联动） |
| Runbook | `Runbook` 模型（已建未实现） | Sprint 3 启用 |
| 入口 | `/ai-terminal` redirect → `/terminal`；`IncidentDetail` 用 `?session=` | 正式入口 |

## 3. 总体架构

```
[AiTerminal.vue]
  选主机/故障 → 输入意图 → 展示 AI 步骤计划
       │                        │
       │ approve/reject       每步命令块（风险标签）
       ▼                        ▼
[后端 ai_terminal.go, AdminAPI.authMiddleware]
   /sessions /plan /steps/:id/execute /steps/:id/reject /sessions/:id/next
       │
       ├─ LLM 规划(pi-agent 模型封装) → 结构化 JSON 步骤
       ├─ 审批门禁 → HostAPI.runCommand → TerminalExecLog
       └─ Runbook(排障沉淀, Sprint 3)
       ▼
   审计：OpsStep(审批) + TerminalExecLog(执行)
```

## 4. 决策与默认值（MVP 定案）

| 问题 | 决策（默认值） | 可配置/备注 |
|---|---|---|
| 审批策略 | **低危(ro) 自动放行；中危人工确认；高危强制审批+理由** | `auto_approve_low` 开关 |
| “大脑”链路 | **复用 pi-agent 现有模型配置** | 不引 opencode |
| 单机 or 批量 | **MVP 单主机** | Sprint 2 扩主机组 |
| Runbook | **本轮不实现**，Sprint 3 | 本轮只预留 save 占位 |
| 手动终端兜底 | **页面内并排保留 `TerminalPane`** | keep-alive |
| 审计展示 | **时间线**（沿用 Hosts/Incident 风格） | — |
| aiops 补鉴权 | **本轮顺带补（除 `/stats/overview`）** | 安全项 |
| 审批人 | MVP **同用户自审批** | Sprint 3 引 RBAC/第二人 |

## 5. 数据模型

新增表注册进 `pkg/database/models.go` 的 `AutoMigrate`。

### 5.1 `OpsSession`
| 字段 | 类型 | 说明 |
|---|---|---|
| ID | uint | 主键 |
| Title | string | 标题（意图简写） |
| HostID | uint | 目标主机 |
| HostKey | string | 冗余主机标识 |
| IncidentID | *uint | 关联故障（可空） |
| Username | string | 创建者 |
| Status | string | running / completed / failed |
| CreatedAt / UpdatedAt | time | 时间戳 |

### 5.2 `OpsStep`
| 字段 | 类型 | 说明 |
|---|---|---|
| ID | uint | 主键 |
| SessionID | uint | 所属会话 |
| Seq | int | 序号 |
| Command | string | 命令 |
| Purpose | string | 用途 |
| RiskLevel | string | low / medium / high |
| Status | string | pending / approved / rejected / running / success / failed |
| Output | string | 输出（截断 20K） |
| Error | string | 错误信息 |
| ExitCode | int | 退出码 |
| ApprovedBy | string | 审批人 |
| RejectReason | string | 拒绝原因 |
| AutoApproved | bool | 是否自动放行 |
| ExecutedAt / ApprovedAt | *time | 时间 |
| CreatedAt / UpdatedAt | time | 时间戳 |

> 索引建议：`OpsStep` 建 `(session_id, seq)` 联合索引；`OpsSession` 建 `username`、`host_id` 索引。

### 5.3 Runbook
沿用现有 `Runbook` 模型，Sprint 3 实现 CRUD / 执行。

## 6. 审批状态机与风险分级

### 6.1 风险分级（LLM 标注 + 启发式兜底）
| 级别 | 含义 | 示例（前缀匹配） |
|---|---|---|
| low | 只读/查询 | df ps tail cat grep uptime lsblk free ss -tnl |
| medium | 写入/服务操作 | mkdir touch echo> systemctl restart mv cp |
| high | 破坏性/危险 | rm -rf mkfs kill -9 dd >/dev/sd FLUSH shutdown reboot |

优先级：**启发式命中 high 时强制按 high 处理（安全硬顶），忽略 LLM 的 low**；其余按 LLM 标注为准，未标注时用启发式兜底。

### 6.2 审批状态机
```
pending → approved → running → success
       ↘ rejected（可带原因）
pending → running（low 自动放行：approved+auto_approved=true）
```
- 未批准不得进入 running。
- 主机离线 / 命令超时 → 置 failed 并记 Error。

## 7. 后端接口（新增 ai_terminal.go）

全部过 `AdminAPI.authMiddleware`。错误响应 `{ "error": "..." }`。

### 7.1 会话
**POST `/api/promai/ai-terminal/sessions`**
```jsonc
// req
{ "host_id": 8, "incident_id": null, "title": "磁盘排查" }
// 201 resp
{ "id":1, "title":"磁盘排查", "host_id":8, "incident_id":null,
  "status":"running", "username":"admin", "created_at":"..." }
```

**GET `/api/promai/ai-terminal/sessions`** → `{ "items":[...OpsSession], "total":n }`

**GET `/api/promai/ai-terminal/sessions/:id`** → `{ "session":{...}, "steps":[OpsStep...] }`

### 7.2 规划与循环
**POST `/api/promai/ai-terminal/sessions/:id/plan`**
```jsonc
// req { "message": "磁盘满了，排查并清理 /tmp" }
// resp
{ "steps":[
    { "seq":1, "command":"df -h", "purpose":"查看磁盘使用", "risk":"low" },
    { "seq":2, "command":"du -sh /tmp", "purpose":"定位 /tmp 占用", "risk":"low" },
    { "seq":3, "command":"rm -rf /tmp/*.tmp", "purpose":"清理临时文件", "risk":"high" }
]}
```
生成步骤写入 DB（status=pending）。

**POST `/api/promai/ai-terminal/sessions/:id/next`**
```jsonc
// req { "message": "继续" }   // 携带上一步输出上下文
// resp { "steps":[...] }
```

### 7.3 步骤处理
**POST `/api/promai/ai-terminal/steps/:id/execute`**
```jsonc
// req { "reason": "确认清理", "approve": true }
// 若 approve=false 或风险未过审批 → 400
// resp
{ "id":1, "status":"success", "exit_code":0, "output":"...",
  "duration_ms":120, "approved_by":"admin" }
```
流程：审批门禁 → runCommand(host, command, timeout) → 更新 OpsStep → 写 TerminalExecLog。

**POST `/api/promai/ai-terminal/steps/:id/reject`**
```jsonc
// req { "reason": "命令有风险" } → resp { "status":"rejected" }
```

### 7.4 Runbook（占位，Sprint 3）
`POST /steps/:id/save-runbook` 本轮返回 501 或建草稿记录。

## 8. LLM 规划协议

复用 `pkg/pi-agent` 的模型封装（非流式，一次性返回 JSON）。

- System prompt：角色、仅输出 JSON、schema、风险分级规则、禁止伪造结果。
- 输入：目标主机上下文 + 用户意图。
- 输出（期望）：
  ```json
  { "steps":[ { "command":"...", "purpose":"...", "risk":"low|medium|high" } ] }
  ```
- 容错：解析失败重试 1 次；仍失败返回 502。启发式 high 命令即使 LLM 标 low 也按 high。

## 9. 前端设计与交互流程

**`AiTerminal.vue`**（路由 `/ai-terminal` 替换 redirect；支持 `?session=` 联动；keep-alive）

- 左：主机选择 / 关联故障 / 会话历史（列表+新建）。
- 中：对话线程（用户意图气泡 → AI 步骤计划 → 每步命令块+风险+用途 → 审批执行/拒绝 → 输出展示）。
- 右下：`TerminalPane` 手动终端兜底。
- 菜单：App.vue 新增「智能终端」，显示策略沿用 embedEnabled。
- API/types：补齐 OpsSession / OpsStep 与接口。

### 交互时序（执行一步）
选主机 → 输入意图 → /plan → 展示 pending 步骤
→ 用户点“审批执行”(low 自动) → /steps/:id/execute
→ 后端审批门禁 → SSH 执行 → 返回 {output,exit_code}
→ 前端置 success/failed + 展示输出
→ 用户点“继续” → /next → AI 建议下一步 → 追加 pending 步骤

## 10. 安全设计

- 所有新接口过 `AdminAPI.authMiddleware`（JWT）；WebSocket 终端沿用 `?token=`。
- 审批门禁：未批准不得执行；high 强制人工；low 按 auto_approve_low 自动。
- 审计：OpsStep(审批) + TerminalExecLog(执行) 双记录。
- MVP 同用户自审批；Sprint 3 引 RBAC/第二审批人。
- 本轮顺带给 aiops 未鉴权路由补认证（除 /stats/overview）。

## 11. 非功能需求与错误处理

- 超时：命令执行默认 60s，最长 1800s（沿用现有）。
- 并发：同一会话串行执行；并发会话数以内配置限制。
- 边界：主机离线→failed；LLM 无响应/解析失败→可重试；已执行步骤不可再执行。
- 数据：输出截断 20K；SQLite WAL 持久化。
- 可观测：每个接口 logRequest 记录；关键日志带 session/step 上下文。

## 12. MVP 实施任务清单（Sprint 1）

### 12.1 后端
- [ ] 模型 `OpsSession/OpsStep` + `AutoMigrate`
- [ ] 风险分级 `RiskLevelFor(cmd)` + 单测
- [ ] `ai_terminal.go`：会话/plan/next/execute/reject 接口
- [ ] LLM 规划：prompt→JSON schema + 解析容错
- [ ] 审批门禁 + 状态机 + 审计
- [ ] 鉴权接入 + `main.go` 注册
- [ ] aiops 未鉴权接口补认证
- [ ] 单测：风险分级 / plan 解析 / 审批状态机

### 12.2 前端
- [ ] types + api 补齐
- [ ] `AiTerminal.vue`（主机选择/会话/计划/审批/输出/TerminalPane 兜底）
- [ ] 路由 `/ai-terminal` 替换 redirect；保留 `?session=`
- [ ] 菜单「智能终端」
- [ ] `npm run build` / `go build ./...` / `go vet ./...` 通过

### 12.3 验收
- [ ] 端到端：建会话 → 提问 → 出方案 → 审批(low 自动/high 确认) → 执行 → 审计 → 继续

## 13. 阶段计划

| 阶段 | 范围 |
|---|---|
| Sprint 1（本轮） | 单主机 AI 方案 + 审批执行 + 审计 + AiTerminal 页面 + aiops 鉴权 |
| Sprint 2 | 多步循环、风险放行配置、会话持久化、故障联动、多主机 |
| Sprint 3 | Runbook 引擎、知识库、RBAC 与第二审批人 |

## 14. 交付物

- 后端：`ai_terminal.go`、`models.go` 迁移、`main.go` 注册。
- 前端：`AiTerminal.vue`、路由、菜单、`api/types`。
- 文档：本设计 + 迁移说明 + 测试清单。

---

*（本文件随评审反馈更新；第 4 节决策为 MVP 默认值，若变更先改本节。）*
