package httpserver

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	openapi_types "github.com/oapi-codegen/runtime/types"
	"github.com/sbekti/intern/internal/api"
	"github.com/sbekti/intern/internal/auth"
	"github.com/sbekti/intern/internal/identity"
)

func registerSystemProfileRoutes(
	r chi.Router,
	authorizer *auth.Authorizer,
	sessionService SessionService,
) {
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

	r.With(authorizer.RequireAuthenticated()).Get("/profile", func(w http.ResponseWriter, r *http.Request) {
		user, ok := identity.FromContext(r.Context())
		if !ok {
			writeAPIError(w, http.StatusInternalServerError, "internal_error", "current user missing")
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
}
