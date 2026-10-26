# PromAI 使用说明文档

PromAI 是一个基于 Prometheus 的智能监控报告生成与巡检系统。它能够自动收集 Prometheus 指标，分析系统健康状态，生成可视化的 HTML 报告，并通过多种渠道（钉钉、企业微信、邮件）发送告警通知。

## 目录

- [功能特性](#功能特性)
- [快速开始](#快速开始)
  - [源码编译](#源码编译)
  - [Docker 部署](#docker-部署)
  - [Kubernetes 部署](#kubernetes-部署)
- [配置指南](#配置指南)
  - [基础配置](#基础配置)
  - [多数据源配置](#多数据源配置)
  - [通知配置](#通知配置)
  - [指标与阈值配置](#指标与阈值配置)
- [使用指南](#使用指南)
  - [Web 界面](#web-界面)
  - [API 接口](#api-接口)
  - [AI Skills（技能管理）](#ai-skills技能管理)
  - [定时任务](#定时任务)
- [常见问题](#常见问题)

## 功能特性

- **多维度监控**：支持基础设施、Kubernetes、中间件（MySQL, Redis, MongoDB等）、应用服务等多层级监控。
- **智能巡检**：自动分析指标趋势，基于阈值（大于、小于、等于、范围等）判断健康状态。
- **可视化报告**：生成包含图表、统计数据和详细列表的 HTML 巡检报告。
- **多数据源支持**：可配置多个 Prometheus 集群，并在界面上动态切换。
- **实时看板**：提供 `/status` 页面实时查看服务健康状态。
- **多渠道通知**：支持钉钉、企业微信、邮件通知，报告生成后自动推送。

## 快速开始

### 源码编译

**前置要求**：Go 1.22+

1. **克隆代码**

   ```bash
   git clone https://github.com/kubehan/PromAI.git
   cd PromAI
   ```
2. **编译项目**

   ```bash
   go mod download
   go build -o promai .
   ```
3. **运行**

   ```bash
   # 直接使用现有的 config/config.yaml，按需修改配置

   # 环境变量配置 EXTERNAL_PORT 指定反向代理端口

   ./promai -config config/config.yaml -port :8091
   ```

### Docker 部署

```bash
docker run -d \
  --name promai \
  -p 8091:8091 \
  -v $(pwd)/config/config.yaml:/app/config/config.yaml \
  -v $(pwd)/reports:/app/reports \
  kubehan/promai:latest
```

### Kubernetes 部署

使用提供的部署文件：

```bash
kubectl create namespace promai
kubectl create configmap config --from-file=config/config.yaml -n promai
kubectl apply -f deploy/deployment.yaml
```

## 配置指南

配置文件位于 `config/config.yaml`，采用 YAML 格式。

### 基础配置

```yaml
# 默认 Prometheus 地址
prometheus_url: "http://prometheus-k8s.monitoring.svc.cluster.local:9090"
prometheus_username: ""       # 可选
prometheus_password: ""       # 可选

# 项目名称，显示在报告中
project_name: "巡检报告"

# 管理后台认证（首次启动时同步到 SQLite）
auth:
  username: "admin"
  password: "admin123"
  # JWT 签名密钥必须通过环境变量 PROMAI_JWT_SECRET 提供，不要写入配置文件。

# 定时巡检任务 (Cron 表达式)
# 示例：每天 08:00 和 17:00 执行
cron_schedule: "00 08,17 * * *"

# 报告清理策略
report_cleanup:
  enabled: true
  max_age: 7    # 保留最近 7 天的报告
  cron_schedule: "0 0 * * *"  # 每天凌晨清理
```

> **安全要求**：`PROMAI_JWT_SECRET` 必须是至少 32 位的唯一随机值，建议使用
> `openssl rand -base64 32` 生成，并通过 Secret Manager / Kubernetes Secret 注入。
> 未配置、使用公开默认值或长度不足时，程序会拒绝启动。旋转该密钥会使已签发的登录令牌失效，
> 并导致已加密保存的 AI API Key 无法解密，需要重新配置。

### 多数据源配置

如果需要监控多个 Prometheus 集群，可以配置 `data_sources`：

```yaml
data_sources:
  - name: "cluster1"
    url: "http://prometheus.cluster1.example.com"
  - name: "cluster2"
    url: "http://prometheus.cluster2.example.com"
```

### 通知配置

支持钉钉、邮件、企业微信（机器人+应用）和飞书通知。

```yaml
notifications:
  dingtalk:
    enabled: true
    webhook: "https://oapi.dingtalk.com/robot/send?access_token=YOUR_TOKEN"
    secret: "YOUR_SECRET"
    report_url: "http://your-promai-host:8091" # 报告访问地址

  wechat_work:
    enabled: true
    webhook: "https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=YOUR_KEY"
    report_url: "http://your-promai-host:8091"

  wechat_app:
    enabled: false
    corpid: "wwaf792cfe24e35c78"
    agentid: 1000009
    secret: "YOUR_APP_SECRET"
    touser: "@all"
    proxy_url: ""
    report_url: "http://your-promai-host:8091"

  feishu:
    enabled: false
    webhook: "https://open.feishu.cn/open-apis/bot/v2/hook/YOUR_TOKEN"
    secret: ""
    report_url: "http://your-promai-host:8091"
    timeout: 5
    verify_sign: false

  email:
    enabled: false
    smtp_host: "smtp.exmail.qq.com"
    smtp_port: 465
    username: "demo@demo.cn"
    password: "xxxxxxxxxxxxxxxxxxxx"  # 授权码
    from: "demo@demo.cn"
    to:
      - "demo@demo.cn"
```

### 指标与阈值配置

在 `metric_types` 下定义监控指标。

```yaml
metric_types:
  - type: "基础资源"
    metrics:
      - name: "CPU使用率"
        description: "节点CPU使用率统计"
        # Prometheus 查询语句
        query: "100 - (avg by(instance) (irate(node_cpu_seconds_total{mode='idle'}[5m])) * 100)"
        # 阈值配置
        threshold: 80
        threshold_type: "greater" # greater(>), less(<), equal(==), not_equal(!=)
        threshold_status: "critical" # 触发阈值时的告警级别：critical / warning / normal
        unit: "%"
        labels:
          instance: "节点IP"
```

**特别注意**
资源使用概览部分必须按照现有配置文件中的进行定义，否则无法正常显示，因为报告模板里面定义了如下内容

```html
    const cpuData = getHostMetricValues('CPU性能状态监控');
    const memoryData = getHostMetricValues('内存性能状态监控');
    // 获取磁盘数据，只包含/home 挂载点
    const diskData = getDiskMetricValues('/home');
```

因此必须在配置文件中包含以下内容才能正常显示图表

```yaml
    metrics:
      - name: "CPU性能状态监控"
      - name: "内存性能状态监控"
      - name: "存储设备状态监控"
```

## 使用指南

### Web 界面

启动服务后，访问 `http://localhost:8091/api/promai` 进入首页。

- **首页**：查看系统概览和入口。
- **历史报告**：`http://localhost:8091/api/promai/reports/history`，查看所有生成的巡检报告。
- **实时状态**：`http://localhost:8091/api/promai/status`，查看当前各项指标的实时健康状态。
- **巡检进度**：`http://localhost:8091/api/promai/progress`，查看当前正在执行的巡检任务进度。

### API 接口

PromAI 提供了一系列 API 用于集成和自动化。

- **手动触发巡检并生成报告**：

  ```
  GET /api/promai/getreport
  ```

  参数：

  - `datasource` (可选): 指定数据源名称（如 `cluster1`）或完整的 Prometheus URL。
  - `wechat_bot_key` (可选): 指定企业微信机器人 Key，用于本次巡检结果通知。

  示例：

  ```
  1. 使用配置文件定义的数据源
  http://localhost:8091/api/promai/getreport?datasource=cluster1

  2. 动态指定数据源
  http://localhost:8091/api/promai/getreport?datasource=http://prometheus.cluster1.example.com
  ```
- **获取报告列表**：

  ```
  GET /api/promai/reports/list
  ```
- **查看实时状态**：

  ```
  GET /api/promai/status?datasource=cluster1
  ```

### AI Skills（技能管理）

Skills 是基于 OpenClaw SKILL.md 规范的工作流指令包。它们不直接执行代码，而是指导 AI 如何组合使用已有工具（`query_metrics`、`exec`、`analyze_alert`、`push_report` 等）来完成监控场景任务。

#### 管理页面

在左侧导航栏 **AI 助手 → Skills** 进入技能管理页面。

- **新建**：点右上角 `+`，填写 Name / Description / Instruction / Metadata，保存后自动生效
- **编辑**：在左侧列表选中已有 Skill，修改后保存
- **启用/禁用**：点击顶部的启用/禁用按钮控制，禁用的 Skill 不会被注入 AI 系统提示词
- **删除**：选中 Skill 后点"删除"按钮

#### 内置 Skills

| 名称 | 描述 |
|------|------|
| `disk-check` | 磁盘使用率检查，结合 `query_metrics` 和 `exec` 定位大目录 |
| `resource-analyze` | CPU/内存/负载/Swap 综合分析 |
| `alert-root-cause` | 告警根因分析，关联指标和巡检记录 |
| `inspect-report` | 触发巡检 → 轮询任务 → 推送报告到通知渠道 |
| `node-health` | 一键节点健康检查（CPU/内存/磁盘/网络/运行时间） |
| `datasource-check` | 检查数据源连通性、数据延迟和 Target 在线率 |

#### 文件结构

每个 Skill 对应 `skills/<名称>/SKILL.md` 文件，存储为 YAML frontmatter + Markdown 格式：

```markdown
---
name: disk-check
description: 检查磁盘使用率并给出处理建议
user-invocable: true
x-enabled: true
metadata: {}
---

# 指令内容

指导 AI 如何使用工具的 Markdown 正文。
```

可直接在文件系统中创建/编辑，但需要通过 Admin API 触发热加载生效。

#### SKILL.md 参考

| 字段 | 说明 |
|------|------|
| `name` | 唯一标识符（小写字母、数字、连字符） |
| `description` | 单行描述（≤ 160 字符） |
| `user-invocable` | 是否可作为 slash 命令（默认 true） |
| `x-enabled` | 启用/禁用（默认 true，UI 切换此值） |
| `metadata` | JSON 对象，用于 gating 条件（`requires.bins`、`requires.env`、`os` 等） |

Instruction 正文中可引用平台内置工具：`query_metrics`、`analyze_alert`、`list_datasources`、`trigger_inspect`、`query_task`、`list_reports`、`get_report_detail`、`push_report`、`exec`。

## 常见问题

**Q: 如何添加新的监控指标？**
A: 修改 `config/config.yaml`，在 `metric_types` 中添加新的 `metrics` 项。你需要提供有效的 PromQL 查询语句和合理的阈值。

**Q: 报告中的资源使用概览图表没有数据？**
A: 请检查 `config.yaml` 中指标的 `metrics` 是否配置正确，以及 Prometheus 中是否有对应的历史数据。`name` 需要配置为

```yaml
    metrics:
      - name: "CPU性能状态监控"
      - name: "内存性能状态监控"
      - name: "存储设备状态监控" #需要有home盘
```

**Q: 如何切换查看不同集群的报告？**
A: 在 Web 界面或 API 调用时，使用 `datasource` 参数指定在 `config.yaml` 中配置的数据源名称。
