# 博客项目优化审查报告

> 审查日期：2026-10-14 · 范围：server/（Go 后端）、web/（React 前端）、deploy/（部署配置）、构建链路
> 基线：`go vet ./...` 干净，`go test ./...` 全部通过（db / handlers / markdown 3 个包）。

## 总评

项目整体质量很高：后端有防垃圾四件套、限流、优雅停机、显式超时、SQL 全参数化、JWT 双身份隔离；前端有代码分割、懒加载、防抖 + AbortController、主题预置；部署有健康检查、原子替换、缓存策略。**没有 P0 级问题**。以下按优先级列出可优化项。

## 问题总览

| 编号 | 优先级 | 问题 | 位置 |
|------|--------|------|------|
| 1 | **P1** | 数据库无备份机制，评论/浏览量数据仅存服务器单份 | 运维层面 |
| 2 | **P1** | 非 JS 环境分享文章页拿不到文章级标题/摘要（微信/微博/QQ 卡片） | `web/index.html` / `deploy/nginx-blog.conf` |
| 3 | P2 | 编辑器无草稿自动保存，崩溃场景丢稿 | `web/src/pages/admin/PostEditor.jsx` |
| 4 | P2 | 旧静态版遗留死资产每次构建被打进产物 | `assets/style.css`、`assets/favicon.svg` |
| 5 | P2 | `/assets/` 下不存在的文件返回 index.html（200） | `deploy/nginx-blog.conf:42-45` |
| 6 | P2 | 缺少 CSP / Permissions-Policy 安全头 | `deploy/nginx-blog.conf:17-19` |
| 7 | P3 | gzip 类型清单漏了 sitemap 的 `application/xml` | `deploy/nginx-blog.conf:73` |
| 8 | P3 | 评论限流在文章存在性校验之前，无效请求也消耗配额 | `server/internal/handlers/handlers.go:170-182` |
| 9 | P3 | 搜索未转义 LIKE 通配符 `%`/`_` | `server/internal/db/db.go:256` |
| 10 | P3 | 文章量大后 LIKE 全表扫描，可上 FTS5 | `server/internal/db/db.go:254-261` |
| 11 | P3 | RSS 缺 `<atom:link self>`；标签页未进 sitemap | `server/internal/handlers/seo.go:59-70` |
| 12 | P3 | 文章页无 JSON-LD 结构化数据 | `web/src/seo.js` |
| 13 | P3 | OAuth 日志截断按字节，可能切断多字节字符产生乱码 | `server/internal/handlers/oauth.go:360-365` |
| 14 | P3 | 管理端删除文章时忽略查询错误 | `server/internal/handlers/admin.go:258` |
| 15 | P3 | 整卡覆盖链接 `aria-hidden`，屏幕阅读器读不到卡片标题链接 | `web/src/components/PostCard.jsx:7` |
| 16 | P3 | 前端无 ErrorBoundary，渲染异常整页白屏 | `web/src/main.jsx` |
| 17 | P3 | deploy.sh 每次 `npm ci` 全量重装（2C/1.6G 服务器较慢） | `deploy/deploy.sh:20-22` |
| 18 | P3 | 上域名后需补 HTTPS / HSTS / HTTP/2（当前裸 IP 无法签证书） | `deploy/nginx-blog.conf:6` |

---

## P1 —— 建议尽快处理

### 1. 数据库备份机制缺失

评论（含 GitHub 登录评论者身份）、浏览量、slug 重定向表都只存在于服务器 `47.106.82.66` 上的一个 SQLite 文件里。`content/` 目录是文章的备份，但**评论数据没有任何第二副本**——磁盘故障或误操作 `rm` 后不可恢复。

**建议**（任选其一，10 分钟工作量）：
- systemd timer 或 crontab 每日执行 SQLite 在线备份并保留最近 7 份：
  ```bash
  sqlite3 /var/blog/data/blog.db ".backup /var/backups/blog-$(date +%F).db"
  ```
  （WAL 模式下 `.backup` 是安全的在线备份；也可直接复制 `.db` 文件，WAL 模式下建议配合 `.backup`）
- 进一步可推送至对象存储 / 另一台机器异地保存。
- 恢复演练一次，确认备份可用。

### 2. 非 JS 环境的文章级分享卡片

`web/index.html:25-28` 只有站点级静态 og 兜底；`web/src/seo.js` 的逐页 meta 只在浏览器执行 JS 后生效。微信、微博、QQ、Telegram、飞书等聊天工具的爬虫**不执行 JS**——分享任何文章，卡片永远显示站点首页的标题和描述。

**建议方案（按成本从低到高）**：
1. **Nginx sub_filter 注入**：`location /post/` 反代给 Go 新增一个轻量端点（查 slug → 输出替换过 og:title/og:description/og:url 的 index.html）。改动集中在 Go 一个 handler + Nginx 一段 location。
2. 构建期预渲染（复杂，暂不建议）。
3. 接受现状（个人博客、传播以 GitHub/RSS 为主时影响有限）。

---

## P2 —— 值得做

### 3. 编辑器草稿自动保存

`PostEditor.jsx:29-37` 已用 `beforeunload` 拦截刷新/关闭（主要丢稿场景已覆盖），但浏览器崩溃、断电、误点「离开」仍会丢。建议：`set(key, value)` 时顺手 `localStorage.setItem('draft:'+id, ...)`，加载时若有草稿提示恢复，保存成功后清除。约 20 行改动。

### 4. 死资产清理

- `assets/style.css`（13KB，旧静态版主题）在 `web/src`、`web/index.html`、`content/` 中均无引用（已 grep 确认），但 `web/scripts/sync-assets.mjs` 每次构建都会把它复制进 `web/public/assets/` 并部署到生产。
- `assets/favicon.svg` 与 `web/public/favicon.svg` 重复（index.html 引用的是后者）。

**建议**：删除 `assets/style.css`；确认无文章引用 `assets/favicon.svg` 后删除（或从 sync 排除）。

### 5. ~~`/assets/` 缺失文件返回 200 HTML~~（复查为误报）

> **2026-10-14 复查更正**：nginx 按最长前缀匹配，`/assets/missing.png` 命中 `location /assets/`（而非带 SPA 兜底的 `location /`），缺失文件本就返回 404。此项为误报。已在 `location /assets/` 补 `try_files $uri =404` 作为防御性显式化（顺带把目录请求从 403 统一为 404），非行为修复。

### 6. 安全头补全

`deploy/nginx-blog.conf:17-19` 已有 nosniff / X-Frame-Options / Referrer-Policy，还差两个低成本的：
- `Permissions-Policy: camera=(), microphone=(), geolocation=()`（博客用不到这些能力，直接关掉）。
- `Content-Security-Policy`：因 `index.html:6-18` 有内联主题脚本、文章 HTML 由 goldmark 直出，建议先上**仅报告模式**观察：
  ```
  Content-Security-Policy-Report-Only: default-src 'self'; img-src 'self' https://cravatar.cn https://avatars.githubusercontent.com data:; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'
  ```
  观察无报错后再切正式 `Content-Security-Policy`。

---

## P3 —— 顺手可修（按性价比排序）

7. **gzip 补 `application/xml`**（nginx-blog.conf:73）：sitemap 由 Go 输出 `application/xml`（seo.go:122），当前类型清单只有 `text/xml`，sitemap 未被压缩。一行修复。
8. **限流顺序微调**（handlers.go:170-182）：把 `GetBySlug` 的文章存在性校验提到两个 `Lim.Allow` 之前，避免打错 slug / 已删除文章的请求也消耗当日 20 条配额。
9. **LIKE 通配符转义**（db.go:256）：搜索词里的 `%`/`_` 会被当通配符（无注入风险，仅语义不准）。可在拼 like 前对 `%`、`_`、`\` 做 `ESCAPE '\'` 转义。
10. **FTS5 全文索引**（db.go:254-261）：当前 30 篇文章 LIKE 足够快；若未来超百篇且搜索变慢，modernc.org/sqlite 支持 FTS5，建 `posts_fts` 虚表同步 content 即可。**现阶段不必做，留档**。
11. **RSS/sitemap 完善**（seo.go）：RSS channel 补 `<atom:link rel="self" .../>`（RSS 校验器要求）；sitemap 可补 `/tag/:tag` 列表页。
12. **JSON-LD 结构化数据**（seo.js / Post.jsx）：文章页注入 `BlogPosting` schema（headline/datePublished/author），对 Google 富摘要有帮助。
13. **`snip()` 按字节截断**（oauth.go:360-365）：`s[:200]` 可能切断 UTF-8 字符，日志出现乱码。改用 `[]rune` 截断。
14. **`AdminDeletePost` 忽略错误**（admin.go:258）：`old, _ := h.DB.GetByID(id)`，读取失败时跳过重定向清理。改为显式判错即可（与 `AdminUpdatePost:198-206` 对齐）。
15. **PostCard 可访问性**（PostCard.jsx:7）：整卡覆盖链接 `aria-hidden="true"` 且标题不是链接，屏幕阅读器用户无法激活卡片。建议把 `<h2>` 包进链接、去掉覆盖层上的 aria-hidden（保留覆盖层本身）。
16. **ErrorBoundary**（main.jsx）：包一层 React 错误边界，渲染异常时显示「出错了，刷新重试」而不是白屏。
17. **deploy.sh 构建提速**（deploy.sh:20-22）：`npm ci` 每次全量重装。可在服务器保留 `node_modules` 时改用 `npm ci --prefer-offline`，或本地构建后 rsync dist（2C/1.6G 机器 npm build 压力也更小）。
18. **HTTPS 预留**（nginx-blog.conf:6）：当前裸 IP 直连 HTTP 是已知限制。上域名后记得：Certbot 签证书 → `listen 443 ssl http2` → 80 跳 443 → 加 HSTS → `SITE_URL` 换 https（Cookie Secure 已按前缀自动生效，oauth.go:208）。

## 已确认无需修改的点（审查过但设计合理）

- `IncrViews` 同步写库 + 内存限流去重（handlers.go:73-76）：个人博客量级无压力；limiter 24h 清扫防内存膨胀（limiter.go:67-85）。
- 管理端 token 存 localStorage：有 XSS 暴露面，但管理端仅作者自用、React 全量转义 + 无第三方脚本，风险可接受。
- `MaxOpenConns(4)` + WAL + busy_timeout（db.go:71-79）：单实例 SQLite 合理配置。
- OAuth 重试只对「请求未送达」类失败（oauth.go:276-294）：避免消费一次性授权码，注释解释得很清楚。
- 评论楼中楼孤儿回复处理、删除整棵子树（db.go:582-595）：逻辑完备。

## 建议处理顺序

1. 先做 #1（备份）+ #7（gzip 一行）+ #8（顺序微调）——三个都是小改动、高确定性。
2. 再做 #4（清死资产）+ #5（404 兜底）+ #13/#14（后端小修）。
3. 有整块时间时做 #2（分享卡片）和 #3（草稿自动保存）。

---

## 修复记录（2026-10-14 第二轮）

P2/P3 已全部处理，P1 按要求暂不动。各项落点：

| # | 状态 | 落点 |
|---|------|------|
| 1 备份 | ⏸ 未动（P1） | — |
| 2 分享卡片 | ⏸ 未动（P1） | — |
| 3 草稿自动保存 | ✅ | `web/src/pages/admin/PostEditor.jsx`：localStorage 防抖 800ms 自动保存 + 进页恢复（带差异检测防过期残留）+ 保存成功清除 |
| 4 死资产 | ✅ | 删除 `assets/style.css`、`assets/favicon.svg`（均零引用、git 可恢复）；`web/public/favicon.svg`（真正被引用的）保留 |
| 5 assets 404 | ✅（误报更正） | 复查确认为误报（见上文）；已补 `try_files $uri =404` 作防御性显式化 |
| 6 安全头 | ✅ | `deploy/nginx-blog.conf`：server 级 + 3 个自带 add_header 的 location 补 `Permissions-Policy` 与 `Content-Security-Policy`（img 白名单 = cravatar.cn + GitHub 头像，已核验 content/ 无外链图片） |
| 7 gzip xml | ✅ | `deploy/nginx-blog.conf:90` 补 `application/xml` |
| 8 限流顺序 | ✅ | `server/internal/handlers/handlers.go`：文章存在性校验提前到限流前（带滥用面说明注释） |
| 9 LIKE 转义 | ✅ | `server/internal/db/db.go`：新增 `escapeLike` + `ESCAPE '\'`；新增 `db/like_test.go` 两组测试 |
| 10 FTS5 | 📋 留档 | 按原报告结论：当前规模不做 |
| 11 RSS/sitemap | ✅ | `server/internal/handlers/seo.go`：RSS 补 `xmlns:atom` + `<atom:link rel="self">`；sitemap 补全部标签页 |
| 12 JSON-LD | ✅ | `web/src/seo.js` 新增 `setJsonLd`（setMeta 统一清理残留）；`web/src/pages/Post.jsx` 注入 BlogPosting |
| 13 snip 截断 | ✅ | `server/internal/handlers/oauth.go`：改按 rune 截断 |
| 14 删除错误处理 | ✅ | `server/internal/handlers/admin.go` AdminDeletePost：显式判读错误 + 404 分支 |
| 15 PostCard a11y | ✅ | `web/src/components/PostCard.jsx` 标题改为真实链接（覆盖层保留热区职责）；`style.css` 相应放开 h2 指针事件 |
| 16 ErrorBoundary | ✅ | 新增 `web/src/components/ErrorBoundary.jsx`，`main.jsx` 包裹 App |
| 17 deploy.sh 提速 | ✅ | `deploy/deploy.sh`：package-lock 未变化时跳过 `npm ci`（`FORCE_NPM_CI=1` 可强制） |
| 18 HTTPS | 📋 留档 | 等上域名时一并处理 |

**验证**：`go vet` 干净；`go test ./...` 全过（含新增 like_test）；前端全部源文件经 sucrase JSX 转换验证无语法错误。

**⚠️ 本机构建环境注意**：本机 `npm run build` 自 2026-10-14 起失败——esbuild 原生二进制（旧版 Go 运行时构建）报 `winmm.dll not found` 崩溃，为系统层变化所致（10 月 8 日本机还构建成功；go 1.25.2 编译的程序不受影响）。GitHub CI 与服务器构建均为 Linux，不受影响。顺带修正了 `web/package.json` 中 esbuild 二进制的错误版本钉子（`^0.28.2` → `^0.21.5`，与 vite 依赖的 esbuild JS 版本匹配——该错钉本就是之前手动补装时写入的）。后续可在系统侧排查或升级 vite 后恢复本机构建。

**部署提示**：`deploy/nginx-blog.conf` 与 `deploy/deploy.sh` 的改动需要同步到服务器（nginx conf 覆盖 `/etc/nginx/sites-available/blog` 后 `nginx -t && systemctl reload nginx`；deploy.sh 复制到 `/var/blog/deploy.sh`）。首次上线 CSP 后建议开浏览器控制台确认无资源被拦，异常时优先检查 `img-src`。
