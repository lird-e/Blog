// Package handlers 实现全部 HTTP 接口：公开文章/评论、管理端、SEO 动态输出。
package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/lird-e/Blog/server/internal/config"
	"github.com/lird-e/Blog/server/internal/db"
	"github.com/lird-e/Blog/server/internal/limiter"
)

type Handlers struct {
	DB       *db.DB
	Cfg      *config.Config
	Lim      *limiter.Limiter // 评论频率
	LoginLim *limiter.Limiter // 登录失败锁定
}

// ---- 公开：文章 ----

// ListPosts GET /api/posts?page=&tag=&q=
func (h *Handlers) ListPosts(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))
	tag := strings.TrimSpace(c.Query("tag"))
	opts := db.ListOpts{Tag: tag, Query: c.Query("q"), Page: page, PageSize: pageSize, OnlyPublished: true}
	items, total, err := h.DB.ListPosts(opts)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "读取文章失败"})
		return
	}
	tp := (total + opts.PageSize - 1) / opts.PageSize
	if tp < 1 {
		tp = 1
	}
	c.JSON(http.StatusOK, gin.H{
		"items": items, "total": total,
		"page": opts.Page, "page_size": opts.PageSize, "total_pages": tp,
	})
}

// GetPost GET /api/posts/:slug —— 详情 + 浏览量自增（同 IP 每日计 1 次）
func (h *Handlers) GetPost(c *gin.Context) {
	p, err := h.DB.GetBySlug(c.Param("slug"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "读取文章失败"})
		return
	}
	if p == nil || !p.Published {
		// slug 改名兼容：命中重定向表则告知前端跳转到新地址
		if target := h.DB.GetRedirect(c.Param("slug")); target != "" && target != c.Param("slug") {
			c.JSON(http.StatusOK, gin.H{"redirect_to": target})
			return
		}
		c.JSON(http.StatusNotFound, gin.H{"error": "文章不存在"})
		return
	}
	// 浏览量去重：同 IP 同文章每日计 1 次，返回自增后的值
	if h.Lim.Allow("view:"+p.Slug+":"+c.ClientIP(), 1, timeDay) {
		h.DB.IncrViews(p.Slug)
		p.Views++
	}
	c.JSON(http.StatusOK, p)
}

// ListTags GET /api/tags —— 标签云
func (h *Handlers) ListTags(c *gin.Context) {
	tags, err := h.DB.TagCounts()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "读取标签失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"tags": tags})
}

// ---- 公开：评论 ----

// ListComments GET /api/posts/:slug/comments —— 仅已通过，时间正序，前端组楼中楼
func (h *Handlers) ListComments(c *gin.Context) {
	p, err := h.DB.GetBySlug(c.Param("slug"))
	if err != nil || p == nil || !p.Published {
		c.JSON(http.StatusNotFound, gin.H{"error": "文章不存在"})
		return
	}
	list, err := h.DB.ListCommentsByPost(p.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "读取评论失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": list})
}

// CreateComment POST /api/posts/:slug/comments
// 防垃圾四件套（方案 4.3）：蜜罐字段、提交时间 <3s 拒绝、频率限制、IP 加盐哈希。
func (h *Handlers) CreateComment(c *gin.Context) {
	var req struct {
		Nickname string `json:"nickname"`
		Email    string `json:"email"`
		Content  string `json:"content"`
		ParentID int64  `json:"parent_id"`
		Website  string `json:"website"` // 蜜罐：页面上隐藏，人不会填
		TS       int64  `json:"ts"`      // 表单渲染时刻（毫秒），挡无头脚本直发
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求格式错误"})
		return
	}
	// 1) 蜜罐命中：直接拒绝
	if req.Website != "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "提交无效"})
		return
	}
	// 2) 提交时间 < 3 秒拒绝（ts 来自浏览器 Date.now()，UTC 毫秒，无时区问题；
	//    ts 在未来同样视为异常）
	if req.TS <= 0 || timeNowMilli()-req.TS < 3000 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "提交过快，请稍后再试"})
		return
	}
	// 3) 内容校验：昵称 1~30 字、正文 1~2000 字
	nickname := strings.TrimSpace(req.Nickname)
	content := strings.TrimSpace(req.Content)
	if nickname == "" || len([]rune(nickname)) > 30 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "昵称需为 1~30 个字符"})
		return
	}
	if content == "" || len([]rune(content)) > 2000 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "评论内容需为 1~2000 个字符"})
		return
	}
	ip := c.ClientIP()
	// 4) 频率限制：同 IP 每分钟 1 条、每日 20 条
	if !h.Lim.Allow("cm:"+ip, 1, timeMinute) {
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "评论太频繁，请稍后再试"})
		return
	}
	if !h.Lim.Allow("cd:"+ip, 20, timeDay) {
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "今日评论数已达上限"})
		return
	}
	p, err := h.DB.GetBySlug(c.Param("slug"))
	if err != nil || p == nil || !p.Published {
		c.JSON(http.StatusNotFound, gin.H{"error": "文章不存在"})
		return
	}
	if req.ParentID > 0 {
		parent, err := h.DB.GetCommentForPost(req.ParentID, p.ID)
		if err != nil || parent == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "回复的评论不存在"})
			return
		}
	}
	status := "approved"
	if h.DB.GetSetting("comment_mode", "direct") == "review" {
		status = "pending"
	}
	cm := &db.Comment{
		PostID:    p.ID,
		ParentID:  req.ParentID,
		Nickname:  nickname,
		EmailHash: hashWithSalt(h.Cfg.IPSalt, strings.TrimSpace(req.Email)),
		Content:   content,
		IPHash:    hashWithSalt(h.Cfg.IPSalt, ip),
		Status:    status,
	}
	if _, err := h.DB.CreateComment(cm); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "评论写入失败"})
		return
	}
	msg := "评论已发布"
	if status == "pending" {
		msg = "评论已提交，等待管理员审核后显示"
	}
	c.JSON(http.StatusCreated, gin.H{"message": msg, "status": status})
}

// ---- 工具 ----

func pageParams(c *gin.Context) (page, pageSize int) {
	page, _ = strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ = strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 50 {
		pageSize = 20
	}
	return
}
