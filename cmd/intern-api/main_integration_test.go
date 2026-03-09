//go:build integration

package main

import (
	"bytes"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/sbekti/intern-api/internal/testutil"
)

const helperProcessEnv = "GO_WANT_INTERN_API_HELPER_PROCESS"

func TestMainProcessHelper(t *testing.T) {
	t.Helper()

	if os.Getenv(helperProcessEnv) != "1" {
		return
	}

	main()
	os.Exit(0)
}

func TestMainStartsWithReachableDependencies(t *testing.T) {
	t.Parallel()

	pg := testutil.StartPostgres(t)
	redis := testutil.StartRedis(t)
	addr := reserveLocalAddr(t)

	cmd := helperCommand(t, integrationEnv(addr, pg.URL, redis.URL)...)
	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start helper process: %v", err)
	}
	t.Cleanup(func() {
		if cmd.ProcessState == nil || !cmd.ProcessState.Exited() {
			_ = cmd.Process.Signal(syscall.SIGTERM)
			_, _ = waitWithTimeout(cmd, 5*time.Second)
		}
	})

	waitForHTTP(t, "http://"+addr+"/healthz")

	if cmd.ProcessState != nil && cmd.ProcessState.Exited() {
		t.Fatalf("expected process to remain running, output=%s", commandOutput(cmd))
	}

	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatalf("failed to stop helper process: %v", err)
	}

	exitErr, timedOut := waitWithTimeout(cmd, 5*time.Second)
	if timedOut {
		t.Fatalf("timed out waiting for shutdown, output=%s", commandOutput(cmd))
	}
	if exitErr != nil {
		t.Fatalf("expected graceful shutdown, got %v output=%s", exitErr, commandOutput(cmd))
	}
}

func TestMainFailsWithBrokenDatabaseURL(t *testing.T) {
	t.Parallel()

	redis := testutil.StartRedis(t)
	addr := reserveLocalAddr(t)

	cmd := helperCommand(t, integrationEnv(addr, "postgres://postgres:postgres@127.0.0.1:1/intern_test?sslmode=disable", redis.URL)...)

	exitErr, timedOut := runAndWait(cmd, 10*time.Second)
	if timedOut {
		t.Fatalf("timed out waiting for failed startup, output=%s", commandOutput(cmd))
	}
	if exitErr == nil {
		t.Fatalf("expected startup failure for database, output=%s", commandOutput(cmd))
	}
	if !strings.Contains(commandOutput(cmd), "failed to reach database") {
		t.Fatalf("expected database error in output, got %s", commandOutput(cmd))
	}
}

func TestMainFailsWithBrokenRedisURL(t *testing.T) {
	t.Parallel()

	pg := testutil.StartPostgres(t)
	addr := reserveLocalAddr(t)

	cmd := helperCommand(t, integrationEnv(addr, pg.URL, "redis://127.0.0.1:1/0")...)

	exitErr, timedOut := runAndWait(cmd, 10*time.Second)
	if timedOut {
		t.Fatalf("timed out waiting for failed startup, output=%s", commandOutput(cmd))
	}
	if exitErr == nil {
		t.Fatalf("expected startup failure for redis, output=%s", commandOutput(cmd))
	}
	if !strings.Contains(commandOutput(cmd), "failed to reach redis") {
		t.Fatalf("expected redis error in output, got %s", commandOutput(cmd))
	}
}

func helperCommand(t *testing.T, env ...string) *exec.Cmd {
	t.Helper()

	cmd := exec.Command(os.Args[0], "-test.run=TestMainProcessHelper", "--")
	cmd.Env = append(os.Environ(), env...)
	cmd.Stdout = &bytes.Buffer{}
	cmd.Stderr = cmd.Stdout
	return cmd
}

func integrationEnv(addr, databaseURL, redisURL string) []string {
	return []string{
		helperProcessEnv + "=1",
		"INTERN_API_ADDR=" + addr,
		"INTERN_API_DATABASE_URL=" + databaseURL,
		"INTERN_API_REDIS_URL=" + redisURL,
		"INTERN_API_LOG_LEVEL=info",
		"WEATHER_BASE_URL=https://weather.example.test",
		"WEATHER_LOCATION_NAME=Example Home",
		"WEATHER_LATITUDE=40.7128",
		"WEATHER_LONGITUDE=-74.0060",
		"WEATHER_CACHE_TTL=15m",
		"AUTH_JWT_ISSUER=intern.corp.example.com",
		"AUTH_JWT_AUDIENCE=internctl",
		"AUTH_JWT_HMAC_SECRET=integration-secret",
		"AUTH_ACCESS_TOKEN_TTL=15m",
		"AUTH_REFRESH_IDLE_TTL=720h",
		"AUTH_REFRESH_ABSOLUTE_TTL=2160h",
		"AUTH_DEVICE_CODE_TTL=10m",
		"AUTH_DEVICE_POLL_INTERVAL=5s",
		"AUTH_DEVICE_VERIFICATION_URL=https://intern.corp.example.com/auth/device",
		"TRUSTED_PROXY_CIDRS=127.0.0.1/32,::1/128",
		"AUTH_REMOTE_USER_HEADER=Remote-User",
		"AUTH_REMOTE_NAME_HEADER=Remote-Name",
		"AUTH_REMOTE_EMAIL_HEADER=Remote-Email",
		"AUTH_REMOTE_GROUPS_HEADER=Remote-Groups",
		"AUTH_ADMIN_GROUPS=Super-Users",
	}
}

func reserveLocalAddr(t *testing.T) string {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to reserve local port: %v", err)
	}
	defer listener.Close()

	return listener.Addr().String()
}

func waitForHTTP(t *testing.T, url string) {
	t.Helper()

	client := &http.Client{Timeout: 500 * time.Millisecond}
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := client.Get(url) //nolint:noctx
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return
			}
		}

		time.Sleep(100 * time.Millisecond)
	}

	t.Fatalf("server did not become healthy: %s", url)
}

func runAndWait(cmd *exec.Cmd, timeout time.Duration) (error, bool) {
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start helper process: %w", err), false
	}

	return waitWithTimeout(cmd, timeout)
}

func waitWithTimeout(cmd *exec.Cmd, timeout time.Duration) (error, bool) {
	waitCh := make(chan error, 1)
	go func() {
		waitCh <- cmd.Wait()
	}()

	select {
	case err := <-waitCh:
		if err == nil {
			return nil, false
		}
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return err, false
		}
		return err, false
	case <-time.After(timeout):
		return nil, true
	}
}

func commandOutput(cmd *exec.Cmd) string {
	buffer, ok := cmd.Stdout.(*bytes.Buffer)
	if !ok {
		return ""
	}
	return buffer.String()
}
