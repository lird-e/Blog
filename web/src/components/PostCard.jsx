import { Link } from 'react-router-dom'

// 文章卡片：整卡可点进详情，日期徽章 + 摘要两行截断 + 胶囊标签
export default function PostCard({ post }) {
  return (
    <article className="post-card">
      <Link className="card-link" to={`/post/${post.slug}`} aria-hidden="true" tabIndex={-1} />
      <div className="card-date">
        <span className="date-pill">📅 {post.created_at}</span>
        {post.comment_count > 0 && <span>💬 {post.comment_count}</span>}
        {post.views > 0 && <span>👁 {post.views}</span>}
      </div>
      <h2>{post.title}</h2>
      <p className="card-excerpt">{post.excerpt}</p>
      <div className="card-tags">
        {post.tags_list.map((t) => (
          <Link key={t} className="tag-pill" to={`/tag/${encodeURIComponent(t)}`}># {t}</Link>
        ))}
      </div>
    </article>
  )
}
