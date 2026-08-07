---
title: 用 60 行 Python 写一个静态博客生成器
date: 2026-01-15
tags: Python, 教程, 工具
excerpt: 不依赖任何框架，用标准库 + markdown 把文章变成可托管的静态网站。
---

## 为什么自己写生成器

市面上的博客框架很多，但有时候你只想要一个**完全可控、零依赖**的小工具：写文章、跑一下命令、拿到一堆 HTML，随便找个地方托管就行。

这个博客本身，就是用下面这套思路搭起来的。

## 核心思路

1. 把每篇文章写成 Markdown 文件，头部用 `---` 写元信息（标题、日期、标签）。
2. 用 Python 读取这些文件，渲染成 HTML。
3. 输出到一个 `public/` 目录，直接托管即可。

## 关键代码

```python
import markdown, pathlib

def render(text: str) -> str:
    return markdown.markdown(text, extensions=["fenced_code", "codehilite"])

for md in pathlib.Path("content/posts").glob("*.md"):
    html = render(md.read_text(encoding="utf-8"))
    # ...写入 public/posts/xxx.html
```

就这么简单。剩下的只是把元信息解析、首页列表、代码高亮拼起来。

## 小结

静态站点最大的好处是：**安全、快、随便托管**。没有数据库、没有后端，自然也没有被攻破的入口。对于个人博客来说，这是最省心的选择。
