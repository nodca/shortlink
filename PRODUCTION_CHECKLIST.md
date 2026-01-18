# 真实上线（对公网提供服务）检查清单

> 适用对象：你现在的 Go 服务脚手架（public/admin 双端口、JWT、metrics、trace、PostgreSQL、Docker）。  
> 目标：把“能跑”提升到“能稳定对外跑”，并且成本可控、风险可控。

---

## 1) 域名 / HTTPS / 入口流量

- [ ] 域名：购买并配置解析（A/AAAA 记录指向服务器 IP）
- [ ] HTTPS：使用反向代理（Nginx 或 Caddy）签发并自动续期证书（Let’s Encrypt）
- [ ] 入口限流：反向代理层做最小限流（按 IP、按路径）防扫/防刷（尤其是短链 redirect）
- [ ] 只暴露必要端口：
  - [ ] `80/443`（公网）
  - [ ] public 服务端口可只对内（例如 127.0.0.1:9999），由反代转发
  - [ ] admin 端口永远不直接对公网

---

## 2) 网络与安全（最低要求）

- [ ] 安全组/防火墙：只放行 `80/443`；SSH 只允许固定 IP 或改端口/密钥登录
- [ ] admin server：仅监听 `127.0.0.1`（运维接口、metrics、pprof、readyz/version 等）
- [ ] secrets 管理：
  - [ ] JWT secret、DB 密码、第三方 key 不写入 git（`.env` 也不提交）
  - [ ] 线上用环境变量或平台密钥（必要时分环境：dev/staging/prod）
- [ ] 日志脱敏：不要打印 Authorization、cookie、密码、token、DSN 全量等敏感信息
- [ ] 更新策略：定期打补丁（系统安全更新、依赖更新）

---

## 3) 数据库（PostgreSQL）上线要求

- [ ] 备份（必须）：
  - [ ] 最低标准：每天 `pg_dump`，至少保留 7 天
  - [ ] 备份放到“本机以外”的地方（对象存储/另一台机器/网盘都行）
- [ ] 资源与容量：
  - [ ] 监控磁盘（爆盘是最常见事故之一）
  - [ ] 连接池：设置最大连接数、空闲连接、连接超时
- [ ] schema 管理：
  - [ ] migrations 可重复执行/有版本号（避免“线上手改表”）
  - [ ] 关键唯一约束/索引（如短链 `code` unique、user `username` unique）

---

## 4) 可靠性：超时、优雅退出、自愈

- [ ] 超时（必须）：
  - [ ] HTTP Server：read/write/idle timeout（你已做）
  - [ ] DB 操作：每次 query 都带 context timeout（避免卡死）
- [ ] 优雅退出（必须）：
  - [ ] 收到 SIGTERM 后停止接新请求，等待已有请求完成
  - [ ] shutdown 超时后强制退出（防止无限挂住）
- [ ] 自愈（必须）：
  - [ ] 使用 systemd / supervisor / Docker restart policy，确保进程挂了能自动拉起

---

## 5) 可观测性：日志 / metrics / trace / pprof 的定位边界

- [ ] 日志（必须）：
  - [ ] 结构化日志（slog）
  - [ ] request_id（链路内关联）
  - [ ] 关键错误日志带上下文（method/path/status/latency）
- [ ] Metrics（强烈建议）：
  - [ ] `/metrics` 仅走 admin 端口（本机或内网访问）
  - [ ] 关注：错误率、P95/P99、QPS、inflight、依赖失败数
- [ ] Trace（可选但加分）：
  - [ ] OTel exporter 仅内网/本机到 collector（避免公网暴露）
  - [ ] span 名使用 route pattern（你已做）
- [ ] pprof（强烈建议保留）：
  - [ ] 不对公网，只允许本机或 SSH 隧道访问
  - [ ] 作用：定位 CPU/内存/阻塞等性能问题（不是替代 metrics/trace）

---

## 6) 业务安全：鉴权、权限、滥用防护

- [ ] 鉴权：
  - [ ] login 不受鉴权中间件影响
  - [ ] protected route 必须校验 `Authorization: Bearer <token>`
- [ ] 权限（RBAC）：
  - [ ] admin/management API 必须 require role=admin
- [ ] 滥用防护（短链必备）：
  - [ ] 创建接口限流（IP/user）
  - [ ] redirect 限流（IP 或 code）
  - [ ] 404 负缓存（防枚举）
  - [ ] URL 校验（只允许 http/https；必要时黑名单）

---

## 7) 部署建议（低成本单机版）

- [ ] 服务器：1 台 Linux 云主机即可（public + postgres 同机；后续再拆）
- [ ] 反向代理：Nginx/Caddy（Caddy 更省事，自动 HTTPS）
- [ ] 进程托管：Docker Compose + `restart: unless-stopped`
- [ ] 日志：先落本地 + logrotate；后面再接集中式日志（可选）
- [ ] CI/CD：可选（GitHub Actions 构建镜像，服务器拉取并重启）

---

## 8) “上线验收”最小清单（你上线当天必做）

- [ ] 公网能访问：`GET /healthz` 返回 200
- [ ] 关掉 DB：`/readyz` 变为非 200（证明 readiness 真有检查依赖）
- [ ] JWT 流程可用：login -> token -> protected route
- [ ] metrics 可抓取（但不暴露公网）：本机 `curl 127.0.0.1:<admin>/metrics` OK
- [ ] 备份已跑通：能从备份恢复到新库（至少演练一次）
- [ ] 防火墙已收紧：公网只开 `80/443`

---

## 9) 租云服务器（国内云）流程（以“轻量应用服务器”为例）

> 目标：用最低成本拿到一台公网 Linux 主机，并能 SSH 登录、跑 Docker、对外提供 80/443。

### 9.1 下单前准备

- [ ] 账号与实名：注册云账号并完成实名（国内云通常都需要）
- [ ] 预算与规格（建议起步）：
  - [ ] 1–2 vCPU / 1–2GB 内存 / 40–60GB 磁盘
  - [ ] 系统：Ubuntu 22.04 / Debian 12（二选一）
- [ ] 地域选择：离你最近即可（你自己访问为主，优先低延迟）
- [ ] 登录方式：
  - [ ] 推荐：SSH Key（生成并保存私钥）
  - [ ] 备选：设置强密码 + 关闭密码登录（上线后再做）

### 9.2 下单与开机

- [ ] 购买轻量主机（按月付/年付，先按月付）
- [ ] 记录公网信息：
  - [ ] 公网 IP
  - [ ] 默认 SSH 端口（通常 22）
- [ ] 安全组/防火墙（第一版）：
  - [ ] 放行：`22`（SSH，仅你的 IP 更好）
  - [ ] 放行：`80/443`（后面做反向代理/HTTPS）
  - [ ] 不放行：你的应用端口（如 `9999`）与 admin 端口（如 `6060`）

### 9.3 首次登录与基础加固（最小版）

- [ ] SSH 登录（Windows 可用 PowerShell/Terminal）：
  - [ ] `ssh root@<公网IP>` 或 `ssh ubuntu@<公网IP>`（看发行版默认用户）
- [ ] 更新系统包（避免“裸奔旧系统”）
- [ ] 创建普通用户并赋予 sudo（可选但推荐）
- [ ] 配置时区与时间同步（日志时间很重要）

### 9.4 安装 Docker 并验证

- [ ] 安装 Docker / Docker Compose（插件版 `docker compose`）
- [ ] 验证：
  - [ ] `docker version`
  - [ ] `docker compose version`

### 9.5 部署你的服务（单机低成本版）

- [ ] 把项目部署方式定为二选一：
  - [ ] A：服务器上 `git clone` + `docker compose up -d`
  - [ ] B：CI 构建镜像推仓库，服务器只负责 `docker pull` + `compose up -d`
- [ ] 线上配置：
  - [ ] 使用 `.env`（不要提交到 git）
  - [ ] `JWT_SECRET`、`DB_DSN` 等用强随机值
- [ ] 数据持久化：
  - [ ] Postgres 使用 volume
  - [ ] 定时备份 `pg_dump`（见第 3 节）

### 9.6 反向代理与 HTTPS（对外只暴露 80/443）

- [ ] 选择 Nginx 或 Caddy（Caddy 更省事）
- [ ] 反代到本机 `127.0.0.1:<public_port>`（例如 `127.0.0.1:9999`）
- [ ] 开启 HTTPS：
  - [ ] 有域名：用 Let’s Encrypt 自动签发/续期
  - [ ] 没域名：可先用 HTTP（演示用），或用自签名（不推荐）

### 9.7 备案提醒（只在你决定“长期对外 + 用域名”时再做）

- [ ] 国内云 + 中国内地服务器，如果要用域名长期对外提供 Web 服务，通常需要备案流程（周期可能以周计）
