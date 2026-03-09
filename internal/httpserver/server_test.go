package httpserver

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"

	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/sbekti/intern-api/internal/config"
	"github.com/sbekti/intern-api/internal/db"
)

func TestBootstrapRoutes(t *testing.T) {
	t.Parallel()

	handler := NewHandler(slog.New(slog.NewTextHandler(io.Discard, nil)), mustTestConfig(t), Dependencies{})

	tests := []struct {
		path string
	}{
		{path: "/healthz"},
		{path: "/readyz"},
		{path: "/api/v1/system/ping"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.path, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
			}
			if got := rec.Header().Get("Content-Type"); got != "application/json" {
				t.Fatalf("expected application/json content type, got %q", got)
			}
		})
	}
}

func TestBootstrapRoutesWithForwardAuth(t *testing.T) {
	t.Parallel()

	handler := NewHandler(slog.New(slog.NewTextHandler(io.Discard, nil)), mustTestConfig(t), Dependencies{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/system/ping", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	req.Header.Set("Remote-User", "alice")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("expected application/json content type, got %q", got)
	}
	if rec.Body.String() == "" {
		t.Fatal("expected response body")
	}
}

func TestGetProfileRequiresAuthentication(t *testing.T) {
	t.Parallel()

	handler := NewHandler(slog.New(slog.NewTextHandler(io.Discard, nil)), mustTestConfig(t), Dependencies{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/profile", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, rec.Code)
	}
}

func TestGetProfileSyncsAndReturnsProfile(t *testing.T) {
	t.Parallel()

	called := false
	handler := NewHandler(slog.New(slog.NewTextHandler(io.Discard, nil)), mustTestConfig(t), Dependencies{
		UserStore: fakeProfileUserStore{
			upsertFn: func(ctx context.Context, arg db.UpsertUserByUsernameParams) (db.User, error) {
				called = true
				return db.User{
					Username: arg.Username,
					Name:     arg.Name,
					Email:    arg.Email,
					Groups:   arg.Groups,
				}, nil
			},
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/profile", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	req.Header.Set("Remote-User", "alice")
	req.Header.Set("Remote-Name", "Alice Example")
	req.Header.Set("Remote-Email", "alice@example.com")
	req.Header.Set("Remote-Groups", "Users, Super-Users")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var profile struct {
		Username string              `json:"username"`
		Name     string              `json:"name"`
		Email    openapi_types.Email `json:"email"`
		Groups   []string            `json:"groups"`
		IsAdmin  bool                `json:"is_admin"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&profile); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if profile.Username != "alice" {
		t.Fatalf("expected username alice, got %q", profile.Username)
	}
	if profile.Email != openapi_types.Email("alice@example.com") {
		t.Fatalf("expected alice@example.com, got %q", profile.Email)
	}
	if !profile.IsAdmin {
		t.Fatal("expected profile to be admin")
	}
	if !called {
		t.Fatal("expected user store to be called")
	}
}

type fakeProfileUserStore struct {
	upsertFn func(ctx context.Context, arg db.UpsertUserByUsernameParams) (db.User, error)
}

func (f fakeProfileUserStore) UpsertUserByUsername(ctx context.Context, arg db.UpsertUserByUsernameParams) (db.User, error) {
	return f.upsertFn(ctx, arg)
}

func mustTestConfig(t *testing.T) config.Config {
	t.Helper()

	cfg := config.Config{
		Server:   config.ServerConfig{Addr: ":8080"},
		Database: config.DatabaseConfig{URL: "postgres://postgres:postgres@127.0.0.1:5432/intern_test?sslmode=disable"},
		LogLevel: config.LogLevelInfo,
		Auth: config.AuthConfig{
			JWTIssuer:     "intern.corp.example.com",
			JWTAudience:   "internctl",
			JWTHMACSecret: "test-secret",
		},
		TrustedProxy: config.TrustedProxyConfig{
			CIDRs:        []netip.Prefix{netip.MustParsePrefix("127.0.0.1/32")},
			UserHeader:   "Remote-User",
			NameHeader:   "Remote-Name",
			EmailHeader:  "Remote-Email",
			GroupsHeader: "Remote-Groups",
		},
		Authorization: config.AuthorizationConfig{
			AdminGroups: []string{"Super-Users"},
		},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("failed to validate test config: %v", err)
	}
	return cfg
}
