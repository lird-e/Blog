import { useEffect, useState } from 'react'
import { api } from '../../api.js'

// 站点设置：评论模式（先发后显 / 先审后显），持久化在 SQLite settings 表
export default function AdminSettings() {
  const [mode, setMode] = useState('direct')
  const [saved, setSaved] = useState(false)
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  useEffect(() => {
    api('/admin/settings').then((d) => setMode(d.comment_mode || 'direct')).catch((e) => setError(e.message))
  }, [])

  async function submit(e) {
    e.preventDefault()
    setBusy(true)
    setSaved(false)
    try {
      await api('/admin/settings', { method: 'PUT', body: { comment_mode: mode } })
      setSaved(true)
    } catch (err) {
      setError(err.message)
    } finally {
      setBusy(false)
    }
  }

  return (
    <form className="admin-editor" onSubmit={submit}>
      <h2>设置</h2>
      <fieldset className="settings-group">
        <legend>评论模式</legend>
        <label className="radio-row">
          <input type="radio" name="comment_mode" checked={mode === 'direct'} onChange={() => setMode('direct')} />
          <div>
            <b>先发后显</b>
            <p className="muted">评论提交后立即公开显示，适合基本无垃圾信息的阶段。</p>
          </div>
        </label>
        <label className="radio-row">
          <input type="radio" name="comment_mode" checked={mode === 'review'} onChange={() => setMode('review')} />
          <div>
            <b>先审后显</b>
            <p className="muted">评论提交后进入「待审」队列，审核通过才显示；遇到垃圾评论时切换。</p>
          </div>
        </label>
      </fieldset>
      {error && <p className="form-err">{error}</p>}
      <div className="editor-actions">
        <button type="submit" className="btn btn-primary" disabled={busy}>{busy ? '保存中…' : '保存设置'}</button>
        {saved && <span className="form-ok">已保存 ✓</span>}
      </div>
    </form>
  )
}
