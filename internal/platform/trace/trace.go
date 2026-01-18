package trace

import (
	"context"
	"log/slog"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"
)

func InitTrace(endpoint string, serviceName string) (shutdown func(context.Context) error) {
	ctx, stop := context.WithTimeout(context.Background(), 5*time.Second)
	defer stop()

	exporter, err := otlptracegrpc.New(ctx, otlptracegrpc.WithEndpoint(endpoint), otlptracegrpc.WithInsecure())
	if err != nil {
		slog.Error("Otlptracegrpc Err")
		return
	}
	tp := trace.NewTracerProvider(trace.WithBatcher(exporter), trace.WithResource(resource.NewWithAttributes(semconv.SchemaURL, semconv.ServiceName(serviceName))))
	otel.SetTracerProvider(tp)
	return tp.Shutdown
}
