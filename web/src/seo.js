// 逐页 SEO：SPA 只有一个 index.html，title / description / canonical / Open Graph
// 都得手动同步，否则分享到社交与 IM 时永远显示首页那张卡片。
const SITE_NAME = '我的博客'
const DEFAULT_DESC = '记录技术、思考与生活。'
const DEFAULT_IMAGE = '/assets/og-cover.png'

function upsertMeta(attr, key, value) {
  if (!value) return
  let el = document.head.querySelector(`meta[${attr}="${key}"]`)
  if (!el) {
    el = document.createElement('meta')
    el.setAttribute(attr, key)
    document.head.appendChild(el)
  }
  el.setAttribute('content', value)
}

// canonical 只取路径：?page= / ?login=ok 这类参数不该进入规范链接，
// 否则分页与登录回跳会被当成不同页面，权重被稀释。
function canonicalURL() {
  return window.location.origin + window.location.pathname
}

export function setMeta(title, description, { type = 'website', image } = {}) {
  const canonical = canonicalURL()
  const desc = description || DEFAULT_DESC
  const ogImage = new URL(image || DEFAULT_IMAGE, window.location.origin).href

  if (title) document.title = title
  upsertMeta('name', 'description', desc)
  upsertMeta('property', 'og:title', title || SITE_NAME)
  upsertMeta('property', 'og:description', desc)
  upsertMeta('property', 'og:type', type)
  upsertMeta('property', 'og:url', canonical)
  upsertMeta('property', 'og:image', ogImage)
  upsertMeta('property', 'og:site_name', SITE_NAME)
  upsertMeta('name', 'twitter:card', 'summary_large_image')
  upsertMeta('name', 'twitter:title', title || SITE_NAME)
  upsertMeta('name', 'twitter:description', desc)
  upsertMeta('name', 'twitter:image', ogImage)

  let link = document.head.querySelector('link[rel="canonical"]')
  if (!link) {
    link = document.createElement('link')
    link.setAttribute('rel', 'canonical')
    document.head.appendChild(link)
  }
  link.setAttribute('href', canonical)
}
