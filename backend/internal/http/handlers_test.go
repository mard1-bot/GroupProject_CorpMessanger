package http

import (
	"context"
	"log/slog"
	"net/http/httptest"
	"testing"
)

type testStorage struct{ err error }

func (s testStorage) Ready(context.Context) error { return s.err }
func (s testStorage) Close() error                { return nil }

type testEjabberd struct{ err error }

func (e testEjabberd) Ready(context.Context) error { return e.err }
func (e testEjabberd) Close() error                { return nil }
func TestHealth(t *testing.T) {
	h := NewHandler(slog.Default(), testStorage{}, testEjabberd{})
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/health", nil)
	h.ServeHTTP(rr, req)
	if rr.Code != 200 {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}
func TestReady(t *testing.T) {
	h := NewHandler(slog.Default(), testStorage{}, testEjabberd{})
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/ready", nil)
	h.ServeHTTP(rr, req)
	if rr.Code != 200 {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}
