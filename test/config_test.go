package test

import (
	"testing"
	"time"

	"day.local/internal/platform/config"
)

func TestConfigLoad_UsesDefaults(t *testing.T) {
	t.Setenv("ADDR", "")
	t.Setenv("IDLE_TIMEOUT", "")
	t.Setenv("SHUTDOWN_TIMEOUT", "")
	t.Setenv("READ_HEADER_TIMEOUT", "")
	t.Setenv("READ_TIMEOUT", "")
	t.Setenv("WRITE_TIMEOUT", "")

	cfg := config.Load()

	if cfg.Addr != ":9999" {
		t.Fatalf("Addr: got %q, want %q", cfg.Addr, ":9999")
	}
	if cfg.IdleTimeout != 60*time.Second {
		t.Fatalf("IdleTimeout: got %v, want %v", cfg.IdleTimeout, 60*time.Second)
	}
	if cfg.ShutdownTimeout != 10*time.Second {
		t.Fatalf("ShutdownTimeout: got %v, want %v", cfg.ShutdownTimeout, 10*time.Second)
	}
	if cfg.ReadHeaderTimeout != 5*time.Second {
		t.Fatalf("ReadHeaderTimeout: got %v, want %v", cfg.ReadHeaderTimeout, 5*time.Second)
	}
	if cfg.ReadTimeout != 10*time.Second {
		t.Fatalf("ReadTimeout: got %v, want %v", cfg.ReadTimeout, 10*time.Second)
	}
	if cfg.WriteTimeout != 10*time.Second {
		t.Fatalf("WriteTimeout: got %v, want %v", cfg.WriteTimeout, 10*time.Second)
	}
}

func TestConfigLoad_ReadsEnv(t *testing.T) {
	t.Setenv("ADDR", ":18080")
	t.Setenv("IDLE_TIMEOUT", "2m")
	t.Setenv("SHUTDOWN_TIMEOUT", "3s")
	t.Setenv("READ_HEADER_TIMEOUT", "4s")
	t.Setenv("READ_TIMEOUT", "5s")
	t.Setenv("WRITE_TIMEOUT", "6s")

	cfg := config.Load()

	if cfg.Addr != ":18080" {
		t.Fatalf("Addr: got %q, want %q", cfg.Addr, ":18080")
	}
	if cfg.IdleTimeout != 2*time.Minute {
		t.Fatalf("IdleTimeout: got %v, want %v", cfg.IdleTimeout, 2*time.Minute)
	}
	if cfg.ShutdownTimeout != 3*time.Second {
		t.Fatalf("ShutdownTimeout: got %v, want %v", cfg.ShutdownTimeout, 3*time.Second)
	}
	if cfg.ReadHeaderTimeout != 4*time.Second {
		t.Fatalf("ReadHeaderTimeout: got %v, want %v", cfg.ReadHeaderTimeout, 4*time.Second)
	}
	if cfg.ReadTimeout != 5*time.Second {
		t.Fatalf("ReadTimeout: got %v, want %v", cfg.ReadTimeout, 5*time.Second)
	}
	if cfg.WriteTimeout != 6*time.Second {
		t.Fatalf("WriteTimeout: got %v, want %v", cfg.WriteTimeout, 6*time.Second)
	}
}
