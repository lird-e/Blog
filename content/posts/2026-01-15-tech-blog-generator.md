---
title: 用 Python 写一个静态博客生成器
date: 2026-01-15
tags: Python, 教程, 工具
excerpt: 从 60 行起步，逐步长出分页、标签、RSS、搜索、深色模式与 CMS 后台的纯静态博客工具链。
---

## 为什么自己写生成器

市面上的博客框架很多，但有时候你只想要一个**完全可控**的小工具：写文章、跑一下命令、拿到一堆 HTML，随便找个地方托管就行。

这个博客本身，就是用下面这套思路搭起来的——从一个最小可用版本起步，按需一点点长出了分页、标签、搜索、RSS 和后台。

## 最小内核

生成器的核心其实只有三步：

1. 把每篇文章写成 Markdown 文件，头部用 `---` 写元信息（标题、日期、标签）。
2. 用 Python 读取这些文件，渲染成 HTML。
3. 输出到一个 `public/` 目录，直接托管即可。

```python
import markdown, pathlib

def render(text: str) -> str:
    return markdown.markdown(text, extensions=["fenced_code", "codehilite"])

for md in pathlib.Path("content/posts").glob("*.md"):
    html = render(md.read_text(encoding="utf-8"))
    # ...写入 public/posts/xxx.html
```

这是最初的样子。能跑，但只能出文章页。

## 后来长出来的东西

真正的需求是在用的过程中冒出来的。每加一个，生成器就厚一点：

- **frontmatter 解析**：用 PyYAML 读元信息，标题/日期/标签/摘要/URL 别名都从文件头来，不再靠文件名。
- **首页与分页**：文章多了要分页，按发布日期降序排，导航条带上一页/下一页。
- **标签系统**：每个标签一个聚合页，外加一个标签云索引页。
- **摘要兜底**：frontmatter 没写 excerpt 时，从渲染后的正文提取纯文本截断。
- **搜索**：构建时生成 `search.json` 索引，前端按空格分词、AND 语义实时筛选。
- **RSS / sitemap / robots**：SEO 三件套，静态站都能自动产出。
- **代码高亮**：用 Pygments 生成两份样式（亮色 default + 暗色 github-dark），按 `data-theme` 切换。
- **深色模式**：CSS 变量 + `localStorage` 记忆偏好，加载时同步 Giscus 评论区主题。
- **两套发文后台**：本地 `serve.py` 一个简易表单（仅本机）；线上接 Sveltia CMS，用 GitHub Token 直接提交。
- **构建前清理**：每次 build 先清空 `public/`，改了 slug 不会留旧 HTML 死链。

依赖也从一个 `markdown` 库，长成了 `markdown / pygments / pyyaml` 三件套——标准库能做的还是用标准库，该借力的地方不硬造轮子。

## 小结

静态站点最大的好处是：**安全、快、随便托管**。没有数据库、没有后端，自然也没有被攻破的入口。

自己写生成器的代价是：每多一个需求，就得自己实现一次。好处是：每一行都看得懂，想改就改，不用和框架的约定搏斗。对于个人博客来说，这是最省心也最耐用的选择。
