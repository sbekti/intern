package httpserver

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sbekti/intern/internal/config"
	"github.com/sbekti/intern/internal/db"
	"github.com/sbekti/intern/internal/devices"
)

const testRADIUSMABToken = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

func TestRADIUSMABSnapshot(t *testing.T) {
	t.Parallel()

	records := []devices.DeviceRecord{
		{Device: db.NetworkDevice{MacAddress: "AA:BB:CC:DD:EE:02", DisplayName: "must-not-leak", VlanID: 20}},
		{Device: db.NetworkDevice{MacAddress: "AA:BB:CC:DD:EE:03", VlanID: 30, Disabled: true}},
		{Device: db.NetworkDevice{MacAddress: "AA:BB:CC:DD:EE:01", VlanID: 10}},
	}
	handler := newRADIUSMABTestHandler(t, records)

	req := httptest.NewRequest(http.MethodGet, radiusMABSnapshotPath, nil)
	req.Header.Set("Authorization", "Bearer "+testRADIUSMABToken)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	entries :=
		"aabbccddee01 Cleartext-Password := \"aabbccddee01\"\n" +
			"\tTunnel-Type := VLAN,\n\tTunnel-Medium-Type := IEEE-802,\n\tTunnel-Private-Group-Id := \"10\"\n\n" +
			"aabbccddee02 Cleartext-Password := \"aabbccddee02\"\n" +
			"\tTunnel-Type := VLAN,\n\tTunnel-Medium-Type := IEEE-802,\n\tTunnel-Private-Group-Id := \"20\"\n\n"
	want := expectedRADIUSMABSnapshot(entries)
	if rec.Body.String() != want {
		t.Fatalf("body = %q, want %q", rec.Body.String(), want)
	}
	if got := rec.Header().Get("Content-Type"); got != "text/plain; charset=utf-8" {
		t.Fatalf("content type = %q", got)
	}
	if strings.Contains(rec.Body.String(), "must-not-leak") {
		t.Fatal("snapshot leaked display name")
	}
}

func TestRADIUSMABSnapshotEmpty(t *testing.T) {
	t.Parallel()

	handler := newRADIUSMABTestHandler(t, nil)
	req := httptest.NewRequest(http.MethodGet, radiusMABSnapshotPath, nil)
	req.Header.Set("Authorization", "Bearer "+testRADIUSMABToken)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	want := expectedRADIUSMABSnapshot("")
	if rec.Code != http.StatusOK || rec.Body.String() != want {
		t.Fatalf("status/body = %d/%q, want %d/%q", rec.Code, rec.Body.String(), http.StatusOK, want)
	}
}

func TestRADIUSMABSnapshotTokenIsRouteSpecific(t *testing.T) {
	t.Parallel()

	handler := newRADIUSMABTestHandler(t, nil)

	for name, request := range map[string]struct {
		method, path, token string
		wantStatus          int
	}{
		"missing token":      {http.MethodGet, radiusMABSnapshotPath, "", http.StatusUnauthorized},
		"wrong token":        {http.MethodGet, radiusMABSnapshotPath, "wrong", http.StatusUnauthorized},
		"human route":        {http.MethodGet, "/api/v1/networks/devices", testRADIUSMABToken, http.StatusUnauthorized},
		"human write route":  {http.MethodPost, "/api/v1/networks/devices", testRADIUSMABToken, http.StatusUnauthorized},
		"unsupported method": {http.MethodPost, radiusMABSnapshotPath, testRADIUSMABToken, http.StatusUnauthorized},
	} {
		t.Run(name, func(t *testing.T) {
			req := httptest.NewRequest(request.method, request.path, nil)
			if request.token != "" {
				req.Header.Set("Authorization", "Bearer "+request.token)
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != request.wantStatus {
				t.Fatalf("status = %d, want %d for %s %s", rec.Code, request.wantStatus, request.method, request.path)
			}
		})
	}
}

func newRADIUSMABTestHandler(t *testing.T, records []devices.DeviceRecord) http.Handler {
	t.Helper()
	cfg := mustTestConfig(t)
	hash := sha256.Sum256([]byte(testRADIUSMABToken))
	cfg.RADIUSMAB.TokenHashes = []config.RADIUSMABTokenHash{{Site: "site-one", SHA256: hash}}
	return NewHandler(slog.New(slog.NewTextHandler(io.Discard, nil)), cfg, Dependencies{
		DeviceService: fakeDeviceService{listFn: func(_ context.Context) ([]devices.DeviceRecord, error) {
			return records, nil
		}},
	})
}

func expectedRADIUSMABSnapshot(entries string) string {
	revision := sha256.Sum256([]byte(entries))
	return fmt.Sprintf("# radius-mab-v1\n# revision=%x\n%s", revision, entries)
}
