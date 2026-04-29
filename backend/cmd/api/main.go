package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"corp-messenger/backend/internal/app"
)

func main() {
	application, err := app.New()
	if err != nil {
		slog.Error("application initialization failed", "error", err)
		os.Exit(1)
	}
	defer application.Close()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		if application.Config.TLSEnabled {
			application.Logger.Info("https server starting", "env", application.Config.AppEnv, "port", application.Config.HTTPSPort)
			if err := application.Server.ListenAndServeTLS(application.Config.TLSCertPath, application.Config.TLSKeyPath); err != nil && !errors.Is(err, http.ErrServerClosed) {
				application.Logger.Error("https server failed", "error", err)
				stop()
			}
		} else {
			application.Logger.Info("http server starting", "env", application.Config.AppEnv, "port", application.Config.HTTPPort)
			if err := application.Server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				application.Logger.Error("http server failed", "error", err)
				stop()
			}
		}
	}()

	<-ctx.Done()
	application.Logger.Info("shutdown signal received, initiating graceful shutdown")
	// Close() handles server shutdown, including closing all goroutines
	application.Close()
	application.Logger.Info("application shutdown complete")
}
