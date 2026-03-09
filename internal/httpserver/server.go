package httpserver

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/sbekti/intern-api/internal/auth"
	"github.com/sbekti/intern-api/internal/config"
)

type response struct {
	Status  string `json:"status"`
	Service string `json:"service,omitempty"`
	User    string `json:"user,omitempty"`
}

func NewHandler(logger *slog.Logger, cfg config.Config) http.Handler {
	if logger == nil {
		logger = slog.Default()
	}

	authenticator := auth.NewAuthenticator(cfg)

	router := chi.NewRouter()
	router.Use(middleware.RequestID)
	router.Use(middleware.RealIP)
	router.Use(requestLogger(logger))
	router.Use(middleware.Recoverer)
	router.Use(authenticator.OptionalPrincipalMiddleware())

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
	})

	return router
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
