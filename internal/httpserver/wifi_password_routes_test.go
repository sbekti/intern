package httpserver

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/sbekti/intern/internal/db"
	"github.com/sbekti/intern/internal/devices"
)

func TestWiFiPasswordEndpointsRequireAdminAndNeverCache(t *testing.T) {
	id := uuid.New()
	const password = "forest-river-cloud-moon"
	calls := 0
	handler := NewHandler(slog.New(slog.NewTextHandler(io.Discard, nil)), mustTestConfig(t), Dependencies{
		UserStore: fakeProfileUserStore{upsertFn: func(ctx context.Context, arg db.UpsertUserByUsernameParams) (db.User, error) {
			return db.User{ID: pgtype.UUID{Bytes: [16]byte(id), Valid: true}, Username: arg.Username, Groups: arg.Groups}, nil
		}},
		DeviceService: fakeDeviceService{revealFn: func(context.Context, db.User, uuid.UUID) (string, error) { calls++; return password, nil }},
	})
	for _, path := range []string{"/api/v1/networks/devices/" + id.String() + "/wifi-password/reveal", "/api/v1/networks/devices/wifi-password/generate"} {
		for _, group := range []string{"", "Users", "Super-Users"} {
			req := httptest.NewRequest(http.MethodPost, path, nil)
			req.RemoteAddr = "127.0.0.1:12345"
			if group != "" {
				setForwardAuthHeaders(req, "alice", "Alice", "alice@example.test", group)
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			want := http.StatusOK
			if group == "" {
				want = http.StatusUnauthorized
			} else if group == "Users" {
				want = http.StatusForbidden
			}
			if rec.Code != want {
				t.Fatalf("%s group=%q got status %d, want %d", path, group, rec.Code, want)
			}
			if rec.Header().Get("Cache-Control") != "no-store" {
				t.Fatal("password response could be cached")
			}
			if want != http.StatusOK && strings.Contains(rec.Body.String(), password) {
				t.Fatal("unauthorized response contains password")
			}
		}
	}
	if calls != 1 {
		t.Fatalf("reveal called %d times, want only authorized call", calls)
	}
}

func TestOrdinaryDeviceResponseContainsOnlyCredentialMetadata(t *testing.T) {
	device := toAPINetworkDevice(devices.DeviceRecord{Device: db.NetworkDevice{
		PrivateWifiPasswordEnabled: true, WifiPasswordCiphertext: []byte("test-credential-ciphertext"),
	}})
	data, err := json.Marshal(device)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "ciphertext") || strings.Contains(string(data), `"wifi_password":`) {
		t.Fatal("ordinary response exposes a credential")
	}
	if !device.PrivateWifiPasswordEnabled || !device.WifiPasswordSet {
		t.Fatal("credential metadata missing")
	}
}
