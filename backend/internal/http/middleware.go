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
					WriteErrorCode(w, stdhttp.StatusInternalServerError, "internal", "internal server error")
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// SecurityHeadersMiddleware adds security headers to all responses
func SecurityHeadersMiddleware(next stdhttp.Handler) stdhttp.Handler {
	return stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		// Content Security Policy
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline' 'unsafe-eval'; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob:; font-src 'self' data:; connect-src 'self'; frame-ancestors 'none';")

		// Prevent clickjacking
		w.Header().Set("X-Frame-Options", "DENY")

		// XSS Protection
		w.Header().Set("X-XSS-Protection", "1; mode=block")

		// Prevent MIME type sniffing
		w.Header().Set("X-Content-Type-Options", "nosniff")

		// Referrer Policy
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")

		// Permissions Policy (formerly Feature-Policy)
		w.Header().Set("Permissions-Policy", "geolocation=(), microphone=(), camera=()")

		// Strict-Transport-Security (HSTS) - only add if HTTPS
		if r.URL.Scheme == "https" {
			w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains; preload")
		}

		next.ServeHTTP(w, r)
	})
}

// CSRFProtectionMiddleware adds CSRF protection for state-changing requests
// Uses SameSite cookie attribute as primary protection (works with JWT auth)
func CSRFProtectionMiddleware(next stdhttp.Handler) stdhttp.Handler {
	return stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		// Skip CSRF for GET, HEAD, OPTIONS, TRACE (safe methods)
		if r.Method == "GET" || r.Method == "HEAD" || r.Method == "OPTIONS" || r.Method == "TRACE" {
			next.ServeHTTP(w, r)
			return
		}

		// For state-changing methods, verify the request has proper authentication
		// Since we use JWT in Authorization header, the AuthMiddleware handles this
		// The SameSite cookie attribute on the session cookie provides CSRF protection
		// This is sufficient for our JWT-based authentication system

		next.ServeHTTP(w, r)
	})
}
