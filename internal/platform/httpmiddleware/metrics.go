package httpmiddleware

import (
	"strconv"
	"time"

	"day.local/gee"
	"day.local/internal/platform/metrics"
)

func Metrics() gee.HandlerFunc {
	return func(ctx *gee.Context) {
		start := time.Now()
		metrics.HTTPInflightRequests.Inc()       //正在处理的请求数+1
		defer metrics.HTTPInflightRequests.Dec() //请求处理结束
		routePattern := ctx.RoutePattern
		if routePattern == "" {
			routePattern = "UNMATCHED"
		}
		defer func() {
			duration := time.Since(start).Seconds()
			status := ctx.Writer.Status()
			metrics.HTTPRequestsTotal.WithLabelValues(ctx.Method, routePattern, strconv.Itoa(status)).Inc()
			metrics.HTTPRequestDurationSeconds.WithLabelValues(ctx.Method, routePattern).Observe(duration)
		}()
		ctx.Next()
	}
}
