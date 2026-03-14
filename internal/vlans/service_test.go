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
	listFn             func(ctx context.Context) ([]db.Vlan, error)
	getFn              func(ctx context.Context, arg db.GetVlanByVlanIDParams) (db.Vlan, error)
	createFn           func(ctx context.Context, arg db.CreateVlanParams) (db.Vlan, error)
	updateFn           func(ctx context.Context, arg db.UpdateVlanParams) (db.Vlan, error)
	deleteFn           func(ctx context.Context, arg db.DeleteVlanParams) error
	deleteGroupFn      func(ctx context.Context, arg db.DeleteRadgrouprepliesByGroupnameParams) error
	insertGroupFn      func(ctx context.Context, arg db.InsertRadgroupreplyParams) error
	updateUsergroupsFn func(ctx context.Context, arg db.UpdateRadusergroupsGroupnameParams) error
	createLogFn        func(ctx context.Context, arg db.CreateAuditLogParams) (db.AuditLog, error)
}

func (f fakeQuerier) ListVlans(ctx context.Context) ([]db.Vlan, error) {
	return f.listFn(ctx)
}

func (f fakeQuerier) GetVlanByVlanID(ctx context.Context, arg db.GetVlanByVlanIDParams) (db.Vlan, error) {
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

func (f fakeQuerier) DeleteRadgrouprepliesByGroupname(ctx context.Context, arg db.DeleteRadgrouprepliesByGroupnameParams) error {
	return f.deleteGroupFn(ctx, arg)
}

func (f fakeQuerier) InsertRadgroupreply(ctx context.Context, arg db.InsertRadgroupreplyParams) error {
	return f.insertGroupFn(ctx, arg)
}

func (f fakeQuerier) UpdateRadusergroupsGroupname(ctx context.Context, arg db.UpdateRadusergroupsGroupnameParams) error {
	return f.updateUsergroupsFn(ctx, arg)
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
}

func TestMergePatch(t *testing.T) {
	t.Parallel()

	current := db.Vlan{
		Name:        "guest",
		VlanID:      10,
		Description: "Guest devices",
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
	if !errors.Is(classifyDeleteError(&pgconn.PgError{Code: "23001"}), ErrReferencedByDevices) {
		t.Fatal("expected restrict violation to map to ErrReferencedByDevices")
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
				return db.Vlan{Name: arg.Name, VlanID: arg.VlanID, Description: arg.Description}, nil
			},
			deleteGroupFn: func(ctx context.Context, arg db.DeleteRadgrouprepliesByGroupnameParams) error {
				return nil
			},
			insertGroupFn: func(ctx context.Context, arg db.InsertRadgroupreplyParams) error {
				return nil
			},
			updateUsergroupsFn: func(ctx context.Context, arg db.UpdateRadusergroupsGroupnameParams) error {
				return nil
			},
			createLogFn: func(ctx context.Context, arg db.CreateAuditLogParams) (db.AuditLog, error) {
				logCalled = true
				if arg.Action != "vlan.create" {
					t.Fatalf("expected vlan.create, got %q", arg.Action)
				}
				if arg.ResourceID != "10" {
					t.Fatalf("expected resource id 10, got %q", arg.ResourceID)
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
			getFn: func(ctx context.Context, arg db.GetVlanByVlanIDParams) (db.Vlan, error) {
				return db.Vlan{}, pgx.ErrNoRows
			},
			updateFn: func(ctx context.Context, arg db.UpdateVlanParams) (db.Vlan, error) {
				t.Fatal("expected update not to be called")
				return db.Vlan{}, nil
			},
			deleteGroupFn: func(ctx context.Context, arg db.DeleteRadgrouprepliesByGroupnameParams) error { return nil },
			insertGroupFn: func(ctx context.Context, arg db.InsertRadgroupreplyParams) error { return nil },
			updateUsergroupsFn: func(ctx context.Context, arg db.UpdateRadusergroupsGroupnameParams) error {
				return nil
			},
			createLogFn: func(ctx context.Context, arg db.CreateAuditLogParams) (db.AuditLog, error) {
				t.Fatal("expected audit log not to be called")
				return db.AuditLog{}, nil
			},
		},
	})

	_, err := service.Update(context.Background(), db.User{Username: "alice"}, 10, api.VlanPatch{Name: stringPtr("iot")})
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestServiceDeleteTreatsMissingAsSuccess(t *testing.T) {
	t.Parallel()

	service := NewService(nil, fakeTransactor{
		q: fakeQuerier{
			getFn: func(ctx context.Context, arg db.GetVlanByVlanIDParams) (db.Vlan, error) {
				return db.Vlan{}, pgx.ErrNoRows
			},
			deleteFn: func(ctx context.Context, arg db.DeleteVlanParams) error {
				t.Fatal("expected delete not to be called")
				return nil
			},
			deleteGroupFn: func(ctx context.Context, arg db.DeleteRadgrouprepliesByGroupnameParams) error { return nil },
			insertGroupFn: func(ctx context.Context, arg db.InsertRadgroupreplyParams) error { return nil },
			updateUsergroupsFn: func(ctx context.Context, arg db.UpdateRadusergroupsGroupnameParams) error {
				return nil
			},
			createLogFn: func(ctx context.Context, arg db.CreateAuditLogParams) (db.AuditLog, error) {
				t.Fatal("expected audit log not to be called")
				return db.AuditLog{}, nil
			},
		},
	})

	if err := service.Delete(context.Background(), db.User{Username: "alice"}, 10); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func stringPtr(value string) *string {
	return &value
}
