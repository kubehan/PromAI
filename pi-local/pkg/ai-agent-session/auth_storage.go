package session

import "github.com/jay-y/pi/pkg/ai"

// AuthCredential 认证凭证
type AuthCredential struct {
	Type string `json:"type"` // "apiKey" | "oauth"
}

// OAuthProvider OAuth提供商接口
type OAuthProvider interface {
	GetID() string
	ModifyModels(models []ai.Model, cred *AuthCredential) []ai.Model
}

// AuthStorage 认证存储接口
type AuthStorage interface {
	GetApiKey(provider string) (string, error)
	HasAuth(provider string) bool
	Get(provider string) *AuthCredential
	GetOAuthProviders() []OAuthProvider
	SetFallbackResolver(resolver func(provider string) *string)
}