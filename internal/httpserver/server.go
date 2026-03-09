package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"
	"github.com/sbekti/intern-api/internal/api"
	"github.com/sbekti/intern-api/internal/auth"
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
	UserStore      identity.UserStore
	DashboardStore dashboard.Querier
	WeatherService dashboard.WeatherService
	VLANService    VLANService
	DeviceService  DeviceService
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

	router := chi.NewRouter()
	router.Use(middleware.RequestID)
	router.Use(middleware.RealIP)
	router.Use(requestLogger(logger))
	router.Use(middleware.Recoverer)
	router.Use(authenticator.OptionalPrincipalMiddleware())
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

		r.With(authorizer.RequireAuthenticated()).Get("/networks/vlans", func(w http.ResponseWriter, r *http.Request) {
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

		r.With(authorizer.RequireAuthenticated()).Get("/networks/vlans/{id}", func(w http.ResponseWriter, r *http.Request) {
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
