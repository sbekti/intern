//go:build integration

package gizmos

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/sbekti/intern/internal/api"
	"github.com/sbekti/intern/internal/db"
	"github.com/sbekti/intern/internal/testutil"
)

func TestGizmoLifecycleAndKioskLookup(t *testing.T) {
	t.Parallel()

	pg := testutil.StartPostgres(t)
	ctx := context.Background()
	queries := db.New(pg.Pool)
	actor, err := queries.UpsertUserByUsername(ctx, db.UpsertUserByUsernameParams{
		Username: "alice", Name: "Alice", Email: "alice@example.test", Groups: []string{"Super-Users"},
	})
	if err != nil {
		t.Fatal(err)
	}

	deviceID := uuid.New()
	if _, err := pg.Pool.Exec(ctx, `INSERT INTO network_devices (id, mac_address, display_name, vlan_id) VALUES ($1, $2, $3, $4)`, deviceID, "b4:a9:fc:11:7e:4a", "Kitchen Gizmo", 20); err != nil {
		t.Fatal(err)
	}

	service := NewService(queries, NewPGXTransactor(pg.Pool))
	created, err := service.Create(ctx, actor, api.GizmoWrite{NetworkDeviceId: deviceID})
	if err != nil || created.Gizmo.KioskUrl != nil {
		t.Fatalf("create = %#v, %v", created, err)
	}
	if _, err := service.ResolveKioskURL(ctx, "B4-A9-FC-11-7E-4A"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unconfigured lookup error = %v", err)
	}

	url := " https://example.test/dashboard "
	updated, err := service.Update(ctx, actor, deviceID, api.GizmoUpdate{KioskUrl: &url})
	if err != nil || updated.Gizmo.KioskUrl == nil || *updated.Gizmo.KioskUrl != "https://example.test/dashboard" {
		t.Fatalf("update = %#v, %v", updated, err)
	}
	if got, err := service.ResolveKioskURL(ctx, "b4a9.fc11.7e4a"); err != nil || got != "https://example.test/dashboard" {
		t.Fatalf("lookup = %q, %v", got, err)
	}

	if _, err := pg.Pool.Exec(ctx, `UPDATE network_devices SET disabled = true WHERE id = $1`, deviceID); err != nil {
		t.Fatal(err)
	}
	if _, err := service.ResolveKioskURL(ctx, "b4:a9:fc:11:7e:4a"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("disabled lookup error = %v", err)
	}
	if _, err := pg.Pool.Exec(ctx, `UPDATE network_devices SET disabled = false WHERE id = $1`, deviceID); err != nil {
		t.Fatal(err)
	}
	empty := " "
	cleared, err := service.Update(ctx, actor, deviceID, api.GizmoUpdate{KioskUrl: &empty})
	if err != nil || cleared.Gizmo.KioskUrl != nil {
		t.Fatalf("clear = %#v, %v", cleared, err)
	}
	if _, err := service.ResolveKioskURL(ctx, "b4:a9:fc:11:7e:4a"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cleared lookup error = %v", err)
	}

	if err := service.Delete(ctx, actor, deviceID); err != nil {
		t.Fatal(err)
	}
	if _, err := service.ResolveKioskURL(ctx, "b4:a9:fc:11:7e:4a"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing Gizmo lookup error = %v", err)
	}
	var deviceCount int
	if err := pg.Pool.QueryRow(ctx, `SELECT count(*) FROM network_devices WHERE id = $1`, deviceID).Scan(&deviceCount); err != nil || deviceCount != 1 {
		t.Fatalf("network device count = %d, %v", deviceCount, err)
	}
}
