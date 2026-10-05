package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/lird-e/Blog/server/internal/auth"
)

// do 发一次请求，返回响应。
func do(t *testing.T, r http.Handler, method, path string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, nil)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// TestListPostsRejectsBadPageSize page_size 是用户可任意控制的参数，
// 归一化缺失时 (total + size - 1) / size 会整数除零 panic；
// 同时响应里的 page_size 必须与实际分页一致。
func TestListPostsRejectsBadPageSize(t *testing.T) {
	r := newTestServer(t)
	for _, q := range []string{"page_size=0", "page_size=abc", "page_size=-5", "page_size=999", "page=0"} {
		w := do(t, r, http.MethodGet, "/api/posts?"+q, nil)
		if w.Code != http.StatusOK {
			t.Fatalf("query %s: status=%d body=%s", q, w.Code, w.Body.String())
		}
		var body struct {
			Page       int   `json:"page"`
			PageSize   int   `json:"page_size"`
			TotalPages int   `json:"total_pages"`
			Items      []any `json:"items"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatalf("query %s: 解析失败 %v", q, err)
		}
		if body.PageSize != 10 {
			t.Errorf("query %s: page_size=%d，期望回落到 10", q, body.PageSize)
		}
		if body.Page < 1 || body.TotalPages < 1 {
			t.Errorf("query %s: page=%d total_pages=%d 越界", q, body.Page, body.TotalPages)
		}
	}
}

// TestGetPostOmitsMarkdownSource 公开详情接口只应下发渲染好的 HTML：
// 带上 content_md 会让文章页体积接近翻倍，读者也用不到。
func TestGetPostOmitsMarkdownSource(t *testing.T) {
	r := newTestServer(t)
	w := do(t, r, http.MethodGet, "/api/posts/测", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if _, ok := body["content_md"]; ok {
		t.Errorf("公开响应不应包含 content_md：%v", body["content_md"])
	}
	if body["content_html"] != "<h1>t</h1>" {
		t.Errorf("content_html 缺失: %v", body["content_html"])
	}
}

// TestPublicCacheHeaders 缓存策略必须逐接口区分：
// 列表可公共短缓存；详情按访客而异只能私有；评论列表与登录态接口不得缓存。
func TestPublicCacheHeaders(t *testing.T) {
	r := newTestServer(t)
	cases := map[string]string{
		"/api/posts":            "public, max-age=20",
		"/api/posts/测":          "private, max-age=5, stale-while-revalidate=300",
		"/api/posts/测/comments": "no-store",
		"/api/me":               "no-store",
	}
	for path, want := range cases {
		w := do(t, r, http.MethodGet, path, nil)
		if got := w.Header().Get("Cache-Control"); got != want {
			t.Errorf("%s: Cache-Control=%q，期望 %q", path, got, want)
		}
	}
}

// TestRobotsDisallowsAdmin 后台不应被搜索引擎收录。
func TestRobotsDisallowsAdmin(t *testing.T) {
	r := newTestServer(t)
	w := do(t, r, http.MethodGet, "/robots.txt", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "Disallow: /admin") {
		t.Errorf("robots.txt 缺少后台屏蔽:\n%s", w.Body.String())
	}
}

// TestAdminDeleteCommentRemovesWholeThread 走 HTTP 链路验证整栋删除，
// 并确认响应回报删除条数（管理员需要知道连带删了几条回复）。
func TestAdminDeleteCommentRemovesWholeThread(t *testing.T) {
	r := newTestServer(t)
	// 评论限流按 IP 计（每分钟 1 条），让测试客户端成为可信代理，
	// 两条评论各用不同 X-Forwarded-For 才能都发出去。
	if err := r.SetTrustedProxies([]string{"192.0.2.1"}); err != nil {
		t.Fatal(err)
	}
	// ts 必须早于 3 秒前，否则命中「提交过快」防垃圾阈值
	ts := time.Now().Add(-time.Minute).UnixMilli()
	post := func(body map[string]any, ip string) int {
		req := httptest.NewRequest(http.MethodPost, "/api/posts/测/comments", strings.NewReader(toJSON(t, body)))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Forwarded-For", ip)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusCreated {
			t.Fatalf("发评论 status=%d body=%s", w.Code, w.Body.String())
		}
		var out struct {
			ID int `json:"id"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
			t.Fatal(err)
		}
		return out.ID
	}
	root := post(map[string]any{"nickname": "甲", "content": "根评论", "ts": ts}, "10.0.0.1")
	child := post(map[string]any{"nickname": "乙", "content": "回复", "parent_id": root, "ts": ts}, "10.0.0.2")

	// 删除前先确认两条都可见，否则后面的断言可能空跑
	w := do(t, r, http.MethodGet, "/api/posts/测/comments", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("评论列表 status=%d body=%s", w.Code, w.Body.String())
	}
	if got := listComments(t, w); len(got) != 2 {
		t.Fatalf("删除前应有 2 条，实得 %d: %v", len(got), got)
	}

	token, err := auth.MakeToken("admin", "test-secret")
	if err != nil {
		t.Fatal(err)
	}
	w = do(t, r, http.MethodDelete, "/api/admin/comments/"+strconv.Itoa(root),
		map[string]string{"Authorization": "Bearer " + token})
	if w.Code != http.StatusOK {
		t.Fatalf("删除 status=%d body=%s", w.Code, w.Body.String())
	}
	var out struct {
		Deleted int `json:"deleted"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.Deleted != 2 {
		t.Errorf("deleted=%d，期望 2（父评论 + 其回复）", out.Deleted)
	}

	w = do(t, r, http.MethodGet, "/api/posts/测/comments", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("整栋删除后列表读取失败 status=%d body=%s", w.Code, w.Body.String())
	}
	if got := listComments(t, w); len(got) != 0 {
		t.Errorf("整栋删除后应剩 0 条，实得 %d: %v", len(got), got)
	}

	// 子评论应已随父一起消失
	w = do(t, r, http.MethodDelete, "/api/admin/comments/"+strconv.Itoa(child),
		map[string]string{"Authorization": "Bearer " + token})
	var again struct {
		Deleted int `json:"deleted"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &again); err != nil {
		t.Fatal(err)
	}
	if again.Deleted != 0 {
		t.Errorf("重复删除回复 deleted=%d，期望 0", again.Deleted)
	}
}

// listComments 解析评论列表响应的 items。
func listComments(t *testing.T, w *httptest.ResponseRecorder) []map[string]any {
	t.Helper()
	var body struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("解析评论列表失败: %v body=%s", err, w.Body.String())
	}
	return body.Items
}

// TestGuestCommentKeepsListReadable 游客评论的 commenter_id 落库为 NULL，
// 曾按 int64 扫描该列，导致读者一发完评论，整篇文章的评论列表持续 500。
func TestGuestCommentKeepsListReadable(t *testing.T) {
	r := newTestServer(t)
	if err := r.SetTrustedProxies([]string{"192.0.2.1"}); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/posts/测/comments", strings.NewReader(toJSON(t, map[string]any{
		"nickname": "游客", "content": "第一条评论", "ts": time.Now().Add(-time.Minute).UnixMilli(),
	})))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Forwarded-For", "10.9.9.9")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("发评论 status=%d body=%s", w.Code, w.Body.String())
	}

	w = do(t, r, http.MethodGet, "/api/posts/测/comments", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("游客评论存在时列表 status=%d body=%s", w.Code, w.Body.String())
	}
	items := listComments(t, w)
	if len(items) != 1 {
		t.Fatalf("items=%d，期望 1: %v", len(items), items)
	}
	if items[0]["nickname"] != "游客" {
		t.Errorf("nickname=%v", items[0]["nickname"])
	}
	if id, ok := items[0]["commenter_id"].(float64); ok && id != 0 {
		t.Errorf("游客评论 commenter_id=%v，期望 0", id)
	}
}

func toJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
