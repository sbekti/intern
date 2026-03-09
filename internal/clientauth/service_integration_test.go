//go:build integration

package clientauth

import (
	"bytes"
	"context"
	"errors"
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
	user := createIntegrationUser(t, ctx, queries, "alice", []string{"Users"})

	service := newIntegrationService(pg, queries)
	now := service.now()

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
	if !authzRecord.LastPolledAt.Valid || !authzRecord.LastPolledAt.Time.Equal(now.UTC()) {
		t.Fatalf("expected last_polled_at to be updated, got %#v", authzRecord.LastPolledAt)
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

func TestServiceRefreshTokenReuseRevokesFamily(t *testing.T) {
	t.Parallel()

	pg := testutil.StartPostgres(t)
	ctx := context.Background()
	queries := db.New(pg.Pool)
	user := createIntegrationUser(t, ctx, queries, "alice", []string{"Users"})
	service := newIntegrationService(pg, queries)

	deviceCode, err := service.CreateDeviceCode(ctx, nil)
	if err != nil {
		t.Fatalf("failed to create device code: %v", err)
	}
	if err := service.ApproveDeviceCode(ctx, deviceCode.UserCode, user); err != nil {
		t.Fatalf("failed to approve device code: %v", err)
	}

	token, err := service.ExchangeDeviceCode(ctx, api.DeviceCodeTokenRequest{DeviceCode: deviceCode.DeviceCode}, "internctl/test")
	if err != nil {
		t.Fatalf("failed to exchange device code: %v", err)
	}

	refreshed, err := service.RefreshAccessToken(ctx, api.RefreshTokenRequest{RefreshToken: token.RefreshToken}, "internctl/test")
	if err != nil {
		t.Fatalf("failed to refresh token: %v", err)
	}

	_, err = service.RefreshAccessToken(ctx, api.RefreshTokenRequest{RefreshToken: token.RefreshToken}, "internctl/test")
	if !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("expected ErrUnauthorized on reused refresh token, got %v", err)
	}

	sessions, err := queries.ListAuthSessionsByUserID(ctx, db.ListAuthSessionsByUserIDParams{UserID: user.ID})
	if err != nil {
		t.Fatalf("failed to load sessions: %v", err)
	}
	if len(sessions) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(sessions))
	}

	for _, session := range sessions {
		if !session.RevokedAt.Valid {
			t.Fatalf("expected session to be revoked after refresh token reuse, got %#v", session)
		}
		if session.RevokeReason != "refresh_token_reuse" {
			t.Fatalf("expected revoke reason refresh_token_reuse, got %q", session.RevokeReason)
		}
	}

	_, err = queries.GetAuthSessionByRefreshTokenHash(ctx, db.GetAuthSessionByRefreshTokenHashParams{
		RefreshTokenHash: hashToken(refreshed.RefreshToken),
	})
	if err != nil {
		t.Fatalf("expected rotated session to remain queryable, got %v", err)
	}
}

func TestServiceExchangeDeviceCodeExpired(t *testing.T) {
	t.Parallel()

	pg := testutil.StartPostgres(t)
	ctx := context.Background()
	queries := db.New(pg.Pool)
	service := newIntegrationService(pg, queries)

	deviceCode, err := service.CreateDeviceCode(ctx, nil)
	if err != nil {
		t.Fatalf("failed to create device code: %v", err)
	}

	service.now = func() time.Time {
		return time.Date(2026, 3, 9, 12, 20, 0, 0, time.UTC)
	}

	_, err = service.ExchangeDeviceCode(ctx, api.DeviceCodeTokenRequest{DeviceCode: deviceCode.DeviceCode}, "internctl/test")
	if !errors.Is(err, ErrExpiredToken) {
		t.Fatalf("expected ErrExpiredToken, got %v", err)
	}

	record, err := queries.GetAuthDeviceAuthorizationByDeviceCode(ctx, db.GetAuthDeviceAuthorizationByDeviceCodeParams{
		DeviceCode: deviceCode.DeviceCode,
	})
	if err != nil {
		t.Fatalf("failed to reload device code: %v", err)
	}
	if record.Status != "expired" {
		t.Fatalf("expected expired status, got %q", record.Status)
	}
}

func TestServiceDenyDeviceCode(t *testing.T) {
	t.Parallel()

	pg := testutil.StartPostgres(t)
	ctx := context.Background()
	queries := db.New(pg.Pool)
	user := createIntegrationUser(t, ctx, queries, "alice", []string{"Users"})
	service := newIntegrationService(pg, queries)

	deviceCode, err := service.CreateDeviceCode(ctx, nil)
	if err != nil {
		t.Fatalf("failed to create device code: %v", err)
	}

	if err := service.DenyDeviceCode(ctx, deviceCode.UserCode, user); err != nil {
		t.Fatalf("expected deny to succeed, got %v", err)
	}

	_, err = service.ExchangeDeviceCode(ctx, api.DeviceCodeTokenRequest{DeviceCode: deviceCode.DeviceCode}, "internctl/test")
	if !errors.Is(err, ErrAccessDenied) {
		t.Fatalf("expected ErrAccessDenied, got %v", err)
	}

	record, err := queries.GetAuthDeviceAuthorizationByDeviceCode(ctx, db.GetAuthDeviceAuthorizationByDeviceCodeParams{
		DeviceCode: deviceCode.DeviceCode,
	})
	if err != nil {
		t.Fatalf("failed to reload denied device code: %v", err)
	}
	if record.Status != "denied" {
		t.Fatalf("expected denied status, got %q", record.Status)
	}
	if !record.ApprovedByUserID.Valid || !record.ApprovedAt.Valid {
		t.Fatalf("expected denying user and timestamp to be recorded, got %#v", record)
	}
}

func TestServiceExchangeDeviceCodePollIntervalThrottled(t *testing.T) {
	t.Parallel()

	pg := testutil.StartPostgres(t)
	ctx := context.Background()
	queries := db.New(pg.Pool)
	service := newIntegrationService(pg, queries)

	deviceCode, err := service.CreateDeviceCode(ctx, nil)
	if err != nil {
		t.Fatalf("failed to create device code: %v", err)
	}

	_, err = service.ExchangeDeviceCode(ctx, api.DeviceCodeTokenRequest{DeviceCode: deviceCode.DeviceCode}, "internctl/test")
	if !errors.Is(err, ErrAuthorizationPending) {
		t.Fatalf("expected ErrAuthorizationPending on first poll, got %v", err)
	}

	_, err = service.ExchangeDeviceCode(ctx, api.DeviceCodeTokenRequest{DeviceCode: deviceCode.DeviceCode}, "internctl/test")
	if !errors.Is(err, ErrTooManyRequests) {
		t.Fatalf("expected ErrTooManyRequests on second poll, got %v", err)
	}

	record, err := queries.GetAuthDeviceAuthorizationByDeviceCode(ctx, db.GetAuthDeviceAuthorizationByDeviceCodeParams{
		DeviceCode: deviceCode.DeviceCode,
	})
	if err != nil {
		t.Fatalf("failed to reload pending device code: %v", err)
	}
	if !record.LastPolledAt.Valid {
		t.Fatalf("expected last_polled_at to be recorded after first poll")
	}
	if record.Status != "pending" {
		t.Fatalf("expected pending status to remain, got %q", record.Status)
	}
}

func newIntegrationService(pg *testutil.PostgresContainer, queries *db.Queries) *Service {
	service := NewService(integrationConfig(), queries, NewPGXTransactor(pg.Pool))
	service.now = fixedIntegrationNow
	service.random = bytes.NewReader(bytes.Repeat([]byte("integration-seed-"), 32))
	return service
}

func createIntegrationUser(t *testing.T, ctx context.Context, queries *db.Queries, username string, groups []string) db.User {
	t.Helper()

	user, err := queries.UpsertUserByUsername(ctx, db.UpsertUserByUsernameParams{
		Username: username,
		Name:     "Alice Example",
		Email:    username + "@example.com",
		Groups:   groups,
	})
	if err != nil {
		t.Fatalf("failed to create user: %v", err)
	}
	return user
}

func fixedIntegrationNow() time.Time {
	return time.Date(2026, 3, 9, 12, 0, 0, 0, time.UTC)
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
