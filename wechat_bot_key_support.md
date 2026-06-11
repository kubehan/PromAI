# 企业微信机器人Key参数支持

## 功能概述

现在支持通过 `wechat_bot_key` 参数动态传入企业微信机器人ID，无需修改配置文件即可发送通知到不同的企业微信群。

## 功能特性

### 1. 多种使用方式
- **直接API调用**：通过URL参数传入
- **首页巡检**：在首页模态框中输入
- **组合使用**：可与其他参数（如 `datasource`、`taskid`）组合使用

### 2. 智能优先级
- **URL参数优先**：如果传入 `wechat_bot_key` 参数，优先使用
- **配置文件备用**：如果未传入参数，使用配置文件中的机器人
- **可选功能**：不传入参数时不发送企业微信通知

## 使用方法

### 方式一：直接API调用

```bash
# 基本使用
http://localhost:8091/api/promai/getreport?wechat_bot_key=你的机器人key

# 结合数据源使用
http://localhost:8091/api/promai/getreport?datasource=bruneihealth&wechat_bot_key=你的机器人key

# 完整参数示例
http://localhost:8091/api/promai/getreport?datasource=bruneihealth&taskid=test-123&wechat_bot_key=你的机器人key
```

### 方式二：首页立即巡检

1. 访问首页：`http://localhost:8091/api/promai/`
2. 点击"开始巡检"卡片
3. 在弹出的模态框中：
   - 选择数据源（可选）
   - 输入企业微信机器人key（可选）
4. 点击"开始巡检"

### 方式三：触发巡检时传入

```bash
# 触发巡检时传入企业微信机器人参数
http://localhost:8091/api/promai/getreport?wechat_bot_key=你的机器人key
```

## 参数说明

| 参数 | 类型 | 必需 | 说明 |
|------|------|------|------|
| `wechat_bot_key` | string | 否 | 企业微信机器人key（不需要完整URL） |
| `datasource` | string | 否 | 数据源名称 |
| `taskid` | string | 否 | 任务ID |

## 实现细节

### 后端修改

1. **新增函数**：`SendWeChatWorkWithWebhook()`
   - 支持动态传入机器人key
   - 自动构建完整的webhook URL
   - 支持代理配置

2. **修改通知逻辑**：`sendNotificationsWithContext()`
   - 检查URL中的 `wechat_bot_key` 参数
   - 动态调用企业微信通知

### 前端修改

1. **首页模态框**：添加企业微信机器人key输入框
2. **JavaScript函数**：`startInspection()` 支持机器人key参数
3. **URL构建**：自动将参数添加到巡检请求中

## 示例

### 完整工作流程

1. **用户访问首页**
   ```
   http://localhost:8091/api/promai/
   ```

2. **点击"开始巡检"**
   - 选择数据源：`bruneihealth`
   - 输入机器人key：`0c4ef257-a6f0-430c-b454-d2b82bbd6e53`

3. **系统构建请求URL**
   ```
   /api/promai/getreport?datasource=bruneihealth&taskid=xyz-123&wechat_bot_key=0c4ef257-a6f0-430c-b454-d2b82bbd6e53
   ```

4. **执行巡检并通知**
   - 收集指标数据
   - 生成报告
   - 发送企业微信通知到指定的机器人

### 错误处理

- **无效机器人key**：通知会发送失败，并在日志中显示错误
- **无网络连接**：通知会失败，但不影响报告生成
- **参数为空**：跳过企业微信通知，继续其他通知方式

## 注意事项

1. **机器人key格式**：只需要传入key部分，如 `0c4ef257-a6f0-430c-b454-d2b82bbd6e53`
2. **URL编码**：如果key包含特殊字符，需要URL编码
3. **权限验证**：确保机器人key有发送消息的权限
4. **频率限制**：注意企业微信的发送频率限制

## 兼容性

- **向后兼容**：不影响现有的配置文件方式
- **优先级机制**：URL参数优先于配置文件
- **可选功能**：不传入参数时功能正常