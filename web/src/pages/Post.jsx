import { useEffect, useMemo, useRef, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { api } from '../api.js'
import { setMeta } from '../seo.js'
import Comments from '../components/Comments.jsx'

// 文章详情：TOC 目录卡片 + 阅读时长 + 代码块装饰（mac 圆点 / 一键复制）
export default function Post() {
  const { slug } = useParams()
  const navigate = useNavigate()
  const [post, setPost] = useState(null)
  const [error, setError] = useState('')
  const contentRef = useRef(null)

  useEffect(() => {
    setPost(null)
    setError('')
    api(`/posts/${encodeURIComponent(slug)}`)
      .then((p) => {
        // 旧 slug 命中改名重定向：无感替换到新地址
        if (p.redirect_to) {
          navigate(`/post/${p.redirect_to}`, { replace: true })
          return
        }
        setPost(p)
        setMeta(`${p.title} · 我的博客`, p.excerpt)
      })
      .catch((e) => setError(e.status === 404 ? '文章不存在或已被移动。' : e.message))
    window.scrollTo(0, 0)
  }, [slug, navigate])

  // 从渲染后的 HTML 提取 h2/h3 生成目录（goldmark 已写入 heading id）
  const toc = useMemo(() => {
    if (!post?.content_html) return []
    try {
      const doc = new DOMParser().parseFromString(post.content_html, 'text/html')
      return [...doc.querySelectorAll('h2, h3')]
        .filter((h) => h.id)
        .map((h) => ({ id: h.id, text: h.textContent, level: h.tagName === 'H2' ? 2 : 3 }))
    } catch { return [] }
  }, [post])

  // 中文阅读时长：按每分钟 400 字估算
  const minutes = useMemo(() => {
    if (!post?.content_html) return 0
    const text = post.content_html.replace(/<[^>]+>/g, '').replace(/\s/g, '')
    return Math.max(1, Math.round(text.length / 400))
  }, [post])

  // 代码块增强：包一层装饰容器 + 复制按钮（内容不变，仅在文章加载后执行一次）
  useEffect(() => {
    if (!post || !contentRef.current) return
    const root = contentRef.current
    root.querySelectorAll('pre').forEach((pre) => {
      if (pre.parentElement?.classList.contains('code-block')) return
      const wrap = document.createElement('div')
      wrap.className = 'code-block'
      pre.before(wrap)
      wrap.appendChild(pre)
      const dots = document.createElement('div')
      dots.className = 'cb-dots'
      dots.innerHTML = '<i></i><i></i><i></i>'
      wrap.appendChild(dots)
      const btn = document.createElement('button')
      btn.type = 'button'
      btn.className = 'cb-copy'
      btn.textContent = '复制'
      btn.addEventListener('click', async () => {
        try {
          await navigator.clipboard.writeText(pre.innerText)
          btn.textContent = '已复制 ✓'
          btn.classList.add('copied')
          setTimeout(() => { btn.textContent = '复制'; btn.classList.remove('copied') }, 1600)
        } catch { btn.textContent = '复制失败' }
      })
      wrap.appendChild(btn)
    })
  }, [post])

  if (error) {
    return (
      <div style={{ textAlign: 'center', padding: '60px 0' }}>
        <h1>404</h1>
        <p>{error}</p>
        <Link to="/">← 返回首页</Link>
      </div>
    )
  }
  if (!post) return <p className="muted loading-hint">加载中…</p>

  return (
    <article className="post fade-in">
      <header className="post-header">
        <h1>{post.title}</h1>
        <div className="post-meta">
          <span>📅 {post.created_at}</span>
          <span className="sep">·</span>
          <span className="reading-time">⏱ 约 {minutes} 分钟</span>
          <span className="sep">·</span>
          <span>👁 {post.views} 次阅读</span>
          {post.tags_list.length > 0 && (
            <>
              <span className="sep">·</span>
              <span className="post-tags">
                {post.tags_list.map((t) => (
                  <Link key={t} className="tag-pill" to={`/tag/${encodeURIComponent(t)}`}># {t}</Link>
                ))}
              </span>
            </>
          )}
        </div>
      </header>

      {toc.length >= 3 && (
        <details className="toc-card" open>
          <summary>目录</summary>
          <ul className="toc-list">
            {toc.map((item) => (
              <li key={item.id} className={item.level === 3 ? 'lv3' : 'lv2'}>
                <a href={`#${item.id}`}>{item.text}</a>
              </li>
            ))}
          </ul>
        </details>
      )}

      {/* content_html 由服务端 goldmark 渲染入库，非用户即时输入 */}
      <div className="post-content" ref={contentRef} dangerouslySetInnerHTML={{ __html: post.content_html }} />
      <footer className="post-footer"><Link to="/">← 返回首页</Link></footer>
      <Comments slug={slug} />
    </article>
  )
}
