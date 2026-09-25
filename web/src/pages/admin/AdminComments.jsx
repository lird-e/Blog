import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { api, fmtTime } from '../../api.js'

const TABS = [
  { key: 'pending', label: '待审' },
  { key: 'approved', label: '已通过' },
  { key: 'rejected', label: '已拒绝' },
  { key: '', label: '全部' },
]

// 评论审核：按状态筛选 + 通过/驳回/删除（带分页）
export default function AdminComments() {
  const [status, setStatus] = useState('pending')
  const [data, setData] = useState(null)
  const [page, setPage] = useState(1)
  const [totalPages, setTotalPages] = useState(1)
  const [error, setError] = useState('')

  async function load(s = status, p = page) {
    try {
      const d = await api(`/admin/comments?page=${p}&page_size=20${s ? `&status=${s}` : ''}`)
      if (d.page > d.total_pages && d.total_pages > 0) { load(s, d.total_pages); return }
      setData(d)
      setPage(d.page)
      setTotalPages(d.total_pages)
      setError('')
    } catch (e) { setError(e.message) }
  }
  useEffect(() => { load('pending', 1) }, [])

  async function setStatusOf(id, s) {
    try { await api(`/admin/comments/${id}`, { method: 'PUT', body: { status: s } }); load() }
    catch (e) { setError(e.message) }
  }
  async function remove(id) {
    if (!window.confirm('确定删除这条评论？')) return
    try { await api(`/admin/comments/${id}`, { method: 'DELETE' }); load() }
    catch (e) { setError(e.message) }
  }

  return (
    <div>
      <div className="admin-toolbar">
        <h2>评论审核</h2>
        <div className="admin-tabs">
          {TABS.map((t) => (
            <button key={t.key}
              className={`btn btn-sm ${status === t.key ? 'btn-primary' : 'btn-ghost'}`}
              onClick={() => { setStatus(t.key); load(t.key, 1) }}>
              {t.label}
            </button>
          ))}
        </div>
      </div>
      {error && <p className="form-err">{error}</p>}
      {!data && !error && <p className="muted">加载中…</p>}
      {data && data.items.length === 0 && <p className="muted">没有符合条件的评论。</p>}
      <div className="comment-list">
        {data && data.items.map((c) => (
          <div key={c.id} className={`admin-comment status-${c.status}`}>
            <div className="comment-head">
              <span className="comment-nickname">{c.nickname}</span>
              <span className={`badge badge-${c.status}`}>
                {c.status === 'pending' ? '待审' : c.status === 'approved' ? '已通过' : '已拒绝'}
              </span>
              <span className="muted">评《<Link to={`/post/${c.post_slug}`} target="_blank">{c.post_title}</Link>》</span>
              <time className="comment-time">{fmtTime(c.created_at)}</time>
            </div>
            <p className="comment-content">{c.content}</p>
            <div className="admin-comment-actions">
              {c.status !== 'approved' && <button className="btn btn-sm btn-ok" onClick={() => setStatusOf(c.id, 'approved')}>通过</button>}
              {c.status !== 'rejected' && <button className="btn btn-sm btn-ghost" onClick={() => setStatusOf(c.id, 'rejected')}>拒绝</button>}
              <button className="btn btn-sm btn-danger" onClick={() => remove(c.id)}>删除</button>
            </div>
          </div>
        ))}
      </div>
      {totalPages > 1 && (
        <nav className="pager">
          <button className="pager-prev" type="button" disabled={page <= 1} onClick={() => load(status, page - 1)}>← 上一页</button>
          <span className="pager-num current">{page} / {totalPages}</span>
          <button className="pager-next" type="button" disabled={page >= totalPages} onClick={() => load(status, page + 1)}>下一页 →</button>
        </nav>
      )}
    </div>
  )
}
