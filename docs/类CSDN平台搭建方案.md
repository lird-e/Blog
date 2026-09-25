# 类 CSDN 内容平台搭建方案（含评论回复功能）

> 目标：在现有纯静态博客（GitHub Pages）基础上，升级成「文章 + 用户 + 评论楼中楼回复 + 管理后台」的内容社区。
> 现状：静态博客已具备文章生成、标签、搜索、RSS；评论此前接入 Giscus（GitHub Discussions）。

---

## 一、需求拆解

| 模块 | 功能 | 现状 |
|------|------|------|
| 内容 | 文章发布/编辑/Markdown/标签/分类/搜索/分页 | ✅ 已有（静态生成） |
| 评论 | 一级评论 + 楼中楼回复（回复 @某人）、点赞、Markdown | ❌ 仅有 Giscus（数据在 GitHub） |
| 用户 | 注册/登录（邮箱验证码）、头像、个人主页 | ❌ 无 |
| 通知 | 被回复/被 @ 邮件 + 站内通知 | ❌ 无 |
| 管理 | 评论审核、敏感词过滤、用户封禁、文章置顶 | ❌ 无 |
| 反垃圾 | 人机验证、频率限制 | ❌ 无 |

## 二、三条技术路线对比

| 维度 | A. 静态 + 第三方评论 | B. 静态 + 自托管评论服务 | C. 全栈自建（推荐最终形态） |
|------|---------------------|------------------------|---------------------------|
| 代表方案 | Waline / Twikoo / Giscus | Artalk（Go 单文件） | Go Gin + React + MySQL |
| 工作量 | 0.5~1 天 | 1~2 天（需服务器） | 2~4 周 |
| 数据归属 | 第三方（LeanCloud/Vercel/GitHub） | 自己服务器 | 完全自有 |
| 用户体系 | 无（游客昵称即可） | 无 | 完整注册登录 |
| 回复功能 | ✅ 楼中楼 | ✅ 楼中楼 | ✅ 楼中楼 + @ + 通知 |
| 管理审核 | 基础 | ✅ 后台审核 | ✅ 完整后台 |
| 月成本 | ¥0 | 云服务器 ¥50~100/年 | 云服务器 ¥50~100/年 |
| 适合阶段 | 立刻见效 | 过渡 | 毕业设计/长期 |

**建议路线：A（或 B）先行 → C 分阶段重构。** 你正在把考勤系统重构到 Go(Gin)+React，技术栈完全复用，C 方案的代码大量可迁移。

## 三、路线 C：全栈自建详细设计

### 3.1 技术栈

| 层 | 选型 | 理由 |
|----|------|------|
| 前端 | React 18 + Vite + TypeScript + Ant Design + Tailwind | 与考勤系统重构栈一致 |
| 后端 | Go + Gin + GORM | 你已确定的方向 |
| 数据库 | MySQL 8 | 生态熟、云服务器自带 |
| 缓存 | Redis（可选二期） | 验证码、限流、热帖缓存 |
| 鉴权 | JWT Access + Refresh Token | 无状态，好扩展 |
| 部署 | Nginx 反代 + HTTPS + Docker Compose | 单机一键起 |

### 3.2 核心数据模型（评论部分）

```sql
users      (id, username, email, password_hash, avatar, role, status, created_at)
posts      (id, author_id, title, slug, content_md, content_html, status, views, created_at, updated_at)
tags       (id, name) / post_tags (post_id, tag_id)
comments   (id, post_id, user_id, parent_id, reply_to_user_id, content, status, like_count, created_at)
           -- parent_id = 0 表示一级评论；>0 表示挂在某一级评论下的回复
           -- reply_to_user_id 用于显示「回复 @xxx」
comment_likes (comment_id, user_id, created_at)          -- 联合主键防重复
notifications (id, user_id, type, actor_id, comment_id, is_read, created_at)
sensitive_words (id, word)
```

查询策略：一级评论分页（按时间/热度倒序）+ 每个一级评论下取前 N 条回复，回复数 >N 显示「展开全部 xx 条回复」——与 CSDN 一致的两级展示。

### 3.3 API 设计（REST）

```
POST   /api/auth/register            注册（邮箱验证码）
POST   /api/auth/login               登录 → access + refresh token
GET    /api/posts?page=&tag=&kw=     文章列表
GET    /api/posts/:slug              文章详情（浏览量 +1）
GET    /api/posts/:id/comments?root_page=&reply_page=   评论树
POST   /api/posts/:id/comments       发评论/回复 {content, parent_id, reply_to_user_id}
POST   /api/comments/:id/like        点赞（再点取消）
GET    /api/notifications            站内通知（未读数角标）
管理端（ROLE_ADMIN，复用考勤系统 RBAC 模式）：
GET    /api/admin/comments?status=pending   审核队列
POST   /api/admin/comments/:id/moderate     通过/删除
GET/POST/DELETE /api/admin/sensitive-words  敏感词管理
```

### 3.4 反垃圾与审核

1. **人机验证**：Cloudflare Turnstile（免费，国内可达性尚可）或极验/腾讯云验证码
2. **频率限制**：Redis 滑动窗口，同一 IP 每分钟 ≤3 条评论（一期可用内存版 map 兜底）
3. **敏感词**：本地 AC 自动机词库（github.com/importcjj/sensitive），命中进入待审
4. **审核流**：评论默认「先发后审」或「先审后发」可配置；管理员后台一键通过/删除/封禁

### 3.5 分阶段实施计划

| 阶段 | 内容 | 产出 |
|------|------|------|
| P0 一期（1~2 天） | 现有静态博客接入 Waline（Vercel Serverless + Supabase） | 立即可用楼中楼回复，零成本验证需求 |
| P1 二期（1 周） | 用户体系 + 文章 CRUD + 评论/回复 API | 最小全栈闭环 |
| P2 三期（1 周） | 通知、点赞、个人主页、管理后台、反垃圾 | 达到 CSDN 基础体验 |
| P3 四期（可选） | 富文本编辑器、全文搜索（Meilisearch）、SEO 静态化 | 体验升级 |

### 3.6 部署与合规

- **服务器**：腾讯云/阿里云轻量应用服务器（学生/新用户 ¥50~100/年）或 2C2G 入门机
- **域名 + ICP 备案**：服务器在国内必须备案（约 2~3 周，学生可办）；免备案方案：Render / Fly.io / 香港节点（延迟略高）
- **HTTPS**：Let's Encrypt + certbot 自动续期
- **数据备份**：MySQL 每日 mysqldump 到对象存储（COS/OSS）

## 四、我的建议

1. **本周先做 P0**：静态博客接 Waline，半小时内就有完整的楼中楼回复，用户体验立刻接近 CSDN 评论区，同时不折腾服务器。
2. **中期走 P1~P2**：作为毕业设计或 portfolio 项目非常合适，且与考勤系统重构共用一套 Go(Gin)+React 工程实践。
3. 若最终目标是找工作展示，建议把「评论回复 + 审核 + 反垃圾」做成完整闭环写进简历，比单纯博客更有含金量。
