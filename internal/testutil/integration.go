//go:build integration

package testutil

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	tc "github.com/testcontainers/testcontainers-go"
	postgresmodule "github.com/testcontainers/testcontainers-go/modules/postgres"
	redismodule "github.com/testcontainers/testcontainers-go/modules/redis"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	postgresImage = "postgres:18.3-alpine3.22"
	redisImage    = "redis:8.4.0-alpine3.22"
)

type PostgresContainer struct {
	Container tc.Container
	URL       string
	Pool      *pgxpool.Pool
}

type RedisContainer struct {
	Container tc.Container
	URL       string
	Client    *redis.Client
}

func StartPostgres(t *testing.T) *PostgresContainer {
	t.Helper()
	requireDocker(t)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	container, err := postgresmodule.Run(ctx,
		postgresImage,
		postgresmodule.WithDatabase("intern_test"),
		postgresmodule.WithUsername("postgres"),
		postgresmodule.WithPassword("postgres"),
		tc.WithWaitStrategy(
			wait.ForListeningPort("5432/tcp"),
			wait.ForLog("database system is ready to accept connections"),
		),
	)
	if err != nil {
		t.Fatalf("failed to start postgres container: %v", err)
	}

	t.Cleanup(func() {
		_ = container.Terminate(context.Background())
	})

	url, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("failed to get postgres connection string: %v", err)
	}

	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatalf("failed to create postgres pool: %v", err)
	}
	t.Cleanup(pool.Close)

	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("failed to ping postgres: %v", err)
	}

	applyPostgresSchema(t, ctx, pool)

	return &PostgresContainer{
		Container: container,
		URL:       url,
		Pool:      pool,
	}
}

func StartRedis(t *testing.T) *RedisContainer {
	t.Helper()
	requireDocker(t)

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	container, err := redismodule.Run(ctx,
		redisImage,
		tc.WithWaitStrategy(wait.ForListeningPort("6379/tcp")),
	)
	if err != nil {
		t.Fatalf("failed to start redis container: %v", err)
	}

	t.Cleanup(func() {
		_ = container.Terminate(context.Background())
	})

	addr, err := container.ConnectionString(ctx)
	if err != nil {
		t.Fatalf("failed to get redis connection string: %v", err)
	}

	client := redis.NewClient(&redis.Options{Addr: addr})
	t.Cleanup(func() { _ = client.Close() })

	if err := client.Ping(ctx).Err(); err != nil {
		t.Fatalf("failed to ping redis: %v", err)
	}

	return &RedisContainer{
		Container: container,
		URL:       "redis://" + addr + "/0",
		Client:    client,
	}
}

func applyPostgresSchema(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()

	sqlBytes, err := os.ReadFile(filepath.Join(repoRoot(t), "db", "migrations", "202603080001_initial_schema.sql"))
	if err != nil {
		t.Fatalf("failed to read migration file: %v", err)
	}

	upSQL := extractGooseUp(string(sqlBytes))
	if _, err := pool.Exec(ctx, upSQL); err != nil {
		t.Fatalf("failed to apply schema: %v", err)
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()

	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("failed to resolve caller path")
	}

	return filepath.Clean(filepath.Join(filepath.Dir(filename), "..", ".."))
}

func extractGooseUp(contents string) string {
	upStart := strings.Index(contents, "-- +goose Up")
	downStart := strings.Index(contents, "-- +goose Down")
	if upStart == -1 || downStart == -1 || downStart <= upStart {
		return contents
	}

	section := contents[upStart:downStart]
	lines := strings.Split(section, "\n")
	filtered := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "-- +goose") {
			continue
		}
		filtered = append(filtered, line)
	}

	return strings.TrimSpace(strings.Join(filtered, "\n"))
}

func requireDocker(t *testing.T) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "docker", "info")
	if err := cmd.Run(); err != nil {
		t.Skipf("docker is unavailable for integration tests: %v", err)
	}
}
