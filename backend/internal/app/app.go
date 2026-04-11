package app

import (
	"fmt"
	"net/http"
	"time"

	"go-backend-scaffold/internal/config"
	"go-backend-scaffold/internal/ejabberd"
	apphttp "go-backend-scaffold/internal/http"
	"go-backend-scaffold/internal/logger"
	"go-backend-scaffold/internal/storage"
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
	stg := storage.NewStub()
	ejb := ejabberd.NewStub()
	handler := apphttp.NewHandler(logg.Handler(), stg, ejb)
	server := &http.Server{Addr: fmt.Sprintf(":%d", cfg.HTTPPort), Handler: handler, ReadHeaderTimeout: 5 * time.Second}
	return &App{Config: cfg, Logger: logg, Server: server, Storage: stg, Ejabberd: ejb}, nil
}

func (a *App) Close() {
	_ = a.Storage.Close()
	_ = a.Ejabberd.Close()
}
