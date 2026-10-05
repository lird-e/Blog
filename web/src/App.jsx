import { lazy, Suspense } from 'react'
import { Routes, Route } from 'react-router-dom'
import Layout from './components/Layout.jsx'
import Home from './pages/Home.jsx'
import Post from './pages/Post.jsx'
import Tags from './pages/Tags.jsx'
import Tag from './pages/Tag.jsx'
import Search from './pages/Search.jsx'
import About from './pages/About.jsx'
import NotFound from './pages/NotFound.jsx'

// 管理后台按需加载：绝大多数访客永远不会进 /admin，
// 静态 import 会把登录表单、编辑器、审核台全部打进首屏 bundle。
const Admin = lazy(() => import('./pages/admin/Admin.jsx'))

export default function App() {
  return (
    <Routes>
      <Route element={<Layout />}>
        <Route index element={<Home />} />
        <Route path="post/:slug" element={<Post />} />
        <Route path="tags" element={<Tags />} />
        <Route path="tag/:tag" element={<Tag />} />
        <Route path="search" element={<Search />} />
        <Route path="about" element={<About />} />
        <Route path="*" element={<NotFound />} />
      </Route>
      <Route
        path="/admin/*"
        element={
          <Suspense fallback={<p className="muted loading-hint">加载管理后台…</p>}>
            <Admin />
          </Suspense>
        }
      />
    </Routes>
  )
}
