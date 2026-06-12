# 企业微信应用通知全功能指南

## 1. 功能概览

PromAI 企业微信应用通知功能现已全面升级，支持以下核心特性：

- ✅ **定向推送**：支持发送给全员 (`@all`) 或指定个人/多个用户。
- ✅ **邮箱自动换算**：支持直接使用企业邮箱发送，系统自动换算为 `UserID`。
- ✅ **动态参数覆盖**：支持在 API 调用时通过 URL 参数实时覆盖配置文件设置。
- ✅ **智能图标状态**：针对巡检结果显示不同的状态图标（包含零指标特殊提示）。
- ✅ **高效缓存**：UserID 换算结果缓存 1 小时，兼顾性能与实时性。

---

## 2. 快速配置

编辑 `config/config.yaml`，配置 `wechat_app` 字段：

```yaml
notifications:
  wechat_app:
    enabled: true  # 开启通知
    corpid: "wwaf792cfe24e35c78"  # 企业ID
    agentid: 1000009  # 应用 AgentID
    secret: "SqxOKWYo03W2I9yL2IBoQgTZxlzCzh5RNAhRDyJMhfY"  # 应用 Secret
    touser: "kubehan@kubehan.cn"  # 接收人（支持邮箱/UserID/@all）
    proxy_url: ""  # 可选代理地址
    report_url: "https://alert.kubehan.cn"  # 报告 base url
```

---

## 3. 接收人设置 (touser)

`touser` 字段非常灵活，支持多种格式，且多个接收人之间用 `|` 分隔。

### 方式 A：使用企业邮箱（推荐 ⭐）
直接使用工作邮箱名。
- **配置示例**：`kubehan@kubehan.cn`
- **处理逻辑**：系统会自动识别 `@` 并调用企业微信官方 API 转换为内部 `UserID`。
- **前提条件**：用户在企业微信后台通讯录中必须**设置了邮箱字段**。

### 方式 B：使用 UserID
直接使用企业微信账号（UserID）。
- **配置示例**：`kubehan` 或 `kubehan|ZhangSan`
- **处理逻辑**：系统直接透传，性能最高。

### 方式 C：发送至全员
- **配置示例**：`@all`

### 方式 D：通过首页 UI 输入（新增 ⭐）
在首页点击“立即巡检”后：
1. 在弹出的模态框中可以看到“**巡检人员企业微信邮箱**”输入框。
2. 输入您的企业邮箱（如 `kubehan@kubehan.cn`）或 `UserID`。
3. 点击“开始巡检”，系统将自动将报告推送给您。

---

## 4. 动态调用参数

在手动触发巡检或通过 Webhook 触发时，支持通过 URL 参数强制覆盖配置：

```bash
# 指定接收人为 kubehan (会自动进行邮箱处理或 UserID 识别)
curl "http://localhost:8091/api/promai/getreport?touser=kubehan@kubehan.cn"

# 指定多个人
curl "http://localhost:8091/api/promai/getreport?touser=user1@company.com|user2@company.com"

# 同时指定数据源和接收人
curl "http://localhost:8091/api/promai/getreport?datasource=cluster1&touser=kubehan"
```

---

## 5. 状态图标说明

通知消息中，分类巡检结果前面的图标代表不同的含义：

| 图标 | 含义         | 触发条件                         |
| :--- | :----------- | :------------------------------- |
| ❌    | **严重异常** | 该分类下有 Critical 级别的告警   |
| ⚠️    | **警告异常** | 该分类下有 Warning 级别的告警    |
| ✅    | **全部正常** | 该分类下所有指标均正常且总数 > 0 |
| ⚪    | **暂无数据** | **该分类下指标总数为 0**         |

---

## 6. 常见问题排查 (Troubleshooting)

### Q1: 提示 "获取 UserID 失败(errcode=40058): missing field `email`"
- **原因**：请求格式不符合企业微信 API 规范（已在最新版本中修复，改用 POST JSON 请求）。
- **解决**：确保使用的是最新编译版本。

### Q2: 提示 "未找到邮箱对应的用户"
- **分析**：日志中可能看到 `从企业微信获取到 0 个用户` 或 `Email= `（空）。
- **解决**：
  1. 登录企业微信管理后台 -> 通讯录 -> 编辑对应成员。
  2. 确保 **"邮箱"** 字段已正确填写。
  3. 如果不想设置邮箱，可以直接在配置中使用 **"账号"** 字段对应的 `UserID`。

---

## 7. 开发人员说明

### 缓存机制
UserID 换算结果缓存 1 小时。如需立即刷新，请重启服务。  
换算逻辑位于 `pkg/notify/notify.go` 的 `convertEmailToUserID` 函数中。
