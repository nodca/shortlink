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
)

// Init 注册指标：只允许注册一次（否则 panic: duplicate metrics collector registration）
func Init() {
	once.Do(func() {
		prometheus.MustRegister(
			HTTPRequestsTotal,
			HTTPRequestDurationSeconds,
			HTTPInflightRequests,
		)
	})
}
