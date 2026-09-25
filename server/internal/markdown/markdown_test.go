package markdown

import (
	"strings"
	"testing"
)

func TestRender(t *testing.T) {
	html := Render("# 标题\n\n正文 **加粗**")
	if !strings.Contains(html, "<h1") || !strings.Contains(html, "标题") {
		t.Fatalf("标题未渲染: %s", html)
	}
	if !strings.Contains(html, "<strong>加粗</strong>") {
		t.Fatalf("加粗未渲染: %s", html)
	}
}

func TestExcerpt(t *testing.T) {
	got := Excerpt("<p>第一段</p><script>alert(1)</script><style>.x{}</style>", 120)
	if strings.Contains(got, "alert") || strings.Contains(got, ".x") {
		t.Fatalf("script/style 应被剔除: %q", got)
	}
	if !strings.Contains(got, "第一段") {
		t.Fatalf("正文文本应保留: %q", got)
	}
	long := Excerpt("<p>"+strings.Repeat("字", 200)+"</p>", 120)
	if n := len([]rune(long)); n != 121 { // 120 字 + 省略号
		t.Fatalf("截断长度 = %d, want 121", n)
	}
	if !strings.HasSuffix(long, "…") {
		t.Fatalf("应以省略号结尾: %q", long)
	}
}
