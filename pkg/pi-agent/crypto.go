package piagent

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"io"
	"log"
)

func deriveKey(secret string) []byte {
	h := sha256.Sum256([]byte(secret))
	return h[:]
}

func EncryptAPIKey(plaintext, jwtSecret string) (string, error) {
	key := deriveKey(jwtSecret)
	block, err := aes.NewCipher(key)
	if err != nil {
		log.Printf("[Crypto] 创建加密块失败: %v", err)
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		log.Printf("[Crypto] 创建 GCM 失败: %v", err)
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		log.Printf("[Crypto] 生成 nonce 失败: %v", err)
		return "", err
	}
	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

func DecryptAPIKey(encoded, jwtSecret string) (string, error) {
	key := deriveKey(jwtSecret)
	ciphertext, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		log.Printf("[Crypto] Base64 解码失败: %v", err)
		return "", err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		log.Printf("[Crypto] 创建解密块失败: %v", err)
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		log.Printf("[Crypto] 创建 GCM 失败: %v", err)
		return "", err
	}
	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return "", errors.New("ciphertext too short")
	}
	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		log.Printf("[Crypto] 解密失败: %v", err)
		return "", err
	}
	return string(plaintext), nil
}
