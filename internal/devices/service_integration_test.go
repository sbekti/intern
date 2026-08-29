//go:build integration

package devices

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/sbekti/intern/internal/api"
	"github.com/sbekti/intern/internal/db"
	"github.com/sbekti/intern/internal/testutil"
)

func TestServiceCreateUpdateDeleteWritesDetailedAuditLogs(t *testing.T) {
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

	iotID := vlanNumberByName(t, ctx, pg.Pool, "iot")
	guestID := vlanNumberByName(t, ctx, pg.Pool, "guest")

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

	updated, err := service.Update(ctx, actor, deviceID(created.Device), api.NetworkDevicePatch{
		MacAddress:  stringPtr("AA-BB-CC-DD-EE-99"),
		DisplayName: stringPtr("Porch Camera"),
		VlanId:      int32Ptr(guestID),
	})
	if err != nil {
		t.Fatalf("expected update to succeed, got %v", err)
	}

	if updated.Device.DisplayName != "Porch Camera" || updated.VLAN.Name != "guest" {
		t.Fatalf("unexpected updated record %#v", updated)
	}

	if err := service.Delete(ctx, actor, deviceID(created.Device)); err != nil {
		t.Fatalf("expected delete to succeed, got %v", err)
	}

	var deviceCount int
	if err := pg.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM network_devices`).Scan(&deviceCount); err != nil {
		t.Fatalf("failed to count devices: %v", err)
	}
	if deviceCount != 0 {
		t.Fatalf("expected device row to be removed, got %d", deviceCount)
	}

	assertAuditLogMetadata(t, ctx, pg.Pool, "device.create", map[string]any{
		"after": map[string]any{
			"id":           deviceID(created.Device).String(),
			"mac_address":  "aa:bb:cc:dd:ee:ff",
			"display_name": "Camera",
			"disabled":     false,
			"vlan_id":      float64(iotID),
		},
	})
	assertAuditLogMetadata(t, ctx, pg.Pool, "device.update", map[string]any{
		"before": map[string]any{
			"id":           deviceID(created.Device).String(),
			"mac_address":  "aa:bb:cc:dd:ee:ff",
			"display_name": "Camera",
			"disabled":     false,
			"vlan_id":      float64(iotID),
		},
		"after": map[string]any{
			"id":           deviceID(created.Device).String(),
			"mac_address":  "aa:bb:cc:dd:ee:99",
			"display_name": "Porch Camera",
			"disabled":     false,
			"vlan_id":      float64(guestID),
		},
		"old_mac_address": "aa:bb:cc:dd:ee:ff",
		"new_mac_address": "aa:bb:cc:dd:ee:99",
	})
	assertAuditLogMetadata(t, ctx, pg.Pool, "device.delete", map[string]any{
		"before": map[string]any{
			"id":           deviceID(created.Device).String(),
			"mac_address":  "aa:bb:cc:dd:ee:99",
			"display_name": "Porch Camera",
			"disabled":     false,
			"vlan_id":      float64(guestID),
		},
	})
}

func TestServiceDisableAndEnableUsesLatestValues(t *testing.T) {
	t.Parallel()

	pg := testutil.StartPostgres(t)
	ctx := context.Background()
	queries := db.New(pg.Pool)
	actor := createActor(t, ctx, queries)
	iotID := vlanNumberByName(t, ctx, pg.Pool, "iot")
	guestID := vlanNumberByName(t, ctx, pg.Pool, "guest")

	service := NewService(queries, NewPGXTransactor(pg.Pool))
	created, err := service.Create(ctx, actor, api.NetworkDeviceWrite{
		MacAddress:  "AA-BB-CC-DD-EE-55",
		DisplayName: "Camera",
		VlanId:      iotID,
	})
	if err != nil {
		t.Fatalf("failed to create device: %v", err)
	}

	disabled, err := service.Update(ctx, actor, deviceID(created.Device), api.NetworkDevicePatch{
		Disabled: boolPtr(true),
	})
	if err != nil {
		t.Fatalf("failed to disable device: %v", err)
	}
	if !disabled.Device.Disabled {
		t.Fatal("expected disabled device state")
	}
	updatedWhileDisabled, err := service.Update(ctx, actor, deviceID(created.Device), api.NetworkDevicePatch{
		MacAddress: stringPtr("AA-BB-CC-DD-EE-66"),
		VlanId:     int32Ptr(guestID),
	})
	if err != nil {
		t.Fatalf("failed to update disabled device: %v", err)
	}
	if !updatedWhileDisabled.Device.Disabled {
		t.Fatal("expected device to remain disabled")
	}
	enabled, err := service.Update(ctx, actor, deviceID(created.Device), api.NetworkDevicePatch{
		Disabled: boolPtr(false),
	})
	if err != nil {
		t.Fatalf("failed to enable device: %v", err)
	}
	if enabled.Device.Disabled {
		t.Fatal("expected enabled device state")
	}
	assertAuditLogMetadata(t, ctx, pg.Pool, "device.disable", map[string]any{
		"before": map[string]any{
			"id":           deviceID(created.Device).String(),
			"mac_address":  "aa:bb:cc:dd:ee:55",
			"display_name": "Camera",
			"disabled":     false,
			"vlan_id":      float64(iotID),
		},
		"after": map[string]any{
			"id":           deviceID(created.Device).String(),
			"mac_address":  "aa:bb:cc:dd:ee:55",
			"display_name": "Camera",
			"disabled":     true,
			"vlan_id":      float64(iotID),
		},
	})
	assertAuditLogMetadata(t, ctx, pg.Pool, "device.enable", map[string]any{
		"before": map[string]any{
			"id":           deviceID(created.Device).String(),
			"mac_address":  "aa:bb:cc:dd:ee:66",
			"display_name": "Camera",
			"disabled":     true,
			"vlan_id":      float64(guestID),
		},
		"after": map[string]any{
			"id":           deviceID(created.Device).String(),
			"mac_address":  "aa:bb:cc:dd:ee:66",
			"display_name": "Camera",
			"disabled":     false,
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
	iotID := vlanNumberByName(t, ctx, pg.Pool, "iot")
	guestID := vlanNumberByName(t, ctx, pg.Pool, "guest")

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

func TestServiceUpdateRejectsMissingVLANWithoutPartialChanges(t *testing.T) {
	t.Parallel()

	pg := testutil.StartPostgres(t)
	ctx := context.Background()
	queries := db.New(pg.Pool)
	actor := createActor(t, ctx, queries)
	iotID := vlanNumberByName(t, ctx, pg.Pool, "iot")

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
		VlanId: int32Ptr(999),
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
	iotID := vlanNumberByName(t, ctx, pg.Pool, "iot")
	guestID := vlanNumberByName(t, ctx, pg.Pool, "guest")

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
	iotID := vlanNumberByName(t, ctx, pg.Pool, "iot")

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
	if _, err := pg.Pool.Exec(ctx, `DELETE FROM vlans WHERE vlan_id = $1`, iotID); err != nil {
		t.Fatalf("failed to corrupt vlan reference: %v", err)
	}
	if _, err := pg.Pool.Exec(ctx, `SET session_replication_role = DEFAULT`); err != nil {
		t.Fatalf("failed to restore constraints: %v", err)
	}

	if err := service.Delete(ctx, actor, deviceID(created.Device)); err != nil {
		t.Fatalf("expected delete to succeed with missing referenced vlan, got %v", err)
	}

	var deviceCount int
	if err := pg.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM network_devices WHERE id = $1`, created.Device.ID).Scan(&deviceCount); err != nil {
		t.Fatalf("failed to count device rows: %v", err)
	}
	if deviceCount != 0 {
		t.Fatalf("expected delete to remove device despite missing vlan, got %d rows", deviceCount)
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

func vlanNumberByName(t *testing.T, ctx context.Context, pool db.DBTX, name string) int32 {
	t.Helper()

	var vlanID int32
	if err := pool.QueryRow(ctx, `SELECT vlan_id FROM vlans WHERE name = $1`, name).Scan(&vlanID); err != nil {
		t.Fatalf("failed to look up vlan %q: %v", name, err)
	}
	return vlanID
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

func int32Ptr(value int32) *int32 {
	return &value
}

func boolPtr(value bool) *bool {
	return &value
}

func deviceID(value db.NetworkDevice) uuid.UUID {
	return uuid.UUID(value.ID.Bytes)
}
