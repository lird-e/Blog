# 项目长期记忆：个人博客

- 技术栈：纯静态站点 + Python 生成器（markdown、pygments），无任何前端框架依赖。
- 写作方式：在 `content/posts/` 下新建 `YYYY-MM-DD-slug.md`，文件顶部用 `---` 写元信息（title/date/tags/excerpt），正文为 Markdown。
- 构建：`.venv/Scripts/python build.py` → 输出到 `public/`。
- 部署：**GitHub Pages + Decap CMS（方案 A）**，仓库 `lird-e/Blog`（公开），地址 `https://lird-e.github.io/Blog`。`.github/workflows/deploy.yml` 在 push main 时自动 build 并部署；`public/` 由 CI 生成、已加入 .gitignore。
- 网页发文：Decap CMS（`admin/`）已集成，登录需 GitHub OAuth App（Client ID 填 `admin/config.yml` 的 `app_id`）。
- 约定：`.venv/`、`__pycache__/`、`public/` 已加入 .gitignore。
