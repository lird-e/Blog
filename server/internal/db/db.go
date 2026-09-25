// Package db 封装 SQLite 连接、建表与博客全部数据访问。
// 数据库设计见《博客重构与云服务器部署方案》第五章；
// 额外增加 settings 表，用于持久化评论模式（先发后显 / 先审后显）。
package db

import (
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite" // 纯 Go SQLite 驱动，免 CGO
)

type DB struct{ *sql.DB }

type Post struct {
	ID           int64    `json:"id"`
	Slug         string   `json:"slug"`
	Title        string   `json:"title"`
	Excerpt      string   `json:"excerpt"`
	ContentMD    string   `json:"content_md,omitempty"`
	ContentHTML  string   `json:"content_html,omitempty"`
	Tags         string   `json:"tags,omitempty"`
	TagsList     []string `json:"tags_list"`
	Views        int      `json:"views"`
	Published    bool     `json:"published"`
	CreatedAt    string   `json:"created_at"`
	UpdatedAt    string   `json:"updated_at"`
	CommentCount int      `json:"comment_count,omitempty"`
}

type Comment struct {
	ID        int64  `json:"id"`
	PostID    int64  `json:"post_id"`
	ParentID  int64  `json:"parent_id,omitempty"`
	Nickname  string `json:"nickname"`
	EmailHash string `json:"email_hash,omitempty"`
	Content   string `json:"content"`
	IPHash    string `json:"-"`
	Status    string `json:"status"`
	CreatedAt string `json:"created_at"`
	// 管理端列表额外带出所属文章信息
	PostSlug  string `json:"post_slug,omitempty"`
	PostTitle string `json:"post_title,omitempty"`
}

func Open(path string) (*DB, error) {
	dsn := "file:" + filepath.ToSlash(path) +
		"?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)"
	sqlDB, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	// SQLite 是单文件库，多写会锁库；服务自身单实例，连接池收紧即可
	sqlDB.SetMaxOpenConns(4)
	if err := sqlDB.Ping(); err != nil {
		return nil, err
	}
	d := &DB{sqlDB}
	if err := d.migrate(); err != nil {
		return nil, err
	}
	return d, nil
}

func (d *DB) migrate() error {
	schema := `
CREATE TABLE IF NOT EXISTS posts (
  id           INTEGER PRIMARY KEY AUTOINCREMENT,
  slug         TEXT UNIQUE NOT NULL,
  title        TEXT NOT NULL,
  excerpt      TEXT DEFAULT '',
  content_md   TEXT NOT NULL,
  content_html TEXT NOT NULL,
  tags         TEXT DEFAULT '',
  views        INTEGER DEFAULT 0,
  published    INTEGER DEFAULT 1,
  created_at   TEXT NOT NULL,
  updated_at   TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS comments (
  id         INTEGER PRIMARY KEY AUTOINCREMENT,
  post_id    INTEGER NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
  parent_id  INTEGER REFERENCES comments(id),
  nickname   TEXT NOT NULL,
  email_hash TEXT DEFAULT '',
  content    TEXT NOT NULL,
  ip_hash    TEXT DEFAULT '',
  status     TEXT DEFAULT 'approved',
  created_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS settings (
  key   TEXT PRIMARY KEY,
  value TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS redirects (
  old_slug TEXT PRIMARY KEY,
  new_slug TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_comments_post ON comments(post_id, status, created_at);
CREATE INDEX IF NOT EXISTS idx_posts_created ON posts(created_at DESC);
`
	_, err := d.Exec(schema)
	return err
}

// NormalizeTags 规范化标签：任意分隔输入 → 逗号分隔去空格（入库/查询统一格式）。
func NormalizeTags(raw any) string {
	switch v := raw.(type) {
	case nil:
		return ""
	case []any:
		parts := make([]string, 0, len(v))
		for _, item := range v {
			if s := strings.TrimSpace(fmt.Sprint(item)); s != "" {
				parts = append(parts, s)
			}
		}
		return strings.Join(parts, ",")
	default:
		parts := strings.Split(fmt.Sprint(v), ",")
		out := make([]string, 0, len(parts))
		for _, p := range parts {
			if s := strings.TrimSpace(p); s != "" {
				out = append(out, s)
			}
		}
		return strings.Join(out, ",")
	}
}

// SplitTags 把逗号分隔的标签串拆为列表（供 handlers 组装响应）。
func SplitTags(tags string) []string {
	return splitTags(tags)
}

func splitTags(tags string) []string {
	if strings.TrimSpace(tags) == "" {
		return []string{}
	}
	parts := strings.Split(tags, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if s := strings.TrimSpace(p); s != "" {
			out = append(out, s)
		}
	}
	return out
}

// ---- 查询条件构建 ----

type ListOpts struct {
	Tag           string
	Query         string // 多关键词 AND
	Page          int
	PageSize      int
	OnlyPublished bool
}

func (o *ListOpts) where() (string, []any) {
	conds, args := []string{"1=1"}, []any{}
	if o.OnlyPublished {
		conds = append(conds, "published = 1")
	}
	if o.Tag != "" {
		// tags 存为逗号分隔无空格格式，前后补逗号做精确词匹配
		conds = append(conds, `(','||tags||',') LIKE ?`)
		args = append(args, ","+o.Tag+",")
	}
	if q := strings.TrimSpace(o.Query); q != "" {
		for _, tok := range strings.Fields(q) {
			like := "%" + tok + "%"
			conds = append(conds, "(title LIKE ? OR excerpt LIKE ? OR content_md LIKE ? OR content_html LIKE ?)")
			args = append(args, like, like, like, like)
		}
	}
	return strings.Join(conds, " AND "), args
}

// ListPosts 返回分页文章列表（不含正文，保持响应轻量）。
func (d *DB) ListPosts(o ListOpts) ([]Post, int, error) {
	where, args := o.where()
	var total int
	if err := d.QueryRow("SELECT COUNT(*) FROM posts WHERE "+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	if o.Page < 1 {
		o.Page = 1
	}
	if o.PageSize < 1 || o.PageSize > 50 {
		o.PageSize = 10
	}
	rows, err := d.Query(`
SELECT id, slug, title, excerpt, tags, views, published, created_at, updated_at,
  (SELECT COUNT(*) FROM comments c WHERE c.post_id = posts.id AND c.status = 'approved') AS cc
FROM posts WHERE `+where+`
ORDER BY created_at DESC, id DESC LIMIT ? OFFSET ?`,
		append(args, o.PageSize, (o.Page-1)*o.PageSize)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	list := []Post{}
	for rows.Next() {
		var p Post
		var tags, pub sql.NullString
		if err := rows.Scan(&p.ID, &p.Slug, &p.Title, &p.Excerpt, &tags, &p.Views,
			&pub, &p.CreatedAt, &p.UpdatedAt, &p.CommentCount); err != nil {
			return nil, 0, err
		}
		p.Tags = tags.String
		p.TagsList = splitTags(tags.String)
		p.Published = pub.String == "1"
		list = append(list, p)
	}
	return list, total, rows.Err()
}

func scanPost(scan func(dest ...any) error) (*Post, error) {
	var p Post
	var tags, pub sql.NullString
	if err := scan(&p.ID, &p.Slug, &p.Title, &p.Excerpt, &p.ContentMD, &p.ContentHTML,
		&tags, &p.Views, &pub, &p.CreatedAt, &p.UpdatedAt); err != nil {
		return nil, err
	}
	p.Tags = tags.String
	p.TagsList = splitTags(tags.String)
	p.Published = pub.String == "1"
	return &p, nil
}

const postFullCols = `id, slug, title, excerpt, content_md, content_html, tags, views, published, created_at, updated_at`

// GetBySlug 按别名取文章（含正文）。
func (d *DB) GetBySlug(slug string) (*Post, error) {
	p, err := scanPost(d.QueryRow(`SELECT `+postFullCols+` FROM posts WHERE slug = ?`, slug).Scan)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return p, err
}

// GetByID 管理端编辑回显用。
func (d *DB) GetByID(id int64) (*Post, error) {
	p, err := scanPost(d.QueryRow(`SELECT `+postFullCols+` FROM posts WHERE id = ?`, id).Scan)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return p, err
}

func (d *DB) IncrViews(slug string) {
	_, _ = d.Exec(`UPDATE posts SET views = views + 1 WHERE slug = ?`, slug)
}

// UpsertPost 迁移/创建通用写入：slug 冲突时更新内容但保留 id、views，
// 保证已有评论（外键 post_id）不受影响。
func (d *DB) UpsertPost(p *Post) (int64, error) {
	res, err := d.Exec(`
INSERT INTO posts (slug, title, excerpt, content_md, content_html, tags, published, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(slug) DO UPDATE SET
  title = excluded.title, excerpt = excluded.excerpt,
  content_md = excluded.content_md, content_html = excluded.content_html,
  tags = excluded.tags, published = excluded.published, updated_at = excluded.updated_at`,
		p.Slug, p.Title, p.Excerpt, p.ContentMD, p.ContentHTML, p.Tags,
		boolToInt(p.Published), p.CreatedAt, p.UpdatedAt)
	if err != nil {
		return 0, err
	}
	if id, err := res.LastInsertId(); err == nil && id > 0 {
		return id, nil
	}
	var id int64
	err = d.QueryRow(`SELECT id FROM posts WHERE slug = ?`, p.Slug).Scan(&id)
	return id, err
}

// CreatePost 管理端新建：slug 必须唯一。
func (d *DB) CreatePost(p *Post) (int64, error) {
	now := time.Now().Format(time.RFC3339)
	if p.CreatedAt == "" {
		p.CreatedAt = now
	}
	p.UpdatedAt = now
	return d.UpsertPost(p)
}

func (d *DB) UpdatePost(id int64, p *Post) error {
	p.UpdatedAt = time.Now().Format(time.RFC3339)
	_, err := d.Exec(`
UPDATE posts SET slug = ?, title = ?, excerpt = ?, content_md = ?, content_html = ?,
  tags = ?, published = ?, updated_at = ? WHERE id = ?`,
		p.Slug, p.Title, p.Excerpt, p.ContentMD, p.ContentHTML,
		p.Tags, boolToInt(p.Published), p.UpdatedAt, id)
	return err
}

func (d *DB) DeletePost(id int64) error {
	_, err := d.Exec(`DELETE FROM posts WHERE id = ?`, id)
	return err
}

// SetPostPublished 列表页快速切换发布状态（不触碰正文等其他字段）。
func (d *DB) SetPostPublished(id int64, published bool) error {
	_, err := d.Exec(`UPDATE posts SET published = ? WHERE id = ?`, boolToInt(published), id)
	return err
}

// ---- slug 重定向（改名兼容，保住已收录的旧链接）----

// AddRedirect 记录 old→new 的改名跳转，并把原本指向 old 的条目收敛到 new（避免链式跳转）。
func (d *DB) AddRedirect(oldSlug, newSlug string) error {
	if oldSlug == "" || oldSlug == newSlug {
		return nil
	}
	if _, err := d.Exec(`UPDATE redirects SET new_slug = ? WHERE new_slug = ?`, newSlug, oldSlug); err != nil {
		return err
	}
	if _, err := d.Exec(`
INSERT INTO redirects (old_slug, new_slug) VALUES (?, ?)
ON CONFLICT(old_slug) DO UPDATE SET new_slug = excluded.new_slug`, oldSlug, newSlug); err != nil {
		return err
	}
	// 清理改名往返可能产生的自指向条目
	_, err := d.Exec(`DELETE FROM redirects WHERE old_slug = new_slug`)
	return err
}

// GetRedirect 查询旧 slug 对应的新地址，无记录返回空串。
func (d *DB) GetRedirect(oldSlug string) string {
	var v string
	if err := d.QueryRow(`SELECT new_slug FROM redirects WHERE old_slug = ?`, oldSlug).Scan(&v); err != nil {
		return ""
	}
	return v
}

// DeleteRedirectsTo 文章删除时清理指向它的重定向。
func (d *DB) DeleteRedirectsTo(slug string) error {
	_, err := d.Exec(`DELETE FROM redirects WHERE new_slug = ?`, slug)
	return err
}

// DeleteRedirectFrom 新建文章回收旧 slug 时移除对应跳转记录。
func (d *DB) DeleteRedirectFrom(slug string) error {
	_, err := d.Exec(`DELETE FROM redirects WHERE old_slug = ?`, slug)
	return err
}

// TagCounts 统计已发布文章的标签 → [{name, count}]，按名称排序。
func (d *DB) TagCounts() ([]map[string]any, error) {
	rows, err := d.Query(`SELECT tags FROM posts WHERE published = 1 AND tags != ''`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	counts := map[string]int{}
	var order []string
	for rows.Next() {
		var tags sql.NullString
		if err := rows.Scan(&tags); err != nil {
			return nil, err
		}
		for _, t := range splitTags(tags.String) {
			if _, ok := counts[t]; !ok {
				order = append(order, t)
			}
			counts[t]++
		}
	}
	out := make([]map[string]any, 0, len(counts))
	for _, name := range order {
		out = append(out, map[string]any{"name": name, "count": counts[name]})
	}
	return out, rows.Err()
}

// ---- 评论 ----

func (d *DB) CreateComment(c *Comment) (int64, error) {
	res, err := d.Exec(`
INSERT INTO comments (post_id, parent_id, nickname, email_hash, content, ip_hash, status, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		c.PostID, nullableID(c.ParentID), c.Nickname, c.EmailHash, c.Content, c.IPHash, c.Status,
		time.Now().Format(time.RFC3339))
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func nullableID(id int64) any {
	if id <= 0 {
		return nil
	}
	return id
}

// ListCommentsByPost 游客端：仅已通过评论，按时间正序（前端组装楼中楼）。
func (d *DB) ListCommentsByPost(postID int64) ([]Comment, error) {
	return scanComments(d.Query(`
SELECT id, post_id, parent_id, nickname, email_hash, content, status, created_at
FROM comments WHERE post_id = ? AND status = 'approved' ORDER BY created_at ASC, id ASC`, postID))
}

// AdminListComments 审核列表：按状态筛选（空 = 全部），带所属文章信息。
func (d *DB) AdminListComments(status string, limit, offset int) ([]Comment, int, error) {
	where, args := "1=1", []any{}
	if status != "" {
		where, args = "c.status = ?", []any{status}
	}
	var total int
	if err := d.QueryRow(`SELECT COUNT(*) FROM comments c WHERE `+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := d.Query(`
SELECT c.id, c.post_id, c.parent_id, c.nickname, c.email_hash, c.content, c.status, c.created_at,
       p.slug, p.title
FROM comments c JOIN posts p ON p.id = c.post_id
WHERE `+where+` ORDER BY c.created_at DESC, c.id DESC LIMIT ? OFFSET ?`,
		append(args, limit, offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	list := []Comment{}
	for rows.Next() {
		var c Comment
		var parent, emailHash sql.NullString
		if err := rows.Scan(&c.ID, &c.PostID, &parent, &c.Nickname, &emailHash, &c.Content,
			&c.Status, &c.CreatedAt, &c.PostSlug, &c.PostTitle); err != nil {
			return nil, 0, err
		}
		c.EmailHash = emailHash.String
		fmt.Sscan(parent.String, &c.ParentID)
		list = append(list, c)
	}
	return list, total, rows.Err()
}

func scanComments(rows *sql.Rows, err error) ([]Comment, error) {
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := []Comment{}
	for rows.Next() {
		var c Comment
		var parent, emailHash sql.NullString
		if err := rows.Scan(&c.ID, &c.PostID, &parent, &c.Nickname, &emailHash,
			&c.Content, &c.Status, &c.CreatedAt); err != nil {
			return nil, err
		}
		c.EmailHash = emailHash.String
		fmt.Sscan(parent.String, &c.ParentID)
		list = append(list, c)
	}
	return list, rows.Err()
}

func (d *DB) SetCommentStatus(id int64, status string) error {
	_, err := d.Exec(`UPDATE comments SET status = ? WHERE id = ?`, status, id)
	return err
}

// GetCommentForPost 校验被回复的评论存在且属于同一篇文章。
func (d *DB) GetCommentForPost(commentID, postID int64) (*Comment, error) {
	var c Comment
	err := d.QueryRow(`SELECT id, post_id FROM comments WHERE id = ? AND post_id = ?`,
		commentID, postID).Scan(&c.ID, &c.PostID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (d *DB) DeleteComment(id int64) error {
	_, err := d.Exec(`DELETE FROM comments WHERE id = ?`, id)
	return err
}

// ---- 设置 ----

func (d *DB) GetSetting(key, def string) string {
	var v sql.NullString
	if err := d.QueryRow(`SELECT value FROM settings WHERE key = ?`, key).Scan(&v); err != nil {
		return def
	}
	if v.String == "" {
		return def
	}
	return v.String
}

func (d *DB) SetSetting(key, value string) error {
	_, err := d.Exec(`
INSERT INTO settings (key, value) VALUES (?, ?)
ON CONFLICT(key) DO UPDATE SET value = excluded.value`, key, value)
	return err
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
