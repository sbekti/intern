//go:build integration

package clientauth

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/sbekti/intern-api/internal/api"
	"github.com/sbekti/intern-api/internal/config"
	"github.com/sbekti/intern-api/internal/db"
	"github.com/sbekti/intern-api/internal/testutil"
)

func TestServiceDeviceCodeFlow(t *testing.T) {
	t.Parallel()

	pg := testutil.StartPostgres(t)
	ctx := context.Background()
	queries := db.New(pg.Pool)

	user, err := queries.UpsertUserByUsername(ctx, db.UpsertUserByUsernameParams{
		Username: "alice",
		Name:     "Alice Example",
		Email:    "alice@example.com",
		Groups:   []string{"Users"},
	})
	if err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	service := NewService(integrationConfig(), queries, NewPGXTransactor(pg.Pool))
	service.now = func() time.Time {
		return time.Date(2026, 3, 9, 12, 0, 0, 0, time.UTC)
	}
	service.random = bytes.NewReader(bytes.Repeat([]byte("integration-seed-"), 32))

	deviceCode, err := service.CreateDeviceCode(ctx, &api.DeviceCodeCreateRequest{})
	if err != nil {
		t.Fatalf("expected device code creation to succeed, got %v", err)
	}
	if deviceCode.DeviceCode == "" || deviceCode.UserCode == "" {
		t.Fatalf("expected populated device code response, got %#v", deviceCode)
	}

	if err := service.ApproveDeviceCode(ctx, deviceCode.UserCode, user); err != nil {
		t.Fatalf("expected approval to succeed, got %v", err)
	}

	token, err := service.ExchangeDeviceCode(ctx, api.DeviceCodeTokenRequest{
		DeviceCode: deviceCode.DeviceCode,
	}, "internctl/test")
	if err != nil {
		t.Fatalf("expected token exchange to succeed, got %v", err)
	}
	if token.AccessToken == "" || token.RefreshToken == "" {
		t.Fatalf("expected token response, got %#v", token)
	}

	authzRecord, err := queries.GetAuthDeviceAuthorizationByDeviceCode(ctx, db.GetAuthDeviceAuthorizationByDeviceCodeParams{
		DeviceCode: deviceCode.DeviceCode,
	})
	if err != nil {
		t.Fatalf("failed to load auth device record: %v", err)
	}
	if authzRecord.Status != "exchanged" {
		t.Fatalf("expected exchanged status, got %q", authzRecord.Status)
	}

	refreshed, err := service.RefreshAccessToken(ctx, api.RefreshTokenRequest{
		RefreshToken: token.RefreshToken,
	}, "internctl/test")
	if err != nil {
		t.Fatalf("expected refresh to succeed, got %v", err)
	}
	if refreshed.AccessToken == "" || refreshed.RefreshToken == "" || refreshed.RefreshToken == token.RefreshToken {
		t.Fatalf("expected rotated tokens, got %#v", refreshed)
	}

	sessions, err := queries.ListAuthSessionsByUserID(ctx, db.ListAuthSessionsByUserIDParams{UserID: user.ID})
	if err != nil {
		t.Fatalf("failed to list auth sessions: %v", err)
	}
	if len(sessions) != 2 {
		t.Fatalf("expected 2 auth sessions after refresh rotation, got %d", len(sessions))
	}

	rotatedCount := 0
	for _, session := range sessions {
		if session.RevokeReason == "rotated" {
			rotatedCount++
		}
	}
	if rotatedCount != 1 {
		t.Fatalf("expected 1 rotated session, got %d", rotatedCount)
	}

	if err := service.Logout(ctx, api.LogoutRequest{RefreshToken: refreshed.RefreshToken}); err != nil {
		t.Fatalf("expected logout to succeed, got %v", err)
	}

	loggedOut, err := queries.GetAuthSessionByRefreshTokenHash(ctx, db.GetAuthSessionByRefreshTokenHashParams{
		RefreshTokenHash: hashToken(refreshed.RefreshToken),
	})
	if err != nil {
		t.Fatalf("failed to load refreshed session: %v", err)
	}
	if !loggedOut.RevokedAt.Valid || loggedOut.RevokeReason != "logout" {
		t.Fatalf("expected logout revocation, got %#v", loggedOut)
	}
}

func integrationConfig() config.Config {
	return config.Config{
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
	}
}
