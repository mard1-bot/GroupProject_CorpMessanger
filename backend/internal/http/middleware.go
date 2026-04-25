package http

import (
	"bufio"
	"fmt"
	"log/slog"
	"net"
	stdhttp "net/http"
	"runtime/debug"
	"time"
)

type responseWriter struct {
	stdhttp.ResponseWriter
	status  int
	written bool
}

func (rw *responseWriter) WriteHeader(status int) {
	rw.status = status
	rw.ResponseWriter.WriteHeader(status)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	if !rw.written {
		rw.written = true
		if rw.status == 0 {
			rw.status = stdhttp.StatusOK
		}
	}
	return rw.ResponseWriter.Write(b)
}

// Hijack implements http.Hijacker for WebSocket support
func (rw *responseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hijacker, ok := rw.ResponseWriter.(stdhttp.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf("response writer does not implement http.Hijacker")
	}
	return hijacker.Hijack()
}
func AccessLogMiddleware(logger *slog.Logger) func(stdhttp.Handler) stdhttp.Handler {
	return func(next stdhttp.Handler) stdhttp.Handler {
		return stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
			startedAt := time.Now()
			rw := &responseWriter{ResponseWriter: w, status: stdhttp.StatusOK}
			next.ServeHTTP(rw, r)
			attrs := []any{"method", r.Method, "path", r.URL.Path, "status", rw.status, "duration", time.Since(startedAt).String()}
			switch {
			case rw.status >= 500:
				logger.Error("request completed", attrs...)
			case rw.status >= 400:
				logger.Warn("request completed", attrs...)
			default:
				logger.Info("request completed", attrs...)
			}
		})
	}
}
func RecoverMiddleware(logger *slog.Logger) func(stdhttp.Handler) stdhttp.Handler {
	return func(next stdhttp.Handler) stdhttp.Handler {
		return stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					logger.Error("panic recovered", "method", r.Method, "path", r.URL.Path, "panic", fmt.Sprint(rec), "stack", string(debug.Stack()))
					WriteError(w, stdhttp.StatusInternalServerError, "internal", "internal server error")
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}
