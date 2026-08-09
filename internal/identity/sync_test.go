package identity

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/sbekti/intern/internal/auth"
	"github.com/sbekti/intern/internal/db"
)

type fakeUserStore struct {
	upsertFn func(ctx context.Context, arg db.UpsertUserByUsernameParams) (db.User, error)
}

func (f fakeUserStore) UpsertUserByUsername(ctx context.Context, arg db.UpsertUserByUsernameParams) (db.User, error) {
	return f.upsertFn(ctx, arg)
}

type lookupUserStore struct {
	fakeUserStore
	lookupFn func(ctx context.Context, arg db.GetUserByUsernameParams) (db.User, error)
}

func (f lookupUserStore) GetUserByUsername(ctx context.Context, arg db.GetUserByUsernameParams) (db.User, error) {
	return f.lookupFn(ctx, arg)
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

	t.Run("syncs authenticated user for a mutation", func(t *testing.T) {
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

		req := httptest.NewRequest(http.MethodPost, "/api/v1/networks/vlans", nil)
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

	t.Run("safe methods look up without upserting", func(t *testing.T) {
		principal := &auth.Principal{
			Username: "alice",
			Name:     "Alice Example",
			Email:    "alice@example.com",
			Groups:   []string{"Users"},
		}
		var upsertCalls, lookupCalls int
		syncer := NewSyncer(lookupUserStore{
			fakeUserStore: fakeUserStore{
				upsertFn: func(context.Context, db.UpsertUserByUsernameParams) (db.User, error) {
					upsertCalls++
					return db.User{}, nil
				},
			},
			lookupFn: func(_ context.Context, arg db.GetUserByUsernameParams) (db.User, error) {
				lookupCalls++
				if arg.Username != principal.Username {
					t.Fatalf("lookup username = %q, want %q", arg.Username, principal.Username)
				}
				return db.User{Username: principal.Username}, nil
			},
		})
		handler := syncer.Middleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if _, ok := FromContext(r.Context()); !ok {
				t.Error("expected user in context")
			}
			w.WriteHeader(http.StatusNoContent)
		}))

		for _, method := range []string{http.MethodGet, http.MethodHead, http.MethodOptions} {
			req := httptest.NewRequest(method, "/api/v1/networks/devices", nil)
			req = req.WithContext(auth.NewContext(req.Context(), principal))
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != http.StatusNoContent {
				t.Fatalf("%s status = %d, want %d", method, rec.Code, http.StatusNoContent)
			}
		}
		if upsertCalls != 0 {
			t.Fatalf("upsert calls = %d, want 0", upsertCalls)
		}
		if lookupCalls != 3 {
			t.Fatalf("lookup calls = %d, want 3", lookupCalls)
		}
	})

	t.Run("safe lookup falls back to principal when user is missing", func(t *testing.T) {
		upsertCalls := 0
		syncer := NewSyncer(lookupUserStore{
			fakeUserStore: fakeUserStore{upsertFn: func(context.Context, db.UpsertUserByUsernameParams) (db.User, error) {
				upsertCalls++
				return db.User{}, nil
			}},
			lookupFn: func(context.Context, db.GetUserByUsernameParams) (db.User, error) {
				return db.User{}, pgx.ErrNoRows
			},
		})
		handler := syncer.Middleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, ok := FromContext(r.Context())
			if !ok || user.Username != "alice" {
				t.Fatalf("user = %#v, ok = %v, want principal fallback", user, ok)
			}
			w.WriteHeader(http.StatusNoContent)
		}))
		req := httptest.NewRequest(http.MethodGet, "/api/v1/profile/sessions", nil)
		req = req.WithContext(auth.NewContext(req.Context(), &auth.Principal{Username: "alice"}))
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusNoContent {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusNoContent)
		}
		if upsertCalls != 0 {
			t.Fatalf("upsert calls = %d, want 0", upsertCalls)
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

		req := httptest.NewRequest(http.MethodPost, "/api/v1/networks/vlans", nil)
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
