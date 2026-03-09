package identity

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/sbekti/intern-api/internal/auth"
	"github.com/sbekti/intern-api/internal/db"
)

type fakeUserStore struct {
	upsertFn func(ctx context.Context, arg db.UpsertUserByUsernameParams) (db.User, error)
}

func (f fakeUserStore) UpsertUserByUsername(ctx context.Context, arg db.UpsertUserByUsernameParams) (db.User, error) {
	return f.upsertFn(ctx, arg)
}

func TestSyncerMiddleware(t *testing.T) {
	t.Parallel()

	t.Run("skips anonymous request", func(t *testing.T) {
		t.Parallel()

		called := false
		syncer := NewSyncer(fakeUserStore{
			upsertFn: func(ctx context.Context, arg db.UpsertUserByUsernameParams) (db.User, error) {
				called = true
				return db.User{}, nil
			},
		})

		nextCalled := false
		handler := syncer.Middleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			nextCalled = true
			w.WriteHeader(http.StatusNoContent)
		}))

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusNoContent {
			t.Fatalf("expected status %d, got %d", http.StatusNoContent, rec.Code)
		}
		if called {
			t.Fatal("expected store not to be called")
		}
		if !nextCalled {
			t.Fatal("expected next handler to be called")
		}
	})

	t.Run("syncs authenticated user", func(t *testing.T) {
		t.Parallel()

		called := false
		syncer := NewSyncer(fakeUserStore{
			upsertFn: func(ctx context.Context, arg db.UpsertUserByUsernameParams) (db.User, error) {
				called = true
				if arg.Username != "alice" {
					t.Fatalf("expected username alice, got %q", arg.Username)
				}
				if arg.Email != "alice@example.com" {
					t.Fatalf("expected email alice@example.com, got %q", arg.Email)
				}
				if len(arg.Groups) != 2 {
					t.Fatalf("expected 2 groups, got %d", len(arg.Groups))
				}
				return db.User{
					ID:       pgtype.UUID{Valid: true},
					Username: arg.Username,
					Name:     arg.Name,
					Email:    arg.Email,
					Groups:   arg.Groups,
				}, nil
			},
		})

		nextCalled := false
		handler := syncer.Middleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			nextCalled = true
			user, ok := FromContext(r.Context())
			if !ok {
				t.Fatal("expected stored user in context")
			}
			if user.Username != "alice" {
				t.Fatalf("expected username alice, got %q", user.Username)
			}
			w.WriteHeader(http.StatusNoContent)
		}))

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req = req.WithContext(auth.NewContext(req.Context(), &auth.Principal{
			Username: "alice",
			Name:     "Alice Example",
			Email:    "alice@example.com",
			Groups:   []string{"Users", "Super-Users"},
		}))
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusNoContent {
			t.Fatalf("expected status %d, got %d", http.StatusNoContent, rec.Code)
		}
		if !called {
			t.Fatal("expected store to be called")
		}
		if !nextCalled {
			t.Fatal("expected next handler to be called")
		}
	})

	t.Run("returns internal server error when sync fails", func(t *testing.T) {
		t.Parallel()

		syncer := NewSyncer(fakeUserStore{
			upsertFn: func(ctx context.Context, arg db.UpsertUserByUsernameParams) (db.User, error) {
				return db.User{}, errors.New("boom")
			},
		})

		nextCalled := false
		handler := syncer.Middleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			nextCalled = true
			w.WriteHeader(http.StatusNoContent)
		}))

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req = req.WithContext(auth.NewContext(req.Context(), &auth.Principal{
			Username: "alice",
		}))
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, rec.Code)
		}
		if nextCalled {
			t.Fatal("expected next handler not to be called")
		}
	})
}
