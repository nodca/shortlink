package queue

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

type ResearchQueue struct {
	rdb      *redis.Client
	stream   string
	group    string
	consumer string
}

type ResearchQueueConfig struct {
	Stream   string
	Group    string
	Consumer string
}

func NewResearchQueue(rdb *redis.Client, cfg ResearchQueueConfig) (*ResearchQueue, error) {
	if rdb == nil {
		return nil, errors.New("nil redis client")
	}
	if cfg.Stream == "" {
		cfg.Stream = "ai:jobs:research"
	}
	if cfg.Group == "" {
		cfg.Group = "ai:workers:research"
	}
	if cfg.Consumer == "" {
		cfg.Consumer = "worker-1"
	}

	q := &ResearchQueue{
		rdb:      rdb,
		stream:   cfg.Stream,
		group:    cfg.Group,
		consumer: cfg.Consumer,
	}

	// Create consumer group (idempotent).
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	if err := q.rdb.XGroupCreateMkStream(ctx, q.stream, q.group, "$").Err(); err != nil && !isBusyGroup(err) {
		return nil, err
	}
	return q, nil
}

func isBusyGroup(err error) bool {
	return err != nil && strings.Contains(err.Error(), "BUSYGROUP")
}

func (q *ResearchQueue) Enqueue(ctx context.Context, runID int64) error {
	args := &redis.XAddArgs{
		Stream: q.stream,
		Values: map[string]any{
			"run_id": strconv.FormatInt(runID, 10),
		},
	}
	return q.rdb.XAdd(ctx, args).Err()
}

type ResearchJob struct {
	MessageID string
	RunID     int64
}

func (q *ResearchQueue) Read(ctx context.Context, block time.Duration) ([]ResearchJob, error) {
	res, err := q.rdb.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    q.group,
		Consumer: q.consumer,
		Streams:  []string{q.stream, ">"},
		Count:    10,
		Block:    block,
	}).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, nil
		}
		return nil, err
	}

	var jobs []ResearchJob
	for _, s := range res {
		for _, msg := range s.Messages {
			raw, _ := msg.Values["run_id"]
			runStr, _ := raw.(string)
			runID, err := strconv.ParseInt(runStr, 10, 64)
			if err != nil || runID <= 0 {
				continue
			}
			jobs = append(jobs, ResearchJob{
				MessageID: msg.ID,
				RunID:     runID,
			})
		}
	}
	return jobs, nil
}

func (q *ResearchQueue) Ack(ctx context.Context, messageID string) error {
	return q.rdb.XAck(ctx, q.stream, q.group, messageID).Err()
}
