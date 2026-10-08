package httpserver

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDecodeJSON(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string
		body string
		ok   bool
	}{
		{name: "one document", body: "{\"name\":\"test\"} \n", ok: true},
		{name: "unknown field", body: `{"other":"test"}`},
		{name: "second document", body: `{"name":"test"} {}`},
		{name: "malformed trailing data", body: `{"name":"test"} x`},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(test.body))
			rec := httptest.NewRecorder()
			var dest struct {
				Name string `json:"name"`
			}

			err := decodeJSON(rec, req, &dest)
			if (err == nil) != test.ok {
				t.Fatalf("decodeJSON() error = %v, want success %t", err, test.ok)
			}
			if err != nil {
				writeDecodeJSONError(rec, err)
				if rec.Code != http.StatusBadRequest {
					t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
				}
			}
		})
	}
}

func TestDecodeJSONRejectsOversizedBodies(t *testing.T) {
	t.Parallel()

	body := `{"name":"` + strings.Repeat("a", int(maxJSONBodyBytes)) + `"}`
	for _, contentLength := range []int64{int64(len(body)), -1} {
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
		req.ContentLength = contentLength
		rec := httptest.NewRecorder()
		var dest struct {
			Name string `json:"name"`
		}

		err := decodeJSON(rec, req, &dest)
		if err == nil {
			t.Fatal("decodeJSON() accepted an oversized body")
		}
		writeDecodeJSONError(rec, err)
		if rec.Code != http.StatusRequestEntityTooLarge {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusRequestEntityTooLarge)
		}
		if !strings.Contains(rec.Body.String(), `"code":"request_too_large"`) {
			t.Fatalf("unexpected error body: %s", rec.Body.String())
		}
	}
}

func TestDecodeAuditLogSort(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		query, by, dir string
		invalid        bool
	}{
		{query: "", by: "created_at", dir: "desc"},
		{query: "sort_by=actor_username&sort_dir=asc", by: "actor_username", dir: "asc"},
		{query: "sort_by=action&sort_dir=desc", by: "action", dir: "desc"},
		{query: "sort_by=resource", by: "resource", dir: "desc"},
		{query: "sort_by=metadata", invalid: true},
		{query: "sort_by=created_at%3BDELETE", invalid: true},
		{query: "sort_dir=ASC", invalid: true},
		{query: "sort_dir=up", invalid: true},
	} {
		t.Run(tc.query, func(t *testing.T) {
			params, err := decodeAdminAuditLogParams(httptest.NewRequest(http.MethodGet, "/?"+tc.query, nil))
			if tc.invalid {
				if err == nil {
					t.Fatal("accepted invalid sort")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if string(*params.SortBy) != tc.by || string(*params.SortDir) != tc.dir {
				t.Fatalf("sort = %s %s, want %s %s", *params.SortBy, *params.SortDir, tc.by, tc.dir)
			}
		})
	}
}
