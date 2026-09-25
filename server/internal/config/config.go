// Package config 从环境变量加载服务配置。
// 生产环境通过 systemd EnvironmentFile(/var/blog/.env) 注入；
// 本地开发全部走默认值即可跑通公开接口。
package config

import "os"

type Config struct {
	Addr          string // 监听地址，生产固定 127.0.0.1:8080，仅 Nginx 对外
	DBPath        string // SQLite 数据库文件路径
	AdminUser     string // 管理员用户名
	AdminPassHash string // bcrypt 哈希，由 cmd/genpass 生成
	JWTSecret     string // JWT 签名密钥
	IPSalt        string // 评论 IP / 邮箱加盐哈希的盐
	SiteURL       string // 站点绝对地址（RSS/sitemap/canonical）
	SiteTitle     string
	SiteDesc      string
	WebDir        string // 前端构建产物目录；非空时由 Go 直接托管（本地预览/无 Nginx 兜底）
	Debug         bool   // gin 调试模式
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func Load() *Config {
	return &Config{
		Addr:          getenv("BLOG_ADDR", "127.0.0.1:8080"),
		DBPath:        getenv("BLOG_DB", "blog.db"),
		AdminUser:     getenv("ADMIN_USER", "admin"),
		AdminPassHash: os.Getenv("ADMIN_PASS_HASH"),
		JWTSecret:     os.Getenv("JWT_SECRET"),
		IPSalt:        getenv("IP_SALT", "blog-ip-salt"),
		SiteURL:       getenv("SITE_URL", "http://127.0.0.1:8080"),
		SiteTitle:     getenv("SITE_TITLE", "我的博客"),
		SiteDesc:      getenv("SITE_DESC", "记录技术、思考与生活。"),
		WebDir:        os.Getenv("BLOG_WEB_DIR"),
		Debug:         os.Getenv("BLOG_DEBUG") == "1",
	}
}
