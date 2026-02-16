// Package utils provides AES-256-GCM encryption helpers.
package utils

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

// Encrypt encrypts plaintext using AES-256-GCM and returns a base64 string.
// The key is derived from the input string using SHA-256 to ensure 32 bytes.
//
// Example:
//  cipherText, err := Encrypt([]byte("secret"), "my-32-char-key-string")
func Encrypt(plaintext []byte, key string) (string, error) {
	if len(plaintext) == 0 {
		return "", errors.New("plaintext is empty")
	}
	k, err := deriveKey(key)
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(k)
	if err != nil {
		return "", fmt.Errorf("new cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("new gcm: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("read nonce: %w", err)
	}

	// Prepend nonce to ciphertext for storage.
	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt decrypts a base64 string produced by Encrypt.
//
// Example:
//  plain, err := Decrypt(cipherText, "my-32-char-key-string")
func Decrypt(cipherText string, key string) ([]byte, error) {
	if strings.TrimSpace(cipherText) == "" {
		return nil, errors.New("cipherText is empty")
	}
	k, err := deriveKey(key)
	if err != nil {
		return nil, err
	}

	raw, err := base64.StdEncoding.DecodeString(cipherText)
	if err != nil {
		return nil, fmt.Errorf("decode base64: %w", err)
	}

	block, err := aes.NewCipher(k)
	if err != nil {
		return nil, fmt.Errorf("new cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("new gcm: %w", err)
	}

	if len(raw) < gcm.NonceSize() {
		return nil, errors.New("cipherText too short")
	}
	nonce := raw[:gcm.NonceSize()]
	data := raw[gcm.NonceSize():]

	plain, err := gcm.Open(nil, nonce, data, nil)
	if err != nil {
		return nil, fmt.Errorf("decrypt: %w", err)
	}
	return plain, nil
}

// deriveKey normalizes the input key string to 32 bytes using SHA-256.
// It allows any length string, including a 32-char key as requested.
func deriveKey(key string) ([]byte, error) {
	if strings.TrimSpace(key) == "" {
		return nil, errors.New("key is empty")
	}
	sum := sha256.Sum256([]byte(key))
	return sum[:], nil
}
