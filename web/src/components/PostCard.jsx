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
      <h2>
        {/* 标题本身是真实链接：屏幕阅读器与「中键新标签打开」从这里进入；
            下方的整卡覆盖层只负责扩大点击热区，aria-hidden 避免链接被重复播报 */}
        <Link to={`/post/${post.slug}`}>{post.title}</Link>
      </h2>
      <p className="card-excerpt">{post.excerpt}</p>
      <div className="card-tags">
        {post.tags_list.map((t) => (
          <Link key={t} className="tag-pill" to={`/tag/${encodeURIComponent(t)}`}># {t}</Link>
        ))}
      </div>
    </article>
  )
}
