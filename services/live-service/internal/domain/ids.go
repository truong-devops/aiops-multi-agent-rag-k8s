package domain

import (
	"crypto/rand"
	"encoding/hex"
	"time"
)

func NewID(prefix string) string {
	buf := make([]byte, 12)
	if _, err := rand.Read(buf); err != nil {
		return prefix + "_" + hex.EncodeToString([]byte(time.Now().Format(time.RFC3339Nano)))
	}
	return prefix + "_" + hex.EncodeToString(buf)
}

func NewSecret(byteCount int) (string, error) {
	if byteCount <= 0 {
		byteCount = 32
	}
	buf := make([]byte, byteCount)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return "sk_" + hex.EncodeToString(buf), nil
}
