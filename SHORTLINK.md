# 短链服务（URL Shortener）

> 基于 `gee-starter` 脚手架实现的短链跳转服务，可对外访问、可上线运行。

---

## 一、项目现状

### 1.1 已实现功能

| 模块 | 功能 | 说明 |
|------|------|------|
| **短链核心** | 创建短链 | `POST /api/v1/shortlinks`，Base62 + 自增 ID |
| | 短链跳转 | `GET /r/:code` -> 302 重定向 |
| | 查询短链 | `GET /api/v1/shortlinks/:code` |
| | 禁用短链 | `POST /api/v1/shortlinks/:code/disable` |
| | URL 去重 | 同一 URL 返回同一短码（DB 唯一约束 + upsert） |
| **用户系统** | 注册 | `POST /api/v1/register`（bcrypt 哈希） |
| | 登录 | `POST /api/v1/login`（返回 JWT） |
| | 当前用户 | `GET /api/v1/users/me` |
| | 我的短链 | `GET /api/v1/users/mine` |
| **基础设施** | PostgreSQL | 连接池 + 事务 + 健康检查 |
| | JWT 认证 | HS256 + 可选认证中间件 |
| | Prometheus | `/metrics` 指标暴露 |
| | OpenTelemetry | 分布式追踪 |
| | Web UI | 简单 HTML 页面 |

### 1.2 API 路由

```
# 公开路由
GET  /                              # Web UI
GET  /r/:code                       # 短链跳转（302）

# API 路由
POST /api/v1/shortlinks             # 创建短链（可选认证）
GET  /api/v1/shortlinks/:code       # 查询短链信息
POST /api/v1/shortlinks/:code/disable  # 禁用短链
POST /api/v1/register               # 用户注册
POST /api/v1/login                  # 用户登录

# 需要登录
GET  /api/v1/users/me               # 当前用户信息
GET  /api/v1/users/mine             # 我的短链列表

# 管理端口（内网）
GET  /healthz                       # 存活检查
GET  /readyz                        # 就绪检查（含 DB）
GET  /metrics                       # Prometheus 指标
GET  /version                       # 版本信息
```

### 1.3 数据库表

```sql
-- 用户表
CREATE TABLE users (
    id            BIGSERIAL PRIMARY KEY,
    username      TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    role          TEXT NOT NULL CHECK (role IN ('admin','user')),
    created_at    TIMESTAMPTZ DEFAULT NOW(),
    updated_at    TIMESTAMPTZ DEFAULT NOW()
);

-- 短链表
CREATE TABLE shortlinks (
    id            BIGSERIAL PRIMARY KEY,
    code          TEXT UNIQUE,
    url           TEXT UNIQUE NOT NULL,
    disabled      BOOLEAN NOT NULL DEFAULT FALSE,
    redirect_type TEXT NOT NULL DEFAULT '302' CHECK (redirect_type IN ('301','302')),
    expires_at    TIMESTAMPTZ,
    created_at    TIMESTAMPTZ DEFAULT NOW(),
    updated_at    TIMESTAMPTZ DEFAULT NOW()
);

-- 用户-短链关联表
CREATE TABLE user_shortlinks (
    user_id      BIGINT NOT NULL REFERENCES users(id),
    shortlink_id BIGINT NOT NULL REFERENCES shortlinks(id),
    created_at   TIMESTAMPTZ DEFAULT NOW(),
    PRIMARY KEY (user_id, shortlink_id)
);
```

### 1.4 代码结构

```
internal/app/shortlink/
├── service.go          # 领域模型（Creator/Resolver 接口）
├── base62.go           # Base62 编码
├── validate.go         # URL 校验
├── httpapi/
│   ├── register.go     # 路由注册
│   ├── shortlinks.go   # 短链 handlers
│   ├── users.go        # 用户 handlers
│   ├── web.go          # Web UI
│   └── static/         # 静态文件
└── repo/
    ├── shortlinks.go   # 短链数据访问
    └── users.go        # 用户数据访问
```

---

## 二、TODO 列表

### 2.1 核心功能（优先级高）

- [ ] **Redis 缓存**
  - `code -> url` 缓存，TTL 1 小时
  - 负缓存：不存在的 code 缓存 5 分钟（防穿透）
  - 禁用短链时删除缓存
  - singleflight 防击穿

- [ ] **点击统计**
  - 简单版：跳转时 `UPDATE shortlinks SET click_count = click_count + 1`
  - 进阶版：Kafka 异步投递，消费者批量聚合

- [ ] **过期时间处理**
  - 跳转时检查 `expires_at`
  - 过期短链返回 410 Gone

- [ ] **管理接口完善**
  - `GET /api/v1/admin/shortlinks`：分页列表
  - 支持过滤：`?disabled=true&page=1&size=20`

### 2.2 功能扩展（面试加分）

- [ ] **自定义短码**
  ```json
  POST /api/v1/shortlinks
  { "url": "https://...", "custom_code": "github" }
  ```
  - 校验：长度 3-20，只允许 `a-z A-Z 0-9 -_`
  - 保留关键词：`api`, `admin`, `r`, `healthz` 等

- [ ] **短链预览**
  ```
  GET /r/:code?preview=1
  返回 JSON 而不是跳转
  ```

- [ ] **二维码生成**
  ```
  GET /api/v1/shortlinks/:code/qrcode
  返回 PNG 图片
  ```
  - 使用 `github.com/skip2/go-qrcode`

- [ ] **批量创建**
  ```json
  POST /api/v1/shortlinks/batch
  { "urls": ["https://a.com", "https://b.com"] }
  ```
  - 事务批量插入，限制单次 100 条

- [ ] **点击统计 API**
  ```
  GET /api/v1/shortlinks/:code/stats
  { "total_clicks": 1234, "clicks_today": 89, "clicks_by_day": [...] }
  ```

- [ ] **A/B 测试**（进阶）
  - 一个短码随机跳转到多个目标 URL
  - 按权重分配流量

### 2.3 安全与防护

- [ ] **限流**
  - 创建接口：按 IP 限流（如 10 次/分钟）
  - 跳转接口：按 IP 限流（防枚举）
  - 触发限流时返回 429 + 记录 warn 日志

- [ ] **布隆过滤器**
  - 快速判断 code 是否存在
  - 防止不存在的 code 打穿 DB

- [ ] **URL 安全校验**
  - 黑名单域名（防钓鱼）
  - 敏感词检测（可选）

### 2.4 可观测性

- [ ] **业务指标**
  - `shortlink_create_total{status="success|error"}`
  - `shortlink_redirect_total{status="found|not_found|disabled"}`
  - `shortlink_redirect_latency_seconds`
  - `shortlink_cache_hit_total` / `shortlink_cache_miss_total`

- [ ] **日志增强**
  - 慢请求告警（跳转 > 100ms）
  - 敏感信息脱敏

---

## 三、部署

### 3.1 本地构建镜像

```bash
docker build -t <your-dockerhub>/days-shortlink:0.1.0 .
docker push <your-dockerhub>/days-shortlink:0.1.0
```

### 3.2 环境变量

```bash
ADDR=:9999                    # HTTP 服务端口
ADMIN_ADDR=127.0.0.1:6060     # 管理端口（不对外暴露）
JWT_SECRET=<强随机值>          # 至少 32 字符
DB_DSN=postgres://user:pass@host:5432/db?sslmode=disable
```

### 3.3 上线验收

```bash
# 创建短链
curl -X POST http://localhost:9999/api/v1/shortlinks \
  -H "Content-Type: application/json" \
  -d '{"url": "https://github.com/yourname"}'

# 访问短链
curl -i http://localhost:9999/r/<code>
# 期望：302 Found + Location header

# 健康检查
curl http://localhost:6060/readyz
```

---

## 四、实施顺序建议

| 顺序 | 任务 | 说明 |
|------|------|------|
| 1 | Redis 缓存 | 性能瓶颈，必做 |
| 2 | 点击统计（同步版） | 功能完整性 |
| 3 | 限流 | 安全防护 |
| 4 | 自定义短码 | 用户体验 |
| 5 | 二维码生成 | 简单但实用 |
| 6 | 点击统计 API | 数据能力 |
| 7 | Kafka 异步 | 锦上添花 |

---

## 五、简历亮点

完成后可以这样写：

> **短链服务（Go）**
> - 基于自研 Web 框架实现，支持万级 QPS 的短链跳转服务
> - 短码生成：Base62 + 自增 ID，支持 5600 万+ 短链
> - Redis 缓存 + 布隆过滤器解决缓存穿透，singleflight 防止缓存击穿
> - 限流防刷：按 IP 限流，负缓存防枚举攻击
> - 全链路可观测：Prometheus metrics + OpenTelemetry tracing
> - Docker 容器化部署，支持水平扩展

---

## 六、当前进度

- [x] 短链跳转（MVP）
- [x] URL 去重（upsert）
- [x] 查询 + 禁用短链
- [x] 用户注册 / 登录 / 我的短链
- [x] PostgreSQL 集成
- [x] JWT 认证
- [x] Prometheus + OpenTelemetry
- [x] Web UI
- [x] Docker 部署
- [ ] Redis 缓存
- [ ] 点击统计
- [ ] 限流
- [ ] 自定义短码
