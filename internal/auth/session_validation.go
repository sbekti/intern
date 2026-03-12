package auth

import (
	"context"
	"net/http"
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
				http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
				return
			}
			if validator == nil {
				http.Error(w, http.StatusText(http.StatusServiceUnavailable), http.StatusServiceUnavailable)
				return
			}

			active, err := validator.ValidateSession(r.Context(), principal.SessionID)
			if err != nil {
				http.Error(w, http.StatusText(http.StatusServiceUnavailable), http.StatusServiceUnavailable)
				return
			}
			if !active {
				http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
