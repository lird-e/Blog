package handlers

import (
	"errors"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/lird-e/Blog/server/internal/auth"
	"github.com/lird-e/Blog/server/internal/db"
	"github.com/lird-e/Blog/server/internal/markdown"
)

// slugRe 与旧静态生成器 build.py 的 slugify 对齐：保留中文、字母、数字、下划线。
var slugRe = regexp.MustCompile(`[^\w\x{4e00}-\x{9fff}]+`)

func slugify(s string) string {
	out := strings.Trim(slugRe.ReplaceAllString(strings.ToLower(s), "-"), "-")
	if out == "" {
		out = "post"
	}
	return out
}

// ---- 登录 ----

// Login POST /api/admin/login —— 失败 5 次锁 15 分钟（方案九）
func (h *Handlers) Login(c *gin.Context) {
	if h.Cfg.AdminPassHash == "" || h.Cfg.JWTSecret == "" {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "管理端未配置（缺少 ADMIN_PASS_HASH / JWT_SECRET），参考 deploy/env.example",
		})
		return
	}
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求格式错误"})
		return
	}
	key := "login:" + c.ClientIP()
	if h.LoginLim.Blocked(key, 5, 15*60*timeSecond) {
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "失败次数过多，请 15 分钟后再试"})
		return
	}
	if req.Username != h.Cfg.AdminUser || !auth.CheckPassword(h.Cfg.AdminPassHash, req.Password) {
		h.LoginLim.Fail(key)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "用户名或密码错误"})
		return
	}
	h.LoginLim.Reset(key)
	token, err := auth.MakeToken(req.Username, h.Cfg.JWTSecret)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "签发令牌失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"token": token, "expires_in": int(auth.TokenTTL.Seconds())})
}

const timeSecond = 1e9 // time.Duration(1s)

// AuthRequired JWT 中间件：校验 Bearer 令牌并把用户名挂到上下文。
func (h *Handlers) AuthRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		token, ok := strings.CutPrefix(header, "Bearer ")
		if !ok || token == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
			return
		}
		user, err := auth.ParseToken(token, h.Cfg.JWTSecret)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
			return
		}
		c.Set("admin_user", user)
		c.Next()
	}
}

// ---- 文章管理 ----

// AdminListPosts GET /api/admin/posts —— 含未发布
func (h *Handlers) AdminListPosts(c *gin.Context) {
	page, pageSize := pageParams(c)
	opts := db.ListOpts{Query: c.Query("q"), Page: page, PageSize: pageSize}
	items, total, err := h.DB.ListPosts(opts)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "读取文章失败"})
		return
	}
	tp := (total + pageSize - 1) / pageSize
	if tp < 1 {
		tp = 1
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "total": total, "page": page, "page_size": pageSize, "total_pages": tp})
}

// AdminGetPost GET /api/admin/posts/:id
func (h *Handlers) AdminGetPost(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	p, err := h.DB.GetByID(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "读取文章失败"})
		return
	}
	if p == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "文章不存在"})
		return
	}
	c.JSON(http.StatusOK, p)
}

type postPayload struct {
	Title     string `json:"title"`
	Slug      string `json:"slug"`
	Excerpt   string `json:"excerpt"`
	Tags      any    `json:"tags"`
	ContentMD string `json:"content_md"`
	Published *bool  `json:"published"`
}

// fillPost 校验并补全文章字段：slug 生成、goldmark 渲染、摘要兜底。
func fillPost(p *db.Post, req postPayload) error {
	req.Title = strings.TrimSpace(req.Title)
	if req.Title == "" {
		return errors.New("标题不能为空")
	}
	if len([]rune(req.ContentMD)) == 0 {
		return errors.New("正文不能为空")
	}
	p.Title = req.Title
	if s := strings.TrimSpace(req.Slug); s != "" {
		p.Slug = slugify(s) // 显式 slug 也统一格式，保证 URL 稳定
	} else {
		p.Slug = slugify(req.Title)
	}
	p.Tags = db.NormalizeTags(req.Tags)
	p.TagsList = db.SplitTags(p.Tags)
	html := markdown.Render(req.ContentMD)
	p.ContentMD = req.ContentMD
	p.ContentHTML = html
	if e := strings.TrimSpace(req.Excerpt); e != "" {
		p.Excerpt = e
	} else {
		p.Excerpt = markdown.Excerpt(html, 120)
	}
	p.Published = req.Published == nil || *req.Published
	return nil
}

// AdminCreatePost POST /api/admin/posts
func (h *Handlers) AdminCreatePost(c *gin.Context) {
	var req postPayload
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求格式错误"})
		return
	}
	p := &db.Post{}
	if err := fillPost(p, req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if existing, _ := h.DB.GetBySlug(p.Slug); existing != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "slug 已存在：" + p.Slug})
		return
	}
	id, err := h.DB.CreatePost(p)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存失败"})
		return
	}
	p.ID = id
	// 若该 slug 曾被其他文章改名让出，现在由新文章接管，移除旧跳转记录
	_ = h.DB.DeleteRedirectFrom(p.Slug)
	c.JSON(http.StatusCreated, p)
}

// AdminUpdatePost PUT /api/admin/posts/:id
func (h *Handlers) AdminUpdatePost(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	old, err := h.DB.GetByID(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "读取文章失败"})
		return
	}
	if old == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "文章不存在"})
		return
	}
	var req postPayload
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求格式错误"})
		return
	}
	p := &db.Post{ID: old.ID, CreatedAt: old.CreatedAt, Views: old.Views}
	if err := fillPost(p, req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	// slug 变更时查重（排除自身）
	if p.Slug != old.Slug {
		if existing, _ := h.DB.GetBySlug(p.Slug); existing != nil && existing.ID != id {
			c.JSON(http.StatusConflict, gin.H{"error": "slug 已存在：" + p.Slug})
			return
		}
	}
	if err := h.DB.UpdatePost(id, p); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存失败"})
		return
	}
	// slug 变更时记录改名跳转，保住 RSS/搜索引擎已收录的旧链接
	if p.Slug != old.Slug {
		_ = h.DB.AddRedirect(old.Slug, p.Slug)
	}
	out, _ := h.DB.GetByID(id)
	c.JSON(http.StatusOK, out)
}

// AdminSetPostPublished PUT /api/admin/posts/:id/published
// 列表页快速切换发布状态：只更新 published 字段，不携带正文，
// 避免全量 PUT 因列表接口不含 content_md 而误伤内容。
func (h *Handlers) AdminSetPostPublished(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var req struct {
		Published *bool `json:"published"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Published == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "published 必须为布尔值"})
		return
	}
	if err := h.DB.SetPostPublished(id, *req.Published); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "操作失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "已更新", "published": *req.Published})
}

// AdminDeletePost DELETE /api/admin/posts/:id —— 评论随外键级联删除
func (h *Handlers) AdminDeletePost(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	old, _ := h.DB.GetByID(id)
	if err := h.DB.DeletePost(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除失败"})
		return
	}
	if old != nil {
		_ = h.DB.DeleteRedirectsTo(old.Slug)
	}
	c.JSON(http.StatusOK, gin.H{"message": "已删除"})
}

// ---- 评论审核 ----

// AdminListComments GET /api/admin/comments?status=
func (h *Handlers) AdminListComments(c *gin.Context) {
	page, pageSize := pageParams(c)
	list, total, err := h.DB.AdminListComments(c.Query("status"), pageSize, (page-1)*pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "读取评论失败"})
		return
	}
	tp := (total + pageSize - 1) / pageSize
	if tp < 1 {
		tp = 1
	}
	c.JSON(http.StatusOK, gin.H{"items": list, "total": total, "page": page, "page_size": pageSize, "total_pages": tp})
}

// AdminSetComment PUT /api/admin/comments/:id —— 通过 / 拒绝
func (h *Handlers) AdminSetComment(c *gin.Context) {
	var req struct {
		Status string `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil ||
		(req.Status != "approved" && req.Status != "pending" && req.Status != "rejected") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "status 必须为 approved / pending / rejected"})
		return
	}
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	if err := h.DB.SetCommentStatus(id, req.Status); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "操作失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "已更新"})
}

// AdminDeleteComment DELETE /api/admin/comments/:id
func (h *Handlers) AdminDeleteComment(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	if err := h.DB.DeleteComment(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "已删除"})
}

// ---- 评论模式（先发后显 / 先审后显）----

// GetSettings GET /api/admin/settings
func (h *Handlers) GetSettings(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"comment_mode": h.DB.GetSetting("comment_mode", "direct")})
}

// UpdateSettings PUT /api/admin/settings
func (h *Handlers) UpdateSettings(c *gin.Context) {
	var req struct {
		CommentMode string `json:"comment_mode"`
	}
	if err := c.ShouldBindJSON(&req); err != nil ||
		(req.CommentMode != "direct" && req.CommentMode != "review") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "comment_mode 必须为 direct（先发后显）或 review（先审后显）"})
		return
	}
	if err := h.DB.SetSetting("comment_mode", req.CommentMode); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"comment_mode": req.CommentMode})
}
