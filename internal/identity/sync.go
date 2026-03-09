package identity

import (
	"context"
	"net/http"
	"strings"

	"github.com/sbekti/intern-api/internal/auth"
	"github.com/sbekti/intern-api/internal/db"
)

type UserStore interface {
	UpsertUserByUsername(ctx context.Context, arg db.UpsertUserByUsernameParams) (db.User, error)
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

			user, err := s.users.UpsertUserByUsername(r.Context(), db.UpsertUserByUsernameParams{
				Username: strings.TrimSpace(principal.Username),
				Name:     strings.TrimSpace(principal.Name),
				Email:    strings.TrimSpace(principal.Email),
				Groups:   append([]string(nil), principal.Groups...),
			})
			if err != nil {
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}

			next.ServeHTTP(w, r.WithContext(NewContext(r.Context(), user)))
		})
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
