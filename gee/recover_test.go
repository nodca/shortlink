package gee

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// C3: 测试 Recovery 捕获 panic 并返回 500
func TestRecoveryReturnsFiveHundred(t *testing.T) {
	engine := New()
	engine.Use(Recovery())
	engine.GET("/panic", func(ctx *Context) {
		panic("test panic")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/panic", nil)
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, w.Code)
	}
}

// C3: 测试 panic 后后续 handler 不执行
func TestRecoveryStopsHandlerChain(t *testing.T) {
	executed := make([]int, 0)

	engine := New()
	engine.Use(Recovery())
	engine.Use(func(ctx *Context) {
		executed = append(executed, 1)
		ctx.Next()
		// panic 后 Abort 被调用，但由于 panic 中断了 Next() 的正常返回，
		// 这里的代码不会执行
	})
	engine.GET("/panic", func(ctx *Context) {
		executed = append(executed, 2)
		panic("test panic")
		// 下面不会执行
	}, func(ctx *Context) {
		executed = append(executed, 3) // 不应执行
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/panic", nil)
	engine.ServeHTTP(w, req)

	// 应该执行: 1, 2（panic 后 handler3 不执行）
	if len(executed) != 2 {
		t.Errorf("expected 2 handlers executed, got %d: %v", len(executed), executed)
	}
	// 验证 handler3 没有执行
	for _, v := range executed {
		if v == 3 {
			t.Error("handler after panic should not execute")
		}
	}
}

// C3: 测试 Default() 中 Recovery 在 Logger 之前
func TestDefaultMiddlewareOrder(t *testing.T) {
	engine := Default()

	// 在 Logger 中 panic，Recovery 应该能捕获
	engine.Use(func(ctx *Context) {
		panic("panic in middleware")
	})
	engine.GET("/test", func(ctx *Context) {
		ctx.String(200, "ok")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)

	// 不应 panic
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Recovery should catch panic, but got: %v", r)
		}
	}()

	engine.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, w.Code)
	}
}

// C3: 测试 Recovery 响应体包含错误信息
func TestRecoveryResponseBody(t *testing.T) {
	engine := New()
	engine.Use(Recovery())
	engine.GET("/panic", func(ctx *Context) {
		panic("test panic")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/panic", nil)
	engine.ServeHTTP(w, req)

	body := w.Body.String()
	if !strings.Contains(body, "Internal Server Error") {
		t.Errorf("expected body to contain 'Internal Server Error', got: %s", body)
	}
}

// C3: 测试 Recovery 在响应已写入时不再写入
func TestRecoveryWhenResponseAlreadyWritten(t *testing.T) {
	engine := New()
	engine.Use(Recovery())
	engine.GET("/panic", func(ctx *Context) {
		ctx.String(200, "partial")
		panic("test panic after write")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/panic", nil)
	engine.ServeHTTP(w, req)

	// 状态码应该是最初写入的 200，而不是 500
	if w.Code != 200 {
		t.Errorf("expected status 200 (already written), got %d", w.Code)
	}
}
