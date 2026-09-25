import { useEffect, useRef, useState } from 'react'
import { api } from '../api.js'
import { setMeta } from '../seo.js'
import PostCard from '../components/PostCard.jsx'

// 搜索页：关键词 LIKE 检索（后端多关键词 AND），输入防抖 300ms；
// 用 AbortController 取消上一笔未完成请求，防止慢响应晚到覆盖新结果
export default function Search() {
  const [q, setQ] = useState('')
  const [results, setResults] = useState(null)
  const [loading, setLoading] = useState(false)
  const timer = useRef(null)

  useEffect(() => { setMeta('搜索 · 我的博客') }, [])

  useEffect(() => {
    clearTimeout(timer.current)
    if (!q.trim()) { setResults(null); setLoading(false); return }
    const ctrl = new AbortController()
    timer.current = setTimeout(() => {
      setLoading(true)
      api(`/posts?q=${encodeURIComponent(q.trim())}&page_size=50`, { signal: ctrl.signal })
        .then((d) => setResults(d.items || []))
        .catch((e) => { if (e.name !== 'AbortError') setResults([]) })
        .finally(() => { if (!ctrl.signal.aborted) setLoading(false) })
    }, 300)
    return () => { clearTimeout(timer.current); ctrl.abort() }
  }, [q])

  return (
    <section className="search-page">
      <h1>搜索</h1>
      <p className="subtitle">输入关键词，检索文章标题、标签、摘要与正文。</p>
      <input
        type="search"
        className="search-input"
        placeholder="输入关键词，例如 网络、AI、写作…"
        autoFocus
        value={q}
        onChange={(e) => setQ(e.target.value)}
      />
      <div className="search-results">
        {loading && <p className="muted">搜索中…</p>}
        {!loading && q.trim() && results && results.length === 0 && <p className="search-empty">没有找到相关文章。</p>}
        {!loading && results && results.map((p) => <PostCard key={p.id} post={p} />)}
      </div>
    </section>
  )
}
