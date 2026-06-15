package ai

// ModelApi 调用的 API 接口
type ModelApi string
const (
	ApiOpenAICompletions     ModelApi = "openai-completions"
	ApiOpenAIResponses       ModelApi = "openai-responses"
	ApiAzureOpenAIResponses  ModelApi = "azure-openai-responses"
	ApiOpenAICodexResponses  ModelApi = "openai-codex-responses"
	ApiAnthropicMessages     ModelApi = "anthropic-messages"
	ApiBedrockConverseStream ModelApi = "bedrock-converse-stream"
	ApiGoogleGenerativeAI    ModelApi = "google-generative-ai"
	ApiGoogleGeminiCLI       ModelApi = "google-gemini-cli"
	ApiGoogleVertex          ModelApi = "google-vertex"
)

// ModelProvider 模型供应商
type ModelProvider string
const (
	ProviderAmazonBedrock      ModelProvider = "amazon-bedrock"
	ProviderAnthropic          ModelProvider = "anthropic"
	ProviderGoogle             ModelProvider = "google"
	ProviderGoogleGeminiCLI    ModelProvider = "google-gemini-cli"
	ProviderGoogleAntigravity  ModelProvider = "google-antigravity"
	ProviderGoogleVertex       ModelProvider = "google-vertex"
	ProviderOpenAI             ModelProvider = "openai"
	ProviderAzureOpenAIResponses ModelProvider = "azure-openai-responses"
	ProviderOpenAICodex        ModelProvider = "openai-codex"
	ProviderGitHubCopilot      ModelProvider = "github-copilot"
	ProviderXAI                ModelProvider = "xai"
	ProviderGroq               ModelProvider = "groq"
	ProviderCerebras           ModelProvider = "cerebras"
	ProviderOpenRouter         ModelProvider = "openrouter"
	ProviderVercelAIGateway    ModelProvider = "vercel-ai-gateway"
	ProviderZAI                ModelProvider = "zai"
	ProviderMistral            ModelProvider = "mistral"
	ProviderMinimax            ModelProvider = "minimax"
	ProviderMinimaxCN          ModelProvider = "minimax-cn"
	ProviderHuggingFace        ModelProvider = "huggingface"
	ProviderOpenCode           ModelProvider = "opencode"
	ProviderKimiCoding         ModelProvider = "kimi-coding"
)

// ThinkingLevel 思考级别
type ThinkingLevel string
const (
	ThinkingLevelOff     ThinkingLevel = "off"
	ThinkingLevelMinimal ThinkingLevel = "minimal"
	ThinkingLevelLow     ThinkingLevel = "low"
	ThinkingLevelMedium  ThinkingLevel = "medium"
	ThinkingLevelHigh    ThinkingLevel = "high"
	ThinkingLevelXHigh   ThinkingLevel = "xhigh"
)

// CacheRetention 缓存保留时间
type CacheRetention string
const (
	CacheRetentionNone  CacheRetention = "none"
	CacheRetentionShort CacheRetention = "short"
	CacheRetentionLong  CacheRetention = "long"
)

// Transport 传输协议
type Transport string
const (
	TransportSSE       Transport = "sse"
	TransportWebSocket Transport = "websocket"
	TransportAuto      Transport = "auto"
)

// StopReason 停止原因
type StopReason string
const (
	StopReasonStop    StopReason = "stop"
	StopReasonLength  StopReason = "length"
	StopReasonToolUse StopReason = "toolUse"
	StopReasonError   StopReason = "error"
	StopReasonAborted StopReason = "aborted"
)

const (
	ASSISTANT_MESSAGE_EVENT_START = "start"
	ASSISTANT_MESSAGE_EVENT_TEXT_START = "text_start"
	ASSISTANT_MESSAGE_EVENT_TEXT_DELTA = "text_delta"
	ASSISTANT_MESSAGE_EVENT_TEXT_END = "text_end"
	ASSISTANT_MESSAGE_EVENT_THINKING_START = "thinking_start"
	ASSISTANT_MESSAGE_EVENT_THINKING_DELTA = "thinking_delta"
	ASSISTANT_MESSAGE_EVENT_THINKING_END = "thinking_end"
	ASSISTANT_MESSAGE_EVENT_TOOLCALL_START = "toolcall_start"
	ASSISTANT_MESSAGE_EVENT_TOOLCALL_DELTA = "toolcall_delta"
	ASSISTANT_MESSAGE_EVENT_TOOLCALL_END = "toolcall_end"
	ASSISTANT_MESSAGE_EVENT_DONE = "done"
	ASSISTANT_MESSAGE_EVENT_ERROR = "error"
)