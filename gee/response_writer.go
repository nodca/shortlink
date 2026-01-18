package gee

import "net/http"

type ResponseWriter struct {
	http.ResponseWriter
	statusCode  int
	size        int
	wroteHeader bool
}

func NewResponseWriter(w http.ResponseWriter) *ResponseWriter {
	return &ResponseWriter{
		ResponseWriter: w,
		statusCode:     200,
	}
}

func (rw *ResponseWriter) WriteHeader(code int) {
	if rw.wroteHeader {
		return
	}
	rw.statusCode = code
	rw.wroteHeader = true
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *ResponseWriter) Write(bytes []byte) (int, error) {
	if !rw.wroteHeader {
		rw.WriteHeader(http.StatusOK)
	}
	s, err := rw.ResponseWriter.Write(bytes)
	rw.size += s
	return s, err
}

func (rw *ResponseWriter) SetHeader(key string, value string) {
	rw.ResponseWriter.Header().Set(key, value)
}

func (rw *ResponseWriter) Status() int {
	return rw.statusCode
}

func (rw *ResponseWriter) Size() int {
	return rw.size
}

func (rw *ResponseWriter) Written() bool {
	return rw.wroteHeader
}
