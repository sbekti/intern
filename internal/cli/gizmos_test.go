package cli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sbekti/intern/internal/api"
)

const gizmoResponse = `{"network_device":{"id":"00000000-0000-0000-0000-000000000123","display_name":"Kitchen Gizmo","mac_address":"aa:bb:cc:dd:ee:ff","disabled":false,"vlan":{"name":"iot","vlan_id":20},"created_at":"2026-09-13T00:00:00Z","updated_at":"2026-09-13T00:00:00Z"},"kiosk_url":null,"created_at":"2026-09-13T00:00:00Z","updated_at":"2026-09-13T00:00:00Z"}`

func TestGizmoCreateAndUpdateFlags(t *testing.T) {
	t.Parallel()

	configDir := t.TempDir()
	requests := make([]map[string]any, 0, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		requests = append(requests, body)
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost {
			w.WriteHeader(http.StatusCreated)
		}
		_, _ = w.Write([]byte(gizmoResponse))
	}))
	defer server.Close()
	writeLoggedInProfile(t, configDir, server.URL)

	create := NewRootCommand()
	create.SetOut(new(bytes.Buffer))
	create.SetErr(new(bytes.Buffer))
	create.SetArgs([]string{"gizmo", "create", "--config-dir", configDir, "--device-id", "00000000-0000-0000-0000-000000000123", "--kiosk-url", "http://example.test"})
	if err := create.Execute(); err != nil {
		t.Fatal(err)
	}

	update := NewRootCommand()
	update.SetOut(new(bytes.Buffer))
	update.SetErr(new(bytes.Buffer))
	update.SetArgs([]string{"gizmo", "update", "00000000-0000-0000-0000-000000000123", "--config-dir", configDir, "--kiosk-url", ""})
	if err := update.Execute(); err != nil {
		t.Fatal(err)
	}

	if len(requests) != 2 || requests[0]["kiosk_url"] != "http://example.test" || requests[1]["kiosk_url"] != "" {
		t.Fatalf("requests = %#v", requests)
	}
	if requests[0]["network_device_id"] != "00000000-0000-0000-0000-000000000123" {
		t.Fatalf("create request = %#v", requests[0])
	}
}

func TestGizmoListJSONIncludesKioskURL(t *testing.T) {
	t.Parallel()

	configDir := t.TempDir()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[` + gizmoResponse + `]}`))
	}))
	defer server.Close()
	writeLoggedInProfile(t, configDir, server.URL)

	cmd := NewRootCommand()
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"gizmo", "list", "--config-dir", configDir, "--output", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var items []api.Gizmo
	if err := json.Unmarshal(output.Bytes(), &items); err != nil || len(items) != 1 {
		t.Fatalf("output = %s, error = %v", output.String(), err)
	}
}
