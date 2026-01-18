# GEE 迭代路线图（`gee-core` + `gee-starter`）

> 目标：把当前教学版 `gee` 打磨成「可写在简历上」的后端项目：既能体现你理解 Web 框架机制（`gee-core`），又能体现工程化与上线意识（`gee-starter`）。

---

## 0. 使用方式（建议）

- 每次只做一个迭代（C1/C2/... 或 S0/S1/...），保证 **可运行、可测试、可回滚**。
- 每个迭代都要补：**最小用例（curl/httptest）+ 单测 + README 更新**。
- 每个迭代完成后跑：
  - [ ] `go test ./...`
  - [ ] `go test -race ./...`（可选但强烈建议）

**测试覆盖率目标**：
- [ ] 核心模块（router/context/middleware）测试覆盖率 ≥ 70%
- [ ] 运行 `go test -cover ./gee/...` 查看覆盖率

---

## 1. 推荐仓库结构（可选，但更像"工程项目"）

你可以先不改结构，等 C3/C4 之后再整理；但如果你希望更像真实服务：

- `gee/`：框架核心（保留你现在的代码）
- `cmd/`：可运行入口（替代根目录 `main.go`）
- `internal/`：starter 的业务与基础设施（auth/metrics/trace/db 等）
- `examples/`：最小示例（hello/group/params/middleware/panic）

TODO：
- [x] 使用 `cmd/gee-demo/main.go` 作为 demo 入口（避免根目录 `main.go`）
- [ ] `README.md` 写清楚：`gee-core` 是库，`gee-starter` 是可上线模板

---

## 2. Track A：`gee-core`（像 Gin 的框架内核）

### C0：项目说明与示例（半天）

目标：让别人 3 分钟读懂你做了什么、怎么跑、有哪些特性。

TODO：
- [ ] `README.md`：特性列表、设计取舍、目录结构、快速开始
- [ ] `examples/`：`hello`、`group`、`params(:/*)`、`middleware`、`panic`
- [ ] 把现在的 `main.go` 作为 demo（或迁移到 `cmd/demo`）

验收：
- [ ] 新人能 `go run ...` 跑起来
- [ ] README 里每个特性都有一段最小示例

---

### C1：Middleware 控制能力（`Abort` / `Fail` 必做，1 天）

背景：当前 `Context.Fail()` 只是写响应，不会阻止后续 handler；真实框架必须能中断链条。

建议改动点：`gee/context.go`

TODO：
- [x] `Context` 增加：
  - [x] `Abort()`
  - [x] `IsAborted() bool`
  - [x] `AbortWithStatus(code int)`
  - [x] `AbortWithStatusJSON(code int, obj any)`（可选）
- [x] `Fail()`：写响应后 **直接 Abort**（避免继续执行后续 handler）
- [x] 明确 `Next()` 语义：被 Abort 后不再进入后续 handlers（写成单测锁死行为）

单测（建议 `gee/context_test.go`）：
- [ ] `Fail()` 后下游 handler 不执行
- [ ] `Abort()` 后 `Next()` 不会继续

验收：
- [ ] 任何 handler 调用 `Fail/Abort` 后都不会再有后续 handler 写响应

---

### C2：正确的响应观测（包装 `ResponseWriter`，1 天）

背景：`Logger` 依赖 `ctx.StatusCode`，但静态文件/直接写 `ResponseWriter` 时可能拿不到真实 status/size。

建议改动点：`gee/context.go`, `gee/logger.go`, 以及一个新文件 `gee/response_writer.go`

TODO：
- [x] 实现 `ResponseWriter` 包装器，至少记录：
  - [x] `Status() int`（默认 200）
  - [x] `Size() int`（累计写入字节）
- [x] `Context` 使用包装器（或新增 `ctx.Writer` 为包装类型）
- [x] `Logger()` 改为记录包装器里的 status/size + latency
  - [x] status + latency
  - [x] size(bytes)
- [x] 确定默认 status：没显式 `WriteHeader` 时也应是 200

单测（建议 `gee/logger_test.go` + httptest）：
- [ ] handler 只 `Write([]byte)`，Logger 记录 200
- [ ] handler `Status(201)` 后写 body，Logger 记录 201

验收：
- [x] access log 的 status/bytes 永远正确

---

### C3：Recovery 行为修复（必做，半天~1 天）

背景：当前 `Recovery` recover 后只 `Fail(500, ...)`，但可能继续执行后续 handler；而且 middleware 顺序会导致无法兜住上游 panic。

建议改动点：`gee/recover.go`, `gee/gee.go`

TODO：
- [x] `Recovery()`：发生 panic 后
  - [x] 写 500（统一错误体）
  - [x] **Abort**（阻止后续 handler）
  - [x] 只在响应未写入时写 header（可选进阶）
- [x] `Default()` 中的注册顺序调整为：`Recovery()` 在最外层（通常 `engine.Use(Recovery(), Logger())`）
- [ ] 文档里写清楚：middleware 顺序与兜底范围

单测：
- [ ] handler panic -> 返回 500，且 panic 后的 handler 不执行
- [ ] panic 发生在 Logger 中（可人为制造）时，Recovery 能兜住（通过顺序验证）

验收：
- [ ] "任何 panic 都不会把进程打崩"，且响应可控

---

### C4：404/405 与自定义兜底（半天）

建议改动点：`gee/router.go`, `gee/gee.go`

TODO：
- [x] 支持：
  - [x] `NoRoute(handlers ...HandlerFunc)`
  - [x] `NoMethod(handlers ...HandlerFunc)`（返回 405 + Allow）
- [x] 404/405 支持 middleware（行为与 Gin 类似：也走一遍链）
- [ ] Router 支持列出已注册路由（debug 输出）

单测：
- [ ] 未注册 path -> 404 + 自定义 body
- [ ] method 不支持 -> 405 + Allow header

---

### C5：绑定与参数校验（1~2 天）

建议改动点：`gee/context.go`（或新增 `gee/binding.go`）

TODO：
- [x] `BindJSON(&dst)`（读取 body + json decode）
- [ ] `BindQuery(&dst)`（从 query 映射到 struct，先做最小版也行）
- [ ] `BindAndValidate(&dst)`：绑定 + 自动校验（集成 `go-playground/validator` 或自己写最小版）
  - [ ] 支持 struct tag：`validate:"required,email,gte=0,lte=120"`
  - [ ] 校验失败返回字段级错误详情

单测：
- [ ] 非法 JSON -> 400 + 错误体
- [ ] 缺字段/类型不匹配 -> 400 + 错误体
- [ ] 校验失败 -> 400 + 字段级错误

---

### C6：统一错误模型（1 天）

背景：所有错误响应（400/401/403/404/405/500/panic）都应该走统一结构，这是生产级框架的标志。

建议改动点：`gee/errors.go`（新建）

TODO：
- [x] 统一错误返回结构：
  ```json
  {
    "code": "VALIDATION_ERROR",
    "message": "参数校验失败",
    "request_id": "xxx",
    "details": [{"field": "email", "reason": "格式不合法"}]
  }
  ```
- [ ] `code` 使用业务错误码（字符串），HTTP status 单独设置
- [ ] 常见错误码定义：
  - [ ] `VALIDATION_ERROR` (400)
  - [ ] `UNAUTHORIZED` (401)
  - [ ] `FORBIDDEN` (403)
  - [ ] `NOT_FOUND` (404)
  - [ ] `METHOD_NOT_ALLOWED` (405)
  - [ ] `CONFLICT` (409)
  - [ ] `RATE_LIMITED` (429)
  - [ ] `INTERNAL_ERROR` (500)
- [ ] 所有错误出口统一：
  - [ ] BindJSON 失败 -> 统一结构
  - [ ] 404/405 -> 统一结构
  - [ ] panic/recover -> 统一结构
  - [ ] 业务错误 -> 统一结构

验收：
- [ ] 任何错误响应都是同一个 JSON 结构

---

### C7：内置中间件"小套装"（1~2 天）

建议新增：`gee/middleware/`（可先放在 `gee/` 下，后续再拆）

TODO（按优先级）：
- [ ] `RequestID`：生成/透传 `X-Request-ID`，写进 `Context`（**必做**）
- [ ] `CORS`：最小实现（允许方法/头/凭证可配）（**必做**）
- [ ] `BodyLimit`：请求体大小限制，防止恶意大请求（**建议**）
- [ ] `SecureHeaders`：自动添加安全响应头（X-Content-Type-Options, X-Frame-Options 等）
- [ ] `Timeout`：为 handler 设置超时（需定义清楚：超时后是否还能写响应）
- [ ] `Gzip`（可选）：对特定 content-type 压缩
- [ ] `RateLimit`（可选）：基于 token bucket（先内存版）

验收：
- [ ] 每个中间件都有最小示例 + 单测

---

### C8：路由增强（1 天）

TODO：
- [ ] 路由元信息（Route Metadata）：
  ```go
  r.GET("/admin/users", AdminOnly(), ListUsers).
      Name("admin.users.list").
      Meta("permission", "user:read")
  ```
- [ ] 路由列表导出（用于 debug / API 文档生成）：
  ```go
  routes := engine.Routes() // 返回所有已注册路由
  ```
- [ ] 静态文件服务增强（如果还没有）：
  - [ ] `r.Static("/assets", "./public")`
  - [ ] `r.StaticFile("/favicon.ico", "./public/favicon.ico")`
  - [ ] 目录列表禁用（安全）

验收：
- [ ] 能导出路由列表为 JSON 格式

---

### C9：并发安全与性能（1~2 天）

TODO（先正确性，再性能）：
- [ ] 路由注册并发安全：
  - [ ] 方案 A：启动后冻结路由（`engine.Run` 后不允许再 `GET/POST/Use`）
  - [ ] 方案 B：加锁保护（读写锁）
- [ ] `sync.Pool` 复用 `Context`（必须写"reset 清理"单测）
- [ ] 减少分配：`parsePattern`、params map 复用（可选）

验收：
- [ ] `go test -race ./...` 通过
- [ ] benchmark 有可量化收益（写在 README）

---

### C10：基准测试与对比数据（半天~1 天）

TODO：
- [ ] `go test -bench`：路由匹配、middleware 链开销、JSON 输出
- [ ] README：记录机器环境 + 关键数字（吞吐/延迟/allocs/op）

---

## 3. Track B：`gee-starter`（可直接用的服务脚手架）

> 这一部分更适合"实习/校招"：体现你能做出 **可上线形态**（可观测、可配置、可部署、可治理）。

### S0：工程骨架（半天~1 天）

TODO：
- [x] `cmd/api/main.go`：启动入口
- [x] `internal/` 分层约定（`internal/platform/*` 可复用、`internal/app/*` 仅业务）
- [x] 配置加载：env（`.env`）+ flag（可选）
- [ ] Makefile/Taskfile：`run/test/lint/build`

验收：
- [ ] 一条命令启动：`make run`（或等价）

---

### S1：生产级 HTTP Server（1 天）

TODO：
- [x] 用 `http.Server`（不要直接 `ListenAndServe`）
- [x] 配置超时：
  - [x] `ReadHeaderTimeout`
  - [x] `ReadTimeout`
  - [x] `WriteTimeout`
  - [x] `IdleTimeout`
- [x] graceful shutdown：SIGINT/SIGTERM，优雅停机 + in-flight 等待

验收：
- [ ] `Ctrl+C` 退出时不丢请求（最小示例：sleep handler）

---

### S2：可观测性（日志）（1 天）

TODO：
- [x] 结构化日志（建议 `log/slog`）+ 可配置 level
- [x] access log：method/path/status/latency/bytes/request_id/user_id（有则写）
- [x] panic/recover 日志统一格式（与 `gee-core` 的 Recovery 对齐）
- [ ] 敏感信息脱敏：日志中自动脱敏密码、token 等字段
- [ ] 慢请求告警：超过阈值自动记录 warn 日志

验收：
- [x] 每个请求都有 request_id，可从日志串起来

---

### S3：健康检查 + pprof + 版本信息（半天）

TODO：
- [x] `/healthz`（进程存活）
- [x] `/readyz`（依赖就绪：例如 DB/Redis 可选）
- [x] `/debug/pprof/*`（默认关闭或需鉴权）
- [x] `/version`：输出 build info（commit、build time、go version）

---

### S4：Auth（JWT + 最小 RBAC）（1~2 天）

TODO：
- [x] JWT middleware：解析/校验/过期处理
- [x] RBAC：role -> permissions（最小内存版，不需要完整实现）
- [ ] 统一 401/403 错误体（复用 C6 的统一错误模型）
- [x] 示例业务接口：`/api/v1/users/me`、`/api/v1/admin/*`

验收：
- [x] 无 token -> 401；权限不足 -> 403；合法 -> 200

---

### S5：Metrics + Trace（1~2 天）

分两步做更稳：

TODO（最小可用）：
- [ ] `expvar`：请求计数/错误计数/延迟分布（简化版）

TODO（升级版，可选但很加分）：
- [x] Prometheus metrics（`/metrics`）
- [x] OpenTelemetry tracing（HTTP server span + 关键业务 span）
- [ ] 日志关联 trace_id/span_id

验收：
- [x] 访问一次接口，能在 metrics 里看到计数增长；trace 能串起来

---

### S6：数据库集成（1 天）

背景：后端项目几乎必备数据库，没有 DB 不完整。

TODO：
- [x] 数据库连接池（PostgreSQL：`pgxpool`）
- [x] 迁移：`migrations/001_init_users.sql`（`users` 表）
- [x] dev seed：`migrations/002_seed_users_dev.sql`（可重复执行）
- [x] Users repo：`internal/platform/auth/usersrepo/users.go`（`FindByUsername`）
- [x] 登录从 DB 查用户 + bcrypt 校验（不泄露用户是否存在）
- [x] 连接健康检查：`/readyz` 集成 DB `Ping`
- [x] 优雅退出：defer 关闭连接池

验收：
- [x] `POST /api/v1/login`：正确账号返回 token；错误密码返回 401（且不 panic）
- [x] 停掉 DB 后 `/readyz` 返回非 200；恢复 DB 后 `/readyz` 恢复 200

---

### S7：缓存集成（半天~1 天）

背景：缓存是高并发系统的标配，面试高频话题。

TODO：
- [ ] Redis 连接管理
- [ ] 简单封装：Get/Set/Del + TTL
- [ ] 连接健康检查（集成到 `/readyz`）
- [ ] 示例：接口结果缓存

验收：
- [ ] 能通过 Redis 缓存 API 响应

---

### S8：交付与上线体验（1 天）

TODO：
- [ ] Dockerfile（多阶段构建）
- [ ] docker-compose（可选：带 Redis/DB）
- [ ] CI：test + race + build（可再加 lint）
- [ ] 一页 `DEPLOY.md`：本地、容器、排障（日志/pprof/metrics/trace）

---

### S9：API 文档（可选，半天）

TODO：
- [ ] 支持导出已注册路由列表（JSON 格式，复用 C8）
- [ ] 或集成 Swagger/OpenAPI（进阶）

验收：
- [ ] 能生成可读的 API 列表

---

## 4. 简历写法提示（你做完后再写）

建议把项目拆成两段描述：

- `gee-core`：强调框架机制（middleware/Recovery/路由树/ResponseWriter/参数校验/统一错误模型/benchmark/race）
- `gee-starter`：强调工程化（超时与优雅关闭、鉴权、日志、metrics、trace、DB/Redis、CI、Docker）

---

## 5. 你现在就可以选的"第一步"

如果只选一个最该先做的：

- [ ] 先做 **C1 + C3**（Abort + Recovery 修复）——这是框架正确性的地基
- [ ] 或先做 **S1**（超时 + graceful shutdown）——这是服务上线的地基

---

## 6. 推荐的优先级排序

如果时间有限，按这个顺序做：

| 优先级 | 模块 | 理由 |
|--------|------|------|
| ⭐⭐⭐ | C5 参数校验 | 几乎每个接口都要用 |
| ⭐⭐⭐ | C6 统一错误模型 | 生产级框架标志 |
| ⭐⭐⭐ | S6 数据库集成 | 后端项目没 DB 不完整 |
| ⭐⭐ | C7 RequestID + CORS | 必备中间件 |
| ⭐⭐ | S7 Redis 缓存 | 高频面试话题 |
| ⭐⭐ | S3 健康检查 | 工作量小但很实用 |
| ⭐ | C8 路由元信息 | 框架扩展性 |
| ⭐ | S2 慢请求告警 | 生产环境实用 |

---

## 7. 当前进度（手动更新）

- [x] C1：`Abort/IsAborted/Fail/AbortWithStatus`
- [x] C3：`Default()` 顺序调整为 `engine.Use(Recovery(), Logger())`；panic 返回 500 且中断后续 handlers
- [x] C2：ResponseWriter 包装（status/size）+ Logger 读取 status/size
- [x] C4：`NoRoute/NoMethod` + 404/405 走 middleware + 405 返回 Allow
- [x] S0：`cmd/api` + `internal` 分层 + env 配置加载
- [x] S1：`http.Server` 超时配置 + graceful shutdown
- [x] S2：`slog` 结构化日志 + `X-Request-ID` + access log + panic 日志
- [x] S3：`/healthz` + `/readyz` + `/version` + `pprof`（默认关闭）
- [x] S4：JWT auth + 最小 RBAC（login + me + admin）
- [x] S5：Prometheus `/metrics` + OpenTelemetry tracing（otelhttp + OTLP exporter）
- [x] S6：PostgreSQL 集成（users + login DB 查询 + `/readyz` 检查 DB）

**待补齐的单测**：
- [ ] C1 单测：`Fail()` 后下游 handler 不执行、`Abort()` 后 `Next()` 不会继续
- [ ] C2 单测：Logger 记录正确的 status/size
- [ ] C3 单测：panic 返回 500 且后续 handler 不执行
- [ ] C4 单测：404/405 + Allow header
