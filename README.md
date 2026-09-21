# 我的博客

基于自研纯静态生成器的个人博客，部署于 GitHub Pages。

**在线地址**：https://lird-e.github.io/Blog

## 特性

- **零依赖框架**：Python 单脚本（`build.py`）把 Markdown 渲染成完整静态站点，无 Node / 前端构建链
- **GitHub Primer 主题**：明暗双主题，跟随系统/手动切换，毛玻璃导航
- **全站 SEO**：canonical / Open Graph / Twitter Card / RSS / sitemap.xml / robots.txt
- **站内搜索**：构建时生成 `search.json`，纯前端本地检索
- **代码高亮**：Pygments 双主题（明/暗各一份样式）
- **标签系统**：标签索引 + 独立标签页
- **可选评论**：内置 Waline 集成（当前休眠），部署服务端后填一行配置即启用楼中楼回复
- **CMS 后台**：`admin/` 内置 Sveltia CMS，在线写作直接提交到仓库

## 目录结构

```
build.py          # 静态站点生成器（核心）
serve.py          # 本地预览服务器
publish.bat       # 一键构建 + 推送 GitHub Pages
content/
  posts/          # 文章（Markdown + frontmatter：title/date/tags/excerpt/slug）
  about.md        # 关于页
templates/        # HTML 模板（base/index/post/tags/tag/search）
assets/           # 样式、favicon 等静态资源
admin/            # Sveltia CMS 后台
public/           # 构建产物（GitHub Pages 部署源）
docs/             # 平台方案、Waline 部署指南等文档
```

## 本地运行

```bash
# 首次：创建虚拟环境并安装依赖
python -m venv .venv
.venv/Scripts/pip install -r requirements.txt

# 构建 + 预览
.venv/Scripts/python build.py
.venv/Scripts/python serve.py   # http://localhost:8000
```

## 发布

```bash
publish.bat   # 构建 → commit → push 到 main，GitHub Actions 自动部署
```

## 写文章

在 `content/posts/` 新建 Markdown 文件，frontmatter 示例：

```markdown
---
title: 文章标题
date: 2026-09-21
tags: [技术, 随笔]
excerpt: 一句话摘要（可省略，自动截取正文）
---

正文…
```

或直接在浏览器打开 `https://lird-e.github.io/Blog/admin/` 用 Sveltia CMS 在线写作。

## 技术栈

Python 3.13 · Markdown/Pygments/PyYAML · 原生 HTML/CSS/JS · GitHub Actions
