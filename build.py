"""纯静态博客生成器：把 content/ 下的 Markdown 渲染成 public/ 静态站点。
用法：.venv/Scripts/python build.py
"""
import re, shutil, json, math, hashlib
import html as _html
from pathlib import Path
from collections import defaultdict
from datetime import datetime, timezone
from urllib.parse import urlparse, quote
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

# 站点部署的子路径前缀（如 /Blog），用于 404 页的站点绝对路径
BASE_PATH = urlparse(SITE_URL).path.rstrip("/")  # "/Blog"

# CSS 缓存版本：对样式内容（style.css + 两份 Pygments 生成样式）做哈希，
# 拼到样式链接 ?v= 参数上——内容没变 URL 就不变，浏览器缓存持续有效；
# 改样式后哈希自动变化，用户无需强刷即可拿到新版
PYGMENTS_CSS = HtmlFormatter(style="default").get_style_defs(".codehilite")
# 深色模式专用高亮：get_style_defs 的选择器前缀限定在 [data-theme="dark"] 下生效，
# 与浅色高亮共存互不干扰
PYGMENTS_DARK_CSS = HtmlFormatter(style="github-dark").get_style_defs('[data-theme="dark"] .codehilite')

_css_hash = hashlib.md5()
_css_hash.update((ASSETS / "style.css").read_bytes())
_css_hash.update(PYGMENTS_CSS.encode("utf-8"))
_css_hash.update(PYGMENTS_DARK_CSS.encode("utf-8"))
CSS_VER = _css_hash.hexdigest()[:8]

# Waline 评论配置：已暂停评论功能（保持纯静态部署）。
# 未来想启用：部署服务端后把地址填到 server_url 即可（见 docs/Waline部署指南.md）
WALINE = {
    "server_url": "",        # 形如 https://your-project.service.tcloudbaseapp.com
    "lang": "zh-CN",
}


def waline_html():
    url = WALINE["server_url"].strip().rstrip("/")
    if not url:
        # 未配置服务端：不渲染任何评论区元素，保持纯静态页面
        return ""
    return (
        '<section class="comments">\n'
        '  <div id="waline"></div>\n'
        '  <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/@waline/client@3/dist/waline.css"\n'
        '        onerror="this.onerror=null;this.href=\'https://unpkg.com/@waline/client@v3/dist/waline.css\'">\n'
        '  <script type="module">\n'
        "    (async function(){\n"
        '      var mod;\n'
        "      try { mod = await import('https://cdn.jsdelivr.net/npm/@waline/client@3/dist/waline.js'); }\n"
        '      catch(e) { mod = await import(\'https://unpkg.com/@waline/client@v3/dist/waline.js\'); }\n'
        '      mod.init({\n'
        "        el: '#waline',\n"
        f'        serverURL: "{url}",\n'
        f'        lang: "{WALINE["lang"]}",\n'
        '        dark: \'html[data-theme="dark"]\',\n'
        '        reaction: true\n'
        '      });\n'
        '    })();\n'
        "  </script>\n"
        "</section>"
    )


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
            if v is None:
                v = ""  # YAML 空值（如 "excerpt:" 后无内容）解析为 None
            elif isinstance(v, (datetime, timezone)):
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


def slug_source(meta, f):
    """slug 取值：优先 frontmatter 显式 slug（如英文别名，保证 URL 稳定），
    留空/缺失时回退标题（存量文章行为不变）。"""
    raw = meta.get("slug")
    if isinstance(raw, str) and raw.strip():
        return raw.strip()
    return meta.get("title", f.stem)


def render_markdown(body):
    md = markdown.Markdown(extensions=["fenced_code", "codehilite", "toc", "tables"])
    html = md.convert(body)
    # 正文图片默认懒加载 + 异步解码，减少首屏请求数（构建期一次性注入）
    return re.sub(r"<img ", '<img loading="lazy" decoding="async" ', html)


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


def esc_attr(s):
    """meta/og 标签属性值转义（escape 默认不转双引号）"""
    return escape(str(s), {'"': "&quot;", "'": "&#x27;"})


def make_excerpt(html_text, limit=120):
    """从渲染后的 HTML 提取纯文本摘要：去标签、反转义实体、折叠空白、截断。"""
    text = re.sub(r"<[^>]+>", "", html_text)
    text = _html.unescape(text)
    text = re.sub(r"\s+", " ", text).strip()
    if len(text) <= limit:
        return text
    return text[:limit].rstrip() + "…"


def rss_date(d):
    try:
        dt = datetime.strptime(d, "%Y-%m-%d").replace(tzinfo=timezone.utc)
        return dt.strftime("%a, %d %b %Y %H:%M:%S +0000")
    except Exception:
        return ""


base_tpl = (TEMPLATES / "base.html").read_text(encoding="utf-8")
index_tpl = (TEMPLATES / "index.html").read_text(encoding="utf-8")
post_tpl = (TEMPLATES / "post.html").read_text(encoding="utf-8")
tags_index_tpl = (TEMPLATES / "tags.html").read_text(encoding="utf-8")
tag_tpl = (TEMPLATES / "tag.html").read_text(encoding="utf-8")
search_tpl = (TEMPLATES / "search.html").read_text(encoding="utf-8")


def fill(tpl, **kw):
    # 一次性正则替换：re.sub 不会重扫替换后的文本，
    # 因此正文里字面出现的 {{base}} / {{extra_head}} / {{css_ver}} 等不会被
    # 后续替换误改写（朴素 str.replace 链会导致含占位符的文章页面静默损坏，
    # 例如提示词模板 / Jinja2 示例里常见的 {{变量}} 写法）。
    if not kw:
        return tpl
    pattern = re.compile("|".join(re.escape("{{" + k + "}}") for k in kw))
    return pattern.sub(lambda m: str(kw[m.group(0)[2:-2]]), tpl)


# 构建前清空 public/：避免改 slug / 删文章后旧 HTML 残留成死链。
# 模板已读入内存，后续 mkdir 会重建目录结构；CI 全新 checkout 时 public 不存在，跳过。
if PUBLIC.exists():
    shutil.rmtree(PUBLIC)


def page(title, content, base="", css_ver=CSS_VER, description=SITE_DESC,
         url_path="", og_type="website", noindex=False):
    """title 传原始文本，本函数内部负责 <title> 与 og:title 两种语境的转义。
    url_path：站内相对路径（如 'posts/xxx.html'），空字符串表示首页，
    用于拼 canonical / og:url 绝对地址。"""
    url = SITE_URL + ("/" + url_path if url_path else "/")
    # 默认社交分享封面（1200×630，见 assets/og-cover.png）
    og_image = f"{SITE_URL}/assets/og-cover.png"
    extra_head = (
        f'<meta name="description" content="{esc_attr(description)}">\n'
        f'<link rel="canonical" href="{escape(url)}">\n'
        f'<meta property="og:title" content="{esc_attr(title)}">\n'
        f'<meta property="og:description" content="{esc_attr(description)}">\n'
        f'<meta property="og:url" content="{escape(url)}">\n'
        f'<meta property="og:type" content="{og_type}">\n'
        f'<meta property="og:image" content="{escape(og_image)}">\n'
        f'<meta property="og:image:width" content="1200">\n'
        f'<meta property="og:image:height" content="630">\n'
        f'<meta property="og:site_name" content="{esc_attr(SITE_TITLE)}">\n'
        f'<meta name="twitter:card" content="summary_large_image">\n'
        f'<meta name="twitter:title" content="{esc_attr(title)}">\n'
        f'<meta name="twitter:description" content="{esc_attr(description)}">\n'
        f'<meta name="twitter:image" content="{escape(og_image)}">'
    )
    if noindex:
        extra_head += '\n<meta name="robots" content="noindex">'
    # fill 为一次性正则替换（不重扫替换文本），替换顺序不再影响结果；
    # extra_head 中不能含 {{base}} 占位符（上面已用 f-string 预渲染绝对 URL）。
    return fill(base_tpl, title=escape(title), content=content, base=base,
                extra_head=extra_head, css_ver=css_ver)


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
    rendered_html = render_markdown(body)
    # excerpt 留空时从正文纯文本自动截取，兜底首页卡片 / RSS / meta description
    excerpt = str(meta.get("excerpt", "")).strip() or make_excerpt(rendered_html)
    posts.append({
        "title": meta.get("title", f.stem),
        "date": meta.get("date", ""),
        "tags": meta.get("tags", ""),
        "tags_list": tags_l,
        "excerpt": excerpt,
        "slug": slugify(slug_source(meta, f)),
        "html": rendered_html,
    })

# slug 去重：同标题/同 slug 的文章自动加序号，避免输出 HTML 互相覆盖
used = set()
for p in posts:
    s = p["slug"]
    if s not in used:
        used.add(s)
        continue
    n = 2
    while f"{s}-{n}" in used:
        n += 1
    p["slug"] = f"{s}-{n}"
    used.add(p["slug"])

# 按 frontmatter 发布日期降序（date 缺失/非法的排最后），
# 不能按文件名排序：serve.py/Decap 生成的文件名可能没有日期前缀。
# key 首位用 != 判断：reverse=True 时 (True, date) 在前，无日期 (False, "") 垫底。
posts.sort(key=lambda p: (p["date"] != "", p["date"]), reverse=True)

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
    # 分页页生成在 /page/ 子目录下（page/2.html、page/3.html…），
    # 链接必须回退一级目录（../），否则相对路径会解析成 /page/page/N.html 而 404
    base = "" if current == 1 else "../"
    parts = []
    if current > 1:
        prev = "index.html" if current == 2 else f"page/{current - 1}.html"
        parts.append(f'<a class="pager-prev" href="{base}{prev}">← 上一页</a>')
    else:
        parts.append('<span class="pager-prev disabled">← 上一页</span>')
    for n in range(1, total_pages + 1):
        href = "index.html" if n == 1 else f"page/{n}.html"
        if n == current:
            parts.append(f'<span class="pager-num current">{n}</span>')
        else:
            parts.append(f'<a class="pager-num" href="{base}{href}">{n}</a>')
    if current < total_pages:
        parts.append(f'<a class="pager-next" href="{base}page/{current + 1}.html">下一页 →</a>')
    else:
        parts.append('<span class="pager-next disabled">下一页 →</span>')
    return '<nav class="pager">' + "".join(parts) + '</nav>'


# 生成文章页
out_posts = PUBLIC / "posts"
out_posts.mkdir(parents=True, exist_ok=True)
for p in posts:
    body = fill(post_tpl, title=escape(p["title"]), date=escape(p["date"]),
                tags_html=tag_links(p["tags_list"], "../"),
                content=p["html"], base="../", comments=waline_html())
    (out_posts / (p["slug"] + ".html")).write_text(
        page(p["title"], body, base="../", description=p["excerpt"],
             url_path=f"posts/{p['slug']}.html", og_type="article"),
        encoding="utf-8")

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
            page(f"第 {idx} 页", body, base="../",
                 url_path=f"page/{idx}.html", noindex=True), encoding="utf-8")

# 生成标签索引页
tag_cloud = "\n".join(
    f'<a class="tag-cloud-item" href="tags/{slugify(t)}.html">'
    f'{escape(t)} <span class="tag-count">{len(items)}</span></a>'
    for t, items in sorted(tag_map.items())
)
tags_index_body = fill(tags_index_tpl, tag_cloud=tag_cloud)
(PUBLIC / "tags.html").write_text(
    page("标签", tags_index_body, base="", url_path="tags.html"), encoding="utf-8")

# 生成每个标签页
tags_dir = PUBLIC / "tags"
tags_dir.mkdir(parents=True, exist_ok=True)
for t, items in tag_map.items():
    cards_t = "\n".join(make_card(p, "../") for p in items)
    body = fill(tag_tpl, tag=escape(t), post_cards=cards_t)
    (tags_dir / (slugify(t) + ".html")).write_text(
        page(f"标签：{t}", body, base="../", description=f"标签「{t}」下的文章",
             url_path=f"tags/{slugify(t)}.html"), encoding="utf-8")

# 生成关于页
about_src = CONTENT / "about.md"
if about_src.exists():
    meta, body = parse_frontmatter(about_src.read_text(encoding="utf-8"))
    about_html = fill(post_tpl, title=escape(meta.get("title", "关于")), date="",
                      tags_html="", content=render_markdown(body), base="",
                      comments="")
    (PUBLIC / "about.html").write_text(
        page(meta.get("title", "关于"), about_html, base="",
             url_path="about.html"), encoding="utf-8")

# 生成搜索页
search_body = fill(search_tpl, base="")
(PUBLIC / "search.html").write_text(
    page("搜索", search_body, base="", url_path="search.html", noindex=True), encoding="utf-8")

# 生成搜索索引（紧凑 JSON 不加缩进；正文截 1000 字符，控制索引体积）
search_entries = []
for p in posts:
    text = re.sub(r"<[^>]+>", "", p["html"])
    search_entries.append({
        "title": p["title"],
        "url": f"posts/{p['slug']}.html",
        "date": p["date"],
        "tags": p["tags_list"],
        "excerpt": p["excerpt"],
        "text": text[:1000],
    })
(PUBLIC / "search.json").write_text(
    json.dumps(search_entries, ensure_ascii=False, separators=(",", ":")),
    encoding="utf-8")

# 复制静态资源
if ASSETS.exists():
    shutil.copytree(ASSETS, PUBLIC / "assets", dirs_exist_ok=True)

# 复制 Decap CMS 后台（admin/）到 public，使其可随站点部署并本地预览
ADMIN = ROOT / "admin"
if ADMIN.exists():
    shutil.copytree(ADMIN, PUBLIC / "admin", dirs_exist_ok=True)

# 生成代码高亮样式（内容已在文件头部生成并纳入 CSS_VER 哈希）
(PUBLIC / "assets").mkdir(parents=True, exist_ok=True)
(PUBLIC / "assets" / "pygments.css").write_text(PYGMENTS_CSS, encoding="utf-8")
(PUBLIC / "assets" / "pygments-dark.css").write_text(PYGMENTS_DARK_CSS, encoding="utf-8")

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
    '<rss version="2.0" xmlns:atom="http://www.w3.org/2005/Atom">\n'
    "  <channel>\n"
    f"    <title>{escape(SITE_TITLE)}</title>\n"
    f"    <link>{SITE_URL}</link>\n"
    f"    <description>{escape(SITE_DESC)}</description>\n"
    f'    <atom:link rel="self" type="application/rss+xml" href="{escape(SITE_URL + "/feed.xml")}"/>\n'
    f"    <lastBuildDate>{datetime.now(timezone.utc).strftime('%a, %d %b %Y %H:%M:%S +0000')}</lastBuildDate>\n"
    "    <language>zh-CN</language>\n"
    + "\n".join(rss_items)
    + "\n  </channel>\n</rss>\n"
)
(PUBLIC / "feed.xml").write_text(rss, encoding="utf-8")

# 生成 sitemap.xml（中文 slug 需百分号编码；文章带 lastmod）
def sm_url(loc, lastmod=""):
    loc_e = escape(quote(loc, safe=":/"))
    lm = f"<lastmod>{lastmod}</lastmod>" if lastmod else ""
    return f"  <url><loc>{loc_e}</loc>{lm}</url>"

sm = [sm_url(SITE_URL + "/")]
for idx in range(2, total_pages + 1):
    sm.append(sm_url(f"{SITE_URL}/page/{idx}.html"))
for p in posts:
    sm.append(sm_url(f"{SITE_URL}/posts/{p['slug']}.html", p["date"]))
sm.append(sm_url(f"{SITE_URL}/tags.html"))
for t in tag_map:
    sm.append(sm_url(f"{SITE_URL}/tags/{slugify(t)}.html"))
if about_src.exists():
    sm.append(sm_url(f"{SITE_URL}/about.html"))
sm.append(sm_url(f"{SITE_URL}/search.html"))
sitemap = ('<?xml version="1.0" encoding="UTF-8"?>\n'
           '<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">\n'
           + "\n".join(sm) + "\n</urlset>\n")
(PUBLIC / "sitemap.xml").write_text(sitemap, encoding="utf-8")

# 生成 robots.txt（Sitemap 指令必须是绝对 URL）
(PUBLIC / "robots.txt").write_text(
    "User-agent: *\nAllow: /\n\n"
    f"Sitemap: {SITE_URL}/sitemap.xml\n", encoding="utf-8")

# 生成 404.html：GitHub Pages 对 /Blog/** 下未命中路径返回此页，
# 但展示深度不确定（如 /Blog/a/b/c），相对路径会全部失效，
# 故 base 传站点绝对前缀 /Blog/，让 CSS/favicon/导航链接绝对化。
# 取舍：本地 serve.py 预览此页时样式丢失（正文仍可读），线上正常。
not_found_body = (
    '<div style="text-align:center;padding:60px 0;">'
    "<h1>404</h1>"
    "<p>页面不存在或已被移动。</p>"
    f'<p><a href="{BASE_PATH}/index.html">← 返回首页</a></p>'
    "</div>"
)
(PUBLIC / "404.html").write_text(
    page("404 - 页面不存在", not_found_body, base=BASE_PATH + "/",
         url_path="404.html", noindex=True), encoding="utf-8")

print(f"✅ 生成完成：{len(posts)} 篇文章、{len(tag_map)} 个标签、{total_pages} 页")
