package httpserver

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/sbekti/intern/internal/api"
	"github.com/sbekti/intern/internal/auditlogs"
	"github.com/sbekti/intern/internal/auth"
)

func registerAdminRoutes(
	r chi.Router,
	authorizer *auth.Authorizer,
	sessionService SessionService,
	auditLogService AuditLogService,
) {
	r.With(authorizer.RequireAdmin()).Get("/admin/auth/sessions", func(w http.ResponseWriter, r *http.Request) {
		if sessionService == nil {
			writeAPIError(w, http.StatusInternalServerError, "internal_error", "session service not configured")
			return
		}

		params, err := decodeAuthSessionPageParams(r)
		if err != nil {
			writeAPIError(w, http.StatusBadRequest, "bad_request", err.Error())
			return
		}

		principal, _ := auth.FromContext(r.Context())
		page, err := sessionService.ListAdminSessionsPage(
			r.Context(),
			currentSessionID(principal),
			int32Value(params.Limit, defaultAuthSessionLimit),
			int32Value(params.Offset, 0),
		)
		if err != nil {
			writeAPIError(w, http.StatusInternalServerError, "internal_error", "failed to list admin sessions")
			return
		}

		writeJSON(w, http.StatusOK, page)
	})

	r.With(authorizer.RequireAdmin()).Post("/admin/auth/sessions/revoke_all", func(w http.ResponseWriter, r *http.Request) {
		if sessionService == nil {
			writeAPIError(w, http.StatusInternalServerError, "internal_error", "session service not configured")
			return
		}

		if err := sessionService.RevokeAllAdminSessions(r.Context()); err != nil {
			writeAPIError(w, http.StatusInternalServerError, "internal_error", "failed to revoke admin sessions")
			return
		}

		w.WriteHeader(http.StatusNoContent)
	})

	r.With(authorizer.RequireAdmin()).Post("/admin/auth/sessions/{id}/revoke", func(w http.ResponseWriter, r *http.Request) {
		if sessionService == nil {
			writeAPIError(w, http.StatusInternalServerError, "internal_error", "session service not configured")
			return
		}

		id, err := decodeUUIDPathParam(r, "id")
		if err != nil {
			writeAPIError(w, http.StatusBadRequest, "bad_request", "invalid session id")
			return
		}

		if err := sessionService.RevokeAdminSession(r.Context(), id); err != nil {
			writeAPIError(w, http.StatusInternalServerError, "internal_error", "failed to revoke admin session")
			return
		}

		w.WriteHeader(http.StatusNoContent)
	})

	r.With(authorizer.RequireAdmin()).Get("/admin/audit_logs", func(w http.ResponseWriter, r *http.Request) {
		if auditLogService == nil {
			writeAPIError(w, http.StatusInternalServerError, "internal_error", "audit log service not configured")
			return
		}

		params, err := decodeAdminAuditLogParams(r)
		if err != nil {
			writeAPIError(w, http.StatusBadRequest, "bad_request", err.Error())
			return
		}

		page, err := auditLogService.List(r.Context(), auditlogs.Filter{
			Action:        trimmedString(params.Action),
			ResourceType:  trimmedString(params.ResourceType),
			ResourceID:    trimmedString(params.ResourceId),
			ActorUsername: trimmedString(params.ActorUsername),
			Limit:         int32Value(params.Limit, auditlogs.DefaultLimit),
			Offset:        int32Value(params.Offset, 0),
		})
		if err != nil {
			writeAPIError(w, http.StatusInternalServerError, "internal_error", "failed to list audit logs")
			return
		}

		writeJSON(w, http.StatusOK, api.AuditLogList{
			Items: page.Items,
			Pagination: api.AuditLogPagination{
				Limit:  page.Limit,
				Offset: page.Offset,
				Total:  page.TotalCount,
			},
		})
	})
}
