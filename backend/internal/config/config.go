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
	HTTPSPort            int
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
	HybridMode           bool
	XMPPSyncEnabled      bool
	SchedulerEnabled     bool
	SMTPHost             string
	SMTPPort             int
	SMTPUser             string
	SMTPPassword         string
	SMTPFrom             string
	TURNServerURI        string
	TURNUsername         string
	TURNPassword         string
	TURNExternalIP       string
	TLSEnabled           bool
	TLSCertPath          string
	TLSKeyPath           string
	DBSSLMode            string
	LiveKitURL           string
	LiveKitPublicURL     string
	LiveKitAPIKey        string
	LiveKitAPISecret     string
	RedisURL             string
	SentryDSN            string
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
	// HTTPS_PORT is optional - only required when TLS is enabled
	if value := optional("HTTPS_PORT"); value != "" {
		port, convErr := strconv.Atoi(value)
		if convErr != nil {
			return Config{}, fmt.Errorf("env HTTPS_PORT must be a valid integer")
		}
		cfg.HTTPSPort = port
	} else {
		cfg.HTTPSPort = 8443 // Default HTTPS port
	}
	if cfg.LogLevel, err = required("LOG_LEVEL"); err != nil {
		return Config{}, err
	}
	cfg.DBDSN = optional("DATABASE_URL")
	if cfg.DBDSN == "" {
		cfg.DBDSN = optional("DB_DSN")
	}
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
	// Parse session duration (default 24 hours for better security)
	if h := optional("SESSION_DURATION_HOURS"); h != "" {
		hours, err := strconv.Atoi(h)
		if err != nil {
			return Config{}, fmt.Errorf("env SESSION_DURATION_HOURS must be an integer")
		}
		cfg.SessionDurationHours = hours
	} else {
		cfg.SessionDurationHours = 24 // Default 24 hours for better security
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

	// Parse hybrid mode configuration
	if hybrid := optional("HYBRID_MODE"); hybrid != "" {
		cfg.HybridMode = strings.ToLower(hybrid) == "true"
	} else {
		cfg.HybridMode = false // Default to WebSocket only
	}

	// Parse XMPP sync configuration
	if sync := optional("XMPP_SYNC_ENABLED"); sync != "" {
		cfg.XMPPSyncEnabled = strings.ToLower(sync) == "true"
	} else {
		cfg.XMPPSyncEnabled = cfg.HybridMode // Enable sync by default in hybrid mode
	}

	// Parse scheduler configuration
	if sched := optional("SCHEDULER_ENABLED"); sched != "" {
		cfg.SchedulerEnabled = strings.ToLower(sched) == "true"
	} else {
		cfg.SchedulerEnabled = true // Enable scheduler by default
	}

	// Parse SMTP configuration
	cfg.SMTPHost = optional("SMTP_HOST")
	if port := optional("SMTP_PORT"); port != "" {
		p, err := strconv.Atoi(port)
		if err != nil || p < 1 || p > 65535 {
			return Config{}, fmt.Errorf("env SMTP_PORT must be between 1 and 65535")
		}
		cfg.SMTPPort = p
	} else {
		cfg.SMTPPort = 587 // Default SMTP port
	}
	cfg.SMTPUser = optional("SMTP_USER")
	cfg.SMTPPassword = optional("SMTP_PASSWORD")
	cfg.SMTPFrom = optional("SMTP_FROM")

	// Parse TURN server configuration for WebRTC
	cfg.TURNServerURI = optional("TURN_SERVER_URI")
	cfg.TURNUsername = optional("TURN_USERNAME")
	cfg.TURNPassword = optional("TURN_PASSWORD")
	cfg.TURNExternalIP = optional("TURN_EXTERNAL_IP")

	// Parse TLS configuration
	if tlsEnabled := optional("TLS_ENABLED"); tlsEnabled != "" {
		cfg.TLSEnabled = strings.ToLower(tlsEnabled) == "true"
	} else {
		cfg.TLSEnabled = false // Default to HTTP
	}
	cfg.TLSCertPath = optional("TLS_CERT_PATH")
	cfg.TLSKeyPath = optional("TLS_KEY_PATH")

	// Parse DB SSL mode
	cfg.DBSSLMode = optional("DB_SSL_MODE")
	if cfg.DBSSLMode == "" {
		cfg.DBSSLMode = "disable" // Default to disable SSL for local development
	}

	// Parse LiveKit configuration
	cfg.LiveKitURL = optional("LIVEKIT_URL")
	cfg.LiveKitPublicURL = optional("LIVEKIT_PUBLIC_URL")
	if cfg.LiveKitPublicURL == "" {
		cfg.LiveKitPublicURL = cfg.LiveKitURL // Fallback to internal URL if public is not specified
	}
	cfg.LiveKitAPIKey = optional("LIVEKIT_API_KEY")
	cfg.LiveKitAPISecret = optional("LIVEKIT_API_SECRET")

	// Parse Redis URL (optional - rate limiting will work without Redis)
	cfg.RedisURL = optional("REDIS_URL")
	if cfg.RedisURL == "" {
		cfg.RedisURL = "redis:6379" // Default for local development
	}

	// Parse Sentry DSN (optional - error tracking)
	cfg.SentryDSN = optional("SENTRY_DSN")

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
	if c.EjabberdPort != 0 && (c.EjabberdPort < 1 || c.EjabberdPort > 65535) {
		return fmt.Errorf("env EJABBERD_PORT must be between 1 and 65535 when provided")
	}
	return nil
}

// secretValue reads a config value with Docker secrets support.
// It first checks the env var directly (KEY), then reads from the file path
// stored in the KEY_FILE env var (standard Docker secrets convention).
func secretValue(key string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	if filePath := strings.TrimSpace(os.Getenv(key + "_FILE")); filePath != "" {
		content, err := os.ReadFile(filePath)
		if err == nil {
			return strings.TrimSpace(string(content))
		}
	}
	return ""
}

func required(key string) (string, error) {
	v := secretValue(key)
	if v == "" {
		return "", fmt.Errorf("missing required env %s (or %s_FILE)", key, key)
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
func optional(key string) string { return secretValue(key) }
func contains(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}
