//go:build integration

package devices

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/sbekti/intern-api/internal/api"
	"github.com/sbekti/intern-api/internal/db"
	"github.com/sbekti/intern-api/internal/testutil"
)

func TestServiceSynchronizesRadiusTables(t *testing.T) {
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
		DisplayName: stringPtr("Porch Camera"),
		VlanId:      int64Ptr(guestID),
	})
	if err != nil {
		t.Fatalf("expected update to succeed, got %v", err)
	}

	if updated.Device.DisplayName != "Porch Camera" || updated.VLAN.Name != "guest" {
		t.Fatalf("unexpected updated record %#v", updated)
	}

	assertRadiusState(t, ctx, pg.Pool, "aabbccddeeff", "guest")

	if err := service.Delete(ctx, actor, deviceID(created.Device)); err != nil {
		t.Fatalf("expected delete to succeed, got %v", err)
	}

	var deviceCount, radcheckCount, groupCount, auditCount int
	if err := pg.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM network_devices`).Scan(&deviceCount); err != nil {
		t.Fatalf("failed to count devices: %v", err)
	}
	if err := pg.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM radcheck WHERE username = 'aabbccddeeff'`).Scan(&radcheckCount); err != nil {
		t.Fatalf("failed to count radcheck rows: %v", err)
	}
	if err := pg.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM radusergroup WHERE username = 'aabbccddeeff'`).Scan(&groupCount); err != nil {
		t.Fatalf("failed to count radusergroup rows: %v", err)
	}
	if err := pg.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM audit_logs WHERE resource_type = 'network_device'`).Scan(&auditCount); err != nil {
		t.Fatalf("failed to count audit logs: %v", err)
	}

	if deviceCount != 0 || radcheckCount != 0 || groupCount != 0 {
		t.Fatalf("expected device and radius rows to be removed, got devices=%d radcheck=%d radusergroup=%d", deviceCount, radcheckCount, groupCount)
	}
	if auditCount != 3 {
		t.Fatalf("expected create/update/delete audit logs, got %d", auditCount)
	}
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

func stringPtr(value string) *string {
	return &value
}

func int64Ptr(value int64) *int64 {
	return &value
}

func deviceID(value db.NetworkDevice) uuid.UUID {
	return uuid.UUID(value.ID.Bytes)
}
