package sessions

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/sbekti/intern-api/internal/api"
	"github.com/sbekti/intern-api/internal/db"
)

type Querier interface {
	CountActiveAuthSessions(ctx context.Context) (int64, error)
	CountActiveAuthSessionsByUserID(ctx context.Context, arg db.CountActiveAuthSessionsByUserIDParams) (int64, error)
	GetAuthSessionByID(ctx context.Context, arg db.GetAuthSessionByIDParams) (db.AuthSession, error)
	GetUserByID(ctx context.Context, arg db.GetUserByIDParams) (db.User, error)
	ListActiveAuthSessionsByUserPage(ctx context.Context, arg db.ListActiveAuthSessionsByUserPageParams) ([]db.AuthSession, error)
	ListActiveAuthSessionsPage(ctx context.Context, arg db.ListActiveAuthSessionsPageParams) ([]db.AuthSession, error)
	ListAuthSessionsByUserID(ctx context.Context, arg db.ListAuthSessionsByUserIDParams) ([]db.AuthSession, error)
	RevokeAllActiveAuthSessions(ctx context.Context, arg db.RevokeAllActiveAuthSessionsParams) (int64, error)
	RevokeAuthSession(ctx context.Context, arg db.RevokeAuthSessionParams) (db.AuthSession, error)
	RevokeOtherAuthSessionsForUser(ctx context.Context, arg db.RevokeOtherAuthSessionsForUserParams) (int64, error)
}

type Service struct {
	queries Querier
	now     func() time.Time
}

func NewService(queries Querier) *Service {
	return &Service{
		queries: queries,
		now:     time.Now,
	}
}

func (s *Service) ListProfileSessionsPage(ctx context.Context, user db.User, currentSessionID string, limit, offset int32) (*api.AuthSessionPage, error) {
	totalCount, err := s.queries.CountActiveAuthSessionsByUserID(ctx, db.CountActiveAuthSessionsByUserIDParams{
		UserID: user.ID,
	})
	if err != nil {
		return nil, err
	}

	rows, err := s.queries.ListActiveAuthSessionsByUserPage(ctx, db.ListActiveAuthSessionsByUserPageParams{
		UserID:      user.ID,
		LimitCount:  limit,
		OffsetCount: offset,
	})
	if err != nil {
		return nil, err
	}

	return &api.AuthSessionPage{
		Items: s.toAPISessions(rows, user.Username, currentSessionID),
		Pagination: api.AuthSessionPagination{
			Limit:  limit,
			Offset: offset,
			Total:  totalCount,
		},
	}, nil
}

func (s *Service) RevokeProfileSession(ctx context.Context, user db.User, sessionID uuid.UUID) error {
	record, err := s.queries.GetAuthSessionByID(ctx, db.GetAuthSessionByIDParams{
		ID: pgUUID(sessionID),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		return err
	}
	if !record.UserID.Valid || record.UserID.Bytes != user.ID.Bytes {
		return nil
	}
	if record.RevokedAt.Valid || !s.isSessionActive(record, s.now()) {
		return nil
	}

	_, err = s.queries.RevokeAuthSession(ctx, db.RevokeAuthSessionParams{
		ID:           pgUUID(sessionID),
		RevokeReason: "user_revoke",
	})
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}

	return nil
}

func (s *Service) RevokeOtherProfileSessions(ctx context.Context, user db.User, currentSessionID string) error {
	currentID, ok := parseUUID(currentSessionID)
	if ok {
		_, err := s.queries.RevokeOtherAuthSessionsForUser(ctx, db.RevokeOtherAuthSessionsForUserParams{
			UserID:       user.ID,
			ID:           pgUUID(currentID),
			RevokeReason: "user_revoke_others",
		})
		return err
	}

	rows, err := s.queries.ListAuthSessionsByUserID(ctx, db.ListAuthSessionsByUserIDParams{
		UserID: user.ID,
	})
	if err != nil {
		return err
	}

	for _, record := range rows {
		if !s.isSessionActive(record, s.now()) {
			continue
		}
		if _, err := s.queries.RevokeAuthSession(ctx, db.RevokeAuthSessionParams{
			ID:           record.ID,
			RevokeReason: "user_revoke_others",
		}); err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
	}

	return nil
}

func (s *Service) ListAdminSessionsPage(ctx context.Context, currentSessionID string, limit, offset int32) (*api.AuthSessionPage, error) {
	totalCount, err := s.queries.CountActiveAuthSessions(ctx)
	if err != nil {
		return nil, err
	}

	rows, err := s.queries.ListActiveAuthSessionsPage(ctx, db.ListActiveAuthSessionsPageParams{
		LimitCount:  limit,
		OffsetCount: offset,
	})
	if err != nil {
		return nil, err
	}

	usernames := make(map[[16]byte]string)
	items := make([]api.AuthSession, 0, len(rows))
	for _, record := range rows {
		username, err := s.lookupUsername(ctx, usernames, record.UserID)
		if err != nil {
			return nil, err
		}

		items = append(items, toAPISession(record, username, currentSessionID))
	}

	return &api.AuthSessionPage{
		Items: items,
		Pagination: api.AuthSessionPagination{
			Limit:  limit,
			Offset: offset,
			Total:  totalCount,
		},
	}, nil
}

func (s *Service) ValidateSession(ctx context.Context, sessionID string) (bool, error) {
	id, ok := parseUUID(sessionID)
	if !ok {
		return false, nil
	}

	record, err := s.queries.GetAuthSessionByID(ctx, db.GetAuthSessionByIDParams{
		ID: pgUUID(id),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, err
	}

	return s.isSessionActive(record, s.now()), nil
}

func (s *Service) RevokeAdminSession(ctx context.Context, sessionID uuid.UUID) error {
	record, err := s.queries.GetAuthSessionByID(ctx, db.GetAuthSessionByIDParams{
		ID: pgUUID(sessionID),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		return err
	}
	if record.RevokedAt.Valid || !s.isSessionActive(record, s.now()) {
		return nil
	}

	_, err = s.queries.RevokeAuthSession(ctx, db.RevokeAuthSessionParams{
		ID:           pgUUID(sessionID),
		RevokeReason: "admin_revoke",
	})
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}

	return nil
}

func (s *Service) RevokeAllAdminSessions(ctx context.Context) error {
	_, err := s.queries.RevokeAllActiveAuthSessions(ctx, db.RevokeAllActiveAuthSessionsParams{
		RevokeReason: "admin_revoke_all",
	})
	return err
}

func (s *Service) toAPISessions(rows []db.AuthSession, username, currentSessionID string) []api.AuthSession {
	items := make([]api.AuthSession, 0, len(rows))
	now := s.now()
	for _, record := range rows {
		if !s.isSessionActive(record, now) {
			continue
		}

		items = append(items, toAPISession(record, username, currentSessionID))
	}

	return items
}

func (s *Service) lookupUsername(ctx context.Context, cache map[[16]byte]string, userID pgtype.UUID) (string, error) {
	if !userID.Valid {
		return "", nil
	}
	if username, ok := cache[userID.Bytes]; ok {
		return username, nil
	}

	user, err := s.queries.GetUserByID(ctx, db.GetUserByIDParams{ID: userID})
	if err != nil {
		return "", err
	}
	cache[userID.Bytes] = user.Username
	return user.Username, nil
}

func (s *Service) isSessionActive(record db.AuthSession, now time.Time) bool {
	if record.RevokedAt.Valid {
		return false
	}
	if !record.ExpiresAt.Valid || !record.IdleExpiresAt.Valid {
		return false
	}
	if !record.ExpiresAt.Time.After(now) {
		return false
	}
	if !record.IdleExpiresAt.Time.After(now) {
		return false
	}
	return true
}

func toAPISession(record db.AuthSession, username, currentSessionID string) api.AuthSession {
	item := api.AuthSession{
		ClientName:    record.ClientName,
		CreatedAt:     record.CreatedAt.Time,
		ExpiresAt:     record.ExpiresAt.Time,
		Id:            openapi_types.UUID(uuid.UUID(record.ID.Bytes)),
		IdleExpiresAt: record.IdleExpiresAt.Time,
		IsCurrent:     uuid.UUID(record.ID.Bytes).String() == currentSessionID,
		Username:      username,
	}
	if record.LastUsedAt.Valid {
		value := record.LastUsedAt.Time
		item.LastUsedAt = &value
	}

	return item
}

func pgUUID(id uuid.UUID) pgtype.UUID {
	return pgtype.UUID{
		Bytes: [16]byte(id),
		Valid: true,
	}
}

func parseUUID(value string) (uuid.UUID, bool) {
	id, err := uuid.Parse(value)
	if err != nil {
		return uuid.UUID{}, false
	}
	return id, true
}
