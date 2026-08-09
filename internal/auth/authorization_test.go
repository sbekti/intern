package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sbekti/intern/internal/config"
)

func TestIsAdmin(t *testing.T) {
	t.Parallel()

	authorizer := NewAuthorizer(config.Config{
		Authorization: config.AuthorizationConfig{
			AdminGroups: []string{"Super-Users"},
		},
	})

	if !authorizer.IsAdmin(&Principal{Groups: []string{"Users", "Super-Users"}}) {
		t.Fatal("expected principal to be admin")
	}

	if authorizer.IsAdmin(&Principal{Groups: []string{"Users"}}) {
		t.Fatal("expected principal not to be admin")
	}
}

func TestIsAdminUsesConfiguredGroups(t *testing.T) {
	t.Parallel()

	authorizer := NewAuthorizer(config.Config{
		Authorization: config.AuthorizationConfig{
			AdminGroups: []string{"Network-Admins"},
		},
	})

	if !authorizer.IsAdmin(&Principal{Groups: []string{"Users", "Network-Admins"}}) {
		t.Fatal("expected configured admin group to grant admin access")
	}

	if authorizer.IsAdmin(&Principal{Groups: []string{"Users", "Super-Users"}}) {
		t.Fatal("expected default admin group not to match when overridden")
	}
}

func TestRequireAuthenticated(t *testing.T) {
	t.Parallel()

	authorizer := NewAuthorizer(config.Config{})
	nextCalled := false
	handler := authorizer.RequireAuthenticated()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, rec.Code)
	}
	if nextCalled {
		t.Fatal("expected next handler not to be called")
	}
}

func TestRequireAuthenticatedAllowsPrincipal(t *testing.T) {
	t.Parallel()

	authorizer := NewAuthorizer(config.Config{})
	nextCalled := false
	handler := authorizer.RequireAuthenticated()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(NewContext(req.Context(), &Principal{Username: "alice"}))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d", http.StatusNoContent, rec.Code)
	}
	if !nextCalled {
		t.Fatal("expected next handler to be called")
	}
}

func TestRequireAdmin(t *testing.T) {
	t.Parallel()

	authorizer := NewAuthorizer(config.Config{
		Authorization: config.AuthorizationConfig{
			AdminGroups: []string{"Super-Users"},
		},
	})

	t.Run("missing principal", func(t *testing.T) {
		t.Parallel()

		nextCalled := false
		handler := authorizer.RequireAdmin()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			nextCalled = true
			w.WriteHeader(http.StatusNoContent)
		}))

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, rec.Code)
		}
		if nextCalled {
			t.Fatal("expected next handler not to be called")
		}
	})

	t.Run("non admin", func(t *testing.T) {
		t.Parallel()

		nextCalled := false
		handler := authorizer.RequireAdmin()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			nextCalled = true
			w.WriteHeader(http.StatusNoContent)
		}))

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req = req.WithContext(NewContext(req.Context(), &Principal{
			Username: "alice",
			Groups:   []string{"Users"},
		}))
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected status %d, got %d", http.StatusForbidden, rec.Code)
		}
		if nextCalled {
			t.Fatal("expected next handler not to be called")
		}
	})

	t.Run("admin", func(t *testing.T) {
		t.Parallel()

		nextCalled := false
		handler := authorizer.RequireAdmin()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			nextCalled = true
			w.WriteHeader(http.StatusNoContent)
		}))

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req = req.WithContext(NewContext(req.Context(), &Principal{
			Username: "alice",
			Groups:   []string{"Users", "Super-Users"},
		}))
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusNoContent {
			t.Fatalf("expected status %d, got %d", http.StatusNoContent, rec.Code)
		}
		if !nextCalled {
			t.Fatal("expected next handler to be called")
		}
	})
}
