import { Link, useLocation } from 'react-router-dom'

// 分页导航：保留当前查询参数（如标签筛选），只翻 page
export default function Pager({ page, totalPages }) {
  const { search } = useLocation()
  const params = new URLSearchParams(search)
  function href(p) {
    params.set('page', String(p))
    return '?' + params.toString()
  }
  if (totalPages <= 1) return null
  const nums = Array.from({ length: totalPages }, (_, i) => i + 1)
  return (
    <nav className="pager">
      {page > 1
        ? <Link className="pager-prev" to={href(page - 1)}>← 上一页</Link>
        : <span className="pager-prev disabled">← 上一页</span>}
      {nums.map((n) =>
        n === page
          ? <span key={n} className="pager-num current">{n}</span>
          : <Link key={n} className="pager-num" to={href(n)}>{n}</Link>,
      )}
      {page < totalPages
        ? <Link className="pager-next" to={href(page + 1)}>下一页 →</Link>
        : <span className="pager-next disabled">下一页 →</span>}
    </nav>
  )
}
