package auth

import (
	"context"
	"net/http"

	"github.com/sbekti/intern-api/internal/apierror"
)

type SessionValidator interface {
	ValidateSession(ctx context.Context, sessionID string) (bool, error)
}

type SessionValidatorFunc func(ctx context.Context, sessionID string) (bool, error)

func (f SessionValidatorFunc) ValidateSession(ctx context.Context, sessionID string) (bool, error) {
	return f(ctx, sessionID)
}

func RequireActiveBearerSession(validator SessionValidator) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			principal, ok := FromContext(r.Context())
			if !ok || principal.Source != PrincipalSourceBearerJWT {
				next.ServeHTTP(w, r)
				return
			}

			if principal.SessionID == "" {
				apierror.Write(w, http.StatusUnauthorized, "unauthorized", "authentication required")
				return
			}
			if validator == nil {
				apierror.Write(w, http.StatusServiceUnavailable, "service_unavailable", "session validation unavailable")
				return
			}

			active, err := validator.ValidateSession(r.Context(), principal.SessionID)
			if err != nil {
				apierror.Write(w, http.StatusServiceUnavailable, "service_unavailable", "session validation unavailable")
				return
			}
			if !active {
				apierror.Write(w, http.StatusUnauthorized, "unauthorized", "session is invalid")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
