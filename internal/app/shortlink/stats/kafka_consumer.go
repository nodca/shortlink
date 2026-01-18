package stats

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/segmentio/kafka-go"
)

type KafkaConsumer struct {
	reader    *kafka.Reader
	db        *pgxpool.Pool
	batchSize int
	interval  time.Duration
}

func NewKafkaConsumer(brokers []string, topic string, db *pgxpool.Pool) *KafkaConsumer {
	return &KafkaConsumer{
		reader: kafka.NewReader(kafka.ReaderConfig{
			Brokers:  brokers,
			Topic:    topic,
			GroupID:  "click-stats-consumer",
			MinBytes: 1,
			MaxBytes: 10e6,
		}),
		db:        db,
		batchSize: 100,
		interval:  time.Second,
	}
}

func (k *KafkaConsumer) Run(ctx context.Context) {
	batch := make([]ClickEvent, 0, k.batchSize)
	ticker := time.NewTicker(k.interval)
	defer ticker.Stop()

	// 用于非阻塞读取 Kafka
	msgCh := make(chan ClickEvent, k.batchSize)
	errCh := make(chan error, 1)

	// 启动读取协程
	go func() {
		for {
			msg, err := k.reader.ReadMessage(ctx)
			if err != nil {
				if ctx.Err() != nil {
					close(msgCh)
					return
				}
				slog.Error("kafka read failed", "err", err)
				continue
			}

			var event ClickEvent
			if err := json.Unmarshal(msg.Value, &event); err != nil {
				slog.Error("unmarshal event failed", "err", err)
				continue
			}
			msgCh <- event
		}
	}()

	for {
		select {
		case <-ctx.Done():
			k.flush(batch)
			return

		case event, ok := <-msgCh:
			if !ok {
				k.flush(batch)
				return
			}
			batch = append(batch, event)
			if len(batch) >= k.batchSize {
				k.flush(batch)
				batch = batch[:0]
			}

		case <-ticker.C:
			if len(batch) > 0 {
				k.flush(batch)
				batch = batch[:0]
			}

		case err := <-errCh:
			slog.Error("kafka consumer error", "err", err)
		}
	}
}

func (k *KafkaConsumer) flush(batch []ClickEvent) {
	if len(batch) == 0 {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	tx, err := k.db.Begin(ctx)
	if err != nil {
		slog.Error("kafka consumer: begin tx failed", "err", err)
		return
	}
	defer tx.Rollback(context.Background())

	for _, e := range batch {
		if _, err := tx.Exec(ctx,
			`INSERT INTO click_stats (code,clicked_at,ip,user_agent,referer) VALUES ($1,$2,$3,$4,$5)`,
			e.Code, e.ClickedAt, e.IP, e.UserAgent, e.Referer); err != nil {
			slog.Error("kafka consumer: insert failed", "err", err, "code", e.Code)
			continue
		}
		if _, err := tx.Exec(ctx,
			`UPDATE shortlinks SET click_count = click_count + 1 WHERE code = $1`,
			e.Code); err != nil {
			slog.Error("kafka consumer: update count failed", "err", err, "code", e.Code)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		slog.Error("kafka consumer: commit failed", "err", err)
	} else {
		slog.Debug("kafka consumer: flushed", "count", len(batch))
	}
}

func (k *KafkaConsumer) Close() {
	k.reader.Close()
}
