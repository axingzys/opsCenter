package guacamole

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
)

type AuthPayload struct {
	Username    string                `json:"username"`
	Expires     int64                 `json:"expires,omitempty"`
	Connections map[string]Connection `json:"connections"`
}

type Connection struct {
	ID         string            `json:"id,omitempty"`
	Protocol   string            `json:"protocol"`
	Parameters map[string]string `json:"parameters"`
}

// EncryptAuthPayload 生成 Guacamole JSON Auth data 参数
func EncryptAuthPayload(secretKeyHex string, payload *AuthPayload) (string, error) {
	if payload == nil {
		return "", fmt.Errorf("payload 不能为空")
	}

	key, err := hex.DecodeString(strings.TrimSpace(secretKeyHex))
	if err != nil {
		return "", fmt.Errorf("解析 guacamole json secret 失败: %w", err)
	}
	if len(key) != 16 && len(key) != 24 && len(key) != 32 {
		return "", fmt.Errorf("guacamole json secret 长度无效")
	}

	plain, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("序列化 guacamole payload 失败: %w", err)
	}

	mac := hmac.New(sha256.New, key)
	mac.Write(plain)
	signed := append(mac.Sum(nil), plain...)

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("创建 AES cipher 失败: %w", err)
	}

	padded := pkcs7Pad(signed, block.BlockSize())
	encrypted := make([]byte, len(padded))
	iv := make([]byte, block.BlockSize())
	cipher.NewCBCEncrypter(block, iv).CryptBlocks(encrypted, padded)

	return base64.StdEncoding.EncodeToString(encrypted), nil
}

func pkcs7Pad(data []byte, blockSize int) []byte {
	padding := blockSize - len(data)%blockSize
	out := make([]byte, len(data)+padding)
	copy(out, data)
	for i := len(data); i < len(out); i++ {
		out[i] = byte(padding)
	}
	return out
}
