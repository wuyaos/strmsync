// Package utils provides request id generator.
package utils

import (
	"crypto/rand"
	"encoding/hex"
)

// NewRequestID returns a random request id (32 character hex string).
func NewRequestID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return ""
	}
	return hex.EncodeToString(b[:])
}
