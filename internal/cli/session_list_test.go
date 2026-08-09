package cli

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/spf13/cobra"

	"github.com/sbekti/intern-api/internal/httpclient"
	"github.com/sbekti/intern-api/internal/session"
)

func sessionItemJSON(id, username string) string {
	return fmt.Sprintf(`{"id":"%s","username":"%s","client_name":"internctl","created_at":"2026-03-13T00:00:00Z","expires_at":"2026-03-14T00:00:00Z","idle_expires_at":"2026-03-13T12:00:00Z","is_current":false}`, id, username)
}

func newSessionListClient(t *testing.T, handler http.Handler) (*httpclient.Client, *httptest.Server) {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	manager := writeLoggedInProfile(t, t.TempDir(), server.URL)
	client, err := httpclient.New(server.URL, "default", session.BackendFile, manager)
	if err != nil {
		t.Fatalf("New client returned error: %v", err)
	}
	return client, server
}

func TestListAllSessionsRequestsEveryServerPage(t *testing.T) {
	requestedOffsets := make([]int, 0, 2)
	client, _ := newSessionListClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("limit"); got != "200" {
			t.Fatalf("limit = %q, want 200", got)
		}
		offset, err := strconv.Atoi(r.URL.Query().Get("offset"))
		if err != nil {
			t.Fatalf("invalid offset: %v", err)
		}
		requestedOffsets = append(requestedOffsets, offset)
		w.Header().Set("Content-Type", "application/json")
		if offset == 0 {
			_, _ = fmt.Fprint(w, authSessionPageJSON("["+sessionItemJSON("00000000-0000-0000-0000-000000000111", "alice")+"]", 200, 0, 2))
			return
		}
		if offset == 1 {
			_, _ = fmt.Fprint(w, authSessionPageJSON("["+sessionItemJSON("00000000-0000-0000-0000-000000000222", "bob")+"]", 200, 1, 2))
			return
		}
		http.Error(w, "unexpected offset", http.StatusBadRequest)
	}))

	items, err := listAllSessions(context.Background(), client, false)
	if err != nil {
		t.Fatalf("listAllSessions returned error: %v", err)
	}
	if len(items) != 2 || items[0].Username != "alice" || items[1].Username != "bob" {
		t.Fatalf("items = %+v, want both server pages", items)
	}
	if fmt.Sprint(requestedOffsets) != "[0 1]" {
		t.Fatalf("requested offsets = %v, want [0 1]", requestedOffsets)
	}
}

func TestSessionListEmptyJSONIsAnArray(t *testing.T) {
	_, server := newSessionListClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, authSessionPageJSON("[]", 200, 0, 0))
	}))
	var output string

	// The command-level check below uses the same endpoint but exercises JSON serialization.
	configDir := t.TempDir()
	_ = writeLoggedInProfile(t, configDir, server.URL)
	cmd := NewRootCommand()
	cmd.SetOut(testStringWriter{value: &output})
	cmd.SetErr(testStringWriter{value: new(string)})
	cmd.SetArgs([]string{"session", "list", "--config-dir", configDir, "--output", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if output != "[]\n" {
		t.Fatalf("empty JSON output = %q, want []", output)
	}
}

type testStringWriter struct{ value *string }

func (w testStringWriter) Write(p []byte) (int, error) {
	*w.value += string(p)
	return len(p), nil
}

func TestListAllSessionsRejectsMalformedPagination(t *testing.T) {
	client, _ := newSessionListClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, authSessionPageJSON("[]", 0, 0, 0))
	}))

	_, err := listAllSessions(context.Background(), client, false)
	if err == nil || err.Error() != "list sessions: malformed pagination" {
		t.Fatalf("error = %v, want malformed pagination", err)
	}
}

func TestListAllSessionsRejectsMismatchedOffset(t *testing.T) {
	client, _ := newSessionListClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, authSessionPageJSON("["+sessionItemJSON("00000000-0000-0000-0000-000000000111", "alice")+"]", 200, 1, 1))
	}))

	_, err := listAllSessions(context.Background(), client, false)
	if err == nil || err.Error() != "list sessions: malformed pagination" {
		t.Fatalf("error = %v, want malformed pagination", err)
	}
}

func TestListAllSessionsRejectsOversizedPageLimit(t *testing.T) {
	client, _ := newSessionListClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, authSessionPageJSON("[]", 201, 0, 0))
	}))

	_, err := listAllSessions(context.Background(), client, false)
	if err == nil || err.Error() != "list sessions: malformed pagination" {
		t.Fatalf("error = %v, want malformed pagination", err)
	}
}

func TestListAllSessionsRejectsPrematureEmptyPage(t *testing.T) {
	client, _ := newSessionListClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("offset") == "0" {
			_, _ = fmt.Fprint(w, authSessionPageJSON("["+sessionItemJSON("00000000-0000-0000-0000-000000000111", "alice")+"]", 200, 0, 2))
			return
		}
		_, _ = fmt.Fprint(w, authSessionPageJSON("[]", 200, 1, 2))
	}))

	_, err := listAllSessions(context.Background(), client, false)
	if err == nil || err.Error() != "list sessions: malformed pagination" {
		t.Fatalf("error = %v, want malformed pagination", err)
	}
}

func TestListAllSessionsRejectsShrinkingTotal(t *testing.T) {
	client, _ := newSessionListClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("offset") == "0" {
			_, _ = fmt.Fprint(w, authSessionPageJSON("["+sessionItemJSON("00000000-0000-0000-0000-000000000111", "alice")+"]", 200, 0, 2))
			return
		}
		_, _ = fmt.Fprint(w, authSessionPageJSON("["+sessionItemJSON("00000000-0000-0000-0000-000000000222", "bob")+"]", 200, 1, 1))
	}))

	_, err := listAllSessions(context.Background(), client, false)
	if err == nil || err.Error() != "list sessions: malformed pagination" {
		t.Fatalf("error = %v, want malformed pagination", err)
	}
}

func TestListAllSessionsUsesReturnedItemCountForPartialPages(t *testing.T) {
	requestedOffsets := make([]string, 0, 3)
	client, _ := newSessionListClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		offset := r.URL.Query().Get("offset")
		requestedOffsets = append(requestedOffsets, offset)
		w.Header().Set("Content-Type", "application/json")
		switch offset {
		case "0":
			_, _ = fmt.Fprint(w, authSessionPageJSON("["+sessionItemJSON("00000000-0000-0000-0000-000000000111", "alice")+"]", 200, 0, 3))
		case "1":
			_, _ = fmt.Fprint(w, authSessionPageJSON("["+sessionItemJSON("00000000-0000-0000-0000-000000000222", "bob")+"]", 200, 1, 3))
		case "2":
			_, _ = fmt.Fprint(w, authSessionPageJSON("["+sessionItemJSON("00000000-0000-0000-0000-000000000333", "carol")+"]", 200, 2, 3))
		default:
			http.Error(w, "unexpected offset", http.StatusBadRequest)
		}
	}))

	items, err := listAllSessions(context.Background(), client, false)
	if err != nil {
		t.Fatalf("listAllSessions returned error: %v", err)
	}
	if len(items) != 3 || items[0].Username != "alice" || items[1].Username != "bob" || items[2].Username != "carol" {
		t.Fatalf("items = %+v, want all partial pages without gaps", items)
	}
	if fmt.Sprint(requestedOffsets) != "[0 1 2]" {
		t.Fatalf("requested offsets = %v, want [0 1 2]", requestedOffsets)
	}
}

func TestSessionListDropsPaginationFlags(t *testing.T) {
	root := NewRootCommand()
	var sessionCommand *cobra.Command
	for _, command := range root.Commands() {
		if command.Name() == "session" {
			sessionCommand = command
		}
	}
	if sessionCommand == nil {
		t.Fatal("session command not registered")
	}
	listCommand, _, err := sessionCommand.Find([]string{"list"})
	if err != nil {
		t.Fatalf("find list command: %v", err)
	}
	if listCommand.Flags().Lookup("page") != nil || listCommand.Flags().Lookup("page-size") != nil {
		t.Fatal("session list still exposes removed pagination flags")
	}
}
