# Waline 评论服务部署指南

> 代码侧已完成：博客构建脚本已接入 Waline 客户端。
> 你只需要完成下面「用户侧」步骤拿到服务端地址，再交回给我填入配置即可。
>
> **路线更新（2026-09-21）**：Vercel 账号被风控拦截（user_blocked）→ 改用 **腾讯云 CloudBase（方案一，真免费，国内访问快）或 Zeabur（方案二，14 天免费试用后 $5/月）**。Vercel 方案保留作为备选（方案三）。

---

## 方案一：腾讯云 CloudBase（真·免费，国内最快，推荐）

> 0 元部署，个人博客免费额度完全够用；默认域名是腾讯已备案的 `xxx.service.tcloudbaseapp.com`，**无需自己买域名、无需 ICP 备案**；唯一前提是腾讯云账号实名认证（身份证号即可）。

### 第一步：开通 CloudBase 环境

1. 打开腾讯云 https://cloud.tencent.com 注册/登录，完成个人实名认证
2. 进入 **云开发 CloudBase** 控制台 → 新建环境：
   - 模板选「空模板」
   - 地域选**广州或上海**（离你近）
   - 计费方式选**按量付费**（主要用免费资源，不花钱）
   - 环境名随意（如 `blog-comments`）→ 开通
   - 新用户可先领「云开发标准型（基础版 1）」0 元资源包

### 第二步：一键部署 Waline

3. 打开 Waline 官方部署入口：https://waline.js.org/guide/deploy/cloudbase.html ，点页面上的**「部署到云开发」**按钮（或直接在 CloudBase 控制台 → 我的应用 → 一键部署 Waline 模板）
4. 选择刚才创建的环境 → 下一步：应用配置 → **完成**
5. 等 3~5 分钟构建，条目右侧出现「访问」按钮

### 第三步：拿地址，交给我

6. 点**「访问」**（或在环境的「访问服务 / HTTP 访问服务」里）看到默认域名，形如：
   `https://blog-comments-xxxxx.service.tcloudbaseapp.com`
7. **把地址发给我**，我填入 `build.py` 并重建部署
8. 访问 `<域名>/ui/register` 注册管理员（第一个账号自动成为管理员）

> 注意：若发评论报 "Failed to fetch"，到 CloudBase 环境 → 安全配置 → 添加你博客的域名（`lird-e.github.io`）为安全域名。

---

## 方案二：Zeabur 部署（14 天免费，之后 $5/月）

### 第一步：Fork 官方脚手架

1. 打开 https://github.com/walinejs/zeabur-starter ，点右上角 **Fork**，得到自己账号下的副本（如 `lird-e/zeabur-starter`）

### 第二步：登录 Zeabur 并建项目

2. 打开 https://dash.zeabur.com ，用 **GitHub 账号登录**（GitHub 本身没问题，只是 Vercel 封了号，这里能正常授权）
3. 首次进入会要求创建 Project，取个名字（如 `blog-comments`），区域选默认或新加坡

### 第三步：先部署 MongoDB 数据库

4. 项目内点 **Add New Service** → **Deploy Other Service** → **MongoDB**
5. 名字随意（如 `waline-db`），点 **Deploy**，等状态变 Running

### 第四步：部署 Waline 服务

6. 再点 **Add New Service** → **Deploy Your Source Code** → 在列表里选你第一步 Fork 的 `zeabur-starter` → **Import** → **Deploy**
7. 等状态变 Running（约 1~2 分钟）

### 第五步：生成访问域名并验证

8. 点进 Waline 服务 → **Domains** 选项卡 → **Generate Domain** → 输入想要的前缀（如 `lird-comments`）→ **Save**，得到地址：
   `https://lird-comments.zeabur.app`
9. 浏览器打开该地址，能看到 Waline 欢迎页即成功
10. **把这个地址发给我**，我填入 `build.py` 的 `WALINE["server_url"]` 并重新构建部署

### 第六步：注册管理员

11. 访问 `<域名>/ui/register` 注册——**第一个注册的账号自动成为管理员**，之后可登录 `<域名>/ui` 审核/删除评论

---

## 方案三：Vercel + Neon（备选）

> 仅当你的 Vercel 账号通过申诉解封后使用。

1. 打开部署入口：https://vercel.com/new/clone?repository-url=https%3A%2F%2Fgithub.com%2Fwalinejs%2Fwaline%2Ftree%2Fmain%2Fexample
   （未登录会跳 GitHub 授权，用你 GitHub 账号 lird-e 快捷登录即可）
2. 输入一个项目名（如 `lird-blog-comments`），点 **Create**
3. 等 1~2 分钟部署完成，进入 Dashboard
   - 如果中途报 `500 This Serverless Function has crashed`：正常现象，是还没接数据库，完成下一步后自动恢复
4. Vercel 控制台左侧 **Storage** → **Create Database** → 选 **Neon** → **Continue**
5. 套餐保持默认 → **Continue** → 定义数据库名（默认即可）→ **Create**
6. 点 **Connect** → **Connect Project**，把数据库绑定到你的项目
7. 数据库详情页点 **Open in Neon** → 左侧 **SQL Editor**，把 https://github.com/walinejs/waline/blob/main/assets/waline.pgsql 里的 SQL 全量粘贴进去 → **Run** 建表
8. 回到 Vercel → **Deployments** → 最新一条 → **Redeploy**（让数据库配置生效）
9. 等状态变 **Ready** → 点 **Visit**，打开的地址就是服务端地址，形如：
   `https://lird-blog-comments.vercel.app`
10. **把地址发给我**；随后访问 `<地址>/ui/register` 注册管理员（第一个注册的账号自动成为管理员）
11. 注意：`*.vercel.app` 域名在国内被 DNS 污染，访客可能加载不出评论区，建议后期在 Vercel → **Domains** 绑定自己的域名（CNAME 到 `cname.vercel-dns.com`）

---

## 启用后的功能

- 楼中楼回复（支持「回复 @某人」）、Markdown、表情包、点赞互动（reaction）
- 明暗主题自动跟随博客右上角的主题切换
- 后台管理：评论审核、删除、用户管理
- 可选增强（之后随时可加）：Turnstile 人机验证、邮件通知、文章阅读计数

## 常见坑

| 问题 | 原因与解决 |
|------|-----------|
| Vercel 登录报 user_blocked | Vercel 账号风控，走方案一 Zeabur，或填申诉表等解封 |
| Zeabur 部署后报 Not initialized | 浏览器 ESM 兼容问题：把客户端 waline.js 换 UMD 版，或检查环境变量 |
| 评论区一直转圈 | 服务端域名被墙/未 Running；Zeabur 的 *.zeabur.app 国内基本可达 |
| 管理后台进不去 | 必须先去 `/ui/register` 注册首个账号 |
| 评论发不出去提示登录 | 默认游客可评；也可在后台把 login 设为 force 强制登录 |
