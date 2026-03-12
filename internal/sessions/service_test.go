package sessions

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/sbekti/intern-api/internal/db"
)

type fakeQuerier struct {
	getAuthSessionByIDFn       func(ctx context.Context, arg db.GetAuthSessionByIDParams) (db.AuthSession, error)
	getUserByIDFn              func(ctx context.Context, arg db.GetUserByIDParams) (db.User, error)
	listAuthSessionsFn         func(ctx context.Context) ([]db.AuthSession, error)
	listAuthSessionsByUserIDFn func(ctx context.Context, arg db.ListAuthSessionsByUserIDParams) ([]db.AuthSession, error)
	revokeAuthSessionFn        func(ctx context.Context, arg db.RevokeAuthSessionParams) (db.AuthSession, error)
	revokeOtherAuthSessionsFn  func(ctx context.Context, arg db.RevokeOtherAuthSessionsForUserParams) (int64, error)
}

func (f fakeQuerier) GetAuthSessionByID(ctx context.Context, arg db.GetAuthSessionByIDParams) (db.AuthSession, error) {
	if f.getAuthSessionByIDFn == nil {
		return db.AuthSession{}, pgx.ErrNoRows
	}
	return f.getAuthSessionByIDFn(ctx, arg)
}

func (f fakeQuerier) GetUserByID(ctx context.Context, arg db.GetUserByIDParams) (db.User, error) {
	if f.getUserByIDFn == nil {
		return db.User{}, errors.New("not implemented")
	}
	return f.getUserByIDFn(ctx, arg)
}

func (f fakeQuerier) ListAuthSessions(ctx context.Context) ([]db.AuthSession, error) {
	if f.listAuthSessionsFn == nil {
		return nil, nil
	}
	return f.listAuthSessionsFn(ctx)
}

func (f fakeQuerier) ListAuthSessionsByUserID(ctx context.Context, arg db.ListAuthSessionsByUserIDParams) ([]db.AuthSession, error) {
	if f.listAuthSessionsByUserIDFn == nil {
		return nil, nil
	}
	return f.listAuthSessionsByUserIDFn(ctx, arg)
}

func (f fakeQuerier) RevokeAuthSession(ctx context.Context, arg db.RevokeAuthSessionParams) (db.AuthSession, error) {
	if f.revokeAuthSessionFn == nil {
		return db.AuthSession{}, nil
	}
	return f.revokeAuthSessionFn(ctx, arg)
}

func (f fakeQuerier) RevokeOtherAuthSessionsForUser(ctx context.Context, arg db.RevokeOtherAuthSessionsForUserParams) (int64, error) {
	if f.revokeOtherAuthSessionsFn == nil {
		return 0, nil
	}
	return f.revokeOtherAuthSessionsFn(ctx, arg)
}

func TestValidateSessionReturnsTrueForActiveSession(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 3, 12, 12, 0, 0, 0, time.UTC)
	sessionID := uuid.New()

	service := NewService(fakeQuerier{
		getAuthSessionByIDFn: func(ctx context.Context, arg db.GetAuthSessionByIDParams) (db.AuthSession, error) {
			if arg.ID.Bytes != [16]byte(sessionID) {
				t.Fatalf("expected session ID %s, got %x", sessionID, arg.ID.Bytes)
			}
			return activeSessionRecord(sessionID, now), nil
		},
	})
	service.now = func() time.Time { return now }

	active, err := service.ValidateSession(context.Background(), sessionID.String())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !active {
		t.Fatal("expected session to be active")
	}
}

func TestValidateSessionReturnsFalseForMissingRow(t *testing.T) {
	t.Parallel()

	service := NewService(fakeQuerier{
		getAuthSessionByIDFn: func(ctx context.Context, arg db.GetAuthSessionByIDParams) (db.AuthSession, error) {
			return db.AuthSession{}, pgx.ErrNoRows
		},
	})

	active, err := service.ValidateSession(context.Background(), uuid.New().String())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if active {
		t.Fatal("expected session to be inactive")
	}
}

func TestRevokeOtherProfileSessionsKeepsCurrentBearerSession(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 3, 12, 12, 0, 0, 0, time.UTC)
	currentID := uuid.New()
	userID := uuid.New()
	called := false

	service := NewService(fakeQuerier{
		revokeOtherAuthSessionsFn: func(ctx context.Context, arg db.RevokeOtherAuthSessionsForUserParams) (int64, error) {
			called = true
			if arg.ID.Bytes != [16]byte(currentID) {
				t.Fatalf("expected current session ID %s, got %x", currentID, arg.ID.Bytes)
			}
			if arg.UserID.Bytes != [16]byte(userID) {
				t.Fatalf("expected user ID %s, got %x", userID, arg.UserID.Bytes)
			}
			if arg.RevokeReason != "user_revoke_others" {
				t.Fatalf("expected revoke reason user_revoke_others, got %q", arg.RevokeReason)
			}
			return 2, nil
		},
	})
	service.now = func() time.Time { return now }

	err := service.RevokeOtherProfileSessions(context.Background(), db.User{
		ID:       pgUUID(userID),
		Username: "alice",
	}, currentID.String())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !called {
		t.Fatal("expected bulk revoke to be called")
	}
}

func TestRevokeOtherProfileSessionsWithoutCurrentSessionRevokesAllActiveSessions(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 3, 12, 12, 0, 0, 0, time.UTC)
	userID := uuid.New()
	activeID := uuid.New()
	expiredID := uuid.New()
	revokedIDs := make([]uuid.UUID, 0, 1)

	service := NewService(fakeQuerier{
		listAuthSessionsByUserIDFn: func(ctx context.Context, arg db.ListAuthSessionsByUserIDParams) ([]db.AuthSession, error) {
			if arg.UserID.Bytes != [16]byte(userID) {
				t.Fatalf("expected user ID %s, got %x", userID, arg.UserID.Bytes)
			}
			return []db.AuthSession{
				activeSessionRecord(activeID, now),
				expiredSessionRecord(expiredID, now),
			}, nil
		},
		revokeAuthSessionFn: func(ctx context.Context, arg db.RevokeAuthSessionParams) (db.AuthSession, error) {
			revokedIDs = append(revokedIDs, uuid.UUID(arg.ID.Bytes))
			return db.AuthSession{}, nil
		},
	})
	service.now = func() time.Time { return now }

	err := service.RevokeOtherProfileSessions(context.Background(), db.User{
		ID:       pgUUID(userID),
		Username: "alice",
	}, "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(revokedIDs) != 1 || revokedIDs[0] != activeID {
		t.Fatalf("expected only active session %s to be revoked, got %v", activeID, revokedIDs)
	}
}

func activeSessionRecord(id uuid.UUID, now time.Time) db.AuthSession {
	return db.AuthSession{
		ID:            pgUUID(id),
		ExpiresAt:     pgtype.Timestamptz{Time: now.Add(15 * time.Minute), Valid: true},
		IdleExpiresAt: pgtype.Timestamptz{Time: now.Add(30 * time.Minute), Valid: true},
	}
}

func expiredSessionRecord(id uuid.UUID, now time.Time) db.AuthSession {
	return db.AuthSession{
		ID:            pgUUID(id),
		ExpiresAt:     pgtype.Timestamptz{Time: now.Add(-1 * time.Minute), Valid: true},
		IdleExpiresAt: pgtype.Timestamptz{Time: now.Add(30 * time.Minute), Valid: true},
	}
}
