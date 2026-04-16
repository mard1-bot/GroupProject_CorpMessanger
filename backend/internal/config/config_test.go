package config

import (
	"testing"
)

func TestLoadSuccess(t *testing.T) {
	t.Setenv("APP_ENV", "local")
	t.Setenv("HTTP_PORT", "8080")
	t.Setenv("LOG_LEVEL", "info")
	t.Setenv("JWT_SECRET", "test-secret")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if cfg.HTTPPort != 8080 {
		t.Fatalf("expected port 8080, got %d", cfg.HTTPPort)
	}
}
func TestLoadMissingRequired(t *testing.T) {
	t.Setenv("HTTP_PORT", "8080")
	t.Setenv("LOG_LEVEL", "info")
	if _, err := Load(); err == nil {
		t.Fatal("expected error, got nil")
	}
}
