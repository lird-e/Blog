import { useEffect, useRef, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { api } from '../../api.js'

// Markdown 文章编辑器：保存时服务端 goldmark 渲染入库；
// 有未保存修改时拦截标签页关闭/刷新，取消按钮二次确认；
// 草稿自动保存到 localStorage（800ms 防抖），浏览器崩溃/误退后可恢复。
const DRAFT_PREFIX = 'post_draft:'
const DRAFT_FIELDS = ['title', 'slug', 'tags', 'excerpt', 'content_md', 'published']

export default function PostEditor() {
  const { id } = useParams()
  const isNew = !id
  const navigate = useNavigate()
  const [form, setForm] = useState({
    title: '', slug: '', tags: '', excerpt: '', content_md: '', published: true,
  })
  const [error, setError] = useState('')
  const [notice, setNotice] = useState('')
  const [busy, setBusy] = useState(false)
  const dirtyRef = useRef(false)
  const draftTimer = useRef(null)
  const draftKey = DRAFT_PREFIX + (isNew ? 'new' : id)

  useEffect(() => {
    // 读草稿并判断是否真的有未保存差异（草稿与服务端原文完全一致视为过期残留）
    const readDraft = (base) => {
      try {
        const draft = JSON.parse(localStorage.getItem(draftKey))
        if (!draft) return false
        return DRAFT_FIELDS.some((k) => draft[k] !== undefined && draft[k] !== base[k])
      } catch { return false }
    }
    if (isNew) {
      // 新建：本地有上次没写完的草稿就恢复（有任一非空字段即视为草稿）
      try {
        const draft = JSON.parse(localStorage.getItem(draftKey))
        if (draft && (draft.title || draft.content_md)) {
          setForm((f) => ({ ...f, ...draft }))
          dirtyRef.current = true
          setNotice('已恢复上次未保存的草稿，保存成功后自动清除')
        }
      } catch { /* 损坏数据视为无草稿 */ }
      return
    }
    api(`/admin/posts/${id}`)
      .then((p) => {
        const base = {
          title: p.title, slug: p.slug, tags: p.tags || '', excerpt: p.excerpt || '',
          content_md: p.content_md || '', published: p.published,
        }
        if (readDraft(base)) {
          setForm({ ...base, ...JSON.parse(localStorage.getItem(draftKey)) })
          dirtyRef.current = true
          setNotice('已恢复上次未保存的草稿，保存成功后自动清除')
        } else {
          setForm(base)
        }
      })
      .catch((e) => setError(e.message))
  }, [id, draftKey])

  // 草稿自动保存：内容变更 800ms 后写入 localStorage。
  // dirtyRef 挡住首次加载——别把刚从服务端拉取的原文误存为「草稿」。
  useEffect(() => {
    if (!dirtyRef.current) return
    clearTimeout(draftTimer.current)
    draftTimer.current = setTimeout(() => {
      try { localStorage.setItem(draftKey, JSON.stringify(form)) } catch { /* 隐私模式忽略 */ }
    }, 800)
    return () => clearTimeout(draftTimer.current)
  }, [form, draftKey])

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
      // 保存成功：先停掉可能在途的草稿写入定时器，再清掉草稿
      dirtyRef.current = false
      clearTimeout(draftTimer.current)
      localStorage.removeItem(draftKey)
      navigate('/admin/posts')
    } catch (err) {
      // 保存失败时草稿仍在，刷新或重进本页可恢复
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
      {notice && <p className="form-ok">{notice}</p>}
      <div className="editor-actions">
        <button type="submit" className="btn btn-primary" disabled={busy}>{busy ? '保存中…' : '保存'}</button>
        <button type="button" className="btn btn-ghost" onClick={leave}>取消</button>
      </div>
    </form>
  )
}
