package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"
	"github.com/sbekti/intern-api/internal/api"
	"github.com/sbekti/intern-api/internal/auditlogs"
	"github.com/sbekti/intern-api/internal/auth"
	"github.com/sbekti/intern-api/internal/clientauth"
	"github.com/sbekti/intern-api/internal/config"
	"github.com/sbekti/intern-api/internal/dashboard"
	"github.com/sbekti/intern-api/internal/db"
	"github.com/sbekti/intern-api/internal/devices"
	"github.com/sbekti/intern-api/internal/identity"
	"github.com/sbekti/intern-api/internal/vlans"
)

type response struct {
	Status  string `json:"status"`
	Service string `json:"service,omitempty"`
	User    string `json:"user,omitempty"`
}

type Dependencies struct {
	UserStore         identity.UserStore
	DashboardStore    dashboard.Querier
	WeatherService    dashboard.WeatherService
	VLANService       VLANService
	DeviceService     DeviceService
	ClientAuthService ClientAuthService
	SessionService    SessionService
	AuditLogService   AuditLogService
}

type VLANService interface {
	List(ctx context.Context) ([]db.Vlan, error)
	Get(ctx context.Context, id int64) (db.Vlan, error)
	Create(ctx context.Context, actor db.User, input api.VlanWrite) (db.Vlan, error)
	Update(ctx context.Context, actor db.User, id int64, patch api.VlanPatch) (db.Vlan, error)
	Delete(ctx context.Context, actor db.User, id int64) error
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

func NewHandler(logger *slog.Logger, cfg config.Config, deps Dependencies) http.Handler {
	if logger == nil {
		logger = slog.Default()
	}

	authenticator := auth.NewAuthenticator(cfg)
	authorizer := auth.NewAuthorizer(cfg)
	userSyncer := identity.NewSyncer(deps.UserStore)
	dashboardService := dashboard.NewService(deps.DashboardStore, deps.WeatherService)
	vlanService := deps.VLANService
	deviceService := deps.DeviceService
	clientAuthService := deps.ClientAuthService
	sessionService := deps.SessionService
	auditLogService := deps.AuditLogService

	router := chi.NewRouter()
	router.Use(middleware.RequestID)
	router.Use(middleware.RealIP)
	router.Use(requestLogger(logger))
	router.Use(middleware.Recoverer)
	router.Use(authenticator.OptionalPrincipalMiddleware())
	router.Use(auth.RequireActiveBearerSession(deps.SessionService))
	router.Use(userSyncer.Middleware())

	router.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, response{Status: "ok"})
	})

	router.Get("/readyz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, response{Status: "ok"})
	})

	router.Route("/api/v1", func(r chi.Router) {
		r.Get("/system/ping", func(w http.ResponseWriter, r *http.Request) {
			principal, ok := auth.FromContext(r.Context())
			var username string
			if ok {
				username = principal.Username
			}
			writeJSON(w, http.StatusOK, response{
				Status:  "ok",
				Service: "intern-api",
				User:    username,
			})
		})

		r.With(authorizer.RequireAuthenticated()).Get("/dashboard", func(w http.ResponseWriter, r *http.Request) {
			user, ok := identity.FromContext(r.Context())
			if !ok {
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}

			principal, _ := auth.FromContext(r.Context())
			payload, err := dashboardService.Build(r.Context(), user, principal, authorizer.IsAdmin(principal))
			if err != nil {
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}

			writeJSON(w, http.StatusOK, payload)
		})

		r.With(authorizer.RequireAuthenticated()).Get("/profile", func(w http.ResponseWriter, r *http.Request) {
			user, ok := identity.FromContext(r.Context())
			if !ok {
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}

			writeJSON(w, http.StatusOK, api.Profile{
				Username: user.Username,
				Name:     user.Name,
				Email:    openapi_types.Email(user.Email),
				Groups:   append([]string(nil), user.Groups...),
				IsAdmin:  authorizer.IsAdmin(&auth.Principal{Groups: user.Groups}),
			})
		})

		r.With(authorizer.RequireAuthenticated()).Get("/profile/sessions", func(w http.ResponseWriter, r *http.Request) {
			if sessionService == nil {
				writeAPIError(w, http.StatusInternalServerError, "internal_error", "session service not configured")
				return
			}

			params, err := decodeAuthSessionPageParams(r)
			if err != nil {
				writeAPIError(w, http.StatusBadRequest, "bad_request", err.Error())
				return
			}

			user, ok := identity.FromContext(r.Context())
			if !ok {
				writeAPIError(w, http.StatusInternalServerError, "internal_error", "current user missing")
				return
			}
			principal, _ := auth.FromContext(r.Context())

			page, err := sessionService.ListProfileSessionsPage(
				r.Context(),
				user,
				currentSessionID(principal),
				int32Value(params.Limit, defaultAuthSessionLimit),
				int32Value(params.Offset, 0),
			)
			if err != nil {
				writeAPIError(w, http.StatusInternalServerError, "internal_error", "failed to list sessions")
				return
			}

			writeJSON(w, http.StatusOK, page)
		})

		r.With(authorizer.RequireAuthenticated()).Post("/profile/sessions/{id}/revoke", func(w http.ResponseWriter, r *http.Request) {
			if sessionService == nil {
				writeAPIError(w, http.StatusInternalServerError, "internal_error", "session service not configured")
				return
			}

			user, ok := identity.FromContext(r.Context())
			if !ok {
				writeAPIError(w, http.StatusInternalServerError, "internal_error", "current user missing")
				return
			}

			id, err := decodeUUIDPathParam(r, "id")
			if err != nil {
				writeAPIError(w, http.StatusBadRequest, "bad_request", "invalid session id")
				return
			}

			if err := sessionService.RevokeProfileSession(r.Context(), user, id); err != nil {
				writeAPIError(w, http.StatusInternalServerError, "internal_error", "failed to revoke session")
				return
			}

			w.WriteHeader(http.StatusNoContent)
		})

		r.With(authorizer.RequireAuthenticated()).Post("/profile/sessions/revoke_others", func(w http.ResponseWriter, r *http.Request) {
			if sessionService == nil {
				writeAPIError(w, http.StatusInternalServerError, "internal_error", "session service not configured")
				return
			}

			user, ok := identity.FromContext(r.Context())
			if !ok {
				writeAPIError(w, http.StatusInternalServerError, "internal_error", "current user missing")
				return
			}
			principal, _ := auth.FromContext(r.Context())

			if err := sessionService.RevokeOtherProfileSessions(r.Context(), user, currentSessionID(principal)); err != nil {
				writeAPIError(w, http.StatusInternalServerError, "internal_error", "failed to revoke sessions")
				return
			}

			w.WriteHeader(http.StatusNoContent)
		})

		r.With(authorizer.RequireAdmin()).Get("/admin/auth/sessions", func(w http.ResponseWriter, r *http.Request) {
			if sessionService == nil {
				writeAPIError(w, http.StatusInternalServerError, "internal_error", "session service not configured")
				return
			}

			params, err := decodeAuthSessionPageParams(r)
			if err != nil {
				writeAPIError(w, http.StatusBadRequest, "bad_request", err.Error())
				return
			}

			principal, _ := auth.FromContext(r.Context())
			page, err := sessionService.ListAdminSessionsPage(
				r.Context(),
				currentSessionID(principal),
				int32Value(params.Limit, defaultAuthSessionLimit),
				int32Value(params.Offset, 0),
			)
			if err != nil {
				writeAPIError(w, http.StatusInternalServerError, "internal_error", "failed to list admin sessions")
				return
			}

			writeJSON(w, http.StatusOK, page)
		})

		r.With(authorizer.RequireAdmin()).Post("/admin/auth/sessions/revoke_all", func(w http.ResponseWriter, r *http.Request) {
			if sessionService == nil {
				writeAPIError(w, http.StatusInternalServerError, "internal_error", "session service not configured")
				return
			}

			if err := sessionService.RevokeAllAdminSessions(r.Context()); err != nil {
				writeAPIError(w, http.StatusInternalServerError, "internal_error", "failed to revoke admin sessions")
				return
			}

			w.WriteHeader(http.StatusNoContent)
		})

		r.With(authorizer.RequireAdmin()).Post("/admin/auth/sessions/{id}/revoke", func(w http.ResponseWriter, r *http.Request) {
			if sessionService == nil {
				writeAPIError(w, http.StatusInternalServerError, "internal_error", "session service not configured")
				return
			}

			id, err := decodeUUIDPathParam(r, "id")
			if err != nil {
				writeAPIError(w, http.StatusBadRequest, "bad_request", "invalid session id")
				return
			}

			if err := sessionService.RevokeAdminSession(r.Context(), id); err != nil {
				writeAPIError(w, http.StatusInternalServerError, "internal_error", "failed to revoke admin session")
				return
			}

			w.WriteHeader(http.StatusNoContent)
		})

		r.With(authorizer.RequireAdmin()).Get("/admin/audit_logs", func(w http.ResponseWriter, r *http.Request) {
			if auditLogService == nil {
				writeAPIError(w, http.StatusInternalServerError, "internal_error", "audit log service not configured")
				return
			}

			params, err := decodeAdminAuditLogParams(r)
			if err != nil {
				writeAPIError(w, http.StatusBadRequest, "bad_request", err.Error())
				return
			}

			page, err := auditLogService.List(r.Context(), auditlogs.Filter{
				Action:        trimmedString(params.Action),
				ResourceType:  trimmedString(params.ResourceType),
				ResourceID:    trimmedString(params.ResourceId),
				ActorUsername: trimmedString(params.ActorUsername),
				Limit:         int32Value(params.Limit, auditlogs.DefaultLimit),
				Offset:        int32Value(params.Offset, 0),
			})
			if err != nil {
				writeAPIError(w, http.StatusInternalServerError, "internal_error", "failed to list audit logs")
				return
			}

			writeJSON(w, http.StatusOK, api.AuditLogList{
				Items: page.Items,
				Pagination: api.AuditLogPagination{
					Limit:  page.Limit,
					Offset: page.Offset,
					Total:  page.TotalCount,
				},
			})
		})

		r.With(authorizer.RequireAdmin()).Get("/networks/vlans", func(w http.ResponseWriter, r *http.Request) {
			if vlanService == nil {
				writeAPIError(w, http.StatusInternalServerError, "internal_error", "vlan service not configured")
				return
			}

			items, err := vlanService.List(r.Context())
			if err != nil {
				writeAPIError(w, http.StatusInternalServerError, "internal_error", "failed to list vlans")
				return
			}

			responseItems := make([]api.Vlan, 0, len(items))
			for _, item := range items {
				responseItems = append(responseItems, toAPIVlan(item))
			}

			writeJSON(w, http.StatusOK, api.VlanList{Items: responseItems})
		})

		r.With(authorizer.RequireAdmin()).Get("/networks/vlans/{id}", func(w http.ResponseWriter, r *http.Request) {
			if vlanService == nil {
				writeAPIError(w, http.StatusInternalServerError, "internal_error", "vlan service not configured")
				return
			}

			id, err := decodeInt64PathParam(r, "id")
			if err != nil {
				writeAPIError(w, http.StatusBadRequest, "bad_request", "invalid vlan id")
				return
			}

			vlan, err := vlanService.Get(r.Context(), id)
			if err != nil {
				if errors.Is(err, vlans.ErrNotFound) {
					writeAPIError(w, http.StatusNotFound, "not_found", "vlan not found")
					return
				}
				writeAPIError(w, http.StatusInternalServerError, "internal_error", "failed to load vlan")
				return
			}

			writeJSON(w, http.StatusOK, toAPIVlan(vlan))
		})

		r.With(authorizer.RequireAdmin()).Post("/networks/vlans", func(w http.ResponseWriter, r *http.Request) {
			if vlanService == nil {
				writeAPIError(w, http.StatusInternalServerError, "internal_error", "vlan service not configured")
				return
			}

			actor, ok := identity.FromContext(r.Context())
			if !ok {
				writeAPIError(w, http.StatusInternalServerError, "internal_error", "current user missing")
				return
			}

			var body api.VlanWrite
			if err := decodeJSON(r, &body); err != nil {
				writeAPIError(w, http.StatusBadRequest, "bad_request", "invalid request body")
				return
			}

			created, err := vlanService.Create(r.Context(), actor, body)
			if err != nil {
				handleVLANError(w, err)
				return
			}

			writeJSON(w, http.StatusCreated, toAPIVlan(created))
		})

		r.With(authorizer.RequireAdmin()).Patch("/networks/vlans/{id}", func(w http.ResponseWriter, r *http.Request) {
			if vlanService == nil {
				writeAPIError(w, http.StatusInternalServerError, "internal_error", "vlan service not configured")
				return
			}

			actor, ok := identity.FromContext(r.Context())
			if !ok {
				writeAPIError(w, http.StatusInternalServerError, "internal_error", "current user missing")
				return
			}

			id, err := decodeInt64PathParam(r, "id")
			if err != nil {
				writeAPIError(w, http.StatusBadRequest, "bad_request", "invalid vlan id")
				return
			}

			var body api.VlanPatch
			if err := decodeJSON(r, &body); err != nil {
				writeAPIError(w, http.StatusBadRequest, "bad_request", "invalid request body")
				return
			}

			updated, err := vlanService.Update(r.Context(), actor, id, body)
			if err != nil {
				handleVLANError(w, err)
				return
			}

			writeJSON(w, http.StatusOK, toAPIVlan(updated))
		})

		r.With(authorizer.RequireAdmin()).Delete("/networks/vlans/{id}", func(w http.ResponseWriter, r *http.Request) {
			if vlanService == nil {
				writeAPIError(w, http.StatusInternalServerError, "internal_error", "vlan service not configured")
				return
			}

			actor, ok := identity.FromContext(r.Context())
			if !ok {
				writeAPIError(w, http.StatusInternalServerError, "internal_error", "current user missing")
				return
			}

			id, err := decodeInt64PathParam(r, "id")
			if err != nil {
				writeAPIError(w, http.StatusBadRequest, "bad_request", "invalid vlan id")
				return
			}

			if err := vlanService.Delete(r.Context(), actor, id); err != nil {
				handleVLANError(w, err)
				return
			}

			w.WriteHeader(http.StatusNoContent)
		})

		r.With(authorizer.RequireAdmin()).Get("/networks/devices", func(w http.ResponseWriter, r *http.Request) {
			if deviceService == nil {
				writeAPIError(w, http.StatusInternalServerError, "internal_error", "device service not configured")
				return
			}

			items, err := deviceService.List(r.Context())
			if err != nil {
				writeAPIError(w, http.StatusInternalServerError, "internal_error", "failed to list devices")
				return
			}

			responseItems := make([]api.NetworkDevice, 0, len(items))
			for _, item := range items {
				responseItems = append(responseItems, toAPINetworkDevice(item))
			}

			writeJSON(w, http.StatusOK, api.NetworkDeviceList{Items: responseItems})
		})

		r.With(authorizer.RequireAdmin()).Get("/networks/devices/{id}", func(w http.ResponseWriter, r *http.Request) {
			if deviceService == nil {
				writeAPIError(w, http.StatusInternalServerError, "internal_error", "device service not configured")
				return
			}

			id, err := decodeUUIDPathParam(r, "id")
			if err != nil {
				writeAPIError(w, http.StatusBadRequest, "bad_request", "invalid device id")
				return
			}

			record, err := deviceService.Get(r.Context(), id)
			if err != nil {
				handleDeviceError(w, err)
				return
			}

			writeJSON(w, http.StatusOK, toAPINetworkDevice(record))
		})

		r.With(authorizer.RequireAdmin()).Post("/networks/devices", func(w http.ResponseWriter, r *http.Request) {
			if deviceService == nil {
				writeAPIError(w, http.StatusInternalServerError, "internal_error", "device service not configured")
				return
			}

			actor, ok := identity.FromContext(r.Context())
			if !ok {
				writeAPIError(w, http.StatusInternalServerError, "internal_error", "current user missing")
				return
			}

			var body api.NetworkDeviceWrite
			if err := decodeJSON(r, &body); err != nil {
				writeAPIError(w, http.StatusBadRequest, "bad_request", "invalid request body")
				return
			}

			record, err := deviceService.Create(r.Context(), actor, body)
			if err != nil {
				handleDeviceError(w, err)
				return
			}

			writeJSON(w, http.StatusCreated, toAPINetworkDevice(record))
		})

		r.With(authorizer.RequireAdmin()).Patch("/networks/devices/{id}", func(w http.ResponseWriter, r *http.Request) {
			if deviceService == nil {
				writeAPIError(w, http.StatusInternalServerError, "internal_error", "device service not configured")
				return
			}

			actor, ok := identity.FromContext(r.Context())
			if !ok {
				writeAPIError(w, http.StatusInternalServerError, "internal_error", "current user missing")
				return
			}

			id, err := decodeUUIDPathParam(r, "id")
			if err != nil {
				writeAPIError(w, http.StatusBadRequest, "bad_request", "invalid device id")
				return
			}

			var body api.NetworkDevicePatch
			if err := decodeJSON(r, &body); err != nil {
				writeAPIError(w, http.StatusBadRequest, "bad_request", "invalid request body")
				return
			}

			record, err := deviceService.Update(r.Context(), actor, id, body)
			if err != nil {
				handleDeviceError(w, err)
				return
			}

			writeJSON(w, http.StatusOK, toAPINetworkDevice(record))
		})

		r.With(authorizer.RequireAdmin()).Delete("/networks/devices/{id}", func(w http.ResponseWriter, r *http.Request) {
			if deviceService == nil {
				writeAPIError(w, http.StatusInternalServerError, "internal_error", "device service not configured")
				return
			}

			actor, ok := identity.FromContext(r.Context())
			if !ok {
				writeAPIError(w, http.StatusInternalServerError, "internal_error", "current user missing")
				return
			}

			id, err := decodeUUIDPathParam(r, "id")
			if err != nil {
				writeAPIError(w, http.StatusBadRequest, "bad_request", "invalid device id")
				return
			}

			if err := deviceService.Delete(r.Context(), actor, id); err != nil {
				handleDeviceError(w, err)
				return
			}

			w.WriteHeader(http.StatusNoContent)
		})

		r.Post("/auth/device_codes", func(w http.ResponseWriter, r *http.Request) {
			if clientAuthService == nil {
				writeAPIError(w, http.StatusInternalServerError, "internal_error", "client auth service not configured")
				return
			}

			var body api.DeviceCodeCreateRequest
			if r.ContentLength > 0 {
				if err := decodeJSON(r, &body); err != nil {
					writeAPIError(w, http.StatusBadRequest, "bad_request", "invalid request body")
					return
				}
			}

			result, err := clientAuthService.CreateDeviceCode(r.Context(), &body)
			if err != nil {
				handleClientAuthError(w, err)
				return
			}

			writeJSON(w, http.StatusCreated, result)
		})

		r.With(authorizer.RequireAuthenticated()).Post("/auth/device_codes/{user_code}/approve", func(w http.ResponseWriter, r *http.Request) {
			if clientAuthService == nil {
				writeAPIError(w, http.StatusInternalServerError, "internal_error", "client auth service not configured")
				return
			}

			user, ok := identity.FromContext(r.Context())
			if !ok {
				writeAPIError(w, http.StatusInternalServerError, "internal_error", "current user missing")
				return
			}

			if err := clientAuthService.ApproveDeviceCode(r.Context(), chi.URLParam(r, "user_code"), user); err != nil {
				handleClientAuthError(w, err)
				return
			}

			w.WriteHeader(http.StatusNoContent)
		})

		r.With(authorizer.RequireAuthenticated()).Post("/auth/device_codes/{user_code}/deny", func(w http.ResponseWriter, r *http.Request) {
			if clientAuthService == nil {
				writeAPIError(w, http.StatusInternalServerError, "internal_error", "client auth service not configured")
				return
			}

			user, ok := identity.FromContext(r.Context())
			if !ok {
				writeAPIError(w, http.StatusInternalServerError, "internal_error", "current user missing")
				return
			}

			if err := clientAuthService.DenyDeviceCode(r.Context(), chi.URLParam(r, "user_code"), user); err != nil {
				handleClientAuthError(w, err)
				return
			}

			w.WriteHeader(http.StatusNoContent)
		})

		r.Post("/auth/tokens", func(w http.ResponseWriter, r *http.Request) {
			if clientAuthService == nil {
				writeAPIError(w, http.StatusInternalServerError, "internal_error", "client auth service not configured")
				return
			}

			var body api.DeviceCodeTokenRequest
			if err := decodeJSON(r, &body); err != nil {
				writeAPIError(w, http.StatusBadRequest, "bad_request", "invalid request body")
				return
			}

			result, err := clientAuthService.ExchangeDeviceCode(r.Context(), body, r.UserAgent())
			if err != nil {
				handleClientAuthError(w, err)
				return
			}

			writeJSON(w, http.StatusOK, result)
		})

		r.Post("/auth/tokens/refresh", func(w http.ResponseWriter, r *http.Request) {
			if clientAuthService == nil {
				writeAPIError(w, http.StatusInternalServerError, "internal_error", "client auth service not configured")
				return
			}

			var body api.RefreshTokenRequest
			if err := decodeJSON(r, &body); err != nil {
				writeAPIError(w, http.StatusBadRequest, "bad_request", "invalid request body")
				return
			}

			result, err := clientAuthService.RefreshAccessToken(r.Context(), body, r.UserAgent())
			if err != nil {
				handleClientAuthError(w, err)
				return
			}

			writeJSON(w, http.StatusOK, result)
		})

		r.Post("/auth/logout", func(w http.ResponseWriter, r *http.Request) {
			if clientAuthService == nil {
				writeAPIError(w, http.StatusInternalServerError, "internal_error", "client auth service not configured")
				return
			}

			var body api.LogoutRequest
			if err := decodeJSON(r, &body); err != nil {
				writeAPIError(w, http.StatusBadRequest, "bad_request", "invalid request body")
				return
			}

			if err := clientAuthService.Logout(r.Context(), body); err != nil {
				handleClientAuthError(w, err)
				return
			}

			w.WriteHeader(http.StatusNoContent)
		})
	})

	return router
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeAPIError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, api.ErrorResponse{
		Code:    code,
		Message: message,
	})
}

func decodeJSON(r *http.Request, dest any) error {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(dest); err != nil {
		return err
	}
	if decoder.More() {
		return errors.New("unexpected trailing json")
	}
	return nil
}

func decodeInt64PathParam(r *http.Request, key string) (int64, error) {
	raw := chi.URLParam(r, key)
	return strconv.ParseInt(raw, 10, 64)
}

func decodeUUIDPathParam(r *http.Request, key string) (uuid.UUID, error) {
	raw := chi.URLParam(r, key)
	return uuid.Parse(raw)
}

func decodeAdminAuditLogParams(r *http.Request) (api.ListAdminAuditLogsParams, error) {
	query := r.URL.Query()
	params := api.ListAdminAuditLogsParams{}

	if value := strings.TrimSpace(query.Get("action")); value != "" {
		params.Action = &value
	}
	if value := strings.TrimSpace(query.Get("resource_type")); value != "" {
		params.ResourceType = &value
	}
	if value := strings.TrimSpace(query.Get("resource_id")); value != "" {
		params.ResourceId = &value
	}
	if value := strings.TrimSpace(query.Get("actor_username")); value != "" {
		params.ActorUsername = &value
	}
	if value := strings.TrimSpace(query.Get("limit")); value != "" {
		parsed, err := strconv.ParseInt(value, 10, 32)
		if err != nil || parsed < 1 || parsed > int64(auditlogs.MaxLimit) {
			return api.ListAdminAuditLogsParams{}, errors.New("invalid limit")
		}
		cast := int32(parsed)
		params.Limit = &cast
	}
	if value := strings.TrimSpace(query.Get("offset")); value != "" {
		parsed, err := strconv.ParseInt(value, 10, 32)
		if err != nil || parsed < 0 {
			return api.ListAdminAuditLogsParams{}, errors.New("invalid offset")
		}
		cast := int32(parsed)
		params.Offset = &cast
	}

	return params, nil
}

func decodeAuthSessionPageParams(r *http.Request) (authSessionPageParams, error) {
	query := r.URL.Query()
	params := authSessionPageParams{}

	if value := strings.TrimSpace(query.Get("limit")); value != "" {
		parsed, err := strconv.ParseInt(value, 10, 32)
		if err != nil || parsed < 1 || parsed > int64(maxAuthSessionLimit) {
			return authSessionPageParams{}, errors.New("invalid limit")
		}
		cast := int32(parsed)
		params.Limit = &cast
	}
	if value := strings.TrimSpace(query.Get("offset")); value != "" {
		parsed, err := strconv.ParseInt(value, 10, 32)
		if err != nil || parsed < 0 {
			return authSessionPageParams{}, errors.New("invalid offset")
		}
		cast := int32(parsed)
		params.Offset = &cast
	}

	return params, nil
}

func currentSessionID(principal *auth.Principal) string {
	if principal == nil {
		return ""
	}
	return principal.SessionID
}

func trimmedString(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

func int32Value(value *int32, fallback int32) int32 {
	if value == nil {
		return fallback
	}
	return *value
}

func handleVLANError(w http.ResponseWriter, err error) {
	var validationErr vlans.ValidationError
	switch {
	case errors.As(err, &validationErr):
		writeAPIError(w, http.StatusBadRequest, "bad_request", validationErr.Error())
	case errors.Is(err, vlans.ErrNotFound):
		writeAPIError(w, http.StatusNotFound, "not_found", "vlan not found")
	case errors.Is(err, vlans.ErrConflict):
		writeAPIError(w, http.StatusConflict, "conflict", "vlan conflicts with an existing record")
	default:
		writeAPIError(w, http.StatusInternalServerError, "internal_error", "vlan operation failed")
	}
}

func toAPIVlan(value db.Vlan) api.Vlan {
	return api.Vlan{
		Id:          value.ID,
		Name:        value.Name,
		VlanId:      value.VlanID,
		Description: value.Description,
		IsActive:    value.IsActive,
		CreatedAt:   value.CreatedAt.Time,
		UpdatedAt:   value.UpdatedAt.Time,
	}
}

func handleDeviceError(w http.ResponseWriter, err error) {
	var validationErr devices.ValidationError
	switch {
	case errors.As(err, &validationErr):
		writeAPIError(w, http.StatusBadRequest, "bad_request", validationErr.Error())
	case errors.Is(err, devices.ErrNotFound):
		writeAPIError(w, http.StatusNotFound, "not_found", "device not found")
	case errors.Is(err, devices.ErrConflict):
		writeAPIError(w, http.StatusConflict, "conflict", "device conflicts with an existing record")
	default:
		writeAPIError(w, http.StatusInternalServerError, "internal_error", "device operation failed")
	}
}

func toAPINetworkDevice(record devices.DeviceRecord) api.NetworkDevice {
	result := api.NetworkDevice{
		Id:          openapi_types.UUID(record.Device.ID.Bytes),
		MacAddress:  record.Device.MacAddress,
		DisplayName: record.Device.DisplayName,
		Vlan: api.VlanRef{
			Id:     record.VLAN.ID,
			Name:   record.VLAN.Name,
			VlanId: record.VLAN.VlanID,
		},
		CreatedAt: record.Device.CreatedAt.Time,
		UpdatedAt: record.Device.UpdatedAt.Time,
	}

	if record.Device.CreatedByUserID.Valid {
		value := openapi_types.UUID(record.Device.CreatedByUserID.Bytes)
		result.CreatedByUserId = &value
	}
	if record.Device.UpdatedByUserID.Valid {
		value := openapi_types.UUID(record.Device.UpdatedByUserID.Bytes)
		result.UpdatedByUserId = &value
	}

	return result
}

func handleClientAuthError(w http.ResponseWriter, err error) {
	var validationErr clientauth.ValidationError
	switch {
	case errors.As(err, &validationErr):
		writeAPIError(w, http.StatusBadRequest, "bad_request", validationErr.Error())
	case errors.Is(err, clientauth.ErrNotFound):
		writeAPIError(w, http.StatusNotFound, "not_found", "authorization request not found")
	case errors.Is(err, clientauth.ErrConflict):
		writeAPIError(w, http.StatusConflict, "conflict", "authorization request conflicts with current state")
	case errors.Is(err, clientauth.ErrAuthorizationPending):
		writeJSON(w, http.StatusPreconditionRequired, api.ClientAuthError{
			Error:            api.AuthorizationPending,
			ErrorDescription: "device code request is still pending approval",
		})
	case errors.Is(err, clientauth.ErrSlowDown):
		writeJSON(w, http.StatusBadRequest, api.ClientAuthError{
			Error:            api.SlowDown,
			ErrorDescription: "device code request is being polled too quickly",
		})
	case errors.Is(err, clientauth.ErrExpiredToken):
		writeJSON(w, http.StatusBadRequest, api.ClientAuthError{
			Error:            api.ExpiredToken,
			ErrorDescription: "device code request has expired",
		})
	case errors.Is(err, clientauth.ErrAccessDenied):
		writeJSON(w, http.StatusBadRequest, api.ClientAuthError{
			Error:            api.AccessDenied,
			ErrorDescription: "device code request was denied",
		})
	case errors.Is(err, clientauth.ErrInvalidRequest):
		writeJSON(w, http.StatusBadRequest, api.ClientAuthError{
			Error:            api.InvalidRequest,
			ErrorDescription: "device code request is invalid",
		})
	case errors.Is(err, clientauth.ErrUnauthorized):
		writeAPIError(w, http.StatusUnauthorized, "unauthorized", "refresh token is invalid")
	case errors.Is(err, clientauth.ErrTooManyRequests):
		writeAPIError(w, http.StatusTooManyRequests, "too_many_requests", "too many requests")
	default:
		writeAPIError(w, http.StatusInternalServerError, "internal_error", "client auth operation failed")
	}
}
