package gee

import (
	"log/slog"
	"time"
)

func Logger() HandlerFunc {
	return func(ctx *Context) {
		start := time.Now()
		ctx.Next()
		slog.Info("request",
			"status", ctx.Writer.Status(),
			"method", ctx.Method,
			"path", ctx.Req.RequestURI,
			"latency_us", time.Since(start).Microseconds(),
			"bytes", ctx.Writer.Size(),
		)
	}
}
