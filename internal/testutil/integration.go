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
	tc "github.com/testcontainers/testcontainers-go"
	postgresmodule "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	postgresImage = "postgres:18.3-alpine3.22"
)

type PostgresContainer struct {
	Container tc.Container
	URL       string
	Pool      *pgxpool.Pool
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

	waitForPostgres(t, pool)

	applyPostgresSchema(t, ctx, pool)

	return &PostgresContainer{
		Container: container,
		URL:       url,
		Pool:      pool,
	}
}

func applyPostgresSchema(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()

	migrationPaths, err := filepath.Glob(filepath.Join(repoRoot(t), "db", "migrations", "*.sql"))
	if err != nil {
		t.Fatalf("failed to list migration files: %v", err)
	}
	if len(migrationPaths) == 0 {
		t.Fatal("no migration files found")
	}

	for _, migrationPath := range migrationPaths {
		sqlBytes, err := os.ReadFile(migrationPath)
		if err != nil {
			t.Fatalf("failed to read migration file %s: %v", migrationPath, err)
		}

		upSQL := extractGooseUp(string(sqlBytes))
		if _, err := pool.Exec(ctx, upSQL); err != nil {
			t.Fatalf("failed to apply schema migration %s: %v", filepath.Base(migrationPath), err)
		}
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

func waitForPostgres(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()

	for {
		if err := pool.Ping(ctx); err == nil {
			return
		}

		select {
		case <-ctx.Done():
			t.Fatalf("failed to ping postgres: %v", ctx.Err())
		case <-ticker.C:
		}
	}
}
