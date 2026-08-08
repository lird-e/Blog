"""纯静态博客生成器：把 content/ 下的 Markdown 渲染成 public/ 静态站点。
用法：.venv/Scripts/python build.py
"""
import re, shutil, json, math
from pathlib import Path
from collections import defaultdict
from datetime import datetime, timezone
import markdown
import yaml
from pygments.formatters import HtmlFormatter
from xml.sax.saxutils import escape

ROOT = Path(__file__).resolve().parent
CONTENT = ROOT / "content"
POSTS_DIR = CONTENT / "posts"
TEMPLATES = ROOT / "templates"
ASSETS = ROOT / "assets"
PUBLIC = ROOT / "public"

# === 站点配置（部署时按实际情况修改）===
SITE_URL = "https://lird-e.github.io/Blog"        # RSS / 站点绝对链接用
SITE_TITLE = "我的博客"
SITE_DESC = "记录技术、思考与生活。"
PAGE_SIZE = 10                                   # 每页文章数，超过自动分页

# Giscus 评论配置：到 https://giscus.app 获取 repo-id / category-id
# 前置：GitHub 公开仓库 + 开启 Discussions + 安装 Giscus App
# 把 repo_id / category_id 换成真实值后，文章页会自动启用评论
GISCUS = {
    "repo": "your-username/your-blog",
    "repo_id": "YOUR_REPO_ID",
    "category": "Announcements",
    "category_id": "YOUR_CATEGORY_ID",
    "mapping": "pathname",
    "lang": "zh-CN",
}


def parse_frontmatter(text):
    m = re.match(r"^---\s*\n(.*?)\n---\s*\n?(.*)$", text, re.DOTALL)
    if m:
        fm, body = m.group(1), m.group(2)
        try:
            meta = yaml.safe_load(fm) or {}
        except Exception:
            meta = {}
        # 统一为小写字符串键，兼容手写与 Decap 生成的写法
        norm = {}
        for k, v in meta.items():
            k = str(k).lower()
            if isinstance(v, (datetime, timezone)):
                v = v.isoformat()[:10]
            elif isinstance(v, list):
                pass  # 标签列表保留，由 tag_list 处理
            elif not isinstance(v, str):
                v = str(v)
            norm[k] = v
        return norm, body
    return {}, text


def slugify(title):
    s = re.sub(r"[^\w\u4e00-\u9fff]+", "-", title).strip("-").lower()
    return s or "post"


def render_markdown(body):
    md = markdown.Markdown(extensions=["fenced_code", "codehilite", "toc", "tables"])
    return md.convert(body)


def tag_list(tags):
    if tags is None:
        return []
    if isinstance(tags, list):
        return [str(t).strip() for t in tags if str(t).strip()]
    return [t.strip() for t in str(tags).split(",") if t.strip()]


def tag_links(tags, base):
    return " ".join(
        f'<a class="tag-link" href="{base}tags/{slugify(t)}.html">{escape(t)}</a>'
        for t in tags
    )


def rss_date(d):
    try:
        dt = datetime.strptime(d, "%Y-%m-%d").replace(tzinfo=timezone.utc)
        return dt.strftime("%a, %d %b %Y %H:%M:%S +0000")
    except Exception:
        return ""


def giscus_html():
    g = GISCUS
    if g["repo_id"] == "YOUR_REPO_ID":
        return "<!-- Giscus 未配置：在 build.py 填写 GISCUS 后自动启用评论 -->"
    return (
        '<section class="comments">\n'
        '  <script src="https://giscus.app/client.js"\n'
        f'          data-repo="{g["repo"]}"\n'
        f'          data-repo-id="{g["repo_id"]}"\n'
        f'          data-category="{g["category"]}"\n'
        f'          data-category-id="{g["category_id"]}"\n'
        f'          data-mapping="{g["mapping"]}"\n'
        '          data-strict="0"\n'
        '          data-reactions-enabled="1"\n'
        '          data-emit-metadata="0"\n'
        '          data-input-position="bottom"\n'
        '          data-theme="preferred_color_scheme"\n'
        f'          data-lang="{g["lang"]}"\n'
        '          data-loading="lazy"\n'
        '          crossorigin="anonymous" async>\n'
        '  </script>\n'
        '</section>'
    )


base_tpl = (TEMPLATES / "base.html").read_text(encoding="utf-8")
index_tpl = (TEMPLATES / "index.html").read_text(encoding="utf-8")
post_tpl = (TEMPLATES / "post.html").read_text(encoding="utf-8")
tags_index_tpl = (TEMPLATES / "tags.html").read_text(encoding="utf-8")
tag_tpl = (TEMPLATES / "tag.html").read_text(encoding="utf-8")
search_tpl = (TEMPLATES / "search.html").read_text(encoding="utf-8")


def fill(tpl, **kw):
    for k, v in kw.items():
        tpl = tpl.replace("{{" + k + "}}", str(v))
    return tpl


def page(title, content, base=""):
    return fill(base_tpl, title=title, content=content, base=base, extra_head="")


def make_card(p, base):
    return (
        f'<article class="post-card">'
        f'<a class="card-link" href="{base}posts/{p["slug"]}.html" aria-hidden="true" tabindex="-1"></a>'
        f'<div class="card-date">{escape(p["date"])}</div>'
        f'<h2>{escape(p["title"])}</h2>'
        f'<p class="card-excerpt">{escape(p["excerpt"])}</p>'
        f'<div class="card-tags">{tag_links(p["tags_list"], base)}</div>'
        f'</article>'
    )


# 加载所有文章
posts = []
for f in POSTS_DIR.glob("*.md"):
    meta, body = parse_frontmatter(f.read_text(encoding="utf-8"))
    tags_l = tag_list(meta.get("tags", ""))
    posts.append({
        "title": meta.get("title", f.stem),
        "date": meta.get("date", ""),
        "tags": meta.get("tags", ""),
        "tags_list": tags_l,
        "excerpt": meta.get("excerpt", ""),
        "slug": slugify(meta.get("title", f.stem)),
        "html": render_markdown(body),
    })

# 按 frontmatter 发布日期降序（date 缺失/非法的排最后），
# 不能按文件名排序：serve.py/Decap 生成的文件名可能没有日期前缀
posts.sort(key=lambda p: (p["date"] == "", p["date"]), reverse=True)

# 按标签归类
tag_map = defaultdict(list)
for p in posts:
    for t in p["tags_list"]:
        tag_map[t].append(p)

# 分页
total_pages = max(1, math.ceil(len(posts) / PAGE_SIZE))
pages = [posts[i * PAGE_SIZE:(i + 1) * PAGE_SIZE] for i in range(total_pages)]


def pager_html(current):
    if total_pages <= 1:
        return ""
    parts = []
    if current > 1:
        prev = "index.html" if current == 2 else f"page/{current - 1}.html"
        parts.append(f'<a class="pager-prev" href="{prev}">← 上一页</a>')
    else:
        parts.append('<span class="pager-prev disabled">← 上一页</span>')
    for n in range(1, total_pages + 1):
        href = "index.html" if n == 1 else f"page/{n}.html"
        if n == current:
            parts.append(f'<span class="pager-num current">{n}</span>')
        else:
            parts.append(f'<a class="pager-num" href="{href}">{n}</a>')
    if current < total_pages:
        parts.append(f'<a class="pager-next" href="page/{current + 1}.html">下一页 →</a>')
    else:
        parts.append('<span class="pager-next disabled">下一页 →</span>')
    return '<nav class="pager">' + "".join(parts) + '</nav>'


# 生成文章页
out_posts = PUBLIC / "posts"
out_posts.mkdir(parents=True, exist_ok=True)
for p in posts:
    body = fill(post_tpl, title=p["title"], date=p["date"],
                tags_html=tag_links(p["tags_list"], "../"),
                content=p["html"], base="../", giscus=giscus_html())
    (out_posts / (p["slug"] + ".html")).write_text(
        page(p["title"], body, base="../"), encoding="utf-8")

# 生成首页 + 分页页
page_dir = PUBLIC / "page"
page_dir.mkdir(parents=True, exist_ok=True)
for idx, page_posts in enumerate(pages, start=1):
    base = "" if idx == 1 else "../"
    cards_p = "\n".join(make_card(p, base) for p in page_posts)
    pager = pager_html(idx)
    body = fill(index_tpl, post_cards=cards_p, pager=pager)
    if idx == 1:
        (PUBLIC / "index.html").write_text(
            page("首页", body, base=""), encoding="utf-8")
    else:
        (page_dir / (f"{idx}.html")).write_text(
            page(f"第 {idx} 页", body, base="../"), encoding="utf-8")

# 生成标签索引页
tag_cloud = "\n".join(
    f'<a class="tag-cloud-item" href="tags/{slugify(t)}.html">'
    f'{escape(t)} <span class="tag-count">{len(items)}</span></a>'
    for t, items in sorted(tag_map.items())
)
tags_index_body = fill(tags_index_tpl, tag_cloud=tag_cloud)
(PUBLIC / "tags.html").write_text(page("标签", tags_index_body, base=""), encoding="utf-8")

# 生成每个标签页
tags_dir = PUBLIC / "tags"
tags_dir.mkdir(parents=True, exist_ok=True)
for t, items in tag_map.items():
    cards_t = "\n".join(make_card(p, "../") for p in items)
    body = fill(tag_tpl, tag=escape(t), post_cards=cards_t)
    (tags_dir / (slugify(t) + ".html")).write_text(
        page(f"标签：{t}", body, base="../"), encoding="utf-8")

# 生成关于页
about_src = CONTENT / "about.md"
if about_src.exists():
    meta, body = parse_frontmatter(about_src.read_text(encoding="utf-8"))
    about_html = fill(post_tpl, title=meta.get("title", "关于"), date="",
                      tags_html="", content=render_markdown(body), base="",
                      giscus="")
    (PUBLIC / "about.html").write_text(
        page(meta.get("title", "关于"), about_html, base=""), encoding="utf-8")

# 生成搜索页
search_body = fill(search_tpl, base="")
(PUBLIC / "search.html").write_text(page("搜索", search_body, base=""), encoding="utf-8")

# 生成搜索索引
search_entries = []
for p in posts:
    text = re.sub(r"<[^>]+>", "", p["html"])
    search_entries.append({
        "title": p["title"],
        "url": f"posts/{p['slug']}.html",
        "date": p["date"],
        "tags": p["tags_list"],
        "excerpt": p["excerpt"],
        "text": text[:3000],
    })
(PUBLIC / "search.json").write_text(
    json.dumps(search_entries, ensure_ascii=False, indent=2), encoding="utf-8")

# 复制静态资源
if ASSETS.exists():
    shutil.copytree(ASSETS, PUBLIC / "assets", dirs_exist_ok=True)

# 复制 Decap CMS 后台（admin/）到 public，使其可随站点部署并本地预览
ADMIN = ROOT / "admin"
if ADMIN.exists():
    shutil.copytree(ADMIN, PUBLIC / "admin", dirs_exist_ok=True)

# 生成代码高亮样式
(PUBLIC / "assets").mkdir(parents=True, exist_ok=True)
(PUBLIC / "assets" / "pygments.css").write_text(
    HtmlFormatter(style="default").get_style_defs(".codehilite"), encoding="utf-8")

# 生成 RSS
rss_items = []
for p in posts:
    link = f"{SITE_URL}/posts/{p['slug']}.html"
    rss_items.append(
        "    <item>\n"
        f"      <title>{escape(p['title'])}</title>\n"
        f"      <link>{link}</link>\n"
        f"      <guid>{link}</guid>\n"
        f"      <pubDate>{rss_date(p['date'])}</pubDate>\n"
        f"      <description>{escape(p['excerpt'])}</description>\n"
        "    </item>"
    )
rss = (
    '<?xml version="1.0" encoding="UTF-8"?>\n'
    '<rss version="2.0">\n'
    "  <channel>\n"
    f"    <title>{escape(SITE_TITLE)}</title>\n"
    f"    <link>{SITE_URL}</link>\n"
    f"    <description>{escape(SITE_DESC)}</description>\n"
    "    <language>zh-CN</language>\n"
    + "\n".join(rss_items)
    + "\n  </channel>\n</rss>\n"
)
(PUBLIC / "feed.xml").write_text(rss, encoding="utf-8")

print(f"✅ 生成完成：{len(posts)} 篇文章、{len(tag_map)} 个标签、{total_pages} 页")
