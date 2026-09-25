package handlers

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/lird-e/Blog/server/internal/db"
)

// 动态生成 RSS / sitemap / robots，替代旧静态构建产物，
// 域名/IP 变化时只需改 SITE_URL 环境变量。

func escXML(s string) string {
	var b []byte
	for _, r := range s {
		switch r {
		case '&':
			b = append(b, "&amp;"...)
		case '<':
			b = append(b, "&lt;"...)
		case '>':
			b = append(b, "&gt;"...)
		case '"':
			b = append(b, "&quot;"...)
		case '\'':
			b = append(b, "&apos;"...)
		default:
			b = append(b, string(r)...)
		}
	}
	return string(b)
}

// absURL 站内路径 → 绝对 URL；中文 slug 做 percent 编码（与旧 sitemap 行为一致）。
func absURL(site, path string) string {
	u, err := url.Parse(site)
	if err != nil {
		return site + path
	}
	rel, _ := url.Parse(path)
	u = u.ResolveReference(rel)
	return u.String()
}

// RSS GET /rss.xml —— 最新 20 篇已发布文章
func (h *Handlers) RSS(c *gin.Context) {
	items, _, err := h.DB.ListPosts(db.ListOpts{Page: 1, PageSize: 20, OnlyPublished: true})
	if err != nil {
		c.String(http.StatusInternalServerError, "rss error")
		return
	}
	var b []byte
	b = append(b, `<?xml version="1.0" encoding="UTF-8"?>`+"\n<rss version=\"2.0\">\n<channel>\n"...)
	b = append(b, fmt.Sprintf("<title>%s</title>\n<link>%s</link>\n<description>%s</description>\n<language>zh-CN</language>\n",
		escXML(h.Cfg.SiteTitle), escXML(h.Cfg.SiteURL), escXML(h.Cfg.SiteDesc))...)
	for _, p := range items {
		link := absURL(h.Cfg.SiteURL, "/post/"+p.Slug)
		pubDate := rssDate(p.CreatedAt)
		b = append(b, "<item>\n"...)
		b = append(b, fmt.Sprintf("<title>%s</title>\n<link>%s</link>\n<guid>%s</guid>\n<pubDate>%s</pubDate>\n<description>%s</description>\n",
			escXML(p.Title), escXML(link), escXML(link), pubDate, escXML(p.Excerpt))...)
		b = append(b, "</item>\n"...)
	}
	b = append(b, "</channel>\n</rss>\n"...)
	c.Data(http.StatusOK, "application/rss+xml; charset=utf-8", b)
}

func rssDate(date string) string {
	for _, layout := range []string{time.RFC3339, "2006-01-02", "2006-01-02T15:04:05"} {
		if t, err := time.Parse(layout, date); err == nil {
			return t.UTC().Format("Mon, 02 Jan 2006 15:04:05 +0000")
		}
	}
	return ""
}

// Sitemap GET /sitemap.xml
func (h *Handlers) Sitemap(c *gin.Context) {
	const size = 50
	items, total, err := h.DB.ListPosts(db.ListOpts{Page: 1, PageSize: size, OnlyPublished: true})
	if err != nil {
		c.String(http.StatusInternalServerError, "sitemap error")
		return
	}
	// 按总数翻页取全，避免硬编码页数导致超出部分漏收录
	for page := 2; page <= (total+size-1)/size; page++ {
		more, _, err := h.DB.ListPosts(db.ListOpts{Page: page, PageSize: size, OnlyPublished: true})
		if err != nil {
			break
		}
		items = append(items, more...)
	}
	type urlEntry struct {
		loc, lastmod string
	}
	entries := []urlEntry{{loc: absURL(h.Cfg.SiteURL, "/")}}
	for _, p := range items {
		entries = append(entries, urlEntry{
			loc:     absURL(h.Cfg.SiteURL, "/post/"+p.Slug),
			lastmod: p.UpdatedAt,
		})
	}
	entries = append(entries, urlEntry{loc: absURL(h.Cfg.SiteURL, "/tags")})
	entries = append(entries, urlEntry{loc: absURL(h.Cfg.SiteURL, "/about")})

	b := []byte("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<urlset xmlns=\"http://www.sitemaps.org/schemas/sitemap/0.9\">\n")
	for _, e := range entries {
		lm := ""
		if t, err := time.Parse(time.RFC3339, e.lastmod); err == nil {
			lm = "<lastmod>" + t.Format("2006-01-02") + "</lastmod>"
		}
		b = append(b, fmt.Sprintf("<url><loc>%s</loc>%s</url>\n", escXML(e.loc), lm)...)
	}
	b = append(b, "</urlset>\n"...)
	c.Data(http.StatusOK, "application/xml; charset=utf-8", b)
}

// Robots GET /robots.txt
func (h *Handlers) Robots(c *gin.Context) {
	site := strings.TrimRight(h.Cfg.SiteURL, "/")
	c.String(http.StatusOK, "User-agent: *\nAllow: /\n\nSitemap: %s/sitemap.xml\n", site)
}
