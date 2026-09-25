// 逐页 SEO：更新 document.title 与 meta description（SPA 单入口页，需手动同步）
export function setMeta(title, description) {
  if (title) document.title = title
  if (description) {
    const el = document.querySelector('meta[name="description"]')
    if (el) el.setAttribute('content', description)
  }
}
