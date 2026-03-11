package devices

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/sbekti/intern-api/internal/api"
	"github.com/sbekti/intern-api/internal/db"
)

type fakeQuerier struct {
	listFn             func(ctx context.Context) ([]db.NetworkDevice, error)
	getDeviceFn        func(ctx context.Context, arg db.GetNetworkDeviceByIDParams) (db.NetworkDevice, error)
	createFn           func(ctx context.Context, arg db.CreateNetworkDeviceParams) (db.NetworkDevice, error)
	updateFn           func(ctx context.Context, arg db.UpdateNetworkDeviceParams) (db.NetworkDevice, error)
	deleteFn           func(ctx context.Context, arg db.DeleteNetworkDeviceParams) error
	getVlanFn          func(ctx context.Context, arg db.GetVlanByIDParams) (db.Vlan, error)
	upsertRadcheckFn   func(ctx context.Context, arg db.UpsertRadcheckCleartextPasswordParams) error
	deleteRadcheckFn   func(ctx context.Context, arg db.DeleteRadcheckCleartextPasswordByUsernameParams) error
	deleteUsergroupsFn func(ctx context.Context, arg db.DeleteRadusergroupsByUsernameParams) error
	insertUsergroupFn  func(ctx context.Context, arg db.InsertRadusergroupParams) error
	createAuditLogFn   func(ctx context.Context, arg db.CreateAuditLogParams) (db.AuditLog, error)
}

func (f fakeQuerier) ListNetworkDevices(ctx context.Context) ([]db.NetworkDevice, error) {
	return f.listFn(ctx)
}

func (f fakeQuerier) GetNetworkDeviceByID(ctx context.Context, arg db.GetNetworkDeviceByIDParams) (db.NetworkDevice, error) {
	return f.getDeviceFn(ctx, arg)
}

func (f fakeQuerier) CreateNetworkDevice(ctx context.Context, arg db.CreateNetworkDeviceParams) (db.NetworkDevice, error) {
	return f.createFn(ctx, arg)
}

func (f fakeQuerier) UpdateNetworkDevice(ctx context.Context, arg db.UpdateNetworkDeviceParams) (db.NetworkDevice, error) {
	return f.updateFn(ctx, arg)
}

func (f fakeQuerier) DeleteNetworkDevice(ctx context.Context, arg db.DeleteNetworkDeviceParams) error {
	return f.deleteFn(ctx, arg)
}

func (f fakeQuerier) GetVlanByID(ctx context.Context, arg db.GetVlanByIDParams) (db.Vlan, error) {
	return f.getVlanFn(ctx, arg)
}

func (f fakeQuerier) UpsertRadcheckCleartextPassword(ctx context.Context, arg db.UpsertRadcheckCleartextPasswordParams) error {
	return f.upsertRadcheckFn(ctx, arg)
}

func (f fakeQuerier) DeleteRadcheckCleartextPasswordByUsername(ctx context.Context, arg db.DeleteRadcheckCleartextPasswordByUsernameParams) error {
	return f.deleteRadcheckFn(ctx, arg)
}

func (f fakeQuerier) DeleteRadusergroupsByUsername(ctx context.Context, arg db.DeleteRadusergroupsByUsernameParams) error {
	return f.deleteUsergroupsFn(ctx, arg)
}

func (f fakeQuerier) InsertRadusergroup(ctx context.Context, arg db.InsertRadusergroupParams) error {
	return f.insertUsergroupFn(ctx, arg)
}

func (f fakeQuerier) CreateAuditLog(ctx context.Context, arg db.CreateAuditLogParams) (db.AuditLog, error) {
	return f.createAuditLogFn(ctx, arg)
}

type fakeTransactor struct {
	q Querier
}

func (f fakeTransactor) InTx(ctx context.Context, fn func(q Querier) error) error {
	return fn(f.q)
}

func TestNormalizeMAC(t *testing.T) {
	t.Parallel()

	bare, colon, err := normalizeMAC("AA-BB-CC-DD-EE-FF")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if bare != "aabbccddeeff" {
		t.Fatalf("unexpected bare mac %q", bare)
	}
	if colon != "aa:bb:cc:dd:ee:ff" {
		t.Fatalf("unexpected colon mac %q", colon)
	}

	if _, _, err := normalizeMAC("bad-mac"); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestMergePatch(t *testing.T) {
	t.Parallel()

	current := db.NetworkDevice{
		MacAddress:  "aa:bb:cc:dd:ee:ff",
		DisplayName: "Living Room TV",
		VlanID:      2,
	}

	_, err := mergePatch(current, api.NetworkDevicePatch{})
	var validationErr ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("expected validation error, got %v", err)
	}

	name := "  Updated TV  "
	vlanID := int64(3)
	macAddress := "AA-BB-CC-00-11-22"
	params, err := mergePatch(current, api.NetworkDevicePatch{
		MacAddress:  &macAddress,
		DisplayName: &name,
		VlanId:      &vlanID,
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if params.MacAddress != "aa:bb:cc:00:11:22" || params.DisplayName != "Updated TV" || params.VlanID != 3 {
		t.Fatalf("unexpected patch params %+v", params)
	}
}

func TestServiceCreateWritesRadiusState(t *testing.T) {
	t.Parallel()

	deviceID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	radcheckCalled := false
	radgroupCalled := false
	service := NewService(nil, fakeTransactor{
		q: fakeQuerier{
			getVlanFn: func(ctx context.Context, arg db.GetVlanByIDParams) (db.Vlan, error) {
				return db.Vlan{ID: 2, Name: "iot", VlanID: 20}, nil
			},
			createFn: func(ctx context.Context, arg db.CreateNetworkDeviceParams) (db.NetworkDevice, error) {
				if arg.MacAddress != "aa:bb:cc:dd:ee:ff" {
					t.Fatalf("expected normalized app mac, got %q", arg.MacAddress)
				}
				return db.NetworkDevice{
					ID:          toPgUUID(deviceID),
					MacAddress:  arg.MacAddress,
					DisplayName: arg.DisplayName,
					VlanID:      arg.VlanID,
				}, nil
			},
			upsertRadcheckFn: func(ctx context.Context, arg db.UpsertRadcheckCleartextPasswordParams) error {
				radcheckCalled = true
				if arg.Username != "aabbccddeeff" || arg.Value != "aabbccddeeff" {
					t.Fatalf("unexpected radcheck args %+v", arg)
				}
				return nil
			},
			deleteUsergroupsFn: func(ctx context.Context, arg db.DeleteRadusergroupsByUsernameParams) error {
				return nil
			},
			insertUsergroupFn: func(ctx context.Context, arg db.InsertRadusergroupParams) error {
				radgroupCalled = true
				if arg.Groupname != "iot" {
					t.Fatalf("expected iot group, got %q", arg.Groupname)
				}
				return nil
			},
			createAuditLogFn: func(ctx context.Context, arg db.CreateAuditLogParams) (db.AuditLog, error) {
				return db.AuditLog{}, nil
			},
		},
	})

	record, err := service.Create(context.Background(), db.User{
		ID:       pgtype.UUID{Valid: true},
		Username: "alice",
	}, api.NetworkDeviceWrite{
		MacAddress:  "AA-BB-CC-DD-EE-FF",
		DisplayName: "Camera",
		VlanId:      2,
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if record.Device.MacAddress != "aa:bb:cc:dd:ee:ff" {
		t.Fatalf("unexpected mac address %q", record.Device.MacAddress)
	}
	if !radcheckCalled || !radgroupCalled {
		t.Fatal("expected radius writes")
	}
}

func TestServiceGetReturnsNotFound(t *testing.T) {
	t.Parallel()

	service := NewService(fakeQuerier{
		getDeviceFn: func(ctx context.Context, arg db.GetNetworkDeviceByIDParams) (db.NetworkDevice, error) {
			return db.NetworkDevice{}, pgx.ErrNoRows
		},
	}, nil)

	_, err := service.Get(context.Background(), uuid.MustParse("11111111-1111-1111-1111-111111111111"))
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestServiceDeleteTreatsMissingAsSuccess(t *testing.T) {
	t.Parallel()

	service := NewService(nil, fakeTransactor{
		q: fakeQuerier{
			getDeviceFn: func(ctx context.Context, arg db.GetNetworkDeviceByIDParams) (db.NetworkDevice, error) {
				return db.NetworkDevice{}, pgx.ErrNoRows
			},
			deleteFn: func(ctx context.Context, arg db.DeleteNetworkDeviceParams) error {
				t.Fatal("expected delete not to be called")
				return nil
			},
			deleteUsergroupsFn: func(ctx context.Context, arg db.DeleteRadusergroupsByUsernameParams) error {
				t.Fatal("expected radusergroup delete not to be called")
				return nil
			},
			deleteRadcheckFn: func(ctx context.Context, arg db.DeleteRadcheckCleartextPasswordByUsernameParams) error {
				t.Fatal("expected radcheck delete not to be called")
				return nil
			},
			createAuditLogFn: func(ctx context.Context, arg db.CreateAuditLogParams) (db.AuditLog, error) {
				t.Fatal("expected audit log not to be called")
				return db.AuditLog{}, nil
			},
		},
	})

	if err := service.Delete(context.Background(), db.User{Username: "alice"}, uuid.MustParse("11111111-1111-1111-1111-111111111111")); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestClassifyDBError(t *testing.T) {
	t.Parallel()

	if !errors.Is(classifyDBError(&pgconn.PgError{Code: "23505"}), ErrConflict) {
		t.Fatal("expected unique violation to map to ErrConflict")
	}
}
