package test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"day.local/gee"
	"day.local/gee/middleware"
)

func TestRequestID_PreservesIncoming(t *testing.T) {
	r := gee.New()
	r.Use(middleware.ReqID())
	r.GET("/id", func(ctx *gee.Context) {
		ctx.String(http.StatusOK, "%s", ctx.Req.Header.Get("X-Request-ID"))
	})

	req := httptest.NewRequest(http.MethodGet, "/id", nil)
	req.Header.Set("X-Request-ID", "abc")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if got := rec.Header().Get("X-Request-ID"); got != "abc" {
		t.Fatalf("response X-Request-ID: got %q, want %q", got, "abc")
	}
	if got := strings.TrimSpace(rec.Body.String()); got != "abc" {
		t.Fatalf("body: got %q, want %q", got, "abc")
	}
}

func TestRequestID_GeneratesWhenMissing(t *testing.T) {
	r := gee.New()
	r.Use(middleware.ReqID())
	r.GET("/id", func(ctx *gee.Context) {
		ctx.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/id", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if got := rec.Header().Get("X-Request-ID"); got == "" {
		t.Fatal("response X-Request-ID is empty")
	}
}

func TestAccessLog_EmitsJSONFields(t *testing.T) {
	var buf bytes.Buffer
	old := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(old) })

	r := gee.New()
	r.Use(gee.Recovery(), middleware.ReqID(), middleware.AccessLog())
	r.GET("/get", func(ctx *gee.Context) {
		ctx.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/get", nil)
	req.Header.Set("X-Request-ID", "abc")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	dec := json.NewDecoder(&buf)
	for {
		var m map[string]any
		if err := dec.Decode(&m); err != nil {
			break
		}
		if m["msg"] != "access" {
			continue
		}
		if m["request_id"] != "abc" {
			t.Fatalf("request_id: got %v, want %q", m["request_id"], "abc")
		}
		if m["method"] != http.MethodGet {
			t.Fatalf("method: got %v, want %q", m["method"], http.MethodGet)
		}
		if m["path"] != "/get" {
			t.Fatalf("path: got %v, want %q", m["path"], "/get")
		}
		return
	}
	t.Fatalf("did not find access log entry\nraw=%q", buf.String())
}

func TestRecovery_Returns500AndLogs(t *testing.T) {
	var buf bytes.Buffer
	old := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(old) })

	r := gee.New()
	r.Use(gee.Recovery(), middleware.ReqID())
	r.GET("/panic", func(ctx *gee.Context) {
		panic("boom")
	})

	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	req.Header.Set("X-Request-ID", "abc")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status: got %d, want %d", rec.Code, http.StatusInternalServerError)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "application/json") {
		t.Fatalf("Content-Type: got %q, want contains %q", ct, "application/json")
	}
	if !strings.Contains(buf.String(), `"request_id":"abc"`) {
		t.Fatalf("log does not contain request_id: raw=%q", buf.String())
	}
}
