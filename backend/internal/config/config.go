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
	AppEnv            string
	HTTPPort          int
	LogLevel          string
	DBDSN             string
	JWTSecret         string
	EjabberdHost      string
	EjabberdPort      int
	EjabberdAPISecret string
	StorageEndpoint   string
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
	cfg.JWTSecret = optional("JWT_SECRET")
	cfg.EjabberdHost = optional("EJABBERD_HOST")
	cfg.EjabberdAPISecret = optional("EJABBERD_API_SECRET")
	cfg.StorageEndpoint = optional("STORAGE_ENDPOINT")
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
