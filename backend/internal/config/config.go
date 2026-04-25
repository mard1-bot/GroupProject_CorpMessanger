package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	AppEnv               string
	HTTPPort             int
	LogLevel             string
	DBDSN                string
	JWTSecret            string
	EjabberdHost         string
	EjabberdPort         int
	EjabberdAPISecret    string
	StorageEndpoint      string
	CORSOrigins          []string
	SessionDurationHours int
	BaseURL              string
	RateLimitRequests    int
	RateLimitWindow      int
	MaxRateLimitEntries  int
}

func Load() (Config, error) {
	_ = godotenv.Load()
	cfg := Config{}
	var err error
	if cfg.AppEnv, err = required("APP_ENV"); err != nil {
		return Config{}, err
	}
	if cfg.HTTPPort, err = requiredInt("HTTP_PORT"); err != nil {
		return Config{}, err
	}
	if cfg.LogLevel, err = required("LOG_LEVEL"); err != nil {
		return Config{}, err
	}
	cfg.DBDSN = optional("DB_DSN")
	if cfg.JWTSecret, err = required("JWT_SECRET"); err != nil {
		return Config{}, err
	}
	cfg.EjabberdHost = optional("EJABBERD_HOST")
	cfg.EjabberdAPISecret = optional("EJABBERD_API_SECRET")
	cfg.StorageEndpoint = optional("STORAGE_ENDPOINT")
	cfg.BaseURL = optional("BASE_URL")
	if cfg.BaseURL == "" {
		cfg.BaseURL = "http://localhost:8080" // Default for local development
	}
	if value := optional("EJABBERD_PORT"); value != "" {
		port, convErr := strconv.Atoi(value)
		if convErr != nil {
			return Config{}, fmt.Errorf("env EJABBERD_PORT must be a valid integer")
		}
		cfg.EjabberdPort = port
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	// Parse CORS origins from comma-separated env var
	if origins := optional("CORS_ORIGINS"); origins != "" {
		cfg.CORSOrigins = strings.Split(origins, ",")
	} else {
		// Default to localhost origins for development
		cfg.CORSOrigins = []string{
			"http://localhost:3000",
			"http://localhost:19006",
			"http://localhost:8081",
		}
	}
	// Parse session duration (default 168 hours = 7 days)
	if hours := optional("SESSION_DURATION_HOURS"); hours != "" {
		h, err := strconv.Atoi(hours)
		if err != nil || h < 1 || h > 720 {
			return Config{}, fmt.Errorf("env SESSION_DURATION_HOURS must be between 1 and 720")
		}
		cfg.SessionDurationHours = h
	} else {
		cfg.SessionDurationHours = 168 // Default 7 days
	}

	// Parse rate limit configuration
	if requests := optional("RATE_LIMIT_REQUESTS"); requests != "" {
		r, err := strconv.Atoi(requests)
		if err != nil || r < 1 || r > 1000 {
			return Config{}, fmt.Errorf("env RATE_LIMIT_REQUESTS must be between 1 and 1000")
		}
		cfg.RateLimitRequests = r
	} else {
		cfg.RateLimitRequests = 20 // Default 20 requests
	}

	if window := optional("RATE_LIMIT_WINDOW"); window != "" {
		w, err := strconv.Atoi(window)
		if err != nil || w < 1 || w > 3600 {
			return Config{}, fmt.Errorf("env RATE_LIMIT_WINDOW must be between 1 and 3600 seconds")
		}
		cfg.RateLimitWindow = w
	} else {
		cfg.RateLimitWindow = 60 // Default 60 seconds (1 minute)
	}

	if entries := optional("MAX_RATE_LIMIT_ENTRIES"); entries != "" {
		e, err := strconv.Atoi(entries)
		if err != nil || e < 100 || e > 100000 {
			return Config{}, fmt.Errorf("env MAX_RATE_LIMIT_ENTRIES must be between 100 and 100000")
		}
		cfg.MaxRateLimitEntries = e
	} else {
		cfg.MaxRateLimitEntries = 10000 // Default 10000 entries
	}

	return cfg, nil
}

func (c Config) Validate() error {
	if !contains([]string{"local", "dev", "prod"}, c.AppEnv) {
		return fmt.Errorf("env APP_ENV must be one of: local, dev, prod")
	}
	if c.HTTPPort < 1 || c.HTTPPort > 65535 {
		return fmt.Errorf("env HTTP_PORT must be between 1 and 65535")
	}
	if !contains([]string{"debug", "info", "warn", "error"}, c.LogLevel) {
		return fmt.Errorf("env LOG_LEVEL must be one of: debug, info, warn, error")
	}
	if c.EjabberdPort < 0 || c.EjabberdPort > 65535 {
		return fmt.Errorf("env EJABBERD_PORT must be between 1 and 65535 when provided")
	}
	return nil
}
func required(key string) (string, error) {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return "", fmt.Errorf("missing required env %s", key)
	}
	return v, nil
}
func requiredInt(key string) (int, error) {
	v, err := required(key)
	if err != nil {
		return 0, err
	}
	n, conv := strconv.Atoi(v)
	if conv != nil {
		return 0, errors.New("env " + key + " must be a valid integer")
	}
	return n, nil
}
func optional(key string) string { return strings.TrimSpace(os.Getenv(key)) }
func contains(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}
