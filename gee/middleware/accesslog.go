package middleware

import (
	"log/slog"
	"time"

	"day.local/gee"
)

func AccessLog() gee.HandlerFunc {
	return func(ctx *gee.Context) {
		start := time.Now()

		ctx.Next()

		slog.Info("access",
			"request_id", ctx.Req.Header.Get("X-Request-ID"),
			"method", ctx.Method,
			"path", ctx.Path,
			"status", ctx.Writer.Status(),
			"bytes", ctx.Writer.Size(),
			"latency_ms", time.Since(start).Milliseconds())
	}
}
