package terminal

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

// Vault 使用 AES-256-GCM 加密主机凭据。数据库只保存密文，
// 密钥通过 TERMINAL_VAULT_KEY / HOST_VAULT_KEY 环境变量注入。
type Vault struct {
	aead cipher.AEAD
}

func NewVault(secret string) (*Vault, error) {
	secret = strings.TrimSpace(secret)
	if secret == "" {
		return nil, errors.New("凭据加密密钥不能为空")
	}
	// 统一派生 32 字节密钥，避免用户输入的密钥长度不足。
	key := sha256.Sum256([]byte(secret))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Vault{aead: aead}, nil
}

// VaultFromEnv 优先使用专用环境变量；未配置时从 JWT 密钥派生，
// 保证功能开箱可用，同时生产环境仍建议显式配置独立密钥。
func VaultFromEnv(jwtSecret string) (*Vault, error) {
	secret := strings.TrimSpace(os.Getenv("TERMINAL_VAULT_KEY"))
	if secret == "" {
		secret = strings.TrimSpace(os.Getenv("HOST_VAULT_KEY"))
	}
	if secret == "" {
		if strings.TrimSpace(jwtSecret) == "" {
			return nil, errors.New("未配置 TERMINAL_VAULT_KEY，且 JWT 密钥为空")
		}
		secret = "promai-terminal-vault:" + jwtSecret
	}
	return NewVault(secret)
}

func (v *Vault) Encrypt(plain string) (string, error) {
	if plain == "" {
		return "", nil
	}
	nonce := make([]byte, v.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("生成随机 nonce 失败: %w", err)
	}
	sealed := v.aead.Seal(nonce, nonce, []byte(plain), nil)
	return base64.StdEncoding.EncodeToString(sealed), nil
}

func (v *Vault) Decrypt(encoded string) (string, error) {
	if encoded == "" {
		return "", nil
	}
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("凭据密文格式错误: %w", err)
	}
	nonceSize := v.aead.NonceSize()
	if len(raw) < nonceSize {
		return "", errors.New("凭据密文长度不足")
	}
	plain, err := v.aead.Open(nil, raw[:nonceSize], raw[nonceSize:], nil)
	if err != nil {
		return "", errors.New("凭据解密失败，请检查 TERMINAL_VAULT_KEY 是否变更")
	}
	return string(plain), nil
}
