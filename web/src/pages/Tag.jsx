import { useParams } from 'react-router-dom'
import Home from './Home.jsx'

// 标签页：完全复用首页列表（含分页），只是 tag 固定来自路由参数
export default function Tag() {
  const { tag } = useParams()
  return <Home tag={tag} />
}
