// GitHub OAuth 评论者登录：
//
//	GET /api/auth/github/login?redirect=/post/xxx  → 302 到 GitHub 授权页
//	GET /api/auth/github/callback?code&state       → 换取用户信息、落库、签发会话 Cookie、302 回文章页
//	GET /api/me                                    → 前端查询当前登录身份
//	POST /api/auth/logout                          → 清除会话 Cookie
//
// 会话采用 HttpOnly Cookie（SameSite=Lax），评论者令牌与管理员令牌通过
// sub 前缀 "commenter:" 严格隔离（见 internal/auth）。
// state 使用 HMAC-SHA256 签名 + 10 分钟时效，防 CSRF；redirect 白名单校验防开放跳转。
package handlers

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/lird-e/Blog/server/internal/auth"
	"github.com/lird-e/Blog/server/internal/db"
)

// 包级变量：测试时可替换为 httptest mock 地址。
var (
	githubAuthorizeURL = "https://github.com/login/oauth/authorize"
	githubTokenURL     = "https://github.com/login/oauth/access_token"
	githubUserURL      = "https://api.github.com/user"
	ghHTTP             = &http.Client{Timeout: 10 * time.Second}
)

const (
	commenterCookie = "commenter_sess"
	stateTTL        = 10 * time.Minute
)

type ghUser struct {
	ID        int64  `json:"id"`
	Login     string `json:"login"`
	AvatarURL string `json:"avatar_url"`
}

// GitHubLogin GET /api/auth/github/login?redirect=/post/xxx
func (h *Handlers) GitHubLogin(c *gin.Context) {
	if h.Cfg.GitHubClientID == "" || h.Cfg.GitHubSecret == "" {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "GitHub 登录未配置（缺少 GITHUB_CLIENT_ID / GITHUB_SECRET）",
		})
		return
	}
	redirect := sanitizeRedirect(c.Query("redirect"))
	state, err := h.signState(redirect)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "生成登录状态失败"})
		return
	}
	q := url.Values{}
	q.Set("client_id", h.Cfg.GitHubClientID)
	q.Set("scope", "read:user")
	q.Set("state", state)
	// 回调地址由站点配置拼接，须与 GitHub OAuth App 中登记的 callback URL 一致
	q.Set("redirect_uri", strings.TrimSuffix(h.Cfg.SiteURL, "/")+"/api/auth/github/callback")
	c.Redirect(http.StatusFound, githubAuthorizeURL+"?"+q.Encode())
}

// GitHubCallback GET /api/auth/github/callback
func (h *Handlers) GitHubCallback(c *gin.Context) {
	fail := func(status int, msg string) {
		c.Data(status, "text/html; charset=utf-8",
			[]byte(`<meta charset="utf-8"><p>GitHub 登录失败：`+msg+`</p><p><a href="/">返回首页</a></p>`))
	}
	if h.Cfg.GitHubClientID == "" || h.Cfg.GitHubSecret == "" {
		fail(http.StatusServiceUnavailable, "服务端未配置 GitHub 登录")
		return
	}
	if e := c.Query("error"); e != "" { // 用户在 GitHub 侧点了取消等
		redirect := "/"
		if r, err := h.verifyState(c.Query("state")); err == nil {
			redirect = r
		}
		c.Redirect(http.StatusFound, redirect+"?login=cancelled")
		return
	}
	redirect, err := h.verifyState(c.Query("state"))
	if err != nil {
		fail(http.StatusBadRequest, "state 校验未通过，请返回重新发起登录")
		return
	}
	code := c.Query("code")
	if code == "" {
		fail(http.StatusBadRequest, "缺少授权码")
		return
	}
	user, err := h.fetchGitHubUser(code)
	if err != nil || user.ID <= 0 {
		fail(http.StatusBadGateway, "获取 GitHub 用户信息失败，请稍后重试")
		return
	}
	nickname := user.Login
	if nickname == "" {
		nickname = "gh-" + strconv.FormatInt(user.ID%10000, 10)
	}
	id, err := h.DB.UpsertCommenter("github", strconv.FormatInt(user.ID, 10), nickname, user.AvatarURL)
	if err != nil {
		fail(http.StatusInternalServerError, "保存身份失败")
		return
	}
	token, err := auth.MakeCommenterToken(id, h.Cfg.JWTSecret)
	if err != nil {
		fail(http.StatusInternalServerError, "签发会话失败")
		return
	}
	h.setCommenterCookie(c, token)
	c.Redirect(http.StatusFound, redirect)
}

// Me GET /api/me —— 前端判断评论者登录态；未登录/已过期统一返回 null，不报错。
func (h *Handlers) Me(c *gin.Context) {
	cm := h.currentCommenter(c)
	if cm == nil {
		c.JSON(http.StatusOK, gin.H{"commenter": nil})
		return
	}
	c.JSON(http.StatusOK, gin.H{"commenter": cm})
}

// Logout POST /api/auth/logout —— 清除会话 Cookie。
func (h *Handlers) Logout(c *gin.Context) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name: commenterCookie, Value: "", Path: "/",
		MaxAge: -1, HttpOnly: true, SameSite: http.SameSiteLaxMode,
	})
	c.JSON(http.StatusOK, gin.H{"message": "已退出登录"})
}

// currentCommenter 从会话 Cookie 解析当前登录身份；无效/过期/身份不存在返回 nil。
// 供 /api/me 与 CreateComment 复用。
func (h *Handlers) currentCommenter(c *gin.Context) *db.Commenter {
	ck, err := c.Cookie(commenterCookie)
	if err != nil || ck == "" {
		return nil
	}
	id, err := auth.ParseCommenterToken(ck, h.Cfg.JWTSecret)
	if err != nil {
		return nil
	}
	cm, err := h.DB.GetCommenter(id)
	if err != nil {
		return nil
	}
	return cm
}

// setCommenterCookie 写入会话 Cookie；SiteURL 为 https 时附带 Secure 标志。
func (h *Handlers) setCommenterCookie(c *gin.Context, token string) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     commenterCookie,
		Value:    token,
		Path:     "/",
		MaxAge:   int(auth.CommenterTokenTTL.Seconds()),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   strings.HasPrefix(h.Cfg.SiteURL, "https://"),
	})
}

// ---- GitHub API ----

// fetchGitHubUser 授权码换 access_token 再拉取用户信息。
func (h *Handlers) fetchGitHubUser(code string) (*ghUser, error) {
	form := url.Values{}
	form.Set("client_id", h.Cfg.GitHubClientID)
	form.Set("client_secret", h.Cfg.GitHubSecret)
	form.Set("code", code)
	req, err := http.NewRequest(http.MethodPost, githubTokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := ghHTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil || resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token 交换失败 status=%d", resp.StatusCode)
	}
	var tok struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(body, &tok); err != nil || tok.AccessToken == "" {
		return nil, fmt.Errorf("token 响应无效")
	}

	ureq, err := http.NewRequest(http.MethodGet, githubUserURL, nil)
	if err != nil {
		return nil, err
	}
	ureq.Header.Set("Authorization", "Bearer "+tok.AccessToken)
	ureq.Header.Set("Accept", "application/json")
	uresp, err := ghHTTP.Do(ureq)
	if err != nil {
		return nil, err
	}
	defer uresp.Body.Close()
	ubody, err := io.ReadAll(io.LimitReader(uresp.Body, 1<<20))
	if err != nil || uresp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("用户信息拉取失败 status=%d", uresp.StatusCode)
	}
	var user ghUser
	if err := json.Unmarshal(ubody, &user); err != nil {
		return nil, err
	}
	return &user, nil
}

// ---- state 签名 ----

// signState 生成 "nonce|redirect|ts|mac" 格式的防 CSRF 状态（HMAC-SHA256，密钥用 JWT_SECRET）。
func (h *Handlers) signState(redirect string) (string, error) {
	nonce := make([]byte, 16)
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	payload := fmt.Sprintf("%s|%s|%d", hex.EncodeToString(nonce), redirect, time.Now().Unix())
	return payload + "|" + hmacSHA256Hex(h.Cfg.JWTSecret, payload), nil
}

// verifyState 校验签名与时效，返回原 redirect。
func (h *Handlers) verifyState(state string) (string, error) {
	parts := strings.Split(state, "|")
	if len(parts) != 4 {
		return "", fmt.Errorf("state 格式无效")
	}
	nonce, redirect, tsStr, mac := parts[0], parts[1], parts[2], parts[3]
	payload := nonce + "|" + redirect + "|" + tsStr
	expect := hmacSHA256Hex(h.Cfg.JWTSecret, payload)
	if !hmac.Equal([]byte(expect), []byte(mac)) {
		return "", fmt.Errorf("state 签名不符")
	}
	ts, err := strconv.ParseInt(tsStr, 10, 64)
	if err != nil || time.Since(time.Unix(ts, 0)) > stateTTL || ts > time.Now().Unix()+60 {
		return "", fmt.Errorf("state 已过期")
	}
	return sanitizeRedirect(redirect), nil
}

// sanitizeRedirect 跳转白名单：仅站内绝对路径，拒绝协议相对 URL 与控制字符，兜底回首页。
func sanitizeRedirect(r string) string {
	if r == "" || !strings.HasPrefix(r, "/") || strings.HasPrefix(r, "//") ||
		strings.ContainsAny(r, "|\r\n\\") {
		return "/"
	}
	return r
}

func hmacSHA256Hex(secret, payload string) string {
	m := hmac.New(sha256.New, []byte(secret))
	m.Write([]byte(payload))
	return hex.EncodeToString(m.Sum(nil))
}
