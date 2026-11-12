package httpserver

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sbekti/intern/internal/config"
	"github.com/sbekti/intern/internal/devices"
)

const testRADIUSSnapshotToken = "separate-snapshot-download-token"

func TestRADIUSPolicyAuthorizationAndNoStore(t *testing.T) {
	cfg := mustTestConfig(t)
	cfg.RADIUSMAB.TokenHashes = []config.RADIUSTokenHash{{Site: "site-one", SHA256: sha256.Sum256([]byte(testRADIUSMABToken))}}
	cfg.RADIUSPolicy.TokenHashes = []config.RADIUSTokenHash{{Site: "site-one", SHA256: sha256.Sum256([]byte(testRADIUSSnapshotToken))}}
	var logs bytes.Buffer
	calls := 0
	handler := NewHandler(slog.New(slog.NewTextHandler(&logs, nil)), cfg, Dependencies{DeviceService: fakeDeviceService{
		policyFn: func(context.Context) ([]devices.RADIUSPolicyRecord, error) {
			calls++
			return []devices.RADIUSPolicyRecord{{MACAddress: "02:00:00:00:00:01", VLANID: 100, PrivatePassword: "secret-policy-body"}}, nil
		},
	}})
	for _, token := range []string{"", "wrong-token", testRADIUSMABToken, testRADIUSSnapshotToken} {
		req := httptest.NewRequest(http.MethodGet, radiusPolicySnapshotPath, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		want := http.StatusUnauthorized
		if token == testRADIUSSnapshotToken {
			want = http.StatusOK
		}
		if rec.Code != want || rec.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("policy authorization or no-store behavior is incorrect")
		}
		if want == http.StatusOK && !strings.Contains(rec.Body.String(), "secret-policy-body") {
			t.Fatal("policy body was changed")
		}
	}
	if calls != 1 || strings.Contains(logs.String(), "secret-policy-body") || strings.Contains(logs.String(), testRADIUSSnapshotToken) {
		t.Fatal("unauthorized policy access or secret in request logs")
	}
	if !strings.Contains(logs.String(), "site=site-one") {
		t.Fatal("download is not attributed to its configured site")
	}
	for _, path := range []string{radiusMABSnapshotPath, "/api/v1/networks/devices", "/api/v1/networks/devices/wifi-password/generate"} {
		method := http.MethodGet
		if strings.HasSuffix(path, "/generate") {
			method = http.MethodPost
		}
		req := httptest.NewRequest(method, path, nil)
		req.Header.Set("Authorization", "Bearer "+testRADIUSSnapshotToken)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatal("policy token was accepted by another route")
		}
	}
}

func TestRADIUSPolicyBuildFailureReturnsNoPolicy(t *testing.T) {
	cfg := mustTestConfig(t)
	cfg.RADIUSPolicy.TokenHashes = []config.RADIUSTokenHash{{Site: "site-one", SHA256: sha256.Sum256([]byte(testRADIUSSnapshotToken))}}
	handler := NewHandler(nil, cfg, Dependencies{DeviceService: fakeDeviceService{
		policyFn: func(context.Context) ([]devices.RADIUSPolicyRecord, error) {
			return nil, errors.New("policy unavailable")
		},
	}})
	req := httptest.NewRequest(http.MethodGet, radiusPolicySnapshotPath, nil)
	req.Header.Set("Authorization", "Bearer "+testRADIUSSnapshotToken)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable || rec.Header().Get("Cache-Control") != "no-store" || strings.Contains(rec.Body.String(), "revision=") {
		t.Fatal("policy generation failed without a safe, uncached error")
	}
}

func TestRADIUSPolicyRejectsTheConfiguredSizeLimit(t *testing.T) {
	records := []devices.RADIUSPolicyRecord{{MACAddress: "02:00:00:00:00:01", VLANID: 100, PrivatePassword: "forest-river-cloud-moon"}}
	size := int64(len(renderRADIUSSnapshot("radius-policy-v1", records)))
	for _, limit := range []int64{size - 1, size, size + 1} {
		cfg := mustTestConfig(t)
		cfg.RADIUSSnapshotMaxBytes = limit
		cfg.RADIUSPolicy.TokenHashes = []config.RADIUSTokenHash{{Site: "site-one", SHA256: sha256.Sum256([]byte(testRADIUSSnapshotToken))}}
		handler := NewHandler(nil, cfg, Dependencies{DeviceService: fakeDeviceService{
			policyFn: func(context.Context) ([]devices.RADIUSPolicyRecord, error) { return records, nil },
		}})
		request := httptest.NewRequest(http.MethodGet, radiusPolicySnapshotPath, nil)
		request.Header.Set("Authorization", "Bearer "+testRADIUSSnapshotToken)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if size < limit {
			if response.Code != http.StatusOK {
				t.Fatal("policy below the configured limit was rejected")
			}
		} else if response.Code != http.StatusServiceUnavailable || strings.Contains(response.Body.String(), "revision=") {
			t.Fatal("oversized policy was not rejected without returning partial content")
		}
	}
}
