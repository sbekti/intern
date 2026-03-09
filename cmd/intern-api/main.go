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
	"github.com/redis/go-redis/v9"
	"github.com/sbekti/intern-api/internal/config"
	"github.com/sbekti/intern-api/internal/db"
	"github.com/sbekti/intern-api/internal/devices"
	"github.com/sbekti/intern-api/internal/httpserver"
	"github.com/sbekti/intern-api/internal/vlans"
	"github.com/sbekti/intern-api/internal/weather"
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

	logger.Info("connected to database")

	redisOptions, err := redis.ParseURL(cfg.Redis.URL)
	if err != nil {
		logger.Error("failed to parse redis url", "error", err)
		os.Exit(1)
	}

	redisClient := redis.NewClient(redisOptions)
	defer redisClient.Close()

	if err := redisClient.Ping(connectCtx).Err(); err != nil {
		logger.Error("failed to reach redis", "error", err)
		os.Exit(1)
	}

	logger.Info("connected to redis")

	queries := db.New(pool)
	deviceService := devices.NewService(queries, devices.NewPGXTransactor(pool))
	vlanService := vlans.NewService(queries, vlans.NewPGXTransactor(pool))

	server := &http.Server{
		Addr: cfg.Server.Addr,
		Handler: httpserver.NewHandler(logger, cfg, httpserver.Dependencies{
			UserStore:      queries,
			DashboardStore: queries,
			WeatherService: weather.NewService(cfg, weather.NewRedisCache(redisClient), nil),
			VLANService:    vlanService,
			DeviceService:  deviceService,
		}),
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
