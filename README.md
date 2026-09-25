# 我的博客（前后端分离版）

Go(Gin) + SQLite + React(Vite) 的个人博客，带自建评论区与管理后台，部署于 Ubuntu 云服务器（IP 直连）。

> 架构与部署细节见 `博客重构与云服务器部署方案.md` 与 `deploy/README.md`（服务器部署手册）。

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

管理后台 `http://localhost:5173/admin`：配置 `ADMIN_USER / ADMIN_PASS_HASH / JWT_SECRET` 环境变量后启用，密码哈希用 `go run ./server/cmd/genpass 你的密码` 生成。

## 主要接口

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/api/posts?page=&tag=&q=` | 文章列表（分页/标签/搜索） |
| GET | `/api/posts/:slug` | 文章详情（浏览量自增） |
| GET/POST | `/api/posts/:slug/comments` | 评论列表/发表评论（蜜罐+频率限制+时间阈值） |
| GET | `/api/tags` | 标签云 |
| GET | `/rss.xml` `/sitemap.xml` | 动态 SEO 输出 |
| POST | `/api/admin/login` | 登录（JWT，5 次失败锁 15 分钟） |
| CRUD | `/api/admin/posts` `/api/admin/comments` | 文章管理 / 评论审核（JWT） |

## 发布上线

见 `deploy/README.md`：本地 `git push`（GitHub + Gitee 双推）→ 服务器执行 `/var/blog/deploy.sh`（拉镜像 → 构建 → 重启 systemd）。
