#!/usr/bin/env bash
# 服务器发布脚本：拉取 GitHub 最新代码 → 构建 Go 后端 + React 前端 → 重启服务。
# 用法：ssh 上服务器后执行 /var/blog/deploy.sh
set -euo pipefail

SRC=/var/blog/src
OUT_SERVER=/var/blog/server
OUT_WEB=/var/blog/web

cd "$SRC"
echo "==> git pull (origin main)"
git pull origin main

echo "==> build go backend"
cd "$SRC/server"
go build -o "$OUT_SERVER/blog-api" ./cmd/server

echo "==> build react frontend"
cd "$SRC/web"
npm ci
npm run build
# 原子替换：先拷到临时目录再 mv，避免删除期间出现 404 窗口
rm -rf "$OUT_WEB/dist.new" "$OUT_WEB/dist.old"
cp -r "$SRC/web/dist" "$OUT_WEB/dist.new"
if [ -d "$OUT_WEB/dist" ]; then mv "$OUT_WEB/dist" "$OUT_WEB/dist.old"; fi
mv "$OUT_WEB/dist.new" "$OUT_WEB/dist"
rm -rf "$OUT_WEB/dist.old"

echo "==> restart blog-api"
systemctl restart blog-api
sleep 1
systemctl is-active blog-api >/dev/null && echo "deploy done: $(date)" || {
  echo "blog-api 启动失败，查看日志：journalctl -u blog-api -n 50"; exit 1;
}
