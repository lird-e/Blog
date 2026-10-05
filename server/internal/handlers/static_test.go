package handlers

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/lird-e/Blog/server/internal/config"
)

// TestStaticSPABlocksPathTraversal 手写 ".." 的请求路径不能读到 WebDir 之外的文件。
// 拼接前先把路径 Clean 成绝对形式，这一层自己就保证落在 root 内，
// 不依赖 http.ServeFile 也会拒绝 ".." 这一实现细节。
func TestStaticSPABlocksPathTraversal(t *testing.T) {
	gin.SetMode(gin.TestMode)
	dir := t.TempDir()
	root := filepath.Join(dir, "web")
	if err := os.MkdirAll(filepath.Join(root, "assets"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "index.html"), []byte("INDEX"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "assets", "app.js"), []byte("APP"), 0o644); err != nil {
		t.Fatal(err)
	}
	secret := filepath.Join(dir, "secret.txt")
	if err := os.WriteFile(secret, []byte("SECRET"), 0o644); err != nil {
		t.Fatal(err)
	}

	h := &Handlers{Cfg: &config.Config{WebDir: root}}
	r := gin.New()
	r.NoRoute(h.StaticSPA())

	get := func(target string) (int, string) {
		req := httptest.NewRequest(http.MethodGet, target, nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w.Code, w.Body.String()
	}

	// 穿越尝试：绝不能读到 root 之外的文件。
	// （http.ServeFile 自身会拒绝含 ".." 的请求路径并回 400，这里不再依赖它——
	//  拼接前先 Clean 才是这一层该有的保证。）
	for _, target := range []string{
		"/../secret.txt",
		"/assets/../../secret.txt",
		"/%2e%2e/secret.txt",
		"/..%2fsecret.txt",
	} {
		code, body := get(target)
		if strings.Contains(body, "SECRET") {
			t.Errorf("%s: 穿越读到了 WebDir 之外的文件 status=%d body=%q", target, code, body)
		}
	}

	if code, body := get("/assets/app.js"); code != http.StatusOK || body != "APP" {
		t.Errorf("真实静态文件: status=%d body=%q，期望 200 且原样返回", code, body)
	}
	if code, body := get("/post/some-slug"); code != http.StatusOK || body != "INDEX" {
		t.Errorf("SPA 兜底: status=%d body=%q，期望 200 且返回 index.html", code, body)
	}
	if code, _ := get("/api/nope"); code != http.StatusNotFound {
		t.Errorf("未注册的 API 路径: status=%d，期望 404", code)
	}
}
