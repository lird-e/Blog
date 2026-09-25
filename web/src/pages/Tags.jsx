import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { api } from '../api.js'
import { setMeta } from '../seo.js'

export default function Tags() {
  const [tags, setTags] = useState(null)
  const [error, setError] = useState('')

  useEffect(() => {
    api('/tags').then((d) => {
      setTags(d.tags || [])
      setMeta('标签 · 我的博客')
    }).catch((e) => setError(e.message))
  }, [])

  return (
    <section className="tags-index">
      <h1>标签</h1>
      <p className="subtitle">按主题浏览文章。</p>
      {error && <p className="search-empty">加载失败：{error}</p>}
      {!error && !tags && <p className="muted loading-hint">加载中…</p>}
      {tags && (
        <div className="tag-cloud">
          {tags.length === 0 && <p className="search-empty">暂无标签。</p>}
          {tags.map((t) => {
            // 标签云加权：文章越多字号越大（0.9 ~ 1.15rem）
            const max = Math.max(...tags.map((x) => x.count), 1)
            const size = (0.9 + 0.25 * (t.count / max)).toFixed(2)
            return (
              <Link key={t.name} className="tag-cloud-item"
                style={{ fontSize: size + 'rem' }}
                to={`/tag/${encodeURIComponent(t.name)}`}>
                {t.name} <span className="tag-count">{t.count}</span>
              </Link>
            )
          })}
        </div>
      )}
    </section>
  )
}
