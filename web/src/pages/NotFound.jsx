import { useEffect } from 'react'
import { Link } from 'react-router-dom'
import { setMeta } from '../seo.js'

export default function NotFound() {
  useEffect(() => { setMeta('404 · 页面不存在 · 我的博客') }, [])
  return (
    <div style={{ textAlign: 'center', padding: '60px 0' }}>
      <h1>404</h1>
      <p>页面不存在或已被移动。</p>
      <Link to="/">← 返回首页</Link>
    </div>
  )
}
