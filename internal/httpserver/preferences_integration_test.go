//go:build integration

package httpserver

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSharedPreferences(t *testing.T) {
	env := newHandlerIntegrationEnv(t)
	request := func(method, body, groups string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, "/api/v1/preferences", strings.NewReader(body))
		req.RemoteAddr = "127.0.0.1:43210"
		if groups != "" {
			setForwardAuthHeaders(req, "alice", "Alice", "alice@example.test", groups)
		}
		rec := httptest.NewRecorder()
		env.handler.ServeHTTP(rec, req)
		return rec
	}
	for _, method := range []string{"GET", "PATCH"} {
		if rec := request(method, "{}", ""); rec.Code != http.StatusUnauthorized {
			t.Fatalf("anonymous %s: %d", method, rec.Code)
		}
	}
	if rec := request("GET", "", "Users"); rec.Code != 200 || strings.TrimSpace(rec.Body.String()) != "{}" {
		t.Fatalf("default: %d %s", rec.Code, rec.Body)
	}
	if rec := request("PATCH", `{"dashboard":{}}`, "Users"); rec.Code != 403 {
		t.Fatalf("non-admin write: %d", rec.Code)
	}
	for _, body := range []string{`null`, `[]`, `{"dashboard":null}`, `{"dashboard":[]}`} {
		if rec := request("PATCH", body, "Super-Users"); rec.Code != 400 {
			t.Fatalf("invalid %s: %d", body, rec.Code)
		}
	}
	for _, body := range []string{`{"other":{"value":true}}`, `{"dashboard":{"section_order":["b","a"],"hidden_sections":["a"]}}`} {
		if rec := request("PATCH", body, "Super-Users"); rec.Code != 200 {
			t.Fatalf("save: %d %s", rec.Code, rec.Body)
		}
	}
	rec := request("GET", "", "Users")
	other := httptest.NewRequest(http.MethodGet, "/api/v1/preferences", nil)
	other.RemoteAddr = "127.0.0.1:43210"
	setForwardAuthHeaders(other, "bob", "Bob", "bob@example.test", "Users")
	shared := httptest.NewRecorder()
	env.handler.ServeHTTP(shared, other)
	if shared.Code != 200 || shared.Body.String() != rec.Body.String() {
		t.Fatalf("other user did not receive the shared layout: %d %s", shared.Code, shared.Body)
	}
	var prefs map[string]json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &prefs); err != nil {
		t.Fatal(err)
	}
	var dashboard struct {
		Order  []string `json:"section_order"`
		Hidden []string `json:"hidden_sections"`
	}
	if err := json.Unmarshal(prefs["dashboard"], &dashboard); err != nil {
		t.Fatal(err)
	}
	if rec.Code != 200 || len(prefs) != 2 || strings.Join(dashboard.Order, ",") != "b,a" || strings.Join(dashboard.Hidden, ",") != "a" {
		t.Fatalf("persistent merged preferences: %d %s", rec.Code, rec.Body)
	}
}
