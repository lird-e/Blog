import { useEffect, useState } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import { api } from '../api.js'
import { setMeta } from '../seo.js'
import PostCard from '../components/PostCard.jsx'
import Pager from '../components/Pager.jsx'

// tag prop 供 /tag/:tag 路由复用本组件；查询参数 ?tag= 供首页筛选
export default function Home({ tag: tagProp } = {}) {
  const [params] = useSearchParams()
  const page = parseInt(params.get('page') || '1', 10)
  const tag = tagProp || params.get('tag') || ''
  const [data, setData] = useState(null)
  const [error, setError] = useState('')

  useEffect(() => {
    setData(null)
    api(`/posts?page=${page}${tag ? `&tag=${encodeURIComponent(tag)}` : ''}`)
      .then((d) => {
        setData(d)
        setMeta(tag ? `标签：${tag} · 我的博客` : '我的博客 · 记录技术、思考与生活。')
      })
      .catch((e) => setError(e.message))
  }, [page, tag])

  if (error) return <p className="search-empty">加载失败：{error}</p>
  if (!data) return <p className="muted loading-hint">加载中…</p>

  return (
    <>
      {page === 1 && !tag && (
        <section className="hero fade-in">
          <h1>欢迎来到<span className="gradient-text">我的博客</span></h1>
          <p className="subtitle">记录技术、思考与生活。</p>
          <div className="hero-line" />
          <div className="hero-social">
            <a href="https://github.com/lird-e" target="_blank" rel="noreferrer">
              <svg viewBox="0 0 16 16" aria-hidden="true"><path d="M8 0C3.58 0 0 3.58 0 8c0 3.54 2.29 6.53 5.47 7.59.4.07.55-.17.55-.38 0-.19-.01-.82-.01-1.49-2.01.37-2.53-.49-2.69-.94-.09-.23-.48-.94-.82-1.13-.28-.15-.68-.52-.01-.53.63-.01 1.08.58 1.23.82.72 1.21 1.87.87 2.33.66.07-.52.28-.87.51-1.07-1.78-.2-3.64-.89-3.64-3.95 0-.87.31-1.59.82-2.15-.08-.2-.36-1.02.08-2.12 0 0 .67-.21 2.2.82.64-.18 1.32-.27 2-.27s1.36.09 2 .27c1.53-1.04 2.2-.82 2.2-.82.44 1.1.16 1.92.08 2.12.51.56.82 1.27.82 2.15 0 3.07-1.87 3.75-3.65 3.95.29.25.54.73.54 1.48 0 1.07-.01 1.93-.01 2.2 0 .21.15.46.55.38A8.01 8.01 0 0 0 16 8c0-4.42-3.58-8-8-8Z" /></svg>
              GitHub
            </a>
            <a href="/rss.xml">
              <svg viewBox="0 0 16 16" aria-hidden="true"><path d="M2 2v2c5.52 0 10 4.48 10 10h2C14 7.37 8.63 2 2 2Zm0 4v2c3.31 0 6 2.69 6 6h2c0-4.42-3.58-8-8-8Zm1.5 6a1.5 1.5 0 1 0 0 3 1.5 1.5 0 0 0 0-3Z" /></svg>
              RSS
            </a>
            <a href="https://gitee.com/manboman" target="_blank" rel="noreferrer">Gitee</a>
          </div>
        </section>
      )}
      {tag && (
        <section className="tag-page">
          <h1>标签：{tag}</h1>
          <p className="subtitle">{data.total} 篇文章 · <Link className="back-link" to="/tags">查看全部标签</Link></p>
        </section>
      )}
      <section className="post-list">
        {data.items.length === 0 ? <p className="search-empty">没有找到文章。</p>
          : data.items.map((p) => <PostCard key={p.id} post={p} />)}
      </section>
      <Pager page={data.page} totalPages={data.total_pages} />
    </>
  )
}
