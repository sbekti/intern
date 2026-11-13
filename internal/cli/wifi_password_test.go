package cli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sbekti/intern/internal/api"
	"github.com/spf13/cobra"
)

func TestWiFiPasswordStdinPreservesSpacesAndRejectsOversizedInput(t *testing.T) {
	for _, test := range []struct {
		input string
		valid bool
	}{
		{" leading and trailing spaces \r\n", true},
		{strings.Repeat("a", 63) + "\n", true},
		{strings.Repeat("a", 64) + "\n", false},
		{"too\nshort\n", false},
	} {
		cmd := &cobra.Command{}
		var flags wifiPasswordFlags
		flags.bind(cmd)
		if err := cmd.ParseFlags([]string{"--wifi-password", "--private-wifi-password"}); err != nil {
			t.Fatal(err)
		}
		cmd.SetIn(strings.NewReader(test.input))
		enabled, password, err := flags.read(cmd)
		if (err == nil) != test.valid {
			t.Fatalf("valid=%v, error=%v", test.valid, err)
		}
		if test.valid && (enabled == nil || !*enabled || password == nil || *password != strings.TrimSuffix(strings.TrimSuffix(test.input, "\n"), "\r")) {
			t.Fatal("stdin password or enabled state changed")
		}
	}
}

func TestDevicePasswordCommandsKeepSecretsOutOfWriteOutput(t *testing.T) {
	const id = "11111111-1111-1111-1111-111111111111"
	const password = "forest-river-cloud-moon"
	var patch api.NetworkDevicePatch
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer access-token" {
			t.Error("missing session authorization")
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/networks/devices/" + id:
			if r.Method != http.MethodPatch {
				t.Error("unexpected method")
			}
			if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
				t.Error(err)
			}
			_, _ = w.Write([]byte(`{"id":"` + id + `","display_name":"Example device","private_wifi_password_enabled":true,"wifi_password_set":true}`))
		case "/api/v1/networks/devices/" + id + "/wifi-password/reveal", "/api/v1/networks/devices/wifi-password/generate":
			if r.Method != http.MethodPost {
				t.Error("unexpected method")
			}
			_, _ = w.Write([]byte(`{"password":"` + password + `"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	dir := t.TempDir()
	writeLoggedInProfile(t, dir, server.URL)
	run := func(input string, args ...string) string {
		t.Helper()
		cmd := NewRootCommand()
		var out, errOut bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&errOut)
		cmd.SetIn(strings.NewReader(input))
		cmd.SetArgs(append([]string{"--config-dir", dir, "device"}, args...))
		if err := cmd.Execute(); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(errOut.String(), password) {
			t.Fatal("stderr exposed the password")
		}
		return out.String()
	}
	output := run(password+"\n", "update", id, "--private-wifi-password", "--wifi-password")
	if patch.PrivateWifiPasswordEnabled == nil || !*patch.PrivateWifiPasswordEnabled || patch.WifiPassword == nil || *patch.WifiPassword != password {
		t.Fatal("update did not preserve secret input and enabled state")
	}
	if strings.Contains(output, password) {
		t.Fatal("ordinary update output exposed the password")
	}
	if run("", "password", "show", id) != password+"\n" {
		t.Fatal("explicit show did not return the saved password")
	}
	if run("", "password", "generate") != password+"\n" {
		t.Fatal("explicit generate did not return the unsaved password")
	}
}
