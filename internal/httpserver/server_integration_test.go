//go:build integration

package httpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"
	"time"

	jwt "github.com/golang-jwt/jwt/v5"

	"github.com/sbekti/intern-api/internal/api"
	"github.com/sbekti/intern-api/internal/auth"
	"github.com/sbekti/intern-api/internal/clientauth"
	"github.com/sbekti/intern-api/internal/config"
	"github.com/sbekti/intern-api/internal/db"
	"github.com/sbekti/intern-api/internal/devices"
	"github.com/sbekti/intern-api/internal/testutil"
	"github.com/sbekti/intern-api/internal/vlans"
)

func TestHandlerIntegrationTrustedForwardAuthPersistsUser(t *testing.T) {
	t.Parallel()

	testEnv := newHandlerIntegrationEnv(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/profile", nil)
	req.RemoteAddr = net.JoinHostPort("127.0.0.1", "43210")
	req.Header.Set("Remote-User", "alice")
	req.Header.Set("Remote-Name", "Alice Example")
	req.Header.Set("Remote-Email", "alice@example.com")
	req.Header.Set("Remote-Groups", "Users")

	rec := httptest.NewRecorder()
	testEnv.handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var profile api.Profile
	decodeBody(t, rec.Body, &profile)

	if profile.Username != "alice" || profile.Name != "Alice Example" || string(profile.Email) != "alice@example.com" {
		t.Fatalf("unexpected profile payload %#v", profile)
	}
	if profile.IsAdmin {
		t.Fatalf("expected non-admin profile, got %#v", profile)
	}

	user, err := testEnv.queries.GetUserByUsername(context.Background(), db.GetUserByUsernameParams{Username: "alice"})
	if err != nil {
		t.Fatalf("expected persisted user row, got %v", err)
	}
	if user.Email != "alice@example.com" || len(user.Groups) != 1 || user.Groups[0] != "Users" {
		t.Fatalf("unexpected persisted user %#v", user)
	}
}

func TestHandlerIntegrationRejectsUntrustedForwardHeaders(t *testing.T) {
	t.Parallel()

	testEnv := newHandlerIntegrationEnv(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/profile", nil)
	req.RemoteAddr = net.JoinHostPort("198.51.100.10", "54321")
	req.Header.Set("Remote-User", "alice")

	rec := httptest.NewRecorder()
	testEnv.handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestHandlerIntegrationAdminRouteForbiddenForNonAdmin(t *testing.T) {
	t.Parallel()

	testEnv := newHandlerIntegrationEnv(t)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/networks/vlans", bytes.NewBufferString(`{"name":"lab","vlan_id":30}`))
	req.RemoteAddr = net.JoinHostPort("127.0.0.1", "43210")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Remote-User", "alice")
	req.Header.Set("Remote-Name", "Alice Example")
	req.Header.Set("Remote-Email", "alice@example.com")
	req.Header.Set("Remote-Groups", "Users")

	rec := httptest.NewRecorder()
	testEnv.handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status 403, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestHandlerIntegrationAdminRouteAllowsSuperUsers(t *testing.T) {
	t.Parallel()

	testEnv := newHandlerIntegrationEnv(t)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/networks/vlans", bytes.NewBufferString(`{"name":"lab","vlan_id":30}`))
	req.RemoteAddr = net.JoinHostPort("127.0.0.1", "43210")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Remote-User", "bob")
	req.Header.Set("Remote-Name", "Bob Example")
	req.Header.Set("Remote-Email", "bob@example.com")
	req.Header.Set("Remote-Groups", "Users, Super-Users")

	rec := httptest.NewRecorder()
	testEnv.handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d body=%s", rec.Code, rec.Body.String())
	}

	var vlan api.Vlan
	decodeBody(t, rec.Body, &vlan)

	if vlan.Name != "lab" || vlan.VlanId != 30 {
		t.Fatalf("unexpected vlan payload %#v", vlan)
	}

	var auditCount int
	if err := testEnv.pg.Pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM audit_logs WHERE action = 'vlan.create'`).Scan(&auditCount); err != nil {
		t.Fatalf("failed to count audit rows: %v", err)
	}
	if auditCount != 1 {
		t.Fatalf("expected 1 vlan.create audit log, got %d", auditCount)
	}
}

func TestHandlerIntegrationAdminDeviceListRequiresAdmin(t *testing.T) {
	t.Parallel()

	testEnv := newHandlerIntegrationEnv(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/networks/devices", nil)
	req.RemoteAddr = net.JoinHostPort("127.0.0.1", "43210")
	req.Header.Set("Remote-User", "alice")
	req.Header.Set("Remote-Groups", "Users")

	rec := httptest.NewRecorder()
	testEnv.handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status 403, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestHandlerIntegrationApproveDeviceCodeForAuthenticatedUser(t *testing.T) {
	t.Parallel()

	testEnv := newHandlerIntegrationEnv(t)

	deviceCode, err := testEnv.clientAuthService.CreateDeviceCode(context.Background(), &api.DeviceCodeCreateRequest{
		ClientName: stringPtr("desktop-app"),
	})
	if err != nil {
		t.Fatalf("failed to create device code: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/device_codes/"+deviceCode.UserCode+"/approve", nil)
	req.RemoteAddr = net.JoinHostPort("127.0.0.1", "43210")
	req.Header.Set("Remote-User", "alice")
	req.Header.Set("Remote-Name", "Alice Example")
	req.Header.Set("Remote-Email", "alice@example.com")
	req.Header.Set("Remote-Groups", "Users")

	rec := httptest.NewRecorder()
	testEnv.handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected status 204, got %d body=%s", rec.Code, rec.Body.String())
	}

	record, err := testEnv.queries.GetAuthDeviceAuthorizationByDeviceCode(context.Background(), db.GetAuthDeviceAuthorizationByDeviceCodeParams{
		DeviceCode: deviceCode.DeviceCode,
	})
	if err != nil {
		t.Fatalf("failed to reload device code: %v", err)
	}
	if record.Status != "approved" {
		t.Fatalf("expected approved status, got %q", record.Status)
	}
}

func TestHandlerIntegrationDenyDeviceCodeForAuthenticatedUser(t *testing.T) {
	t.Parallel()

	testEnv := newHandlerIntegrationEnv(t)

	deviceCode, err := testEnv.clientAuthService.CreateDeviceCode(context.Background(), &api.DeviceCodeCreateRequest{
		ClientName: stringPtr("mobile-app"),
	})
	if err != nil {
		t.Fatalf("failed to create device code: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/device_codes/"+deviceCode.UserCode+"/deny", nil)
	req.RemoteAddr = net.JoinHostPort("127.0.0.1", "43210")
	req.Header.Set("Remote-User", "alice")
	req.Header.Set("Remote-Name", "Alice Example")
	req.Header.Set("Remote-Email", "alice@example.com")
	req.Header.Set("Remote-Groups", "Users")

	rec := httptest.NewRecorder()
	testEnv.handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected status 204, got %d body=%s", rec.Code, rec.Body.String())
	}

	record, err := testEnv.queries.GetAuthDeviceAuthorizationByDeviceCode(context.Background(), db.GetAuthDeviceAuthorizationByDeviceCodeParams{
		DeviceCode: deviceCode.DeviceCode,
	})
	if err != nil {
		t.Fatalf("failed to reload device code: %v", err)
	}
	if record.Status != "denied" {
		t.Fatalf("expected denied status, got %q", record.Status)
	}
}

func TestHandlerIntegrationBearerTokenAuthenticatedProfile(t *testing.T) {
	t.Parallel()

	testEnv := newHandlerIntegrationEnv(t)

	token := mintAccessToken(t, testEnv.cfg, auth.AccessTokenClaims{
		Username: "carol",
		Name:     "Carol Example",
		Email:    "carol@example.com",
		Groups:   []string{"Users"},
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "carol",
			Issuer:    testEnv.cfg.Auth.JWTIssuer,
			Audience:  jwt.ClaimStrings{testEnv.cfg.Auth.JWTAudience},
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(15 * time.Minute)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/profile", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	rec := httptest.NewRecorder()
	testEnv.handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var profile api.Profile
	decodeBody(t, rec.Body, &profile)
	if profile.Username != "carol" || string(profile.Email) != "carol@example.com" {
		t.Fatalf("unexpected profile payload %#v", profile)
	}
}

type handlerIntegrationEnv struct {
	cfg               config.Config
	handler           http.Handler
	pg                *testutil.PostgresContainer
	queries           *db.Queries
	clientAuthService *clientauth.Service
}

func newHandlerIntegrationEnv(t *testing.T) handlerIntegrationEnv {
	t.Helper()

	pg := testutil.StartPostgres(t)
	queries := db.New(pg.Pool)
	cfg := integrationHandlerConfig(pg.URL)

	clientAuthService := clientauth.NewService(cfg, queries, clientauth.NewPGXTransactor(pg.Pool))

	handler := NewHandler(slog.New(slog.NewTextHandler(io.Discard, nil)), cfg, Dependencies{
		UserStore:         queries,
		DashboardStore:    queries,
		VLANService:       vlans.NewService(queries, vlans.NewPGXTransactor(pg.Pool)),
		DeviceService:     devices.NewService(queries, devices.NewPGXTransactor(pg.Pool)),
		ClientAuthService: clientAuthService,
	})

	return handlerIntegrationEnv{
		cfg:               cfg,
		handler:           handler,
		pg:                pg,
		queries:           queries,
		clientAuthService: clientAuthService,
	}
}

func integrationHandlerConfig(databaseURL string) config.Config {
	cfg := config.Config{
		Server:   config.ServerConfig{Addr: ":8080"},
		Database: config.DatabaseConfig{URL: databaseURL},
		Redis:    config.RedisConfig{URL: "redis://127.0.0.1:6379/0"},
		Weather:  config.WeatherConfig{BaseURL: "https://weather.example.test", LocationName: "Example Home", Latitude: 40.7128, Longitude: -74.0060, CacheTTL: 15 * time.Minute},
		LogLevel: config.LogLevelInfo,
		Auth: config.AuthConfig{
			JWTIssuer:          "intern.corp.example.com",
			JWTAudience:        "internctl",
			JWTHMACSecret:      "integration-secret",
			AccessTokenTTL:     15 * time.Minute,
			RefreshIdleTTL:     30 * 24 * time.Hour,
			RefreshAbsoluteTTL: 90 * 24 * time.Hour,
			DeviceCodeTTL:      10 * time.Minute,
			DevicePollInterval: 5 * time.Second,
			VerificationURL:    "https://intern.corp.example.com/auth/device",
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
		panic(err)
	}

	return cfg
}

func mintAccessToken(t *testing.T, cfg config.Config, claims auth.AccessTokenClaims) string {
	t.Helper()

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(cfg.Auth.JWTHMACSecret))
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}
	return signed
}

func decodeBody(t *testing.T, body *bytes.Buffer, target any) {
	t.Helper()

	if err := json.NewDecoder(body).Decode(target); err != nil {
		t.Fatalf("failed to decode response body: %v", err)
	}
}

func stringPtr(value string) *string {
	return &value
}
