import React from 'react'

// 全局错误边界：渲染期异常时给出可恢复的提示，而不是整页白屏。
// 注意：异步请求失败不经过这里（Promise 异常由各页面自行处理），
// 它只兜底 React 渲染树抛出的错误（如脏数据导致的渲染崩溃）。
export default class ErrorBoundary extends React.Component {
  constructor(props) {
    super(props)
    this.state = { hasError: false }
  }

  static getDerivedStateFromError() {
    return { hasError: true }
  }

  componentDidCatch(err) {
    console.error('页面渲染异常:', err)
  }

  render() {
    if (!this.state.hasError) return this.props.children
    return (
      <div style={{ textAlign: 'center', padding: '80px 0' }}>
        <h1>页面出错了</h1>
        <p className="muted">渲染时发生异常，刷新页面一般即可恢复。</p>
        <button className="btn btn-primary" type="button" onClick={() => window.location.reload()}>
          刷新页面
        </button>
      </div>
    )
  }
}
