// Package auth 提供管理员口令的 bcrypt 哈希与 JWT 签发/校验，及评论者登录令牌。
package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

// TokenTTL 管理端登录有效期：短有效期，过期重新登录。
const TokenTTL = 12 * time.Hour

// CommenterTokenTTL 评论者（GitHub OAuth）登录有效期：长效免重复授权。
const CommenterTokenTTL = 180 * 24 * time.Hour

// commenterSubPrefix 评论者令牌 sub 前缀：与管理员 sub（用户名）严格隔离，
// 管理端鉴权（AuthRequired）会显式拒绝携带该前缀的令牌。
const commenterSubPrefix = "commenter:"

func HashPassword(password string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(b), err
}

func CheckPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

func MakeToken(username, secret string) (string, error) {
	claims := jwt.MapClaims{
		"sub": username,
		"exp": time.Now().Add(TokenTTL).Unix(),
		"iat": time.Now().Unix(),
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
}

func ParseToken(tokenStr, secret string) (string, error) {
	t, err := parseJWT(tokenStr, secret)
	if err != nil {
		return "", err
	}
	sub, err := t.Claims.GetSubject()
	if err != nil || sub == "" {
		return "", errors.New("token 无效")
	}
	return sub, nil
}

// parseJWT 校验签名与时效并返回原始令牌（供需要读取额外 claim 的调用方使用）。
// 显式限制 HMAC 签名族，防算法混淆。
func parseJWT(tokenStr, secret string) (*jwt.Token, error) {
	t, err := jwt.Parse(tokenStr, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return []byte(secret), nil
	})
	if err != nil || !t.Valid {
		return nil, errors.New("token 无效或已过期")
	}
	return t, nil
}

// CommenterClaims 解析后的评论者令牌内容。
type CommenterClaims struct {
	ID      int64
	Version int // 与 commenters.token_version 比对；小于库中值即已被吊销
}

// MakeCommenterToken 签发评论者令牌：sub = "commenter:<id>"，长效。
// version 取自 commenters.token_version（登录时为 1，退出/吊销后递增）。
func MakeCommenterToken(id int64, version int, secret string) (string, error) {
	if version < 1 {
		version = 1
	}
	claims := jwt.MapClaims{
		"sub": fmt.Sprintf("%s%d", commenterSubPrefix, id),
		"exp": time.Now().Add(CommenterTokenTTL).Unix(),
		"iat": time.Now().Unix(),
		"ver": version, // 令牌版本：服务端递增 token_version 即可吊销全部旧令牌
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
}

// ParseCommenterToken 校验评论者令牌并返回令牌内容；
// 非该前缀的令牌（含管理员令牌）一律拒绝。
// 缺少 ver 的历史令牌按版本 1 处理，保证升级前的会话不失效。
func ParseCommenterToken(tokenStr, secret string) (*CommenterClaims, error) {
	t, err := parseJWT(tokenStr, secret)
	if err != nil {
		return nil, err
	}
	sub, err := t.Claims.GetSubject()
	if err != nil || sub == "" {
		return nil, errors.New("token 无效")
	}
	var id int64
	if _, err := fmt.Sscanf(sub, commenterSubPrefix+"%d", &id); err != nil || id <= 0 {
		return nil, errors.New("非评论者令牌")
	}
	ver := 1
	if mc, ok := t.Claims.(jwt.MapClaims); ok {
		if v, ok := mc["ver"].(float64); ok && v >= 1 {
			ver = int(v)
		}
	}
	return &CommenterClaims{ID: id, Version: ver}, nil
}

// IsCommenterSub 判断管理端鉴权拿到的 sub 是否为评论者令牌（用于显式拒绝）。
func IsCommenterSub(sub string) bool {
	return len(sub) > len(commenterSubPrefix) && sub[:len(commenterSubPrefix)] == commenterSubPrefix
}
