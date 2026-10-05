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
	// 列表响应可短缓存：文章发布/改标签后最多 20 秒生效，
	// 换来首页与标签页翻页时不再每次全量查库。管理端走 /api/admin/posts 不受影响。
	c.Header("Cache-Control", "public, max-age=20")
	page, pageSize := pageParams(c, 10)
	opts := db.ListOpts{
		Tag: strings.TrimSpace(c.Query("tag")), Query: c.Query("q"),
		Page: page, PageSize: pageSize, OnlyPublished: true,
	}
	items, total, err := h.DB.ListPosts(opts)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "读取文章失败"})
		return
	}
	c.JSON(http.StatusOK, listResult(items, total, opts.Page, opts.PageSize))
}

// listResult 组装分页响应。pageSize 必须已由 pageParams 归一为 ≥1，
// 否则这里整数除零会 panic。
func listResult(items []db.Post, total, page, pageSize int) gin.H {
	tp := (total + pageSize - 1) / pageSize
	if tp < 1 {
		tp = 1
	}
	return gin.H{
		"items": items, "total": total,
		"page": page, "page_size": pageSize, "total_pages": tp,
	}
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
	// 公开响应只给渲染好的 HTML：Markdown 原文对读者无用，
	// 带上它会让文章页体积接近翻倍（管理端 /api/admin/posts/:id 才需要原文）。
	p.ContentMD = ""
	// private：响应里的 views 是本次请求按访客 IP 判定的，不能让共享缓存跨访客复用；
	// 只给浏览器 5 秒新鲜度，用于返回/前进时免重复拉正文与免重复写浏览量。
	c.Header("Cache-Control", "private, max-age=5, stale-while-revalidate=300")
	c.JSON(http.StatusOK, p)
}

// ListTags GET /api/tags —— 标签云
func (h *Handlers) ListTags(c *gin.Context) {
	c.Header("Cache-Control", "public, max-age=60")
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
	// 不缓存：用户发完评论会立刻重新拉列表，任何缓存都会让自己的新评论「消失」
	c.Header("Cache-Control", "no-store")
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
// 防垃圾四件套：蜜罐字段、提交时间 <3s 拒绝、频率限制、IP 加盐哈希。
// 身份两种来源（互斥，登录态优先）：
//   - GitHub 登录（commenter_sess Cookie）：自动采用 GitHub 昵称与头像，忽略表单昵称/邮箱；
//   - 游客 + 本机身份记忆：昵称必填，email_hash 直传（MD5(email) 记忆回带）或按邮箱现算。
func (h *Handlers) CreateComment(c *gin.Context) {
	var req struct {
		Nickname  string `json:"nickname"`
		Email     string `json:"email"`
		EmailHash string `json:"email_hash"` // 身份记忆直传（32 位 hex md5），非法值忽略
		Content   string `json:"content"`
		ParentID  int64  `json:"parent_id"`
		Website   string `json:"website"` // 蜜罐：页面上隐藏，人不会填
		TS        int64  `json:"ts"`      // 表单渲染时刻（毫秒），挡无头脚本直发
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
	// 3) 内容校验：正文 1~2000 字；昵称仅游客需要校验（登录身份由 GitHub 提供）
	content := strings.TrimSpace(req.Content)
	if content == "" || len([]rune(content)) > 2000 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "评论内容需为 1~2000 个字符"})
		return
	}
	// GitHub 登录态优先：cookie 有效即采用其昵称与头像快照
	commenter := h.currentCommenter(c)
	var nickname string
	if commenter != nil {
		nickname = commenter.Nickname
	} else {
		nickname = strings.TrimSpace(req.Nickname)
		if nickname == "" || len([]rune(nickname)) > 30 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "昵称需为 1~30 个字符"})
			return
		}
		if nickname == h.DB.GetSetting("blogger_name", "博主") {
			c.JSON(http.StatusBadRequest, gin.H{"error": "该昵称为保留昵称，请换一个"})
			return
		}
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
		PostID:   p.ID,
		ParentID: req.ParentID,
		Nickname: nickname,
		Content:  content,
		IPHash:   hashWithSalt(h.Cfg.IPSalt, ip),
		Status:   status,
	}
	if commenter != nil { // GitHub 登录：身份字段来自 commenters 表，头像以 URL 快照落库
		cm.CommenterID = commenter.ID
		cm.AvatarURL = commenter.AvatarURL
	} else {
		// 邮箱哈希：填了新邮箱按新邮箱算（Cravatar 要求未加盐 MD5）；
		// 未填但带合法的记忆哈希则直采，其余情况视为无头像。
		cm.EmailHash = md5Hex(strings.TrimSpace(req.Email))
		if cm.EmailHash == "" && emailHashRe.MatchString(req.EmailHash) {
			cm.EmailHash = strings.ToLower(req.EmailHash)
		}
	}
	id, err := h.DB.CreateComment(cm)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "评论写入失败"})
		return
	}
	msg := "评论已发布"
	if status == "pending" {
		msg = "评论已提交，等待管理员审核后显示"
	}
	c.JSON(http.StatusCreated, gin.H{
		"message": msg, "status": status,
		"id": id, "email_hash": cm.EmailHash, // 供前端保存身份、滚动定位
	})
}

// ---- 工具 ----

// pageParams 归一分页参数。非法或越界的 page_size 回落到 defaultSize，
// 保证返回值恒 ≥1——调用方会用它做除法，归零会 panic。
func pageParams(c *gin.Context, defaultSize int) (page, pageSize int) {
	page, _ = strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ = strconv.Atoi(c.DefaultQuery("page_size", strconv.Itoa(defaultSize)))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 50 {
		pageSize = defaultSize
	}
	return page, pageSize
}
