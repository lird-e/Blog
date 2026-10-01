import { useEffect, useMemo, useRef, useState } from 'react'
import { api, fmtTime } from '../api.js'

// 评论区：楼中楼（两级）+ 双身份体系。
// - GitHub 登录（档位 B）：/api/me 判断登录态，HttpOnly Cookie 会话，提交时后端自动采用身份；
// - 游客 + 本机身份记忆（档位 A）：昵称 + email_hash 存 localStorage，邮箱明文不落盘。
// 防垃圾配合后端：website 为蜜罐隐藏字段，ts 为表单渲染时刻（毫秒），登录态同样携带。
function avatarSrc(c) {
  if (c.avatar_url) return c.avatar_url // GitHub 登录评论的头像快照
  return c.email_hash ? `https://cravatar.cn/avatar/${c.email_hash}?d=mp&s=48` : null
}

const PROFILE_KEY = 'commenter'

function loadProfile() {
  try {
    const p = JSON.parse(localStorage.getItem(PROFILE_KEY))
    if (p && typeof p.nickname === 'string' && p.nickname) return p
  } catch { /* 损坏数据视为无身份 */ }
  return null
}

function CommentNode({ comment, onReply }) {
  const src = avatarSrc(comment)
  return (
    <div className="comment" id={`comment-${comment.id}`}>
      <div className="comment-head">
        {src
          ? <img className="comment-avatar" src={src} alt="" loading="lazy" />
          : <span className="comment-avatar comment-avatar-fallback">{comment.nickname.slice(0, 1).toUpperCase()}</span>}
        {/* provider 由后端带出（游客为空）：比用 avatar_url 是否存在判断更可靠，
            因为游客填了邮箱也有 Cravatar 头像 */}
        <span className="comment-nickname" title={comment.provider ? '登录用户' : undefined}>{comment.nickname}</span>
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

// loginNotice 把回跳 URL 上的 login 参数翻译成用户可读提示。
// 后端在授权成功/取消时分别回跳 ?login=ok / ?login=cancelled。
function loginNotice(value) {
  if (value === 'ok') return { type: 'ok', text: 'GitHub 登录成功' }
  if (value === 'cancelled') return { type: 'err', text: '已取消 GitHub 授权' }
  return null
}

export default function Comments({ slug }) {
  const [items, setItems] = useState([])
  const [loading, setLoading] = useState(true)
  const [replyTo, setReplyTo] = useState(null)
  const [sending, setSending] = useState(false)
  const [message, setMessage] = useState('')
  const [error, setError] = useState('')
  const [profile, setProfile] = useState(loadProfile)
  const [me, setMe] = useState(null) // { commenter: {...} | null }，GitHub 登录态
  const [redirecting, setRedirecting] = useState(false) // 正在跳转 GitHub
  const [notice, setNotice] = useState(null) // 登录结果提示 { type, text }
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

  async function loadMe() {
    try { setMe(await api('/me')) } catch { setMe({ commenter: null }) }
  }
  useEffect(() => { load(); loadMe() }, [slug])

  // 读取 OAuth 回跳带来的 login 参数并提示，随后从地址栏清掉，
  // 避免刷新后重复提示、也避免污染用户可分享的链接。
  useEffect(() => {
    const params = new URLSearchParams(window.location.search)
    const n = loginNotice(params.get('login'))
    if (!n) return
    setNotice(n)
    params.delete('login')
    const qs = params.toString()
    window.history.replaceState({}, '',
      window.location.pathname + (qs ? '?' + qs : '') + window.location.hash)
  }, [])

  const isLogin = !!me?.commenter

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

  // 清除本机游客身份（档位 A）：表单一并重置
  function clearProfile() {
    localStorage.removeItem(PROFILE_KEY)
    setProfile(null)
    if (formRef.current?.nickname) formRef.current.nickname.value = ''
    if (formRef.current?.email) formRef.current.email.value = ''
  }

  // 退出 GitHub 登录：服务端吊销令牌并清 Cookie，本地态同步
  async function logout() {
    try { await api('/auth/logout', { method: 'POST' }) } catch { /* Cookie 本已失效时忽略 */ }
    setMe({ commenter: null })
    setNotice(null) // 清掉可能残留的「登录成功」提示
  }

  const loginURL = `/api/auth/github/login?redirect=${encodeURIComponent(window.location.pathname + window.location.search)}`

  async function submit(e) {
    e.preventDefault()
    const fd = new FormData(e.target)
    const nickname = (fd.get('nickname') || '').trim()
    const email = (fd.get('email') || '').trim()
    setError('')
    setSending(true)
    try {
      const res = await api(`/posts/${encodeURIComponent(slug)}/comments`, {
        method: 'POST',
        body: {
          // GitHub 登录态下表单无昵称/邮箱输入，后端以会话身份为准
          nickname,
          email,
          // 游客：未填新邮箱时带上记忆的头像哈希；填了则以新邮箱为准
          email_hash: !isLogin && !email && profile?.email_hash ? profile.email_hash : '',
          content: fd.get('content') || '',
          parent_id: replyTo ? replyTo.id : 0,
          website: fd.get('website') || '', // 蜜罐
          ts: mountedAt.current,
        },
      })
      setMessage(res.message || '评论成功')
      setReplyTo(null)
      if (!isLogin) {
        // 游客：保存本机身份（昵称 + 头像哈希，不存邮箱明文）
        const next = { nickname, email_hash: res.email_hash || '' }
        localStorage.setItem(PROFILE_KEY, JSON.stringify(next))
        setProfile(next)
      }
      e.target.reset()
      if (!isLogin && formRef.current && profile?.nickname) {
        formRef.current.nickname.value = profile.nickname
      }
      await load()
      // 先发后显模式下滚动定位到新评论；先审后显暂不可见则不滚
      if (res.status === 'approved' && res.id) {
        setTimeout(() => {
          document.getElementById(`comment-${res.id}`)
            ?.scrollIntoView({ behavior: 'smooth', block: 'center' })
        }, 150)
      }
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
        {/* OAuth 回跳提示：取消授权时用户此前完全无感知，会以为按钮坏了 */}
        {notice && (
          <p className={notice.type === 'ok' ? 'form-ok' : 'form-err'} role="status">
            {notice.text}
          </p>
        )}
        {isLogin ? (
          <div className="comment-identity">
            {me.commenter.avatar_url
              ? <img className="comment-avatar" src={me.commenter.avatar_url} alt="" />
              : <span className="comment-avatar comment-avatar-fallback">
                  {(me.commenter.nickname || '?').slice(0, 1).toUpperCase()}
                </span>}
            以 GitHub 账号 <b>{me.commenter.nickname}</b> 身份评论
            <button type="button" className="link-btn" onClick={logout}>退出</button>
          </div>
        ) : (
          <>
            {!me && <p className="muted comment-identity-loading">检查登录状态…</p>}
            {me && !me.commenter && (
              <div className="comment-oauth">
                <a
                  className="gh-login-btn"
                  href={loginURL}
                  aria-busy={redirecting}
                  onClick={() => setRedirecting(true)}
                >
                  <svg width="16" height="16" viewBox="0 0 16 16" fill="currentColor" aria-hidden="true"><path d="M8 0C3.58 0 0 3.58 0 8c0 3.54 2.29 6.53 5.47 7.59.4.07.55-.17.55-.38 0-.19-.01-.82-.01-1.49-2.01.37-2.53-.49-2.69-.94-.09-.23-.48-.94-.82-1.13-.28-.15-.68-.52-.01-.53.63-.01 1.08.58 1.23.82.72 1.21 1.87.87 2.33.66.07-.52.28-.87.51-1.07-1.78-.2-3.64-.89-3.64-3.95 0-.87.31-1.59.82-2.15-.08-.2-.36-1.02.08-2.12 0 0 .67-.21 2.2.82.64-.18 1.32-.27 2-.27s1.36.09 2 .27c1.53-1.04 2.2-.82 2.2-.82.44 1.1.16 1.92.08 2.12.51.56.82 1.27.82 2.15 0 3.07-1.87 3.75-3.65 3.95.29.25.54.73.54 1.48 0 1.07-.01 1.93-.01 2.2 0 .21.15.46.55.38A8.01 8.01 0 0 0 16 8c0-4.42-3.58-8-8-8Z"/></svg>
                  {redirecting ? '正在跳转 GitHub…' : '使用 GitHub 登录'}
                </a>
                <span className="muted">登录后免填昵称，头像自动同步</span>
              </div>
            )}
            {profile && me && !me.commenter && (
              <div className="comment-identity">
                以 <b>{profile.nickname}</b> 身份评论 · 头像已记住
                <button type="button" className="link-btn" onClick={clearProfile}>换个身份</button>
              </div>
            )}
            {me && !me.commenter && (
              <div className="comment-form-row">
                <input name="nickname" placeholder="昵称 *（1~30 字符）" required maxLength={30}
                  defaultValue={profile?.nickname || ''} />
                <input name="email" type="email" placeholder={profile?.email_hash
                  ? '头像已记住 · 填写新邮箱可更换'
                  : '邮箱（选填，仅用于头像，不公开）'} />
              </div>
            )}
          </>
        )}
        {replyTo && (
          <div className="replying">
            回复 <b>{replyTo.nickname}</b>：
            <button type="button" className="link-btn" onClick={() => setReplyTo(null)}>取消</button>
          </div>
        )}
        {/* 蜜罐字段：任何身份下均渲染（普通用户不可见） */}
        <input name="website" tabIndex={-1} autoComplete="off" className="hp-field" aria-hidden="true" />
        <textarea name="content" rows={4} placeholder="请友善发言，1~2000 字符" required maxLength={2000} />
        <div className="comment-form-actions">
          <span className="comment-hint">邮箱仅存哈希用于头像 · IP 仅存加盐哈希 · 请友善发言</span>
          <button type="submit" disabled={sending || !me}>{sending ? '提交中…' : '发表评论'}</button>
        </div>
        {message && <p className="form-ok">{message}</p>}
        {error && <p className="form-err">{error}</p>}
      </form>
    </section>
  )
}
