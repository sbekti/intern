package httpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
	"time"

	jwt "github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/sbekti/intern/internal/api"
	"github.com/sbekti/intern/internal/auditlogs"
	"github.com/sbekti/intern/internal/auth"
	"github.com/sbekti/intern/internal/authspam"
	"github.com/sbekti/intern/internal/clientauth"
	"github.com/sbekti/intern/internal/config"
	"github.com/sbekti/intern/internal/db"
	"github.com/sbekti/intern/internal/devices"
	"github.com/sbekti/intern/internal/requestmeta"
	"github.com/sbekti/intern/internal/vlans"
)

func TestBootstrapRoutes(t *testing.T) {
	t.Parallel()

	handler := NewHandler(slog.New(slog.NewTextHandler(io.Discard, nil)), mustTestConfig(t), Dependencies{})

	tests := []struct {
		path string
	}{
		{path: "/healthz"},
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

type fakeDatabasePinger struct{ err error }

func (f fakeDatabasePinger) Ping(context.Context) error { return f.err }

func TestReadyReturnsServiceUnavailableWithoutDatabasePinger(t *testing.T) {
	t.Parallel()

	handler := NewHandler(slog.New(slog.NewTextHandler(io.Discard, nil)), mustTestConfig(t), Dependencies{})
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
}

func TestReadyReturnsOKWhenDatabasePingSucceeds(t *testing.T) {
	t.Parallel()

	handler := NewHandler(slog.New(slog.NewTextHandler(io.Discard, nil)), mustTestConfig(t), Dependencies{
		DatabasePinger: fakeDatabasePinger{},
	})
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestReadyReturnsServiceUnavailableWhenDatabasePingFails(t *testing.T) {
	t.Parallel()

	handler := NewHandler(slog.New(slog.NewTextHandler(io.Discard, nil)), mustTestConfig(t), Dependencies{
		DatabasePinger: fakeDatabasePinger{err: errors.New("database unavailable")},
	})
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("content type = %q, want application/json", got)
	}
}

func TestBootstrapRoutesWithForwardAuth(t *testing.T) {
	t.Parallel()

	handler := NewHandler(slog.New(slog.NewTextHandler(io.Discard, nil)), mustTestConfig(t), Dependencies{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/system/ping", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	req.Header.Set("Remote-User", "alice")
	req.Header.Set("X-Intern-Forward-Auth", "authenticated-ingress")

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

func TestGetProfileBearerSessionValidationFailureReturnsServiceUnavailable(t *testing.T) {
	t.Parallel()

	cfg := mustTestConfig(t)
	handler := NewHandler(slog.New(slog.NewTextHandler(io.Discard, nil)), cfg, Dependencies{
		SessionService: fakeSessionService{
			validateSessionFn: func(ctx context.Context, sessionID string) (bool, error) {
				return false, errors.New("database unavailable")
			},
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/profile", nil)
	req.Header.Set("Authorization", "Bearer "+mintBearerAccessTokenForTest(t, cfg, "alice", nil, "session-1"))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status %d, got %d", http.StatusServiceUnavailable, rec.Code)
	}
}

func TestGetProfileLooksUpWithoutWriting(t *testing.T) {
	t.Parallel()

	store := &readOnlyProfileUserStore{}
	handler := NewHandler(slog.New(slog.NewTextHandler(io.Discard, nil)), mustTestConfig(t), Dependencies{
		UserStore: store,
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/profile", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	setForwardAuthHeaders(req, "alice", "Alice Example", "alice@example.com", "Users, Super-Users")
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
	if store.upsertCalled {
		t.Fatal("profile lookup unexpectedly attempted a write")
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

func TestListDevicesRejectsRevokedBearerAdminSession(t *testing.T) {
	t.Parallel()

	cfg := mustTestConfig(t)
	handler := NewHandler(slog.New(slog.NewTextHandler(io.Discard, nil)), cfg, Dependencies{
		SessionService: fakeSessionService{
			validateSessionFn: func(ctx context.Context, sessionID string) (bool, error) {
				if sessionID != "session-1" {
					t.Fatalf("expected session-1, got %q", sessionID)
				}
				return false, nil
			},
		},
		DeviceService: fakeDeviceService{},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/networks/devices", nil)
	req.Header.Set("Authorization", "Bearer "+mintBearerAccessTokenForTest(t, cfg, "alice", []string{"Super-Users"}, "session-1"))

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
					Name:        "guest",
					VlanID:      10,
					Description: "Guest devices",
					CreatedAt:   testTimestamp(),
					UpdatedAt:   testTimestamp(),
				}}, nil
			},
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/networks/vlans", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	setForwardAuthHeaders(req, "bob", "Bob Example", "bob@example.com", "Users, Super-Users")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
}

func TestListVlansRequiresAdmin(t *testing.T) {
	t.Parallel()

	handler := NewHandler(slog.New(slog.NewTextHandler(io.Discard, nil)), mustTestConfig(t), Dependencies{
		UserStore: fakeProfileUserStore{
			upsertFn: func(ctx context.Context, arg db.UpsertUserByUsernameParams) (db.User, error) {
				return db.User{Username: arg.Username, Name: arg.Name, Email: arg.Email, Groups: arg.Groups}, nil
			},
		},
		VLANService: fakeVLANService{
			listFn: func(ctx context.Context) ([]db.Vlan, error) {
				t.Fatal("expected list not to be called")
				return nil, nil
			},
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/networks/vlans", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	setForwardAuthHeaders(req, "alice", "Alice Example", "alice@example.com", "Users")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, rec.Code)
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
	setForwardAuthHeaders(req, "alice", "Alice Example", "alice@example.com", "Users")
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
					Name:        input.Name,
					VlanID:      input.VlanId,
					Description: "",
					CreatedAt:   testTimestamp(),
					UpdatedAt:   testTimestamp(),
				}, nil
			},
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/networks/vlans", strings.NewReader(`{"name":"guest","vlan_id":10}`))
	req.RemoteAddr = "127.0.0.1:12345"
	req.Header.Set("Content-Type", "application/json")
	setForwardAuthHeaders(req, "alice", "Alice Example", "alice@example.com", "Users, Super-Users")
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
			getFn: func(ctx context.Context, vlanID int32) (db.Vlan, error) {
				return db.Vlan{}, vlans.ErrNotFound
			},
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/networks/vlans/999", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	setForwardAuthHeaders(req, "bob", "Bob Example", "bob@example.com", "Users, Super-Users")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, rec.Code)
	}
}

func TestGetVlanRequiresAdmin(t *testing.T) {
	t.Parallel()

	handler := NewHandler(slog.New(slog.NewTextHandler(io.Discard, nil)), mustTestConfig(t), Dependencies{
		UserStore: fakeProfileUserStore{
			upsertFn: func(ctx context.Context, arg db.UpsertUserByUsernameParams) (db.User, error) {
				return db.User{Username: arg.Username, Name: arg.Name, Email: arg.Email, Groups: arg.Groups}, nil
			},
		},
		VLANService: fakeVLANService{
			getFn: func(ctx context.Context, vlanID int32) (db.Vlan, error) {
				t.Fatal("expected get not to be called")
				return db.Vlan{}, nil
			},
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/networks/vlans/999", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	setForwardAuthHeaders(req, "alice", "Alice Example", "alice@example.com", "Users")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, rec.Code)
	}
}

func TestListDevicesRequiresAdmin(t *testing.T) {
	t.Parallel()

	handler := NewHandler(slog.New(slog.NewTextHandler(io.Discard, nil)), mustTestConfig(t), Dependencies{
		UserStore: fakeProfileUserStore{
			upsertFn: func(ctx context.Context, arg db.UpsertUserByUsernameParams) (db.User, error) {
				return db.User{Username: arg.Username, Name: arg.Name, Email: arg.Email, Groups: arg.Groups}, nil
			},
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/networks/devices", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	setForwardAuthHeaders(req, "alice", "Alice Example", "alice@example.com", "Users")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, rec.Code)
	}
}

func TestCreateDeviceReturnsCreated(t *testing.T) {
	t.Parallel()

	deviceID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	handler := NewHandler(slog.New(slog.NewTextHandler(io.Discard, nil)), mustTestConfig(t), Dependencies{
		UserStore: fakeProfileUserStore{
			upsertFn: func(ctx context.Context, arg db.UpsertUserByUsernameParams) (db.User, error) {
				return db.User{
					ID:       pgtype.UUID{Bytes: [16]byte(deviceID), Valid: true},
					Username: arg.Username,
					Name:     arg.Name,
					Email:    arg.Email,
					Groups:   arg.Groups,
				}, nil
			},
		},
		DeviceService: fakeDeviceService{
			createFn: func(ctx context.Context, actor db.User, input api.NetworkDeviceWrite) (devices.DeviceRecord, error) {
				disabled := input.Disabled != nil && *input.Disabled
				return devices.DeviceRecord{
					Device: db.NetworkDevice{
						ID:              pgtype.UUID{Bytes: [16]byte(deviceID), Valid: true},
						MacAddress:      "aa:bb:cc:dd:ee:ff",
						DisplayName:     input.DisplayName,
						Disabled:        disabled,
						VlanID:          input.VlanId,
						CreatedAt:       testTimestamp(),
						UpdatedAt:       testTimestamp(),
						CreatedByUserID: actor.ID,
						UpdatedByUserID: actor.ID,
					},
					VLAN: db.Vlan{
						Name:   "iot",
						VlanID: 20,
					},
				}, nil
			},
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/networks/devices", strings.NewReader(`{"mac_address":"AA-BB-CC-DD-EE-FF","display_name":"Camera","disabled":true,"vlan_id":20}`))
	req.RemoteAddr = "127.0.0.1:12345"
	req.Header.Set("Content-Type", "application/json")
	setForwardAuthHeaders(req, "alice", "Alice Example", "alice@example.com", "Users, Super-Users")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d", http.StatusCreated, rec.Code)
	}

	var payload api.NetworkDevice
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !payload.Disabled {
		t.Fatalf("expected disabled device payload, got %#v", payload)
	}
}

func TestGetDeviceReturnsNotFound(t *testing.T) {
	t.Parallel()

	handler := NewHandler(slog.New(slog.NewTextHandler(io.Discard, nil)), mustTestConfig(t), Dependencies{
		UserStore: fakeProfileUserStore{
			upsertFn: func(ctx context.Context, arg db.UpsertUserByUsernameParams) (db.User, error) {
				return db.User{Username: arg.Username, Name: arg.Name, Email: arg.Email, Groups: arg.Groups}, nil
			},
		},
		DeviceService: fakeDeviceService{
			getFn: func(ctx context.Context, id uuid.UUID) (devices.DeviceRecord, error) {
				return devices.DeviceRecord{}, devices.ErrNotFound
			},
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/networks/devices/11111111-1111-1111-1111-111111111111", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	setForwardAuthHeaders(req, "alice", "Alice Example", "alice@example.com", "Users, Super-Users")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, rec.Code)
	}
}

func TestCreateDeviceAuthorizationReturnsCreated(t *testing.T) {
	t.Parallel()

	handler := NewHandler(slog.New(slog.NewTextHandler(io.Discard, nil)), mustTestConfig(t), Dependencies{
		ClientAuthService: fakeClientAuthService{
			createFn: func(ctx context.Context, request *api.DeviceCodeCreateRequest) (*api.DeviceCode, error) {
				return &api.DeviceCode{
					DeviceCode:              "device-code",
					UserCode:                "ABCD-EFGH",
					VerificationUri:         "https://intern.corp.example.com/auth/device",
					VerificationUriComplete: "https://intern.corp.example.com/auth/device?user_code=ABCD-EFGH",
					ExpiresIn:               600,
					Interval:                5,
				}, nil
			},
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/device_codes", strings.NewReader(`{"client_name":"internctl"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d", http.StatusCreated, rec.Code)
	}
}

func TestCreateDeviceAuthorizationRateLimitedReturns429(t *testing.T) {
	t.Parallel()

	called := false
	handler := NewHandler(slog.New(slog.NewTextHandler(io.Discard, nil)), mustTestConfig(t), Dependencies{
		ClientAuthService: fakeClientAuthService{
			createFn: func(ctx context.Context, request *api.DeviceCodeCreateRequest) (*api.DeviceCode, error) {
				called = true
				return &api.DeviceCode{}, nil
			},
		},
		AuthSpamService: fakeAuthSpamService{
			checkDeviceCodeCreateFn: func(ctx context.Context, clientInfo requestmeta.ClientInfo) error {
				return authspam.RateLimitedError{RetryAfter: 3 * time.Second}
			},
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/device_codes", strings.NewReader(`{"client_name":"internctl"}`))
	req.RemoteAddr = "127.0.0.1:12345"
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected status %d, got %d", http.StatusTooManyRequests, rec.Code)
	}
	if rec.Header().Get("Retry-After") != "3" {
		t.Fatalf("expected Retry-After 3, got %q", rec.Header().Get("Retry-After"))
	}
	if called {
		t.Fatal("expected request to be blocked before client auth service")
	}
}

func TestApproveDeviceAuthorizationRequiresAuth(t *testing.T) {
	t.Parallel()

	handler := NewHandler(slog.New(slog.NewTextHandler(io.Discard, nil)), mustTestConfig(t), Dependencies{
		ClientAuthService: fakeClientAuthService{},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/device_codes/ABCD-EFGH/approve", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, rec.Code)
	}
}

func TestExchangeDeviceAuthorizationPendingReturns428(t *testing.T) {
	t.Parallel()

	handler := NewHandler(slog.New(slog.NewTextHandler(io.Discard, nil)), mustTestConfig(t), Dependencies{
		ClientAuthService: fakeClientAuthService{
			exchangeFn: func(ctx context.Context, request api.DeviceCodeTokenRequest, userAgent string) (*api.TokenResponse, error) {
				return nil, clientauth.ErrAuthorizationPending
			},
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/tokens", strings.NewReader(`{"device_code":"device-code"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusPreconditionRequired {
		t.Fatalf("expected status %d, got %d", http.StatusPreconditionRequired, rec.Code)
	}
}

func TestExchangeDeviceAuthorizationSlowDownReturns400(t *testing.T) {
	t.Parallel()

	handler := NewHandler(slog.New(slog.NewTextHandler(io.Discard, nil)), mustTestConfig(t), Dependencies{
		ClientAuthService: fakeClientAuthService{
			exchangeFn: func(ctx context.Context, request api.DeviceCodeTokenRequest, userAgent string) (*api.TokenResponse, error) {
				return nil, clientauth.ErrSlowDown
			},
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/tokens", strings.NewReader(`{"device_code":"device-code"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"error":"slow_down"`) {
		t.Fatalf("expected slow_down error body, got %s", rec.Body.String())
	}
}

func TestExchangeDeviceAuthorizationRateLimitedReturns429(t *testing.T) {
	t.Parallel()

	called := false
	handler := NewHandler(slog.New(slog.NewTextHandler(io.Discard, nil)), mustTestConfig(t), Dependencies{
		ClientAuthService: fakeClientAuthService{
			exchangeFn: func(ctx context.Context, request api.DeviceCodeTokenRequest, userAgent string) (*api.TokenResponse, error) {
				called = true
				return &api.TokenResponse{}, nil
			},
		},
		AuthSpamService: fakeAuthSpamService{
			checkDeviceTokenExchangeFn: func(ctx context.Context, clientInfo requestmeta.ClientInfo) error {
				return authspam.RateLimitedError{RetryAfter: 4 * time.Second}
			},
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/tokens", strings.NewReader(`{"device_code":"device-code"}`))
	req.RemoteAddr = "127.0.0.1:12345"
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected status %d, got %d", http.StatusTooManyRequests, rec.Code)
	}
	if rec.Header().Get("Retry-After") != "4" {
		t.Fatalf("expected Retry-After 4, got %q", rec.Header().Get("Retry-After"))
	}
	if called {
		t.Fatal("expected request to be blocked before client auth service")
	}
}

func TestRefreshAccessTokenUnauthorized(t *testing.T) {
	t.Parallel()

	handler := NewHandler(slog.New(slog.NewTextHandler(io.Discard, nil)), mustTestConfig(t), Dependencies{
		ClientAuthService: fakeClientAuthService{
			refreshFn: func(ctx context.Context, request api.RefreshTokenRequest, userAgent string) (*api.TokenResponse, error) {
				return nil, clientauth.ErrUnauthorized
			},
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/tokens/refresh", strings.NewReader(`{"refresh_token":"bad"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, rec.Code)
	}
}

func TestRefreshAccessTokenRateLimitedReturns429(t *testing.T) {
	t.Parallel()

	called := false
	handler := NewHandler(slog.New(slog.NewTextHandler(io.Discard, nil)), mustTestConfig(t), Dependencies{
		ClientAuthService: fakeClientAuthService{
			refreshFn: func(ctx context.Context, request api.RefreshTokenRequest, userAgent string) (*api.TokenResponse, error) {
				called = true
				return &api.TokenResponse{}, nil
			},
		},
		AuthSpamService: fakeAuthSpamService{
			checkRefreshTokenFn: func(ctx context.Context, clientInfo requestmeta.ClientInfo) error {
				return authspam.RateLimitedError{RetryAfter: 7 * time.Second}
			},
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/tokens/refresh", strings.NewReader(`{"refresh_token":"good"}`))
	req.RemoteAddr = "127.0.0.1:12345"
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected status %d, got %d", http.StatusTooManyRequests, rec.Code)
	}
	if rec.Header().Get("Retry-After") != "7" {
		t.Fatalf("expected Retry-After 7, got %q", rec.Header().Get("Retry-After"))
	}
	if called {
		t.Fatal("expected request to be blocked before refresh")
	}
}

func TestRefreshAccessTokenRateLimitLogsStructuredWarning(t *testing.T) {
	t.Parallel()

	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))

	handler := NewHandler(logger, mustTestConfig(t), Dependencies{
		ClientAuthService: fakeClientAuthService{},
		AuthSpamService: fakeAuthSpamService{
			checkRefreshTokenFn: func(ctx context.Context, clientInfo requestmeta.ClientInfo) error {
				return authspam.RateLimitedError{RetryAfter: 9 * time.Second}
			},
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/tokens/refresh", strings.NewReader(`{"refresh_token":"good"}`))
	req.RemoteAddr = "127.0.0.1:12345"
	req.Header.Set("X-Forwarded-For", "203.0.113.77, 127.0.0.1")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	output := logs.String()
	if !strings.Contains(output, `"msg":"auth rate limit exceeded"`) {
		t.Fatalf("expected auth rate limit log, got %s", output)
	}
	if !strings.Contains(output, `"scope":"refresh_token"`) {
		t.Fatalf("expected refresh scope log, got %s", output)
	}
	if !strings.Contains(output, `"retry_after_seconds":9`) {
		t.Fatalf("expected retry_after_seconds log, got %s", output)
	}
}

func TestLogoutRateLimitedReturns429(t *testing.T) {
	t.Parallel()

	called := false
	handler := NewHandler(slog.New(slog.NewTextHandler(io.Discard, nil)), mustTestConfig(t), Dependencies{
		ClientAuthService: fakeClientAuthService{
			logoutFn: func(ctx context.Context, request api.LogoutRequest) error {
				called = true
				return nil
			},
		},
		AuthSpamService: fakeAuthSpamService{
			checkLogoutFn: func(ctx context.Context, clientInfo requestmeta.ClientInfo) error {
				return authspam.RateLimitedError{RetryAfter: 8 * time.Second}
			},
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", strings.NewReader(`{"refresh_token":"good"}`))
	req.RemoteAddr = "127.0.0.1:12345"
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected status %d, got %d", http.StatusTooManyRequests, rec.Code)
	}
	if rec.Header().Get("Retry-After") != "8" {
		t.Fatalf("expected Retry-After 8, got %q", rec.Header().Get("Retry-After"))
	}
	if called {
		t.Fatal("expected request to be blocked before logout")
	}
}

func TestListProfileSessionsReturnsItems(t *testing.T) {
	t.Parallel()

	sessionID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	called := false
	handler := NewHandler(slog.New(slog.NewTextHandler(io.Discard, nil)), mustTestConfig(t), Dependencies{
		UserStore: fakeProfileUserStore{
			upsertFn: func(ctx context.Context, arg db.UpsertUserByUsernameParams) (db.User, error) {
				return db.User{
					ID:       pgtype.UUID{Bytes: [16]byte(sessionID), Valid: true},
					Username: arg.Username,
					Name:     arg.Name,
					Email:    arg.Email,
					Groups:   arg.Groups,
				}, nil
			},
		},
		SessionService: fakeSessionService{
			listProfilePageFn: func(ctx context.Context, user db.User, currentSessionID string, limit, offset int32) (*api.AuthSessionPage, error) {
				called = true
				if limit != defaultAuthSessionLimit {
					t.Fatalf("expected default limit %d, got %d", defaultAuthSessionLimit, limit)
				}
				if offset != 0 {
					t.Fatalf("expected offset 0, got %d", offset)
				}
				return &api.AuthSessionPage{
					Items: []api.AuthSession{{
						Id:            openapi_types.UUID(sessionID),
						Username:      user.Username,
						ClientName:    "internctl",
						CreatedAt:     testTimestamp().Time,
						ExpiresAt:     testTimestamp().Time.Add(time.Hour),
						IdleExpiresAt: testTimestamp().Time.Add(30 * time.Minute),
					}},
					Pagination: api.AuthSessionPagination{
						Limit:  defaultAuthSessionLimit,
						Offset: 0,
						Total:  1,
					},
				}, nil
			},
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/profile/sessions", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	setForwardAuthHeaders(req, "alice", "Alice Example", "alice@example.com", "Users")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if !called {
		t.Fatal("expected session service to be called")
	}
}

func TestListProfileSessionsRejectsBadLimit(t *testing.T) {
	t.Parallel()

	handler := NewHandler(slog.New(slog.NewTextHandler(io.Discard, nil)), mustTestConfig(t), Dependencies{
		UserStore: fakeProfileUserStore{
			upsertFn: func(ctx context.Context, arg db.UpsertUserByUsernameParams) (db.User, error) {
				return db.User{Username: arg.Username, Name: arg.Name, Email: arg.Email, Groups: arg.Groups}, nil
			},
		},
		SessionService: fakeSessionService{},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/profile/sessions?limit=999", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	setForwardAuthHeaders(req, "alice", "Alice Example", "alice@example.com", "Users")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestRevokeProfileSessionReturnsNoContent(t *testing.T) {
	t.Parallel()

	sessionID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	called := false
	handler := NewHandler(slog.New(slog.NewTextHandler(io.Discard, nil)), mustTestConfig(t), Dependencies{
		UserStore: fakeProfileUserStore{
			upsertFn: func(ctx context.Context, arg db.UpsertUserByUsernameParams) (db.User, error) {
				return db.User{Username: arg.Username, Name: arg.Name, Email: arg.Email, Groups: arg.Groups}, nil
			},
		},
		SessionService: fakeSessionService{
			revokeProfileFn: func(ctx context.Context, user db.User, id uuid.UUID) error {
				called = true
				if id != sessionID {
					t.Fatalf("expected session id %s, got %s", sessionID, id)
				}
				return nil
			},
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/profile/sessions/"+sessionID.String()+"/revoke", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	setForwardAuthHeaders(req, "alice", "Alice Example", "alice@example.com", "Users")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d", http.StatusNoContent, rec.Code)
	}
	if !called {
		t.Fatal("expected revoke profile session to be called")
	}
}

func TestListAdminSessionsRequiresAdmin(t *testing.T) {
	t.Parallel()

	handler := NewHandler(slog.New(slog.NewTextHandler(io.Discard, nil)), mustTestConfig(t), Dependencies{
		UserStore: fakeProfileUserStore{
			upsertFn: func(ctx context.Context, arg db.UpsertUserByUsernameParams) (db.User, error) {
				return db.User{Username: arg.Username, Name: arg.Name, Email: arg.Email, Groups: arg.Groups}, nil
			},
		},
		SessionService: fakeSessionService{},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/auth/sessions", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	setForwardAuthHeaders(req, "alice", "Alice Example", "alice@example.com", "Users")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, rec.Code)
	}
}

func TestListAdminSessionsReturnsItems(t *testing.T) {
	t.Parallel()

	sessionID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	handler := NewHandler(slog.New(slog.NewTextHandler(io.Discard, nil)), mustTestConfig(t), Dependencies{
		UserStore: fakeProfileUserStore{
			upsertFn: func(ctx context.Context, arg db.UpsertUserByUsernameParams) (db.User, error) {
				return db.User{Username: arg.Username, Name: arg.Name, Email: arg.Email, Groups: arg.Groups}, nil
			},
		},
		SessionService: fakeSessionService{
			listAdminPageFn: func(ctx context.Context, currentSessionID string, limit, offset int32) (*api.AuthSessionPage, error) {
				if limit != defaultAuthSessionLimit {
					t.Fatalf("expected default limit %d, got %d", defaultAuthSessionLimit, limit)
				}
				if offset != 0 {
					t.Fatalf("expected offset 0, got %d", offset)
				}
				return &api.AuthSessionPage{
					Items: []api.AuthSession{{
						Id:            openapi_types.UUID(sessionID),
						Username:      "alice",
						ClientName:    "internctl",
						CreatedAt:     testTimestamp().Time,
						ExpiresAt:     testTimestamp().Time.Add(time.Hour),
						IdleExpiresAt: testTimestamp().Time.Add(30 * time.Minute),
					}},
					Pagination: api.AuthSessionPagination{
						Limit:  defaultAuthSessionLimit,
						Offset: 0,
						Total:  1,
					},
				}, nil
			},
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/auth/sessions", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	setForwardAuthHeaders(req, "bob", "Bob Example", "bob@example.com", "Users, Super-Users")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
}

func TestListAdminSessionsRejectsBadLimit(t *testing.T) {
	t.Parallel()

	handler := NewHandler(slog.New(slog.NewTextHandler(io.Discard, nil)), mustTestConfig(t), Dependencies{
		UserStore: fakeProfileUserStore{
			upsertFn: func(ctx context.Context, arg db.UpsertUserByUsernameParams) (db.User, error) {
				return db.User{Username: arg.Username, Name: arg.Name, Email: arg.Email, Groups: arg.Groups}, nil
			},
		},
		SessionService: fakeSessionService{},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/auth/sessions?limit=999", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	setForwardAuthHeaders(req, "bob", "Bob Example", "bob@example.com", "Users, Super-Users")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestRevokeAllAdminSessionsRequiresAdmin(t *testing.T) {
	t.Parallel()

	handler := NewHandler(slog.New(slog.NewTextHandler(io.Discard, nil)), mustTestConfig(t), Dependencies{
		UserStore: fakeProfileUserStore{
			upsertFn: func(ctx context.Context, arg db.UpsertUserByUsernameParams) (db.User, error) {
				return db.User{Username: arg.Username, Name: arg.Name, Email: arg.Email, Groups: arg.Groups}, nil
			},
		},
		SessionService: fakeSessionService{},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/auth/sessions/revoke_all", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	setForwardAuthHeaders(req, "alice", "Alice Example", "alice@example.com", "Users")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, rec.Code)
	}
}

func TestRevokeAllAdminSessionsReturnsNoContent(t *testing.T) {
	t.Parallel()

	called := false
	handler := NewHandler(slog.New(slog.NewTextHandler(io.Discard, nil)), mustTestConfig(t), Dependencies{
		UserStore: fakeProfileUserStore{
			upsertFn: func(ctx context.Context, arg db.UpsertUserByUsernameParams) (db.User, error) {
				return db.User{Username: arg.Username, Name: arg.Name, Email: arg.Email, Groups: arg.Groups}, nil
			},
		},
		SessionService: fakeSessionService{
			revokeAllAdminFn: func(ctx context.Context) error {
				called = true
				return nil
			},
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/auth/sessions/revoke_all", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	setForwardAuthHeaders(req, "bob", "Bob Example", "bob@example.com", "Users, Super-Users")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d", http.StatusNoContent, rec.Code)
	}
	if !called {
		t.Fatal("expected global revoke to be called")
	}
}

func TestListAdminAuditLogsRequiresAdmin(t *testing.T) {
	t.Parallel()

	handler := NewHandler(slog.New(slog.NewTextHandler(io.Discard, nil)), mustTestConfig(t), Dependencies{
		UserStore: fakeProfileUserStore{
			upsertFn: func(ctx context.Context, arg db.UpsertUserByUsernameParams) (db.User, error) {
				return db.User{Username: arg.Username, Name: arg.Name, Email: arg.Email, Groups: arg.Groups}, nil
			},
		},
		AuditLogService: fakeAuditLogService{},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/audit_logs", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	setForwardAuthHeaders(req, "alice", "Alice Example", "alice@example.com", "Users")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, rec.Code)
	}
}

func TestListAdminAuditLogsRejectsBadLimit(t *testing.T) {
	t.Parallel()

	handler := NewHandler(slog.New(slog.NewTextHandler(io.Discard, nil)), mustTestConfig(t), Dependencies{
		UserStore: fakeProfileUserStore{
			upsertFn: func(ctx context.Context, arg db.UpsertUserByUsernameParams) (db.User, error) {
				return db.User{Username: arg.Username, Name: arg.Name, Email: arg.Email, Groups: arg.Groups}, nil
			},
		},
		AuditLogService: fakeAuditLogService{},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/audit_logs?limit=999", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	setForwardAuthHeaders(req, "bob", "Bob Example", "bob@example.com", "Users, Super-Users")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestListAdminAuditLogsReturnsPage(t *testing.T) {
	t.Parallel()

	handler := NewHandler(slog.New(slog.NewTextHandler(io.Discard, nil)), mustTestConfig(t), Dependencies{
		UserStore: fakeProfileUserStore{
			upsertFn: func(ctx context.Context, arg db.UpsertUserByUsernameParams) (db.User, error) {
				return db.User{Username: arg.Username, Name: arg.Name, Email: arg.Email, Groups: arg.Groups}, nil
			},
		},
		AuditLogService: fakeAuditLogService{
			listFn: func(ctx context.Context, filter auditlogs.Filter) (*auditlogs.Page, error) {
				if filter.Limit != 25 {
					t.Fatalf("expected limit 25, got %d", filter.Limit)
				}
				if filter.Offset != 50 {
					t.Fatalf("expected offset 50, got %d", filter.Offset)
				}
				if filter.Action != "device.update" {
					t.Fatalf("expected action filter to be passed through, got %q", filter.Action)
				}
				return &auditlogs.Page{
					Items: []api.AuditLogEntry{{
						Id:            openapi_types.UUID(uuid.MustParse("11111111-1111-1111-1111-111111111111")),
						ActorUsername: "bob",
						Action:        "device.update",
						ResourceType:  "device",
						ResourceId:    "11111111-1111-1111-1111-111111111111",
						Metadata: map[string]interface{}{
							"field": "mac_address",
						},
						CreatedAt: testTimestamp().Time,
					}},
					Limit:      25,
					Offset:     50,
					TotalCount: 101,
				}, nil
			},
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/audit_logs?action=device.update&limit=25&offset=50", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	setForwardAuthHeaders(req, "bob", "Bob Example", "bob@example.com", "Users, Super-Users")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
}

type fakeProfileUserStore struct {
	upsertFn func(ctx context.Context, arg db.UpsertUserByUsernameParams) (db.User, error)
}

type readOnlyProfileUserStore struct {
	upsertCalled bool
}

func (f *readOnlyProfileUserStore) UpsertUserByUsername(context.Context, db.UpsertUserByUsernameParams) (db.User, error) {
	f.upsertCalled = true
	return db.User{}, errors.New("profile lookup attempted a write")
}

func (f *readOnlyProfileUserStore) GetUserByUsername(context.Context, db.GetUserByUsernameParams) (db.User, error) {
	return db.User{
		Username: "alice",
		Name:     "Alice Example",
		Email:    "alice@example.com",
		Groups:   []string{"Users", "Super-Users"},
	}, nil
}

func (f fakeProfileUserStore) UpsertUserByUsername(ctx context.Context, arg db.UpsertUserByUsernameParams) (db.User, error) {
	return f.upsertFn(ctx, arg)
}

type fakeVLANService struct {
	listFn   func(ctx context.Context) ([]db.Vlan, error)
	getFn    func(ctx context.Context, vlanID int32) (db.Vlan, error)
	createFn func(ctx context.Context, actor db.User, input api.VlanWrite) (db.Vlan, error)
	updateFn func(ctx context.Context, actor db.User, vlanID int32, patch api.VlanPatch) (db.Vlan, error)
	deleteFn func(ctx context.Context, actor db.User, vlanID int32) error
}

func (f fakeVLANService) List(ctx context.Context) ([]db.Vlan, error) {
	return f.listFn(ctx)
}

func (f fakeVLANService) Get(ctx context.Context, vlanID int32) (db.Vlan, error) {
	return f.getFn(ctx, vlanID)
}

func (f fakeVLANService) Create(ctx context.Context, actor db.User, input api.VlanWrite) (db.Vlan, error) {
	return f.createFn(ctx, actor, input)
}

func (f fakeVLANService) Update(ctx context.Context, actor db.User, vlanID int32, patch api.VlanPatch) (db.Vlan, error) {
	return f.updateFn(ctx, actor, vlanID, patch)
}

func (f fakeVLANService) Delete(ctx context.Context, actor db.User, vlanID int32) error {
	return f.deleteFn(ctx, actor, vlanID)
}

func testTimestamp() pgtype.Timestamptz {
	return pgtype.Timestamptz{
		Time:  time.Date(2026, 3, 9, 12, 0, 0, 0, time.UTC),
		Valid: true,
	}
}

type fakeDeviceService struct {
	listFn   func(ctx context.Context) ([]devices.DeviceRecord, error)
	getFn    func(ctx context.Context, id uuid.UUID) (devices.DeviceRecord, error)
	createFn func(ctx context.Context, actor db.User, input api.NetworkDeviceWrite) (devices.DeviceRecord, error)
	updateFn func(ctx context.Context, actor db.User, id uuid.UUID, patch api.NetworkDevicePatch) (devices.DeviceRecord, error)
	deleteFn func(ctx context.Context, actor db.User, id uuid.UUID) error
}

func (f fakeDeviceService) List(ctx context.Context) ([]devices.DeviceRecord, error) {
	return f.listFn(ctx)
}

func (f fakeDeviceService) Get(ctx context.Context, id uuid.UUID) (devices.DeviceRecord, error) {
	return f.getFn(ctx, id)
}

func (f fakeDeviceService) Create(ctx context.Context, actor db.User, input api.NetworkDeviceWrite) (devices.DeviceRecord, error) {
	return f.createFn(ctx, actor, input)
}

func (f fakeDeviceService) Update(ctx context.Context, actor db.User, id uuid.UUID, patch api.NetworkDevicePatch) (devices.DeviceRecord, error) {
	return f.updateFn(ctx, actor, id, patch)
}

func (f fakeDeviceService) Delete(ctx context.Context, actor db.User, id uuid.UUID) error {
	return f.deleteFn(ctx, actor, id)
}

type fakeClientAuthService struct {
	createFn   func(ctx context.Context, request *api.DeviceCodeCreateRequest) (*api.DeviceCode, error)
	approveFn  func(ctx context.Context, userCode string, user db.User) error
	denyFn     func(ctx context.Context, userCode string, user db.User) error
	exchangeFn func(ctx context.Context, request api.DeviceCodeTokenRequest, userAgent string) (*api.TokenResponse, error)
	refreshFn  func(ctx context.Context, request api.RefreshTokenRequest, userAgent string) (*api.TokenResponse, error)
	logoutFn   func(ctx context.Context, request api.LogoutRequest) error
}

func (f fakeClientAuthService) CreateDeviceCode(ctx context.Context, request *api.DeviceCodeCreateRequest) (*api.DeviceCode, error) {
	if f.createFn == nil {
		return nil, nil
	}
	return f.createFn(ctx, request)
}

func (f fakeClientAuthService) ApproveDeviceCode(ctx context.Context, userCode string, user db.User) error {
	if f.approveFn == nil {
		return nil
	}
	return f.approveFn(ctx, userCode, user)
}

func (f fakeClientAuthService) DenyDeviceCode(ctx context.Context, userCode string, user db.User) error {
	if f.denyFn == nil {
		return nil
	}
	return f.denyFn(ctx, userCode, user)
}

func (f fakeClientAuthService) ExchangeDeviceCode(ctx context.Context, request api.DeviceCodeTokenRequest, userAgent string) (*api.TokenResponse, error) {
	if f.exchangeFn == nil {
		return nil, nil
	}
	return f.exchangeFn(ctx, request, userAgent)
}

func (f fakeClientAuthService) RefreshAccessToken(ctx context.Context, request api.RefreshTokenRequest, userAgent string) (*api.TokenResponse, error) {
	if f.refreshFn == nil {
		return nil, nil
	}
	return f.refreshFn(ctx, request, userAgent)
}

func (f fakeClientAuthService) Logout(ctx context.Context, request api.LogoutRequest) error {
	if f.logoutFn == nil {
		return nil
	}
	return f.logoutFn(ctx, request)
}

type fakeSessionService struct {
	validateSessionFn func(ctx context.Context, sessionID string) (bool, error)
	listProfilePageFn func(ctx context.Context, user db.User, currentSessionID string, limit, offset int32) (*api.AuthSessionPage, error)
	revokeProfileFn   func(ctx context.Context, user db.User, sessionID uuid.UUID) error
	revokeOthersFn    func(ctx context.Context, user db.User, currentSessionID string) error
	listAdminPageFn   func(ctx context.Context, currentSessionID string, limit, offset int32) (*api.AuthSessionPage, error)
	revokeAdminFn     func(ctx context.Context, sessionID uuid.UUID) error
	revokeAllAdminFn  func(ctx context.Context) error
}

func (f fakeSessionService) ValidateSession(ctx context.Context, sessionID string) (bool, error) {
	if f.validateSessionFn == nil {
		return true, nil
	}
	return f.validateSessionFn(ctx, sessionID)
}

func (f fakeSessionService) ListProfileSessionsPage(ctx context.Context, user db.User, currentSessionID string, limit, offset int32) (*api.AuthSessionPage, error) {
	if f.listProfilePageFn == nil {
		return &api.AuthSessionPage{}, nil
	}
	return f.listProfilePageFn(ctx, user, currentSessionID, limit, offset)
}

func (f fakeSessionService) RevokeProfileSession(ctx context.Context, user db.User, sessionID uuid.UUID) error {
	if f.revokeProfileFn == nil {
		return nil
	}
	return f.revokeProfileFn(ctx, user, sessionID)
}

func (f fakeSessionService) RevokeOtherProfileSessions(ctx context.Context, user db.User, currentSessionID string) error {
	if f.revokeOthersFn == nil {
		return nil
	}
	return f.revokeOthersFn(ctx, user, currentSessionID)
}

func (f fakeSessionService) ListAdminSessionsPage(ctx context.Context, currentSessionID string, limit, offset int32) (*api.AuthSessionPage, error) {
	if f.listAdminPageFn == nil {
		return &api.AuthSessionPage{}, nil
	}
	return f.listAdminPageFn(ctx, currentSessionID, limit, offset)
}

func (f fakeSessionService) RevokeAdminSession(ctx context.Context, sessionID uuid.UUID) error {
	if f.revokeAdminFn == nil {
		return nil
	}
	return f.revokeAdminFn(ctx, sessionID)
}

func (f fakeSessionService) RevokeAllAdminSessions(ctx context.Context) error {
	if f.revokeAllAdminFn == nil {
		return nil
	}
	return f.revokeAllAdminFn(ctx)
}

type fakeAuditLogService struct {
	listFn func(ctx context.Context, filter auditlogs.Filter) (*auditlogs.Page, error)
}

func (f fakeAuditLogService) List(ctx context.Context, filter auditlogs.Filter) (*auditlogs.Page, error) {
	if f.listFn == nil {
		return &auditlogs.Page{}, nil
	}
	return f.listFn(ctx, filter)
}

type fakeAuthSpamService struct {
	checkDeviceCodeCreateFn    func(ctx context.Context, clientInfo requestmeta.ClientInfo) error
	checkDeviceTokenExchangeFn func(ctx context.Context, clientInfo requestmeta.ClientInfo) error
	checkDeviceDecisionFn      func(ctx context.Context, username string, clientInfo requestmeta.ClientInfo) error
	checkRefreshTokenFn        func(ctx context.Context, clientInfo requestmeta.ClientInfo) error
	checkLogoutFn              func(ctx context.Context, clientInfo requestmeta.ClientInfo) error
}

func (f fakeAuthSpamService) CheckDeviceCodeCreate(ctx context.Context, clientInfo requestmeta.ClientInfo) error {
	if f.checkDeviceCodeCreateFn == nil {
		return nil
	}
	return f.checkDeviceCodeCreateFn(ctx, clientInfo)
}

func (f fakeAuthSpamService) CheckDeviceTokenExchange(ctx context.Context, clientInfo requestmeta.ClientInfo) error {
	if f.checkDeviceTokenExchangeFn == nil {
		return nil
	}
	return f.checkDeviceTokenExchangeFn(ctx, clientInfo)
}

func (f fakeAuthSpamService) CheckDeviceDecision(ctx context.Context, username string, clientInfo requestmeta.ClientInfo) error {
	if f.checkDeviceDecisionFn == nil {
		return nil
	}
	return f.checkDeviceDecisionFn(ctx, username, clientInfo)
}

func (f fakeAuthSpamService) CheckRefreshToken(ctx context.Context, clientInfo requestmeta.ClientInfo) error {
	if f.checkRefreshTokenFn == nil {
		return nil
	}
	return f.checkRefreshTokenFn(ctx, clientInfo)
}

func (f fakeAuthSpamService) CheckLogout(ctx context.Context, clientInfo requestmeta.ClientInfo) error {
	if f.checkLogoutFn == nil {
		return nil
	}
	return f.checkLogoutFn(ctx, clientInfo)
}

func mustTestConfig(t *testing.T) config.Config {
	t.Helper()

	cfg := config.Config{
		Server:   config.ServerConfig{Addr: ":8080"},
		Database: config.DatabaseConfig{URL: "postgres://postgres:postgres@127.0.0.1:5432/intern_test?sslmode=disable"},
		LogLevel: config.LogLevelInfo,
		Auth: config.AuthConfig{
			PublicBaseURL:      "https://intern.corp.example.com",
			JWTIssuer:          "intern.corp.example.com",
			JWTAudience:        "internctl",
			JWTHMACSecret:      "test-secret",
			AccessTokenTTL:     15 * time.Minute,
			RefreshIdleTTL:     30 * 24 * time.Hour,
			RefreshAbsoluteTTL: 90 * 24 * time.Hour,
			DeviceCodeTTL:      10 * time.Minute,
			DevicePollInterval: 5 * time.Second,
			RateLimit: config.AuthRateLimitConfig{
				DeviceCodeCreate:    config.AuthRateLimitRule{Limit: 10, Window: time.Minute},
				DeviceTokenExchange: config.AuthRateLimitRule{Limit: 120, Window: time.Minute},
				DeviceDecision:      config.AuthRateLimitRule{Limit: 30, Window: time.Minute},
				RefreshToken:        config.AuthRateLimitRule{Limit: 60, Window: time.Minute},
				Logout:              config.AuthRateLimitRule{Limit: 60, Window: time.Minute},
			},
		},
		TrustedProxy: config.TrustedProxyConfig{
			CIDRs:        []netip.Prefix{netip.MustParsePrefix("127.0.0.1/32")},
			UserHeader:   "Remote-User",
			NameHeader:   "Remote-Name",
			EmailHeader:  "Remote-Email",
			GroupsHeader: "Remote-Groups",
			MarkerHeader: "X-Intern-Forward-Auth",
			MarkerValue:  "authenticated-ingress",
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

func mintTestAccessToken(t *testing.T, cfg config.Config, claims auth.AccessTokenClaims) string {
	t.Helper()

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, err := token.SignedString([]byte(cfg.Auth.JWTHMACSecret))
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}

	return signedToken
}

func mintBearerAccessTokenForTest(t *testing.T, cfg config.Config, username string, groups []string, sessionID string) string {
	t.Helper()

	now := time.Now()
	return mintTestAccessToken(t, cfg, auth.AccessTokenClaims{
		Username:  username,
		Groups:    append([]string(nil), groups...),
		SessionID: sessionID,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   username,
			Issuer:    cfg.Auth.JWTIssuer,
			Audience:  jwt.ClaimStrings{cfg.Auth.JWTAudience},
			ExpiresAt: jwt.NewNumericDate(now.Add(15 * time.Minute)),
			IssuedAt:  jwt.NewNumericDate(now),
		},
	})
}

func setForwardAuthHeaders(req *http.Request, username, name, email, groups string) {
	req.Header.Set("Remote-User", username)
	if name != "" {
		req.Header.Set("Remote-Name", name)
	}
	if email != "" {
		req.Header.Set("Remote-Email", email)
	}
	if groups != "" {
		req.Header.Set("Remote-Groups", groups)
	}
	req.Header.Set("X-Intern-Forward-Auth", "authenticated-ingress")
}
