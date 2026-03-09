package httpserver

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	openapi_types "github.com/oapi-codegen/runtime/types"
	"github.com/sbekti/intern-api/internal/api"
	"github.com/sbekti/intern-api/internal/auth"
	"github.com/sbekti/intern-api/internal/config"
	"github.com/sbekti/intern-api/internal/identity"
)

type response struct {
	Status  string `json:"status"`
	Service string `json:"service,omitempty"`
	User    string `json:"user,omitempty"`
}

type Dependencies struct {
	UserStore identity.UserStore
}

func NewHandler(logger *slog.Logger, cfg config.Config, deps Dependencies) http.Handler {
	if logger == nil {
		logger = slog.Default()
	}

	authenticator := auth.NewAuthenticator(cfg)
	authorizer := auth.NewAuthorizer(cfg)
	userSyncer := identity.NewSyncer(deps.UserStore)

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

		r.With(authorizer.RequireAuthenticated()).Get("/profile", func(w http.ResponseWriter, r *http.Request) {
			user, ok := identity.FromContext(r.Context())
			if !ok {
				principal, principalOK := auth.FromContext(r.Context())
				if !principalOK {
					http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
					return
				}

				writeJSON(w, http.StatusOK, api.Profile{
					Username: principal.Username,
					Name:     principal.Name,
					Email:    openapi_types.Email(principal.Email),
					Groups:   append([]string(nil), principal.Groups...),
					IsAdmin:  authorizer.IsAdmin(principal),
				})
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
	})

	return router
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
