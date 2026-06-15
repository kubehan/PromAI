# pi - AI Agent SDK

一个模块化的 AI Agent SDK，提供统一的 LLM API 抽象层、有状态的 Agent 引擎、会话管理和内置工具集。

> 主要学习参考 [pi-mono](https://github.com/badlogic/pi-mono)

## 特性

- **统一 LLM API** - 支持 OpenAI、Anthropic、Google、Ollama 等多个提供者
- **有状态 Agent** - 支持工具调用、事件流、中断控制（Steering/Follow-up）
- **会话管理** - 内存会话、持久化会话、会话恢复、自动压缩和重试
- **内置工具** - 文件读写、Bash 执行、代码搜索、文件查找等
- **思考/推理** - 支持模型思考过程的统一接口（Minimal/Low/Medium/High/XHigh 级别）
- **模型管理** - 多模型支持、模型循环切换、API Key 管理
- **扩展系统** - 支持工具扩展、自定义运行时环境

## 项目结构

```
pi/
├── cmd/                        # 命令行程序入口
│   └── ai/
│       ├── main.go             # 主入口，演示基础 LLM 调用
│       ├── agent/              # Agent 演示程序
│       │   ├── main.go         # Agent 核心功能演示（Steering/Follow-up）
│       │   └── session/        # 会话相关演示
│       │       └── main.go     # AgentSession 会话管理演示
│       └── session/            # 会话程序入口
├── pkg/
│   ├── ai/                     # 统一 LLM API 抽象层
│   │   ├── api_provider_openai.go  # OpenAI API 实现
│   │   ├── api_registry.go     # API 提供者注册表
│   │   ├── constants.go        # 常量定义（API、Provider、Thinking Level、StopReason等）
│   │   ├── events.go           # Assistant 消息事件类型定义
│   │   ├── event_stream.go     # 通用事件流处理
│   │   ├── messages.go         # 消息类型定义（UserMessage, AssistantMessage, ToolResultMessage）
│   │   ├── model.go            # 模型接口和实现
│   │   ├── stream.go           # 流式和非流式调用接口
│   │   ├── tool_call.go        # 工具调用数据结构
│   │   ├── types.go            # 通用类型定义（ThinkingBudgets, Cost, Usage等）
│   │   └── utils.go            # 工具函数（API密钥获取、成本计算等）
│   ├── ai-agent/               # 有状态 Agent 引擎
│   │   ├── agent.go            # Agent 核心实现
│   │   ├── agent_events.go     # Agent 事件类型定义
│   │   ├── agent_tool.go       # Agent 工具接口定义
│   │   ├── agent_loop.go       # 代理循环逻辑（Steering/Follow-up处理）
│   │   ├── constants.go        # Agent 常量定义
│   │   └── proxy.go            # 代理服务事件转换工具
│   ├── ai-agent-session/       # 会话管理模块
│   │   ├── session.go          # AgentSession 核心实现
│   │   ├── session_events.go   # 会话事件类型定义
│   │   ├── session_model.go    # 模型切换和思考级别管理
│   │   ├── session_manager.go  # 会话管理器（持久化/恢复）
│   │   ├── settings_manager.go # 设置管理器（全局/项目配置）
│   │   ├── session_prompt.go   # 提示词管理
│   │   ├── session_tool.go     # 工具扩展管理
│   │   ├── model_registry.go   # 模型注册表和 API Key 管理
│   │   ├── resource_loader.go  # 资源加载器
│   │   ├── utils.go            # 工具函数
│   │   ├── constants.go        # 会话常量定义
│   │   ├── session_compaction.go  # 会话压缩
│   │   └── session_retry.go    # 会话重试机制
│   └── ai-agent-tools/         # 内置工具集
│       ├── tools.go            # 工具创建工厂函数
│       ├── utils.go            # 工具工具函数（文件截断、路径解析等）
│       ├── tools_read.go       # 文件读取工具（支持图片）
│       ├── tools_write.go      # 文件写入工具
│       ├── tools_edit.go       # 文件编辑工具（基于 Diff）
│       ├── tools_bash.go       # Bash 命令执行工具
│       ├── tools_grep.go       # 文件内容搜索工具（基于 ripgrep）
│       ├── tools_find.go       # 文件查找工具
│       └── tools_ls.go         # 目录列出工具
├── docs/                       # 详细文档
│   ├── 01-快速开始.md
│   ├── 02-模型选择.md
│   ├── 03-系统提示词.md
│   ├── 04-技能管理.md
│   ├── 05-会话管理.md
│   └── 06-完整控制.md
├── configs/                    # 配置文件
├── scripts/                    # 构建脚本
├── test/                       # 测试数据
└── pkg/ai-agent-session/       # 会话相关工具和扩展支持
```

## 核心概念

### Agent 与 Session 的区别

- **Agent**: 更底层的接口，提供对推理循环的完全控制，适合需要精细控制的场景
- **Session**: 基于 Agent 的高级接口，提供会话管理、自动压缩、重试等高级功能，适合大多数应用场景

### 事件驱动架构

整个 SDK 采用事件驱动架构，所有交互都通过事件进行：

1. **LLM 调用事件**: 助手消息生成过程中的各个阶段
2. **Agent 事件**: 代理执行过程中的各种状态变化
3. **Session 事件**: 会话生命周期中的各种事件

这种架构使得 UI 和业务逻辑可以轻松分离，便于扩展和测试。

### 工具执行模式

工具通过 Agent 自动执行，遵循以下模式：

1. Agent 接收到需要工具调用的响应
2. 解析工具调用信息
3. 执行对应的工具
4. 将结果作为新消息发送给模型
5. 模型继续生成响应（如果有更多工具调用则重复）

这种模式支持复杂的多步骤任务自动执行。

### 思考级别

SDK 支持多种思考级别，不同模型支持的级别不同：

- `Off`: 关闭思考
- `Minimal`: 最小思考
- `Low`: 低思考
- `Medium`: 中等思考
- `High`: 高思考
- `XHigh`: 超高思考（仅特定模型支持）

思考级别越高，模型推理时间越长，但结果质量可能更高。

### pkg/ai - 统一 LLM API

提供统一的 LLM 调用接口，支持多个提供者：

```go
import "github.com/jay-y/pi/pkg/ai"

// 注册所有内置 API 提供者
ai.RegisterBuiltInApiProviders()

// 创建模型
model := &ai.BaseModel{
    ID:            "gpt-4o-mini",
    Name:          "openai/gpt-4o-mini",
    API:           ai.ModelApi(ai.ApiOpenAICompletions),
    Provider:      ai.ModelProvider("openai"),
    BaseURL:       "https://api.openai.com/v1",
    Reasoning:     false,
    Input:         []string{"text"},
    Cost:          ai.ModelCost{},
    ContextWindow: 128000,
    MaxTokens:     16384,
}

// 构建对话上下文
ctx := ai.Context{
    SystemPrompt: "You are a helpful assistant.",
    Messages:     []ai.Message{ai.NewUserMessage("Hello!")},
}

// 流式调用
stream, _ := ai.Stream(model, ctx, &ai.ProviderStreamOptions{
    StreamOptions: ai.StreamOptions{
        Ctx:    context.Background(),
        APIKey: "your-api-key",
    },
})

for event := range stream.Events() {
    if e, ok := event.(*ai.AssistantMessageEventTextDelta); ok {
        fmt.Print(e.Delta)
    }
}
```

**支持的提供者**: OpenAI, Azure OpenAI, Anthropic, Google, Vertex AI, Ollama, Mock（模拟）

主要接口：

- `Model`: 模型接口，提供模型元数据（ID, Name, API, Provider, BaseURL等）
- `ApiProvider`: API 提供者接口，支持流式和非流式调用
- `Context`: 对话上下文，包含系统提示词、消息列表和工具列表
- `StreamOptions`: 流式调用选项（温度、token限制、API密钥、传输协议等）
- `SimpleStreamOptions`: 简单流式调用选项（带思考级别）
- `AssistantMessageEventStream`: 助手消息事件流

**事件类型**:

- `AssistantMessageEventStart`: 消息流开始
- `AssistantMessageEventTextStart/TextDelta/TextEnd`: 文本内容事件
- `AssistantMessageEventThinkingStart/ThinkingDelta/ThinkingEnd`: 思考内容事件
- `AssistantMessageEventToolCallStart/ToolCallDelta/ToolCallEnd`: 工具调用事件
- `AssistantMessageEventDone`: 消息流完成
- `AssistantMessageEventError`: 消息流错误

**消息类型**:

- `UserMessage`: 用户消息
- `AssistantMessage`: 助手消息（支持文本、思考、工具调用等多模态内容）
- `ToolResultMessage`: 工具结果消息
- `ContentBlock`: 内容块接口（TextContentBlock, ThinkingContentBlock, ImageContentBlock, ToolCall）

### pkg/ai-agent - Agent 引擎

有状态的 Agent，支持工具调用和事件流：

```go
import agent "github.com/jay-y/pi/pkg/ai-agent"

// 创建 Agent
ag := agent.NewAgent(agent.AgentOptions{
    InitialState: &agent.AgentState{
        SystemPrompt: "You are a helpful assistant.",
        Model:        model,
    },
})

// 订阅事件
unsub := ag.Subscribe(func(event agent.AgentEvent) {
    switch e := event.(type) {
    case *agent.AgentEventMessageUpdate:
        if delta, ok := e.AssistantMessageEvent.(*ai.AssistantMessageEventTextDelta); ok {
            fmt.Print(delta.Delta)
        }
    case *agent.AgentEventToolExecutionStart:
        fmt.Printf("Tool: %s\n", e.ToolName)
    }
})
defer unsub()

// 发送提示
ag.Prompt(ctx, "Hello!")

// Steering: 中途更改指令
ag.Steer(ai.NewUserMessage("停下！换个任务"))

// Follow-up: 任务完成后继续
ag.FollowUp(ai.NewUserMessage("添加错误处理"))
```

主要事件类型：

- `AgentEventStart`: Agent 开始
- `AgentEventEnd`: Agent 结束
- `AgentEventTurnStart/TurnEnd`: 轮次开始/结束
- `AgentEventMessageStart/Update/End`: 消息开始/更新/结束
- `AgentEventToolExecutionStart/Update/End`: 工具执行开始/更新/结束

支持的功能：

- **思考级别控制**: Minimal/Low/Medium/High/XHigh
- **消息队列**: Steering（中断）和 Follow-up（后续）消息队列
- **工作流管理**: 自动工具调用循环、错误处理
- **配置选项**: 自定义 LLM 转换、上下文变换、API Key 获取等

### pkg/ai-agent-session - 会话管理

完整的会话生命周期管理：

```go
import "github.com/jay-y/pi/pkg/ai-agent-session"

// 创建 Agent
agent := agent.NewAgent(agent.AgentOptions{
    InitialState: &agent.AgentState{
        SystemPrompt: "You are a helpful assistant.",
        Model:        model,
    },
})

// 创建会话
agentSession := session.NewAgentSession(&session.AgentSessionConfig{
    Agent:         agent,
    ModelRegistry: &session.ModelRegistry{},
    SessionManager: session.InMemorySessionManager(cwd),
    SettingsManager: &session.SettingsManager{},
})

// 订阅事件
agentSession.Subscribe(func(event session.AgentSessionEvent) {
    if event.GetType() == session.EventTypeMessageUpdate {
        if msgEvent, ok := event.AssistantMessageEvent.(*session.TextDeltaEvent); ok {
            fmt.Print(msgEvent.Delta)
        }
    }
})

// 发送提示
agentSession.Prompt(ctx, "帮我分析这个项目")

// Follow-up 消息
agentSession.FollowUp(ctx, "添加更多细节")

// 模型切换
agentSession.CycleModel(ctx, "forward")

// 思考级别切换
agentSession.CycleThinkingLevel()
```

主要功能：

- 会话持久化（JSONL 格式）
- 会话统计（消息数、token 数、成本）
- 会话管理（列出、切换、删除、压缩）
- 设置管理（模型、思考级别、工具等）
- 自动压缩（超过 token 限制自动压缩）
- 自动重试（网络错误自动重试）
- 模型循环切换
- 思考级别自动适配

**事件类型**:

- `AgentSessionEvent`: 基础会话事件接口
- `AutoCompactionStart/EndEvent`: 自动压缩开始/结束
- `AutoRetryStart/EndEvent`: 自动重试开始/结束
- `SessionSwitchEvent`: 会话切换事件
- `ModelSelectEvent`: 模型选择事件

### pkg/ai-agent-tools - 内置工具

提供文件操作、Bash 执行等工具：

```go
import "github.com/jay-y/pi/pkg/ai-agent-tools"

cwd, _ := os.Getwd()

// 创建编码工具（读写编辑Bash）
codingTools := tools.CreateCodingTools(cwd)

// 创建只读工具（读取搜索查找列出）
readOnlyTools := tools.CreateReadOnlyTools(cwd)

// 创建所有工具
allTools := tools.CreateAllTools(cwd)
```

支持的工具：

- `read`: 读取文件（支持文本和图片，支持偏移和限制读取）
- `write`: 写入文件（原子写入，自动创建目录）
- `edit`: 替换文件中的文本（基于 Diff，自动换行符处理）
- `bash`: 执行 Bash 命令（超时控制，输出截断，支持临时文件）
- `grep`: 搜索文件内容（基于 ripgrep，支持上下文、限制、glob过滤）
- `find`: 查找文件（支持按名称和类型搜索）
- `ls`: 列出目录内容（排序输出，支持 limit）

所有工具提供：

- 参数验证（JSON Schema 验证）
- 错误处理
- 输出截断（行数和字节数限制）
- 详细的工具结果（包括截断信息、路径信息等）
- 上下文支持（可取消操作）

## 快速开始

### 安装

```bash
go get github.com/jay-y/pi
```

### 最小示例

```go
package main

import (
    "context"
    "fmt"

    "github.com/jay-y/pi/pkg/ai"
)

func main() {
    ctx := context.Background()

    // 注册内置 API 提供者
    ai.RegisterBuiltInApiProviders()

    // 创建模型
    model := &ai.BaseModel{
        ID:            "gpt-4o-mini",
        Name:          "openai/gpt-4o-mini",
        API:           ai.ModelApi(ai.ApiOpenAICompletions),
        Provider:      ai.ModelProvider("openai"),
        BaseURL:       "https://api.openai.com/v1",
        Reasoning:     false,
        Input:         []string{"text"},
        Cost:          ai.ModelCost{},
        ContextWindow: 128000,
        MaxTokens:     16384,
    }

    // 构建对话上下文
    ctxData := ai.Context{
        SystemPrompt: "You are a helpful assistant.",
        Messages: []ai.Message{
            ai.NewUserMessage("你好！"),
        },
    }

    // 流式调用
    stream, err := ai.Stream(model, ctxData, &ai.ProviderStreamOptions{
        StreamOptions: ai.StreamOptions{
            Ctx:    ctx,
            APIKey: "your-api-key",
        },
    })
    if err != nil {
        fmt.Printf("Error: %v\n", err)
        return
    }

    // 处理事件流
    for event := range stream.Events() {
        switch e := event.(type) {
        case *ai.AssistantMessageEventTextDelta:
            fmt.Print(e.Delta)
        case *ai.AssistantMessageEventDone:
            fmt.Printf("\nTotal tokens: %d\n", e.Message.Usage.TotalTokens)
        }
    }

    // 完整调用（非流式）
    response, err := ai.Complete(model, ctxData, &ai.ProviderStreamOptions{
        StreamOptions: ai.StreamOptions{
            Ctx:    ctx,
            APIKey: "your-api-key",
        },
    })
    if err != nil {
        fmt.Printf("Error: %v\n", err)
        return
    }

    for _, block := range response.Content {
        if t, ok := block.(*ai.TextContentBlock); ok {
            fmt.Println(t.Text)
        }
    }
}
```

### 使用 Ollama 模型

```go
package main

import (
    "context"
    "fmt"

    "github.com/jay-y/pi/pkg/ai"
)

func main() {
    // 注册内置 API 提供者
    ai.RegisterBuiltInApiProviders()

    // 创建 Ollama 模型
    ollamaModel := &ai.BaseModel{
        ID:            "qwen3-coder-next:q8_0",
        Name:          "ollama/qwen3-coder-next:q8_0",
        API:           ai.ModelApi(ai.ApiOpenAICompletions),
        Provider:      ai.ModelProvider("ollama"),
        BaseURL:       "http://127.0.0.1:11434/v1",
        Reasoning:     false,
        Input:         []string{"text"},
        Cost:          ai.ModelCost{},
        ContextWindow: 128000,
        MaxTokens:     32000,
    }

    // 构建对话上下文
    ctxData := ai.Context{
        SystemPrompt: "You are a helpful assistant.",
        Messages: []ai.Message{
            ai.NewUserMessage("你好！"),
        },
    }

    // 流式调用
    stream, err := ai.Stream(ollamaModel, ctxData, &ai.ProviderStreamOptions{
        StreamOptions: ai.StreamOptions{
            Ctx:    context.Background(),
            APIKey: "ollama",
        },
    })
    if err != nil {
        fmt.Printf("Error: %v\n", err)
        return
    }

    for event := range stream.Events() {
        switch e := event.(type) {
        case *ai.AssistantMessageEventTextDelta:
            fmt.Print(e.Delta)
        }
    }
}
```

### 使用 Agent 和工具

```go
package main

import (
    "context"
    "fmt"
    "os"

    "github.com/jay-y/pi/pkg/ai"
    agent "github.com/jay-y/pi/pkg/ai-agent"
    tools "github.com/jay-y/pi/pkg/ai-agent-tools"
)

func main() {
    ctx := context.Background()

    // 注册 API 提供者
    ai.RegisterBuiltInApiProviders()

    // 创建模型
    model := &ai.BaseModel{
        ID:            "gpt-4o-mini",
        Name:          "openai/gpt-4o-mini",
        API:           ai.ModelApi(ai.ApiOpenAICompletions),
        Provider:      ai.ModelProvider("openai"),
        BaseURL:       "https://api.openai.com/v1",
        Reasoning:     true,
        Input:         []string{"text"},
        Cost:          ai.ModelCost{},
        ContextWindow: 128000,
        MaxTokens:     16384,
    }

    // 创建 Agent
    ag := agent.NewAgent(agent.AgentOptions{
        InitialState: &agent.AgentState{
            SystemPrompt: "你是一个助手。一步一步完成任务。",
            Model:        model,
        },
    })

    // 设置工具
    cwd, _ := os.Getwd()
    ag.SetTools(tools.CreateCodingTools(cwd))

    // 订阅事件
    unsub := ag.Subscribe(func(event agent.AgentEvent) {
        switch e := event.(type) {
        case *agent.AgentEventMessageUpdate:
            if delta, ok := e.AssistantMessageEvent.(*ai.AssistantMessageEventTextDelta); ok {
                fmt.Print(delta.Delta)
            }
        case *agent.AgentEventToolExecutionStart:
            fmt.Printf("\n[工具] %s\n", e.ToolName)
        }
    })
    defer unsub()

    // 发送提示
    ag.Prompt(ctx, "帮我创建一个 hello.go 文件")

    // Steering: 中途更改指令
    ag.Steer(ai.NewUserMessage("暂停，先创建一个配置文件"))

    // Follow-up: 完成后继续
    ag.FollowUp(ai.NewUserMessage("添加错误处理和注释"))
}
```

### 使用 AgentSession

```go
package main

import (
    "context"
    "fmt"
    "os"

    "github.com/jay-y/pi/pkg/ai"
    agent "github.com/jay-y/pi/pkg/ai-agent"
    session "github.com/jay-y/pi/pkg/ai-agent-session"
    tools "github.com/jay-y/pi/pkg/ai-agent-tools"
)

func main() {
    ctx := context.Background()

    // 注册 API 提供者
    ai.RegisterBuiltInApiProviders()

    // 创建模型
    model := &ai.BaseModel{
        ID:            "gpt-4o-mini",
        Name:          "openai/gpt-4o-mini",
        API:           ai.ModelApi(ai.ApiOpenAICompletions),
        Provider:      ai.ModelProvider("openai"),
        BaseURL:       "https://api.openai.com/v1",
        Reasoning:     true,
        Input:         []string{"text"},
        Cost:          ai.ModelCost{},
        ContextWindow: 128000,
        MaxTokens:     16384,
    }

    // 创建 Agent
    ag := agent.NewAgent(agent.AgentOptions{
        InitialState: &agent.AgentState{
            SystemPrompt: "你是一个智能助手。",
            Model:        model,
        },
    })

    // 创建会话
    cwd, _ := os.Getwd()
    agentSession := session.NewAgentSession(&session.AgentSessionConfig{
        Agent:         ag,
        ModelRegistry: &session.ModelRegistry{},
        SessionManager: session.InMemorySessionManager(cwd),
        SettingsManager: &session.SettingsManager{},
    })

    // 订阅事件
    agentSession.Subscribe(func(event session.AgentSessionEvent) {
        switch e := event.(type) {
        case *agent.AgentEventMessageUpdate:
            if delta, ok := e.AssistantMessageEvent.(*ai.AssistantMessageEventTextDelta); ok {
                fmt.Print(delta.Delta)
            }
        case *agent.AgentEventToolExecutionStart:
            fmt.Printf("\n[工具] %s\n", e.ToolName)
        }
    })

    // 发送提示
    agentSession.Prompt(ctx, "分析当前项目结构", nil)

    // Follow-up 消息
    agentSession.FollowUp(ctx, "生成项目文档", nil)

    // 模型切换
    agentSession.CycleModel(ctx, "forward")

    // 思考级别切换
    agentSession.CycleThinkingLevel()
}
```

## 文档

| 文档                                   | 描述                                            |
| -------------------------------------- | ----------------------------------------------- |
| [01-快速开始](docs/01-快速开始.md)     | 最简单的使用方式，包含 LLM 调用、Agent 基础使用 |
| [02-模型选择](docs/02-模型选择.md)     | 选择模型和思考级别，支持多模型切换              |
| [03-系统提示词](docs/03-系统提示词.md) | 自定义系统提示词，包括工具提示和上下文          |
| [04-技能管理](docs/04-技能管理.md)     | 发现、过滤、合并技能                            |
| [05-会话管理](docs/05-会话管理.md)     | 内存会话、持久化会话、会话压缩和重试            |
| [06-完整控制](docs/06-完整控制.md)     | 完全自定义配置，包括自定义工具和扩展            |
| [07-工具开发](docs/07-工具开发.md)     | 开发自定义工具，包括参数验证和错误处理          |
| [08-API参考](docs/08-API参考.md)       | 完整的 API 参考文档                             |

## 开发

```bash
# 构建
make build

# 测试
make test

# 清理
make clean

# 运行命令行程序
make run-cli

# 运行 Agent 示例
go run cmd/ai/agent/main.go

# 运行 Session 示例
go run cmd/ai/agent/session/main.go
```

## 常见问题

### 1. 如何选择合适的思考级别？

- 对于简单任务：使用 `minimal` 或 `low`
- 对于中等复杂度任务：使用 `medium`
- 对于复杂推理任务：使用 `high`
- 对于需要深度推理的任务（如代码生成、数学问题）：使用 `xhigh`（如果模型支持）

### 2. 如何处理工具执行失败？

所有工具都有内置的错误处理机制，执行失败时会返回错误信息作为工具结果。可以在 Agent 事件中捕获并处理：

```go
case *agent.AgentEventToolExecutionEnd:
    if e.IsError {
        fmt.Printf("工具 %s 执行失败: %v\n", e.ToolName, e.Result)
    }
```

### 3. 如何限制工具的执行时间？

对于 Bash 工具，可以通过设置 `timeout` 参数限制执行时间：

```go
params := map[string]any{
    "command": "sleep 10",
    "timeout": 5, // 5秒超时
}
```

### 4. 如何处理大文件？

所有工具都有内置的输出截断机制：

- 默认最大行数：2000 行
- 默认最大字节数：50KB
- 超出限制时会显示截断信息和完整输出文件路径

### 5. 如何自定义工具？

可以实现 `agent.AgentToolConfig` 接口来自定义工具：

```go
type MyTool struct{}

func (t *MyTool) GetName() string         { return "mytool" }
func (t *MyTool) GetLabel() string        { return "My Tool" }
func (t *MyTool) GetDescription() string  { return "My custom tool" }
func (t *MyTool) GetParameters() map[string]any {
    return map[string]any{
        "type": "object",
        "properties": map[string]any{
            "param": map[string]any{
                "type": "string",
                "description": "Parameter for my tool",
            },
        },
        "required": []string{"param"},
    }
}
func (t *MyTool) Execute(ctx context.Context, params map[string]any, onUpdate func(*agent.AgentToolResult)) (*agent.AgentToolResult, error) {
    // 实现工具逻辑
    return &agent.AgentToolResult{
        Content: []ai.ContentBlock{ai.NewTextContentBlock("Result")},
    }, nil
}
```

### 6. 如何调试工具调用？

可以启用详细日志来调试工具调用，观察工具执行的各个阶段：

```go
ag.Subscribe(func(event agent.AgentEvent) {
    if e, ok := event.(*agent.AgentEventToolExecutionStart); ok {
        fmt.Printf("[DEBUG] 开始执行工具: %s\n", e.ToolName)
        fmt.Printf("[DEBUG] 参数: %v\n", e.Args)
    }
})
```

## 开发者指南

### 贡献指南

欢迎贡献！请遵循以下步骤：

1. Fork 该项目
2. 创建分支 (`git checkout -b feature/AmazingFeature`)
3. 提交更改 (`git commit -m 'Add some AmazingFeature'`)
4. 推送到分支 (`git push origin feature/AmazingFeature`)
5. 开启 Pull Request

### 代码结构说明

- `pkg/ai-*`: 各功能模块代码
- `cmd/ai-*`: 命令行程序和示例
- `docs/`: 文档
- `test/`: 测试数据

### 运行测试

```bash
make test
```

## 许可证

MIT
