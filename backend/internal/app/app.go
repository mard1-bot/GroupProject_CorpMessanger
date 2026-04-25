package app

import (
	"fmt"
	"net/http"
	"time"

	"corp-messenger/backend/internal/config"
	"corp-messenger/backend/internal/ejabberd"
	apphttp "corp-messenger/backend/internal/http"
	"corp-messenger/backend/internal/logger"
	"corp-messenger/backend/internal/notifications"
	"corp-messenger/backend/internal/storage"
	"corp-messenger/backend/internal/websocket"
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

	// Initialize XMPP client (ejabberd) if configured
	var ejb ejabberd.Client
	if cfg.EjabberdHost != "" && cfg.EjabberdAPISecret != "" {
		ejb = ejabberd.NewXMPPClient(cfg.EjabberdHost, cfg.EjabberdPort, cfg.EjabberdAPISecret)
		logg.Info("initialized XMPP client", "host", cfg.EjabberdHost, "port", cfg.EjabberdPort)
	} else {
		ejb = ejabberd.NewStub()
		logg.Warn("using stub XMPP client, no ejabberd configuration")
	}

	jwtSecret := cfg.JWTSecret
	if jwtSecret == "" {
		return nil, fmt.Errorf("JWT_SECRET is required")
	}
	// Validate minimum secret length for HS256 (32 bytes = 256 bits recommended)
	if len(jwtSecret) < 32 {
		return nil, fmt.Errorf("JWT_SECRET must be at least 32 characters long for security (HS256)")
	}

	// Create WebSocket hub
	hub := websocket.NewHub(stg)
	go hub.Run()

	// Initialize notification service
	notificationSvc, err := notifications.NewService(logg.Handler(), stg)
	if err != nil {
		logg.Error("failed to initialize notification service", "error", err)
		// Continue without notifications - not critical
		notificationSvc = nil
	}

	handler := apphttp.NewHandler(logg.Handler(), stg, ejb, jwtSecret, cfg.CORSOrigins, time.Duration(cfg.SessionDurationHours)*time.Hour, hub, notificationSvc, cfg.BaseURL, cfg.RateLimitRequests, cfg.RateLimitWindow, cfg.MaxRateLimitEntries)

	// Start rate limiter cleanup goroutine to prevent memory exhaustion
	apphttp.StartRateLimitCleanup()

	// Start call cleanup goroutine to remove abandoned calls
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			hub.CallManager().CleanupAbandonedCalls(2 * time.Minute)
		}
	}()

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
	apphttp.StopRateLimitCleanup()
	_ = a.Storage.Close()
	_ = a.Ejabberd.Close()
}
