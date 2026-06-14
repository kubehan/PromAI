package ai

import (
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"
)

// getCurrentTimestamp 获取当前时间戳
func getCurrentTimestamp() int64 {
	return time.Now().Unix()
}

// contains 判断字符串是否包含子字符串
func contains(s, substr string) bool {
	if len(s) < len(substr) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

var (
	cachedVertexAdcCredentialsExists *bool
	vertexAdcMu                       sync.Once
)

// hasVertexAdcCredentials 检查是否存在 Vertex ADC 凭据
func hasVertexAdcCredentials() bool {
	vertexAdcMu.Do(func() {
		// 首先检查 GOOGLE_APPLICATION_CREDENTIALS 环境变量
		gacPath := os.Getenv("GOOGLE_APPLICATION_CREDENTIALS")
		if gacPath != "" {
			if _, err := os.Stat(gacPath); err == nil {
				val := true
				cachedVertexAdcCredentialsExists = &val
				return
			}
		}

		// 回退到默认 ADC 路径
		var homeDir string
		if runtime.GOOS == "windows" {
			homeDir = os.Getenv("USERPROFILE")
		} else {
			homeDir = os.Getenv("HOME")
		}

		if homeDir != "" {
			adcPath := filepath.Join(homeDir, ".config", "gcloud", "application_default_credentials.json")
			if _, err := os.Stat(adcPath); err == nil {
				val := true
				cachedVertexAdcCredentialsExists = &val
				return
			}
		}

		val := false
		cachedVertexAdcCredentialsExists = &val
	})

	if cachedVertexAdcCredentialsExists != nil {
		return *cachedVertexAdcCredentialsExists
	}
	return false
}

// getEnvApiKey 从已知环境变量获取提供者的 API 密钥
func getEnvApiKey(provider ModelProvider) string {
	providerStr := string(provider)

	switch providerStr {
	case "github-copilot":
		if token := os.Getenv("COPILOT_GITHUB_TOKEN"); token != "" {
			return token
		}
		if token := os.Getenv("GH_TOKEN"); token != "" {
			return token
		}
		return os.Getenv("GITHUB_TOKEN")

	case "anthropic":
		if token := os.Getenv("ANTHROPIC_OAUTH_TOKEN"); token != "" {
			return token
		}
		return os.Getenv("ANTHROPIC_API_KEY")

	case "google-vertex":
		hasCredentials := hasVertexAdcCredentials()
		hasProject := os.Getenv("GOOGLE_CLOUD_PROJECT") != "" || os.Getenv("GCLOUD_PROJECT") != ""
		hasLocation := os.Getenv("GOOGLE_CLOUD_LOCATION") != ""

		if hasCredentials && hasProject && hasLocation {
			return "<authenticated>"
		}
		return ""

	case "amazon-bedrock":
		if os.Getenv("AWS_PROFILE") != "" ||
			(os.Getenv("AWS_ACCESS_KEY_ID") != "" && os.Getenv("AWS_SECRET_ACCESS_KEY") != "") ||
			os.Getenv("AWS_BEARER_TOKEN_BEDROCK") != "" ||
			os.Getenv("AWS_CONTAINER_CREDENTIALS_RELATIVE_URI") != "" ||
			os.Getenv("AWS_CONTAINER_CREDENTIALS_FULL_URI") != "" ||
			os.Getenv("AWS_WEB_IDENTITY_TOKEN_FILE") != "" {
			return "<authenticated>"
		}
		return ""
	}

	envMap := map[string]string{
		"openai":                 "OPENAI_API_KEY",
		"azure-openai-responses": "AZURE_OPENAI_API_KEY",
		"google":                 "GEMINI_API_KEY",
		"groq":                   "GROQ_API_KEY",
		"cerebras":               "CEREBRAS_API_KEY",
		"xai":                    "XAI_API_KEY",
		"openrouter":             "OPENROUTER_API_KEY",
		"vercel-ai-gateway":      "AI_GATEWAY_API_KEY",
		"zai":                    "ZAI_API_KEY",
		"mistral":                "MISTRAL_API_KEY",
		"minimax":                "MINIMAX_API_KEY",
		"minimax-cn":             "MINIMAX_CN_API_KEY",
		"huggingface":            "HF_TOKEN",
		"opencode":               "OPENCODE_API_KEY",
		"kimi-coding":            "KIMI_API_KEY",
	}

	if envVar, ok := envMap[providerStr]; ok {
		return os.Getenv(envVar)
	}
	return ""
}

func calculateCost(model Model, usage *Usage) Cost {
	if usage == nil {
		return Cost{
			Input: 0,
			Output: 0,
			CacheRead: 0,
			CacheWrite: 0,
			Total: 0,
		}
	}
	usage.Cost.Input = (model.GetCost().Input / 1000000) * float64(usage.Input)
	usage.Cost.Output = (model.GetCost().Output / 1000000) * float64(usage.Output)
	usage.Cost.CacheRead = (model.GetCost().CacheRead / 1000000) * float64(usage.CacheRead)
	usage.Cost.CacheWrite = (model.GetCost().CacheWrite / 1000000) * float64(usage.CacheWrite)
	usage.Cost.Total = usage.Cost.Input + usage.Cost.Output + usage.Cost.CacheRead + usage.Cost.CacheWrite
	return usage.Cost
}