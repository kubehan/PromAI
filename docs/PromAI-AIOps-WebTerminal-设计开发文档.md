# PromAI - AIOps 平台演进与 AI-Chat Web终端设计开发文档

> 文档版本：V1.0  
> 编写日期：2025-10-28  
> 适用范围：PromAI 平台整体演进设计与核心功能开发  
> 文档状态：设计阶段（待评审）

## 目录

- [1. 文档概述](#1-文档概述)
- [2. 项目现状分析](#2-项目现状分析)
- [3. AIOps 平台演进设计](#3-aiops-平台演进设计)
- [4. AI-Chat Web终端能力设计](#4-ai-chat-web终端能力设计)
- [5. 整体架构融合设计](#5-整体架构融合设计)
- [6. 数据模型设计](#6-数据模型设计)
- [7. API 接口设计](#7-api-接口设计)
- [8. 前端界面设计](#8-前端界面设计)
- [9. 安全设计](#9-安全设计)
- [10. 配置设计](#10-配置设计)
- [11. 功能开发计划（具体到人，人员留空）](#11-功能开发计划具体到人人员留空)
- [12. 测试验收标准](#12-测试验收标准)
- [13. 风险评估与应对](#13-风险评估与应对)
- [14. 附录](#14-附录)


## 1. 文档概述

### 1.1 编写目的
本文档旨在将 PromAI 从当前的"Prometheus 巡检报告生成器"演进为"面向业务视角的智能运维（AIOps）平台"，并在此基础上重点建设 **AI-Chat Web终端能力**，实现从告警到事件、从故障到根因、从排查到闭环的全链路智能运维。

### 1.2 设计范围
- **AIOps 平台演进**：事件中心、故障管理、告警关联与降噪、拓扑与CMDB、AI根因分析（AI-RCA）、知识库（Runbook）、值班管理、SLO管理、AIOps驾驶舱
- **AI-Chat Web终端**：安全SSH登录、多窗口、命令执行、命令审批、会话录制、会话回放、会话分享、会话总结、AI协同、全量审计
- **系统融合**：与现有告警子系统、通知渠道、AI Chat、pi-agent 的深度整合

### 1.3 术语定义
| 术语 | 定义 |
|------|------|
| AIOps | 智能运维（Artificial Intelligence for IT Operations），利用AI/ML技术实现运维自动化、智能化 |
| Incident | 故障单，聚合多个相关事件的故障实体，贯穿故障全生命周期管理 |
| Event | 统一事件，来自各类数据源的标准化运维事件 |
| RCA | 根因分析（Root Cause Analysis） |
| MTTA | 平均确认时间（Mean Time To Acknowledge） |
| MTTR | 平均修复时间（Mean Time To Repair） |
| Runbook | 运维操作手册，标准化的故障处理预案 |
| Playbook | 可执行的自动化处置脚本/流程 |
| CMDB | 配置管理数据库（Configuration Management Database） |
| PTY | 伪终端（Pseudo Terminal） |

## 2. 项目现状分析

### 2.1 当前能力盘点
| 模块 | 状态 | 说明 |
|------|------|------|
| 核心定位 | 已有 | Prometheus 巡检报告生成器，具备多数据源、模板、定时、通知等能力 |
| 告警子系统 | 较完善 | 已有 AlertRule/Instance/History/Silence/Inhibit/Route/Group/NotifyLog，evaluator/dispatcher/notifier 完整链路 |
| AI 能力 | 有基础 | AI Chat、Skills 管理、pi-agent 模块、通知模板预留 cause/impact 字段 |
| 管理后台 | 较完善 | Vue 3 + Element Plus，路由清晰，功能模块齐全 |
| 数据存储 | 可用 | SQLite（GORM），模型定义集中 |
| 实时交互 | 部分支持 | SSE 用于 AI Chat，已处理 gzip 跳过 |

### 2.2 能力缺口
| 能力域 | 缺口 | 建议优先级 |
|--------|------|------------|
| 事件聚合与关联 | 缺乏跨规则跨数据源关联、告警收敛 | P0 |
| 故障管理（Incident） | 无故障单、状态流转、Timeline、MTTA/MTTR | P0 |
| Web终端 | 完全缺失 | P0（本次核心） |
| 命令审批与审计 | 完全缺失 | P0 |
| 拓扑与CMDB | 缺失 | P1 |
| AI-RCA深度整合 | AI Chat 存在但未与故障上下文深度结合 | P1 |
| 知识库Runbook | 缺失 | P1 |
| 值班管理OnCall | 缺失 | P1 |
| SLO/错误预算 | 缺失 | P2 |

## 3. AIOps 平台演进设计

### 3.1 设计愿景
从"Prometheus 巡检与告警平台"演进为"面向业务视角的智能运维（AIOps）平台"。核心理念：**"从告警到事件，从事件到故障，从故障到根因，从根因到闭环"**。

### 3.2 核心能力目标
| 能力 | 目标描述 |
|------|----------|
| 统一事件入口（Event Hub） | 将 Prometheus 告警、日志、链路、变更、人工上报等多源事件统一纳管 |
| 告警收敛（Correlation） | 基于时间窗口、标签、拓扑依赖将上千条告警压缩为少量业务影响事件 |
| 故障全生命周期（Incident） | 故障单、处理流转、责任人、升级、Timeline、MTTA/MTTR 统计 |
| 智能根因（AI-RCA） | 结合拓扑、变更、指标、历史故障，自动生成疑似根因、影响面、建议处置 |
| 知识沉淀（Runbook） | 将处置经验固化并在相似故障中推荐 |
| 运维效能量化（SRE） | SLO/错误预算、告警有效率、收敛率、MTTA、MTTR 等指标 |
| 可闭环处置（Playbook） | 从建议到半自动/审批自愈，严格权限与审计控制 |

### 3.3 整体架构
```
┌─────────────────────────────────────────────────────────────────────────┐
│ UI层（Vue3） - 事件中心/故障管理/拓扑图/值班管理/SLO/智能分析/知识库        │
└─────────────────────────────────────────────────────────────────────────┘
                                    │ REST / SSE / WebSocket
┌─────────────────────────────────────────────────────────────────────────┐
│ API网关（main.go） - JWT鉴权、RBAC、审计中间件                              │
└─────────────────────────────────────────────────────────────────────────┘
                                    │
┌─────────────┬─────────────┬─────────────┬─────────────┬─────────────────┐
│ 事件中心      │ 故障管理      │ 智能引擎      │ 知识与自愈    │ 可观测治理        │
│ (Event Hub)   │ (Incident)    │ (AI-RCA/关联) │ (Runbook/Play)│ (SLO/OnCall/Audit)│
└─────────────┴─────────────┴─────────────┴─────────────┴─────────────────┘
                                    │
┌─────────────────────────────────────────────────────────────────────────┐
│ 事件总线（EventBus） - 统一事件封装                                         │
└─────────────────────────────────────────────────────────────────────────┘
                                    │
┌─────────────┬─────────────┬─────────────┬─────────────────────────────────┐
│ 数据接入      │ 关联引擎      │ 处置引擎      │ 存储层（GORM + SQLite/Postgres） │
└─────────────┴─────────────┴─────────────┴─────────────────────────────────┘
```

### 3.4 核心流程
1. **告警->事件->关联->故障**：Evaluator产生告警 -> 桥接到EventBus -> Correlation引擎关联 -> 自动/手动创建Incident
2. **故障->AI-RCA**：收集拓扑、告警、变更、指标、历史故障 -> 组装上下文 -> pi-agent分析 -> 结构化结果落库
3. **处置->闭环**：认领->调查->匹配Runbook->执行->缓解->解决->关闭，统计MTTA/MTTR，沉淀经验


## 4. AI-Chat Web终端能力设计

### 4.1 设计目标
将 AI Chat 升级为 **"AI 驱动的安全运维终端（AI-Terminal）"**，实现浏览器安全登录受管主机、命令执行与审批、会话全生命周期管理、AI 辅助排查与沉淀。

### 4.2 核心能力清单
| 能力 | 优先级 | 说明 |
|------|--------|------|
| 安全SSH登录 | P0 | 主机白名单、密钥/密码（优先密钥）、权限校验、凭据加密存储 |
| Web终端渲染（xterm.js） | P0 | UTF-8、ANSI色彩、光标、快捷键、复制粘贴、交互式兼容 |
| 多窗口（多标签） | P0 | 多个独立SSH会话同时打开，支持切换、关闭，预留分屏 |
| 命令执行与交互兼容 | P0 | 全量透传stdin/stdout/stderr，兼容top/vim/less/tail -f等 |
| 命令审批 | P0 | 高危命令拦截、审批队列、一次性放行、按环境/角色策略 |
| 会话录制（asciicast v2） | P0 | 全量I/O录制、元数据入库、文件落盘 |
| 会话回放 | P0 | 只读回放、倍速、暂停、跳转 |
| 会话总结（AI） | P0 | 脱敏精简后调用pi-agent生成结构化总结，关联故障单/知识库 |
| 会话分享 | P1 | 只读临时分享链接、时效、次数限制、可撤销 |
| 审计日志（不可变） | P0 | 全量操作留痕，仅增不改 |
| AI协同 | P1 | 自然语言转命令、命令解释、风险提示、异常分析 |

### 4.3 整体架构
```
前端（Vue3 + xterm.js） --WSS/REST--> API网关（JWT+RBAC+审计）
                                        │
                                    ┌───┴───┐
                                  Hub    Policy
                                    │       │
                                  SSH    Approval
                                    │       │
                                Session  Recorder
                                    │       │
                                  Audit   Share
                                    │       │
                                   Vault   AI Summarizer
                                    │       │
                                  DB + Files
```

### 4.4 WebSocket通信协议
**客户端->服务端：** init、input、resize、ping、close  
**服务端->客户端：** ready、output、approval_required、approval_result、error、warn、closed、pong

### 4.5 命令审批策略要点
- 复合命令（&&、;、||、|）逐段匹配，任意一段命中即拦截
- 生产环境（env=prod）高危命令强制审批
- 整机强制审批（TerminalHost.RequireApproval=true）
- 审批超时自动拒绝（默认10分钟）
- 一次性放行，仅对当前命令生效

### 4.6 AI总结结构化输出
```json
{
  "title": "简短排查标题",
  "summary": "整体排查概述",
  "commands": [{"cmd": "...", "purpose": "..."}],
  "findings": [{"point": "...", "evidence": "..."}],
  "actions": [{"action": "...", "result": "..."}],
  "risks": [{"risk": "...", "suggestion": "..."}],
  "suggestions": [{"item": "...", "priority": "high|medium|low"}]
}
```


## 5. 整体架构融合设计

### 5.1 融合原则
- **渐进式演进**：增量扩展，不推翻重构
- **最大复用**：复用 alerting、pi-agent、database、auth、notify 等现有模块
- **向后兼容**：保留现有 AlertInstance，新增 Event 双写过渡
- **安全优先**：Web终端安全基石先行
- **最小可行**：先打通核心闭环再扩展

### 5.2 模块融合映射
| 现有模块 | 融合方式 | 说明 |
|----------|----------|------|
| alerting（evaluator/dispatcher/notifier） | 增强+桥接 | 保留逻辑，新增桥接层将告警转发到EventBus，逐步过渡到Event中心视图 |
| notifications | 扩展 | 结合OnCall、Incident状态使用故障通知模板，支持值班人路由 |
| pi-agent/AI Chat | 深度复用 | RCA、终端AI总结、终端AI协同均复用配置、调用链、技能机制 |
| database（GORM/SQLite） | 增量扩展 | 新增表，AutoMigrate，预留Postgres切换路径 |
| main/routes/auth | 最小侵入 | 尾部新增路由，沿用JWT鉴权 |
| Incident（规划中） | 深度关联 | 终端可关联故障单，排查总结附加到Timeline |

### 5.3 事件流融合
```
Prometheus告警 -> AlertInstance(现有) + Event(新增，双写)
                -> CorrelationGroup关联
                -> Incident聚合（手动/建议）
                -> AI-RCA上下文收集
                -> 终端排查（可从Incident一键进入）
                -> AI总结沉淀到IncidentTimeline
                -> 经验固化到Runbook
```

## 6. 数据模型设计

### 6.1 AIOps 新增模型
| 模型 | 用途 | 说明 |
|------|------|------|
| Event | 统一事件 | source/type/severity/status/labels/fingerprint/incident_id/correlation_group_id |
| Incident | 故障单 | incident_no/status/severity/assignee/starts_at/acknowledged_at/resolved_at/MTTA/MTTR |
| IncidentTimeline | 故障时间线（不可变） | event_created/ack/assign/comment/escalate/ai_rca/runbook/resolve/close |
| IncidentRelation | 关联关系 | 事件-故障、故障-变更、故障-Runbook、故障-CI |
| CMDBItem | 配置项 | ciid/name/type/env/cluster/owner/team/tags |
| TopologyEdge | 拓扑依赖 | from_ci_id/to_ci_id/type/weight |
| ChangeEvent | 变更事件 | changeno/source/type/affected/started_at/finished_at |
| Runbook | 知识库 | name/category/content/applicable/checklist/use_count |
| OnCallSchedule | 值班排班 | group_id/user_id/shift_type/start_at/end_at |
| RCAResult | AI根因结果 | incident_id/model/suspects/impact/suggestions/confidence |
| CorrelationGroup | 告警关联组 | group_key/strategy/event_count/incident_id |

### 6.2 Web终端新增模型
| 模型 | 用途 | 说明 |
|------|------|------|
| TerminalHost | 受管主机清单 | host_key/host/port/username/auth_type/env/group/allow_users/allow_roles/require_approval |
| TerminalCredential | 凭据（加密） | host_id/key_ciphertext/pass_ciphertext/key_nonce/pass_nonce/fingerprint（json:"-"） |
| TerminalSession | 会话元数据 | id/user_id/host_id/status/cols/rows/incident_id/record_path/started_at/ended_at/duration_sec |
| TerminalCommandLog | 命令日志 | session_id/command/action/rule_id/approval_id/executed/blocked/block_reason |
| TerminalApproval | 审批队列 | req_id/session_id/command/reason/severity/status/approver/expires_at/approved_at/rejected_at |
| TerminalApprovalRule | 审批规则 | name/enabled/priority/match_type/pattern/envs/groups/users/roles/action/severity |
| TerminalAudit | 审计日志（不可变） | user_id/action/session_id/host_id/detail_json/client_ip/user_agent/result/created_at |
| TerminalShare | 会话分享 | share_token/session_id/owner_id/expires_at/max_views/used_views/status |
| TerminalSummary | AI总结 | session_id/incident_id/model/title/summary/commands_json/findings_json/actions_json/risks_json/suggestions_json |

### 6.3 模型注册
所有新增模型需在 `pkg/database/models.go` 的 `AutoMigrate` 列表中添加。

## 7. API 接口设计

### 7.1 AIOps API（新增）
| 方法 | 路径 | 功能 |
|------|------|------|
| GET/POST/PUT/DELETE | /api/promai/aiops/events | 事件中心CRUD/查询/过滤 |
| POST | /api/promai/aiops/events/merge | 合并事件到关联组 |
| POST | /api/promai/aiops/events/to-incident | 从事件创建Incident |
| GET/POST/PUT/DELETE | /api/promai/aiops/incidents | 故障管理CRUD |
| POST | /api/promai/aiops/incidents/:id/ack/assign/resolve | 状态流转 |
| GET | /api/promai/aiops/incidents/:id/timeline | 故障时间线 |
| POST/GET | /api/promai/aiops/incidents/:id/rca | 触发/获取AI-RCA |
| GET/POST/PUT/DELETE | /api/promai/aiops/cmdb | CMDB管理 |
| GET | /api/promai/aiops/topology | 拓扑图数据 |
| GET | /api/promai/aiops/topology/impact | 影响面分析 |
| GET/POST/PUT/DELETE | /api/promai/aiops/runbooks | 知识库管理 |
| POST | /api/promai/aiops/runbooks/match | 匹配Runbook |
| GET/POST/PUT | /api/promai/aiops/oncall | 值班管理 |
| GET | /api/promai/aiops/oncall/current | 当前值班人 |
| GET/POST/PUT/DELETE | /api/promai/aiops/changes | 变更事件 |
| GET | /api/promai/aiops/stats/overview | AIOps驾驶舱统计 |

### 7.2 Web终端 API（新增）
| 方法 | 路径 | 功能 | 权限 |
|------|------|------|------|
| GET | /api/promai/terminal/hosts | 可用主机列表 | 已登录 |
| GET/POST/PUT/DELETE | /api/promai/terminal/hosts/manage | 主机管理（预留） | terminal_admin |
| GET | /api/promai/terminal/sessions | 会话列表 | 已登录 |
| GET | /api/promai/terminal/sessions/:id | 会话详情 | 发起人/管理员 |
| POST | /api/promai/terminal/sessions/:id/end | 强制结束 | 发起人/terminal_admin |
| GET | /api/promai/terminal/approvals/pending | 待审批列表 | 可审批人 |
| GET | /api/promai/terminal/approvals/mine | 我的审批记录 | 已登录 |
| POST | /api/promai/terminal/approvals/:reqId/approve | 审批通过 | 可审批人 |
| POST | /api/promai/terminal/approvals/:reqId/reject | 审批拒绝 | 可审批人 |
| POST | /api/promai/terminal/sessions/:id/share | 创建分享 | 发起人 |
| GET | /api/promai/terminal/shares | 我的分享列表 | 已登录 |
| POST | /api/promai/terminal/shares/:token/revoke | 撤销分享 | 创建者/管理员 |
| GET | /api/promai/terminal/shares/:token | 分享元数据（公开） | 无需登录 |
| GET | /api/promai/terminal/shares/:token/record | 分享录制（公开） | 无需登录 |
| POST | /api/promai/terminal/sessions/:id/summary | 触发AI总结 | 发起人/关联人员 |
| GET | /api/promai/terminal/sessions/:id/summary | 获取AI总结 | 发起人/关联人员/管理员 |
| POST | /api/promai/terminal/sessions/:id/link-incident | 关联故障单 | 发起人 |
| GET | /api/promai/terminal/audit | 审计日志 | terminal_admin |

### 7.3 WebSocket
- **端点：** `GET /api/promai/terminal/ws`
- **鉴权：** JWT（Authorization或query token）
- **协议：** 支持 `Sec-WebSocket-Protocol: terminal.v1`


## 8. 前端界面设计

### 8.1 路由新增
**AIOps路由（router/index.ts）：** /aiops/events、/aiops/incidents、/aiops/incidents/:id、/aiops/topology、/aiops/cmdb、/aiops/changes、/aiops/runbooks、/aiops/oncall、/aiops/slo、/aiops/dashboard

**终端路由：** /ai-terminal、/terminal/share/:token（noAuth）

### 8.2 新增视图
| 视图路径 | 功能说明 |
|----------|----------|
| views/aiops/Events.vue | 事件中心：过滤、关联组、批量合并、转故障 |
| views/aiops/Incidents.vue | 故障列表：状态/严重度/责任人筛选、MTTA/MTTR展示 |
| views/aiops/IncidentDetail.vue | 故障详情：概览、Timeline、AI-RCA、关联事件、影响面、Runbook、变更 |
| views/aiops/Topology.vue | 服务拓扑：vis-network渲染、故障高亮、影响面分析 |
| views/aiops/CMDB.vue | 配置项管理 |
| views/aiops/Runbooks.vue | 知识库：Markdown编辑、适用条件、使用统计 |
| views/aiops/Dashboard.vue | AIOps驾驶舱：KPI卡片、趋势图、收敛率 |
| views/Terminal.vue | 独立终端页：主机列表、多标签、终端容器、审批提醒、状态栏 |
| views/TerminalShare.vue | 只读分享回放页 |
| AiChat.vue（改造） | 新增"AI对话 | Web终端"切换，终端视图嵌入多标签终端 |

### 8.3 终端组件（components/terminal/）
| 组件 | 功能 |
|------|------|
| TerminalTabs.vue | 标签页管理、新建/关闭、右键菜单 |
| TerminalHostList.vue | 主机列表（按环境分组）、搜索、权限状态 |
| TerminalXterm.vue | xterm.js封装、WS连接、输入输出、resize、fit |
| TerminalApprovalModal.vue | 审批弹窗（命令、原因、严重度、倒计时） |
| TerminalApprovalDrawer.vue | 待审批抽屉 |
| TerminalSessionList.vue | 历史会话（回放、分享、总结、关联故障） |
| TerminalReplay.vue | asciicast回放器（倍速、暂停、跳转） |
| TerminalShareDialog.vue | 创建分享（有效期、次数） |
| TerminalSummaryPanel.vue | AI总结渲染（结构化卡片） |
| TerminalAIAssist.vue | 嵌入式AI助手（转命令、解释、分析） |

### 8.4 状态管理（Pinia）
stores/terminal.ts 管理 hosts、tabs、activeSessionId、approvals、ws、connecting 等状态。

## 9. 安全设计

### 9.1 凭据安全
- AES-256-GCM加密，独立nonce，密钥从环境变量 TERMINAL_VAULT_KEY 注入（>=32字节）
- 加密字段 json:"-"，仅服务端解密，用后立即清理内存
- 禁止日志打印凭据

### 9.2 访问控制（RBAC）
| 角色 | 权限 |
|------|------|
| terminal_viewer | 会话列表、回放、分享查看 |
| terminal_operator | 登录dev/test，高危需审批 |
| terminal_oncall | 登录stage及授权prod，高危强制审批 |
| terminal_admin | 主机/凭据/规则/审批/强制结束/审计全权限 |

### 9.3 命令审批安全
- 复合命令逐段拦截
- prod环境加权
- 整机强制审批
- 审批超时自动拒绝
- 一次性放行
- 服务端强制校验审批状态

### 9.4 会话安全
- 主机白名单强制校验，禁止前端传入任意host/port
- 并发限制（默认5）、最大时长（240分钟）、空闲超时（10分钟）
- 强制WSS、Origin校验
- 心跳保活

### 9.5 录制与分享安全
- asciicast v2落盘，路径隔离
- 分享高熵token、时效（30分钟）、次数（5次）、可撤销
- 只读回放，无输入无WS
- 全量访问审计

### 9.6 AI总结安全
- 脱敏（token/key/password/Authorization等）
- 精简上下文，限制长度（默认10000字符）
- 强制结构化JSON输出
- RawPrompt/RawResponse不返回前端
- 不自动触发，需手动生成

## 10. 配置设计

### 10.1 config/config.yaml 新增
```yaml
aiops:
  enabled: true
  correlation:
    enabled: true
    time_window_minutes: 15
    min_event_count: 2
  incident:
    auto_create: false  # 建议建单，避免误建
  rca:
    enabled: true
    auto_trigger: false
    model: ""
    max_context_chars: 12000

terminal:
  enabled: true
  vault_key_env: "TERMINAL_VAULT_KEY"
  max_concurrent_per_user: 5
  max_session_duration_minutes: 240
  idle_timeout_minutes: 10
  approval_timeout_minutes: 10
  record_dir: "outputs/terminal-records"
  record_retention_days: 30
  share_default_expire_minutes: 30
  share_default_max_views: 5
  require_wss: true
  allowed_origins: ["http://localhost:5173", "https://your-domain.com"]
  audit_enabled: true
  ai_summary_enabled: true
  ai_summary_max_context_chars: 10000
```

### 10.2 环境变量
- `TERMINAL_VAULT_KEY`：AES-256密钥（必需，生产环境）
- `PROMAI_JWT_SECRET`：已存在，继续使用


## 11. 功能开发计划（具体到人，人员留空）

> 说明：以下人员字段均留空，由项目负责人根据实际排期分配。任务按优先级和依赖关系排序，采用增量迭代方式开发。

### 11.1 总体里程碑
| 里程碑 | 时间预估 | 目标 |
|--------|----------|------|
| M1：AIOps基础（事件+故障） | 第1-2周 | 事件中心、故障管理、状态流转、Timeline |
| M2：告警关联+拓扑CMDB | 第2-3周 | Correlation引擎、CMDB、拓扑、影响面 |
| M3：Web终端核心 | 第3-4周 | SSH登录、多窗口、命令审批、录制回放 |
| M4：AI-RCA+终端AI | 第4-5周 | AI-RCA、终端AI总结、AI协同、故障联动 |
| M5：知识库+值班+驾驶舱+加固 | 第5-6周 | Runbook、OnCall、SLO、审计加固、测试验收 |

### 11.2 阶段一：AIOps 基础模块（P0）

#### 任务组 1.1 数据模型与基础设施
| 任务ID | 任务名称 | 详细说明 | 预估工时(h) | 优先级 | 负责人 | 依赖 | 验收标准 |
|--------|----------|----------|--------------|--------|--------|------|----------|
| AIOPS-001 | AIOps数据模型设计与实现 | 新增Event、Incident、IncidentTimeline、IncidentRelation、CorrelationGroup等模型到pkg/database/models.go并AutoMigrate | 4 | P0 | （留空） | 无 | 模型编译通过，AutoMigrate成功创建表 |
| AIOPS-002 | 事件桥接层实现 | 在alerting/evaluator/dispatcher产出告警时桥接到EventBus，双写AlertInstance+Event | 6 | P0 | （留空） | AIOPS-001 | 告警触发时Event表有记录，字段映射正确 |
| AIOPS-003 | 事件中心API实现 | pkg/eventhub实现事件列表、过滤、详情、合并、转故障接口 | 6 | P0 | （留空） | AIOPS-001,AIOPS-002 | REST接口可用，权限校验正常 |

#### 任务组 1.2 故障管理
| 任务ID | 任务名称 | 详细说明 | 预估工时(h) | 优先级 | 负责人 | 依赖 | 验收标准 |
|--------|----------|----------|--------------|--------|--------|------|----------|
| AIOPS-004 | 故障管理服务实现 | pkg/incident实现CRUD、状态机校验、INC编号生成、MTTA/MTTR计算、Timeline写入 | 8 | P0 | （留空） | AIOPS-001 | 状态流转校验正确，编号唯一，时间统计准确 |
| AIOPS-005 | 故障管理API实现 | 事件转故障、认领、转派、解决、关闭、Timeline等接口 | 4 | P0 | （留空） | AIOPS-004 | 全部状态流转接口可用 |
| AIOPS-006 | 前端-事件中心视图 | views/aiops/Events.vue：列表、过滤、关联组、合并、转故障 | 8 | P0 | （留空） | AIOPS-003 | 界面正常，操作流程完整 |
| AIOPS-007 | 前端-故障管理视图 | views/aiops/Incidents.vue + IncidentDetail.vue：列表、详情、Timeline、流转操作 | 10 | P0 | （留空） | AIOPS-005 | 故障全生命周期可操作，Timeline正确展示 |
| AIOPS-008 | 路由与菜单集成 | router/index.ts新增AIOps路由，App.vue侧边栏新增AIOps分组 | 2 | P0 | （留空） | AIOPS-006,AIOPS-007 | 菜单可访问，权限路由正常 |

**小计：** 48h

### 11.3 阶段二：告警关联、拓扑、CMDB（P0-P1）

#### 任务组 2.1 告警关联引擎
| 任务ID | 任务名称 | 详细说明 | 预估工时(h) | 优先级 | 负责人 | 依赖 | 验收标准 |
|--------|----------|----------|--------------|--------|--------|------|----------|
| AIOPS-009 | 关联引擎实现 | pkg/correlation：时间窗口+标签交集策略，规则可配置，写入CorrelationGroup | 8 | P0 | （留空） | AIOPS-001,AIOPS-002 | 同主机同服务短时间告警自动归组 |
| AIOPS-010 | 关联组API与前端增强 | 关联组展示、自动建单建议、一键转故障 | 4 | P0 | （留空） | AIOPS-009,AIOPS-006 | 事件中心按关联组收敛展示，收敛率可见 |

#### 任务组 2.2 拓扑、CMDB、变更
| 任务ID | 任务名称 | 详细说明 | 预估工时(h) | 优先级 | 负责人 | 依赖 | 验收标准 |
|--------|----------|----------|--------------|--------|--------|------|----------|
| AIOPS-011 | CMDB模型与API | 新增CMDBItem，CRUD接口，标签支持 | 4 | P1 | （留空） | AIOPS-001 | CMDB增删改查正常 |
| AIOPS-012 | 拓扑模型与服务 | 新增TopologyEdge，邻接表构建，BFS影响面分析 | 6 | P1 | （留空） | AIOPS-011 | 拓扑图数据接口正常，影响面计算正确 |
| AIOPS-013 | 变更事件模型与API | 新增ChangeEvent，CRUD，关联Incident | 3 | P1 | （留空） | AIOPS-001 | 变更事件可录入并关联 |
| AIOPS-014 | 前端-CMDB、拓扑、变更视图 | CMDB.vue、Topology.vue（vis-network）、Changes.vue | 10 | P1 | （留空） | AIOPS-011,AIOPS-012,AIOPS-013 | 拓扑渲染正常，节点点击可查看详情，影响面可展示 |

**小计：** 35h

### 11.4 阶段三：Web终端核心能力（P0 - 本次核心）

#### 任务组 3.1 基础设施（SSH、WS、凭据）
| 任务ID | 任务名称 | 详细说明 | 预估工时(h) | 优先级 | 负责人 | 依赖 | 验收标准 |
|--------|----------|----------|--------------|--------|--------|------|----------|
| TERM-001 | 终端数据模型实现 | 新增TerminalHost/Credential/Session/CommandLog/Approval/ApprovalRule/Audit/Share/Summary，AutoMigrate | 6 | P0 | （留空） | 无 | 模型编译通过，表创建成功 |
| TERM-002 | 凭据加密Vault实现 | pkg/terminal/vault：AES-256-GCM加解密，密钥从环境变量读取，内存清理 | 4 | P0 | （留空） | TERM-001 | 加解密正常，密文不出现在日志 |
| TERM-003 | SSH管理器实现 | pkg/terminal/ssh：基于golang.org/x/crypto/ssh，PTY请求，认证（key/password），白名单校验 | 8 | P0 | （留空） | TERM-001,TERM-002 | 可连接白名单主机，PTY交互正常 |
| TERM-004 | WebSocket Hub实现 | pkg/terminal/hub：连接管理、心跳、消息路由、并发限制、空闲超时 | 8 | P0 | （留空） | TERM-003 | WS握手鉴权正常，心跳保活，断线清理 |

#### 任务组 3.2 Web终端前端核心
| 任务ID | 任务名称 | 详细说明 | 预估工时(h) | 优先级 | 负责人 | 依赖 | 验收标准 |
|--------|----------|----------|--------------|--------|--------|------|----------|
| TERM-005 | 主机列表与权限API | GET /terminal/hosts，仅返回有权限主机，过滤敏感字段 | 3 | P0 | （留空） | TERM-001,TERM-004 | 权限校验正确，不返回未授权主机 |
| TERM-006 | xterm.js封装组件 | TerminalXterm.vue：初始化xterm、fitAddon、webLinksAddon、输入输出、resize、WS通信 | 8 | P0 | （留空） | TERM-004 | 终端渲染正常，输入输出无延迟感知，窗口自适应 |
| TERM-007 | 主机列表、标签页组件 | TerminalHostList.vue、TerminalTabs.vue，Pinia store（terminal.ts） | 6 | P0 | （留空） | TERM-005,TERM-006 | 多主机切换、多标签管理正常 |
| TERM-008 | 终端页面整合 | views/Terminal.vue + AiChat.vue改造（AI对话|Web终端切换） | 6 | P0 | （留空） | TERM-007 | 独立页和整合页均可用，布局正常 |
| TERM-009 | 路由注册 | router/index.ts新增/ai-terminal、/terminal/share/:token | 1 | P0 | （留空） | TERM-008 | 路由可访问，分享页noAuth生效 |

#### 任务组 3.3 命令审批与审计
| 任务ID | 任务名称 | 详细说明 | 预估工时(h) | 优先级 | 负责人 | 依赖 | 验收标准 |
|--------|----------|----------|--------------|--------|--------|------|----------|
| TERM-010 | 命令策略引擎 | pkg/terminal/policy：命令拆分（&&、;、||、|）、正则匹配、环境加权、整机强制审批 | 8 | P0 | （留空） | TERM-004 | 高危命令拦截，复合命令逐段校验正确 |
| TERM-011 | 审批服务实现 | pkg/terminal/approval：队列、超时自动拒绝、一次性放行、权限校验、通知推送预留 | 6 | P0 | （留空） | TERM-010 | 审批流转完整，超时生效，一次性放行生效 |
| TERM-012 | 审批API与前端组件 | 审批接口（pending/mine/approve/reject）、TerminalApprovalModal.vue、TerminalApprovalDrawer.vue | 6 | P0 | （留空） | TERM-011 | 审批弹窗正常，倒计时正确，审批结果推送到终端 |
| TERM-013 | 审计日志实现 | pkg/terminal/audit：全量记录登录/连接/执行/审批/分享/结束等，仅增不改，审计API（terminal_admin） | 4 | P0 | （留空） | TERM-004 | 所有关键操作均有审计记录，不可修改删除 |

**小计：** 74h

### 11.5 阶段四：会话录制、分享、AI总结、故障联动（P0-P1）

#### 任务组 4.1 会话录制与回放
| 任务ID | 任务名称 | 详细说明 | 预估工时(h) | 优先级 | 负责人 | 依赖 | 验收标准 |
|--------|----------tag:task | 依赖 | 验收标准 |
|----------|----------|----------|--------------|--------|--------|------|----------|
| TERM-014 | 会话录制器实现 | pkg/terminal/recorder：asciicast v2写入（头部+事件流），落盘outputs/terminal-records，收尾处理 | 6 | P0 | （留空） | TERM-004,TERM-006 | 会话结束自动生成.cast文件，格式符合asciicast v2 |
| TERM-015 | 历史会话列表与详情 | 会话列表API、TerminalSessionList.vue、会话详情页（命令日志、审批记录） | 4 | P0 | （留空） | TERM-001,TERM-014 | 历史会话可查看，命令日志完整 |
| TERM-016 | 回放组件实现 | TerminalReplay.vue：解析asciicast v2，按时间轴写入xterm，倍速、暂停、跳转、全屏 | 6 | P0 | （留空） | TERM-014,TERM-015 | 回放还原度高，控制正常 |

#### 任务组 4.2 会话分享
| 任务ID | 任务名称 | 详细说明 | 预估工时(h) | 优先级 | 负责人 | 依赖 | 验收标准 |
|--------|----------|----------|--------------|--------|--------|------|----------|
| TERM-017 | 分享服务实现 | pkg/terminal/share：高熵token（crypto/rand 32字节）、时效、次数、IP记录、撤销 | 4 | P1 | （留空） | TERM-001,TERM-014 | token不可枚举，时效次数限制生效，可撤销 |
| TERM-018 | 分享API与前端 | 创建分享（TerminalShareDialog.vue）、我的分享列表、撤销；TerminalShare.vue（noAuth只读回放） | 5 | P1 | （留空） | TERM-017,TERM-016 | 分享链接可创建，分享页仅回放无输入无WS |
| TERM-019 | 分享审计 | 分享创建/访问全量记录到TerminalAudit | 1 | P1 | （留空） | TERM-017,TERM-013 | 分享全流程有审计 |

#### 任务组 4.3 AI总结与故障联动
| 任务ID | 任务名称 | 详细说明 | 预估工时(h) | 优先级 | 负责人 | 依赖 | 验收标准 |
|--------|----------|----------|--------------|--------|--------|------|----------|
| TERM-020 | AI总结引擎实现 | pkg/terminal/ai：脱敏（token/key/password等）、精简、上下文组装（限制10000字符）、调用pi-agent、强制结构化JSON解析 | 8 | P0 | （留空） | TERM-014,pi-agent现有 | 脱敏生效，结构化输出解析成功，RawPrompt/RawResponse不暴露 |
| TERM-021 | AI总结API与面板 | 触发总结（异步）、获取总结、TerminalSummaryPanel.vue渲染结构化卡片 | 4 | P0 | （留空） | TERM-020,TERM-015 | 总结可生成，卡片渲染正确 |
| TERM-022 | 终端-AI助手嵌入 | TerminalAIAssist.vue：自然语言转命令（风险提示）、命令解释、异常分析，基于当前会话上下文 | 6 | P1 | （留空） | TERM-020,TERM-006 | AI助手可用，转命令带风险等级提示 |
| TERM-023 | 故障联动实现 | 会话关联Incident（link-incident）、总结附加到IncidentTimeline，IncidentDetail页一键跳转终端 | 6 | P0 | （留空） | TERM-021,AIOPS-004,AIOPS-007 | 从故障详情可进入终端并关联，总结可附加到Timeline |

**小计：** 40h

### 11.6 阶段五：AI-RCA、知识库、值班、驾驶舱、加固（P1-P2）

#### 任务组 5.1 AI-RCA
| 任务ID | 任务名称 | 详细说明 | 预估工时(h) | 优先级 | 负责人 | 依赖 | 验收标准 |
|--------|----------|----------|--------------|--------|--------|------|----------|
| RCA-001 | RCA上下文构建器 | pkg/rca/context_builder：收集Incident、关联Event、CMDB、拓扑、变更、指标快照、历史相似Incident | 8 | P1 | （留空） | AIOPS-004,AIOPS-012,AIOPS-013 | 上下文组装完整，数据结构合理 |
| RCA-002 | RCA引擎实现 | Prompt模板（强制JSON）、调用pi-agent、结构化解析、RCAResult落库、异步触发 | 6 | P1 | （留空） | RCA-001,pi-agent现有 | RCA可触发并返回结构化结果 |
| RCA-003 | RCA前端整合 | IncidentDetail新增"AI智能分析"面板：疑似根因、证据、影响面、建议步骤、待确认假设 | 6 | P1 | （留空） | RCA-002,AIOPS-007 | 面板渲染正确，结果可复制 |

**小计：** 20h

#### 任务组 5.2 知识库、值班、驾驶舱
| 任务ID | 任务名称 | 详细说明 | 预估工时(h) | 优先级 | 负责人 | 依赖 | 验收标准 |
|--------|----------|----------|--------------|--------|--------|------|----------|
| KB-001 | Runbook模型与API | 新增Runbook，CRUD、适用条件匹配、使用计数、关联Incident | 4 | P1 | （留空） | AIOPS-001 | Runbook管理正常，匹配逻辑可用 |
| KB-002 | OnCall模型与API | 新增OnCallSchedule，排班、当前值班查询，可接入通知路由预留 | 4 | P1 | （留空） | AIOPS-001 | 排班正确，当前值班可查询 |
| KB-003 | 前端-Runbook视图 | views/aiops/Runbooks.vue：Markdown编辑器（md-editor-v3或vditor）、适用条件编辑、使用统计 | 8 | P1 | （留空） | KB-001 | Markdown编辑预览正常，推荐逻辑生效 |
| KB-004 | 前端-OnCall视图 | views/aiops/OnCall.vue：排班表、值班日历 | 4 | P1 | （留空） | KB-002 | 排班界面可用 |
| KB-005 | AIOps驾驶舱 | views/aiops/Dashboard.vue：KPI（活跃告警/未闭环Incident/MTTA/MTTR/收敛率）、趋势图（ECharts） | 8 | P1 | （留空） | AIOPS-004,AIOPS-009 | 数据准确，图表渲染正常 |

**小计：** 28h

#### 任务组 5.3 安全加固、配置、测试、文档
| 任务ID | 任务名称 | 详细说明 | 预估工时(h) | 优先级 | 负责人 | 依赖 | 验收标准 |
|--------|----------|----------|--------------|--------|--------|------|----------|
| SEC-001 | 配置项集成 | config结构体新增aiops/terminal配置，config.yaml示例更新，环境变量校验 | 3 | P0 | （留空） | 全部模块 | 配置加载正常，默认值生效，环境变量校验正确 |
| SEC-002 | 安全加固 | 并发限制、空闲清理、最大时长、WSS/Origin校验、录制清理定时任务、审计全量走查 | 6 | P0 | （留空） | TERM全系列,SEC-001 | 所有安全限制生效，无敏感信息日志泄露 |
| SEC-003 | 权限细化与菜单权限 | 角色权限映射、菜单根据权限显示、API权限校验全量补充 | 4 | P0 | （留空） | 全部API | 不同角色权限隔离正确 |
| TEST-001 | 功能集成测试 | 核心流程测试：告警->事件->故障、终端登录->审批->录制->分享->总结->联动 | 8 | P0 | （留空） | 全部P0任务 | 核心闭环全流程通过 |
| TEST-002 | 安全测试 | 白名单绕过、凭据泄露、审批绕过、分享越权、审计篡改尝试 | 6 | P0 | （留空） | SEC-002 | 安全清单全项通过 |
| DOC-001 | 文档更新 | 更新README.md（新增AIOps和Web终端说明）、使用手册补充、部署说明（环境变量） | 4 | P1 | （留空） | 全部功能完成 | 文档完整可用 |

**小计：** 31h

### 11.7 工时汇总
| 阶段 | 工时（小时） | 说明 |
|------|--------------|------|
| 阶段一：AIOps基础 | 48 | 事件中心、故障管理 |
| 阶段二：关联、拓扑、CMDB | 35 | 告警收敛、拓扑图、CMDB、变更 |
| 阶段三：Web终端核心 | 74 | SSH、WS、多窗口、命令审批、审计 |
| 阶段四：录制、分享、AI总结、联动 | 40 | asciicast、分享、AI总结、故障联动 |
| 阶段五：AI-RCA、知识库、值班、驾驶舱、加固 | 79 | RCA、Runbook、OnCall、驾驶舱、安全加固、测试、文档 |
| **总计** | **276** | 约35个工作日（按8h/天） |

> 注：实际工时会因人员熟悉度、代码细节调整而有所偏差。建议按模块并行开发，充分利用现有代码复用降低工作量。

### 11.8 开发顺序建议
1. **并行起步（Week1）**：AIOPS-001/002/003（后端模型+事件桥接） + TERM-001/002/003（终端模型+Vault+SSH）
2. **核心闭环（Week2）**：故障管理全套 + 终端WS Hub + 前端终端基础
3. **安全先行（Week3）**：命令审批 + 审计（TERMA系列）务必优先完成再做分享等扩展
4. **AI整合（Week4-5）**：AI总结 + 故障联动 + AI-RCA
5. **收尾优化（Week5-6）**：驾驶舱、加固、测试、文档


## 12. 测试验收标准

### 12.1 AIOps 功能验收
| 测试项 | 验收标准 | 测试方式 |
|--------|----------|----------|
| 事件中心 | 告警自动转化为Event，支持过滤、合并、转故障 | 功能测试 |
| 告警关联 | 同主机/服务短时间告警自动归组，收敛率>50%（示例场景） | 功能测试 |
| 故障管理 | 故障单全生命周期流转正常，Timeline记录完整，MTTA/MTTR计算正确 | 功能测试 |
| 拓扑图 | CMDB+拓扑可维护，图形渲染正常，影响面分析正确 | 功能测试 |
| AI-RCA | 基于上下文能生成结构化疑似根因和建议，带证据 | 功能测试 |
| Runbook匹配 | 按标签/CI匹配到合适Runbook并可推荐 | 功能测试 |

### 12.2 Web终端功能验收
| 测试项 | 验收标准 | 测试方式 |
|--------|----------|----------|
| 安全登录 | 仅白名单主机可连接，非白名单被拒绝，权限校验生效 | 安全测试 |
| 多窗口 | 同时打开>=3个标签，切换、关闭正常，独立会话互不干扰 | 功能测试 |
| 交互兼容 | vim/top/less/tail -f/htop正常使用，复制粘贴正常，窗口resize自适应 | 功能测试 |
| 命令审批 | 高危命令拦截，复合命令逐段校验，审批通过后一次性放行，拒绝阻断，超时自动拒绝 | 功能测试 |
| 会话录制回放 | 会话结束生成.cast，回放完整还原，倍速/暂停/跳转正常 | 功能测试 |
| 会话分享 | 只读回放、过期失效、次数用尽失效、可撤销、无输入无WS | 安全测试 |
| AI总结 | 脱敏生效，结构化JSON正确渲染，可附加到故障Timeline | 功能测试 |
| AI协同 | 自然语言转命令带风险提示，命令解释可用 | 功能测试 |
| 故障联动 | 从IncidentDetail一键进入终端并关联，终端可关联故障单 | 功能测试 |

### 12.3 安全验收清单
| 检查项 | 验收标准 | 状态（待测） |
|--------|----------|--------------|
| 凭据加密 | 数据库无明文密码/私钥，接口无凭据返回，密钥从环境变量读取 | □ |
| 白名单强制 | 禁止前端传入host/port直连，服务端强制校验 | □ |
| 审批强制 | 高危命令无法绕过审批弹窗直接执行，服务端强制校验审批状态 | □ |
| 审计不可变 | TerminalAudit仅增不改，无更新删除接口 | □ |
| 分享隔离 | 分享页无输入组件、不建立WebSocket、仅回放 | □ |
| 脱敏生效 | AI总结上下文敏感信息替换为[REDACTED] | □ |
| WSS/Origin | 生产环境强制WSS，Origin校验生效 | □ |
| 权限隔离 | 不同角色权限严格隔离，越权请求被拒绝 | □ |
| 日志安全 | 日志中无密码、私钥、token、审批命令明文敏感信息 | □ |
| 并发限制 | 超过最大并发数被拒绝，空闲超时/最大时长生效 | □ |

## 13. 风险评估与应对

| 风险 | 影响 | 概率 | 风险等级 | 应对措施 | 责任建议 |
|------|------|------|----------|----------|----------|
| 凭据泄露 | 极高 | 低 | 严重 | AES-GCM加密、json:"-"、内存清理、环境变量、严禁日志打印 | 后端安全负责人 |
| 高危命令误操作 | 极高 | 中 | 严重 | 命令审批强制拦截、复合命令逐段校验、prod加权、整机强制审批、一次性放行 | 后端+运维 |
| SSH绕过白名单 | 极高 | 低 | 严重 | 禁止前端传入连接参数，仅用hostId查库、全量校验 | 后端 |
| 会话劫持/跨站 | 高 | 低 | 高 | JWT+Origin校验、WSS强制、会话归属校验 | 后端+前端 |
| 录制/分享泄露 | 高 | 中 | 高 | 权限校验、时效次数限制、可撤销、只读回放、脱敏 | 后端+前端 |
| 交互式程序兼容性 | 中 | 中 | 中 | 必须PTY、TERM=xterm-256color、窗口resize同步，重点测试vim/top | 后端+前端 |
| AI幻觉导致误导 | 中 | 中 | 中 | 强制结构化JSON、带证据、标注"仅供参考"、不自动执行、脱敏 | AI模块 |
| SQLite并发限制 | 中 | 中 | 中 | 审计异步写入、批量处理、预留Postgres切换路径 | 后端 |
| 过度设计延误 | 中 | 中 | 中 | 严格按P0优先级，MVP先行，配置开关控制新功能 | 项目负责人 |
| 权限配置错误暴露生产 | 极高 | 低 | 严重 | 默认最小权限、prod需显式授权、全量审计、上线前走查 | 运维+安全 |

## 14. 附录

### 14.1 环境变量清单
| 变量名 | 必需 | 说明 | 示例 |
|--------|------|------|------|
| PROMAI_JWT_SECRET | 是（生产） | JWT签名密钥，>=32位随机值 | openssl rand -base64 32 |
| TERMINAL_VAULT_KEY | 是（启用终端） | AES-256密钥，>=32字节 | openssl rand -hex 32 |
| PROMETHEUS_URL | 否 | Prometheus地址（可配置文件） | http://prometheus:9090 |
| PROMAI_DB_PATH | 否 | SQLite路径，默认promai.db | ./data/promai.db |

### 14.2 依赖新增建议
**后端（go.mod）：** `golang.org/x/crypto/ssh`（SSH客户端）  
**前端（package.json）：** `xterm`、`xterm-addon-fit`、`xterm-addon-web-links`、`xterm-addon-search`、`vis-network`（拓扑）、`md-editor-v3` 或 `vditor`（Runbook Markdown编辑）

### 14.3 目录结构建议
```
PromAI/
├── pkg/
│   ├── aiops/        # EventBus、Correlation等（或按功能拆分）
│   ├── eventhub/
│   ├── incident/
│   ├── correlation/
│   ├── topology/
│   ├── rca/
│   ├── runbook/
│   ├── oncall/
│   └── terminal/     # hub/ssh/session/policy/approval/recorder/share/audit/vault/ai/service
├── frontend/src/
│   ├── views/
│   │   ├── aiops/
│   │   ├── Terminal.vue
│   │   └── TerminalShare.vue
│   ├── components/terminal/
│   ├── api/aiops.ts, terminal.ts
│   ├── types/aiops.ts, terminal.ts
│   └── stores/terminal.ts
└── docs/
    ├── PromAI-AIOps-WebTerminal-设计开发文档.md（本文档）
    └── ...
```

### 14.4 注意事项
- **安全第一**：Web终端所有P0安全功能（白名单、加密、审批、审计）必须完成并验收通过后再考虑开放生产使用
- **渐进上线**：AIOps新功能通过配置开关控制（aiops.enabled、correlation.enabled等），默认可关闭
- **双写过渡**：告警桥接采用双写模式，确保不影响现有告警流程
- **AI可控**：所有AI功能均需人工触发，不自动执行高风险操作
- **审计不可篡改**：TerminalAudit表不提供更新删除API，严格遵守
- **最小权限**：生产环境主机优先用密钥认证，最小化allow_roles

---

**文档结束**

> 本文档基于对 PromAI 现有代码库的深入调研编写，结合 AIOps 平台演进需求和 AI-Chat Web终端核心能力需求，旨在指导后续设计评审、开发排期、测试验收等工作。
