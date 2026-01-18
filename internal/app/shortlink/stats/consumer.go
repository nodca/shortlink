package stats

import (
	"context"
	"log/slog"
	"time"

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

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	tx, err := c.db.Begin(ctx)
	if err != nil {
		slog.Error("click stats: begin tx failed", "err", err)
		return
	}
	defer tx.Rollback(context.Background())

	for _, e := range batch {
		//插入详细统计记录
		if _, err := tx.
			Exec(ctx, `INSERT INTO click_stats (code,clicked_at,ip,user_agent,referer) VALUES ($1,$2,$3,$4,$5)`, e.Code, e.ClickedAt, e.IP, e.UserAgent, e.Referer); err != nil {
			slog.Error("click stats: insert failed", "err", err, "code", e.Code)
			continue
		}
		//更新计数
		if _, err := tx.
			Exec(ctx, `UPDATE shortlinks SET click_count = click_count + 1 WHERE code = $1`, e.Code); err != nil {
			slog.Error("click stats: update count failed", "err", err, "code", e.Code)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		slog.Error("click stats: commit failed", "err", err)
	} else {
		slog.Debug("click stats: flushed", "count", len(batch))
	}
}
