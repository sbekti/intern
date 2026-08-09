package httpserver

import (
	"context"

	"github.com/google/uuid"
	"github.com/sbekti/intern-api/internal/api"
	"github.com/sbekti/intern-api/internal/auditlogs"
	"github.com/sbekti/intern-api/internal/db"
	"github.com/sbekti/intern-api/internal/devices"
	"github.com/sbekti/intern-api/internal/identity"
	"github.com/sbekti/intern-api/internal/requestmeta"
)

type response struct {
	Status  string `json:"status"`
	Service string `json:"service,omitempty"`
	User    string `json:"user,omitempty"`
}

type Dependencies struct {
	UserStore         identity.UserStore
	DatabasePinger    DatabasePinger
	VLANService       VLANService
	DeviceService     DeviceService
	ClientAuthService ClientAuthService
	AuthSpamService   AuthSpamService
	SessionService    SessionService
	AuditLogService   AuditLogService
}

type DatabasePinger interface {
	Ping(context.Context) error
}

type VLANService interface {
	List(ctx context.Context) ([]db.Vlan, error)
	Get(ctx context.Context, vlanID int32) (db.Vlan, error)
	Create(ctx context.Context, actor db.User, input api.VlanWrite) (db.Vlan, error)
	Update(ctx context.Context, actor db.User, vlanID int32, patch api.VlanPatch) (db.Vlan, error)
	Delete(ctx context.Context, actor db.User, vlanID int32) error
}

type DeviceService interface {
	List(ctx context.Context) ([]devices.DeviceRecord, error)
	Get(ctx context.Context, id uuid.UUID) (devices.DeviceRecord, error)
	Create(ctx context.Context, actor db.User, input api.NetworkDeviceWrite) (devices.DeviceRecord, error)
	Update(ctx context.Context, actor db.User, id uuid.UUID, patch api.NetworkDevicePatch) (devices.DeviceRecord, error)
	Delete(ctx context.Context, actor db.User, id uuid.UUID) error
}

type ClientAuthService interface {
	CreateDeviceCode(ctx context.Context, request *api.DeviceCodeCreateRequest) (*api.DeviceCode, error)
	ApproveDeviceCode(ctx context.Context, userCode string, user db.User) error
	DenyDeviceCode(ctx context.Context, userCode string, user db.User) error
	ExchangeDeviceCode(ctx context.Context, request api.DeviceCodeTokenRequest, userAgent string) (*api.TokenResponse, error)
	RefreshAccessToken(ctx context.Context, request api.RefreshTokenRequest, userAgent string) (*api.TokenResponse, error)
	Logout(ctx context.Context, request api.LogoutRequest) error
}

type AuthSpamService interface {
	CheckDeviceCodeCreate(ctx context.Context, clientInfo requestmeta.ClientInfo) error
	CheckDeviceTokenExchange(ctx context.Context, clientInfo requestmeta.ClientInfo) error
	CheckDeviceDecision(ctx context.Context, username string, clientInfo requestmeta.ClientInfo) error
	CheckRefreshToken(ctx context.Context, clientInfo requestmeta.ClientInfo) error
	CheckLogout(ctx context.Context, clientInfo requestmeta.ClientInfo) error
}

type SessionService interface {
	ValidateSession(ctx context.Context, sessionID string) (bool, error)
	ListProfileSessionsPage(ctx context.Context, user db.User, currentSessionID string, limit, offset int32) (*api.AuthSessionPage, error)
	RevokeProfileSession(ctx context.Context, user db.User, sessionID uuid.UUID) error
	RevokeOtherProfileSessions(ctx context.Context, user db.User, currentSessionID string) error
	ListAdminSessionsPage(ctx context.Context, currentSessionID string, limit, offset int32) (*api.AuthSessionPage, error)
	RevokeAdminSession(ctx context.Context, sessionID uuid.UUID) error
	RevokeAllAdminSessions(ctx context.Context) error
}

type AuditLogService interface {
	List(ctx context.Context, filter auditlogs.Filter) (*auditlogs.Page, error)
}

const (
	defaultAuthSessionLimit int32 = 25
	maxAuthSessionLimit     int32 = 200
)

type authSessionPageParams struct {
	Limit  *int32
	Offset *int32
}
