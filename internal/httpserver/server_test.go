package httpserver

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/sbekti/intern-api/internal/api"
	"github.com/sbekti/intern-api/internal/config"
	"github.com/sbekti/intern-api/internal/db"
	"github.com/sbekti/intern-api/internal/vlans"
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

func TestGetDashboardRequiresAuthentication(t *testing.T) {
	t.Parallel()

	handler := NewHandler(slog.New(slog.NewTextHandler(io.Discard, nil)), mustTestConfig(t), Dependencies{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/dashboard", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, rec.Code)
	}
}

func TestGetDashboardReturnsSummary(t *testing.T) {
	t.Parallel()

	handler := NewHandler(slog.New(slog.NewTextHandler(io.Discard, nil)), mustTestConfig(t), Dependencies{
		UserStore: fakeProfileUserStore{
			upsertFn: func(ctx context.Context, arg db.UpsertUserByUsernameParams) (db.User, error) {
				return db.User{
					Username: arg.Username,
					Name:     arg.Name,
					Email:    arg.Email,
					Groups:   arg.Groups,
				}, nil
			},
		},
		DashboardStore: fakeDashboardStore{
			deviceCount: 7,
			vlanCount:   3,
		},
		WeatherService: fakeDashboardWeatherService{
			locationName: "Example Home",
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/dashboard", nil)
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

	var payload struct {
		WelcomeMessage string `json:"welcome_message"`
		NetworkSummary struct {
			DeviceCount int64 `json:"device_count"`
			VlanCount   int64 `json:"vlan_count"`
		} `json:"network_summary"`
		Weather *struct {
			LocationName string `json:"location_name"`
		} `json:"weather"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("failed to decode dashboard response: %v", err)
	}

	if payload.WelcomeMessage != "Welcome, Alice Example" {
		t.Fatalf("unexpected welcome message %q", payload.WelcomeMessage)
	}
	if payload.NetworkSummary.DeviceCount != 7 {
		t.Fatalf("expected 7 devices, got %d", payload.NetworkSummary.DeviceCount)
	}
	if payload.NetworkSummary.VlanCount != 3 {
		t.Fatalf("expected 3 vlans, got %d", payload.NetworkSummary.VlanCount)
	}
	if payload.Weather == nil || payload.Weather.LocationName != "Example Home" {
		t.Fatal("expected weather payload")
	}
}

func TestListVlansRequiresAuthentication(t *testing.T) {
	t.Parallel()

	handler := NewHandler(slog.New(slog.NewTextHandler(io.Discard, nil)), mustTestConfig(t), Dependencies{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/networks/vlans", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, rec.Code)
	}
}

func TestListVlansReturnsItems(t *testing.T) {
	t.Parallel()

	handler := NewHandler(slog.New(slog.NewTextHandler(io.Discard, nil)), mustTestConfig(t), Dependencies{
		UserStore: fakeProfileUserStore{
			upsertFn: func(ctx context.Context, arg db.UpsertUserByUsernameParams) (db.User, error) {
				return db.User{Username: arg.Username, Name: arg.Name, Email: arg.Email, Groups: arg.Groups}, nil
			},
		},
		VLANService: fakeVLANService{
			listFn: func(ctx context.Context) ([]db.Vlan, error) {
				return []db.Vlan{{
					ID:          1,
					Name:        "guest",
					VlanID:      10,
					Description: "Guest devices",
					IsActive:    true,
					CreatedAt:   testTimestamp(),
					UpdatedAt:   testTimestamp(),
				}}, nil
			},
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/networks/vlans", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	req.Header.Set("Remote-User", "alice")
	req.Header.Set("Remote-Name", "Alice Example")
	req.Header.Set("Remote-Email", "alice@example.com")
	req.Header.Set("Remote-Groups", "Users")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
}

func TestCreateVlanRequiresAdmin(t *testing.T) {
	t.Parallel()

	handler := NewHandler(slog.New(slog.NewTextHandler(io.Discard, nil)), mustTestConfig(t), Dependencies{
		UserStore: fakeProfileUserStore{
			upsertFn: func(ctx context.Context, arg db.UpsertUserByUsernameParams) (db.User, error) {
				return db.User{Username: arg.Username, Name: arg.Name, Email: arg.Email, Groups: arg.Groups}, nil
			},
		},
		VLANService: fakeVLANService{
			createFn: func(ctx context.Context, actor db.User, input api.VlanWrite) (db.Vlan, error) {
				t.Fatal("expected create not to be called")
				return db.Vlan{}, nil
			},
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/networks/vlans", strings.NewReader(`{"name":"guest","vlan_id":10}`))
	req.RemoteAddr = "127.0.0.1:12345"
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Remote-User", "alice")
	req.Header.Set("Remote-Name", "Alice Example")
	req.Header.Set("Remote-Email", "alice@example.com")
	req.Header.Set("Remote-Groups", "Users")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, rec.Code)
	}
}

func TestCreateVlanReturnsCreated(t *testing.T) {
	t.Parallel()

	handler := NewHandler(slog.New(slog.NewTextHandler(io.Discard, nil)), mustTestConfig(t), Dependencies{
		UserStore: fakeProfileUserStore{
			upsertFn: func(ctx context.Context, arg db.UpsertUserByUsernameParams) (db.User, error) {
				return db.User{Username: arg.Username, Name: arg.Name, Email: arg.Email, Groups: arg.Groups}, nil
			},
		},
		VLANService: fakeVLANService{
			createFn: func(ctx context.Context, actor db.User, input api.VlanWrite) (db.Vlan, error) {
				return db.Vlan{
					ID:          1,
					Name:        input.Name,
					VlanID:      input.VlanId,
					Description: "",
					IsActive:    true,
					CreatedAt:   testTimestamp(),
					UpdatedAt:   testTimestamp(),
				}, nil
			},
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/networks/vlans", strings.NewReader(`{"name":"guest","vlan_id":10}`))
	req.RemoteAddr = "127.0.0.1:12345"
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Remote-User", "alice")
	req.Header.Set("Remote-Name", "Alice Example")
	req.Header.Set("Remote-Email", "alice@example.com")
	req.Header.Set("Remote-Groups", "Users, Super-Users")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d", http.StatusCreated, rec.Code)
	}
}

func TestGetVlanReturnsNotFound(t *testing.T) {
	t.Parallel()

	handler := NewHandler(slog.New(slog.NewTextHandler(io.Discard, nil)), mustTestConfig(t), Dependencies{
		UserStore: fakeProfileUserStore{
			upsertFn: func(ctx context.Context, arg db.UpsertUserByUsernameParams) (db.User, error) {
				return db.User{Username: arg.Username, Name: arg.Name, Email: arg.Email, Groups: arg.Groups}, nil
			},
		},
		VLANService: fakeVLANService{
			getFn: func(ctx context.Context, id int64) (db.Vlan, error) {
				return db.Vlan{}, vlans.ErrNotFound
			},
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/networks/vlans/999", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	req.Header.Set("Remote-User", "alice")
	req.Header.Set("Remote-Name", "Alice Example")
	req.Header.Set("Remote-Email", "alice@example.com")
	req.Header.Set("Remote-Groups", "Users")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, rec.Code)
	}
}

type fakeProfileUserStore struct {
	upsertFn func(ctx context.Context, arg db.UpsertUserByUsernameParams) (db.User, error)
}

func (f fakeProfileUserStore) UpsertUserByUsername(ctx context.Context, arg db.UpsertUserByUsernameParams) (db.User, error) {
	return f.upsertFn(ctx, arg)
}

type fakeDashboardStore struct {
	deviceCount int64
	vlanCount   int64
}

func (f fakeDashboardStore) CountNetworkDevices(ctx context.Context) (int64, error) {
	return f.deviceCount, nil
}

func (f fakeDashboardStore) CountVlans(ctx context.Context) (int64, error) {
	return f.vlanCount, nil
}

type fakeDashboardWeatherService struct {
	locationName string
}

func (f fakeDashboardWeatherService) GetSummary(ctx context.Context) (*api.WeatherSummary, error) {
	return &api.WeatherSummary{
		LocationName: f.locationName,
		Timezone:     "America/New_York",
		Current: api.WeatherCurrent{
			TemperatureC: 20,
			WindSpeedKph: 10,
			WeatherCode:  1,
		},
	}, nil
}

type fakeVLANService struct {
	listFn   func(ctx context.Context) ([]db.Vlan, error)
	getFn    func(ctx context.Context, id int64) (db.Vlan, error)
	createFn func(ctx context.Context, actor db.User, input api.VlanWrite) (db.Vlan, error)
	updateFn func(ctx context.Context, actor db.User, id int64, patch api.VlanPatch) (db.Vlan, error)
	deleteFn func(ctx context.Context, actor db.User, id int64) error
}

func (f fakeVLANService) List(ctx context.Context) ([]db.Vlan, error) {
	return f.listFn(ctx)
}

func (f fakeVLANService) Get(ctx context.Context, id int64) (db.Vlan, error) {
	return f.getFn(ctx, id)
}

func (f fakeVLANService) Create(ctx context.Context, actor db.User, input api.VlanWrite) (db.Vlan, error) {
	return f.createFn(ctx, actor, input)
}

func (f fakeVLANService) Update(ctx context.Context, actor db.User, id int64, patch api.VlanPatch) (db.Vlan, error) {
	return f.updateFn(ctx, actor, id, patch)
}

func (f fakeVLANService) Delete(ctx context.Context, actor db.User, id int64) error {
	return f.deleteFn(ctx, actor, id)
}

func testTimestamp() pgtype.Timestamptz {
	return pgtype.Timestamptz{
		Time:  time.Date(2026, 3, 9, 12, 0, 0, 0, time.UTC),
		Valid: true,
	}
}

func mustTestConfig(t *testing.T) config.Config {
	t.Helper()

	cfg := config.Config{
		Server:   config.ServerConfig{Addr: ":8080"},
		Database: config.DatabaseConfig{URL: "postgres://postgres:postgres@127.0.0.1:5432/intern_test?sslmode=disable"},
		Redis:    config.RedisConfig{URL: "redis://127.0.0.1:6379/0"},
		Weather:  config.WeatherConfig{BaseURL: "https://weather.example.test", LocationName: "Example Home", Latitude: 40.7128, Longitude: -74.0060, CacheTTL: 15 * time.Minute},
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
