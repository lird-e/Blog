# 云服务器部署手册（Ubuntu · IP 直连）

> 对应《博客重构与云服务器部署方案》第八、九章。按顺序执行，一次部署约 30 分钟。

## 0. 前置

- Ubuntu 20.04+ 云服务器（Ubuntu 22.04 为例），有 root 或 sudo 权限
- 本地已有 GitHub 仓库 `lird-e/Blog`（main 分支）

## 1. 服务器初始化（方案 阶段 0）

```bash
# 1.1 专用用户（Go 服务不用 root 跑）
sudo adduser --disabled-password --gecos "" blog

# 1.2 目录规划
sudo mkdir -p /var/blog/{src,server,web,backups}
sudo chown -R blog:blog /var/blog

# 1.3 防火墙：仅放行 SSH 与 HTTP
sudo ufw allow 22/tcp
sudo ufw allow 80/tcp
sudo ufw enable
# ⚠ 同步在云控制台安全组放行 22/80（有的厂商默认全开，建议也收紧）

# 1.4 SSH 加固（建议先用密钥登录验证成功后再禁密码）
sudo sed -i 's/^#\?PasswordAuthentication.*/PasswordAuthentication no/' /etc/ssh/sshd_config
sudo sed -i 's/^#\?PermitRootLogin.*/PermitRootLogin no/' /etc/ssh/sshd_config
sudo systemctl restart ssh

# 1.5 安装依赖
sudo apt update
sudo apt install -y nginx git sqlite3
# Go（amd64 示例，按实际架构调整版本号）
wget https://golang.google.cn/dl/go1.24.6.linux-amd64.tar.gz
sudo rm -rf /usr/local/go && sudo tar -C /usr/local -xzf go1.24.6.linux-amd64.tar.gz
# Node 22（Vite 构建用）
curl -fsSL https://deb.nodesource.com/setup_22.x | sudo -E bash -
sudo apt install -y nodejs
# 写入环境
sudo tee -a /etc/profile.d/dev.sh <<'EOF'
export PATH=$PATH:/usr/local/go/bin
export GOPROXY=https://goproxy.cn,direct
EOF
source /etc/profile.d/dev.sh
```

## 2. 代码同步：GitHub 单远端

服务器直接克隆 GitHub 仓库（只读拉取，不在服务器上 push）：

```bash
sudo -u blog git clone https://github.com/lird-e/Blog.git /var/blog/src
cd /var/blog/src && sudo -u blog git remote set-url --push origin DISABLED
```

> 国内服务器直连 GitHub 可能偏慢或不稳定，可给 git 配置代理（如服务器上有可用代理：
> `sudo -u blog git config --global http.proxy http://127.0.0.1:端口`），
> 或退回「GitHub push + Gitee 镜像 pull」的双远端方案。

## 3. 安装 Nginx 配置与 systemd

```bash
# 3.1 Nginx 限流区域：在 /etc/nginx/nginx.conf 的 http {} 块内加入
#   limit_req_zone $binary_remote_addr zone=api:10m   rate=10r/s;
#   limit_req_zone $binary_remote_addr zone=admin:10m rate=1r/s;
sudo cp /var/blog/src/deploy/nginx-blog.conf /etc/nginx/sites-available/blog
sudo ln -sf /etc/nginx/sites-available/blog /etc/nginx/sites-enabled/blog
sudo rm -f /etc/nginx/sites-enabled/default
sudo nginx -t && sudo systemctl reload nginx

# 3.2 systemd
sudo cp /var/blog/src/deploy/blog-api.service /etc/systemd/system/
```

## 4. 数据库初始化与管理员配置

```bash
# 4.1 首次迁移文章（29 篇 + 关于页 → SQLite）
cd /var/blog/src/server
sudo -u blog env GOPROXY=https://goproxy.cn,direct \
  bash -c 'export PATH=$PATH:/usr/local/go/bin; go run ./cmd/migrate -content ../content -db /var/blog/blog.db'

# 4.2 生成管理员密码哈希与密钥
go run ./cmd/genpass 你的强密码        # → 输出 bcrypt hash
openssl rand -hex 32                   # → JWT_SECRET
openssl rand -hex 32                   # → IP_SALT

# 4.3 写入 .env（对照 deploy/env.example）
sudo -u blog cp /var/blog/src/deploy/env.example /var/blog/.env
sudo -u blog vim /var/blog/.env        # 填入上面三个值 + SITE_URL=http://服务器公网IP
sudo chmod 600 /var/blog/.env
```

## 5. 首次发布

```bash
sudo cp /var/blog/src/deploy/deploy.sh /var/blog/deploy.sh
sudo chown blog:blog /var/blog/deploy.sh && sudo chmod +x /var/blog/deploy.sh
sudo -u blog /var/blog/deploy.sh
# 浏览器访问 http://服务器公网IP 验证；后台在 /admin
```

## 6. 每日备份（cron）

```bash
sudo -u blog crontab -e
# 加入一行（每天 04:00 备份，保留 14 天）：
0 4 * * * sqlite3 /var/blog/blog.db ".backup /var/blog/backups/blog-$(date +\%F).db" && find /var/blog/backups -mtime +14 -delete
```

## 7. 停用 GitHub Pages

GitHub 仓库 → Settings → Pages → Source 选 "None"。
（仓库里的 `.github/workflows/deploy.yml` 已在本次重构中删除。）

## 8. 日常发布流程

```bash
# 本地
git push origin main
# 服务器
ssh 你的服务器 "sudo -u blog /var/blog/deploy.sh"
```

稳定后可加 GitHub Actions 步骤 SSH 自动执行 deploy.sh（升级项）。

## 9. 常用运维

| 操作 | 命令 |
|---|---|
| 服务状态 | `systemctl status blog-api` |
| 查看日志 | `journalctl -u blog-api -n 100 -f` |
| 重启 | `systemctl restart blog-api` |
| 改 Nginx 后生效 | `nginx -t && systemctl reload nginx` |
| 手动备份 | `sqlite3 /var/blog/blog.db ".backup /var/blog/backups/manual.db"` |

## 10. 安全备忘（方案 第九章）

- 裸 IP 无法签发 Let's Encrypt 证书，当前为 HTTP；管理操作尽量在家用网络下进行
- 域名备案通过后：Nginx 加 443 + 证书，SITE_URL 改为 https 域名
- 可选：`/api/admin` 加 IP 白名单或 WireGuard 内网访问
