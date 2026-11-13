//go:build integration

package httpserver

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sbekti/intern/internal/api"
	"github.com/sbekti/intern/internal/config"
	"github.com/sbekti/intern/internal/db"
	"github.com/sbekti/intern/internal/devices"
)

func TestHandlerIntegrationRADIUSPolicy(t *testing.T) {
	env := newHandlerIntegrationEnv(t)
	ctx := context.Background()
	// The native protocol fixture uses Trusted 100; the test schema starts at 1.
	if _, err := env.pg.Pool.Exec(ctx, `INSERT INTO vlans (vlan_id, name) VALUES (100, 'trusted-wifi')`); err != nil {
		t.Fatal(err)
	}
	cipher, err := devices.NewPasswordCipher(env.cfg.WiFiPasswordEncryptionKey)
	if err != nil {
		t.Fatal(err)
	}
	env.cfg.RADIUSPolicy.TokenHashes = []config.RADIUSTokenHash{{Site: "integration", SHA256: sha256.Sum256([]byte(testRADIUSSnapshotToken))}}
	service := devices.NewService(env.queries, devices.NewPGXTransactor(env.pg.Pool), cipher)
	handler := NewHandler(slog.New(slog.NewTextHandler(io.Discard, nil)), env.cfg, Dependencies{UserStore: env.queries, DeviceService: service})
	password := `private-$ENV{PATH}-%{User-Name}-'"\`
	privatePasswordEnabled := true
	input := api.NetworkDeviceWrite{DisplayName: "Private", MacAddress: "02:00:00:00:00:01", VlanId: 100,
		PrivateWifiPasswordEnabled: &privatePasswordEnabled, WifiPassword: &password}
	body, _ := json.Marshal(input)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/networks/devices", bytes.NewReader(body))
	req.RemoteAddr = "127.0.0.1:12345"
	setForwardAuthHeaders(req, "alice", "Alice", "alice@example.test", "Super-Users")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated || strings.Contains(rec.Body.String(), password) {
		t.Fatal("private device creation failed or returned its password")
	}
	var device api.NetworkDevice
	decodeBody(t, rec.Body, &device)
	actor, err := env.queries.GetUserByUsername(ctx, db.GetUserByUsernameParams{Username: "alice"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Create(ctx, actor, api.NetworkDeviceWrite{DisplayName: "Shared", MacAddress: "02:00:00:00:00:02", VlanId: 20}); err != nil {
		t.Fatal(err)
	}
	download := func(path, token string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return rec
	}
	policy := download(radiusPolicySnapshotPath, testRADIUSSnapshotToken)
	if policy.Code != http.StatusOK || policy.Header().Get("Cache-Control") != "no-store" || !strings.Contains(policy.Body.String(), "Tunnel-Password :=") {
		t.Fatal("authenticated private-policy download failed")
	}
	again := download(radiusPolicySnapshotPath, testRADIUSSnapshotToken)
	if !bytes.Equal(policy.Body.Bytes(), again.Body.Bytes()) {
		t.Fatal("unchanged inventory changed the revision")
	}
	mab := download(radiusMABSnapshotPath, integrationRADIUSMABToken)
	if mab.Code != http.StatusOK || strings.Contains(mab.Body.String(), "Tunnel-Password") {
		t.Fatal("private passwords altered the credential-free MAB contract")
	}
	if script := os.Getenv("RADIUS_NATIVE_TEST_SCRIPT"); script != "" {
		image := os.Getenv("RADIUS_NATIVE_TEST_IMAGE")
		if image == "" {
			t.Fatal("set RADIUS_NATIVE_TEST_IMAGE with RADIUS_NATIVE_TEST_SCRIPT")
		}
		directory := t.TempDir()
		policyPath, passwordPath := filepath.Join(directory, "policy-users"), filepath.Join(directory, "private-password")
		if err := os.WriteFile(policyPath, policy.Body.Bytes(), 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(passwordPath, []byte(password), 0600); err != nil {
			t.Fatal(err)
		}
		command := exec.CommandContext(ctx, script, image, policyPath, passwordPath)
		output, err := command.CombinedOutput()
		if err != nil {
			t.Fatalf("native RADIUS protocol checks failed: %v\n%s", err, output)
		}
		t.Log(string(output))
	}
	privatePasswordEnabled = false
	if _, err := service.Update(ctx, actor, device.Id, api.NetworkDevicePatch{PrivateWifiPasswordEnabled: &privatePasswordEnabled}); err != nil {
		t.Fatal(err)
	}
	shared := download(radiusPolicySnapshotPath, testRADIUSSnapshotToken)
	if shared.Code != http.StatusOK || strings.Contains(shared.Body.String(), "Tunnel-Password") || bytes.Equal(policy.Body.Bytes(), shared.Body.Bytes()) {
		t.Fatal("disabling private access did not remove the delivered password")
	}
	privatePasswordEnabled = true
	if _, err := service.Update(ctx, actor, device.Id, api.NetworkDevicePatch{PrivateWifiPasswordEnabled: &privatePasswordEnabled}); err != nil {
		t.Fatal(err)
	}
	if _, err := env.pg.Pool.Exec(ctx, `UPDATE network_devices SET wifi_password_ciphertext=$1 WHERE id=$2`, []byte{1, 2, 3}, device.Id); err != nil {
		t.Fatal(err)
	}
	unavailable := download(radiusPolicySnapshotPath, testRADIUSSnapshotToken)
	if unavailable.Code != http.StatusServiceUnavailable || strings.Contains(unavailable.Body.String(), "revision=") {
		t.Fatal("invalid ciphertext returned a partial policy instead of retaining last-good behavior")
	}
}
