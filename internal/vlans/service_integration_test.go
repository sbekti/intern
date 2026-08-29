//go:build integration

package vlans

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

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

	service := NewService(queries, NewPGXTransactor(pg.Pool))
	created, err := service.Create(ctx, actor, api.VlanWrite{
		Name:        "lab",
		VlanId:      30,
		Description: stringPtrIntegration("Lab devices"),
	})
	if err != nil {
		t.Fatalf("expected create to succeed, got %v", err)
	}

	updated, err := service.Update(ctx, actor, created.VlanID, api.VlanPatch{
		Name:        stringPtrIntegration("lab-updated"),
		VlanId:      int32PtrIntegration(31),
		Description: stringPtrIntegration("Updated lab devices"),
	})
	if err != nil {
		t.Fatalf("expected update to succeed, got %v", err)
	}

	if err := service.Delete(ctx, actor, updated.VlanID); err != nil {
		t.Fatalf("expected delete to succeed, got %v", err)
	}

	assertVLANAuditMetadata(t, ctx, pg.Pool, "vlan.create", map[string]any{
		"after": map[string]any{
			"name":        "lab",
			"vlan_id":     float64(30),
			"description": "Lab devices",
		},
	})
	assertVLANAuditMetadata(t, ctx, pg.Pool, "vlan.update", map[string]any{
		"before": map[string]any{
			"name":        "lab",
			"vlan_id":     float64(30),
			"description": "Lab devices",
		},
		"after": map[string]any{
			"name":        "lab-updated",
			"vlan_id":     float64(31),
			"description": "Updated lab devices",
		},
	})
	assertVLANAuditMetadata(t, ctx, pg.Pool, "vlan.delete", map[string]any{
		"before": map[string]any{
			"name":        "lab-updated",
			"vlan_id":     float64(31),
			"description": "Updated lab devices",
		},
	})
}

func TestServiceCreateConflictAndAuditLog(t *testing.T) {
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

	service := NewService(queries, NewPGXTransactor(pg.Pool))
	created, err := service.Create(ctx, actor, api.VlanWrite{
		Name:        "lab",
		VlanId:      30,
		Description: stringPtrIntegration("Lab devices"),
	})
	if err != nil {
		t.Fatalf("expected create to succeed, got %v", err)
	}

	if created.Name != "lab" || created.VlanID != 30 {
		t.Fatalf("unexpected vlan %#v", created)
	}

	var auditCount int
	if err := pg.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM audit_logs WHERE action = 'vlan.create'`).Scan(&auditCount); err != nil {
		t.Fatalf("failed to count audit logs: %v", err)
	}
	if auditCount != 1 {
		t.Fatalf("expected 1 audit log, got %d", auditCount)
	}

	_, err = service.Create(ctx, actor, api.VlanWrite{
		Name:   "LAB",
		VlanId: 31,
	})
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("expected ErrConflict for duplicate vlan name, got %v", err)
	}
}

func assertVLANAuditMetadata(t *testing.T, ctx context.Context, pool db.DBTX, action string, want map[string]any) {
	t.Helper()

	var raw []byte
	if err := pool.QueryRow(ctx, `SELECT metadata FROM audit_logs WHERE action = $1 ORDER BY created_at DESC, id DESC LIMIT 1`, action).Scan(&raw); err != nil {
		t.Fatalf("failed to load vlan audit metadata for %s: %v", action, err)
	}

	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("failed to unmarshal vlan audit metadata: %v", err)
	}

	wantBytes, _ := json.Marshal(want)
	gotBytes, _ := json.Marshal(got)
	if string(wantBytes) != string(gotBytes) {
		t.Fatalf("unexpected vlan audit metadata for %s\nwant=%s\ngot=%s", action, string(wantBytes), string(gotBytes))
	}
}

func stringPtrIntegration(value string) *string {
	return &value
}

func int32PtrIntegration(value int32) *int32 {
	return &value
}
