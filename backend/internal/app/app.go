package app

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"corp-messenger/backend/internal/config"
	"corp-messenger/backend/internal/crypto"
	"corp-messenger/backend/internal/ejabberd"
	apphttp "corp-messenger/backend/internal/http"
	"corp-messenger/backend/internal/livekit"
	"corp-messenger/backend/internal/logger"
	"corp-messenger/backend/internal/notifications"
	"corp-messenger/backend/internal/scheduler"
	"corp-messenger/backend/internal/storage"
	"corp-messenger/backend/internal/websocket"
	"corp-messenger/backend/internal/xmppsync"
)

type App struct {
	Config           config.Config
	Logger           *logger.Logger
	Server           *http.Server
	Storage          storage.Storage
	Ejabberd         ejabberd.Client
	Hub              *websocket.Hub
	LiveKit          *livekit.Service
	SyncService      *xmppsync.SyncService
	ReconcileService *xmppsync.ReconciliationService
	Scheduler        *scheduler.Scheduler
	ctx              context.Context
	cancel           context.CancelFunc
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

	// Initialize encryption master key for envelope encryption
	// For production, ENCRYPTION_MASTER_KEY should be set. For development, it's optional.
	if err := crypto.InitMasterKey(); err != nil {
		if cfg.AppEnv == "prod" {
			return nil, fmt.Errorf("encryption master key is required in production: %w", err)
		}
		logg.Warn("encryption master key not initialized, encryption features will be disabled (acceptable for development)", "error", err)
	} else {
		logg.Info("encryption master key initialized successfully")
	}

	var stg storage.Storage
	if cfg.DBDSN != "" {
		// Update DB DSN with SSL mode if specified
		dbDSN := cfg.DBDSN
		if cfg.DBSSLMode != "" {
			// Use regex to replace any existing sslmode parameter
			sslmodeRegex := regexp.MustCompile(`sslmode=[^&]+`)
			if sslmodeRegex.MatchString(dbDSN) {
				dbDSN = sslmodeRegex.ReplaceAllString(dbDSN, "sslmode="+cfg.DBSSLMode)
			} else {
				// Add sslmode parameter
				if strings.Contains(dbDSN, "?") {
					dbDSN = dbDSN + "&sslmode=" + cfg.DBSSLMode
				} else {
					dbDSN = dbDSN + "?sslmode=" + cfg.DBSSLMode
				}
			}
		}
		stg, err = storage.NewPostgres(dbDSN)
		if err != nil {
			return nil, fmt.Errorf("init postgres: %w", err)
		}
		logg.Info("connected to postgres", "dsn", dbDSN)
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

	// Initialize LiveKit service if configured
	var lk *livekit.Service
	if cfg.LiveKitURL != "" && cfg.LiveKitAPIKey != "" && cfg.LiveKitAPISecret != "" {
		lk = livekit.NewService(cfg.LiveKitAPIKey, cfg.LiveKitAPISecret, cfg.LiveKitURL)
		logg.Info("initialized LiveKit service", "url", cfg.LiveKitURL)
	} else {
		lk = livekit.NewService("", "", "")
		logg.Warn("using stub LiveKit service, no LiveKit configuration")
	}

	jwtSecret := cfg.JWTSecret
	if jwtSecret == "" {
		return nil, fmt.Errorf("JWT_SECRET is required")
	}
	// Validate minimum secret length for HS256 (32 bytes = 256 bits recommended)
	if len(jwtSecret) < 32 {
		return nil, fmt.Errorf("JWT_SECRET must be at least 32 characters long for security (HS256)")
	}

	// Validate TLS configuration before starting goroutines
	if cfg.TLSEnabled {
		if cfg.TLSCertPath == "" || cfg.TLSKeyPath == "" {
			return nil, fmt.Errorf("TLS_CERT_PATH and TLS_KEY_PATH must be set when TLS_ENABLED=true")
		}
	}

	// Create context for goroutines to enable proper cleanup
	ctx, cancel := context.WithCancel(context.Background())

	// Create WebSocket hub
	hub := websocket.NewHub(stg, lk, cfg.LiveKitURL)
	go hub.Run(ctx)

	// Initialize notification service
	notificationSvc, err := notifications.NewService(logg.Handler(), stg)
	if err != nil {
		logg.Error("failed to initialize notification service", "error", err)
		// Continue without notifications - not critical
		notificationSvc = nil
	}

	// Start XMPP sync service if hybrid mode is enabled
	var syncService *xmppsync.SyncService
	var reconcileService *xmppsync.ReconciliationService
	if cfg.HybridMode && cfg.XMPPSyncEnabled {
		syncService = xmppsync.NewSyncService(stg, ejb, true, logg.Handler())
		go syncService.Start(ctx)
		logg.Info("XMPP sync service started in hybrid mode")

		// Start XMPP reconciliation service
		reconcileService = xmppsync.NewReconciliationService(stg, ejb, true, logg.Handler())
		go reconcileService.Start(ctx)
		logg.Info("XMPP reconciliation service started")
	}

	// Start scheduler for scheduled messages
	var sched *scheduler.Scheduler
	if cfg.SchedulerEnabled {
		sched = scheduler.NewScheduler(stg, hub, true, logg.Handler())
		go sched.Start(ctx)
		logg.Info("Scheduler started")
	}

	handler := apphttp.NewHandler(logg.Handler(), stg, ejb, jwtSecret, cfg.CORSOrigins, time.Duration(cfg.SessionDurationHours)*time.Hour, hub, notificationSvc, cfg.BaseURL, cfg.RateLimitRequests, cfg.RateLimitWindow, cfg.MaxRateLimitEntries, syncService, cfg.TURNServerURI, cfg.TURNUsername, cfg.TURNPassword, lk, cfg.RedisURL)

	// Start rate limiter cleanup goroutine to prevent memory exhaustion
	apphttp.StartRateLimitCleanup()

	// Start call cleanup goroutine to remove abandoned calls
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				hub.CallManager().CleanupAbandonedCalls(2 * time.Minute)
			case <-ctx.Done():
				return
			}
		}
	}()

	// Determine server address based on TLS configuration
	serverAddr := fmt.Sprintf(":%d", cfg.HTTPPort)
	if cfg.TLSEnabled {
		serverAddr = fmt.Sprintf(":%d", cfg.HTTPSPort)
	}

	server := &http.Server{
		Addr:              serverAddr,
		Handler:           handler,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      30 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		MaxHeaderBytes:    1 << 20, // 1 MB
		TLSConfig: &tls.Config{
			MinVersion: tls.VersionTLS13,
		},
	}

	// Configure TLS if enabled
	if cfg.TLSEnabled {
		logg.Info("TLS enabled", "cert", cfg.TLSCertPath, "key", cfg.TLSKeyPath, "port", cfg.HTTPSPort)
	}
	return &App{
		Config:           cfg,
		Logger:           logg,
		Server:           server,
		Storage:          stg,
		Ejabberd:         ejb,
		Hub:              hub,
		LiveKit:          lk,
		SyncService:      syncService,
		ReconcileService: reconcileService,
		Scheduler:        sched,
		ctx:              ctx,
		cancel:           cancel,
	}, nil
}

func (a *App) Close() {
	a.Logger.Info("Starting graceful shutdown")

	// Cancel all goroutines first
	if a.cancel != nil {
		a.cancel()
	}

	// Shutdown WebSocket hub
	if a.Hub != nil {
		a.Hub.Shutdown()
	}

	// Shutdown HTTP server with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := a.Server.Shutdown(ctx); err != nil {
		a.Logger.Error("Server shutdown error", "error", err)
	}

	// Stop rate limiter cleanup
	apphttp.StopRateLimitCleanup()

	// Close storage
	if err := a.Storage.Close(); err != nil {
		a.Logger.Error("Storage close error", "error", err)
	}

	// Close ejabberd connection
	if err := a.Ejabberd.Close(); err != nil {
		a.Logger.Error("Ejabberd close error", "error", err)
	}

	// Note: SyncService, ReconcileService, and Scheduler goroutines
	// are stopped via context cancellation. They don't have explicit Close methods.

	a.Logger.Info("Graceful shutdown completed")
}
