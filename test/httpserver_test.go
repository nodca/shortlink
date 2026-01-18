package test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"day.local/internal/platform/config"
	"day.local/internal/platform/httpserver"
)

func TestHTTPServerNew_UsesConfigAndHandler(t *testing.T) {
	cfg := config.Config{
		Addr:              "127.0.0.1:0",
		ReadHeaderTimeout: 2 * time.Second,
		ReadTimeout:       3 * time.Second,
		WriteTimeout:      4 * time.Second,
		IdleTimeout:       5 * time.Second,
	}
	handler := http.NewServeMux()

	srv := httpserver.New(cfg, handler)

	if srv.Addr != cfg.Addr {
		t.Fatalf("Addr: got %q, want %q", srv.Addr, cfg.Addr)
	}
	if srv.Handler != handler {
		t.Fatalf("Handler: got %T, want %T", srv.Handler, handler)
	}
	if srv.ReadHeaderTimeout != cfg.ReadHeaderTimeout {
		t.Fatalf("ReadHeaderTimeout: got %v, want %v", srv.ReadHeaderTimeout, cfg.ReadHeaderTimeout)
	}
	if srv.ReadTimeout != cfg.ReadTimeout {
		t.Fatalf("ReadTimeout: got %v, want %v", srv.ReadTimeout, cfg.ReadTimeout)
	}
	if srv.WriteTimeout != cfg.WriteTimeout {
		t.Fatalf("WriteTimeout: got %v, want %v", srv.WriteTimeout, cfg.WriteTimeout)
	}
	if srv.IdleTimeout != cfg.IdleTimeout {
		t.Fatalf("IdleTimeout: got %v, want %v", srv.IdleTimeout, cfg.IdleTimeout)
	}
}

func TestRunWithGracefulShutdownContext_CancelStopsServer(t *testing.T) {
	cfg := config.Config{
		Addr:              "127.0.0.1:0",
		ReadHeaderTimeout: 500 * time.Millisecond,
		ReadTimeout:       500 * time.Millisecond,
		WriteTimeout:      500 * time.Millisecond,
		IdleTimeout:       500 * time.Millisecond,
	}
	srv := httpserver.New(cfg, http.NewServeMux())

	stopCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- httpserver.RunWithGracefulShutdownContext(srv, 500*time.Millisecond, stopCtx)
	}()

	time.Sleep(100 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for shutdown")
	}
}
