package app

import (
	"fmt"
	"net/http"
	"time"

	"corp-messenger/backend/internal/config"
	"corp-messenger/backend/internal/ejabberd"
	apphttp "corp-messenger/backend/internal/http"
	"corp-messenger/backend/internal/logger"
	"corp-messenger/backend/internal/storage"
)

type App struct {
	Config   config.Config
	Logger   *logger.Logger
	Server   *http.Server
	Storage  storage.Storage
	Ejabberd ejabberd.Client
}

func New() (*App, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}
	logg, err := logger.New(cfg.AppEnv, cfg.LogLevel)
	if err != nil {
		return nil, fmt.Errorf("init logger: %w", err)
	}

	var stg storage.Storage
	if cfg.DBDSN != "" {
		stg, err = storage.NewPostgres(cfg.DBDSN)
		if err != nil {
			return nil, fmt.Errorf("init postgres: %w", err)
		}
		logg.Info("connected to postgres", "dsn", cfg.DBDSN)
	} else {
		stg = storage.NewStub()
		logg.Warn("using stub storage, no database connection")
	}

	ejb := ejabberd.NewStub()

	jwtSecret := cfg.JWTSecret
	if jwtSecret == "" {
		return nil, fmt.Errorf("JWT_SECRET is required")
	}

	handler := apphttp.NewHandler(logg.Handler(), stg, ejb, jwtSecret, cfg.CORSOrigins)
	server := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.HTTPPort),
		Handler:           handler,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      30 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		MaxHeaderBytes:    1 << 20, // 1 MB
	}
	return &App{Config: cfg, Logger: logg, Server: server, Storage: stg, Ejabberd: ejb}, nil
}

func (a *App) Close() {
	_ = a.Storage.Close()
	_ = a.Ejabberd.Close()
}
