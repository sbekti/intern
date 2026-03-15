package presence

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/sbekti/intern-api/internal/config"
)

func TestParseCalledStationID(t *testing.T) {
	t.Parallel()

	bssid, ssid := parseCalledStationID("1A-E8-29-19-CB-5D:corp-wifi")
	if bssid != "1a:e8:29:19:cb:5d" {
		t.Fatalf("expected normalized bssid, got %q", bssid)
	}
	if ssid != "corp-wifi" {
		t.Fatalf("expected ssid, got %q", ssid)
	}
}

func TestUniFiHTTPClientFallbackLoginAndActiveClients(t *testing.T) {
	t.Parallel()

	loginCalls := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/auth/login":
			loginCalls++
			http.NotFound(w, r)
		case "/api/login":
			loginCalls++
			w.WriteHeader(http.StatusOK)
		case "/api/s/default/stat/sta":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":[{"mac":"80-B9-89-30-9D-63","hostname":"handset-01","essid":"corp-wifi","ap_mac":"18:E8:29:49:CB:5C","bssid":"1A-E8-29-19-CB-5D","assoc_time":1742061600,"last_seen":1742062200}]}`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer upstream.Close()

	client := NewUniFiHTTPClient(upstream.Client())
	clients, err := client.ListActiveClients(context.Background(), config.PresenceSourceConfig{
		Host: "http://" + upstream.Listener.Addr().String(),
		Site: "default",
		CredentialEnv: config.PresenceSourceCredentialEnvConfig{
			Username: "neteng",
			Password: "secret",
		},
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if loginCalls != 2 {
		t.Fatalf("expected both login endpoints to be tried, got %d calls", loginCalls)
	}
	if len(clients) != 1 {
		t.Fatalf("expected one client, got %d", len(clients))
	}
	if clients[0].MAC != "80-B9-89-30-9D-63" || clients[0].ESSID != "corp-wifi" {
		t.Fatalf("unexpected client payload %#v", clients[0])
	}
	if clients[0].LastSeen.IsZero() || clients[0].AssocTime.IsZero() {
		t.Fatalf("expected parsed timestamps, got %#v", clients[0])
	}
}

func TestParseUniFiTimestamp(t *testing.T) {
	t.Parallel()

	value := parseUniFiTimestamp(json.Number("1742062200"))
	if value.IsZero() {
		t.Fatal("expected timestamp to parse")
	}
	if !value.Equal(time.Unix(1742062200, 0).UTC()) {
		t.Fatalf("unexpected parsed time %s", value)
	}
}
