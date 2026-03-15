package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sbekti/intern-api/internal/config"
	"github.com/sbekti/intern-api/internal/presence"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: cfg.LogLevel.Leveler(),
	}))

	if !cfg.Presence.Enabled {
		logger.Info("presence worker disabled")
		return
	}

	connectCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pgxConfig, err := pgxpool.ParseConfig(cfg.Database.URL)
	if err != nil {
		logger.Error("failed to parse database url", "error", err)
		os.Exit(1)
	}

	pool, err := pgxpool.NewWithConfig(connectCtx, pgxConfig)
	if err != nil {
		logger.Error("failed to create database pool", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	if err := pool.Ping(connectCtx); err != nil {
		logger.Error("failed to reach database", "error", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	service := presence.NewService(logger, pool, cfg.Presence, nil, nil)
	runner := presence.NewRunner(logger, cfg.Presence, service)
	if err := runner.Run(ctx); err != nil {
		logger.Error("presence worker exited with error", "error", err)
		os.Exit(1)
	}
}
