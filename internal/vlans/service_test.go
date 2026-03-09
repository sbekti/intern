package vlans

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/sbekti/intern-api/internal/api"
	"github.com/sbekti/intern-api/internal/db"
)

type fakeQuerier struct {
	listFn      func(ctx context.Context) ([]db.Vlan, error)
	getFn       func(ctx context.Context, arg db.GetVlanByIDParams) (db.Vlan, error)
	createFn    func(ctx context.Context, arg db.CreateVlanParams) (db.Vlan, error)
	updateFn    func(ctx context.Context, arg db.UpdateVlanParams) (db.Vlan, error)
	deleteFn    func(ctx context.Context, arg db.DeleteVlanParams) error
	createLogFn func(ctx context.Context, arg db.CreateAuditLogParams) (db.AuditLog, error)
}

func (f fakeQuerier) ListVlans(ctx context.Context) ([]db.Vlan, error) {
	return f.listFn(ctx)
}

func (f fakeQuerier) GetVlanByID(ctx context.Context, arg db.GetVlanByIDParams) (db.Vlan, error) {
	return f.getFn(ctx, arg)
}

func (f fakeQuerier) CreateVlan(ctx context.Context, arg db.CreateVlanParams) (db.Vlan, error) {
	return f.createFn(ctx, arg)
}

func (f fakeQuerier) UpdateVlan(ctx context.Context, arg db.UpdateVlanParams) (db.Vlan, error) {
	return f.updateFn(ctx, arg)
}

func (f fakeQuerier) DeleteVlan(ctx context.Context, arg db.DeleteVlanParams) error {
	return f.deleteFn(ctx, arg)
}

func (f fakeQuerier) CreateAuditLog(ctx context.Context, arg db.CreateAuditLogParams) (db.AuditLog, error) {
	return f.createLogFn(ctx, arg)
}

type fakeTransactor struct {
	q Querier
}

func (f fakeTransactor) InTx(ctx context.Context, fn func(q Querier) error) error {
	return fn(f.q)
}

func TestNormalizeCreate(t *testing.T) {
	t.Parallel()

	_, err := normalizeCreate(api.VlanWrite{Name: " ", VlanId: 10})
	var validationErr ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("expected validation error, got %v", err)
	}

	params, err := normalizeCreate(api.VlanWrite{Name: " guest ", VlanId: 10})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if params.Name != "guest" {
		t.Fatalf("expected trimmed name guest, got %q", params.Name)
	}
	if !params.IsActive {
		t.Fatal("expected default is_active true")
	}
}

func TestMergePatch(t *testing.T) {
	t.Parallel()

	current := db.Vlan{
		ID:          1,
		Name:        "guest",
		VlanID:      10,
		Description: "Guest devices",
		IsActive:    true,
	}

	_, err := mergePatch(current, api.VlanPatch{})
	var validationErr ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("expected validation error, got %v", err)
	}

	updatedName := "iot"
	params, err := mergePatch(current, api.VlanPatch{Name: &updatedName})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if params.Name != "iot" || params.VlanID != 10 {
		t.Fatalf("unexpected merged params %+v", params)
	}
}

func TestClassifyDBError(t *testing.T) {
	t.Parallel()

	if !errors.Is(classifyDBError(&pgconn.PgError{Code: "23505"}), ErrConflict) {
		t.Fatal("expected unique violation to map to ErrConflict")
	}
	if !errors.Is(classifyDBError(&pgconn.PgError{Code: "23503"}), ErrConflict) {
		t.Fatal("expected foreign key violation to map to ErrConflict")
	}
}

func TestServiceCreateWritesAuditLog(t *testing.T) {
	t.Parallel()

	createCalled := false
	logCalled := false
	service := NewService(nil, fakeTransactor{
		q: fakeQuerier{
			createFn: func(ctx context.Context, arg db.CreateVlanParams) (db.Vlan, error) {
				createCalled = true
				return db.Vlan{ID: 1, Name: arg.Name, VlanID: arg.VlanID, Description: arg.Description, IsActive: arg.IsActive}, nil
			},
			createLogFn: func(ctx context.Context, arg db.CreateAuditLogParams) (db.AuditLog, error) {
				logCalled = true
				if arg.Action != "vlan.create" {
					t.Fatalf("expected vlan.create, got %q", arg.Action)
				}
				return db.AuditLog{}, nil
			},
		},
	})

	created, err := service.Create(context.Background(), db.User{Username: "alice"}, api.VlanWrite{Name: "guest", VlanId: 10})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if created.Name != "guest" {
		t.Fatalf("expected guest, got %q", created.Name)
	}
	if !createCalled || !logCalled {
		t.Fatal("expected create and audit log to be called")
	}
}

func TestServiceUpdateReturnsNotFound(t *testing.T) {
	t.Parallel()

	service := NewService(nil, fakeTransactor{
		q: fakeQuerier{
			getFn: func(ctx context.Context, arg db.GetVlanByIDParams) (db.Vlan, error) {
				return db.Vlan{}, pgx.ErrNoRows
			},
			updateFn: func(ctx context.Context, arg db.UpdateVlanParams) (db.Vlan, error) {
				t.Fatal("expected update not to be called")
				return db.Vlan{}, nil
			},
			createLogFn: func(ctx context.Context, arg db.CreateAuditLogParams) (db.AuditLog, error) {
				t.Fatal("expected audit log not to be called")
				return db.AuditLog{}, nil
			},
		},
	})

	_, err := service.Update(context.Background(), db.User{Username: "alice"}, 1, api.VlanPatch{Name: stringPtr("iot")})
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestServiceDeleteTreatsMissingAsSuccess(t *testing.T) {
	t.Parallel()

	service := NewService(nil, fakeTransactor{
		q: fakeQuerier{
			getFn: func(ctx context.Context, arg db.GetVlanByIDParams) (db.Vlan, error) {
				return db.Vlan{}, pgx.ErrNoRows
			},
			deleteFn: func(ctx context.Context, arg db.DeleteVlanParams) error {
				t.Fatal("expected delete not to be called")
				return nil
			},
			createLogFn: func(ctx context.Context, arg db.CreateAuditLogParams) (db.AuditLog, error) {
				t.Fatal("expected audit log not to be called")
				return db.AuditLog{}, nil
			},
		},
	})

	if err := service.Delete(context.Background(), db.User{Username: "alice"}, 1); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func stringPtr(value string) *string {
	return &value
}
