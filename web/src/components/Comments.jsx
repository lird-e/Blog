import { useEffect, useMemo, useRef, useState } from 'react'
import { api, fmtTime } from '../api.js'

// 评论区：楼中楼（两级）+ 游客表单（昵称必填、邮箱选填仅存哈希）。
// 防垃圾配合后端：website 为蜜罐隐藏字段，ts 为表单渲染时刻（毫秒）。
function avatarURL(hash) {
  return hash ? `https://cravatar.cn/avatar/${hash}?d=mp&s=48` : null
}

function CommentNode({ comment, onReply }) {
  return (
    <div className="comment" id={`comment-${comment.id}`}>
      <div className="comment-head">
        {avatarURL(comment.email_hash)
          ? <img className="comment-avatar" src={avatarURL(comment.email_hash)} alt="" loading="lazy" />
          : <span className="comment-avatar comment-avatar-fallback">{comment.nickname.slice(0, 1).toUpperCase()}</span>}
        <span className="comment-nickname">{comment.nickname}</span>
        <time className="comment-time">{fmtTime(comment.created_at)}</time>
        <button type="button" className="comment-reply-btn" onClick={() => onReply(comment)}>回复</button>
      </div>
      <p className="comment-content">{comment.content}</p>
      {comment.children?.length > 0 && (
        <div className="comment-children">
          {comment.children.map((c) => <CommentNode key={c.id} comment={c} onReply={onReply} />)}
        </div>
      )}
    </div>
  )
}

export default function Comments({ slug }) {
  const [items, setItems] = useState([])
  const [loading, setLoading] = useState(true)
  const [replyTo, setReplyTo] = useState(null)
  const [sending, setSending] = useState(false)
  const [message, setMessage] = useState('')
  const [error, setError] = useState('')
  const mountedAt = useRef(Date.now())
  const formRef = useRef(null)

  async function load() {
    setLoading(true)
    try {
      const data = await api(`/posts/${encodeURIComponent(slug)}/comments`)
      setItems(data.items || [])
    } catch {
      setItems([])
    } finally {
      setLoading(false)
    }
  }
  useEffect(() => { load() }, [slug])

  // 扁平列表 → 楼中楼树
  const tree = useMemo(() => {
    const map = new Map(items.map((c) => [c.id, { ...c, children: [] }]))
    const roots = []
    for (const c of map.values()) {
      if (c.parent_id && map.has(c.parent_id)) map.get(c.parent_id).children.push(c)
      else roots.push(c)
    }
    return roots
  }, [items])

  function onReply(comment) {
    setReplyTo(comment)
    setMessage('')
    setError('')
    formRef.current?.scrollIntoView({ behavior: 'smooth', block: 'center' })
  }

  async function submit(e) {
    e.preventDefault()
    const fd = new FormData(e.target)
    setError('')
    setSending(true)
    try {
      const res = await api(`/posts/${encodeURIComponent(slug)}/comments`, {
        method: 'POST',
        body: {
          nickname: fd.get('nickname') || '',
          email: fd.get('email') || '',
          content: fd.get('content') || '',
          parent_id: replyTo ? replyTo.id : 0,
          website: fd.get('website') || '', // 蜜罐
          ts: mountedAt.current,
        },
      })
      setMessage(res.message || '评论成功')
      setReplyTo(null)
      e.target.reset()
      await load()
    } catch (err) {
      setError(err.message)
    } finally {
      setSending(false)
    }
  }

  return (
    <section className="comments" id="comments">
      <h2 className="comments-title">评论 {items.length > 0 && <span className="tag-count">{items.length}</span>}</h2>

      {loading ? <p className="muted">加载中…</p>
        : tree.length === 0 ? <p className="muted">还没有评论，来说点什么吧～</p>
        : <div className="comment-list">{tree.map((c) => <CommentNode key={c.id} comment={c} onReply={onReply} />)}</div>}

      <form ref={formRef} className="comment-form" onSubmit={submit}>
        {replyTo && (
          <div className="replying">
            回复 <b>{replyTo.nickname}</b>：
            <button type="button" className="link-btn" onClick={() => setReplyTo(null)}>取消</button>
          </div>
        )}
        <div className="comment-form-row">
          <input name="nickname" placeholder="昵称 *（1~30 字符）" required maxLength={30} />
          <input name="email" type="email" placeholder="邮箱（选填，仅用于头像，不公开）" />
          {/* 蜜罐字段：普通用户不可见 */}
          <input name="website" tabIndex={-1} autoComplete="off" className="hp-field" aria-hidden="true" />
        </div>
        <textarea name="content" rows={4} placeholder="支持 Markdown 观点交流，1~2000 字符" required maxLength={2000} />
        <div className="comment-form-actions">
          <span className="comment-hint">邮箱仅存哈希用于生成头像 · IP 仅存加盐哈希 · 请友善发言</span>
          <button type="submit" disabled={sending}>{sending ? '提交中…' : '发表评论'}</button>
        </div>
        {message && <p className="form-ok">{message}</p>}
        {error && <p className="form-err">{error}</p>}
      </form>
    </section>
  )
}
