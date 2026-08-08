"""本地博客后台：托管 public/ 静态站点 + /admin 网页发文。
用法：
    .venv/Scripts/python serve.py
然后浏览器打开 http://localhost:8000/admin
- 在网页上填标题/标签/正文，提交后自动写入 content/posts/*.md 并重新生成站点。
- 普通访问（如 / 、/posts/...）直接托管 public/ 静态文件。
"""
import re, sys, subprocess, webbrowser, threading
from pathlib import Path
from datetime import date
from http.server import ThreadingHTTPServer, SimpleHTTPRequestHandler
from urllib.parse import urlparse, parse_qs, quote

ROOT = Path(__file__).resolve().parent
PUBLIC = ROOT / "public"
CONTENT_POSTS = ROOT / "content" / "posts"
BUILD = ROOT / "build.py"
PORT = 8000


def slugify(title):
    s = re.sub(r"[^\w\u4e00-\u9fff]+", "-", title).strip("-").lower()
    return s or "post"


def today():
    return date.today().isoformat()


def build_site():
    py = ROOT / ".venv" / "Scripts" / "python.exe"
    if not py.exists():
        py = "python"
    try:
        subprocess.run([str(py), str(BUILD)], cwd=str(ROOT), check=True,
                       capture_output=True, text=True)
        return True, ""
    except subprocess.CalledProcessError as e:
        return False, (e.stderr or str(e))[-500:]


def list_posts_html():
    files = sorted(CONTENT_POSTS.glob("*.md"), reverse=True)
    if not files:
        return '<li class="empty">还没有文章，发一篇吧。</li>'
    out = []
    for f in files:
        text = f.read_text(encoding="utf-8")
        m = re.match(r"^---\s*\n(.*?)\n---", text, re.DOTALL)
        title = f.stem
        if m:
            for line in m.group(1).splitlines():
                if line.lower().startswith("title:"):
                    title = line.split(":", 1)[1].strip()
        slug = slugify(title)
        out.append(f'<li><a href="/posts/{slug}.html" target="_blank">{title}</a></li>')
    return "\n".join(out)


ADMIN_HTML = """<!DOCTYPE html>
<html lang="zh-CN">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>发文后台 - 我的博客</title>
<style>
 body{font-family:-apple-system,BlinkMacSystemFont,"PingFang SC","Microsoft YaHei",sans-serif;background:#fafafa;color:#1f2328;margin:0;padding:30px;}
 .wrap{max-width:760px;margin:0 auto;background:#fff;border:1px solid #e6e8eb;border-radius:12px;padding:28px;box-shadow:0 1px 3px rgba(0,0,0,.04);}
 h1{font-size:1.6rem;margin:0 0 4px;}
 .sub{color:#656d76;margin:0 0 18px;}
 label{display:block;font-weight:600;margin:16px 0 6px;}
 input[type=text],textarea{width:100%;padding:10px 12px;border:1px solid #e6e8eb;border-radius:8px;font-size:1rem;box-sizing:border-box;font-family:inherit;}
 textarea{min-height:260px;resize:vertical;font-family:Consolas,Monaco,monospace;line-height:1.6;}
 .row{display:flex;gap:12px;}
 .row>div{flex:1;}
 button{margin-top:20px;background:#2f6f4f;color:#fff;border:0;border-radius:8px;padding:12px 26px;font-size:1rem;cursor:pointer;}
 button:hover{opacity:.9;}
 .err{color:#b42318;background:#fef3f2;border:1px solid #fecdca;padding:10px 14px;border-radius:8px;margin-top:14px;}
 .list{margin-top:30px;border-top:1px solid #e6e8eb;padding-top:18px;}
 .list ul{list-style:none;padding:0;margin:10px 0 0;}
 .list li{padding:8px 0;border-bottom:1px solid #f0f2f4;}
 .list li.empty{color:#8b949e;border:0;}
 .list a{color:#2f6f4f;text-decoration:none;}
 .list a:hover{text-decoration:underline;}
 .back{display:inline-block;margin-bottom:14px;color:#656d76;text-decoration:none;font-size:.9rem;}
 .back:hover{color:#2f6f4f;}
</style>
</head>
<body>
<div class="wrap">
 <a class="back" href="/">← 返回博客首页</a>
 <h1>发文后台</h1>
 <p class="sub">填写下方表单，发布后会自动生成静态页面。</p>
 <!--MSG-->
 <form method="post" action="/admin">
  <label>标题 *</label>
  <input type="text" name="title" required>
  <div class="row">
   <div><label>日期</label><input type="text" name="date" placeholder="YYYY-MM-DD（留空=今天）"></div>
   <div><label>标签（逗号分隔）</label><input type="text" name="tags" placeholder="技术, 随笔"></div>
  </div>
  <label>摘要</label>
  <input type="text" name="excerpt" placeholder="一句话摘要（显示在首页卡片与 RSS）">
  <label>正文（Markdown）*</label>
  <textarea name="content" required placeholder="用 Markdown 写正文，支持 **加粗**、`代码`、代码块、表格等。"></textarea>
  <button type="submit">发布文章</button>
 </form>
 <div class="list">
  <strong>已发布文章</strong>
  <ul>
<!--LIST-->
  </ul>
 </div>
</div>
</body>
</html>
"""


class Handler(SimpleHTTPRequestHandler):
    def __init__(self, *a, **k):
        super().__init__(*a, directory=str(PUBLIC), **k)

    def _admin(self, msg=""):
        html = (ADMIN_HTML
                .replace("<!--MSG-->", f'<p class="err">{msg}</p>' if msg else "")
                .replace("<!--LIST-->", list_posts_html()))
        self.send_response(200)
        self.send_header("Content-Type", "text/html; charset=utf-8")
        self.end_headers()
        self.wfile.write(html.encode("utf-8"))

    def do_GET(self):
        parsed = urlparse(self.path)
        if parsed.path in ("/admin", "/admin/"):
            self._admin()
            return
        if parsed.path in ("", "/"):
            self.path = "/index.html"
        super().do_GET()

    def do_POST(self):
        parsed = urlparse(self.path)
        if parsed.path not in ("/admin", "/admin/"):
            self.send_error(404)
            return
        length = int(self.headers.get("Content-Length", 0))
        body = self.rfile.read(length).decode("utf-8")
        data = parse_qs(body, keep_blank_values=True)
        # 注意：parse_qs 已自动做 URL 解码，这里不能再 unquote，
        # 否则正文中字面出现的 %xx（如 URL 编码链接）会被二次解码损坏
        g = lambda k: data.get(k, [""])[0].strip()
        title, date_s = g("title"), g("date") or today()
        tags, excerpt, content = g("tags"), g("excerpt"), g("content")
        if not title or not content:
            self._admin("标题和正文都不能为空。")
            return
        # 保存 md 文件（避免覆盖同名文件）
        slug = slugify(title)
        candidate = CONTENT_POSTS / (slug + ".md")
        n = 2
        while candidate.exists():
            candidate = CONTENT_POSTS / (f"{slug}-{n}.md")
            n += 1
        md = (f"---\ntitle: {title}\ndate: {date_s}\ntags: {tags}\n"
              f"excerpt: {excerpt}\n---\n\n{content}\n")
        candidate.write_text(md, encoding="utf-8")
        ok, err = build_site()
        if not ok:
            self._admin("文章已保存，但重新生成站点失败：" + err)
            return
        self.send_response(302)
        # Location 头只能含 ASCII（latin-1），中文 slug 必须 URL 编码，
        # 否则 HTTP 头编码异常导致浏览器收到空响应
        self.send_header("Location", f"/posts/{quote(slug)}.html")
        self.end_headers()


if __name__ == "__main__":
    if not PUBLIC.exists():
        print("⚠ public/ 不存在，请先运行 build.py 生成站点。")
        sys.exit(1)
    print(f"🚀 本地博客后台已启动： http://localhost:{PORT}/admin")
    print("   普通访问 http://localhost:%d 可看博客；Ctrl+C 停止。" % PORT)
    server = ThreadingHTTPServer(("0.0.0.0", PORT), Handler)
    # 启动后自动打开浏览器跳到发文后台（仅本机有效）
    def open_browser():
        try:
            webbrowser.open(f"http://localhost:{PORT}/admin")
        except Exception:
            pass
    threading.Timer(1.0, open_browser).start()
    try:
        server.serve_forever()
    except KeyboardInterrupt:
        print("\n已停止。")
