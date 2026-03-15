package clientauth

import (
	"context"
	"errors"
	"io"
	"strings"
	"sync"
	"sync/atomic"
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
	createAuditLogFn   func(ctx context.Context, arg db.CreateAuditLogParams) (db.AuditLog, error)
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
func (f fakeQuerier) CreateAuditLog(ctx context.Context, arg db.CreateAuditLogParams) (db.AuditLog, error) {
	return f.createAuditLogFn(ctx, arg)
}

type fakeTransactor struct {
	q Querier
}

func (f fakeTransactor) InTx(ctx context.Context, fn func(q Querier) error) error {
	return fn(f.q)
}

func TestCreateDeviceCode(t *testing.T) {
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

	response, err := service.CreateDeviceCode(context.Background(), nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !called {
		t.Fatal("expected create auth device authorization to be called")
	}
	if response.VerificationUri != "https://intern.corp.example.com/auth/device" {
		t.Fatalf("verification_uri = %q", response.VerificationUri)
	}
	if response.VerificationUriComplete == "" {
		t.Fatal("expected verification_uri_complete")
	}
	if response.ExpiresIn != 600 {
		t.Fatalf("expires_in = %d, want 600", response.ExpiresIn)
	}
	if response.Interval != 5 {
		t.Fatalf("interval = %d, want 5", response.Interval)
	}
}

func TestApproveDeviceCode(t *testing.T) {
	t.Parallel()

	updated := false
	audited := false
	service := testService(fakeQuerier{
		getAuthzByUserFn: func(ctx context.Context, arg db.GetAuthDeviceAuthorizationByUserCodeParams) (db.AuthDeviceAuthorization, error) {
			return db.AuthDeviceAuthorization{
				ID:         pgUUID(uuid.MustParse("11111111-1111-1111-1111-111111111111")),
				UserCode:   arg.UserCode,
				ClientName: "desktop-app",
				Status:     "pending",
				ExpiresAt:  timestamptz(time.Date(2026, 3, 9, 13, 0, 0, 0, time.UTC)),
			}, nil
		},
		updateAuthzFn: func(ctx context.Context, arg db.UpdateAuthDeviceAuthorizationStatusParams) (db.AuthDeviceAuthorization, error) {
			updated = true
			if arg.Status != "approved" {
				t.Fatalf("expected approved status, got %q", arg.Status)
			}
			return db.AuthDeviceAuthorization{
				ID:         arg.ID,
				UserCode:   "ABCD-EFGH",
				ClientName: "desktop-app",
				Status:     "approved",
			}, nil
		},
		createAuditLogFn: func(ctx context.Context, arg db.CreateAuditLogParams) (db.AuditLog, error) {
			audited = true
			if arg.Action != "auth.device_code.approve" {
				t.Fatalf("unexpected audit action %q", arg.Action)
			}
			return db.AuditLog{}, nil
		},
	})

	err := service.ApproveDeviceCode(context.Background(), "ABCD-EFGH", db.User{
		ID:       pgUUID(uuid.MustParse("22222222-2222-2222-2222-222222222222")),
		Username: "alice",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !updated {
		t.Fatal("expected update auth device authorization to be called")
	}
	if !audited {
		t.Fatal("expected audit log to be written")
	}
}

func TestExchangeDeviceCodePending(t *testing.T) {
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

	_, err := service.ExchangeDeviceCode(context.Background(), api.DeviceCodeTokenRequest{DeviceCode: "code"}, "internctl")
	if !errors.Is(err, ErrAuthorizationPending) {
		t.Fatalf("expected ErrAuthorizationPending, got %v", err)
	}
}

func TestExchangeDeviceCodeSlowDown(t *testing.T) {
	t.Parallel()

	service := testService(fakeQuerier{
		getAuthzByDeviceFn: func(ctx context.Context, arg db.GetAuthDeviceAuthorizationByDeviceCodeParams) (db.AuthDeviceAuthorization, error) {
			return db.AuthDeviceAuthorization{
				ID:           pgUUID(uuid.MustParse("11111111-1111-1111-1111-111111111111")),
				DeviceCode:   arg.DeviceCode,
				Status:       "pending",
				LastPolledAt: timestamptz(time.Date(2026, 3, 9, 11, 59, 57, 0, time.UTC)),
				ExpiresAt:    timestamptz(time.Date(2026, 3, 9, 13, 0, 0, 0, time.UTC)),
			}, nil
		},
	})

	_, err := service.ExchangeDeviceCode(context.Background(), api.DeviceCodeTokenRequest{DeviceCode: "code"}, "internctl")
	if !errors.Is(err, ErrSlowDown) {
		t.Fatalf("expected ErrSlowDown, got %v", err)
	}
}

func TestExchangeDeviceCodeApproved(t *testing.T) {
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
		createAuditLogFn: func(ctx context.Context, arg db.CreateAuditLogParams) (db.AuditLog, error) {
			if arg.Action != "auth.device_code.exchange" {
				t.Fatalf("unexpected audit action %q", arg.Action)
			}
			return db.AuditLog{}, nil
		},
	}, strings.NewReader(strings.Repeat("a", 64)))

	response, err := service.ExchangeDeviceCode(context.Background(), api.DeviceCodeTokenRequest{DeviceCode: "code"}, "internctl")
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

func TestRefreshAccessTokenConcurrentReuseRevokesFamily(t *testing.T) {
	t.Parallel()

	userID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	sessionID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	familyID := uuid.MustParse("44444444-4444-4444-4444-444444444444")

	var revokeCalls atomic.Int32
	var createSessionCalls atomic.Int32
	var familyRevokeCalls atomic.Int32
	var auditCalls atomic.Int32
	firstEntered := make(chan struct{})
	secondEntered := make(chan struct{})

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
			switch revokeCalls.Add(1) {
			case 1:
				close(firstEntered)
				<-secondEntered
				return db.AuthSession{}, nil
			case 2:
				close(secondEntered)
				return db.AuthSession{}, pgx.ErrNoRows
			default:
				t.Fatalf("unexpected extra revoke attempt")
				return db.AuthSession{}, nil
			}
		},
		createSessionFn: func(ctx context.Context, arg db.CreateAuthSessionParams) (db.AuthSession, error) {
			createSessionCalls.Add(1)
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
			familyRevokeCalls.Add(1)
			if arg.RevokeReason != "refresh_token_reuse" {
				t.Fatalf("unexpected revoke family reason %q", arg.RevokeReason)
			}
			return 2, nil
		},
		createAuditLogFn: func(ctx context.Context, arg db.CreateAuditLogParams) (db.AuditLog, error) {
			auditCalls.Add(1)
			if arg.Action != "auth.session.family_revoke" {
				t.Fatalf("unexpected audit action %q", arg.Action)
			}
			return db.AuditLog{}, nil
		},
	}, strings.NewReader(strings.Repeat("d", 64)))

	type result struct {
		response *api.TokenResponse
		err      error
	}

	results := make(chan result, 2)
	var wg sync.WaitGroup
	wg.Add(2)

	for i := 0; i < 2; i++ {
		go func() {
			defer wg.Done()
			response, err := service.RefreshAccessToken(context.Background(), api.RefreshTokenRequest{RefreshToken: "refresh"}, "internctl")
			results <- result{response: response, err: err}
		}()
	}

	<-firstEntered
	wg.Wait()
	close(results)

	successes := 0
	unauthorized := 0
	for result := range results {
		switch {
		case result.err == nil:
			successes++
			if result.response == nil || result.response.RefreshToken == "" || result.response.AccessToken == "" {
				t.Fatalf("unexpected success response %+v", result.response)
			}
		case errors.Is(result.err, ErrUnauthorized):
			unauthorized++
		default:
			t.Fatalf("unexpected refresh result err=%v", result.err)
		}
	}

	if successes != 1 || unauthorized != 1 {
		t.Fatalf("expected 1 success and 1 unauthorized, got %d success and %d unauthorized", successes, unauthorized)
	}
	if createSessionCalls.Load() != 1 {
		t.Fatalf("expected 1 new session, got %d", createSessionCalls.Load())
	}
	if familyRevokeCalls.Load() != 1 {
		t.Fatalf("expected 1 family revoke, got %d", familyRevokeCalls.Load())
	}
	if auditCalls.Load() != 1 {
		t.Fatalf("expected 1 family revoke audit log, got %d", auditCalls.Load())
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
			PublicBaseURL:      "https://intern.corp.example.com",
			JWTIssuer:          "intern.corp.example.com",
			JWTAudience:        "internctl",
			JWTHMACSecret:      "test-secret",
			AccessTokenTTL:     15 * time.Minute,
			RefreshIdleTTL:     30 * 24 * time.Hour,
			RefreshAbsoluteTTL: 90 * 24 * time.Hour,
			DeviceCodeTTL:      10 * time.Minute,
			DevicePollInterval: 5 * time.Second,
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
