package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go-backend-scaffold/internal/app"
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
		application.Logger.Info("http server starting", "env", application.Config.AppEnv, "port", application.Config.HTTPPort)
		if err := application.Server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			application.Logger.Error("http server failed", "error", err)
			stop()
		}
	}()

	<-ctx.Done()
	application.Logger.Info("shutdown signal received")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := application.Server.Shutdown(shutdownCtx); err != nil {
		application.Logger.Error("graceful shutdown failed", "error", err)
		os.Exit(1)
	}
	application.Logger.Info("http server stopped")
}
