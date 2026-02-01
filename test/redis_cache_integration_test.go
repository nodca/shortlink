package test

import (
	"context"
	"os"
	"strconv"
	"testing"
	"time"

	slcache "day.local/internal/app/shortlink/cache"
	"day.local/internal/app/shortlink/repo"
	"day.local/internal/platform/db"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

func setupPostgresAndRedis(t *testing.T) (*repo.ShortlinksRepo, *pgxpool.Pool, *redis.Client, func()) {
	t.Helper()

	// Postgres
	dbCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	dsn := os.Getenv("DB_DSN")
	if dsn == "" {
		dsn = "postgres://days:days@localhost:5432/days?sslmode=disable"
	}
	dbPool, err := db.New(dbCtx, dsn)
	if err != nil {
		t.Skipf("skip: cannot connect to postgres: %v", err)
	}
	if err := dbPool.Ping(dbCtx); err != nil {
		dbPool.Close()
		t.Skipf("skip: cannot ping postgres: %v", err)
	}

	// Redis
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
	redisClient := redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Password: redisPassword,
		DB:       redisDB,
	})

	pingCtx, pingCancel := context.WithTimeout(context.Background(), 800*time.Millisecond)
	defer pingCancel()
	if err := redisClient.Ping(pingCtx).Err(); err != nil {
		redisClient.Close()
		dbPool.Close()
		t.Skipf("skip: cannot connect to redis at %s: %v", redisAddr, err)
	}

	cache := slcache.NewShortlinkCache(redisClient, nil)
	slRepo := repo.NewShortlinksRepo(dbPool, cache, nil)

	cleanup := func() {
		_ = redisClient.Close()
		dbPool.Close()
	}
	return slRepo, dbPool, redisClient, cleanup
}

func TestRedisCache_ResolveCachesAndDisableInvalidates(t *testing.T) {
	slRepo, dbPool, redisClient, cleanup := setupPostgresAndRedis(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	url1 := "https://example.com/cache-test-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	code, err := slRepo.Create(ctx, url1, nil)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// 1) 第一次 Resolve：走 DB -> 写缓存
	got1 := slRepo.Resolve(ctx, code)
	if got1 != url1 {
		t.Fatalf("Resolve#1: got %q, want %q", got1, url1)
	}
	val1, err := redisClient.Get(ctx, "sl:"+code).Result()
	if err != nil {
		t.Fatalf("redis GET: %v", err)
	}
	if val1 != url1 {
		t.Fatalf("redis value: got %q, want %q", val1, url1)
	}

	// 2) 修改 DB 中的 url，再次 Resolve 应优先命中缓存（返回旧值）
	url2 := "https://example.com/cache-test-updated-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	if _, err := dbPool.Exec(ctx, "UPDATE shortlinks SET url=$1 WHERE code=$2", url2, code); err != nil {
		t.Fatalf("update db url: %v", err)
	}
	got2 := slRepo.Resolve(ctx, code)
	if got2 != url1 {
		t.Fatalf("Resolve#2 (expect cache hit): got %q, want %q", got2, url1)
	}

	// 3) 禁用后必须删缓存，且 Resolve 返回 not found
	if err := slRepo.DisableByCode(ctx, code); err != nil {
		t.Fatalf("DisableByCode: %v", err)
	}
	_, err = redisClient.Get(ctx, "sl:"+code).Result()
	if err == nil {
		t.Fatalf("expected cache key to be deleted after disable")
	}
	got3 := slRepo.Resolve(ctx, code)
	if got3 != "" {
		t.Fatalf("Resolve#3 after disable: got %q, want empty", got3)
	}
}

func TestRedisCache_NegativeCacheTTLAndCustomCodeOverrides(t *testing.T) {
	slRepo, _, redisClient, cleanup := setupPostgresAndRedis(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	customCode := "C" + strconv.FormatInt(time.Now().UnixNano()%1_000_000_000_000, 36)

	// 1) 先制造负缓存
	slRepo.Resolve(ctx, customCode)
	val, err := redisClient.Get(ctx, "sl:"+customCode).Result()
	if err != nil {
		t.Fatalf("redis GET negative: %v", err)
	}
	if val != "__nil__" {
		t.Fatalf("expected negative sentinel, got %q", val)
	}
	ttl := redisClient.TTL(ctx, "sl:"+customCode).Val()
	if ttl <= 0 || ttl > 2*time.Minute {
		t.Fatalf("unexpected negative TTL: %v", ttl)
	}

	// 2) 创建同名自定义短码后，应覆盖负缓存
	url := "https://example.com/custom-code-override-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	gotCode, err := slRepo.CreateWithCustomCode(ctx, url, customCode, nil)
	if err != nil {
		t.Fatalf("CreateWithCustomCode: %v", err)
	}
	if gotCode != customCode {
		t.Fatalf("code mismatch: got %q, want %q", gotCode, customCode)
	}

	val2, err := redisClient.Get(ctx, "sl:"+customCode).Result()
	if err != nil {
		t.Fatalf("redis GET after override: %v", err)
	}
	if val2 != url {
		t.Fatalf("expected cached url after override, got %q, want %q", val2, url)
	}
}
