// Blog API：Go(Gin) + SQLite 博客后端入口。
// 生产环境仅监听 127.0.0.1:8080，由 Nginx 反代对外；本地开发可配 BLOG_WEB_DIR 直接托管前端产物。
package main

import (
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/lird-e/Blog/server/internal/config"
	"github.com/lird-e/Blog/server/internal/db"
	"github.com/lird-e/Blog/server/internal/handlers"
	"github.com/lird-e/Blog/server/internal/limiter"
)

func main() {
	cfg := config.Load()

	database, err := db.Open(cfg.DBPath)
	if err != nil {
		log.Fatalf("打开数据库失败 %s: %v", cfg.DBPath, err)
	}

	h := &handlers.Handlers{
		DB:       database,
		Cfg:      cfg,
		Lim:      limiter.New(),
		LoginLim: limiter.New(),
	}

	if cfg.Debug {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())
	// 仅信任本机 Nginx 反代：只有来自 127.0.0.1 的请求才解析 X-Real-IP /
	// X-Forwarded-For，直连请求伪造代理头无效（防绕过评论限流与登录锁定）
	_ = r.SetTrustedProxies([]string{"127.0.0.1"})

	// ---- SEO ----
	r.GET("/rss.xml", h.RSS)
	r.GET("/sitemap.xml", h.Sitemap)
	r.GET("/robots.txt", h.Robots)

	// ---- 公开 API ----
	api := r.Group("/api")
	{
		api.GET("/posts", h.ListPosts)
		api.GET("/posts/:slug", h.GetPost)
		api.GET("/posts/:slug/comments", h.ListComments)
		api.POST("/posts/:slug/comments", h.CreateComment)
		api.GET("/tags", h.ListTags)
	}

	// ---- 管理 API（JWT 保护，登录除外）----
	admin := r.Group("/api/admin")
	{
		admin.POST("/login", h.Login)
		guarded := admin.Group("")
		guarded.Use(h.AuthRequired())
		{
			guarded.GET("/posts", h.AdminListPosts)
			guarded.GET("/posts/:id", h.AdminGetPost)
			guarded.POST("/posts", h.AdminCreatePost)
			guarded.PUT("/posts/:id", h.AdminUpdatePost)
			guarded.PUT("/posts/:id/published", h.AdminSetPostPublished)
			guarded.DELETE("/posts/:id", h.AdminDeletePost)
			guarded.GET("/comments", h.AdminListComments)
			guarded.PUT("/comments/:id", h.AdminSetComment)
			guarded.DELETE("/comments/:id", h.AdminDeleteComment)
			guarded.GET("/settings", h.GetSettings)
			guarded.PUT("/settings", h.UpdateSettings)
		}
	}

	// ---- 前端静态托管（可选）：配置 BLOG_WEB_DIR 后由 Go 直接服务 SPA，
	//      用于本地整站预览；生产环境交给 Nginx。 ----
	if cfg.WebDir != "" {
		indexFile := filepath.Join(cfg.WebDir, "index.html")
		r.NoRoute(func(c *gin.Context) {
			path := c.Request.URL.Path
			if strings.HasPrefix(path, "/api") {
				c.JSON(http.StatusNotFound, gin.H{"error": "接口不存在"})
				return
			}
			p := filepath.Join(cfg.WebDir, filepath.FromSlash(path))
			if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
				c.File(p)
				return
			}
			c.File(indexFile) // React Router history 模式兜底
		})
	}

	log.Printf("blog-api 已启动: http://%s (db=%s)", cfg.Addr, cfg.DBPath)
	if err := r.Run(cfg.Addr); err != nil {
		log.Fatalf("服务退出: %v", err)
	}
}
