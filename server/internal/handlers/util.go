package handlers

import (
	"crypto/sha256"
	"encoding/hex"
	"time"
)

const (
	timeMinute = time.Minute
	timeDay    = 24 * time.Hour
)

func timeNowMilli() int64 { return time.Now().UnixMilli() }

// hashWithSalt 加盐 SHA-256：评论存 IP/邮箱哈希而非明文（隐私设计，方案 4.3/5）。
// 邮箱传空串时返回空串，避免空邮箱也生成固定哈希。
func hashWithSalt(salt, value string) string {
	if value == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(salt + "|" + value))
	return hex.EncodeToString(sum[:])
}
