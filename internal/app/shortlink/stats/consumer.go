package stats

import (
	"context"
	"log/slog"
	"time"

	"day.local/internal/platform/metrics"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// 消费点击事件
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
		batchSize: 100,         //批量写入大小
		interval:  time.Second, //最大等待时间
	}
}

// 阻塞 消费循环
func (c *Consumer) Run(ctx context.Context) {
	batch := make([]ClickEvent, 0, c.batchSize)
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			c.flush(batch) //清理剩余事件
			return
		case event, ok := <-c.collector.Events():
			if !ok {
				c.flush(batch)
				return
			}
			batch = append(batch, event)
			if len(batch) >= c.batchSize {
				c.flush(batch)
				batch = batch[:0] //清空切片，但保留容量不变，避免反复分配内存
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
	start := time.Now() // 记录开始时间
	defer func() {
		metrics.StatsFlushDuration.Observe(time.Since(start).Seconds())
		metrics.StatsFlushSize.Observe(float64(len(batch)))
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	tx, err := c.db.Begin(ctx)
	if err != nil {
		slog.Error("click stats: begin tx failed", "err", err)
		return
	}
	defer tx.Rollback(context.Background())

	//使用CopyFrom批量插入 click_stats
	rows := make([][]any, len(batch))
	for i, e := range batch {
		rows[i] = []any{e.Code, e.ClickedAt, e.IP, e.UserAgent, e.Referer}
	}

	_, err = tx.CopyFrom(
		ctx,
		pgx.Identifier{"click_stats"},
		[]string{"code", "clicked_at", "ip", "user_agent", "referer"},
		pgx.CopyFromRows(rows),
	)
	if err != nil {
		slog.Error("click stats: copy failed", "err", err)
		return
	}
	//聚合统计每个 code 的点击数
	counts := make(map[string]int)
	for _, e := range batch {
		counts[e.Code]++
	}
	//批量更新数据库的count 使用 unnest
	codes := make([]string, 0, len(counts))
	deltas := make([]int, 0, len(counts))
	for code, delta := range counts {
		codes = append(codes, code)
		deltas = append(deltas, delta)
	}

	_, err = tx.Exec(ctx, `
          UPDATE shortlinks s
          SET click_count = s.click_count + v.delta,
              updated_at = now()
          FROM unnest($1::text[], $2::int[]) AS v(code, delta)
          WHERE s.code = v.code
      `, codes, deltas)
	if err != nil {
		slog.Error("click stats: batch update failed", "err", err)
		return
	}

	if err := tx.Commit(ctx); err != nil {
		slog.Error("click stats: commit failed", "err", err)
	} else {
		slog.Debug("click stats: flushed", "count", len(batch))
	}
}
