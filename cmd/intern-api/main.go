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

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sbekti/intern-api/internal/config"
	"github.com/sbekti/intern-api/internal/db"
	"github.com/sbekti/intern-api/internal/httpserver"
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

	var (
		pool    *pgxpool.Pool
		handler http.Handler
	)

	if cfg.Database.URL != "" {
		connectCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		pgxConfig, err := pgxpool.ParseConfig(cfg.Database.URL)
		if err != nil {
			logger.Error("failed to parse database url", "error", err)
			os.Exit(1)
		}

		pool, err = pgxpool.NewWithConfig(connectCtx, pgxConfig)
		if err != nil {
			logger.Error("failed to create database pool", "error", err)
			os.Exit(1)
		}

		if err := pool.Ping(connectCtx); err != nil {
			pool.Close()
			logger.Error("failed to reach database", "error", err)
			os.Exit(1)
		}

		logger.Info("connected to database")
		handler = httpserver.NewHandler(logger, cfg, httpserver.Dependencies{
			UserStore: db.New(pool),
		})
	} else {
		logger.Warn("database URL not configured; persisted identity-backed endpoints are running in degraded mode")
		handler = httpserver.NewHandler(logger, cfg, httpserver.Dependencies{})
	}
	if pool != nil {
		defer pool.Close()
	}

	server := &http.Server{
		Addr:              cfg.Server.Addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		logger.Info("starting server", "addr", cfg.Server.Addr, "log_level", cfg.LogLevel.String())
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case err := <-errCh:
		if err != nil {
			logger.Error("server exited with error", "error", err)
			os.Exit(1)
		}
	case <-ctx.Done():
		logger.Info("shutdown signal received")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("graceful shutdown failed", "error", err)
		os.Exit(1)
	}

	logger.Info("server stopped")
}
