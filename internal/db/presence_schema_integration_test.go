//go:build integration

package db_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/sbekti/intern-api/internal/testutil"
)

func TestPresenceSchemaSupportsGenericSourceData(t *testing.T) {
	t.Parallel()

	pg := testutil.StartPostgres(t)
	ctx := context.Background()

	var createdByUserID string
	if err := pg.Pool.QueryRow(ctx, `
		INSERT INTO users (username, name, email, groups)
		VALUES ('test-admin', 'Test Admin', 'test-admin@example.com', ARRAY['Super-Users'])
		RETURNING id::text
	`).Scan(&createdByUserID); err != nil {
		t.Fatalf("failed to insert user: %v", err)
	}

	var networkDeviceID string
	if err := pg.Pool.QueryRow(ctx, `
		INSERT INTO network_devices (mac_address, display_name, vlan_id, created_by_user_id, updated_by_user_id)
		VALUES ('aa:bb:cc:dd:ee:ff', 'Managed Handset', 1, $1::uuid, $1::uuid)
		RETURNING id::text
	`, createdByUserID).Scan(&networkDeviceID); err != nil {
		t.Fatalf("failed to insert network device: %v", err)
	}

	var observationPointID string
	var observationPointUpdatedAt time.Time
	if err := pg.Pool.QueryRow(ctx, `
		INSERT INTO presence_observation_points (
			source_key,
			source_type,
			medium,
			external_id,
			parent_external_id,
			display_name,
			location_label,
			notes,
			ssid,
			metadata,
			last_seen_at
		) VALUES (
			'juniper-switch-a',
			'juniper-snmp',
			'wired',
			'GE-0/0/5',
			'switch-a',
			'ge-0/0/5',
			'Desk 12',
			'Initial location',
			NULL,
			'{"vlan_id": 1}'::jsonb,
			NOW()
		)
		RETURNING id::text, updated_at
	`).Scan(&observationPointID, &observationPointUpdatedAt); err != nil {
		t.Fatalf("failed to insert observation point: %v", err)
	}

	_, err := pg.Pool.Exec(ctx, `
		INSERT INTO presence_observation_points (source_key, source_type, medium, external_id)
		VALUES ('juniper-switch-a', 'juniper-snmp', 'wired', 'ge-0/0/5')
	`)
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23505" {
		t.Fatalf("expected unique violation for duplicate observation point, got %v", err)
	}

	time.Sleep(20 * time.Millisecond)

	var observationPointUpdatedAtAfter time.Time
	if err := pg.Pool.QueryRow(ctx, `
		UPDATE presence_observation_points
		SET notes = 'Updated location note'
		WHERE id = $1::uuid
		RETURNING updated_at
	`, observationPointID).Scan(&observationPointUpdatedAtAfter); err != nil {
		t.Fatalf("failed to update observation point: %v", err)
	}
	if !observationPointUpdatedAtAfter.After(observationPointUpdatedAt) {
		t.Fatalf("expected updated_at to advance, before=%s after=%s", observationPointUpdatedAt, observationPointUpdatedAtAfter)
	}

	var clientID string
	if err := pg.Pool.QueryRow(ctx, `
		INSERT INTO presence_clients (
			mac_address,
			network_device_id,
			status,
			first_seen_at,
			last_seen_at,
			last_source_key,
			last_source_type,
			last_medium,
			last_observation_point_id,
			last_metadata
		) VALUES (
			'aa:bb:cc:dd:ee:ff',
			$1::uuid,
			'online',
			NOW() - INTERVAL '5 minutes',
			NOW(),
			'juniper-switch-a',
			'juniper-snmp',
			'wired',
			$2::uuid,
			'{"vlan_id": 1, "authenticated": true}'::jsonb
		)
		RETURNING id::text
	`, networkDeviceID, observationPointID).Scan(&clientID); err != nil {
		t.Fatalf("failed to insert presence client: %v", err)
	}

	var sessionID string
	if err := pg.Pool.QueryRow(ctx, `
		INSERT INTO presence_sessions (
			source_key,
			source_type,
			medium,
			source_session_key,
			client_mac_address,
			network_device_id,
			observation_point_id,
			started_at,
			source_updated_at,
			last_seen_at,
			metadata
		) VALUES (
			'juniper-switch-a',
			'juniper-snmp',
			'wired',
			'switch-a:ge-0/0/5:aa:bb:cc:dd:ee:ff',
			'aa:bb:cc:dd:ee:ff',
			$1::uuid,
			$2::uuid,
			NOW() - INTERVAL '5 minutes',
			NOW(),
			NOW(),
			'{"method": "mac-radius"}'::jsonb
		)
		RETURNING id::text
	`, networkDeviceID, observationPointID).Scan(&sessionID); err != nil {
		t.Fatalf("failed to insert presence session: %v", err)
	}

	if _, err := pg.Pool.Exec(ctx, `
		INSERT INTO presence_worker_state (worker_name, source_key, state, last_polled_at, last_succeeded_at)
		VALUES
			('juniper-snmp-sync', 'juniper-switch-a', '{"cursor": "42"}'::jsonb, NOW(), NOW()),
			('session-retention', '', '{"retained_days": 180}'::jsonb, NOW(), NOW())
	`); err != nil {
		t.Fatalf("failed to insert worker state rows: %v", err)
	}

	var clientCount, sessionCount, workerStateCount int
	if err := pg.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM presence_clients WHERE id = $1::uuid`, clientID).Scan(&clientCount); err != nil {
		t.Fatalf("failed to count presence clients: %v", err)
	}
	if err := pg.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM presence_sessions WHERE id = $1::uuid`, sessionID).Scan(&sessionCount); err != nil {
		t.Fatalf("failed to count presence sessions: %v", err)
	}
	if err := pg.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM presence_worker_state`).Scan(&workerStateCount); err != nil {
		t.Fatalf("failed to count presence worker state rows: %v", err)
	}

	if clientCount != 1 || sessionCount != 1 || workerStateCount != 2 {
		t.Fatalf("unexpected presence row counts: clients=%d sessions=%d worker_state=%d", clientCount, sessionCount, workerStateCount)
	}
}
