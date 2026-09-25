import { Routes, Route } from 'react-router-dom'
import Layout from './components/Layout.jsx'
import Home from './pages/Home.jsx'
import Post from './pages/Post.jsx'
import Tags from './pages/Tags.jsx'
import Tag from './pages/Tag.jsx'
import Search from './pages/Search.jsx'
import About from './pages/About.jsx'
import NotFound from './pages/NotFound.jsx'
import Admin from './pages/admin/Admin.jsx'

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
      <Route path="/admin/*" element={<Admin />} />
    </Routes>
  )
}
