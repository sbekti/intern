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

func TestServiceSyncWirelessOnceNormalizesRadiusAndUniFi(t *testing.T) {
	t.Parallel()

	pg := testutil.StartPostgres(t)
	ctx := context.Background()
	queries := db.New(pg.Pool)

	actor, err := queries.UpsertUserByUsername(ctx, db.UpsertUserByUsernameParams{
		Username: "presence-admin",
		Name:     "Presence Admin",
		Email:    "presence-admin@example.com",
		Groups:   []string{"Super-Users"},
	})
	if err != nil {
		t.Fatalf("failed to create actor: %v", err)
	}

	device, err := queries.CreateNetworkDevice(ctx, db.CreateNetworkDeviceParams{
		MacAddress:      "80:b9:89:30:9d:63",
		DisplayName:     "Alice iPhone",
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
			('sess-open', 'unique-open', '10.20.0.1', $1, $2, '1A-E8-29-19-CB-5D:bektinet-wpa', '80-B9-89-30-9D-63'),
			('sess-closed', 'unique-closed', '10.20.0.1', $1 - interval '1 hour', $2 - interval '40 minutes', '18-E8-29-49-CB-5C:bektinet-psk', '6A-1A-F8-41-BA-9F')
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
		Enabled:             true,
		PollIntervalDefault: 5 * time.Minute,
		Sources: []config.PresenceSourceConfig{
			{
				Key:          "unifi-jfk1",
				Type:         config.PresenceSourceTypeUnifi,
				DisplayName:  "JFK1 UniFi",
				Host:         "https://unifi.example.test",
				Port:         443,
				Site:         "default",
				PollInterval: 5 * time.Minute,
			},
		},
	}, fakeUniFiClient{
		clients: []UniFiActiveClient{
			{
				MAC:       "80-B9-89-30-9D-63",
				Hostname:  "alice-iphone",
				ESSID:     "bektinet-wpa",
				APMAC:     "18-E8-29-49-CB-5C",
				BSSID:     "1A-E8-29-19-CB-5D",
				AssocTime: startedAt,
				LastSeen:  lastSeenAt,
			},
		},
	})
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
	if lastSSID == nil || *lastSSID != "bektinet-wpa" {
		t.Fatalf("expected last_ssid to be bektinet-wpa, got %#v", lastSSID)
	}

	var decoded map[string]any
	if err := json.Unmarshal(lastMetadata, &decoded); err != nil {
		t.Fatalf("failed to decode metadata: %v", err)
	}
	if decoded["hostname"] != "alice-iphone" {
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
