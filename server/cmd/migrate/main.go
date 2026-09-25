// cmd/migrate 一次性迁移脚本：把 content/ 下的 Markdown 原文导入 SQLite。
// 用法（仓库根目录执行）:
//
//	go run ./server/cmd/migrate -content content -db blog.db
//
// 幂等：以 slug 冲突更新内容，不动 id 与浏览量，可重复执行。
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/lird-e/Blog/server/internal/db"
	"github.com/lird-e/Blog/server/internal/markdown"
)

var (
	fmRe   = regexp.MustCompile(`(?s)^---\s*\n(.*?)\n---\s*\n?(.*)$`)
	slugRe = regexp.MustCompile(`[^\w\x{4e00}-\x{9fff}]+`)
	// 旧站部署在 GitHub Pages 子路径 /Blog 下，部分图片写成 /Blog/assets/...，
	// 新站部署在根路径，统一去掉前缀（相对路径 assets/... 在 /post/:slug 下
	// 会解析为 /assets/...，Nginx 直接命中前端产物里的同名文件）
	oldPrefixRe = regexp.MustCompile(`(?m)(src=["']|href=["']|\()/Blog/assets/`)
)

func slugify(s string) string {
	out := strings.Trim(slugRe.ReplaceAllString(strings.ToLower(s), "-"), "-")
	if out == "" {
		out = "post"
	}
	return out
}

// parseFrontmatter 与 build.py 的解析行为对齐：键转小写、空值归空串、
// 标签列表/逗号串统一交给 db.NormalizeTags。
func parseFrontmatter(text string) (map[string]string, string) {
	m := fmRe.FindStringSubmatch(text)
	meta := map[string]string{}
	body := text
	if m != nil {
		body = m[2]
		var raw map[string]any
		if err := yaml.Unmarshal([]byte(m[1]), &raw); err == nil {
			for k, v := range raw {
				key := strings.ToLower(k)
				switch val := v.(type) {
				case nil:
					meta[key] = ""
				case []any:
					meta[key] = db.NormalizeTags(val)
				default:
					s := fmt.Sprint(val)
					if t, ok := val.(string); ok {
						s = t
					}
					meta[key] = strings.TrimSpace(s)
				}
			}
		} else {
			log.Printf("⚠ frontmatter 解析失败: %v", err)
		}
	}
	return meta, body
}

// normalizeDate 把 YAML 日期（可能是 "2026-08-18" / "2026-08-18 00:00:00 +0800 CST"
// 等形式）归一化为 YYYY-MM-DD，仅用于 date 字段。
func normalizeDate(s string) string {
	s = strings.TrimSpace(s)
	for _, layout := range []string{"2006-01-02 15:04:05 -0700 MST", "2006-01-02 15:04:05", "2006-01-02"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.Format("2006-01-02")
		}
	}
	if len(s) >= 10 {
		return s[:10]
	}
	return s
}

func countPosts(database *db.DB) (int, error) {
	var n int
	err := database.QueryRow(`SELECT COUNT(*) FROM posts`).Scan(&n)
	return n, err
}

// ---- 文章加载 ----

type loadedPost struct {
	slug, title, tags, excerpt, date, md string
}

func loadPosts(contentDir string) ([]loadedPost, error) {
	postsDir := filepath.Join(contentDir, "posts")
	entries, err := os.ReadDir(postsDir)
	if err != nil {
		return nil, fmt.Errorf("读取 %s 失败: %w", postsDir, err)
	}
	var out []loadedPost
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(postsDir, e.Name()))
		if err != nil {
			return nil, err
		}
		meta, body := parseFrontmatter(string(raw))
		title := meta["title"]
		if title == "" {
			title = strings.TrimSuffix(e.Name(), ".md")
		}
		slug := strings.TrimSpace(meta["slug"])
		if slug == "" {
			slug = slugify(title) // 与 build.py 一致：显式 slug 优先，否则回退标题
		}
		out = append(out, loadedPost{
			slug:    slug,
			title:   title,
			tags:    meta["tags"],
			excerpt: strings.TrimSpace(meta["excerpt"]),
			date:    normalizeDate(meta["date"]),
			md:      oldPrefixRe.ReplaceAllString(body, "${1}assets/"),
		})
	}
	// 稳定输出顺序，便于重复执行与日志核对
	sort.Slice(out, func(i, j int) bool { return out[i].slug < out[j].slug })
	// slug 去重（与 build.py 一致）：同 slug 的文章自动加序号，避免互相覆盖
	used := map[string]bool{}
	for i := range out {
		s := out[i].slug
		if !used[s] {
			used[s] = true
			continue
		}
		n := 2
		for used[fmt.Sprintf("%s-%d", s, n)] {
			n++
		}
		out[i].slug = fmt.Sprintf("%s-%d", s, n)
		used[out[i].slug] = true
	}
	return out, nil
}

func main() {
	contentDir := flag.String("content", "content", "content 目录路径（含 posts/ 与 about.md）")
	dbPath := flag.String("db", "blog.db", "SQLite 数据库文件路径")
	flag.Parse()

	database, err := db.Open(*dbPath)
	if err != nil {
		log.Fatalf("打开数据库失败: %v", err)
	}

	posts, err := loadPosts(*contentDir)
	if err != nil {
		log.Fatalf("%v", err)
	}
	for _, p := range posts {
		html := markdown.Render(p.md)
		excerpt := p.excerpt
		if excerpt == "" {
			excerpt = markdown.Excerpt(html, 120)
		}
		date := p.date
		if date == "" {
			log.Printf("⚠ 跳过 %s：缺少 date 字段", p.slug)
			continue
		}
		post := &db.Post{
			Slug:        p.slug,
			Title:       p.title,
			Excerpt:     excerpt,
			ContentMD:   p.md,
			ContentHTML: html,
			Tags:        p.tags,
			Published:   true,
			CreatedAt:   date,
			UpdatedAt:   date,
		}
		if _, err := database.UpsertPost(post); err != nil {
			log.Fatalf("写入 %s 失败: %v", p.slug, err)
		}
	}
	fmt.Printf("✅ 文章迁移完成：%d 篇\n", len(posts))

	// about.md → posts 表 slug=about，/about 页面走同一条数据链路
	aboutPath := filepath.Join(*contentDir, "about.md")
	if raw, err := os.ReadFile(aboutPath); err == nil {
		meta, body := parseFrontmatter(string(raw))
		title := meta["title"]
		if title == "" {
			title = "关于"
		}
		body = oldPrefixRe.ReplaceAllString(body, "${1}assets/")
		html := markdown.Render(body)
		post := &db.Post{
			Slug:        "about",
			Title:       title,
			ContentMD:   body,
			ContentHTML: html,
			Published:   true,
			CreatedAt:   "1970-01-01",
			UpdatedAt:   "1970-01-01",
		}
		if _, err := database.UpsertPost(post); err != nil {
			log.Fatalf("写入 about 失败: %v", err)
		}
		fmt.Println("✅ 关于页迁移完成")
	}

	total, err := countPosts(database)
	if err != nil {
		log.Fatalf("统计失败: %v", err)
	}
	fmt.Printf("数据库 %s 当前共 %d 篇文章\n", *dbPath, total)
}
