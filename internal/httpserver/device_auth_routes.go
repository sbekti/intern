package httpserver

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/sbekti/intern/internal/api"
	"github.com/sbekti/intern/internal/auth"
	"github.com/sbekti/intern/internal/identity"
)

func registerDeviceAuthRoutes(
	r chi.Router,
	logger *slog.Logger,
	authorizer *auth.Authorizer,
	clientAuthService ClientAuthService,
	authSpamService AuthSpamService,
) {
	r.Post("/auth/device_codes", func(w http.ResponseWriter, r *http.Request) {
		if clientAuthService == nil {
			writeAPIError(w, http.StatusInternalServerError, "internal_error", "client auth service not configured")
			return
		}
		if err := enforceDeviceFlowRateLimit(r, authSpamService, deviceFlowRateLimitCreate, ""); err != nil {
			handleAuthSpamError(w, logger, r, "device_code_create", "", err)
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
		if err := enforceDeviceFlowRateLimit(r, authSpamService, deviceFlowRateLimitDecision, user.Username); err != nil {
			handleAuthSpamError(w, logger, r, "device_decision", user.Username, err)
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
		if err := enforceDeviceFlowRateLimit(r, authSpamService, deviceFlowRateLimitDecision, user.Username); err != nil {
			handleAuthSpamError(w, logger, r, "device_decision", user.Username, err)
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
		if err := enforceDeviceFlowRateLimit(r, authSpamService, deviceFlowRateLimitExchange, ""); err != nil {
			handleAuthSpamError(w, logger, r, "device_token_exchange", "", err)
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
		if err := enforceDeviceFlowRateLimit(r, authSpamService, deviceFlowRateLimitRefresh, ""); err != nil {
			handleAuthSpamError(w, logger, r, "refresh_token", "", err)
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
		if err := enforceDeviceFlowRateLimit(r, authSpamService, deviceFlowRateLimitLogout, ""); err != nil {
			handleAuthSpamError(w, logger, r, "logout", "", err)
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
}
