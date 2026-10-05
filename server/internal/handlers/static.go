package handlers

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
)

// StaticSPA 用 Go 直接托管前端构建产物，仅在配置了 BLOG_WEB_DIR 时启用
// （本地整站预览，或没有 Nginx 的兜底部署）。生产环境由 Nginx 服务静态文件。
//
// 命中真实文件就返回它，否则回 index.html —— React Router history 模式需要这个兜底。
func (h *Handlers) StaticSPA() gin.HandlerFunc {
	root := filepath.Clean(h.Cfg.WebDir)
	indexFile := filepath.Join(root, "index.html")
	return func(c *gin.Context) {
		path := c.Request.URL.Path
		if strings.HasPrefix(path, "/api") {
			c.JSON(http.StatusNotFound, gin.H{"error": "接口不存在"})
			return
		}
		// Gin 不做请求路径归一化，".." 会原样出现在 URL.Path 里。
		// 虽然 http.ServeFile 目前也会拒绝含 ".." 的请求路径，但不该把这层安全
		// 寄托在标准库的实现细节上：先补前导斜杠再 Clean，上跳被折叠掉，
		// Join 出的路径必然落在 root 内。
		p := filepath.Join(root, filepath.Clean("/"+path))
		if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
			c.File(p)
			return
		}
		c.File(indexFile)
	}
}
