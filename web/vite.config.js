import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// 开发模式把 API/SEO 路径代理到本地 Go 服务；
// 构建产物 dist/ 由 Nginx 托管（或 Go 端 BLOG_WEB_DIR 兜底）。
export default defineConfig({
  plugins: [react()],
  server: {
    proxy: {
      '/api': 'http://127.0.0.1:8080',
      '/rss.xml': 'http://127.0.0.1:8080',
      '/sitemap.xml': 'http://127.0.0.1:8080',
      '/robots.txt': 'http://127.0.0.1:8080',
    },
  },
})
