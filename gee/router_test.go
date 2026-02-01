package gee

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// C4: 测试 404 Not Found
func TestNotFound(t *testing.T) {
	engine := New()
	engine.GET("/exists", func(ctx *Context) {
		ctx.String(200, "ok")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/not-exists", nil)
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, w.Code)
	}
}

// C4: 测试自定义 NoRoute handler
func TestCustomNoRoute(t *testing.T) {
	engine := New()
	engine.NoRoute(func(ctx *Context) {
		ctx.JSON(http.StatusNotFound, H{"error": "page not found"})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/not-exists", nil)
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, w.Code)
	}
	if !strings.Contains(w.Body.String(), "page not found") {
		t.Errorf("expected custom error message, got: %s", w.Body.String())
	}
}

// C4: 测试 405 Method Not Allowed
func TestMethodNotAllowed(t *testing.T) {
	engine := New()
	engine.GET("/test", func(ctx *Context) {
		ctx.String(200, "ok")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/test", nil)
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status %d, got %d", http.StatusMethodNotAllowed, w.Code)
	}
}

// C4: 测试 405 返回 Allow header
func TestMethodNotAllowedWithAllowHeader(t *testing.T) {
	engine := New()
	engine.GET("/test", func(ctx *Context) {
		ctx.String(200, "ok")
	})
	engine.POST("/test", func(ctx *Context) {
		ctx.String(200, "ok")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("DELETE", "/test", nil)
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status %d, got %d", http.StatusMethodNotAllowed, w.Code)
	}

	allow := w.Header().Get("Allow")
	if allow == "" {
		t.Error("expected Allow header to be set")
	}
	if !strings.Contains(allow, "GET") || !strings.Contains(allow, "POST") {
		t.Errorf("expected Allow header to contain GET and POST, got: %s", allow)
	}
}

// C4: 测试自定义 NoMethod handler
func TestCustomNoMethod(t *testing.T) {
	engine := New()
	engine.NoMethod(func(ctx *Context) {
		ctx.JSON(http.StatusMethodNotAllowed, H{"error": "method not allowed"})
	})
	engine.GET("/test", func(ctx *Context) {
		ctx.String(200, "ok")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/test", nil)
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status %d, got %d", http.StatusMethodNotAllowed, w.Code)
	}
	if !strings.Contains(w.Body.String(), "method not allowed") {
		t.Errorf("expected custom error message, got: %s", w.Body.String())
	}
}

// C4: 测试 404/405 也走 middleware
func TestNotFoundGoThroughMiddleware(t *testing.T) {
	middlewareExecuted := false

	engine := New()
	engine.Use(func(ctx *Context) {
		middlewareExecuted = true
		ctx.Next()
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/not-exists", nil)
	engine.ServeHTTP(w, req)

	if !middlewareExecuted {
		t.Error("middleware should be executed for 404")
	}
}

// C4: 测试 405 也走 middleware
func TestMethodNotAllowedGoThroughMiddleware(t *testing.T) {
	middlewareExecuted := false

	engine := New()
	engine.Use(func(ctx *Context) {
		middlewareExecuted = true
		ctx.Next()
	})
	engine.GET("/test", func(ctx *Context) {
		ctx.String(200, "ok")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/test", nil)
	engine.ServeHTTP(w, req)

	if !middlewareExecuted {
		t.Error("middleware should be executed for 405")
	}
}
