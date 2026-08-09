package identity

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/sbekti/intern/internal/apierror"
	"github.com/sbekti/intern/internal/auth"
	"github.com/sbekti/intern/internal/db"
)

type UserStore interface {
	UpsertUserByUsername(ctx context.Context, arg db.UpsertUserByUsernameParams) (db.User, error)
}

type LookupUserStore interface {
	GetUserByUsername(ctx context.Context, arg db.GetUserByUsernameParams) (db.User, error)
}

type Syncer struct {
	users UserStore
}

type contextKey string

const userContextKey contextKey = "user"

func NewSyncer(users UserStore) *Syncer {
	return &Syncer{users: users}
}

func (s *Syncer) Middleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			principal, ok := auth.FromContext(r.Context())
			if !ok || s == nil || s.users == nil {
				next.ServeHTTP(w, r)
				return
			}

			var (
				user db.User
				err  error
			)
			if isReadOnlyMethod(r.Method) {
				if lookup, ok := s.users.(LookupUserStore); ok {
					user, err = lookup.GetUserByUsername(r.Context(), db.GetUserByUsernameParams{Username: strings.TrimSpace(principal.Username)})
					if errors.Is(err, pgx.ErrNoRows) {
						user, err = userFromPrincipal(principal), nil
					}
				} else {
					user = userFromPrincipal(principal)
				}
			} else {
				user, err = s.users.UpsertUserByUsername(r.Context(), db.UpsertUserByUsernameParams{
					Username: strings.TrimSpace(principal.Username),
					Name:     strings.TrimSpace(principal.Name),
					Email:    strings.TrimSpace(principal.Email),
					Groups:   append([]string(nil), principal.Groups...),
				})
			}
			if err != nil {
				apierror.Write(w, http.StatusInternalServerError, "internal_error", "failed to load authenticated user")
				return
			}

			next.ServeHTTP(w, r.WithContext(NewContext(r.Context(), user)))
		})
	}
}

func isReadOnlyMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return true
	default:
		return false
	}
}

func userFromPrincipal(principal *auth.Principal) db.User {
	return db.User{
		Username: strings.TrimSpace(principal.Username),
		Name:     strings.TrimSpace(principal.Name),
		Email:    strings.TrimSpace(principal.Email),
		Groups:   append([]string(nil), principal.Groups...),
	}
}

func NewContext(ctx context.Context, user db.User) context.Context {
	return context.WithValue(ctx, userContextKey, user)
}

func FromContext(ctx context.Context) (db.User, bool) {
	user, ok := ctx.Value(userContextKey).(db.User)
	if !ok {
		return db.User{}, false
	}
	return user, true
}
