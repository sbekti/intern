package config

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/netip"
	"testing"
	"time"
)

func testConfig(secret string) Config {
	return Config{
		WiFiPasswordEncryptionKey: make([]byte, 32),
		RADIUSSnapshotMaxBytes:    4096,
		Server:                    ServerConfig{Addr: ":8080"},
		Database:                  DatabaseConfig{URL: "postgres://localhost/intern"},
		LogLevel:                  LogLevelInfo,
		Auth: AuthConfig{
			PublicBaseURL:      "https://intern.example.test",
			JWTIssuer:          "intern.example.test",
			JWTAudience:        "internctl",
			JWTHMACSecret:      secret,
			AccessTokenTTL:     15 * time.Minute,
			RefreshIdleTTL:     24 * time.Hour,
			RefreshAbsoluteTTL: 7 * 24 * time.Hour,
			DeviceCodeTTL:      10 * time.Minute,
			DevicePollInterval: 5 * time.Second,
			RateLimit: AuthRateLimitConfig{
				DeviceCodeCreate:    AuthRateLimitRule{Limit: 10, Window: time.Minute},
				DeviceTokenExchange: AuthRateLimitRule{Limit: 10, Window: time.Minute},
				DeviceDecision:      AuthRateLimitRule{Limit: 10, Window: time.Minute},
				RefreshToken:        AuthRateLimitRule{Limit: 10, Window: time.Minute},
				Logout:              AuthRateLimitRule{Limit: 10, Window: time.Minute},
			},
		},
		TrustedProxy: TrustedProxyConfig{
			CIDRs:        []netip.Prefix{netip.MustParsePrefix("127.0.0.1/32")},
			UserHeader:   "Remote-User",
			MarkerHeader: "X-Intern-Forward-Auth",
			MarkerValue:  "configured-marker",
		},
		Authorization: AuthorizationConfig{AdminGroups: []string{"Super-Users"}},
	}
}

func TestLoadWiFiPasswordKeyIsRequiredAndValidated(t *testing.T) {
	for key, value := range map[string]string{
		"INTERN_API_DATABASE_URL": "postgres://localhost/intern_test",
		"AUTH_PUBLIC_BASE_URL":    "https://example.test", "AUTH_JWT_ISSUER": "example.test",
		"AUTH_JWT_AUDIENCE": "test", "AUTH_JWT_HMAC_SECRET": "test-jwt-secret",
		"AUTH_FORWARD_AUTH_MARKER_HEADER": "X-Test-Auth", "AUTH_FORWARD_AUTH_MARKER_VALUE": "test-marker",
		"RADIUS_SNAPSHOT_TOKEN_HASHES": "",
		"RADIUS_SNAPSHOT_MAX_BYTES":    "4096",
	} {
		t.Setenv(key, value)
	}
	valid := bytes.Repeat([]byte{7}, 32)
	for _, value := range []string{"", "not base64", base64.StdEncoding.EncodeToString(valid[:31]), base64.StdEncoding.EncodeToString(valid)} {
		t.Setenv("WIFI_PASSWORD_ENCRYPTION_KEY", value)
		cfg, err := Load()
		if value == base64.StdEncoding.EncodeToString(valid) {
			if err != nil || !bytes.Equal(cfg.WiFiPasswordEncryptionKey, valid) || cfg.RADIUSSnapshotMaxBytes != 4096 {
				t.Fatal("valid key was not loaded")
			}
		} else if err == nil {
			t.Fatal("accepted missing or invalid encryption key")
		}
	}
}
func TestValidateRequiresSecurityConfiguration(t *testing.T) {
	t.Parallel()
	if err := testConfig("secret").Validate(); err != nil {
		t.Fatalf("valid config returned %v", err)
	}
	for name, mutate := range map[string]func(*Config){
		"Wi-Fi encryption key": func(cfg *Config) { cfg.WiFiPasswordEncryptionKey = nil },
		"snapshot size limit":  func(cfg *Config) { cfg.RADIUSSnapshotMaxBytes = 0 },
		"database":             func(cfg *Config) { cfg.Database.URL = "" },
		"issuer":               func(cfg *Config) { cfg.Auth.JWTIssuer = "" },
		"audience":             func(cfg *Config) { cfg.Auth.JWTAudience = "" },
		"secret":               func(cfg *Config) { cfg.Auth.JWTHMACSecret = "" },
		"marker":               func(cfg *Config) { cfg.TrustedProxy.MarkerValue = "" },
	} {
		cfg := testConfig("secret")
		mutate(&cfg)
		if err := cfg.Validate(); err == nil {
			t.Errorf("%s configuration unexpectedly validated", name)
		}
	}
}

func TestLoadDoesNotProvideInsecureJWTDefaults(t *testing.T) {
	keys := []string{"INTERN_API_DATABASE_URL", "AUTH_PUBLIC_BASE_URL", "AUTH_JWT_ISSUER", "AUTH_JWT_AUDIENCE", "AUTH_JWT_HMAC_SECRET", "AUTH_FORWARD_AUTH_MARKER_HEADER", "AUTH_FORWARD_AUTH_MARKER_VALUE"}
	for _, key := range keys {
		t.Setenv(key, "")
	}
	if _, err := Load(); err == nil {
		t.Fatal("expected missing required configuration to fail closed")
	}
}

func TestEnvRADIUSTokenHashes(t *testing.T) {
	first := sha256.Sum256([]byte("first-token"))
	second := sha256.Sum256([]byte("second-token"))
	t.Setenv("RADIUS_SNAPSHOT_TOKEN_HASHES", fmt.Sprintf("site-one=%x,site-two=%x", first, second))

	hashes, err := envRADIUSTokenHashes("RADIUS_SNAPSHOT_TOKEN_HASHES")
	if err != nil {
		t.Fatalf("parse token hashes: %v", err)
	}
	if len(hashes) != 2 || hashes[0].Site != "site-one" || hashes[0].SHA256 != first || hashes[1].Site != "site-two" || hashes[1].SHA256 != second {
		t.Fatalf("unexpected token hashes: %#v", hashes)
	}
}

func TestEnvRADIUSTokenHashesRejectsInvalidEntries(t *testing.T) {
	for name, value := range map[string]string{
		"missing site":   "=0123",
		"invalid hash":   "site-one=0123",
		"duplicate site": fmt.Sprintf("site-one=%064x,site-one=%064x", 1, 2),
		"reused token":   fmt.Sprintf("site-one=%064x,site-two=%064x", 1, 1),
	} {
		t.Run(name, func(t *testing.T) {
			t.Setenv("RADIUS_SNAPSHOT_TOKEN_HASHES", value)
			if _, err := envRADIUSTokenHashes("RADIUS_SNAPSHOT_TOKEN_HASHES"); err == nil {
				t.Fatal("invalid token hashes unexpectedly parsed")
			}
		})
	}
}
