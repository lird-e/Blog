import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { api } from '../../api.js'

// 文章管理列表：新建 / 编辑 / 删除 / 快速切换发布状态（带分页）
export default function AdminPosts() {
  const [data, setData] = useState(null)
  const [page, setPage] = useState(1)
  const [totalPages, setTotalPages] = useState(1)
  const [q, setQ] = useState('')
  const [error, setError] = useState('')

  async function load(p = page, query = q) {
    try {
      const d = await api(`/admin/posts?page=${p}&page_size=20${query.trim() ? `&q=${encodeURIComponent(query.trim())}` : ''}`)
      if (d.page > d.total_pages && d.total_pages > 0) { load(d.total_pages, query); return }
      setData(d)
      setPage(d.page)
      setTotalPages(d.total_pages)
      setError('')
    } catch (e) { setError(e.message) }
  }
  useEffect(() => { load(1, '') }, [])

  async function togglePublish(p) {
    // 专用接口只更新 published，避免全量 PUT 需要正文
    try {
      await api(`/admin/posts/${p.id}/published`, { method: 'PUT', body: { published: !p.published } })
      load()
    } catch (e) { setError(e.message) }
  }

  async function remove(p) {
    if (!window.confirm(`确定删除《${p.title}》？其评论将一并删除，且不可恢复。`)) return
    try { await api(`/admin/posts/${p.id}`, { method: 'DELETE' }); load() }
    catch (e) { setError(e.message) }
  }

  function search() { load(1) }

  return (
    <div>
      <div className="admin-toolbar">
        <h2>文章</h2>
        <input
          className="admin-search"
          placeholder="按标题/内容搜索…"
          value={q}
          onChange={(e) => setQ(e.target.value)}
          onKeyDown={(e) => e.key === 'Enter' && search()}
        />
        <button className="btn" onClick={search}>搜索</button>
        <Link className="btn btn-primary" to="/admin/posts/new">+ 新建文章</Link>
      </div>
      {error && <p className="form-err">{error}</p>}
      {!data && !error && <p className="muted">加载中…</p>}
      {data && (
        <table className="admin-table">
          <thead>
            <tr><th>标题</th><th>slug</th><th>日期</th><th>浏览</th><th>评论</th><th>状态</th><th>操作</th></tr>
          </thead>
          <tbody>
            {data.items.map((p) => (
              <tr key={p.id}>
                <td><Link to={`/admin/posts/${p.id}`}>{p.title}</Link></td>
                <td className="mono">{p.slug}</td>
                <td>{p.created_at}</td>
                <td>{p.views}</td>
                <td>{p.comment_count}</td>
                <td>
                  <button className={`btn btn-sm ${p.published ? 'btn-ok' : 'btn-ghost'}`} onClick={() => togglePublish(p)}>
                    {p.published ? '已发布' : '草稿'}
                  </button>
                </td>
                <td>
                  <Link className="btn btn-sm btn-ghost" to={`/post/${p.slug}`} target="_blank">预览</Link>{' '}
                  <button className="btn btn-sm btn-danger" onClick={() => remove(p)}>删除</button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
      {data && data.items.length === 0 && <p className="muted">还没有文章，点右上角「新建文章」开始写作。</p>}
      {totalPages > 1 && (
        <nav className="pager">
          <button className="pager-prev" type="button" disabled={page <= 1} onClick={() => load(page - 1)}>← 上一页</button>
          <span className="pager-num current">{page} / {totalPages}</span>
          <button className="pager-next" type="button" disabled={page >= totalPages} onClick={() => load(page + 1)}>下一页 →</button>
        </nav>
      )}
    </div>
  )
}
