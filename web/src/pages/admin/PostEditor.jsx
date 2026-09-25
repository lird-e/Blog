import { useEffect, useRef, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { api } from '../../api.js'

// Markdown 文章编辑器：保存时服务端 goldmark 渲染入库；
// 有未保存修改时拦截标签页关闭/刷新，取消按钮二次确认
export default function PostEditor() {
  const { id } = useParams()
  const isNew = !id
  const navigate = useNavigate()
  const [form, setForm] = useState({
    title: '', slug: '', tags: '', excerpt: '', content_md: '', published: true,
  })
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  const dirtyRef = useRef(false)

  useEffect(() => {
    if (isNew) return
    api(`/admin/posts/${id}`)
      .then((p) => setForm({
        title: p.title, slug: p.slug, tags: p.tags || '', excerpt: p.excerpt || '',
        content_md: p.content_md || '', published: p.published,
      }))
      .catch((e) => setError(e.message))
  }, [id])

  // 有未保存修改时拦截标签页关闭/刷新（SPA 应用内跳转由取消按钮二次确认兜底）
  useEffect(() => {
    const onBeforeUnload = (e) => {
      if (!dirtyRef.current) return
      e.preventDefault()
      e.returnValue = ''
    }
    window.addEventListener('beforeunload', onBeforeUnload)
    return () => window.removeEventListener('beforeunload', onBeforeUnload)
  }, [])

  function set(key, value) {
    dirtyRef.current = true
    setForm((f) => ({ ...f, [key]: value }))
  }

  function leave() {
    if (!dirtyRef.current || window.confirm('有未保存的修改，确定离开？')) navigate('/admin/posts')
  }

  async function submit(e) {
    e.preventDefault()
    setError('')
    setBusy(true)
    try {
      if (isNew) await api('/admin/posts', { method: 'POST', body: form })
      else await api(`/admin/posts/${id}`, { method: 'PUT', body: form })
      dirtyRef.current = false
      navigate('/admin/posts')
    } catch (err) {
      setError(err.message)
    } finally {
      setBusy(false)
    }
  }

  return (
    <form className="admin-editor" onSubmit={submit}>
      <h2>{isNew ? '新建文章' : '编辑文章'}</h2>
      <label>标题 *
        <input value={form.title} onChange={(e) => set('title', e.target.value)} required />
      </label>
      <div className="editor-grid">
        <label>slug（留空按标题自动生成，改slug会改变链接）
          <input value={form.slug} onChange={(e) => set('slug', e.target.value)} placeholder="如 go-gin-notes" />
        </label>
        <label>标签（逗号分隔）
          <input value={form.tags} onChange={(e) => set('tags', e.target.value)} placeholder="网络, 随笔" />
        </label>
      </div>
      <label>摘要（留空自动从正文提取 120 字）
        <textarea rows={2} value={form.excerpt} onChange={(e) => set('excerpt', e.target.value)} />
      </label>
      <label>正文（Markdown）
        <textarea className="editor-md" rows={20} value={form.content_md} onChange={(e) => set('content_md', e.target.value)} required />
      </label>
      <label className="checkbox-row">
        <input type="checkbox" checked={form.published} onChange={(e) => set('published', e.target.checked)} />
        立即发布（取消勾选则为草稿）
      </label>
      {error && <p className="form-err">{error}</p>}
      <div className="editor-actions">
        <button type="submit" className="btn btn-primary" disabled={busy}>{busy ? '保存中…' : '保存'}</button>
        <button type="button" className="btn btn-ghost" onClick={leave}>取消</button>
      </div>
    </form>
  )
}
