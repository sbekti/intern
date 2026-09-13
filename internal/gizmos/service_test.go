package gizmos

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/sbekti/intern/internal/db"
)

type lookupQuerier struct {
	lookup func(context.Context, db.GetKioskURLByMACParams) (*string, error)
}

func (q lookupQuerier) GetKioskURLByMAC(ctx context.Context, arg db.GetKioskURLByMACParams) (*string, error) {
	return q.lookup(ctx, arg)
}
func (lookupQuerier) ListGizmos(context.Context) ([]db.ListGizmosRow, error) { return nil, nil }
func (lookupQuerier) GetGizmoByNetworkDeviceID(context.Context, db.GetGizmoByNetworkDeviceIDParams) (db.GetGizmoByNetworkDeviceIDRow, error) {
	return db.GetGizmoByNetworkDeviceIDRow{}, nil
}
func (lookupQuerier) CreateGizmo(context.Context, db.CreateGizmoParams) (db.Gizmo, error) {
	return db.Gizmo{}, nil
}
func (lookupQuerier) UpdateGizmo(context.Context, db.UpdateGizmoParams) (db.Gizmo, error) {
	return db.Gizmo{}, nil
}
func (lookupQuerier) DeleteGizmo(context.Context, db.DeleteGizmoParams) error { return nil }
func (lookupQuerier) CreateAuditLog(context.Context, db.CreateAuditLogParams) (db.AuditLog, error) {
	return db.AuditLog{}, nil
}

func TestNormalizeKioskURL(t *testing.T) {
	t.Parallel()

	for _, raw := range []string{" http://example.test/path ", "https://example.test/path"} {
		got, err := normalizeKioskURL(&raw)
		if err != nil || got == nil || *got != strings.TrimSpace(raw) {
			t.Fatalf("normalizeKioskURL(%q) = %v, %v", raw, got, err)
		}
	}

	empty := "  "
	if got, err := normalizeKioskURL(&empty); err != nil || got != nil {
		t.Fatalf("blank URL = %v, %v; want nil", got, err)
	}

	limit := strings.Repeat("a", 2048)
	if got, err := normalizeKioskURL(&limit); err != nil || got == nil {
		t.Fatalf("2048-character URL rejected: %v", err)
	}
	overLimit := limit + "a"
	if _, err := normalizeKioskURL(&overLimit); err == nil {
		t.Fatal("2049-character URL was accepted")
	}
}

func TestResolveKioskURLNormalizesMACAndMapsMissing(t *testing.T) {
	t.Parallel()

	want := "https://example.test/dashboard"
	service := NewService(lookupQuerier{lookup: func(_ context.Context, arg db.GetKioskURLByMACParams) (*string, error) {
		if arg.MacAddress != "b4:a9:fc:11:7e:4a" {
			t.Fatalf("MAC = %q", arg.MacAddress)
		}
		return &want, nil
	}}, nil)
	got, err := service.ResolveKioskURL(context.Background(), "B4-A9-FC-11-7E-4A")
	if err != nil || got != want {
		t.Fatalf("ResolveKioskURL = %q, %v", got, err)
	}

	missing := NewService(lookupQuerier{lookup: func(context.Context, db.GetKioskURLByMACParams) (*string, error) {
		return nil, pgx.ErrNoRows
	}}, nil)
	if _, err := missing.ResolveKioskURL(context.Background(), "b4a9fc117e4a"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing lookup error = %v", err)
	}
	if _, err := service.ResolveKioskURL(context.Background(), "bad"); err == nil {
		t.Fatal("malformed MAC was accepted")
	}
}
