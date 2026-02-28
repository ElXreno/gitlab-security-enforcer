package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ElXreno/gitlab-security-enforcer/internal/config"
	"github.com/ElXreno/gitlab-security-enforcer/internal/gitlab"
	"github.com/ElXreno/gitlab-security-enforcer/internal/handler"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{}))

	cfg, err := config.Load()
	if err != nil {
		logger.Error("failed to load configuration", "error", err)
		os.Exit(1)
	}

	client, err := gitlab.New(cfg.GitLabURL, cfg.GitLabToken, logger)
	if err != nil {
		logger.Error("failed to initialize gitlab client", "error", err)
		os.Exit(1)
	}

	app := handler.NewApp(cfg.HookSecret, client, logger)

	shutdownDone := make(chan struct{})
	go func() {
		defer close(shutdownDone)
		waitForShutdown(app, logger)
	}()

	logger.Info("starting server", "listen_addr", cfg.ListenAddr)
	if err := app.Listen(cfg.ListenAddr); err != nil {
		logger.Error("server stopped with error", "error", err)
		os.Exit(1)
	}

	<-shutdownDone
	logger.Info("server stopped")
}

func waitForShutdown(app interface {
	ShutdownWithContext(ctx context.Context) error
}, logger *slog.Logger) {
	signalCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	<-signalCtx.Done()
	logger.Info("shutdown signal received")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := app.ShutdownWithContext(ctx); err != nil {
		logger.Error("graceful shutdown failed", "error", err)
	}
}
