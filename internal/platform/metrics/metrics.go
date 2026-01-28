package metrics

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	// once 用来保证指标只注册一次。
	// Prometheus 的 registry 不允许重复注册同名指标，否则会直接 panic。
	once sync.Once

	// HTTPRequestsTotal：累计请求数（Counter）。
	//
	// 它会随着每次请求结束而 +1，不会减少；常用于计算 QPS/错误率。
	//
	// labels：
	// - method：HTTP 方法，例如 GET/POST
	// - route：路由模板（推荐用 pattern，例如 /api/v1/users/me；不要用带 id 的真实 path），如user/1，否则会产生无限label
	// - status：HTTP 状态码字符串，例如 "200"/"401"/"500"
	HTTPRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_request_total",
			Help: "HTTP请求的总数",
		},
		[]string{"method", "route", "status"},
	)

	// HTTPRequestDurationSeconds：请求耗时分布（Histogram）。
	//
	// 每次请求结束后用 Observe(durationSeconds) 记录一次耗时。
	// Histogram 会按 Buckets 分桶累计，这样 Prometheus/Grafana 才能算出 P95/P99 等分位数延迟。P95/P99 是延迟的分位数（percentile latency），用来描述“慢请求有多慢”。把所有请求的耗时从小到大排序：
	/*P95：有 95% 的请求耗时 ≤ 这个值；最慢的 5% 会更慢
	  P99：有 99% 的请求耗时 ≤ 这个值；最慢的 1% 会更慢*/
	//
	// labels：
	// - method：HTTP 方法
	// - route：路由模板（同上，避免高基数）
	HTTPRequestDurationSeconds = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request latency distributions.",
			Buckets: prometheus.DefBuckets, // 你也可以自定义
		},
		[]string{"method", "route"},
	)

	// HTTPInflightRequests：当前正在处理中的请求数（Gauge）。
	//
	// 请求开始时 +1，请求结束时 -1；常用于观察服务的并发压力与是否“堆积”。
	HTTPInflightRequests = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "http_inflight_requests",
			Help: "Current number of in-flight HTTP requests.",
		},
	)
	// ========== 缓存指标 ==========

	// CacheOperations：缓存操作计数
	// labels:
	// - level: "l1"（本地）或 "l2"（Redis）
	// - result: "hit"（命中）、"miss"（未命中）、"hit_negative"（命中负缓存）
	CacheOperations = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "shortlink_cache_operations_total",
			Help: "缓存操作计数",
		},
		[]string{"level", "result"},
	)
	// ========== 短链业务指标 ==========

	// ShortlinkCreated：短链创建计数
	ShortlinkCreated = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "shortlink_created_total",
			Help: "创建的短链总数",
		},
	)

	// ShortlinkRedirects：短链跳转计数
	ShortlinkRedirects = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "shortlink_redirects_total",
			Help: "短链跳转总数",
		},
	)

	// ========== 数据库指标 ==========

	// DBQueryDuration：数据库查询耗时
	// labels:
	// - operation: "resolve"、"create"、"find" 等
	DBQueryDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "shortlink_db_query_duration_seconds",
			Help:    "数据库查询耗时分布",
			Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1},
		},
		[]string{"operation"},
	)
	// StatsFlushDuration：统计写入耗时
	StatsFlushDuration = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "shortlink_stats_flush_duration_seconds",
			Help:    "统计批量写入耗时",
			Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1},
		},
	)
	// StatsFlushSize：每次 flush 的批量大小
	StatsFlushSize = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "shortlink_stats_flush_size",
			Help:    "每次 flush 的事件数量",
			Buckets: []float64{1, 10, 25, 50, 75, 100, 150, 200},
		},
	)
)

// Init 注册指标：只允许注册一次（否则 panic: duplicate metrics collector registration）
func Init() {
	once.Do(func() {
		prometheus.MustRegister(
			HTTPRequestsTotal,
			HTTPRequestDurationSeconds,
			HTTPInflightRequests,
			CacheOperations,
			ShortlinkCreated,
			ShortlinkRedirects,
			DBQueryDuration,
			StatsFlushDuration,
			StatsFlushSize,
		)
	})
}
