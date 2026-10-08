package httpserver

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/sbekti/intern/internal/api"
	"github.com/sbekti/intern/internal/auditlogs"
	"github.com/sbekti/intern/internal/auth"
)

const maxJSONBodyBytes int64 = 1 << 20

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

func decodeJSON(w http.ResponseWriter, r *http.Request, dest any) error {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxJSONBodyBytes))
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(dest); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		if err != nil {
			return err
		}
		return errors.New("unexpected trailing json")
	}
	return nil
}

func writeDecodeJSONError(w http.ResponseWriter, err error) {
	var maxBytesError *http.MaxBytesError
	if errors.As(err, &maxBytesError) {
		writeAPIError(w, http.StatusRequestEntityTooLarge, "request_too_large", "request body too large")
		return
	}
	writeAPIError(w, http.StatusBadRequest, "bad_request", "invalid request body")
}

func decodeInt64PathParam(r *http.Request, key string) (int64, error) {
	raw := chi.URLParam(r, key)
	return strconv.ParseInt(raw, 10, 64)
}

func decodeInt32PathParam(r *http.Request, key string) (int32, error) {
	value, err := decodeInt64PathParam(r, key)
	if err != nil {
		return 0, err
	}
	if value < 1 || value > 4094 {
		return 0, strconv.ErrRange
	}
	return int32(value), nil
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
	sortBy := api.ListAdminAuditLogsParamsSortBy(query.Get("sort_by"))
	if sortBy == "" {
		sortBy = api.CreatedAt
	}
	if !sortBy.Valid() {
		return params, errors.New("invalid sort_by")
	}
	params.SortBy = &sortBy
	sortDir := api.ListAdminAuditLogsParamsSortDir(query.Get("sort_dir"))
	if sortDir == "" {
		sortDir = api.Desc
	}
	if !sortDir.Valid() {
		return params, errors.New("invalid sort_dir")
	}
	params.SortDir = &sortDir
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
