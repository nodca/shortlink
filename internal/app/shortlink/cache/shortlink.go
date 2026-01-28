package cache

import (
	"context"
	"log/slog"
	"time"

	"day.local/internal/platform/metrics"
	"github.com/redis/go-redis/v9"
)

const notFoundSentinel = "__nil__"

type ShortlinkCache struct {
	client   *redis.Client
	local    *LocalCache // L1 本地缓存
	ttl      time.Duration
	emptyTTL time.Duration
}

func NewShortlinkCache(client *redis.Client, local *LocalCache) *ShortlinkCache {
	return &ShortlinkCache{
		client:   client,
		local:    local,
		ttl:      time.Hour,
		emptyTTL: 30 * time.Second,
	}
}

func (c *ShortlinkCache) Get(ctx context.Context, code string) (string, error) {
	// L1: 本地缓存
	if c.local != nil {
		if url, ok := c.local.Get(code); ok {
			if url == notFoundSentinel {
				metrics.CacheOperations.WithLabelValues("l1", "hit_negative").Inc()
			} else {
				metrics.CacheOperations.WithLabelValues("l1", "hit").Inc()
			}
			return url, nil
		}
	}

	// L2: Redis
	key := "sl:" + code
	res, err := c.client.Get(ctx, key).Result()
	if err == redis.Nil {
		metrics.CacheOperations.WithLabelValues("l2", "miss").Inc()
		return "", nil // 缓存未命中
	}
	if err != nil {
		return "", err
	}
	// L2 命中
	if res == notFoundSentinel {
		metrics.CacheOperations.WithLabelValues("l2", "hit_negative").Inc()
	} else {
		metrics.CacheOperations.WithLabelValues("l2", "hit").Inc()
	}

	// 回填本地缓存
	if c.local != nil {
		if res == notFoundSentinel {
			c.local.SetNotFound(code)
		} else {
			c.local.Set(code, res)
		}
	}
	return res, nil
}

func (c *ShortlinkCache) Set(ctx context.Context, code, url string) error {
	// 同时写入本地缓存
	if c.local != nil {
		c.local.Set(code, url)
	}
	return c.client.Set(ctx, "sl:"+code, url, c.ttl).Err()
}

func (c *ShortlinkCache) Delete(ctx context.Context, code string) error {
	// 同时删除本地缓存
	if c.local != nil {
		c.local.Del(code)
	}
	return c.client.Del(ctx, "sl:"+code).Err()
}

// SetNotFound 用明确哨兵值做"负缓存"，避免缓存穿透。
// 不要用 "" 作为哨兵值（可读性差、也容易把"未命中"和"命中空值"混淆）。
func (c *ShortlinkCache) SetNotFound(ctx context.Context, code string) error {
	// 同时写入本地负缓存
	if c.local != nil {
		c.local.SetNotFound(code)
	}
	return c.client.Set(ctx, "sl:"+code, notFoundSentinel, c.emptyTTL).Err()
}

// Close 关闭本地缓存
func (c *ShortlinkCache) Close() {
	if c.local != nil {
		c.local.Close()
		slog.Info("本地缓存已关闭")
	}
}
