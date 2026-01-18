package test

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"testing"
	"time"

	"day.local/internal/platform/ratelimit"
	"github.com/redis/go-redis/v9"
)

func TestLimiterSlidingWindow(t *testing.T) {
	t.Helper()

	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}
	redisPassword := os.Getenv("REDIS_PASSWORD")
	redisDB := 0
	if v := os.Getenv("REDIS_DB"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			redisDB = n
		}
	}

	client := redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Password: redisPassword,
		DB:       redisDB,
	})
	t.Cleanup(func() { _ = client.Close() })

	pingCtx, cancel := context.WithTimeout(context.Background(), 800*time.Millisecond)
	defer cancel()
	if err := client.Ping(pingCtx).Err(); err != nil {
		t.Skipf("skip: redis not available at %s: %v", redisAddr, err)
	}

	limiter := ratelimit.NewLimiter(client)

	key := fmt.Sprintf("test:rl:%d", time.Now().UnixNano())
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = client.Del(ctx, key).Err()
	})

	window := 2 * time.Second
	limit := 3

	callAllow := func(member string) (bool, time.Duration) {
		ctx, cancel := context.WithTimeout(context.Background(), 800*time.Millisecond)
		defer cancel()

		allowed, retryAfter, err := limiter.Allow(ctx, key, limit, window, member)
		if err != nil {
			t.Fatalf("Allow: %v", err)
		}
		return allowed, retryAfter
	}

	// 前 limit 次应放行
	for i := 0; i < limit; i++ {
		allowed, _ := callAllow(fmt.Sprintf("%d-%d", time.Now().UnixNano(), i))
		if !allowed {
			t.Fatalf("expected allowed at attempt %d", i+1)
		}
	}

	// 第 limit+1 次应被拒绝
	allowed, retryAfter := callAllow(fmt.Sprintf("%d-over", time.Now().UnixNano()))
	if allowed {
		t.Fatalf("expected denied at attempt %d", limit+1)
	}
	if retryAfter <= 0 || retryAfter > window {
		t.Fatalf("unexpected retryAfter: %v (window=%v)", retryAfter, window)
	}

	// 等窗口滑过后应该重新放行
	time.Sleep(retryAfter + 200*time.Millisecond)
	allowed, _ = callAllow(fmt.Sprintf("%d-after", time.Now().UnixNano()))
	if !allowed {
		t.Fatalf("expected allowed after waiting, retryAfter=%v", retryAfter)
	}
}
