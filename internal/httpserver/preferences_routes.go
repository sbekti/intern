package httpserver

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/sbekti/intern/internal/auth"
	"github.com/sbekti/intern/internal/db"
)

func registerPreferencesRoutes(r chi.Router, authorizer *auth.Authorizer, store PreferencesStore) {
	r.With(authorizer.RequireAuthenticated()).Get("/api/v1/preferences", func(w http.ResponseWriter, r *http.Request) {
		if store == nil {
			writeAPIError(w, http.StatusServiceUnavailable, "unavailable", "preferences unavailable")
			return
		}
		data, err := store.GetApplicationPreferences(r.Context())
		if errors.Is(err, pgx.ErrNoRows) {
			data, err = []byte("{}"), nil
		}
		if err != nil {
			writeAPIError(w, http.StatusInternalServerError, "internal_error", "failed to load preferences")
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		writeJSON(w, http.StatusOK, json.RawMessage(data))
	})
	r.With(authorizer.RequireAdmin()).Patch("/api/v1/preferences", func(w http.ResponseWriter, r *http.Request) {
		var patch map[string]map[string]json.RawMessage
		if err := decodeJSON(w, r, &patch); err != nil {
			writeDecodeJSONError(w, err)
			return
		}
		if patch == nil {
			writeAPIError(w, http.StatusBadRequest, "bad_request", "preferences must be an object")
			return
		}
		for key, value := range patch {
			if key == "" || value == nil {
				writeAPIError(w, http.StatusBadRequest, "bad_request", "preference namespaces must be objects")
				return
			}
		}
		if store == nil {
			writeAPIError(w, http.StatusServiceUnavailable, "unavailable", "preferences unavailable")
			return
		}
		data, _ := json.Marshal(patch)
		data, err := store.MergeApplicationPreferences(r.Context(), db.MergeApplicationPreferencesParams{Patch: data})
		if err != nil {
			writeAPIError(w, http.StatusInternalServerError, "internal_error", "failed to save preferences")
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		writeJSON(w, http.StatusOK, json.RawMessage(data))
	})
}
