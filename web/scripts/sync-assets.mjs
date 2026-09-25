// 构建前把仓库根的 assets/（博客图片等）同步进 web/public/assets，
// 使文章内 /assets/images/** 链接在构建产物 dist/ 中可用（Nginx 同样命中该目录）。
import { cpSync, existsSync, mkdirSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'

const webDir = dirname(dirname(fileURLToPath(import.meta.url)))
const src = join(webDir, '..', 'assets')
const dst = join(webDir, 'public', 'assets')

if (existsSync(src)) {
  mkdirSync(dst, { recursive: true })
  cpSync(src, dst, { recursive: true })
  console.log('[sync-assets] assets/ -> web/public/assets/ 完成')
} else {
  console.log('[sync-assets] 未找到仓库 assets/，跳过')
}
