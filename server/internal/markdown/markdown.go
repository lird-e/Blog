// Package markdown 提供服务端 Markdown → HTML 渲染（goldmark）与摘要提取。
// 与旧静态生成器 build.py 的能力对齐：fenced code / 表格 / 代码高亮 / 标题锚点。
package markdown

import (
	"bytes"
	"html"
	"regexp"
	"strings"

	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	highlighting "github.com/yuin/goldmark-highlighting/v2"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	goldmarkhtml "github.com/yuin/goldmark/renderer/html"
)

var md = goldmark.New(
	goldmark.WithExtensions(
		extension.Table,
		extension.Strikethrough,
		extension.TaskList,
		// 代码高亮输出 chroma class（.chroma/.k/.s...），颜色由前端 CSS 提供，
		// 浅色用 github 主题、深色在 [data-theme="dark"] 下覆盖为 github-dark 配色
		highlighting.NewHighlighting(
			highlighting.WithStyle("github"),
			highlighting.WithFormatOptions(chromahtml.WithClasses(true)),
		),
	),
	goldmark.WithParserOptions(parser.WithAutoHeadingID()),
	// 允许 Markdown 内嵌原始 HTML（与 GitHub 渲染一致）：
	// about 页是手写 <p> 段落，老文章正文里有 <a id="secN"> 锚点。
	// 内容仅来自作者本人（content/ 与受 JWT 保护的管理端），非游客输入，安全可控。
	goldmark.WithRendererOptions(goldmarkhtml.WithUnsafe()),
)

// Render 渲染 Markdown，失败时降级为转义后的纯文本，保证入库内容永远可用。
func Render(source string) string {
	var buf bytes.Buffer
	if err := md.Convert([]byte(source), &buf); err != nil {
		return "<p>" + html.EscapeString(source) + "</p>"
	}
	return buf.String()
}

var (
	tagRe      = regexp.MustCompile(`<[^>]+>`)
	spaceRe    = regexp.MustCompile(`\s+`)
	noScriptRe = regexp.MustCompile(`(?is)<script[^>]*>.*?</script>`)
	noStyleRe  = regexp.MustCompile(`(?is)<style[^>]*>.*?</style>`)
)

// Excerpt 从渲染后的 HTML 提取纯文本摘要（与 build.py 逻辑一致）：
// 去标签、反转义实体、折叠空白、截断。
func Excerpt(htmlText string, limit int) string {
	text := noScriptRe.ReplaceAllString(htmlText, "")
	text = noStyleRe.ReplaceAllString(text, "")
	text = tagRe.ReplaceAllString(text, "")
	text = html.UnescapeString(text)
	text = spaceRe.ReplaceAllString(text, " ")
	text = strings.TrimSpace(text)
	r := []rune(text)
	if len(r) <= limit {
		return text
	}
	return strings.TrimRight(string(r[:limit]), " ") + "…"
}
