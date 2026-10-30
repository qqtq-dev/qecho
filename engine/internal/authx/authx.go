package authx

import (
	"crypto/rand"
	"encoding/hex"
)

// NewToken 生成 32 字节随机 session token。
func NewToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
