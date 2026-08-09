package sessions

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/sbekti/intern/internal/db"
)

type fakeQuerier struct {
	countActiveAuthSessionsFn         func(ctx context.Context) (int64, error)
	countActiveAuthSessionsByUserIDFn func(ctx context.Context, arg pgtype.UUID) (int64, error)
	getAuthSessionByIDFn              func(ctx context.Context, arg db.GetAuthSessionByIDParams) (db.AuthSession, error)
	getUserByIDFn                     func(ctx context.Context, arg db.GetUserByIDParams) (db.User, error)
	listActiveAuthSessionsByUserFn    func(ctx context.Context, arg db.ListActiveAuthSessionsByUserPageParams) ([]db.AuthSession, error)
	listActiveAuthSessionsFn          func(ctx context.Context, arg db.ListActiveAuthSessionsPageParams) ([]db.AuthSession, error)
	listAuthSessionsByUserIDFn        func(ctx context.Context, arg db.ListAuthSessionsByUserIDParams) ([]db.AuthSession, error)
	revokeAllActiveFn                 func(ctx context.Context, arg db.RevokeAllActiveAuthSessionsParams) (int64, error)
	revokeAuthSessionFn               func(ctx context.Context, arg db.RevokeAuthSessionParams) (db.AuthSession, error)
	revokeOtherAuthSessionsFn         func(ctx context.Context, arg db.RevokeOtherAuthSessionsForUserParams) (int64, error)
}

func (f fakeQuerier) CountActiveAuthSessions(ctx context.Context) (int64, error) {
	if f.countActiveAuthSessionsFn == nil {
		return 0, nil
	}
	return f.countActiveAuthSessionsFn(ctx)
}

func (f fakeQuerier) CountActiveAuthSessionsByUserID(ctx context.Context, arg db.CountActiveAuthSessionsByUserIDParams) (int64, error) {
	if f.countActiveAuthSessionsByUserIDFn == nil {
		return 0, nil
	}
	return f.countActiveAuthSessionsByUserIDFn(ctx, arg.UserID)
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

func (f fakeQuerier) ListActiveAuthSessionsPage(ctx context.Context, arg db.ListActiveAuthSessionsPageParams) ([]db.AuthSession, error) {
	if f.listActiveAuthSessionsFn == nil {
		return nil, nil
	}
	return f.listActiveAuthSessionsFn(ctx, arg)
}

func (f fakeQuerier) ListActiveAuthSessionsByUserPage(ctx context.Context, arg db.ListActiveAuthSessionsByUserPageParams) ([]db.AuthSession, error) {
	if f.listActiveAuthSessionsByUserFn == nil {
		return nil, nil
	}
	return f.listActiveAuthSessionsByUserFn(ctx, arg)
}

func (f fakeQuerier) ListAuthSessionsByUserID(ctx context.Context, arg db.ListAuthSessionsByUserIDParams) ([]db.AuthSession, error) {
	if f.listAuthSessionsByUserIDFn == nil {
		return nil, nil
	}
	return f.listAuthSessionsByUserIDFn(ctx, arg)
}

func (f fakeQuerier) RevokeAllActiveAuthSessions(ctx context.Context, arg db.RevokeAllActiveAuthSessionsParams) (int64, error) {
	if f.revokeAllActiveFn == nil {
		return 0, nil
	}
	return f.revokeAllActiveFn(ctx, arg)
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

func TestListProfileSessionsPageUsesActiveQueryAndPaginates(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 3, 12, 12, 0, 0, 0, time.UTC)
	userID := uuid.New()
	sessionID := uuid.New()

	service := NewService(fakeQuerier{
		countActiveAuthSessionsByUserIDFn: func(ctx context.Context, arg pgtype.UUID) (int64, error) {
			if arg.Bytes != [16]byte(userID) {
				t.Fatalf("expected user ID %s, got %x", userID, arg.Bytes)
			}
			return 3, nil
		},
		listActiveAuthSessionsByUserFn: func(ctx context.Context, arg db.ListActiveAuthSessionsByUserPageParams) ([]db.AuthSession, error) {
			if arg.UserID.Bytes != [16]byte(userID) {
				t.Fatalf("expected user ID %s, got %x", userID, arg.UserID.Bytes)
			}
			if arg.LimitCount != 25 {
				t.Fatalf("expected limit 25, got %d", arg.LimitCount)
			}
			if arg.OffsetCount != 25 {
				t.Fatalf("expected offset 25, got %d", arg.OffsetCount)
			}
			return []db.AuthSession{
				{
					ID:            pgUUID(sessionID),
					UserID:        pgUUID(userID),
					ClientName:    "eve-laptop",
					CreatedAt:     pgtype.Timestamptz{Time: now, Valid: true},
					LastUsedAt:    pgtype.Timestamptz{Time: now.Add(-2 * time.Minute), Valid: true},
					ExpiresAt:     pgtype.Timestamptz{Time: now.Add(15 * time.Minute), Valid: true},
					IdleExpiresAt: pgtype.Timestamptz{Time: now.Add(10 * time.Minute), Valid: true},
				},
			}, nil
		},
	})
	service.now = func() time.Time { return now }

	page, err := service.ListProfileSessionsPage(context.Background(), db.User{
		ID:       pgUUID(userID),
		Username: "eve",
	}, sessionID.String(), 25, 25)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if page.Pagination.Total != 3 {
		t.Fatalf("expected total 3, got %d", page.Pagination.Total)
	}
	if page.Pagination.Limit != 25 || page.Pagination.Offset != 25 {
		t.Fatalf("unexpected pagination %+v", page.Pagination)
	}
	if len(page.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(page.Items))
	}
	if page.Items[0].Username != "eve" {
		t.Fatalf("expected username eve, got %q", page.Items[0].Username)
	}
	if page.Items[0].ClientName != "eve-laptop" {
		t.Fatalf("expected client name eve-laptop, got %q", page.Items[0].ClientName)
	}
	if !page.Items[0].IsCurrent {
		t.Fatal("expected current session to be marked")
	}
}

func TestListAdminSessionsPageUsesActiveQueryAndPaginates(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 3, 12, 12, 0, 0, 0, time.UTC)
	userID := uuid.New()
	sessionID := uuid.New()

	service := NewService(fakeQuerier{
		countActiveAuthSessionsFn: func(ctx context.Context) (int64, error) {
			return 7, nil
		},
		listActiveAuthSessionsFn: func(ctx context.Context, arg db.ListActiveAuthSessionsPageParams) ([]db.AuthSession, error) {
			if arg.LimitCount != 25 {
				t.Fatalf("expected limit 25, got %d", arg.LimitCount)
			}
			if arg.OffsetCount != 50 {
				t.Fatalf("expected offset 50, got %d", arg.OffsetCount)
			}
			return []db.AuthSession{
				{
					ID:            pgUUID(sessionID),
					UserID:        pgUUID(userID),
					ClientName:    "internctl",
					CreatedAt:     pgtype.Timestamptz{Time: now, Valid: true},
					LastUsedAt:    pgtype.Timestamptz{Time: now.Add(-5 * time.Minute), Valid: true},
					ExpiresAt:     pgtype.Timestamptz{Time: now.Add(15 * time.Minute), Valid: true},
					IdleExpiresAt: pgtype.Timestamptz{Time: now.Add(10 * time.Minute), Valid: true},
				},
			}, nil
		},
		getUserByIDFn: func(ctx context.Context, arg db.GetUserByIDParams) (db.User, error) {
			if arg.ID.Bytes != [16]byte(userID) {
				t.Fatalf("expected user ID %s, got %x", userID, arg.ID.Bytes)
			}
			return db.User{ID: pgUUID(userID), Username: "alice"}, nil
		},
	})
	service.now = func() time.Time { return now }

	page, err := service.ListAdminSessionsPage(context.Background(), "", 25, 50)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if page.Pagination.Total != 7 {
		t.Fatalf("expected total 7, got %d", page.Pagination.Total)
	}
	if page.Pagination.Limit != 25 || page.Pagination.Offset != 50 {
		t.Fatalf("unexpected pagination %+v", page.Pagination)
	}
	if len(page.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(page.Items))
	}
	if page.Items[0].Username != "alice" {
		t.Fatalf("expected username alice, got %q", page.Items[0].Username)
	}
}

func TestRevokeAllAdminSessionsUsesGlobalReason(t *testing.T) {
	t.Parallel()

	called := false
	service := NewService(fakeQuerier{
		revokeAllActiveFn: func(ctx context.Context, arg db.RevokeAllActiveAuthSessionsParams) (int64, error) {
			called = true
			if arg.RevokeReason != "admin_revoke_all" {
				t.Fatalf("expected revoke reason admin_revoke_all, got %q", arg.RevokeReason)
			}
			return 4, nil
		},
	})

	if err := service.RevokeAllAdminSessions(context.Background()); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !called {
		t.Fatal("expected global revoke to be called")
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
