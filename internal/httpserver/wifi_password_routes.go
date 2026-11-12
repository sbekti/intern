package httpserver

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/sbekti/intern/internal/api"
	"github.com/sbekti/intern/internal/auth"
	"github.com/sbekti/intern/internal/devices"
	"github.com/sbekti/intern/internal/identity"
)

func registerWiFiPasswordRoutes(r chi.Router, authorizer *auth.Authorizer, service DeviceService) {
	noStore := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Cache-Control", "no-store")
			next.ServeHTTP(w, r)
		})
	}
	r.With(noStore, authorizer.RequireAdmin()).Post("/networks/devices/wifi-password/generate", func(w http.ResponseWriter, r *http.Request) {
		password, err := devices.GenerateWiFiPassword()
		if err != nil {
			writeAPIError(w, http.StatusInternalServerError, "internal_error", "could not generate Wi-Fi password")
			return
		}
		writeJSON(w, http.StatusOK, api.WiFiPassword{Password: password})
	})
	r.With(noStore, authorizer.RequireAdmin()).Post("/networks/devices/{id}/wifi-password/reveal", func(w http.ResponseWriter, r *http.Request) {
		if service == nil {
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
		password, err := service.RevealWiFiPassword(r.Context(), actor, id)
		if err != nil {
			handleDeviceError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, api.WiFiPassword{Password: password})
	})
}
