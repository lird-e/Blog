package handlers

import (
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"time"
)

const (
	timeMinute = time.Minute
	timeDay    = 24 * time.Hour
)

func timeNowMilli() int64 { return time.Now().UnixMilli() }

// hashWithSalt 加盐 SHA-256：评论存 IP 哈希而非明文（隐私设计）。
// IP 无对外用途，加盐防彩虹表反查。
func hashWithSalt(salt, value string) string {
	if value == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(salt + "|" + value))
	return hex.EncodeToString(sum[:])
}

// md5Hex 未加盐 MD5：Cravatar/Gravatar 头像协议要求 MD5(email) 原值，
// 加盐会导致头像永远无法匹配。MD5(email) 单向不可反查明文（Gravatar 全网同款），
// 邮箱传空串时返回空串。
func md5Hex(value string) string {
	if value == "" {
		return ""
	}
	sum := md5.Sum([]byte(value))
	return hex.EncodeToString(sum[:])
}

// emailHashRe 校验前端直传的 email_hash：仅接受 32 位小写 hex（md5Hex 的输出格式），
// 防止垃圾数据借该字段入库。
var emailHashRe = regexp.MustCompile(`^[0-9a-f]{32}$`)

// authConfigured 判断 JWT_SECRET 是否已配置。
// 管理端登录与 OAuth 登录都依赖它签发/校验令牌，也依赖它给 OAuth state 做 HMAC；
// 缺失时必须以 503 拒绝，绝不能用空密钥签名（否则令牌与 state 均可伪造）。
func (h *Handlers) authConfigured() bool {
	return h.Cfg != nil && h.Cfg.JWTSecret != ""
}
