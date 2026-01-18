package test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"
	"time"

	"day.local/gee"
	"day.local/internal/platform/httpmiddleware"
	"day.local/internal/platform/ratelimit"
	"github.com/redis/go-redis/v9"
)

func TestRateLimitMiddleware_HTTP(t *testing.T) {
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

	r := gee.New()
	window := 2 * time.Second
	limit := 2
	r.GET("/t",
		httpmiddleware.RateLimit(limiter, "test", limit, window),
		func(ctx *gee.Context) { ctx.String(http.StatusOK, "ok") },
	)

	doReq := func(remoteAddr string, headers map[string]string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/t", nil)
		req.RemoteAddr = remoteAddr
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		return rec
	}

	// 模拟 Cloudflare->Caddy：RemoteAddr=127.0.0.1(可信代理)，真实 IP 放在 CF-Connecting-IP。
	h := map[string]string{"CF-Connecting-IP": "203.0.113.10"}
	if got := doReq("127.0.0.1:1234", h).Code; got != http.StatusOK {
		t.Fatalf("1st request: got %d, want %d", got, http.StatusOK)
	}
	if got := doReq("127.0.0.1:1234", h).Code; got != http.StatusOK {
		t.Fatalf("2nd request: got %d, want %d", got, http.StatusOK)
	}

	rec := doReq("127.0.0.1:1234", h)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("3rd request: got %d, want %d, body=%s", rec.Code, http.StatusTooManyRequests, rec.Body.String())
	}

	// 等窗口滑过后应恢复
	time.Sleep(window + 200*time.Millisecond)
	if got := doReq("127.0.0.1:1234", h).Code; got != http.StatusOK {
		t.Fatalf("after window: got %d, want %d", got, http.StatusOK)
	}
}
