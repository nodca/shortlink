package main

import (
	"context"
	"encoding/json"
	"log"
	"log/slog"
	"net/http"
	"net/http/pprof"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"day.local/gee"
	"day.local/gee/middleware"
	slcache "day.local/internal/app/shortlink/cache"
	shortlinkhttpapi "day.local/internal/app/shortlink/httpapi"
	"day.local/internal/app/shortlink/repo"
	"day.local/internal/app/shortlink/stats"
	"day.local/internal/platform/auth"
	platformcache "day.local/internal/platform/cache"
	"day.local/internal/platform/config"
	"day.local/internal/platform/db"
	"day.local/internal/platform/httpmiddleware"
	"day.local/internal/platform/httpserver"
	"day.local/internal/platform/metrics"
	"day.local/internal/platform/ratelimit"
	"day.local/internal/platform/trace"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

var (
	version   = "dev"
	commit    = "none"
	buildTime = "unknown"
)

func main() {
	cfg := config.Load()

	h := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: cfg.LogLevel,
	})
	slog.SetDefault(slog.New(h))
	//DB
	dbCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	dbPool, errDB := db.New(dbCtx, cfg.DBDSN)
	if errDB != nil {
		log.Fatal(errDB)
	}
	defer dbPool.Close()
	if err := dbPool.Ping(dbCtx); err != nil {
		log.Fatal(err)
	}
	slog.Info("数据库连接成功")

	usersRepo := repo.NewUsersRepo(dbPool)

	//Redis
	redisClient, errRedis := platformcache.NewRedisClient(cfg.RedisAddr, cfg.RedisPassword, cfg.RedisDB)
	if errRedis != nil {
		log.Fatal(errRedis)
	}
	defer redisClient.Close()
	//限流器
	var limiter *ratelimit.Limiter
	if cfg.RateLimitEnabled {
		limiter = ratelimit.NewLimiter(redisClient)
	} else {
		slog.Warn("RateLimit disabled by config", "RATELIMIT_ENABLED", false)
	}
	//短链缓存
	localCache, errLocal := slcache.NewLocalCache(100000, 1<<24) // 10万条目，16MB
	if errLocal != nil {
		log.Fatal(errLocal)
	}
	slCache := slcache.NewShortlinkCache(redisClient, localCache)
	defer slCache.Close()
	//创建布隆过滤器 预期 100 万短码，1% 误判率
	bloomFilter := slcache.NewBloomFilter(1_000_000, 0.01)

	slRepo := repo.NewShortlinksRepo(dbPool, slCache, bloomFilter)

	//初始化统计收集器（根据配置选择 Channel 或 Kafka）
	var collector stats.Collector
	var kafkaConsumer *stats.KafkaConsumer
	var channelConsumer *stats.Consumer
	if cfg.KafkaEnabled {
		slog.Info("使用 Kafka 收集点击统计", "brokers", cfg.KafkaBrokers, "topic", cfg.KafkaTopic)
		collector = stats.NewKafkaCollector(cfg.KafkaBrokers, cfg.KafkaTopic)
		kafkaConsumer = stats.NewKafkaConsumer(cfg.KafkaBrokers, cfg.KafkaTopic, dbPool)
	} else {
		slog.Info("使用 Channel 收集点击统计")
		channelCollector := stats.NewChannelCollector(10000)
		collector = channelCollector
		channelConsumer = stats.NewConsumer(dbPool, channelCollector)
	}

	// JWT
	ts, jwtErr := auth.NewHS256Service(cfg.JWTSecret, cfg.JWTIssuer, cfg.JWTTTL)
	if jwtErr != nil {
		log.Fatal(jwtErr)
	}

	metrics.Init()

	var shutdown func(context.Context) error
	if cfg.TracingEnabled {
		shutdown = trace.InitTrace(cfg.OtlpGrpcEndpoint, cfg.OtlpServiceName)
		if shutdown == nil {
			slog.Error("Trace init failed")
		} else {
			defer func() {
				ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
				defer cancel()
				if err := shutdown(ctx); err != nil {
					slog.Error(err.Error())
				}
			}()
		}
	} else {
		slog.Warn("Tracing disabled by config", "TRACING_ENABLED", false)
	}

	// 对外业务
	r := gee.New()
	r.Use(gee.Recovery(), middleware.ReqID(), middleware.AccessLog(), httpmiddleware.Metrics(), httpmiddleware.TraceName())

	api := r.Group("/api/v1")

	// App routes (can mount multiple apps).
	shortlinkhttpapi.RegisterWebRoutes(r)
	shortlinkhttpapi.RegisterPublicRoutes(r, slRepo, collector, limiter)
	shortlinkhttpapi.RegisterAPIRoutes(api, slRepo, usersRepo, ts, limiter)

	r.GET("/healthz", func(ctx *gee.Context) {
		ctx.String(http.StatusOK, "ok")
	})

	publicHandler := http.Handler(r)
	if cfg.TracingEnabled {
		publicHandler = otelhttp.NewHandler(r, "http")
	}
	publicSrv := httpserver.New(cfg, publicHandler)

	// 仅本机/内网
	adminMux := http.NewServeMux()
	adminMux.Handle("/metrics", promhttp.Handler())
	// 数据库连接状态检测
	adminMux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		dbCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := dbPool.Ping(dbCtx); err != nil {
			w.WriteHeader(500)
			w.Write([]byte("DB Ping Err"))
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("DB ready"))
	})

	adminMux.HandleFunc("/version", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"service_name": cfg.ServiceName,
			"version":      version,
			"commit":       commit,
			"build_time":   buildTime,
			"go_version":   runtime.Version(),
		})
	})

	if cfg.PprofEnabled {
		adminMux.HandleFunc("/debug/pprof/", pprof.Index)
		adminMux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
		adminMux.HandleFunc("/debug/pprof/profile", pprof.Profile)
		adminMux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
		adminMux.HandleFunc("/debug/pprof/trace", pprof.Trace)
	}

	adminSrv := &http.Server{
		Addr:              cfg.AdminAddr, // 推荐：127.0.0.1:6060
		Handler:           adminMux,
		ReadHeaderTimeout: cfg.ReadHeaderTimeout,
		ReadTimeout:       cfg.ReadTimeout,
		WriteTimeout:      cfg.WriteTimeout,
		IdleTimeout:       cfg.IdleTimeout,
	}

	stopCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errch := make(chan error, 2)

	go func() {
		errch <- httpserver.RunWithGracefulShutdownContext(publicSrv, cfg.ShutdownTimeout, stopCtx)
	}()
	go func() {
		errch <- httpserver.RunWithGracefulShutdownContext(adminSrv, cfg.ShutdownTimeout, stopCtx)
	}()

	// 启动 Kafka consumer（如果启用）
	if kafkaConsumer != nil {
		go kafkaConsumer.Run(stopCtx)
		defer kafkaConsumer.Close()
	}
	// 启动 Channel consumer（如果启用）
	if channelConsumer != nil {
		go channelConsumer.Run(stopCtx)
	}
	defer collector.Close()

	err := <-errch
	if err != nil {
		stop()
		select {
		case <-errch:
		case <-time.After(cfg.ShutdownTimeout + time.Second):
		}
		log.Fatal(err)
	}

	stop()
	<-errch
}
