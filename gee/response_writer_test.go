package gee

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// C2: 测试 ResponseWriter 默认状态码为 200
func TestResponseWriterDefaultStatus(t *testing.T) {
	w := httptest.NewRecorder()
	rw := NewResponseWriter(w)

	if rw.Status() != 200 {
		t.Errorf("expected default status 200, got %d", rw.Status())
	}
}

// C2: 测试 ResponseWriter 记录状态码
func TestResponseWriterStatus(t *testing.T) {
	w := httptest.NewRecorder()
	rw := NewResponseWriter(w)

	rw.WriteHeader(201)

	if rw.Status() != 201 {
		t.Errorf("expected status 201, got %d", rw.Status())
	}
}

// C2: 测试 ResponseWriter 只写入一次 header
func TestResponseWriterWriteHeaderOnce(t *testing.T) {
	w := httptest.NewRecorder()
	rw := NewResponseWriter(w)

	rw.WriteHeader(201)
	rw.WriteHeader(500) // 第二次调用应被忽略

	if rw.Status() != 201 {
		t.Errorf("expected status 201, got %d", rw.Status())
	}
	if w.Code != 201 {
		t.Errorf("expected underlying status 201, got %d", w.Code)
	}
}

// C2: 测试 ResponseWriter 记录写入字节数
func TestResponseWriterSize(t *testing.T) {
	w := httptest.NewRecorder()
	rw := NewResponseWriter(w)

	rw.Write([]byte("hello"))
	rw.Write([]byte(" world"))

	if rw.Size() != 11 {
		t.Errorf("expected size 11, got %d", rw.Size())
	}
}

// C2: 测试 Write 时自动写入 200 状态码
func TestResponseWriterAutoWriteHeader(t *testing.T) {
	w := httptest.NewRecorder()
	rw := NewResponseWriter(w)

	rw.Write([]byte("hello"))

	if rw.Status() != 200 {
		t.Errorf("expected status 200, got %d", rw.Status())
	}
	if !rw.Written() {
		t.Error("expected Written() to be true")
	}
}

// C2: 测试 Written 方法
func TestResponseWriterWritten(t *testing.T) {
	w := httptest.NewRecorder()
	rw := NewResponseWriter(w)

	if rw.Written() {
		t.Error("expected Written() to be false before write")
	}

	rw.WriteHeader(200)

	if !rw.Written() {
		t.Error("expected Written() to be true after WriteHeader")
	}
}

// C2: 测试 Logger 记录正确的 status 和 size
func TestLoggerRecordsStatusAndSize(t *testing.T) {
	engine := New()

	var recordedStatus int
	var recordedSize int

	// 自定义 logger 来捕获值
	customLogger := func(ctx *Context) {
		ctx.Next()
		recordedStatus = ctx.Writer.Status()
		recordedSize = ctx.Writer.Size()
	}

	engine.Use(customLogger)
	engine.GET("/test", func(ctx *Context) {
		ctx.String(http.StatusCreated, "hello")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	engine.ServeHTTP(w, req)

	if recordedStatus != http.StatusCreated {
		t.Errorf("expected status %d, got %d", http.StatusCreated, recordedStatus)
	}
	if recordedSize != 5 {
		t.Errorf("expected size 5, got %d", recordedSize)
	}
}

// C2: 测试只 Write 不显式设置状态码时，Logger 记录 200
func TestLoggerRecords200WhenOnlyWrite(t *testing.T) {
	engine := New()

	var recordedStatus int

	customLogger := func(ctx *Context) {
		ctx.Next()
		recordedStatus = ctx.Writer.Status()
	}

	engine.Use(customLogger)
	engine.GET("/test", func(ctx *Context) {
		ctx.Writer.Write([]byte("hello"))
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	engine.ServeHTTP(w, req)

	if recordedStatus != 200 {
		t.Errorf("expected status 200, got %d", recordedStatus)
	}
}
