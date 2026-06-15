# AI Agent SDK 使用文档

本文档介绍如何使用 Go SDK 创建和运行 AI 代理会话。

## 文档索引

| 文档                              | 描述                               |
| --------------------------------- | ---------------------------------- |
| [01-快速开始](01-快速开始.md)     | 最简单的使用方式，使用所有默认配置 |
| [02-模型选择](02-模型选择.md)     | 选择模型和思考级别                 |
| [03-系统提示词](03-系统提示词.md) | 替换或修改系统提示词               |
| [04-技能管理](04-技能管理.md)     | 发现、过滤、合并技能               |
| [05-会话管理](05-会话管理.md)     | 内存会话、持久化会话、继续会话     |
| [06-完整控制](06-完整控制.md)     | 完全自定义所有配置，不使用自动发现 |

## 快速参考

### 基础导入

```go
import "github.com/jay-y/pi/pkg/ai-agent-session"
```

### 最小化使用

```go
// 创建默认会话
agentSession, err := session.NewAgentSession(&session.AgentSessionConfig{})

// 订阅事件
agentSession.Subscribe(func(event session.AgentSessionEvent) {
    if event.Type == session.EventTypeMessageUpdate {
        if msgEvent, ok := event.AssistantMessageEvent.(*session.TextDeltaEvent); ok {
            fmt.Print(msgEvent.Delta)
        }
    }
})

// 发送提示
err = agentSession.Prompt(ctx, "你好！")
```

### 模型选择

```go
// 创建模型注册表
modelRegistry := session.NewModelRegistry()

// 查找模型
model := modelRegistry.Find("anthropic", "claude-opus-4-5")

// 创建会话
agentSession, err := session.NewAgentSession(&session.AgentSessionConfig{
    Model:         model,
    ThinkingLevel: session.ThinkingLevelMedium,
    ModelRegistry: modelRegistry,
})
```

### 自定义系统提示词

```go
resourceLoader := session.NewDefaultResourceLoader(session.DefaultResourceLoaderOptions{
    SystemPromptOverride: func(base string) string {
        return "你是一个专业的代码审查助手。"
    },
})
resourceLoader.Reload()

agentSession, err := session.NewAgentSession(&session.AgentSessionConfig{
    ResourceLoader: resourceLoader,
})
```

### 会话管理

```go
// 内存会话（不持久化）
sessionManager := session.NewInMemorySessionManager()

// 新建持久化会话
sessionManager := session.NewFileSessionManager("/path/to/project")

// 继续最近会话
sessionManager := session.NewContinueRecentSessionManager("/path/to/project")

// 打开特定会话
sessionManager := session.NewOpenSessionManager("/path/to/session.jsonl")

// 创建会话
agentSession, err := session.NewAgentSession(&session.AgentSessionConfig{
    SessionManager: sessionManager,
})
```

### 技能管理

```go
// 加载所有技能
result := session.LoadSkills(session.LoadSkillsOptions{})

// 过滤技能
resourceLoader := session.NewDefaultResourceLoader(session.DefaultResourceLoaderOptions{
    SkillsOverride: func(current session.SkillsResult) session.SkillsResult {
        filtered := []session.SkillInfo{}
        for _, skill := range current.Skills {
            if strings.Contains(skill.Name, "code") {
                filtered = append(filtered, skill)
            }
        }
        return session.SkillsResult{Skills: filtered}
    },
})
```

### 完整控制

```go
// 完全自定义所有组件
agentSession, err := session.NewAgentSession(&session.AgentSessionConfig{
    Cwd:             "/path/to/project",
    AgentDir:        "/tmp/my-agent",
    Model:           model,
    ThinkingLevel:   session.ThinkingLevelOff,
    AuthStorage:     authStorage,
    ModelRegistry:   modelRegistry,
    ResourceLoader:  customResourceLoader,
    Tools:           customTools,
    SessionManager:  sessionManager,
    SettingsManager: settingsManager,
})
```

## 核心概念

### 会话 (Session)

会话是代理与用户交互的上下文，包含消息历史、设置和状态。

### 资源加载器 (ResourceLoader)

负责加载系统提示词、技能、扩展、主题等资源。

### 模型注册表 (ModelRegistry)

管理内置模型和自定义模型，处理 API 密钥解析。

### 设置管理器 (SettingsManager)

管理全局和项目级别的设置，包括压缩、重试、终端配置等。

### 会话管理器 (SessionManager)

控制会话的持久化方式：内存、新建文件、继续最近、打开特定。

## 目录结构

SDK 会自动从以下目录加载资源：

```
~/.pi/agent/              # 全局配置
├── skills/               # 全局技能
├── extensions/           # 全局扩展
├── SYSTEM.md            # 全局系统提示词
├── APPEND_SYSTEM.md     # 全局追加提示词
└── models.json          # 自定义模型

<project>/.pi/           # 项目配置
├── skills/              # 项目技能
├── extensions/          # 项目扩展
├── SYSTEM.md           # 项目系统提示词
├── APPEND_SYSTEM.md    # 项目追加提示词
└── AGENTS.md           # 项目上下文
```

## 事件类型

```go
const (
    EventTypeMessageUpdate      // 消息更新
    EventTypeToolStart          // 工具开始执行
    EventTypeToolComplete       // 工具执行完成
    EventTypeToolError          // 工具执行错误
    EventTypeSessionCreated     // 会话创建
    EventTypeMessageAdded       // 消息添加
    EventTypeCompact            // 会话压缩
    EventTypeBranchCreated      // 分支创建
)
```

## 思考级别

```go
const (
    ThinkingLevelOff    = "off"    // 不思考
    ThinkingLevelLow    = "low"    // 少量思考
    ThinkingLevelMedium = "medium" // 中等思考
    ThinkingLevelHigh   = "high"   // 深度思考
)
```

## 更多信息

查看各个详细文档了解更多信息：

- [01-快速开始](01-快速开始.md) - 开始使用 SDK
- [02-模型选择](02-模型选择.md) - 配置模型和 API 密钥
- [03-系统提示词](03-系统提示词.md) - 自定义提示词
- [04-技能管理](04-技能管理.md) - 管理技能
- [05-会话管理](05-会话管理.md) - 控制会话持久化
- [06-完整控制](06-完整控制.md) - 完全自定义配置
