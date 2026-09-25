import { useEffect, useState } from 'react'
import { Link, NavLink, Route, Routes, useNavigate } from 'react-router-dom'
import { api, getToken, setToken } from '../../api.js'
import AdminPosts from './AdminPosts.jsx'
import PostEditor from './PostEditor.jsx'
import AdminComments from './AdminComments.jsx'
import AdminSettings from './AdminSettings.jsx'

// 管理后台入口：未登录显示登录表单，登录后是 文章/评论/设置 三个子页
export default function Admin() {
  const [authed, setAuthed] = useState(!!getToken())
  const [checking, setChecking] = useState(authed)

  // 有旧 token 时先验证一次，避免进入后每个请求都 401
  useEffect(() => {
    if (!authed) return
    api('/admin/posts?page=1&page_size=1')
      .then(() => setChecking(false))
      .catch((e) => {
        if (e.status === 401) { setToken(''); setAuthed(false) }
        setChecking(false)
      })
  }, [authed])

  // 会话过期（api 层统一广播 wb-logout）时回到登录页
  useEffect(() => {
    const onLogout = () => { setAuthed(false); setChecking(false) }
    window.addEventListener('wb-logout', onLogout)
    return () => window.removeEventListener('wb-logout', onLogout)
  }, [])

  if (!authed) return <Login onLogin={() => setAuthed(true)} />
  if (checking) return <p className="muted loading-hint" style={{ padding: 40 }}>验证登录状态…</p>
  return <Panel />
}

function Login({ onLogin }) {
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  const navigate = useNavigate()

  async function submit(e) {
    e.preventDefault()
    setError('')
    setBusy(true)
    try {
      const res = await api('/admin/login', { method: 'POST', body: { username, password } })
      setToken(res.token)
      onLogin()
    } catch (err) {
      setError(err.message)
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="admin-login-wrap">
      <form className="admin-login" onSubmit={submit}>
        <h1>管理后台</h1>
        <label>用户名
          <input value={username} onChange={(e) => setUsername(e.target.value)} autoComplete="username" required />
        </label>
        <label>密码
          <input type="password" value={password} onChange={(e) => setPassword(e.target.value)} autoComplete="current-password" required />
        </label>
        {error && <p className="form-err">{error}</p>}
        <button type="submit" disabled={busy}>{busy ? '登录中…' : '登录'}</button>
        <Link to="/" className="back-link">← 返回博客</Link>
      </form>
    </div>
  )
}

function Panel() {
  const navigate = useNavigate()
  function logout() {
    setToken('')
    navigate('/admin')
    navigate(0) // 刷新整体状态
  }
  return (
    <div className="admin-page">
      <header className="admin-topbar">
        <div className="container admin-topbar-inner">
          <b>我的博客 · 管理后台</b>
          <nav className="admin-tabs">
            <NavLink to="/admin/posts">文章</NavLink>
            <NavLink to="/admin/comments">评论审核</NavLink>
            <NavLink to="/admin/settings">设置</NavLink>
          </nav>
          <div>
            <Link to="/" className="back-link">查看博客</Link>
            <button type="button" className="btn btn-ghost" onClick={logout}>退出</button>
          </div>
        </div>
      </header>
      <main className="container admin-main">
        <Routes>
          <Route index element={<AdminPosts />} />
          <Route path="posts" element={<AdminPosts />} />
          <Route path="posts/new" element={<PostEditor />} />
          <Route path="posts/:id" element={<PostEditor />} />
          <Route path="comments" element={<AdminComments />} />
          <Route path="settings" element={<AdminSettings />} />
        </Routes>
      </main>
    </div>
  )
}
