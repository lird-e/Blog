import { useEffect, useState } from 'react'
import { api } from '../api.js'
import { setMeta } from '../seo.js'

// 关于页：数据同文章链路（migrate 时 slug 固定为 about）
export default function About() {
  const [post, setPost] = useState(null)
  const [error, setError] = useState('')

  useEffect(() => {
    api('/posts/about').then((p) => {
      setPost(p)
      setMeta(`${p.title} · 我的博客`, p.excerpt)
    }).catch((e) => setError(e.message))
  }, [])

  if (error) return <p className="search-empty">关于页暂未就绪：{error}</p>
  if (!post) return <p className="muted loading-hint">加载中…</p>

  return (
    <article className="post">
      <header className="post-header">
        <h1>{post.title}</h1>
      </header>
      <div className="post-content" dangerouslySetInnerHTML={{ __html: post.content_html }} />
    </article>
  )
}
