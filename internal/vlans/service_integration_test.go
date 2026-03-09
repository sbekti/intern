//go:build integration

package vlans

import (
	"context"
	"errors"
	"testing"

	"github.com/sbekti/intern-api/internal/api"
	"github.com/sbekti/intern-api/internal/db"
	"github.com/sbekti/intern-api/internal/testutil"
)

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

func stringPtrIntegration(value string) *string {
	return &value
}
