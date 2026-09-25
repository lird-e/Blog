import { Outlet, NavLink, Link } from 'react-router-dom'
import { useEffect, useState } from 'react'
import { getToken } from '../api.js'

// 站点框架：毛玻璃吸顶导航 + 阅读进度条 + 回到顶部 + 明暗切换
export default function Layout() {
  const [theme, setTheme] = useState(() => document.documentElement.getAttribute('data-theme') || 'light')
  const [logged, setLogged] = useState(!!getToken())
  const [scrolled, setScrolled] = useState(false)
  const [progress, setProgress] = useState(0)
  const [showTop, setShowTop] = useState(false)

  useEffect(() => {
    let ticking = false
    function onScroll() {
      if (ticking) return
      ticking = true
      requestAnimationFrame(() => {
        const y = window.scrollY
        const max = document.documentElement.scrollHeight - window.innerHeight
        setScrolled(y > 8)
        setShowTop(y > 420)
        setProgress(max > 0 ? Math.min(100, (y / max) * 100) : 0)
        ticking = false
      })
    }
    window.addEventListener('scroll', onScroll, { passive: true })
    onScroll()
    return () => window.removeEventListener('scroll', onScroll)
  }, [])

  useEffect(() => {
    const onAuth = () => setLogged(!!getToken())
    window.addEventListener('storage', onAuth)
    window.addEventListener('wb-logout', onAuth) // api 层 401 时广播，隐藏「写文章」入口
    return () => {
      window.removeEventListener('storage', onAuth)
      window.removeEventListener('wb-logout', onAuth)
    }
  }, [])

  function toggleTheme() {
    const next = theme === 'dark' ? 'light' : 'dark'
    document.documentElement.setAttribute('data-theme', next)
    try { localStorage.setItem('theme', next) } catch { /* 隐私模式忽略 */ }
    setTheme(next)
  }

  return (
    <>
      <div className="read-progress" style={{ width: progress + '%' }} />
      <header className={`site-header${scrolled ? ' scrolled' : ''}`}>
        <div className="container">
          <Link to="/" className="logo"><span className="logo-dot" />我的博客</Link>
          <nav>
            <NavLink to="/">首页</NavLink>
            <NavLink to="/tags">标签</NavLink>
            <NavLink to="/search">搜索</NavLink>
            <NavLink to="/about">关于</NavLink>
            {logged && <NavLink to="/admin" className="admin-nav">写文章</NavLink>}
            <button className="theme-toggle" type="button" onClick={toggleTheme}>
              {theme === 'dark' ? '☀️ 浅色' : '🌙 深色'}
            </button>
          </nav>
        </div>
      </header>
      <main className="container">
        <Outlet />
      </main>
      <footer className="site-footer">
        <div className="container">
          © {new Date().getFullYear()} 我的博客 ·
          <a href="/rss.xml">RSS</a> ·
          <NavLink to="/admin">管理</NavLink>
        </div>
      </footer>
      <button
        type="button"
        className={`back-top${showTop ? ' show' : ''}`}
        onClick={() => window.scrollTo({ top: 0, behavior: 'smooth' })}
        aria-label="回到顶部"
      >↑</button>
    </>
  )
}
