# Redis 集成：限流 + 跳转缓存

## 概述

集成 Redis 实现两个功能：
1. **限流** - 防止接口被恶意刷
2. **跳转缓存** - 减少 DB 查询压力

## 项目现状（2026-01-17）

- `docker-compose.yml` 已包含 `redis:7-alpine`
- 已有 Redis client：`internal/platform/cache/redis.go`
- 已有 ratelimit 包骨架：`internal/platform/ratelimit/limiter.go`（待实现 `Allow`）
- **gee 框架已支持“每路由多个 handlers”**：`GET/POST/DELETE` 支持 variadic，可按本 TODO 用“中间件式限流”
- 已实现短链跳转缓存（含负缓存）：`internal/app/shortlink/cache/shortlink.go` + `internal/app/shortlink/repo/shortlinks.go`
- 已补充缓存集成测试：`test/redis_cache_integration_test.go`
- 已补充 Redis 缓存压测输出：`test/k6/result_redirect_redis.txt`、`test/k6/summary_redirect_redis.json`、`test/k6/result_load_redis.txt`、`test/k6/summary_load_redis.json`
- 压测可关闭 tracing：`TRACING_ENABLED=false`（避免将 Jaeger/OTEL 开销混入业务 QPS）

## 第一阶段：Redis 基础设施

### 1.1 Docker 配置

```yaml
# docker-compose.yml
redis:
  image: redis:7-alpine
  ports:
    - "6379:6379"
  volumes:
    - redis_data:/data
  # 短链项目里 Redis 主要用于缓存/限流：允许丢数据（重启后可回源 DB / 重新计数），优先吞吐与稳定尾延迟。
  # AOF 会带来额外写放大与 IO 抖动；默认关闭。
  command: redis-server --appendonly no
```

### 1.2 Go 客户端

```bash
go get github.com/redis/go-redis/v9
```

### 1.3 配置项

```go
// config.go
RedisAddr     string `env:"REDIS_ADDR" envDefault:"localhost:6379"`
RedisPassword string `env:"REDIS_PASSWORD" envDefault:""`
RedisDB       int    `env:"REDIS_DB" envDefault:"0"`
```

### 1.4 连接封装

```go
// internal/platform/cache/redis.go
package cache

import (
    "context"
    "github.com/redis/go-redis/v9"
)

func NewRedisClient(addr, password string, db int) (*redis.Client, error) {
    client := redis.NewClient(&redis.Options{
        Addr:     addr,
        Password: password,
        DB:       db,
    })

    if err := client.Ping(context.Background()).Err(); err != nil {
        return nil, err
    }
    return client, nil
}
```

---

## 第二阶段：限流

### 2.1 限流策略

| 接口 | 限制 | Key 格式 |
|------|------|----------|
| 创建短链 | 10次/分钟/IP | `rl:create:{ip}` |
| 短链跳转 | 100次/分钟/IP | `rl:redirect:{ip}` |
| 登录 | 5次/分钟/IP | `rl:login:{ip}` |
| 注册 | 3次/分钟/IP | `rl:register:{ip}` |

### 2.2 滑动窗口限流器

> “一步到位”的滑动窗口建议：**ZSET + Lua(EVAL) 原子脚本**。
>
> 为什么不直接用 pipeline：并发下会出现“偶尔多放/少算”；而且 `ZADD Member=now_ms` 会在同毫秒并发时覆盖，导致**低估请求数**。
>
> 实现要点：
> - `score = now_ms`
> - `member` 必须唯一（避免覆盖）：建议 `now_ms + "-" + X-Request-ID`
> - 超限时 `ZREM member` 回滚，让被拒绝的请求不占配额
> - `PEXPIRE key window_ms`，避免 key 永久增长

```go
// internal/platform/ratelimit/limiter.go
package ratelimit

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

type Limiter struct {
	client *redis.Client
}

func NewLimiter(client *redis.Client) *Limiter {
	return &Limiter{client: client}
}

// Allow 返回：allowed、retryAfter（仅当超限时有意义）
func (l *Limiter) Allow(ctx context.Context, key string, limit int, window time.Duration, member string) (bool, time.Duration, error) {
	nowMs := time.Now().UnixMilli()
	windowMs := window.Milliseconds()

	const lua = `
local key = KEYS[1]
local now = tonumber(ARGV[1])
local window = tonumber(ARGV[2])
local limit = tonumber(ARGV[3])
local member = ARGV[4]

local windowStart = now - window
redis.call("ZREMRANGEBYSCORE", key, 0, windowStart)
redis.call("ZADD", key, now, member)
local count = redis.call("ZCARD", key)
redis.call("PEXPIRE", key, window)

if count <= limit then
  return {1, 0}
end

redis.call("ZREM", key, member)

local oldest = redis.call("ZRANGE", key, 0, 0, "WITHSCORES")
if oldest[2] ~= nil then
  local oldestScore = tonumber(oldest[2])
  local retryAfter = (oldestScore + window) - now
  if retryAfter < 0 then retryAfter = 0 end
  return {0, retryAfter}
end
return {0, window}
`

	res, err := l.client.Eval(ctx, lua, []string{key}, nowMs, windowMs, limit, member).Result()
	if err != nil {
		return false, 0, err
	}

	arr, ok := res.([]any)
	if !ok || len(arr) < 2 {
		return false, 0, fmt.Errorf("unexpected redis eval result: %T %v", res, res)
	}

	allowed, _ := arr[0].(int64)
	var retryAfterMs int64
	switch v := arr[1].(type) {
	case int64:
		retryAfterMs = v
	case string:
		retryAfterMs, _ = strconv.ParseInt(v, 10, 64)
	}

	return allowed == 1, time.Duration(retryAfterMs) * time.Millisecond, nil
}
```

### 2.3 简单计数器限流（备选方案）

```go
// 更简单，但精度稍低
func (l *Limiter) AllowSimple(ctx context.Context, key string, limit int, window time.Duration) (bool, error) {
    count, err := l.client.Incr(ctx, key).Result()
    if err != nil {
        return false, err
    }

    if count == 1 {
        l.client.Expire(ctx, key, window)
    }

    return count <= int64(limit), nil
}
```

### 2.4 限流中间件

```go
// internal/platform/httpmiddleware/ratelimit.go
package httpmiddleware

func RateLimit(limiter *ratelimit.Limiter, prefix string, limit int, window time.Duration) gee.HandlerFunc {
    return func(c *gee.Context) {
        ip := c.ClientIP()
        key := fmt.Sprintf("rl:%s:%s", prefix, ip)

        allowed, err := limiter.Allow(c.Request.Context(), key, limit, window)
        if err != nil {
            slog.Error("rate limit check failed", "err", err)
            c.Next() // Redis 故障时放行
            return
        }

        if !allowed {
            c.AbortWithError(http.StatusTooManyRequests, "rate limit exceeded")
            return
        }

        c.Next()
    }
}
```

### 2.5 路由集成

```go
// 创建短链 - 10次/分钟
api.POST("/shortlinks",
    httpmiddleware.RateLimit(limiter, "create", 10, time.Minute),
    shortlinkhttpapi.NewCreateHandler(slRepo))

// 登录 - 5次/分钟
api.POST("/login",
    httpmiddleware.RateLimit(limiter, "login", 5, time.Minute),
    shortlinkhttpapi.NewLoginHandler(usersRepo, ts))

// 跳转 - 100次/分钟
r.GET("/:code",
    httpmiddleware.RateLimit(limiter, "redirect", 100, time.Minute),
    shortlinkhttpapi.NewRedirectHandler(slRepo, collector))
```

---

## 第三阶段：跳转缓存

### 3.1 缓存策略

- **Key**: `sl:{code}`
- **Value**: 原始 URL
- **TTL**: 1 小时（热点数据自动续期）
- **失效**: 短链禁用时主动删除

### 3.2 缓存层封装

```go
// internal/app/shortlink/cache/shortlink.go
package cache

import (
    "context"
    "time"
    "github.com/redis/go-redis/v9"
)

type ShortlinkCache struct {
    client *redis.Client
    ttl    time.Duration
    emptyTTL time.Duration
}

func NewShortlinkCache(client *redis.Client) *ShortlinkCache {
    return &ShortlinkCache{
        client: client,
        ttl:    time.Hour,
        emptyTTL: 30 * time.Second,
    }
}

func (c *ShortlinkCache) Get(ctx context.Context, code string) (string, error) {
    key := "sl:" + code
    // 高频跳转场景推荐：只做 GET（纯读），不要在命中时续期 TTL。
    // 续期意味着额外的写操作（EXPIRE/PEXPIRE），在高 QPS 下会拖慢吞吐并放大尾延迟。
    //
    // 如果担心热点过期回源：把 ttl 设长一点（并加随机抖动防雪崩），并在禁用/更新时 DEL 或覆盖写缓存。
    url, err := c.client.Get(ctx, key).Result()
    if err == redis.Nil {
        return "", nil // 缓存未命中
    }
    return url, err
}

func (c *ShortlinkCache) Set(ctx context.Context, code, url string) error {
    return c.client.Set(ctx, "sl:"+code, url, c.ttl).Err()
}

func (c *ShortlinkCache) Delete(ctx context.Context, code string) error {
    return c.client.Del(ctx, "sl:"+code).Err()
}

// SetNotFound 用明确哨兵值做“负缓存”，避免缓存穿透。
// 注意：不要用 "" 作为哨兵值（可读性差、也容易把“未命中”和“命中空值”混淆）。
func (c *ShortlinkCache) SetNotFound(ctx context.Context, code string) error {
    return c.client.Set(ctx, "sl:"+code, "__nil__", c.emptyTTL).Err()
}
```

### 3.3 修改 Resolve 方法

```go
// repo/shortlinks.go
type ShortlinksRepo struct {
    db    *pgxpool.Pool
    cache *cache.ShortlinkCache // 新增
}

func (r *ShortlinksRepo) Resolve(ctx context.Context, code string) string {
    // 1. 查缓存
    if r.cache != nil {
        if url, err := r.cache.Get(ctx, code); err == nil {
            if url == "__nil__" {
                return "" // 命中负缓存
            }
            if url != "" {
                return url
            }
        }
    }

    // 2. 查数据库
    var url string
    err := r.db.QueryRow(ctx,
        `SELECT url FROM shortlinks WHERE code = $1 AND NOT disabled`,
        code).Scan(&url)
    if err != nil {
        // 只在“确实不存在”时写负缓存；其它 DB 错误不要写（避免把故障当成 404 缓存住）。
        // 具体判断请在真实代码里用 errors.Is(err, pgx.ErrNoRows)。
        if r.cache != nil {
            r.cache.SetNotFound(ctx, code)
        }
        return ""
    }

    // 3. 写缓存
    if r.cache != nil && url != "" {
        r.cache.Set(ctx, code, url)
    }

    return url
}
```

### 3.4 禁用时删除缓存

```go
// 你的仓库里实现是 DisableByCode(ctx, code)，缓存删除应挂在 DisableByCode 成功后。
func (r *ShortlinksRepo) DisableByCode(ctx context.Context, code string) error {
    _, err := r.db.Exec(ctx,
        `UPDATE shortlinks SET disabled = true WHERE code = $1`,
        code)
    if err != nil {
        return err
    }

    // 删除缓存
    if r.cache != nil {
        r.cache.Delete(ctx, code)
    }

    return nil
}
```

### 3.5 防止缓存穿透

对于不存在的短码，使用“负缓存”（哨兵值）：

```go
func (r *ShortlinksRepo) Resolve(ctx context.Context, code string) string {
    // 1. 查缓存
    if r.cache != nil {
        url, err := r.cache.Get(ctx, code)
        if err == nil {
            if url == "__nil__" {
                return "" // 命中负缓存，直接返回
            }
            return url
        }
    }

    // 2. 查数据库
    var url string
    err := r.db.QueryRow(ctx, ...).Scan(&url)

    // 3. 写缓存
    if r.cache != nil {
        if url == "" {
            r.cache.SetNotFound(ctx, code) // 写负缓存
        } else {
            r.cache.Set(ctx, code, url)
        }
    }

    return url
}
```

---

## 第四阶段：main.go 集成

```go
// main.go
import "day.local/internal/platform/cache"
import "day.local/internal/platform/ratelimit"

func main() {
    // ... 现有代码 ...

    // Redis
    redisClient, err := cache.NewRedisClient(cfg.RedisAddr, cfg.RedisPassword, cfg.RedisDB)
    if err != nil {
        slog.Warn("Redis 连接失败，限流和缓存功能禁用", "err", err)
    }
    if redisClient != nil {
        defer redisClient.Close()
    }

    // 限流器
    var limiter *ratelimit.Limiter
    if redisClient != nil {
        limiter = ratelimit.NewLimiter(redisClient)
    }

    // 短链缓存
    var slCache *slcache.ShortlinkCache
    if redisClient != nil {
        slCache = slcache.NewShortlinkCache(redisClient)
    }

    // 注入到 repo
    slRepo := repo.NewShortlinksRepo(dbPool, slCache)

    // ... 路由注册时使用 limiter ...
}
```

---

## 测试计划

### 限流测试

```bash
# 快速发送 15 个请求，应该有 5 个被拒绝
for i in {1..15}; do
  curl -s -o /dev/null -w "%{http_code}\n" -X POST http://localhost:9999/api/v1/shortlinks \
    -H "Content-Type: application/json" \
    -d '{"url": "https://example.com/'$i'"}'
done
```

### 缓存测试

```bash
# 直接跑集成测试（需要本地 docker 的 Postgres + Redis）
go test ./test -run RedisCache -count=1
```

---

## 文件清单

```
internal/
├── platform/
│   ├── cache/
│   │   └── redis.go           # Redis 连接
│   └── ratelimit/
│       └── limiter.go         # 限流器
└── app/shortlink/
    └── cache/
        └── shortlink.go       # 短链缓存
```

---

## 注意事项

1. **Redis 故障降级** - Redis 不可用时，限流放行，缓存跳过
2. **缓存一致性** - 禁用短链时必须删除缓存
3. **空值缓存** - 防止缓存穿透，但 TTL 要短
4. **监控** - 添加缓存命中率指标
