package httpserver

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

type response struct {
	Status  string `json:"status"`
	Service string `json:"service,omitempty"`
}

func NewHandler(logger *slog.Logger) http.Handler {
	if logger == nil {
		logger = slog.Default()
	}

	router := chi.NewRouter()
	router.Use(middleware.RequestID)
	router.Use(middleware.RealIP)
	router.Use(requestLogger(logger))
	router.Use(middleware.Recoverer)

	router.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, response{Status: "ok"})
	})

	router.Get("/readyz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, response{Status: "ok"})
	})

	router.Route("/api/v1", func(r chi.Router) {
		r.Get("/system/ping", func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, http.StatusOK, response{
				Status:  "ok",
				Service: "intern-api",
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
