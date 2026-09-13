package httpserver

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/sbekti/intern/internal/api"
	"github.com/sbekti/intern/internal/auth"
	"github.com/sbekti/intern/internal/devices"
	"github.com/sbekti/intern/internal/gizmos"
	"github.com/sbekti/intern/internal/identity"
)

func registerGizmoRoutes(r chi.Router, authorizer *auth.Authorizer, service GizmoService) {
	r.With(authorizer.RequireAdmin()).Get("/gizmos", func(w http.ResponseWriter, r *http.Request) {
		if service == nil {
			writeAPIError(w, http.StatusInternalServerError, "internal_error", "gizmo service not configured")
			return
		}
		records, err := service.List(r.Context())
		if err != nil {
			writeAPIError(w, http.StatusInternalServerError, "internal_error", "failed to list gizmos")
			return
		}
		items := make([]api.Gizmo, 0, len(records))
		for _, record := range records {
			items = append(items, toAPIGizmo(record))
		}
		writeJSON(w, http.StatusOK, api.GizmoList{Items: items})
	})

	r.With(authorizer.RequireAdmin()).Post("/gizmos", func(w http.ResponseWriter, r *http.Request) {
		if service == nil {
			writeAPIError(w, http.StatusInternalServerError, "internal_error", "gizmo service not configured")
			return
		}
		actor, ok := identity.FromContext(r.Context())
		if !ok {
			writeAPIError(w, http.StatusInternalServerError, "internal_error", "current user missing")
			return
		}
		var body api.GizmoWrite
		if err := decodeJSON(w, r, &body); err != nil {
			writeDecodeJSONError(w, err)
			return
		}
		record, err := service.Create(r.Context(), actor, body)
		if err != nil {
			handleGizmoError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, toAPIGizmo(record))
	})

	r.With(authorizer.RequireAdmin()).Put("/gizmos/{network_device_id}", func(w http.ResponseWriter, r *http.Request) {
		if service == nil {
			writeAPIError(w, http.StatusInternalServerError, "internal_error", "gizmo service not configured")
			return
		}
		actor, ok := identity.FromContext(r.Context())
		if !ok {
			writeAPIError(w, http.StatusInternalServerError, "internal_error", "current user missing")
			return
		}
		id, err := decodeUUIDPathParam(r, "network_device_id")
		if err != nil {
			writeAPIError(w, http.StatusBadRequest, "bad_request", "invalid network device id")
			return
		}
		var body api.GizmoUpdate
		if err := decodeJSON(w, r, &body); err != nil {
			writeDecodeJSONError(w, err)
			return
		}
		record, err := service.Update(r.Context(), actor, id, body)
		if err != nil {
			handleGizmoError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, toAPIGizmo(record))
	})

	r.With(authorizer.RequireAdmin()).Delete("/gizmos/{network_device_id}", func(w http.ResponseWriter, r *http.Request) {
		if service == nil {
			writeAPIError(w, http.StatusInternalServerError, "internal_error", "gizmo service not configured")
			return
		}
		actor, ok := identity.FromContext(r.Context())
		if !ok {
			writeAPIError(w, http.StatusInternalServerError, "internal_error", "current user missing")
			return
		}
		id, err := decodeUUIDPathParam(r, "network_device_id")
		if err != nil {
			writeAPIError(w, http.StatusBadRequest, "bad_request", "invalid network device id")
			return
		}
		if err := service.Delete(r.Context(), actor, id); err != nil {
			handleGizmoError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
}

func registerKioskRoutes(r chi.Router, service GizmoService) {
	r.Get("/api/v1/kiosks/{mac_address}", func(w http.ResponseWriter, r *http.Request) {
		if service == nil {
			writeAPIError(w, http.StatusInternalServerError, "internal_error", "gizmo service not configured")
			return
		}
		url, err := service.ResolveKioskURL(r.Context(), chi.URLParam(r, "mac_address"))
		if err != nil {
			var validationErr gizmos.ValidationError
			switch {
			case errors.As(err, &validationErr):
				writeAPIError(w, http.StatusBadRequest, "bad_request", validationErr.Error())
			case errors.Is(err, gizmos.ErrNotFound):
				writeAPIError(w, http.StatusNotFound, "not_found", "kiosk assignment not found")
			default:
				writeAPIError(w, http.StatusInternalServerError, "internal_error", "kiosk lookup failed")
			}
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		writeJSON(w, http.StatusOK, api.KioskAssignment{Url: url})
	})
}

func handleGizmoError(w http.ResponseWriter, err error) {
	var validationErr gizmos.ValidationError
	switch {
	case errors.As(err, &validationErr):
		writeAPIError(w, http.StatusBadRequest, "bad_request", validationErr.Error())
	case errors.Is(err, gizmos.ErrNotFound):
		writeAPIError(w, http.StatusNotFound, "not_found", "gizmo not found")
	case errors.Is(err, gizmos.ErrConflict):
		writeAPIError(w, http.StatusConflict, "conflict", "gizmo conflicts with an existing record")
	default:
		writeAPIError(w, http.StatusInternalServerError, "internal_error", "gizmo operation failed")
	}
}

func toAPIGizmo(record gizmos.Record) api.Gizmo {
	return api.Gizmo{
		NetworkDevice: toAPINetworkDevice(devices.DeviceRecord{Device: record.Device, VLAN: record.VLAN}),
		KioskUrl:      record.Gizmo.KioskUrl,
		CreatedAt:     record.Gizmo.CreatedAt.Time,
		UpdatedAt:     record.Gizmo.UpdatedAt.Time,
	}
}
