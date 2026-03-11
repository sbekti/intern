//go:build integration

package devices

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/sbekti/intern-api/internal/api"
	"github.com/sbekti/intern-api/internal/db"
	"github.com/sbekti/intern-api/internal/testutil"
)

func TestServiceSynchronizesRadiusTablesAndWritesDetailedAuditLogs(t *testing.T) {
	t.Parallel()

	pg := testutil.StartPostgres(t)
	ctx := context.Background()
	queries := db.New(pg.Pool)

	actor, err := queries.UpsertUserByUsername(ctx, db.UpsertUserByUsernameParams{
		Username: "alice",
		Name:     "Alice Example",
		Email:    "alice@example.com",
		Groups:   []string{"Super-Users"},
	})
	if err != nil {
		t.Fatalf("failed to create actor: %v", err)
	}

	iotID := vlanIDByName(t, ctx, pg.Pool, "iot")
	guestID := vlanIDByName(t, ctx, pg.Pool, "guest")

	service := NewService(queries, NewPGXTransactor(pg.Pool))
	created, err := service.Create(ctx, actor, api.NetworkDeviceWrite{
		MacAddress:  "AA-BB-CC-DD-EE-FF",
		DisplayName: "Camera",
		VlanId:      iotID,
	})
	if err != nil {
		t.Fatalf("expected create to succeed, got %v", err)
	}

	if created.Device.MacAddress != "aa:bb:cc:dd:ee:ff" {
		t.Fatalf("expected normalized mac, got %q", created.Device.MacAddress)
	}

	assertRadiusState(t, ctx, pg.Pool, "aabbccddeeff", "iot")

	updated, err := service.Update(ctx, actor, deviceID(created.Device), api.NetworkDevicePatch{
		MacAddress:  stringPtr("AA-BB-CC-DD-EE-99"),
		DisplayName: stringPtr("Porch Camera"),
		VlanId:      int64Ptr(guestID),
	})
	if err != nil {
		t.Fatalf("expected update to succeed, got %v", err)
	}

	if updated.Device.DisplayName != "Porch Camera" || updated.VLAN.Name != "guest" {
		t.Fatalf("unexpected updated record %#v", updated)
	}

	assertRadiusState(t, ctx, pg.Pool, "aabbccddee99", "guest")
	assertRadiusRowsAbsent(t, ctx, pg.Pool, "aabbccddeeff")

	if err := service.Delete(ctx, actor, deviceID(created.Device)); err != nil {
		t.Fatalf("expected delete to succeed, got %v", err)
	}

	var deviceCount, radcheckCount, groupCount int
	if err := pg.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM network_devices`).Scan(&deviceCount); err != nil {
		t.Fatalf("failed to count devices: %v", err)
	}
	if err := pg.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM radcheck WHERE username = 'aabbccddee99'`).Scan(&radcheckCount); err != nil {
		t.Fatalf("failed to count radcheck rows: %v", err)
	}
	if err := pg.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM radusergroup WHERE username = 'aabbccddee99'`).Scan(&groupCount); err != nil {
		t.Fatalf("failed to count radusergroup rows: %v", err)
	}

	if deviceCount != 0 || radcheckCount != 0 || groupCount != 0 {
		t.Fatalf("expected device and radius rows to be removed, got devices=%d radcheck=%d radusergroup=%d", deviceCount, radcheckCount, groupCount)
	}

	assertAuditLogMetadata(t, ctx, pg.Pool, "device.create", map[string]any{
		"after": map[string]any{
			"id":           deviceID(created.Device).String(),
			"mac_address":  "aa:bb:cc:dd:ee:ff",
			"display_name": "Camera",
			"vlan_id":      float64(iotID),
			"radius_group": "iot",
		},
	})
	assertAuditLogMetadata(t, ctx, pg.Pool, "device.update", map[string]any{
		"before": map[string]any{
			"id":           deviceID(created.Device).String(),
			"mac_address":  "aa:bb:cc:dd:ee:ff",
			"display_name": "Camera",
			"vlan_id":      float64(iotID),
		},
		"after": map[string]any{
			"id":           deviceID(created.Device).String(),
			"mac_address":  "aa:bb:cc:dd:ee:99",
			"display_name": "Porch Camera",
			"vlan_id":      float64(guestID),
			"radius_group": "guest",
		},
		"old_mac_address": "aa:bb:cc:dd:ee:ff",
		"new_mac_address": "aa:bb:cc:dd:ee:99",
	})
	assertAuditLogMetadata(t, ctx, pg.Pool, "device.delete", map[string]any{
		"before": map[string]any{
			"id":           deviceID(created.Device).String(),
			"mac_address":  "aa:bb:cc:dd:ee:99",
			"display_name": "Porch Camera",
			"vlan_id":      float64(guestID),
		},
	})
}

func TestServiceCreateRejectsDuplicateNormalizedMAC(t *testing.T) {
	t.Parallel()

	pg := testutil.StartPostgres(t)
	ctx := context.Background()
	queries := db.New(pg.Pool)
	actor := createActor(t, ctx, queries)
	iotID := vlanIDByName(t, ctx, pg.Pool, "iot")
	guestID := vlanIDByName(t, ctx, pg.Pool, "guest")

	service := NewService(queries, NewPGXTransactor(pg.Pool))
	_, err := service.Create(ctx, actor, api.NetworkDeviceWrite{
		MacAddress:  "AA-BB-CC-DD-EE-FF",
		DisplayName: "Camera",
		VlanId:      iotID,
	})
	if err != nil {
		t.Fatalf("failed to create first device: %v", err)
	}

	_, err = service.Create(ctx, actor, api.NetworkDeviceWrite{
		MacAddress:  "aabb.ccdd.eeff",
		DisplayName: "Other Camera",
		VlanId:      guestID,
	})
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("expected ErrConflict for duplicate normalized MAC, got %v", err)
	}

	var deviceCount int
	if err := pg.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM network_devices`).Scan(&deviceCount); err != nil {
		t.Fatalf("failed to count devices: %v", err)
	}
	if deviceCount != 1 {
		t.Fatalf("expected only one device row after duplicate conflict, got %d", deviceCount)
	}
}

func TestServiceUpdateRejectsMissingVLANWithoutPartialRadiusChanges(t *testing.T) {
	t.Parallel()

	pg := testutil.StartPostgres(t)
	ctx := context.Background()
	queries := db.New(pg.Pool)
	actor := createActor(t, ctx, queries)
	iotID := vlanIDByName(t, ctx, pg.Pool, "iot")

	service := NewService(queries, NewPGXTransactor(pg.Pool))
	created, err := service.Create(ctx, actor, api.NetworkDeviceWrite{
		MacAddress:  "AA-BB-CC-DD-EE-11",
		DisplayName: "Sensor",
		VlanId:      iotID,
	})
	if err != nil {
		t.Fatalf("failed to create device: %v", err)
	}

	_, err = service.Update(ctx, actor, deviceID(created.Device), api.NetworkDevicePatch{
		VlanId: int64Ptr(999999),
	})
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("expected ErrConflict for missing target vlan, got %v", err)
	}

	record, err := service.Get(ctx, deviceID(created.Device))
	if err != nil {
		t.Fatalf("failed to reload device: %v", err)
	}
	if record.Device.DisplayName != "Sensor" || record.VLAN.Name != "iot" {
		t.Fatalf("expected device state to remain unchanged, got %#v", record)
	}

	assertRadiusState(t, ctx, pg.Pool, "aabbccddee11", "iot")

	var auditCount int
	if err := pg.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM audit_logs WHERE action = 'device.update'`).Scan(&auditCount); err != nil {
		t.Fatalf("failed to count device.update audit logs: %v", err)
	}
	if auditCount != 0 {
		t.Fatalf("expected no device.update audit log on failed update, got %d", auditCount)
	}
}

func TestServiceUpdateRejectsDuplicateNormalizedMACWithoutPartialChanges(t *testing.T) {
	t.Parallel()

	pg := testutil.StartPostgres(t)
	ctx := context.Background()
	queries := db.New(pg.Pool)
	actor := createActor(t, ctx, queries)
	iotID := vlanIDByName(t, ctx, pg.Pool, "iot")
	guestID := vlanIDByName(t, ctx, pg.Pool, "guest")

	service := NewService(queries, NewPGXTransactor(pg.Pool))
	_, err := service.Create(ctx, actor, api.NetworkDeviceWrite{
		MacAddress:  "AA-BB-CC-DD-EE-31",
		DisplayName: "Sensor",
		VlanId:      iotID,
	})
	if err != nil {
		t.Fatalf("failed to create first device: %v", err)
	}

	second, err := service.Create(ctx, actor, api.NetworkDeviceWrite{
		MacAddress:  "AA-BB-CC-DD-EE-32",
		DisplayName: "Tablet",
		VlanId:      guestID,
	})
	if err != nil {
		t.Fatalf("failed to create second device: %v", err)
	}

	_, err = service.Update(ctx, actor, deviceID(second.Device), api.NetworkDevicePatch{
		MacAddress: stringPtr("aabb.ccdd.ee31"),
	})
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("expected ErrConflict for duplicate normalized MAC, got %v", err)
	}

	record, err := service.Get(ctx, deviceID(second.Device))
	if err != nil {
		t.Fatalf("failed to reload second device: %v", err)
	}
	if record.Device.MacAddress != "aa:bb:cc:dd:ee:32" || record.VLAN.Name != "guest" {
		t.Fatalf("expected second device to remain unchanged, got %#v", record)
	}

	assertRadiusState(t, ctx, pg.Pool, "aabbccddee31", "iot")
	assertRadiusState(t, ctx, pg.Pool, "aabbccddee32", "guest")

	var auditCount int
	if err := pg.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM audit_logs WHERE action = 'device.update' AND resource_id = $1`, deviceID(second.Device).String()).Scan(&auditCount); err != nil {
		t.Fatalf("failed to count device.update audit logs: %v", err)
	}
	if auditCount != 0 {
		t.Fatalf("expected no device.update audit log on failed MAC update, got %d", auditCount)
	}
}

func TestServiceDeleteSucceedsWithMissingReferencedVLAN(t *testing.T) {
	t.Parallel()

	pg := testutil.StartPostgres(t)
	ctx := context.Background()
	queries := db.New(pg.Pool)
	actor := createActor(t, ctx, queries)
	iotID := vlanIDByName(t, ctx, pg.Pool, "iot")

	service := NewService(queries, NewPGXTransactor(pg.Pool))
	created, err := service.Create(ctx, actor, api.NetworkDeviceWrite{
		MacAddress:  "AA-BB-CC-DD-EE-22",
		DisplayName: "Tablet",
		VlanId:      iotID,
	})
	if err != nil {
		t.Fatalf("failed to create device: %v", err)
	}

	if _, err := pg.Pool.Exec(ctx, `SET session_replication_role = replica`); err != nil {
		t.Fatalf("failed to disable constraints: %v", err)
	}
	if _, err := pg.Pool.Exec(ctx, `DELETE FROM vlans WHERE id = $1`, iotID); err != nil {
		t.Fatalf("failed to corrupt vlan reference: %v", err)
	}
	if _, err := pg.Pool.Exec(ctx, `SET session_replication_role = DEFAULT`); err != nil {
		t.Fatalf("failed to restore constraints: %v", err)
	}

	if err := service.Delete(ctx, actor, deviceID(created.Device)); err != nil {
		t.Fatalf("expected delete to succeed with missing referenced vlan, got %v", err)
	}

	var deviceCount, radcheckCount, groupCount int
	if err := pg.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM network_devices WHERE id = $1`, created.Device.ID).Scan(&deviceCount); err != nil {
		t.Fatalf("failed to count device rows: %v", err)
	}
	if err := pg.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM radcheck WHERE username = 'aabbccddee22'`).Scan(&radcheckCount); err != nil {
		t.Fatalf("failed to count radcheck rows: %v", err)
	}
	if err := pg.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM radusergroup WHERE username = 'aabbccddee22'`).Scan(&groupCount); err != nil {
		t.Fatalf("failed to count radusergroup rows: %v", err)
	}
	if deviceCount != 0 || radcheckCount != 0 || groupCount != 0 {
		t.Fatalf("expected delete to clean up all rows despite missing vlan, got devices=%d radcheck=%d radusergroup=%d", deviceCount, radcheckCount, groupCount)
	}
}

func createActor(t *testing.T, ctx context.Context, queries *db.Queries) db.User {
	t.Helper()

	actor, err := queries.UpsertUserByUsername(ctx, db.UpsertUserByUsernameParams{
		Username: "alice",
		Name:     "Alice Example",
		Email:    "alice@example.com",
		Groups:   []string{"Super-Users"},
	})
	if err != nil {
		t.Fatalf("failed to create actor: %v", err)
	}
	return actor
}

func vlanIDByName(t *testing.T, ctx context.Context, pool db.DBTX, name string) int64 {
	t.Helper()

	var id int64
	if err := pool.QueryRow(ctx, `SELECT id FROM vlans WHERE name = $1`, name).Scan(&id); err != nil {
		t.Fatalf("failed to look up vlan %q: %v", name, err)
	}
	return id
}

func assertRadiusState(t *testing.T, ctx context.Context, pool db.DBTX, username, groupName string) {
	t.Helper()

	var value, actualGroup string
	if err := pool.QueryRow(ctx, `SELECT value FROM radcheck WHERE username = $1 AND attribute = 'Cleartext-Password'`, username).Scan(&value); err != nil {
		t.Fatalf("failed to load radcheck row: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT groupname FROM radusergroup WHERE username = $1`, username).Scan(&actualGroup); err != nil {
		t.Fatalf("failed to load radusergroup row: %v", err)
	}

	if value != username {
		t.Fatalf("expected radcheck value %q, got %q", username, value)
	}
	if actualGroup != groupName {
		t.Fatalf("expected radusergroup %q, got %q", groupName, actualGroup)
	}
}

func assertRadiusRowsAbsent(t *testing.T, ctx context.Context, pool db.DBTX, username string) {
	t.Helper()

	var radcheckCount, groupCount int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM radcheck WHERE username = $1`, username).Scan(&radcheckCount); err != nil {
		t.Fatalf("failed to count radcheck rows for %q: %v", username, err)
	}
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM radusergroup WHERE username = $1`, username).Scan(&groupCount); err != nil {
		t.Fatalf("failed to count radusergroup rows for %q: %v", username, err)
	}

	if radcheckCount != 0 || groupCount != 0 {
		t.Fatalf("expected no radius rows for %q, got radcheck=%d radusergroup=%d", username, radcheckCount, groupCount)
	}
}

func assertAuditLogMetadata(t *testing.T, ctx context.Context, pool db.DBTX, action string, want map[string]any) {
	t.Helper()

	var raw []byte
	if err := pool.QueryRow(ctx, `SELECT metadata FROM audit_logs WHERE action = $1 ORDER BY created_at DESC, id DESC LIMIT 1`, action).Scan(&raw); err != nil {
		t.Fatalf("failed to load audit metadata for %s: %v", action, err)
	}

	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("failed to unmarshal audit metadata: %v", err)
	}

	if diff := compareJSONMaps(want, got); diff != "" {
		t.Fatalf("unexpected audit metadata for %s: %s\ngot=%s", action, diff, string(raw))
	}
}

func compareJSONMaps(want, got map[string]any) string {
	wantBytes, _ := json.Marshal(want)
	gotBytes, _ := json.Marshal(got)
	if string(wantBytes) == string(gotBytes) {
		return ""
	}
	return "mismatch"
}

func stringPtr(value string) *string {
	return &value
}

func int64Ptr(value int64) *int64 {
	return &value
}

func deviceID(value db.NetworkDevice) uuid.UUID {
	return uuid.UUID(value.ID.Bytes)
}
