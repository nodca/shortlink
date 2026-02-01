package gee

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// C1: 测试 Abort 功能
func TestAbort(t *testing.T) {
	c := &Context{index: -1}
	if c.IsAborted() {
		t.Error("new context should not be aborted")
	}

	c.Abort()
	if !c.IsAborted() {
		t.Error("context should be aborted after Abort()")
	}
}

// C1: 测试 Abort 后 Next 不再执行后续 handler
func TestAbortStopsHandlerChain(t *testing.T) {
	executed := make([]int, 0)

	handler1 := func(c *Context) {
		executed = append(executed, 1)
		c.Next()
	}
	handler2 := func(c *Context) {
		executed = append(executed, 2)
		c.Abort()
		c.Next() // 即使调用 Next，也不应继续
	}
	handler3 := func(c *Context) {
		executed = append(executed, 3) // 不应执行
	}

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	c := newContext(w, req)
	c.handlers = []HandlerFunc{handler1, handler2, handler3}

	c.Next()

	if len(executed) != 2 {
		t.Errorf("expected 2 handlers executed, got %d: %v", len(executed), executed)
	}
	if executed[0] != 1 || executed[1] != 2 {
		t.Errorf("expected [1, 2], got %v", executed)
	}
}

// C1: 测试 Fail 后下游 handler 不执行
func TestFailStopsHandlerChain(t *testing.T) {
	executed := make([]int, 0)

	handler1 := func(c *Context) {
		executed = append(executed, 1)
		c.Next()
	}
	handler2 := func(c *Context) {
		executed = append(executed, 2)
		c.Fail(http.StatusBadRequest, "bad request")
	}
	handler3 := func(c *Context) {
		executed = append(executed, 3) // 不应执行
	}

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	c := newContext(w, req)
	c.handlers = []HandlerFunc{handler1, handler2, handler3}

	c.Next()

	if len(executed) != 2 {
		t.Errorf("expected 2 handlers executed, got %d: %v", len(executed), executed)
	}
	if !c.IsAborted() {
		t.Error("context should be aborted after Fail()")
	}
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

// C1: 测试 AbortWithStatus
func TestAbortWithStatus(t *testing.T) {
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	c := newContext(w, req)

	c.AbortWithStatus(http.StatusForbidden)

	if !c.IsAborted() {
		t.Error("context should be aborted")
	}
	if w.Code != http.StatusForbidden {
		t.Errorf("expected status %d, got %d", http.StatusForbidden, w.Code)
	}
}

// C1: 测试 AbortWithStatusJSON
func TestAbortWithStatusJSON(t *testing.T) {
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	c := newContext(w, req)

	c.AbortWithStatusJSON(http.StatusUnauthorized, H{"error": "unauthorized"})

	if !c.IsAborted() {
		t.Error("context should be aborted")
	}
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
	if w.Header().Get("Content-Type") != "application/json" {
		t.Errorf("expected Content-Type application/json, got %s", w.Header().Get("Content-Type"))
	}
}

// C1: 测试中间件链正常执行顺序
func TestMiddlewareExecutionOrder(t *testing.T) {
	order := make([]string, 0)

	middleware1 := func(c *Context) {
		order = append(order, "m1-before")
		c.Next()
		order = append(order, "m1-after")
	}
	middleware2 := func(c *Context) {
		order = append(order, "m2-before")
		c.Next()
		order = append(order, "m2-after")
	}
	handler := func(c *Context) {
		order = append(order, "handler")
	}

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	c := newContext(w, req)
	c.handlers = []HandlerFunc{middleware1, middleware2, handler}

	c.Next()

	expected := []string{"m1-before", "m2-before", "handler", "m2-after", "m1-after"}
	if len(order) != len(expected) {
		t.Errorf("expected %v, got %v", expected, order)
		return
	}
	for i, v := range expected {
		if order[i] != v {
			t.Errorf("at index %d: expected %s, got %s", i, v, order[i])
		}
	}
}
