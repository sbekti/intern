package cliauth

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/sbekti/intern-api/internal/api"
	"github.com/sbekti/intern-api/internal/config"
	"github.com/sbekti/intern-api/internal/db"
)

type fakeQuerier struct {
	createAuthzFn      func(ctx context.Context, arg db.CreateAuthDeviceAuthorizationParams) (db.AuthDeviceAuthorization, error)
	getAuthzByDeviceFn func(ctx context.Context, arg db.GetAuthDeviceAuthorizationByDeviceCodeParams) (db.AuthDeviceAuthorization, error)
	getAuthzByUserFn   func(ctx context.Context, arg db.GetAuthDeviceAuthorizationByUserCodeParams) (db.AuthDeviceAuthorization, error)
	updateAuthzFn      func(ctx context.Context, arg db.UpdateAuthDeviceAuthorizationStatusParams) (db.AuthDeviceAuthorization, error)
	createSessionFn    func(ctx context.Context, arg db.CreateAuthSessionParams) (db.AuthSession, error)
	getSessionByHashFn func(ctx context.Context, arg db.GetAuthSessionByRefreshTokenHashParams) (db.AuthSession, error)
	getUserByIDFn      func(ctx context.Context, arg db.GetUserByIDParams) (db.User, error)
	revokeSessionFn    func(ctx context.Context, arg db.RevokeAuthSessionParams) (db.AuthSession, error)
	revokeFamilyFn     func(ctx context.Context, arg db.RevokeAuthSessionFamilyParams) (int64, error)
}

func (f fakeQuerier) CreateAuthDeviceAuthorization(ctx context.Context, arg db.CreateAuthDeviceAuthorizationParams) (db.AuthDeviceAuthorization, error) {
	return f.createAuthzFn(ctx, arg)
}
func (f fakeQuerier) GetAuthDeviceAuthorizationByDeviceCode(ctx context.Context, arg db.GetAuthDeviceAuthorizationByDeviceCodeParams) (db.AuthDeviceAuthorization, error) {
	return f.getAuthzByDeviceFn(ctx, arg)
}
func (f fakeQuerier) GetAuthDeviceAuthorizationByUserCode(ctx context.Context, arg db.GetAuthDeviceAuthorizationByUserCodeParams) (db.AuthDeviceAuthorization, error) {
	return f.getAuthzByUserFn(ctx, arg)
}
func (f fakeQuerier) UpdateAuthDeviceAuthorizationStatus(ctx context.Context, arg db.UpdateAuthDeviceAuthorizationStatusParams) (db.AuthDeviceAuthorization, error) {
	return f.updateAuthzFn(ctx, arg)
}
func (f fakeQuerier) CreateAuthSession(ctx context.Context, arg db.CreateAuthSessionParams) (db.AuthSession, error) {
	return f.createSessionFn(ctx, arg)
}
func (f fakeQuerier) GetAuthSessionByRefreshTokenHash(ctx context.Context, arg db.GetAuthSessionByRefreshTokenHashParams) (db.AuthSession, error) {
	return f.getSessionByHashFn(ctx, arg)
}
func (f fakeQuerier) GetUserByID(ctx context.Context, arg db.GetUserByIDParams) (db.User, error) {
	return f.getUserByIDFn(ctx, arg)
}
func (f fakeQuerier) RevokeAuthSession(ctx context.Context, arg db.RevokeAuthSessionParams) (db.AuthSession, error) {
	return f.revokeSessionFn(ctx, arg)
}
func (f fakeQuerier) RevokeAuthSessionFamily(ctx context.Context, arg db.RevokeAuthSessionFamilyParams) (int64, error) {
	return f.revokeFamilyFn(ctx, arg)
}

type fakeTransactor struct {
	q Querier
}

func (f fakeTransactor) InTx(ctx context.Context, fn func(q Querier) error) error {
	return fn(f.q)
}

func TestCreateDeviceAuthorization(t *testing.T) {
	t.Parallel()

	called := false
	service := testService(fakeQuerier{
		createAuthzFn: func(ctx context.Context, arg db.CreateAuthDeviceAuthorizationParams) (db.AuthDeviceAuthorization, error) {
			called = true
			if arg.ClientName != "internctl" {
				t.Fatalf("expected default client name internctl, got %q", arg.ClientName)
			}
			if arg.Status != "pending" {
				t.Fatalf("expected pending status, got %q", arg.Status)
			}
			return db.AuthDeviceAuthorization{
				DeviceCode: arg.DeviceCode,
				UserCode:   arg.UserCode,
				ExpiresAt:  arg.ExpiresAt,
				Status:     arg.Status,
			}, nil
		},
	})

	result, err := service.CreateDeviceAuthorization(context.Background(), nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !called {
		t.Fatal("expected create auth device authorization to be called")
	}
	if result.VerificationUrl != "https://intern.corp.example.com/auth/device" {
		t.Fatalf("unexpected verification url %q", result.VerificationUrl)
	}
}

func TestApproveDeviceAuthorization(t *testing.T) {
	t.Parallel()

	updated := false
	service := testService(fakeQuerier{
		getAuthzByUserFn: func(ctx context.Context, arg db.GetAuthDeviceAuthorizationByUserCodeParams) (db.AuthDeviceAuthorization, error) {
			return db.AuthDeviceAuthorization{
				ID:        pgUUID(uuid.MustParse("11111111-1111-1111-1111-111111111111")),
				UserCode:  arg.UserCode,
				Status:    "pending",
				ExpiresAt: timestamptz(time.Date(2026, 3, 9, 13, 0, 0, 0, time.UTC)),
			}, nil
		},
		updateAuthzFn: func(ctx context.Context, arg db.UpdateAuthDeviceAuthorizationStatusParams) (db.AuthDeviceAuthorization, error) {
			updated = true
			if arg.Status != "approved" {
				t.Fatalf("expected approved status, got %q", arg.Status)
			}
			return db.AuthDeviceAuthorization{}, nil
		},
	})

	err := service.ApproveDeviceAuthorization(context.Background(), "ABCD-EFGH", db.User{
		ID:       pgUUID(uuid.MustParse("22222222-2222-2222-2222-222222222222")),
		Username: "alice",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !updated {
		t.Fatal("expected update auth device authorization to be called")
	}
}

func TestExchangeDeviceAuthorizationPending(t *testing.T) {
	t.Parallel()

	service := testService(fakeQuerier{
		getAuthzByDeviceFn: func(ctx context.Context, arg db.GetAuthDeviceAuthorizationByDeviceCodeParams) (db.AuthDeviceAuthorization, error) {
			return db.AuthDeviceAuthorization{
				ID:         pgUUID(uuid.MustParse("11111111-1111-1111-1111-111111111111")),
				DeviceCode: arg.DeviceCode,
				Status:     "pending",
				ExpiresAt:  timestamptz(time.Date(2026, 3, 9, 13, 0, 0, 0, time.UTC)),
			}, nil
		},
		updateAuthzFn: func(ctx context.Context, arg db.UpdateAuthDeviceAuthorizationStatusParams) (db.AuthDeviceAuthorization, error) {
			return db.AuthDeviceAuthorization{}, nil
		},
	})

	_, err := service.ExchangeDeviceAuthorization(context.Background(), api.DeviceTokenRequest{DeviceCode: "code"}, "internctl")
	if !errors.Is(err, ErrAuthorizationPending) {
		t.Fatalf("expected ErrAuthorizationPending, got %v", err)
	}
}

func TestExchangeDeviceAuthorizationApproved(t *testing.T) {
	t.Parallel()

	userID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	sessionID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	service := testServiceWithTransactor(fakeQuerier{
		getAuthzByDeviceFn: func(ctx context.Context, arg db.GetAuthDeviceAuthorizationByDeviceCodeParams) (db.AuthDeviceAuthorization, error) {
			return db.AuthDeviceAuthorization{
				ID:               pgUUID(uuid.MustParse("11111111-1111-1111-1111-111111111111")),
				DeviceCode:       arg.DeviceCode,
				ClientName:       "internctl",
				Status:           "approved",
				ApprovedByUserID: pgUUID(userID),
				ApprovedAt:       timestamptz(time.Date(2026, 3, 9, 11, 0, 0, 0, time.UTC)),
				ExpiresAt:        timestamptz(time.Date(2026, 3, 9, 13, 0, 0, 0, time.UTC)),
			}, nil
		},
		getUserByIDFn: func(ctx context.Context, arg db.GetUserByIDParams) (db.User, error) {
			return db.User{
				ID:       pgUUID(userID),
				Username: "alice",
				Name:     "Alice Example",
				Email:    "alice@example.com",
				Groups:   []string{"Users"},
			}, nil
		},
		createSessionFn: func(ctx context.Context, arg db.CreateAuthSessionParams) (db.AuthSession, error) {
			return db.AuthSession{
				ID:                   pgUUID(sessionID),
				UserID:               arg.UserID,
				ClientName:           arg.ClientName,
				UserAgent:            arg.UserAgent,
				RefreshTokenFamilyID: arg.RefreshTokenFamilyID,
				ExpiresAt:            arg.ExpiresAt,
				IdleExpiresAt:        arg.IdleExpiresAt,
			}, nil
		},
		updateAuthzFn: func(ctx context.Context, arg db.UpdateAuthDeviceAuthorizationStatusParams) (db.AuthDeviceAuthorization, error) {
			return db.AuthDeviceAuthorization{}, nil
		},
	}, strings.NewReader(strings.Repeat("a", 64)))

	response, err := service.ExchangeDeviceAuthorization(context.Background(), api.DeviceTokenRequest{DeviceCode: "code"}, "internctl")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if response.TokenType != "Bearer" || response.AccessToken == "" || response.RefreshToken == "" {
		t.Fatalf("unexpected token response %+v", response)
	}
}

func TestRefreshAccessTokenRotatesSession(t *testing.T) {
	t.Parallel()

	userID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	sessionID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	familyID := uuid.MustParse("44444444-4444-4444-4444-444444444444")
	service := testServiceWithTransactor(fakeQuerier{
		getSessionByHashFn: func(ctx context.Context, arg db.GetAuthSessionByRefreshTokenHashParams) (db.AuthSession, error) {
			return db.AuthSession{
				ID:                   pgUUID(sessionID),
				UserID:               pgUUID(userID),
				ClientName:           "internctl",
				UserAgent:            "internctl",
				RefreshTokenFamilyID: pgUUID(familyID),
				ExpiresAt:            timestamptz(time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)),
				IdleExpiresAt:        timestamptz(time.Date(2026, 3, 10, 0, 0, 0, 0, time.UTC)),
			}, nil
		},
		getUserByIDFn: func(ctx context.Context, arg db.GetUserByIDParams) (db.User, error) {
			return db.User{
				ID:       pgUUID(userID),
				Username: "alice",
				Name:     "Alice Example",
				Email:    "alice@example.com",
				Groups:   []string{"Users"},
			}, nil
		},
		revokeSessionFn: func(ctx context.Context, arg db.RevokeAuthSessionParams) (db.AuthSession, error) {
			return db.AuthSession{}, nil
		},
		createSessionFn: func(ctx context.Context, arg db.CreateAuthSessionParams) (db.AuthSession, error) {
			return db.AuthSession{
				ID:                   pgUUID(uuid.MustParse("55555555-5555-5555-5555-555555555555")),
				UserID:               arg.UserID,
				ClientName:           arg.ClientName,
				UserAgent:            arg.UserAgent,
				RefreshTokenFamilyID: arg.RefreshTokenFamilyID,
				ExpiresAt:            arg.ExpiresAt,
				IdleExpiresAt:        arg.IdleExpiresAt,
			}, nil
		},
		revokeFamilyFn: func(ctx context.Context, arg db.RevokeAuthSessionFamilyParams) (int64, error) {
			return 0, nil
		},
	}, strings.NewReader(strings.Repeat("b", 64)))

	response, err := service.RefreshAccessToken(context.Background(), api.RefreshTokenRequest{RefreshToken: "refresh"}, "internctl")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if response.RefreshToken == "" || response.AccessToken == "" {
		t.Fatalf("unexpected response %+v", response)
	}
}

func TestLogoutMissingSessionIsNoop(t *testing.T) {
	t.Parallel()

	service := testService(fakeQuerier{
		getSessionByHashFn: func(ctx context.Context, arg db.GetAuthSessionByRefreshTokenHashParams) (db.AuthSession, error) {
			return db.AuthSession{}, pgx.ErrNoRows
		},
	})

	if err := service.Logout(context.Background(), api.LogoutRequest{RefreshToken: "missing"}); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func testService(q Querier) *Service {
	return testServiceWithTransactor(q, strings.NewReader(strings.Repeat("c", 64)))
}

func testServiceWithTransactor(q Querier, random io.Reader) *Service {
	service := NewService(config.Config{
		Auth: config.AuthConfig{
			JWTIssuer:          "intern.corp.example.com",
			JWTAudience:        "internctl",
			JWTHMACSecret:      "test-secret",
			AccessTokenTTL:     15 * time.Minute,
			RefreshIdleTTL:     30 * 24 * time.Hour,
			RefreshAbsoluteTTL: 90 * 24 * time.Hour,
			DeviceCodeTTL:      10 * time.Minute,
			DevicePollInterval: 5 * time.Second,
			VerificationURL:    "https://intern.corp.example.com/auth/device",
		},
	}, q, fakeTransactor{q: q})
	service.now = func() time.Time {
		return time.Date(2026, 3, 9, 12, 0, 0, 0, time.UTC)
	}
	service.random = random
	return service
}

func pgUUID(value uuid.UUID) pgtype.UUID {
	var raw [16]byte
	copy(raw[:], value[:])
	return pgtype.UUID{Bytes: raw, Valid: true}
}
