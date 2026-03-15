//go:build integration

package presence

import (
	"context"
	"encoding/json"
	"log/slog"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/sbekti/intern-api/internal/config"
	"github.com/sbekti/intern-api/internal/db"
	"github.com/sbekti/intern-api/internal/testutil"
)

type fakeUniFiClient struct {
	clients []UniFiActiveClient
}

func (f fakeUniFiClient) ListActiveClients(ctx context.Context, source config.PresenceSourceConfig) ([]UniFiActiveClient, error) {
	return append([]UniFiActiveClient(nil), f.clients...), nil
}

type scriptedUniFiClient struct {
	clients []UniFiActiveClient
}

func (f *scriptedUniFiClient) ListActiveClients(ctx context.Context, source config.PresenceSourceConfig) ([]UniFiActiveClient, error) {
	return append([]UniFiActiveClient(nil), f.clients...), nil
}

type fakeJuniperClient struct {
	clients []polledPresenceClient
}

func (f fakeJuniperClient) ListActiveClients(ctx context.Context, source config.PresenceSourceConfig, pollTime time.Time) ([]polledPresenceClient, error) {
	return append([]polledPresenceClient(nil), f.clients...), nil
}

func TestServiceSyncWirelessOnceNormalizesRadiusAndUniFi(t *testing.T) {
	t.Parallel()

	pg := testutil.StartPostgres(t)
	ctx := context.Background()
	queries := db.New(pg.Pool)

	actor, err := queries.UpsertUserByUsername(ctx, db.UpsertUserByUsernameParams{
		Username: "test-admin",
		Name:     "Test Admin",
		Email:    "test-admin@example.com",
		Groups:   []string{"Super-Users"},
	})
	if err != nil {
		t.Fatalf("failed to create actor: %v", err)
	}

	device, err := queries.CreateNetworkDevice(ctx, db.CreateNetworkDeviceParams{
		MacAddress:      "80:b9:89:30:9d:63",
		DisplayName:     "Managed Handset",
		VlanID:          1,
		CreatedByUserID: actor.ID,
		UpdatedByUserID: actor.ID,
	})
	if err != nil {
		t.Fatalf("failed to create network device: %v", err)
	}

	startedAt := time.Date(2026, 3, 15, 18, 24, 20, 0, time.UTC)
	updatedAt := time.Date(2026, 3, 15, 18, 30, 20, 0, time.UTC)
	lastSeenAt := time.Date(2026, 3, 15, 18, 49, 11, 0, time.UTC)

	if _, err := pg.Pool.Exec(ctx, `
		INSERT INTO radacct (
			acctsessionid,
			acctuniqueid,
			nasipaddress,
			acctstarttime,
			acctupdatetime,
			calledstationid,
			callingstationid
		) VALUES
			('sess-open', 'unique-open', '10.20.0.1', $1, $2, '1A-E8-29-19-CB-5D:corp-wifi', '80-B9-89-30-9D-63'),
			('sess-closed', 'unique-closed', '10.20.0.1', $1 - interval '1 hour', $2 - interval '40 minutes', '18-E8-29-49-CB-5C:corp-guest', '6A-1A-F8-41-BA-9F')
	`, startedAt, updatedAt); err != nil {
		t.Fatalf("failed to seed radacct rows: %v", err)
	}

	if _, err := pg.Pool.Exec(ctx, `
		UPDATE radacct
		SET acctstoptime = acctupdatetime
		WHERE acctuniqueid = 'unique-closed'
	`); err != nil {
		t.Fatalf("failed to close radacct row: %v", err)
	}

	service := NewService(slog.Default(), pg.Pool, config.PresenceConfig{
		Enabled:                true,
		PollIntervalDefault:    5 * time.Minute,
		DisconnectGraceDefault: 15 * time.Minute,
		Sources: []config.PresenceSourceConfig{
			{
				Key:             "unifi-site-a",
				Type:            config.PresenceSourceTypeUnifi,
				DisplayName:     "Site A UniFi",
				Host:            "https://controller.internal.example",
				Port:            443,
				Site:            "default",
				PollInterval:    5 * time.Minute,
				DisconnectGrace: 15 * time.Minute,
			},
		},
	}, fakeUniFiClient{
		clients: []UniFiActiveClient{
			{
				MAC:       "80-B9-89-30-9D-63",
				Hostname:  "handset-01",
				ESSID:     "corp-wifi",
				APMAC:     "18-E8-29-49-CB-5C",
				BSSID:     "1A-E8-29-19-CB-5D",
				AssocTime: startedAt,
				LastSeen:  lastSeenAt,
			},
		},
	}, fakeJuniperClient{})
	service.now = func() time.Time { return lastSeenAt }

	if err := service.SyncWirelessOnce(ctx); err != nil {
		t.Fatalf("expected sync to succeed, got %v", err)
	}

	var sessionCount, clientCount, observationPointCount, workerStateCount int
	if err := pg.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM presence_sessions`).Scan(&sessionCount); err != nil {
		t.Fatalf("failed to count sessions: %v", err)
	}
	if err := pg.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM presence_clients`).Scan(&clientCount); err != nil {
		t.Fatalf("failed to count clients: %v", err)
	}
	if err := pg.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM presence_observation_points`).Scan(&observationPointCount); err != nil {
		t.Fatalf("failed to count observation points: %v", err)
	}
	if err := pg.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM presence_worker_state`).Scan(&workerStateCount); err != nil {
		t.Fatalf("failed to count worker state: %v", err)
	}

	if sessionCount != 2 || clientCount != 2 || observationPointCount != 3 || workerStateCount != 2 {
		t.Fatalf("unexpected counts sessions=%d clients=%d observation_points=%d worker_state=%d", sessionCount, clientCount, observationPointCount, workerStateCount)
	}

	var lastSourceType, status string
	var networkDeviceID pgtype.UUID
	var lastSSID *string
	var lastMetadata []byte
	if err := pg.Pool.QueryRow(ctx, `
		SELECT last_source_type, status, network_device_id, last_ssid, last_metadata
		FROM presence_clients
		WHERE lower(mac_address) = '80:b9:89:30:9d:63'
	`).Scan(&lastSourceType, &status, &networkDeviceID, &lastSSID, &lastMetadata); err != nil {
		t.Fatalf("failed to load managed presence client: %v", err)
	}

	if lastSourceType != sourceTypeUnifi {
		t.Fatalf("expected latest source to be unifi, got %q", lastSourceType)
	}
	if status != clientStatusOnline {
		t.Fatalf("expected online status, got %q", status)
	}
	if networkDeviceID != device.ID {
		t.Fatalf("expected managed device link %v, got %v", device.ID, networkDeviceID)
	}
	if lastSSID == nil || *lastSSID != "corp-wifi" {
		t.Fatalf("expected last_ssid to be corp-wifi, got %#v", lastSSID)
	}

	var decoded map[string]any
	if err := json.Unmarshal(lastMetadata, &decoded); err != nil {
		t.Fatalf("failed to decode metadata: %v", err)
	}
	if decoded["hostname"] != "handset-01" {
		t.Fatalf("expected unifi hostname metadata, got %#v", decoded)
	}

	var radiusState []byte
	if err := pg.Pool.QueryRow(ctx, `
		SELECT state
		FROM presence_worker_state
		WHERE worker_name = $1 AND source_key = $2
	`, workerNameRadiusSync, radiusSourceKey).Scan(&radiusState); err != nil {
		t.Fatalf("failed to load radius worker state: %v", err)
	}
	var radiusCursor radiusWorkerState
	if err := json.Unmarshal(radiusState, &radiusCursor); err != nil {
		t.Fatalf("failed to decode radius worker state: %v", err)
	}
	if radiusCursor.LastRadacctID != 2 {
		t.Fatalf("expected last radacct id 2, got %d", radiusCursor.LastRadacctID)
	}

	if err := service.SyncRadius(ctx); err != nil {
		t.Fatalf("expected second radius sync to succeed, got %v", err)
	}

	if err := pg.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM presence_sessions`).Scan(&sessionCount); err != nil {
		t.Fatalf("failed to recount sessions: %v", err)
	}
	if sessionCount != 2 {
		t.Fatalf("expected no duplicate sessions after re-sync, got %d", sessionCount)
	}
}

func TestServiceSyncUniFiSourceClosesSyntheticSessionsAfterGrace(t *testing.T) {
	t.Parallel()

	pg := testutil.StartPostgres(t)
	ctx := context.Background()
	unifiSource := config.PresenceSourceConfig{
		Key:             "unifi-site-a",
		Type:            config.PresenceSourceTypeUnifi,
		DisplayName:     "Site A UniFi",
		Host:            "https://controller.internal.example",
		Port:            443,
		Site:            "default",
		PollInterval:    5 * time.Minute,
		DisconnectGrace: 15 * time.Minute,
	}

	activeClient := &scriptedUniFiClient{
		clients: []UniFiActiveClient{
			{
				MAC:       "80-B9-89-30-9D-63",
				Hostname:  "handset-01",
				ESSID:     "corp-wifi",
				APMAC:     "18-E8-29-49-CB-5C",
				BSSID:     "1A-E8-29-19-CB-5D",
				AssocTime: time.Date(2026, 3, 15, 18, 24, 20, 0, time.UTC),
				LastSeen:  time.Date(2026, 3, 15, 18, 30, 20, 0, time.UTC),
			},
		},
	}

	service := NewService(slog.Default(), pg.Pool, config.PresenceConfig{
		Enabled:                true,
		PollIntervalDefault:    5 * time.Minute,
		DisconnectGraceDefault: 15 * time.Minute,
		Sources:                []config.PresenceSourceConfig{unifiSource},
	}, activeClient, fakeJuniperClient{})
	service.now = func() time.Time { return time.Date(2026, 3, 15, 18, 30, 20, 0, time.UTC) }

	if err := service.SyncUniFiSource(ctx, unifiSource); err != nil {
		t.Fatalf("expected initial unifi sync to succeed, got %v", err)
	}

	activeClient.clients = nil
	service.now = func() time.Time { return time.Date(2026, 3, 15, 18, 50, 20, 0, time.UTC) }
	if err := service.SyncUniFiSource(ctx, unifiSource); err != nil {
		t.Fatalf("expected stale unifi sync to succeed, got %v", err)
	}

	var status string
	var endedAt time.Time
	var metadata []byte
	if err := pg.Pool.QueryRow(ctx, `
		SELECT c.status, s.ended_at, s.metadata
		FROM presence_clients c
		JOIN presence_sessions s ON lower(s.client_mac_address) = lower(c.mac_address)
		WHERE lower(c.mac_address) = '80:b9:89:30:9d:63'
		ORDER BY s.started_at DESC
		LIMIT 1
	`).Scan(&status, &endedAt, &metadata); err != nil {
		t.Fatalf("failed to load synthetic session: %v", err)
	}

	if status != clientStatusOffline {
		t.Fatalf("expected client to be offline, got %q", status)
	}
	expectedEndedAt := time.Date(2026, 3, 15, 18, 30, 20, 0, time.UTC)
	if !endedAt.Equal(expectedEndedAt) {
		t.Fatalf("expected session to end at last seen %s, got %s", expectedEndedAt, endedAt)
	}

	var decoded map[string]any
	if err := json.Unmarshal(metadata, &decoded); err != nil {
		t.Fatalf("failed to decode session metadata: %v", err)
	}
	if decoded["ended_by"] != "unifi_poll_timeout" {
		t.Fatalf("expected unifi poll timeout metadata, got %#v", decoded)
	}
	if decoded["accounting_backed"] != false {
		t.Fatalf("expected synthetic session metadata, got %#v", decoded)
	}
}

func TestServiceSyncUniFiSourceClosesRadiusSessionsAfterGrace(t *testing.T) {
	t.Parallel()

	pg := testutil.StartPostgres(t)
	ctx := context.Background()

	startedAt := time.Date(2026, 3, 15, 18, 24, 20, 0, time.UTC)
	if _, err := pg.Pool.Exec(ctx, `
		INSERT INTO radacct (
			acctsessionid,
			acctuniqueid,
			nasipaddress,
			acctstarttime,
			acctupdatetime,
			calledstationid,
			callingstationid
		) VALUES
			('sess-open', 'unique-open', '10.20.0.1', $1, $1, '1A-E8-29-19-CB-5D:corp-wifi', '80-B9-89-30-9D-63')
	`, startedAt); err != nil {
		t.Fatalf("failed to seed radacct row: %v", err)
	}

	unifiSource := config.PresenceSourceConfig{
		Key:             "unifi-site-a",
		Type:            config.PresenceSourceTypeUnifi,
		DisplayName:     "Site A UniFi",
		Host:            "https://controller.internal.example",
		Port:            443,
		Site:            "default",
		PollInterval:    5 * time.Minute,
		DisconnectGrace: 15 * time.Minute,
	}
	activeClient := &scriptedUniFiClient{
		clients: []UniFiActiveClient{
			{
				MAC:       "80-B9-89-30-9D-63",
				Hostname:  "handset-01",
				ESSID:     "corp-wifi",
				APMAC:     "18-E8-29-49-CB-5C",
				BSSID:     "1A-E8-29-19-CB-5D",
				AssocTime: startedAt,
				LastSeen:  startedAt.Add(5 * time.Minute),
			},
		},
	}

	service := NewService(slog.Default(), pg.Pool, config.PresenceConfig{
		Enabled:                true,
		PollIntervalDefault:    5 * time.Minute,
		DisconnectGraceDefault: 15 * time.Minute,
		Sources:                []config.PresenceSourceConfig{unifiSource},
	}, activeClient, fakeJuniperClient{})
	service.now = func() time.Time { return startedAt.Add(5 * time.Minute) }

	if err := service.SyncRadius(ctx); err != nil {
		t.Fatalf("expected radius sync to succeed, got %v", err)
	}
	if err := service.SyncUniFiSource(ctx, unifiSource); err != nil {
		t.Fatalf("expected unifi sync to succeed, got %v", err)
	}

	activeClient.clients = nil
	service.now = func() time.Time { return startedAt.Add(25 * time.Minute) }
	if err := service.SyncUniFiSource(ctx, unifiSource); err != nil {
		t.Fatalf("expected stale unifi sync to succeed, got %v", err)
	}

	var endedAt time.Time
	var sourceType string
	var metadata []byte
	if err := pg.Pool.QueryRow(ctx, `
		SELECT ended_at, source_type, metadata
		FROM presence_sessions
		WHERE source_session_key = 'unique-open'
	`).Scan(&endedAt, &sourceType, &metadata); err != nil {
		t.Fatalf("failed to load radius-backed session: %v", err)
	}

	if sourceType != sourceTypeRadius {
		t.Fatalf("expected radius-backed session to retain source type, got %q", sourceType)
	}
	expectedEndedAt := startedAt.Add(5 * time.Minute)
	if !endedAt.Equal(expectedEndedAt) {
		t.Fatalf("expected radius-backed session to close at %s, got %s", expectedEndedAt, endedAt)
	}

	var decoded map[string]any
	if err := json.Unmarshal(metadata, &decoded); err != nil {
		t.Fatalf("failed to decode radius-backed session metadata: %v", err)
	}
	if decoded["ended_by"] != "unifi_poll_timeout" {
		t.Fatalf("expected poll timeout metadata, got %#v", decoded)
	}
	if decoded["accounting_backed"] != true {
		t.Fatalf("expected accounting_backed to be preserved, got %#v", decoded)
	}

	var openCount int
	if err := pg.Pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM presence_sessions
		WHERE lower(client_mac_address) = '80:b9:89:30:9d:63'
		  AND medium = $1
		  AND ended_at IS NULL
	`, mediumWireless).Scan(&openCount); err != nil {
		t.Fatalf("failed to count open sessions after inferred close: %v", err)
	}
	if openCount != 0 {
		t.Fatalf("expected no open session after inferred close, got %d", openCount)
	}
}

func TestServiceSyncUniFiSourceSplitsSessionsWhenObservationMoves(t *testing.T) {
	t.Parallel()

	pg := testutil.StartPostgres(t)
	ctx := context.Background()
	unifiSource := config.PresenceSourceConfig{
		Key:             "unifi-site-a",
		Type:            config.PresenceSourceTypeUnifi,
		DisplayName:     "Site A UniFi",
		Host:            "https://controller.internal.example",
		Port:            443,
		Site:            "default",
		PollInterval:    5 * time.Minute,
		DisconnectGrace: 15 * time.Minute,
	}
	activeClient := &scriptedUniFiClient{
		clients: []UniFiActiveClient{
			{
				MAC:       "80-B9-89-30-9D-63",
				Hostname:  "handset-01",
				ESSID:     "corp-wifi",
				APMAC:     "18-E8-29-49-CB-5C",
				BSSID:     "1A-E8-29-19-CB-5D",
				AssocTime: time.Date(2026, 3, 15, 18, 24, 20, 0, time.UTC),
				LastSeen:  time.Date(2026, 3, 15, 18, 30, 20, 0, time.UTC),
			},
		},
	}

	service := NewService(slog.Default(), pg.Pool, config.PresenceConfig{
		Enabled:                true,
		PollIntervalDefault:    5 * time.Minute,
		DisconnectGraceDefault: 15 * time.Minute,
		Sources:                []config.PresenceSourceConfig{unifiSource},
	}, activeClient, fakeJuniperClient{})
	service.now = func() time.Time { return time.Date(2026, 3, 15, 18, 30, 20, 0, time.UTC) }

	if err := service.SyncUniFiSource(ctx, unifiSource); err != nil {
		t.Fatalf("expected first unifi sync to succeed, got %v", err)
	}

	activeClient.clients = []UniFiActiveClient{
		{
			MAC:       "80-B9-89-30-9D-63",
			Hostname:  "handset-01",
			ESSID:     "corp-wifi",
			APMAC:     "18-E8-29-49-CB-5D",
			BSSID:     "1A-E8-29-19-CB-5E",
			AssocTime: time.Date(2026, 3, 15, 18, 35, 20, 0, time.UTC),
			LastSeen:  time.Date(2026, 3, 15, 18, 40, 20, 0, time.UTC),
		},
	}
	service.now = func() time.Time { return time.Date(2026, 3, 15, 18, 40, 20, 0, time.UTC) }
	if err := service.SyncUniFiSource(ctx, unifiSource); err != nil {
		t.Fatalf("expected roaming unifi sync to succeed, got %v", err)
	}

	rows, err := pg.Pool.Query(ctx, `
		SELECT source_session_key, ended_at, last_seen_at
		FROM presence_sessions
		WHERE lower(client_mac_address) = '80:b9:89:30:9d:63'
		ORDER BY started_at ASC
	`)
	if err != nil {
		t.Fatalf("failed to list roaming sessions: %v", err)
	}
	defer rows.Close()

	var sessionKeys []string
	var endedAts []pgtype.Timestamptz
	for rows.Next() {
		var key string
		var endedAt pgtype.Timestamptz
		var lastSeen pgtype.Timestamptz
		if err := rows.Scan(&key, &endedAt, &lastSeen); err != nil {
			t.Fatalf("failed to scan roaming session: %v", err)
		}
		sessionKeys = append(sessionKeys, key)
		endedAts = append(endedAts, endedAt)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("failed to iterate roaming sessions: %v", err)
	}

	if len(sessionKeys) != 2 {
		t.Fatalf("expected two split sessions, got %d", len(sessionKeys))
	}
	if !endedAts[0].Valid {
		t.Fatalf("expected first session to be closed after roam")
	}
	if endedAts[1].Valid {
		t.Fatalf("expected second session to remain open after roam")
	}
}

func TestServiceSyncJuniperSourceCreatesAndClosesSyntheticWiredSessions(t *testing.T) {
	t.Parallel()

	pg := testutil.StartPostgres(t)
	ctx := context.Background()
	juniperSource := config.PresenceSourceConfig{
		Key:             "juniper-switch-a",
		Type:            config.PresenceSourceTypeJuniperSNMP,
		DisplayName:     "switch-a",
		Host:            "192.0.2.10",
		Port:            161,
		PollInterval:    5 * time.Minute,
		DisconnectGrace: 15 * time.Minute,
	}
	activeClient := &fakeJuniperClient{
		clients: []polledPresenceClient{
			{
				MAC:                   "EC-B5-FA-B0-C2-00",
				DisplayName:           "ge-0/0/4",
				ObservationExternalID: "ge-0/0/4",
				FirstSeen:             time.Date(2026, 3, 15, 19, 0, 0, 0, time.UTC),
				LastSeen:              time.Date(2026, 3, 15, 19, 0, 0, 0, time.UTC),
				Metadata: map[string]any{
					"interface_name":   "ge-0/0/4",
					"selection_reason": "single_mac_non_lldp_port",
				},
			},
		},
	}

	service := NewService(slog.Default(), pg.Pool, config.PresenceConfig{
		Enabled:                true,
		PollIntervalDefault:    5 * time.Minute,
		DisconnectGraceDefault: 15 * time.Minute,
		Sources:                []config.PresenceSourceConfig{juniperSource},
	}, fakeUniFiClient{}, activeClient)
	service.now = func() time.Time { return time.Date(2026, 3, 15, 19, 0, 0, 0, time.UTC) }

	if err := service.SyncJuniperSource(ctx, juniperSource); err != nil {
		t.Fatalf("expected initial juniper sync to succeed, got %v", err)
	}

	activeClient.clients = nil
	service.now = func() time.Time { return time.Date(2026, 3, 15, 19, 20, 0, 0, time.UTC) }
	if err := service.SyncJuniperSource(ctx, juniperSource); err != nil {
		t.Fatalf("expected stale juniper sync to succeed, got %v", err)
	}

	var status string
	var sourceType string
	var medium string
	var endedAt time.Time
	if err := pg.Pool.QueryRow(ctx, `
		SELECT c.status, s.source_type, s.medium, s.ended_at
		FROM presence_clients c
		JOIN presence_sessions s ON lower(s.client_mac_address) = lower(c.mac_address)
		WHERE lower(c.mac_address) = 'ec:b5:fa:b0:c2:00'
		ORDER BY s.started_at DESC
		LIMIT 1
	`).Scan(&status, &sourceType, &medium, &endedAt); err != nil {
		t.Fatalf("failed to load juniper-backed session: %v", err)
	}

	if status != clientStatusOffline {
		t.Fatalf("expected client to be offline, got %q", status)
	}
	if sourceType != sourceTypeJuniperSNMP || medium != mediumWired {
		t.Fatalf("expected juniper wired session, got source_type=%q medium=%q", sourceType, medium)
	}
	expectedEndedAt := time.Date(2026, 3, 15, 19, 0, 0, 0, time.UTC)
	if !endedAt.Equal(expectedEndedAt) {
		t.Fatalf("expected wired session to end at %s, got %s", expectedEndedAt, endedAt)
	}
}

func TestServiceSyncJuniperSourceSplitsSessionsWhenPortChanges(t *testing.T) {
	t.Parallel()

	pg := testutil.StartPostgres(t)
	ctx := context.Background()
	juniperSource := config.PresenceSourceConfig{
		Key:             "juniper-switch-a",
		Type:            config.PresenceSourceTypeJuniperSNMP,
		DisplayName:     "switch-a",
		Host:            "192.0.2.10",
		Port:            161,
		PollInterval:    5 * time.Minute,
		DisconnectGrace: 15 * time.Minute,
	}
	activeClient := &fakeJuniperClient{
		clients: []polledPresenceClient{
			{
				MAC:                   "EC-B5-FA-B0-C2-00",
				DisplayName:           "ge-0/0/4",
				ObservationExternalID: "ge-0/0/4",
				FirstSeen:             time.Date(2026, 3, 15, 19, 0, 0, 0, time.UTC),
				LastSeen:              time.Date(2026, 3, 15, 19, 0, 0, 0, time.UTC),
				Metadata:              map[string]any{"interface_name": "ge-0/0/4"},
			},
		},
	}

	service := NewService(slog.Default(), pg.Pool, config.PresenceConfig{
		Enabled:                true,
		PollIntervalDefault:    5 * time.Minute,
		DisconnectGraceDefault: 15 * time.Minute,
		Sources:                []config.PresenceSourceConfig{juniperSource},
	}, fakeUniFiClient{}, activeClient)
	service.now = func() time.Time { return time.Date(2026, 3, 15, 19, 0, 0, 0, time.UTC) }

	if err := service.SyncJuniperSource(ctx, juniperSource); err != nil {
		t.Fatalf("expected first juniper sync to succeed, got %v", err)
	}

	activeClient.clients = []polledPresenceClient{
		{
			MAC:                   "EC-B5-FA-B0-C2-00",
			DisplayName:           "ge-0/0/5",
			ObservationExternalID: "ge-0/0/5",
			FirstSeen:             time.Date(2026, 3, 15, 19, 5, 0, 0, time.UTC),
			LastSeen:              time.Date(2026, 3, 15, 19, 5, 0, 0, time.UTC),
			Metadata:              map[string]any{"interface_name": "ge-0/0/5"},
		},
	}
	service.now = func() time.Time { return time.Date(2026, 3, 15, 19, 5, 0, 0, time.UTC) }
	if err := service.SyncJuniperSource(ctx, juniperSource); err != nil {
		t.Fatalf("expected moved juniper sync to succeed, got %v", err)
	}

	rows, err := pg.Pool.Query(ctx, `
		SELECT source_session_key, ended_at
		FROM presence_sessions
		WHERE lower(client_mac_address) = 'ec:b5:fa:b0:c2:00'
		ORDER BY started_at ASC
	`)
	if err != nil {
		t.Fatalf("failed to list juniper sessions: %v", err)
	}
	defer rows.Close()

	var endedAts []pgtype.Timestamptz
	for rows.Next() {
		var key string
		var endedAt pgtype.Timestamptz
		if err := rows.Scan(&key, &endedAt); err != nil {
			t.Fatalf("failed to scan juniper session: %v", err)
		}
		endedAts = append(endedAts, endedAt)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("failed to iterate juniper sessions: %v", err)
	}

	if len(endedAts) != 2 {
		t.Fatalf("expected two split wired sessions, got %d", len(endedAts))
	}
	if !endedAts[0].Valid {
		t.Fatalf("expected first wired session to be closed after port move")
	}
	if endedAts[1].Valid {
		t.Fatalf("expected second wired session to remain open after port move")
	}
}
