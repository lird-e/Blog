# 项目长期记忆：个人博客

- 技术栈：纯静态站点 + Python 生成器（markdown、pygments），无任何前端框架依赖。
- 写作方式：在 `content/posts/` 下新建 `YYYY-MM-DD-slug.md`，文件顶部用 `---` 写元信息（title/date/tags/excerpt），正文为 Markdown。
- 构建：`.venv/Scripts/python build.py` → 输出到 `public/`；依赖清单统一在 `requirements.txt`（markdown/pygments/pyyaml）。
- 文章排序：**按 frontmatter 的 date 字段降序**（build.py 中 posts.sort），不要按文件名排序（serve.py/Decap 生成的文件名可能无日期前缀）。
- 部署：**GitHub Pages + Decap CMS（方案 A）**，仓库 `lird-e/Blog`（公开），地址 `https://lird-e.github.io/Blog`。`.github/workflows/deploy.yml` 在 push main 时自动 build 并部署；`public/` 由 CI 生成、已加入 .gitignore。
- 网页发文：Decap CMS（`admin/`）已集成，登录需 GitHub OAuth App（Client ID 填 `admin/config.yml` 的 `app_id`）。图片上传 public_folder 已设为 `/Blog/assets/uploads`（子路径部署必须带前缀）。
- 本地发文：`publish.bat` 会先 build 再启动 serve.py（`/admin` 本地表单，无认证，仅限本机/局域网使用）。
- 约定：`.venv/`、`__pycache__/`、`public/` 已加入 .gitignore。
- 本机环境坑：系统代理 HTTP(S)_PROXY=127.0.0.1:7897 可能未运行，pip index-url 残缺 → 装依赖时用 `HTTP_PROXY= HTTPS_PROXY= pip install -r requirements.txt --index-url https://pypi.tuna.tsinghua.edu.cn/simple`。
