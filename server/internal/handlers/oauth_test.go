package handlers

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/lird-e/Blog/server/internal/auth"
	"github.com/lird-e/Blog/server/internal/config"
	"github.com/lird-e/Blog/server/internal/db"
	"github.com/lird-e/Blog/server/internal/limiter"
)

// ---- 测试装配 ----

func newTestServer(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	database, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("打开测试库失败: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	// 测试文章：评论接口均依赖已发布文章存在
	if _, err := database.UpsertPost(&db.Post{
		Slug: "测", Title: "测试文章", ContentMD: "# t", ContentHTML: "<h1>t</h1>", Published: true,
		CreatedAt: "2026-01-01T00:00:00+08:00", UpdatedAt: "2026-01-01T00:00:00+08:00",
	}); err != nil {
		t.Fatalf("插入测试文章失败: %v", err)
	}
	h := &Handlers{
		DB: database,
		Cfg: &config.Config{
			SiteURL: "http://127.0.0.1:8080", JWTSecret: "test-secret", IPSalt: "test-salt",
			GitHubClientID: "test-client-id", GitHubSecret: "test-client-secret",
		},
		Lim:      limiter.New(),
		LoginLim: limiter.New(),
	}
	r := gin.New()
	api := r.Group("/api")
	{
		api.GET("/posts/:slug/comments", h.ListComments)
		api.POST("/posts/:slug/comments", h.CreateComment)
		api.GET("/auth/github/login", h.GitHubLogin)
		api.GET("/auth/github/callback", h.GitHubCallback)
		api.POST("/auth/logout", h.Logout)
		api.GET("/me", h.Me)
	}
	guarded := r.Group("/api/admin", h.AuthRequired())
	guarded.GET("/posts", h.AdminListPosts)
	return r
}

func doReq(r http.Handler, method, path, body string, headers map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// mockGitHub 用 httptest 顶替 GitHub 的 token/user 接口，返回 (mock用户服务地址, 清理函数)。
func mockGitHub(t *testing.T, tokenResp, userResp string, wantBearer string) {
	t.Helper()
	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, tokenResp)
	}))
	userSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if wantBearer != "" && r.Header.Get("Authorization") != "Bearer "+wantBearer {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, userResp)
	}))
	oldTok, oldUser := githubTokenURL, githubUserURL
	githubTokenURL, githubUserURL = tokenSrv.URL, userSrv.URL
	t.Cleanup(func() {
		githubTokenURL, githubUserURL = oldTok, oldUser
		tokenSrv.Close()
		userSrv.Close()
	})
}

// startLogin 走一遍 login → callback，返回带会话 Cookie 的请求头与落地跳转。
func startLogin(t *testing.T, r *gin.Engine) (cookieHeader string) {
	t.Helper()
	w := doReq(r, http.MethodGet, "/api/auth/github/login?redirect=/post/abc", "", nil)
	if w.Code != http.StatusFound {
		t.Fatalf("login 应 302，got %d: %s", w.Code, w.Body.String())
	}
	loc, err := url.Parse(w.Header().Get("Location"))
	if err != nil {
		t.Fatalf("login Location 解析失败: %v", err)
	}
	q := loc.Query()
	if q.Get("client_id") != "test-client-id" || q.Get("scope") != "read:user" || q.Get("state") == "" {
		t.Fatalf("login Location 参数缺失: %s", loc)
	}
	if !strings.HasPrefix(q.Get("redirect_uri"), "http://127.0.0.1:8080/api/auth/github/callback") {
		t.Fatalf("redirect_uri 不符: %s", q.Get("redirect_uri"))
	}
	w2 := doReq(r, http.MethodGet, "/api/auth/github/callback?code=good-code&state="+url.QueryEscape(q.Get("state")), "", nil)
	if w2.Code != http.StatusFound || w2.Header().Get("Location") != "/post/abc" {
		t.Fatalf("callback 应 302 回 /post/abc，got %d -> %s: %s",
			w2.Code, w2.Header().Get("Location"), w2.Body.String())
	}
	cookie := w2.Header().Get("Set-Cookie")
	if !strings.Contains(cookie, "commenter_sess=") || !strings.Contains(cookie, "HttpOnly") {
		t.Fatalf("callback 未正确下发会话 Cookie: %s", cookie)
	}
	return strings.Split(cookie, ";")[0]
}

// ---- 用例 ----

func TestGitHubLoginUnconfigured(t *testing.T) {
	r := newTestServer(t)
	// 未配置凭据：临时清空 Cfg 字段
	// （newTestServer 已配置，这里直接构造一个未配置实例）
	gin.SetMode(gin.TestMode)
	h := &Handlers{DB: nil, Cfg: &config.Config{JWTSecret: "s"}}
	sub := gin.New()
	sub.GET("/api/auth/github/login", h.GitHubLogin)
	w := doReq(sub, http.MethodGet, "/api/auth/github/login", "", nil)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("未配置应 503，got %d", w.Code)
	}
	_ = r
}

func TestGitHubCallbackBadState(t *testing.T) {
	r := newTestServer(t)
	mockGitHub(t, `{"access_token":"x"}`, `{"id":1}`, "")
	w := doReq(r, http.MethodGet, "/api/auth/github/callback?code=c&state=tampered|/|123|deadbeef", "", nil)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("篡改 state 应 400，got %d", w.Code)
	}
}

func TestGitHubCallbackExpiredState(t *testing.T) {
	r := newTestServer(t)
	mockGitHub(t, `{"access_token":"x"}`, `{"id":1}`, "")
	old := fmt.Sprintf("%s|/post/abc|%d", "cafebabe", time.Now().Add(-11*time.Minute).Unix())
	tampered := old + "|" + hmacSHA256Hex("test-secret", old)
	w := doReq(r, http.MethodGet, "/api/auth/github/callback?code=c&state="+url.QueryEscape(tampered), "", nil)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("过期 state 应 400，got %d", w.Code)
	}
}

func TestGitHubCallbackCancelled(t *testing.T) {
	r := newTestServer(t)
	mockGitHub(t, `{"access_token":"x"}`, `{"id":1}`, "")
	w := doReq(r, http.MethodGet, "/api/auth/github/login?redirect=/post/xyz", "", nil)
	state := mustState(t, w.Header().Get("Location"))
	w2 := doReq(r, http.MethodGet, "/api/auth/github/callback?error=access_denied&state="+url.QueryEscape(state), "", nil)
	if w2.Code != http.StatusFound || !strings.HasPrefix(w2.Header().Get("Location"), "/post/xyz") {
		t.Fatalf("取消授权应 302 回原页，got %d -> %s", w2.Code, w2.Header().Get("Location"))
	}
}

func mustState(t *testing.T, location string) string {
	t.Helper()
	u, err := url.Parse(location)
	if err != nil {
		t.Fatal(err)
	}
	return u.Query().Get("state")
}

func TestGitHubLoginAndCommentFlow(t *testing.T) {
	r := newTestServer(t)
	mockGitHub(t,
		`{"access_token":"gho_test","token_type":"bearer"}`,
		`{"id":12345,"login":"octocat","avatar_url":"http://example.com/a.png"}`,
		"gho_test")
	cookie := startLogin(t, r)

	// /me 应返回登录身份
	w := doReq(r, http.MethodGet, "/api/me", "", map[string]string{"Cookie": cookie})
	if !strings.Contains(w.Body.String(), `"nickname":"octocat"`) ||
		!strings.Contains(w.Body.String(), `a.png`) {
		t.Fatalf("/me 未返回 GitHub 身份: %s", w.Body.String())
	}

	// 登录态评论：不带昵称邮箱，后端采用 GitHub 身份
	w2 := doReq(r, http.MethodPost, "/api/posts/%E6%B5%8B/comments",
		`{"content":"GitHub 用户来啦","ts":1,"website":""}`,
		map[string]string{"Cookie": cookie, "Content-Type": "application/json", "X-Real-IP": "9.9.9.9"})
	if w2.Code != http.StatusCreated {
		t.Fatalf("登录态评论应 201，got %d: %s", w2.Code, w2.Body.String())
	}

	// 列表带出头像快照与昵称
	w3 := doReq(r, http.MethodGet, "/api/posts/%E6%B5%8B/comments", "", nil)
	body := w3.Body.String()
	if !strings.Contains(body, `"nickname":"octocat"`) ||
		!strings.Contains(body, `"avatar_url":"http://example.com/a.png"`) ||
		!strings.Contains(body, `"commenter_id":1`) {
		t.Fatalf("列表未带出登录身份字段: %s", body)
	}

	// 退出（清 Cookie）后评论：无会话即按游客校验，缺昵称 → 400
	// （logout 仅为浏览器侧清 Cookie，无状态 JWT 无法服务端吊销；
	//   真实场景下浏览器退出后不会再携带 Cookie，故此处不带 Cookie 模拟）
	w4 := doReq(r, http.MethodPost, "/api/auth/logout", "", map[string]string{"Cookie": cookie})
	if w4.Code != http.StatusOK {
		t.Fatalf("logout 应 200，got %d", w4.Code)
	}
	w5 := doReq(r, http.MethodPost, "/api/posts/%E6%B5%8B/comments",
		`{"content":"退出后无昵称","ts":1,"website":""}`,
		map[string]string{"Content-Type": "application/json", "X-Real-IP": "9.9.9.8"})
	if w5.Code != http.StatusBadRequest {
		t.Fatalf("退出后缺昵称应 400，got %d: %s", w5.Code, w5.Body.String())
	}
}

func TestCommenterTokenCannotAccessAdmin(t *testing.T) {
	r := newTestServer(t)
	// 直接签发一个合法的评论者令牌，尝试访问管理端必须 401
	tok, err := auth.MakeCommenterToken(1, "test-secret")
	if err != nil {
		t.Fatal(err)
	}
	w := doReq(r, http.MethodGet, "/api/admin/posts", "",
		map[string]string{"Authorization": "Bearer " + tok})
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("评论者令牌访问管理端应 401，got %d: %s", w.Code, w.Body.String())
	}
}

func TestReservedNickname(t *testing.T) {
	r := newTestServer(t)
	w := doReq(r, http.MethodPost, "/api/posts/%E6%B5%8B/comments",
		`{"nickname":"博主","content":"冒充博主","ts":1,"website":""}`,
		map[string]string{"Content-Type": "application/json", "X-Real-IP": "9.9.9.7"})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("保留昵称应 400，got %d: %s", w.Code, w.Body.String())
	}
}

func TestSanitizeRedirect(t *testing.T) {
	cases := map[string]string{
		"/post/abc":     "/post/abc",
		"/post/x?y=1":   "/post/x?y=1",
		"":              "/",
		"//evil.com":    "/",
		"https://x.com": "/",
		"/a|b":          "/",
		"/a\\b":         "/",
	}
	for in, want := range cases {
		if got := sanitizeRedirect(in); got != want {
			t.Errorf("sanitizeRedirect(%q) = %q, want %q", in, got, want)
		}
	}
}
