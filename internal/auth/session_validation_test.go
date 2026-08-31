package auth

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

type SessionValidatorFunc func(ctx context.Context, sessionID string) (bool, error)

func (f SessionValidatorFunc) ValidateSession(ctx context.Context, sessionID string) (bool, error) {
	return f(ctx, sessionID)
}

func TestRequireActiveBearerSessionPassesWithoutPrincipal(t *testing.T) {
	t.Parallel()

	nextCalled := false
	handler := RequireActiveBearerSession(nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d", http.StatusNoContent, rec.Code)
	}
	if !nextCalled {
		t.Fatal("expected next handler to be called")
	}
}

func TestRequireActiveBearerSessionPassesForwardAuthPrincipal(t *testing.T) {
	t.Parallel()

	nextCalled := false
	handler := RequireActiveBearerSession(nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(NewContext(req.Context(), &Principal{
		Username: "alice",
		Source:   PrincipalSourceForwardAuth,
	}))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d", http.StatusNoContent, rec.Code)
	}
	if !nextCalled {
		t.Fatal("expected next handler to be called")
	}
}

func TestRequireActiveBearerSessionAllowsActiveBearerSession(t *testing.T) {
	t.Parallel()

	nextCalled := false
	handler := RequireActiveBearerSession(SessionValidatorFunc(func(ctx context.Context, sessionID string) (bool, error) {
		if sessionID != "session-1" {
			t.Fatalf("expected session-1, got %q", sessionID)
		}
		return true, nil
	}))(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(NewContext(req.Context(), &Principal{
		Username:  "alice",
		SessionID: "session-1",
		Source:    PrincipalSourceBearerJWT,
	}))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d", http.StatusNoContent, rec.Code)
	}
	if !nextCalled {
		t.Fatal("expected next handler to be called")
	}
}

func TestRequireActiveBearerSessionRejectsMissingSessionID(t *testing.T) {
	t.Parallel()

	handler := RequireActiveBearerSession(SessionValidatorFunc(func(ctx context.Context, sessionID string) (bool, error) {
		t.Fatal("did not expect validator to be called")
		return false, nil
	}))(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(NewContext(req.Context(), &Principal{
		Username: "alice",
		Source:   PrincipalSourceBearerJWT,
	}))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, rec.Code)
	}
}

func TestRequireActiveBearerSessionRejectsInactiveSession(t *testing.T) {
	t.Parallel()

	handler := RequireActiveBearerSession(SessionValidatorFunc(func(ctx context.Context, sessionID string) (bool, error) {
		return false, nil
	}))(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(NewContext(req.Context(), &Principal{
		Username:  "alice",
		Source:    PrincipalSourceBearerJWT,
		SessionID: "session-1",
	}))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, rec.Code)
	}
}

func TestRequireActiveBearerSessionRejectsWhenValidatorUnavailable(t *testing.T) {
	t.Parallel()

	handler := RequireActiveBearerSession(nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(NewContext(req.Context(), &Principal{
		Username:  "alice",
		Source:    PrincipalSourceBearerJWT,
		SessionID: "session-1",
	}))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status %d, got %d", http.StatusServiceUnavailable, rec.Code)
	}
}

func TestRequireActiveBearerSessionRejectsWhenValidationErrors(t *testing.T) {
	t.Parallel()

	handler := RequireActiveBearerSession(SessionValidatorFunc(func(ctx context.Context, sessionID string) (bool, error) {
		return false, errors.New("db unavailable")
	}))(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(NewContext(req.Context(), &Principal{
		Username:  "alice",
		Source:    PrincipalSourceBearerJWT,
		SessionID: "session-1",
	}))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status %d, got %d", http.StatusServiceUnavailable, rec.Code)
	}
}
