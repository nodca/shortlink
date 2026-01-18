# 短链流量统计 - Channel 异步方案

## 目标

实现短链点击统计功能，使用 Go channel 异步写入，为后续迁移 Kafka 打基础。

---

## 第一阶段：数据库准备

### 1.1 新建迁移文件 `migrations/006_click_stats.sql`

```sql
-- 点击统计表（不使用外键，解耦设计）
-- 理由：
--   1. 统计是高频写入场景，外键检查有性能开销
--   2. 统计数据非核心业务，允许与主表解耦
--   3. 删除短链时不需要级联处理统计数据
CREATE TABLE IF NOT EXISTS click_stats (
    id BIGSERIAL PRIMARY KEY,
    code TEXT NOT NULL,
    clicked_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    ip TEXT,
    user_agent TEXT,
    referer TEXT
);

-- 索引：按短链 code 查询
CREATE INDEX idx_click_stats_code ON click_stats(code);

-- 索引：按时间查询（用于统计报表）
CREATE INDEX idx_click_stats_clicked_at ON click_stats(clicked_at);

-- 在 shortlinks 表加计数器字段（用于快速查询总数，避免 COUNT）
ALTER TABLE shortlinks ADD COLUMN IF NOT EXISTS click_count BIGINT DEFAULT 0;
```

### 1.2 执行迁移

```bash
psql -U days -d days -f migrations/006_click_stats.sql
```

---

## 第二阶段：实现 Channel 异步统计

### 2.1 新建 `internal/app/shortlink/stats/collector.go`

```go
package stats

import (
    "time"
)

// ClickEvent 点击事件
type ClickEvent struct {
    Code      string
    ClickedAt time.Time
    IP        string
    UserAgent string
    Referer   string
}

// Collector 收集器接口（方便后续换 Kafka）
type Collector interface {
    Collect(event ClickEvent)
    Close()
}

// ChannelCollector 基于 channel 的收集器
type ChannelCollector struct {
    ch     chan ClickEvent
    closed bool
}

func NewChannelCollector(bufferSize int) *ChannelCollector {
    return &ChannelCollector{
        ch: make(chan ClickEvent, bufferSize),
    }
}

func (c *ChannelCollector) Collect(event ClickEvent) {
    if c.closed {
        return
    }
    select {
    case c.ch <- event:
    default:
        // channel 满了，丢弃（生产环境可以记录日志或 metrics）
    }
}

func (c *ChannelCollector) Events() <-chan ClickEvent {
    return c.ch
}

func (c *ChannelCollector) Close() {
    c.closed = true
    close(c.ch)
}
```

### 2.2 新建 `internal/app/shortlink/stats/consumer.go`

```go
package stats

import (
    "context"
    "log/slog"
    "time"

    "github.com/jackc/pgx/v5/pgxpool"
)

// Consumer 消费点击事件并写入数据库
type Consumer struct {
    db        *pgxpool.Pool
    collector *ChannelCollector
    batchSize int
    interval  time.Duration
}

func NewConsumer(db *pgxpool.Pool, collector *ChannelCollector) *Consumer {
    return &Consumer{
        db:        db,
        collector: collector,
        batchSize: 100,           // 批量写入大小
        interval:  time.Second,   // 最大等待时间
    }
}

// Run 启动消费循环（阻塞，应在 goroutine 中调用）
func (c *Consumer) Run(ctx context.Context) {
    batch := make([]ClickEvent, 0, c.batchSize)
    ticker := time.NewTicker(c.interval)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            // 优雅关闭：处理剩余事件
            c.flush(batch)
            return

        case event, ok := <-c.collector.Events():
            if !ok {
                c.flush(batch)
                return
            }
            batch = append(batch, event)
            if len(batch) >= c.batchSize {
                c.flush(batch)
                batch = batch[:0]
            }

        case <-ticker.C:
            if len(batch) > 0 {
                c.flush(batch)
                batch = batch[:0]
            }
        }
    }
}

func (c *Consumer) flush(batch []ClickEvent) {
    if len(batch) == 0 {
        return
    }

    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    // 批量插入 + 更新计数器
    tx, err := c.db.Begin(ctx)
    if err != nil {
        slog.Error("click stats: begin tx failed", "err", err)
        return
    }
    defer tx.Rollback(ctx)

    for _, e := range batch {
        // 插入详细记录
        _, err := tx.Exec(ctx,
            `INSERT INTO click_stats (code, clicked_at, ip, user_agent, referer)
             VALUES ($1, $2, $3, $4, $5)`,
            e.Code, e.ClickedAt, e.IP, e.UserAgent, e.Referer)
        if err != nil {
            slog.Error("click stats: insert failed", "err", err, "code", e.Code)
            continue
        }

        // 更新计数器
        _, err = tx.Exec(ctx,
            `UPDATE shortlinks SET click_count = click_count + 1 WHERE code = $1`,
            e.Code)
        if err != nil {
            slog.Error("click stats: update count failed", "err", err, "code", e.Code)
        }
    }

    if err := tx.Commit(ctx); err != nil {
        slog.Error("click stats: commit failed", "err", err)
    } else {
        slog.Debug("click stats: flushed", "count", len(batch))
    }
}
```

---

## 第三阶段：集成到应用

### 3.1 修改 `httpapi/shortlinks.go` - RedirectHandler

```go
func NewRedirectHandler(r *repo.ShortlinksRepo, collector stats.Collector) gee.HandlerFunc {
    return func(ctx *gee.Context) {
        code := ctx.Param("code")
        url := r.Resolve(ctx.Req.Context(), code)
        if url == "" {
            ctx.AbortWithError(http.StatusNotFound, "url not found")
            return
        }

        // 异步记录点击
        collector.Collect(stats.ClickEvent{
            Code:      code,
            ClickedAt: time.Now(),
            IP:        ctx.Req.RemoteAddr,
            UserAgent: ctx.Req.UserAgent(),
            Referer:   ctx.Req.Referer(),
        })

        ctx.SetHeader("Location", url)
        ctx.Status(http.StatusFound)
    }
}
```

### 3.2 修改 `httpapi/register.go`

```go
func RegisterPublicRoutes(engine *gee.Engine, r *repo.ShortlinksRepo, collector stats.Collector) {
    engine.GET("/:code", NewRedirectHandler(r, collector))
}
```

### 3.3 修改 `cmd/api/main.go`

```go
// 初始化统计收集器
collector := stats.NewChannelCollector(10000)
consumer := stats.NewConsumer(dbPool, collector)

// 启动消费者
go consumer.Run(stopCtx)

// 注册路由时传入 collector
httpapi.RegisterPublicRoutes(r, slRepo, collector)

// 优雅关闭时
defer collector.Close()
```

---

## 第四阶段：查询统计数据

### 4.1 新增 API 端点（游标分页）

```
GET /api/v1/users/shortlinks/:code/stats?limit=20&cursor=xxx
```

参数：
- `limit`: 每页条数，默认 20，最大 100
- `cursor`: 游标（上一页最后一条记录的 id），首次请求不传

返回：
```json
{
    "code": "abc",
    "total_clicks": 1234,
    "recent_clicks": [
        {
            "id": 100,
            "clicked_at": "2026-01-16T10:00:00Z",
            "ip": "1.2.3.4",
            "user_agent": "Mozilla/5.0...",
            "referer": "https://google.com"
        },
        ...
    ],
    "next_cursor": 80
}
```

说明：
- `total_clicks`: 从 `shortlinks.click_count` 读取，O(1) 查询
- `recent_clicks`: 按 `id DESC` 排序的点击明细
- `next_cursor`: 下一页游标，为 null 时表示没有更多数据

### 4.2 数据库查询

```sql
-- 首次查询（无 cursor）
SELECT id, clicked_at, ip, user_agent, referer
FROM click_stats
WHERE code = $1
ORDER BY id DESC
LIMIT $2;

-- 翻页查询（有 cursor）
SELECT id, clicked_at, ip, user_agent, referer
FROM click_stats
WHERE code = $1 AND id < $2
ORDER BY id DESC
LIMIT $3;
```

### 4.3 在"我的短链"列表中显示点击数

修改 `ListByUserID` 返回 `click_count`。

---

## 第五阶段：后续迁移 Kafka（可选）

当 channel 方案验证通过后，迁移到 Kafka：

1. 新建 `stats/kafka_collector.go` 实现 `Collector` 接口
2. 新建 `stats/kafka_consumer.go` 或独立服务
3. `main.go` 中根据配置选择使用哪个 Collector

```go
var collector stats.Collector
if cfg.KafkaEnabled {
    collector = stats.NewKafkaCollector(cfg.KafkaBrokers, "click-events")
} else {
    collector = stats.NewChannelCollector(10000)
}
```

---

## 检查清单

- [ ] 创建迁移文件 `006_click_stats.sql`
- [ ] 执行数据库迁移
- [ ] 实现 `stats/collector.go`
- [ ] 实现 `stats/consumer.go`
- [ ] 修改 `RedirectHandler` 收集点击事件
- [ ] 修改 `RegisterPublicRoutes` 传入 collector
- [ ] 修改 `main.go` 初始化和启动 consumer
- [ ] 测试：点击短链后检查数据库记录
- [ ] 新增统计查询 API
- [ ] 前端显示点击数
