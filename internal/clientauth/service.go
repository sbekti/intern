package clientauth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/url"
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
	"github.com/sbekti/intern-api/internal/requestmeta"
)

var (
	ErrNotFound              = errors.New("resource not found")
	ErrConflict              = errors.New("resource conflict")
	ErrAuthorizationPending  = errors.New("authorization pending")
	ErrSlowDown              = errors.New("slow down")
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
	CreateAuditLog(ctx context.Context, arg db.CreateAuditLogParams) (db.AuditLog, error)
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
			verificationURI, verificationURIComplete := s.verificationURIs(userCode)
			return &api.DeviceCode{
				DeviceCode:              deviceCode,
				UserCode:                userCode,
				VerificationUri:         verificationURI,
				VerificationUriComplete: verificationURIComplete,
				ExpiresIn:               int32(s.cfg.DeviceCodeTTL.Seconds()),
				Interval:                int32(s.cfg.DevicePollInterval.Seconds()),
			}, nil
		}
		if !isUniqueViolation(err) {
			return nil, err
		}
	}

	return nil, ErrTooManyRequests
}

func (s *Service) verificationURIs(userCode string) (string, string) {
	baseURL := strings.TrimSpace(s.cfg.PublicBaseURL)
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return strings.TrimRight(baseURL, "/") + "/auth/device", strings.TrimRight(baseURL, "/") + "/auth/device?user_code=" + url.QueryEscape(userCode)
	}

	parsed.Path = strings.TrimRight(parsed.Path, "/") + "/auth/device"
	parsed.RawQuery = ""
	parsed.Fragment = ""
	verificationURI := parsed.String()

	query := parsed.Query()
	query.Set("user_code", userCode)
	parsed.RawQuery = query.Encode()

	return verificationURI, parsed.String()
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

	if s.tx == nil {
		return ErrTransactorNotProvided
	}

	return s.tx.InTx(ctx, func(q Querier) error {
		record, err := q.GetAuthDeviceAuthorizationByUserCode(ctx, db.GetAuthDeviceAuthorizationByUserCodeParams{
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

		updated, err := q.UpdateAuthDeviceAuthorizationStatus(ctx, db.UpdateAuthDeviceAuthorizationStatusParams{
			ID:               record.ID,
			Status:           nextStatus,
			ApprovedByUserID: user.ID,
			ApprovedAt:       timestamptz(s.now()),
			LastPolledAt:     record.LastPolledAt,
		})
		if err != nil {
			return err
		}

		return s.writeAuditLog(ctx, q, &user, "auth.device_code."+deviceCodeTransitionAction(nextStatus), "auth_device_authorization", uuidString(updated.ID), map[string]any{
			"device_flow": true,
			"user_code":   updated.UserCode,
			"client_name": updated.ClientName,
			"status":      nextStatus,
		})
	})
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
		return nil, ErrSlowDown
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

			if err := s.writeAuditLog(ctx, q, &user, "auth.device_code.exchange", "auth_session", uuidString(session.ID), map[string]any{
				"device_flow":               true,
				"client_name":               record.ClientName,
				"user_code":                 record.UserCode,
				"auth_device_authorization": uuidString(record.ID),
			}); err != nil {
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
		user, _ := s.lookupAuditActor(ctx, s.queries, session.UserID)
		_ = s.revokeFamily(ctx, session.RefreshTokenFamilyID, "refresh_token_reuse")
		_ = s.writeAuditLog(ctx, s.queries, user, "auth.session.family_revoke", "auth_session_family", uuidString(session.RefreshTokenFamilyID), map[string]any{
			"client_name":   session.ClientName,
			"revoke_reason": "refresh_token_reuse",
			"session_id":    uuidString(session.ID),
		})
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

	actor, _ := s.lookupAuditActor(ctx, s.queries, session.UserID)

	_, err = s.queries.RevokeAuthSession(ctx, db.RevokeAuthSessionParams{
		ID:           session.ID,
		RevokeReason: "logout",
	})
	if err != nil {
		return err
	}

	return s.writeAuditLog(ctx, s.queries, actor, "auth.session.logout", "auth_session", uuidString(session.ID), map[string]any{
		"client_name": session.ClientName,
	})
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

func (s *Service) lookupAuditActor(ctx context.Context, q Querier, userID pgtype.UUID) (*db.User, error) {
	if !userID.Valid {
		return nil, nil
	}

	user, err := q.GetUserByID(ctx, db.GetUserByIDParams{ID: userID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &user, nil
}

func (s *Service) writeAuditLog(ctx context.Context, q Querier, actor *db.User, action, resourceType, resourceID string, metadata map[string]any) error {
	if metadata == nil {
		metadata = map[string]any{}
	}

	if clientInfo, ok := requestmeta.FromContext(ctx); ok {
		if strings.TrimSpace(clientInfo.IP) != "" {
			metadata["client_ip"] = clientInfo.IP
		}
		if strings.TrimSpace(clientInfo.IPSource) != "" {
			metadata["client_ip_source"] = clientInfo.IPSource
		}
	}

	payload, err := json.Marshal(metadata)
	if err != nil {
		return err
	}

	params := db.CreateAuditLogParams{
		ActorUsername: "",
		Action:        action,
		ResourceType:  resourceType,
		ResourceID:    resourceID,
		Metadata:      payload,
	}
	if actor != nil {
		params.ActorUserID = actor.ID
		params.ActorUsername = actor.Username
	}

	_, err = q.CreateAuditLog(ctx, params)
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

func uuidString(value pgtype.UUID) string {
	if !value.Valid {
		return ""
	}
	return uuid.UUID(value.Bytes).String()
}

func deviceCodeTransitionAction(status string) string {
	switch status {
	case "approved":
		return "approve"
	case "denied":
		return "deny"
	default:
		return status
	}
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
