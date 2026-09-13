package httpserver

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/sbekti/intern/internal/api"
	"github.com/sbekti/intern/internal/db"
	"github.com/sbekti/intern/internal/gizmos"
)

type fakeGizmoService struct {
	resolve func(context.Context, string) (string, error)
}

func (fakeGizmoService) List(context.Context) ([]gizmos.Record, error) { return nil, nil }
func (fakeGizmoService) Create(context.Context, db.User, api.GizmoWrite) (gizmos.Record, error) {
	return gizmos.Record{}, nil
}
func (fakeGizmoService) Update(context.Context, db.User, uuid.UUID, api.GizmoUpdate) (gizmos.Record, error) {
	return gizmos.Record{}, nil
}
func (fakeGizmoService) Delete(context.Context, db.User, uuid.UUID) error { return nil }
func (f fakeGizmoService) ResolveKioskURL(ctx context.Context, mac string) (string, error) {
	return f.resolve(ctx, mac)
}

func TestKioskLookupIsPublicAndMinimal(t *testing.T) {
	t.Parallel()

	handler := NewHandler(slog.New(slog.NewTextHandler(io.Discard, nil)), mustTestConfig(t), Dependencies{
		GizmoService: fakeGizmoService{resolve: func(_ context.Context, mac string) (string, error) {
			if mac != "B4-A9-FC-11-7E-4A" {
				t.Fatalf("MAC = %q", mac)
			}
			return "https://example.test/dashboard", nil
		}},
	})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/kiosks/B4-A9-FC-11-7E-4A", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK || rec.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("status/cache = %d/%q", rec.Code, rec.Header().Get("Cache-Control"))
	}
	var body map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if len(body) != 1 || body["url"] != "https://example.test/dashboard" {
		t.Fatalf("body = %#v", body)
	}
}

func TestKioskLookupErrorsDoNotLeakInventoryState(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string
		err  error
		want int
	}{
		{name: "missing disabled or unconfigured", err: gizmos.ErrNotFound, want: http.StatusNotFound},
		{name: "malformed MAC", err: gizmos.ValidationError{Message: "bad MAC"}, want: http.StatusBadRequest},
	} {
		t.Run(test.name, func(t *testing.T) {
			handler := NewHandler(slog.New(slog.NewTextHandler(io.Discard, nil)), mustTestConfig(t), Dependencies{
				GizmoService: fakeGizmoService{resolve: func(context.Context, string) (string, error) { return "", test.err }},
			})
			req := httptest.NewRequest(http.MethodGet, "/api/v1/kiosks/value", nil)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != test.want {
				t.Fatalf("status = %d, want %d", rec.Code, test.want)
			}
		})
	}
}

func TestGizmoAdministrationRequiresAuthentication(t *testing.T) {
	t.Parallel()

	handler := NewHandler(slog.New(slog.NewTextHandler(io.Discard, nil)), mustTestConfig(t), Dependencies{})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/gizmos", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}
