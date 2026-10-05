// Blog API：Go(Gin) + SQLite 博客后端入口。
// 生产环境仅监听 127.0.0.1:8080，由 Nginx 反代对外；本地开发可配 BLOG_WEB_DIR 直接托管前端产物。
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

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

	warnInsecureDefaults(cfg)

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

	// ---- 运维探针：供 systemd 部署后自检与外部监控拨测 ----
	r.GET("/healthz", func(c *gin.Context) {
		if err := database.Ping(); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"ok": false, "error": "db 不可用"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

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

		// GitHub OAuth 评论者登录（未配置凭据时接口返回 503 提示）
		api.GET("/auth/github/login", h.GitHubLogin)
		api.GET("/auth/github/callback", h.GitHubCallback)
		api.POST("/auth/logout", h.Logout)
		api.GET("/me", h.Me)
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
		r.NoRoute(h.StaticSPA())
	}

	srv := &http.Server{
		Addr:    cfg.Addr,
		Handler: r,
		// 显式超时：默认 http.Server 无任何时限，慢客户端能长期占住连接（Slowloris）
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       20 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	go func() {
		log.Printf("blog-api 已启动: http://%s (db=%s)", cfg.Addr, cfg.DBPath)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("服务退出: %v", err)
		}
	}()

	// 优雅停机：systemd stop/restart 发 SIGTERM，等在途请求做完再关库，
	// 避免 deploy.sh 重启时切断正在写评论/上传文章的请求。
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit
	log.Println("收到退出信号，开始优雅停机…")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("停机等待超时: %v", err)
	}
	if err := database.Close(); err != nil {
		log.Printf("关闭数据库: %v", err)
	}
	log.Println("已退出")
}

// warnInsecureDefaults 生产模式下提示仍在使用模板默认值的敏感配置。
// 只告警不退出：本地开发（BLOG_DEBUG=1）本就依赖这些默认值跑通。
func warnInsecureDefaults(cfg *config.Config) {
	if cfg.Debug {
		return
	}
	if cfg.IPSalt == "" || cfg.IPSalt == config.DefaultIPSalt {
		log.Printf("[警告] IP_SALT 未配置，评论 IP 哈希使用了公开的默认盐值，" +
			"任何人拿默认盐都能反查 IP。请在 .env 里设为随机串：openssl rand -hex 32（改动后历史哈希不再关联，可接受）")
	}
	if cfg.JWTSecret == "" {
		log.Printf("[警告] JWT_SECRET 未配置：管理端与 GitHub 登录均返回 503")
	}
	if cfg.AdminPassHash == "" {
		log.Printf("[警告] ADMIN_PASS_HASH 未配置：管理端登录已禁用")
	}
}
