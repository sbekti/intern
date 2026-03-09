package clientauth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"io"
	"strings"
	"time"

	jwt "github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sbekti/intern-api/internal/api"
	"github.com/sbekti/intern-api/internal/auth"
	"github.com/sbekti/intern-api/internal/config"
	"github.com/sbekti/intern-api/internal/db"
)

var (
	ErrNotFound              = errors.New("resource not found")
	ErrConflict              = errors.New("resource conflict")
	ErrAuthorizationPending  = errors.New("authorization pending")
	ErrExpiredToken          = errors.New("expired token")
	ErrAccessDenied          = errors.New("access denied")
	ErrInvalidRequest        = errors.New("invalid request")
	ErrUnauthorized          = errors.New("unauthorized")
	ErrTooManyRequests       = errors.New("too many requests")
	ErrTransactorNotProvided = errors.New("transactor not configured")
)

type ValidationError struct {
	Message string
}

func (e ValidationError) Error() string {
	return e.Message
}

type Querier interface {
	CreateAuthDeviceAuthorization(ctx context.Context, arg db.CreateAuthDeviceAuthorizationParams) (db.AuthDeviceAuthorization, error)
	GetAuthDeviceAuthorizationByDeviceCode(ctx context.Context, arg db.GetAuthDeviceAuthorizationByDeviceCodeParams) (db.AuthDeviceAuthorization, error)
	GetAuthDeviceAuthorizationByUserCode(ctx context.Context, arg db.GetAuthDeviceAuthorizationByUserCodeParams) (db.AuthDeviceAuthorization, error)
	UpdateAuthDeviceAuthorizationStatus(ctx context.Context, arg db.UpdateAuthDeviceAuthorizationStatusParams) (db.AuthDeviceAuthorization, error)
	CreateAuthSession(ctx context.Context, arg db.CreateAuthSessionParams) (db.AuthSession, error)
	GetAuthSessionByRefreshTokenHash(ctx context.Context, arg db.GetAuthSessionByRefreshTokenHashParams) (db.AuthSession, error)
	GetUserByID(ctx context.Context, arg db.GetUserByIDParams) (db.User, error)
	RevokeAuthSession(ctx context.Context, arg db.RevokeAuthSessionParams) (db.AuthSession, error)
	RevokeAuthSessionFamily(ctx context.Context, arg db.RevokeAuthSessionFamilyParams) (int64, error)
}

type Transactor interface {
	InTx(ctx context.Context, fn func(q Querier) error) error
}

type Service struct {
	queries Querier
	tx      Transactor
	cfg     config.AuthConfig
	now     func() time.Time
	random  io.Reader
}

type PGXTransactor struct {
	pool *pgxpool.Pool
}

func NewService(cfg config.Config, queries Querier, tx Transactor) *Service {
	return &Service{
		queries: queries,
		tx:      tx,
		cfg:     cfg.Auth,
		now:     time.Now,
		random:  rand.Reader,
	}
}

func NewPGXTransactor(pool *pgxpool.Pool) *PGXTransactor {
	return &PGXTransactor{pool: pool}
}

func (t *PGXTransactor) InTx(ctx context.Context, fn func(q Querier) error) error {
	tx, err := t.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}

	queries := db.New(tx)
	if err := fn(queries); err != nil {
		_ = tx.Rollback(ctx)
		return err
	}

	return tx.Commit(ctx)
}

func (s *Service) CreateDeviceCode(ctx context.Context, request *api.DeviceCodeCreateRequest) (*api.DeviceCode, error) {
	clientName := "internctl"
	if request != nil && request.ClientName != nil && strings.TrimSpace(*request.ClientName) != "" {
		clientName = strings.TrimSpace(*request.ClientName)
	}

	now := s.now()
	expiresAt := now.Add(s.cfg.DeviceCodeTTL)
	for i := 0; i < 5; i++ {
		deviceCode, err := s.randomToken(32)
		if err != nil {
			return nil, err
		}
		userCode, err := s.randomUserCode()
		if err != nil {
			return nil, err
		}

		_, err = s.queries.CreateAuthDeviceAuthorization(ctx, db.CreateAuthDeviceAuthorizationParams{
			DeviceCode:      deviceCode,
			UserCode:        userCode,
			ClientName:      clientName,
			RequestedScopes: []string{"api"},
			ExpiresAt:       timestamptz(expiresAt),
			Status:          "pending",
		})
		if err == nil {
			return &api.DeviceCode{
				DeviceCode:          deviceCode,
				UserCode:            userCode,
				VerificationUrl:     s.cfg.VerificationURL,
				ExpiresInSeconds:    int32(s.cfg.DeviceCodeTTL.Seconds()),
				PollIntervalSeconds: int32(s.cfg.DevicePollInterval.Seconds()),
			}, nil
		}
		if !isUniqueViolation(err) {
			return nil, err
		}
	}

	return nil, ErrTooManyRequests
}

func (s *Service) ApproveDeviceCode(ctx context.Context, userCode string, user db.User) error {
	return s.transitionDeviceCode(ctx, userCode, user, "approved")
}

func (s *Service) DenyDeviceCode(ctx context.Context, userCode string, user db.User) error {
	return s.transitionDeviceCode(ctx, userCode, user, "denied")
}

func (s *Service) transitionDeviceCode(ctx context.Context, userCode string, user db.User, nextStatus string) error {
	if strings.TrimSpace(userCode) == "" {
		return ValidationError{Message: "user_code must not be empty"}
	}

	record, err := s.queries.GetAuthDeviceAuthorizationByUserCode(ctx, db.GetAuthDeviceAuthorizationByUserCodeParams{
		UserCode: strings.TrimSpace(userCode),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}

	if s.isExpired(record.ExpiresAt) {
		return ErrNotFound
	}
	if record.Status != "pending" {
		return ErrConflict
	}

	_, err = s.queries.UpdateAuthDeviceAuthorizationStatus(ctx, db.UpdateAuthDeviceAuthorizationStatusParams{
		ID:               record.ID,
		Status:           nextStatus,
		ApprovedByUserID: user.ID,
		ApprovedAt:       timestamptz(s.now()),
		LastPolledAt:     record.LastPolledAt,
	})
	return err
}

func (s *Service) ExchangeDeviceCode(ctx context.Context, request api.DeviceCodeTokenRequest, userAgent string) (*api.TokenResponse, error) {
	deviceCode := strings.TrimSpace(request.DeviceCode)
	if deviceCode == "" {
		return nil, ValidationError{Message: "device_code must not be empty"}
	}

	record, err := s.queries.GetAuthDeviceAuthorizationByDeviceCode(ctx, db.GetAuthDeviceAuthorizationByDeviceCodeParams{
		DeviceCode: deviceCode,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrInvalidRequest
		}
		return nil, err
	}

	now := s.now()
	if s.isExpired(record.ExpiresAt) {
		_, _ = s.queries.UpdateAuthDeviceAuthorizationStatus(ctx, db.UpdateAuthDeviceAuthorizationStatusParams{
			ID:               record.ID,
			Status:           "expired",
			ApprovedByUserID: record.ApprovedByUserID,
			ApprovedAt:       record.ApprovedAt,
			LastPolledAt:     timestamptz(now),
		})
		return nil, ErrExpiredToken
	}

	if record.LastPolledAt.Valid && now.Before(record.LastPolledAt.Time.Add(s.cfg.DevicePollInterval)) {
		return nil, ErrTooManyRequests
	}

	switch record.Status {
	case "pending":
		_, err = s.queries.UpdateAuthDeviceAuthorizationStatus(ctx, db.UpdateAuthDeviceAuthorizationStatusParams{
			ID:               record.ID,
			Status:           "pending",
			ApprovedByUserID: record.ApprovedByUserID,
			ApprovedAt:       record.ApprovedAt,
			LastPolledAt:     timestamptz(now),
		})
		if err != nil {
			return nil, err
		}
		return nil, ErrAuthorizationPending
	case "approved":
		if !record.ApprovedByUserID.Valid {
			return nil, ErrInvalidRequest
		}

		var response *api.TokenResponse
		err := s.tx.InTx(ctx, func(q Querier) error {
			user, err := q.GetUserByID(ctx, db.GetUserByIDParams{ID: record.ApprovedByUserID})
			if err != nil {
				if errors.Is(err, pgx.ErrNoRows) {
					return ErrInvalidRequest
				}
				return err
			}

			session, refreshToken, err := s.createSession(ctx, q, user, record.ClientName, userAgent, uuid.New(), now.Add(s.cfg.RefreshAbsoluteTTL))
			if err != nil {
				return err
			}

			token, err := s.mintAccessToken(user, session)
			if err != nil {
				return err
			}

			_, err = q.UpdateAuthDeviceAuthorizationStatus(ctx, db.UpdateAuthDeviceAuthorizationStatusParams{
				ID:               record.ID,
				Status:           "exchanged",
				ApprovedByUserID: record.ApprovedByUserID,
				ApprovedAt:       record.ApprovedAt,
				LastPolledAt:     timestamptz(now),
			})
			if err != nil {
				return err
			}

			response = &api.TokenResponse{
				AccessToken:      token,
				TokenType:        "Bearer",
				ExpiresInSeconds: int32(s.cfg.AccessTokenTTL.Seconds()),
				RefreshToken:     refreshToken,
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
		return response, nil
	case "denied":
		return nil, ErrAccessDenied
	case "expired":
		return nil, ErrExpiredToken
	default:
		return nil, ErrInvalidRequest
	}
}

func (s *Service) RefreshAccessToken(ctx context.Context, request api.RefreshTokenRequest, userAgent string) (*api.TokenResponse, error) {
	refreshToken := strings.TrimSpace(request.RefreshToken)
	if refreshToken == "" {
		return nil, ValidationError{Message: "refresh_token must not be empty"}
	}

	session, err := s.queries.GetAuthSessionByRefreshTokenHash(ctx, db.GetAuthSessionByRefreshTokenHashParams{
		RefreshTokenHash: hashToken(refreshToken),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUnauthorized
		}
		return nil, err
	}

	now := s.now()
	if session.RevokedAt.Valid {
		_ = s.revokeFamily(ctx, session.RefreshTokenFamilyID, "refresh_token_reuse")
		return nil, ErrUnauthorized
	}
	if now.After(session.ExpiresAt.Time) || now.After(session.IdleExpiresAt.Time) {
		_, _ = s.queries.RevokeAuthSession(ctx, db.RevokeAuthSessionParams{
			ID:           session.ID,
			RevokeReason: "expired",
		})
		return nil, ErrUnauthorized
	}

	user, err := s.queries.GetUserByID(ctx, db.GetUserByIDParams{ID: session.UserID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUnauthorized
		}
		return nil, err
	}

	var response *api.TokenResponse
	err = s.tx.InTx(ctx, func(q Querier) error {
		_, err := q.RevokeAuthSession(ctx, db.RevokeAuthSessionParams{
			ID:           session.ID,
			RevokeReason: "rotated",
		})
		if err != nil {
			return err
		}

		nextUserAgent := strings.TrimSpace(userAgent)
		if nextUserAgent == "" {
			nextUserAgent = session.UserAgent
		}

		nextSession, nextRefreshToken, err := s.createSession(ctx, q, user, session.ClientName, nextUserAgent, uuid.UUID(session.RefreshTokenFamilyID.Bytes), session.ExpiresAt.Time)
		if err != nil {
			return err
		}

		accessToken, err := s.mintAccessToken(user, nextSession)
		if err != nil {
			return err
		}

		response = &api.TokenResponse{
			AccessToken:      accessToken,
			TokenType:        "Bearer",
			ExpiresInSeconds: int32(s.cfg.AccessTokenTTL.Seconds()),
			RefreshToken:     nextRefreshToken,
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return response, nil
}

func (s *Service) Logout(ctx context.Context, request api.LogoutRequest) error {
	refreshToken := strings.TrimSpace(request.RefreshToken)
	if refreshToken == "" {
		return ValidationError{Message: "refresh_token must not be empty"}
	}

	session, err := s.queries.GetAuthSessionByRefreshTokenHash(ctx, db.GetAuthSessionByRefreshTokenHashParams{
		RefreshTokenHash: hashToken(refreshToken),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		return err
	}
	if session.RevokedAt.Valid {
		return nil
	}

	_, err = s.queries.RevokeAuthSession(ctx, db.RevokeAuthSessionParams{
		ID:           session.ID,
		RevokeReason: "logout",
	})
	return err
}

func (s *Service) createSession(ctx context.Context, q Querier, user db.User, clientName, userAgent string, familyID uuid.UUID, absoluteExpiry time.Time) (db.AuthSession, string, error) {
	for i := 0; i < 5; i++ {
		refreshToken, err := s.randomToken(32)
		if err != nil {
			return db.AuthSession{}, "", err
		}
		session, err := q.CreateAuthSession(ctx, db.CreateAuthSessionParams{
			UserID:               user.ID,
			ClientName:           clientName,
			UserAgent:            strings.TrimSpace(userAgent),
			RefreshTokenHash:     hashToken(refreshToken),
			RefreshTokenFamilyID: pgUUIDFromUUID(familyID),
			LastUsedAt:           timestamptz(s.now()),
			ExpiresAt:            timestamptz(absoluteExpiry),
			IdleExpiresAt:        timestamptz(s.now().Add(s.cfg.RefreshIdleTTL)),
		})
		if err == nil {
			return session, refreshToken, nil
		}
		if !isUniqueViolation(err) {
			return db.AuthSession{}, "", err
		}
	}
	return db.AuthSession{}, "", ErrTooManyRequests
}

func (s *Service) mintAccessToken(user db.User, session db.AuthSession) (string, error) {
	claims := auth.AccessTokenClaims{
		Username:  user.Username,
		Name:      user.Name,
		Email:     user.Email,
		Groups:    append([]string(nil), user.Groups...),
		Scopes:    []string{"api"},
		SessionID: uuid.UUID(session.ID.Bytes).String(),
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   user.Username,
			Issuer:    s.cfg.JWTIssuer,
			Audience:  []string{s.cfg.JWTAudience},
			ExpiresAt: jwt.NewNumericDate(s.now().Add(s.cfg.AccessTokenTTL)),
			IssuedAt:  jwt.NewNumericDate(s.now()),
			NotBefore: jwt.NewNumericDate(s.now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.cfg.JWTHMACSecret))
}

func (s *Service) randomToken(bytesLen int) (string, error) {
	buf := make([]byte, bytesLen)
	if _, err := io.ReadFull(s.random, buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func (s *Service) randomUserCode() (string, error) {
	const alphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	buf := make([]byte, 8)
	if _, err := io.ReadFull(s.random, buf); err != nil {
		return "", err
	}
	for i := range buf {
		buf[i] = alphabet[int(buf[i])%len(alphabet)]
	}
	return string(buf[:4]) + "-" + string(buf[4:]), nil
}

func (s *Service) revokeFamily(ctx context.Context, familyID pgtype.UUID, reason string) error {
	if !familyID.Valid {
		return nil
	}
	_, err := s.queries.RevokeAuthSessionFamily(ctx, db.RevokeAuthSessionFamilyParams{
		RefreshTokenFamilyID: familyID,
		RevokeReason:         reason,
	})
	return err
}

func (s *Service) isExpired(value pgtype.Timestamptz) bool {
	return value.Valid && s.now().After(value.Time)
}

func hashToken(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func timestamptz(value time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: value.UTC(), Valid: true}
}

func pgUUIDFromUUID(value uuid.UUID) pgtype.UUID {
	var raw [16]byte
	copy(raw[:], value[:])
	return pgtype.UUID{Bytes: raw, Valid: true}
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
