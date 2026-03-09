package auth

import (
	"net/http"
	"strings"

	"github.com/sbekti/intern-api/internal/config"
)

type Authorizer struct {
	adminGroups []string
}

func NewAuthorizer(cfg config.Config) *Authorizer {
	groups := make([]string, 0, len(cfg.Authorization.AdminGroups))
	for _, group := range cfg.Authorization.AdminGroups {
		trimmed := strings.TrimSpace(group)
		if trimmed != "" {
			groups = append(groups, trimmed)
		}
	}
	return &Authorizer{adminGroups: groups}
}

func (a *Authorizer) IsAdmin(principal *Principal) bool {
	if principal == nil {
		return false
	}

	for _, group := range principal.Groups {
		trimmed := strings.TrimSpace(group)
		for _, adminGroup := range a.adminGroups {
			if trimmed == adminGroup {
				return true
			}
		}
	}

	return false
}

func (a *Authorizer) RequireAuthenticated() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if _, ok := FromContext(r.Context()); !ok {
				http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func (a *Authorizer) RequireAdmin() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			principal, ok := FromContext(r.Context())
			if !ok {
				http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
				return
			}
			if !a.IsAdmin(principal) {
				http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
