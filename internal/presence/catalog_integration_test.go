//go:build integration

package presence

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/sbekti/intern-api/internal/api"
	"github.com/sbekti/intern-api/internal/db"
	"github.com/sbekti/intern-api/internal/testutil"
)

func TestCatalogServiceListsManagedAndObservedPresence(t *testing.T) {
	t.Parallel()

	pg := testutil.StartPostgres(t)
	ctx := context.Background()
	queries := db.New(pg.Pool)
	service := NewCatalogService(queries, pg.Pool)

	actor, err := queries.UpsertUserByUsername(ctx, db.UpsertUserByUsernameParams{
		Username: "test-admin",
		Name:     "Test Admin",
		Email:    "test-admin@example.com",
		Groups:   []string{"Super-Users"},
	})
	if err != nil {
		t.Fatalf("failed to create actor: %v", err)
	}

	deviceID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	if _, err := pg.Pool.Exec(ctx, `
		INSERT INTO network_devices (
			id,
			mac_address,
			display_name,
			vlan_id,
			created_by_user_id,
			updated_by_user_id
		) VALUES ($1, 'aa:bb:cc:dd:ee:ff', 'Managed Handset', 1, $2, $2)
	`, deviceID, actor.ID); err != nil {
		t.Fatalf("failed to create network device: %v", err)
	}

	observationPointID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	lastSeenAt := time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC)
	if _, err := pg.Pool.Exec(ctx, `
		INSERT INTO presence_observation_points (
			id,
			source_key,
			source_type,
			medium,
			external_id,
			display_name,
			location_label,
			ssid,
			last_seen_at
		) VALUES ($1, 'unifi-site-a', 'unifi', 'wireless', 'aa:bb:cc:dd:ee:00', 'AP Lobby', 'Front lobby', 'corp-wifi', $2)
	`, observationPointID, lastSeenAt); err != nil {
		t.Fatalf("failed to create observation point: %v", err)
	}

	clientID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	if _, err := pg.Pool.Exec(ctx, `
		INSERT INTO presence_clients (
			id,
			mac_address,
			network_device_id,
			status,
			first_seen_at,
			last_seen_at,
			last_source_key,
			last_source_type,
			last_medium,
			last_observation_point_id,
			last_ssid
		) VALUES ($1, 'aa:bb:cc:dd:ee:ff', $2, 'online', $3::timestamptz - interval '1 hour', $3, 'unifi-site-a', 'unifi', 'wireless', $4, 'corp-wifi')
	`, clientID, deviceID, lastSeenAt, observationPointID); err != nil {
		t.Fatalf("failed to create presence client: %v", err)
	}

	managed, err := service.ListManagedPresence(ctx)
	if err != nil {
		t.Fatalf("expected managed presence list to succeed, got %v", err)
	}

	managedSummary, ok := managed[deviceID]
	if !ok {
		t.Fatalf("expected managed presence summary for %s", deviceID)
	}
	if managedSummary.LocationLabel != "Front lobby" || managedSummary.SSID != "corp-wifi" {
		t.Fatalf("unexpected managed presence summary: %#v", managedSummary)
	}

	page, err := service.ListObservedClients(ctx, ObservedClientFilter{
		Status: "online",
		Limit:  25,
		Offset: 0,
	})
	if err != nil {
		t.Fatalf("expected observed presence list to succeed, got %v", err)
	}
	if page.Pagination.Total != 1 || len(page.Items) != 1 {
		t.Fatalf("expected one observed client, got %#v", page)
	}
	if page.Items[0].ManagedDeviceName != "Managed Handset" {
		t.Fatalf("expected managed device name, got %#v", page.Items[0])
	}
}

func TestCatalogServiceUpdatesObservationPointAndWritesAuditLog(t *testing.T) {
	t.Parallel()

	pg := testutil.StartPostgres(t)
	ctx := context.Background()
	queries := db.New(pg.Pool)
	service := NewCatalogService(queries, pg.Pool)

	actor, err := queries.UpsertUserByUsername(ctx, db.UpsertUserByUsernameParams{
		Username: "test-admin",
		Name:     "Test Admin",
		Email:    "test-admin@example.com",
		Groups:   []string{"Super-Users"},
	})
	if err != nil {
		t.Fatalf("failed to create actor: %v", err)
	}

	observationPointID := uuid.MustParse("44444444-4444-4444-4444-444444444444")
	if _, err := pg.Pool.Exec(ctx, `
		INSERT INTO presence_observation_points (
			id,
			source_key,
			source_type,
			medium,
			external_id,
			display_name,
			location_label,
			notes
		) VALUES ($1, 'juniper-switch-a', 'juniper-snmp', 'wired', 'ge-0/0/5', 'Port ge-0/0/5', '', '')
	`, observationPointID); err != nil {
		t.Fatalf("failed to create observation point: %v", err)
	}

	locationLabel := "Desk cluster"
	notes := "Primary seating row"
	record, err := service.UpdateObservationPoint(ctx, actor, observationPointID, api.PresenceObservationPointPatch{
		LocationLabel: &locationLabel,
		Notes:         &notes,
	})
	if err != nil {
		t.Fatalf("expected update to succeed, got %v", err)
	}
	if record.LocationLabel != locationLabel || record.Notes != notes {
		t.Fatalf("unexpected updated observation point: %#v", record)
	}

	var action, resourceType string
	var resourceID string
	if err := pg.Pool.QueryRow(ctx, `
		SELECT action, resource_type, resource_id
		FROM audit_logs
		ORDER BY created_at DESC
		LIMIT 1
	`).Scan(&action, &resourceType, &resourceID); err != nil {
		t.Fatalf("failed to load audit log: %v", err)
	}
	if action != "presence.observation_point.update" || resourceType != "presence_observation_point" || resourceID != observationPointID.String() {
		t.Fatalf("unexpected audit log action=%q resource_type=%q resource_id=%q", action, resourceType, resourceID)
	}
}
