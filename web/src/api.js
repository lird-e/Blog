// 统一 API 封装：自动携带管理员 JWT，错误统一抛出 {status, message}
const BASE = '/api'
const TOKEN_KEY = 'admin_token'

export function getToken() {
  return localStorage.getItem(TOKEN_KEY) || ''
}

export function setToken(t) {
  if (t) localStorage.setItem(TOKEN_KEY, t)
  else localStorage.removeItem(TOKEN_KEY)
}

export async function api(path, { method = 'GET', body, signal } = {}) {
  const headers = {}
  if (body !== undefined) headers['Content-Type'] = 'application/json'
  const token = getToken()
  if (token) headers['Authorization'] = 'Bearer ' + token
  const res = await fetch(BASE + path, {
    method,
    headers,
    body: body !== undefined ? JSON.stringify(body) : undefined,
    signal,
  })
  let data = {}
  try { data = await res.json() } catch { /* 非 JSON 响应按空对象处理 */ }
  if (!res.ok) {
    // 会话过期统一处理：清除本地令牌并广播，管理页自动回到登录
    if (res.status === 401 && getToken()) {
      setToken('')
      window.dispatchEvent(new Event('wb-logout'))
    }
    const err = new Error(data.error || ('请求失败 ' + res.status))
    err.status = res.status
    throw err
  }
  return data
}

// 时间显示：RFC3339 → 本地化短格式；纯日期原样返回
export function fmtTime(s) {
  if (!s) return ''
  if (/^\d{4}-\d{2}-\d{2}$/.test(s)) return s
  const d = new Date(s)
  if (isNaN(d.getTime())) return s
  const p = (n) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())} ${p(d.getHours())}:${p(d.getMinutes())}`
}
