// Package utils 提供AES-256-GCM加密工具
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"
)

// Encrypt 使用AES-256-GCM加密明文并返回base64字符串
// 密钥通过SHA-256从输入字符串派生以确保32字节
//
// 示例：
//  cipherText, err := Encrypt([]byte("secret"), "my-32-char-key-string")
func Encrypt(plaintext []byte, key string) (string, error) {
	if len(plaintext) == 0 {
		return "", errors.New("明文为空")
	}
	k, err := deriveKey(key)
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(k)
	if err != nil {
		return "", fmt.Errorf("创建密码器失败: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("创建GCM失败: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("读取随机数失败: %w", err)
	}

	// 将nonce前置到密文以便存储
	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt 解密由Encrypt生成的base64字符串
//
// 示例：
//  plain, err := Decrypt(cipherText, "my-32-char-key-string")
func Decrypt(cipherText string, key string) ([]byte, error) {
	if strings.TrimSpace(cipherText) == "" {
		return nil, errors.New("密文为空")
	}
	k, err := deriveKey(key)
	if err != nil {
		return nil, err
	}

	raw, err := base64.StdEncoding.DecodeString(cipherText)
	if err != nil {
		return nil, fmt.Errorf("base64解码失败: %w", err)
	}

	block, err := aes.NewCipher(k)
	if err != nil {
		return nil, fmt.Errorf("创建密码器失败: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("创建GCM失败: %w", err)
	}

	if len(raw) < gcm.NonceSize() {
		return nil, errors.New("密文过短")
	}
	nonce := raw[:gcm.NonceSize()]
	data := raw[gcm.NonceSize():]

	plain, err := gcm.Open(nil, nonce, data, nil)
	if err != nil {
		return nil, fmt.Errorf("解密失败: %w", err)
	}
	return plain, nil
}

// deriveKey 使用SHA-256将输入密钥字符串规范化为32字节
// 允许任何长度的字符串，包括32字符的密钥
func deriveKey(key string) ([]byte, error) {
	if strings.TrimSpace(key) == "" {
		return nil, errors.New("密钥为空")
	}
	sum := sha256.Sum256([]byte(key))
	return sum[:], nil
}
