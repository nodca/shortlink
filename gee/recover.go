package gee

import (
	"fmt"
	"log/slog"
	"net/http"
	"runtime"
	"strings"
)

// print stack trace for debug
func trace(message string) string {
	var pcs [32]uintptr
	n := runtime.Callers(3, pcs[:]) // skip first 3 caller

	var str strings.Builder
	str.WriteString(message + "\nTraceback:")
	for _, pc := range pcs[:n] {
		fn := runtime.FuncForPC(pc)
		file, line := fn.FileLine(pc)
		str.WriteString(fmt.Sprintf("\n\t%s:%d", file, line))
	}
	return str.String()
}

func Recovery() HandlerFunc {
	return func(ctx *Context) {
		defer func() {
			if err := recover(); err != nil {
				message := fmt.Sprintf("%v", err)
				slog.Error("Error",
					"request_id", ctx.Req.Header.Get("X-Request-ID"),
					"method", ctx.Method,
					"path", ctx.Path,
					"panic", err,
					"stack", trace(message),
				)
				if ctx.Writer.Written() {
					ctx.Abort()
					return
				}
				ctx.AbortWithError(http.StatusInternalServerError, "Internal Server Error")
			}

		}()
		ctx.Next()
	}
}
