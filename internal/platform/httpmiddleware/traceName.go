package httpmiddleware

import (
	"day.local/gee"
	"go.opentelemetry.io/otel/trace"
)

func TraceName() gee.HandlerFunc {
	return func(ctx *gee.Context) {
		span := trace.SpanFromContext(ctx.Req.Context())
		span.SetName(ctx.Method + " " + ctx.RoutePattern)
		ctx.Next()
	}
}
