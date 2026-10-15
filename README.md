# 我的博客（前后端分离版）

Go(Gin) + SQLite + React(Vite) 的个人博客，带自建评论区（支持 GitHub OAuth 登录）与管理后台，部署于 Ubuntu 云服务器（IP 直连）。

> 架构设计见 `docs/类CSDN平台搭建方案.md`，服务器部署步骤见 `deploy/README.md`（部署手册）。

## 目录结构

```
├── server/            # Go 后端（Gin + modernc.org/sqlite + goldmark）
│   ├── cmd/server/    #   API 服务入口
│   ├── cmd/migrate/   #   content/*.md → SQLite 一次性迁移脚本（幂等）
│   ├── cmd/genpass/   #   管理员密码 bcrypt 哈希生成器
│   └── internal/      #   config / db / markdown / auth / limiter / handlers
├── web/               # React 前端（Vite + React Router）
│   └── src/pages/     #   列表/详情/标签/搜索/关于/评论区 + /admin 管理后台
├── deploy/            # Nginx 配置、systemd 服务、deploy.sh、env 模板、部署手册
├── content/           # Markdown 原文（迁移源与备份）
└── data/              # 本地开发用 SQLite（已 gitignore）
```

## 本地开发

```bash
# 1. 后端（默认 127.0.0.1:8080；BLOG_WEB_DIR 指向前端产物即可整站预览）
cd server
go run ./cmd/migrate -content ../content -db ../data/blog.db   # 首次迁移文章
BLOG_DB=../data/blog.db BLOG_WEB_DIR=../web/dist go run ./cmd/server

# 2. 前端（开发模式带 /api 代理，另开终端）
cd web
npm install
npm run dev          # http://localhost:5173
npm run build        # 产物 dist/，构建前自动同步博客图片到 public/assets
```

管理后台 `http://localhost:5173/admin`：配置 `ADMIN_USER / ADMIN_PASS_HASH / JWT_SECRET` 环境变量后启用，密码哈希用 `cd server && go run ./cmd/genpass 你的密码` 生成（生产环境变量参考 `deploy/env.example`）。

## 主要接口

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/api/posts?page=&tag=&q=` | 文章列表（分页/标签/搜索） |
| GET | `/api/posts/:slug` | 文章详情（浏览量自增，仅返回渲染后的 HTML） |
| GET/POST | `/api/posts/:slug/comments` | 评论列表/发表评论（蜜罐+频率限制+时间阈值） |
| GET | `/api/tags` | 标签云 |
| GET/POST | `/api/auth/github/*` `/api/auth/logout` `/api/me` | GitHub OAuth 评论者登录/登出（未配置凭据时返回 503） |
| GET | `/rss.xml` `/sitemap.xml` `/robots.txt` | 动态 SEO 输出 |
| GET | `/healthz` | 健康检查（含数据库 ping），供 deploy.sh 与监控使用 |
| POST | `/api/admin/login` | 登录（JWT，5 次失败锁 15 分钟） |
| CRUD | `/api/admin/posts` `/api/admin/comments` | 文章管理 / 评论审核（JWT，删除评论会连带删除其整栋回复） |
| GET/PUT | `/api/admin/settings` | 站点设置（评论模式：先发后显 / 先审后显） |

## 发布上线

见 `deploy/README.md`：本地 `git push`（GitHub 单远端 `lird-e/Blog`）→ 服务器执行 `/var/blog/deploy.sh`（git pull → go build → npm build → 重启 systemd）。
