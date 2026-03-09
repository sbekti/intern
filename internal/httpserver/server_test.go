package httpserver

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBootstrapRoutes(t *testing.T) {
	t.Parallel()

	handler := NewHandler(slog.New(slog.NewTextHandler(io.Discard, nil)))

	tests := []struct {
		path string
	}{
		{path: "/healthz"},
		{path: "/readyz"},
		{path: "/api/v1/system/ping"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.path, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
			}
			if got := rec.Header().Get("Content-Type"); got != "application/json" {
				t.Fatalf("expected application/json content type, got %q", got)
			}
		})
	}
}
