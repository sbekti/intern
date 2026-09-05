package devices

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/sbekti/intern/internal/api"
	"github.com/sbekti/intern/internal/db"
)

type fakeQuerier struct {
	listFn           func(ctx context.Context) ([]db.ListNetworkDevicesRow, error)
	getDeviceFn      func(ctx context.Context, arg db.GetNetworkDeviceByIDParams) (db.NetworkDevice, error)
	createFn         func(ctx context.Context, arg db.CreateNetworkDeviceParams) (db.NetworkDevice, error)
	updateFn         func(ctx context.Context, arg db.UpdateNetworkDeviceParams) (db.NetworkDevice, error)
	deleteFn         func(ctx context.Context, arg db.DeleteNetworkDeviceParams) error
	getVlanFn        func(ctx context.Context, arg db.GetVlanByVlanIDParams) (db.Vlan, error)
	createAuditLogFn func(ctx context.Context, arg db.CreateAuditLogParams) (db.AuditLog, error)
}

func (f fakeQuerier) ListNetworkDevices(ctx context.Context) ([]db.ListNetworkDevicesRow, error) {
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

func (f fakeQuerier) GetVlanByVlanID(ctx context.Context, arg db.GetVlanByVlanIDParams) (db.Vlan, error) {
	return f.getVlanFn(ctx, arg)
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

	colon, err := normalizeMAC("AA-BB-CC-DD-EE-FF")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if colon != "aa:bb:cc:dd:ee:ff" {
		t.Fatalf("unexpected colon mac %q", colon)
	}

	if _, err := normalizeMAC("bad-mac"); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestServiceListMapsJoinedRows(t *testing.T) {
	t.Parallel()

	service := NewService(fakeQuerier{
		listFn: func(context.Context) ([]db.ListNetworkDevicesRow, error) {
			return []db.ListNetworkDevicesRow{{
				NetworkDevice: db.NetworkDevice{DisplayName: "Camera", VlanID: 20},
				Vlan:          db.Vlan{Name: "iot", VlanID: 20},
			}}, nil
		},
	}, nil)

	records, err := service.List(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(records) != 1 || records[0].Device.DisplayName != "Camera" || records[0].VLAN.Name != "iot" {
		t.Fatalf("unexpected records: %#v", records)
	}
}

func TestMergePatch(t *testing.T) {
	t.Parallel()

	current := db.NetworkDevice{
		MacAddress:  "aa:bb:cc:dd:ee:ff",
		DisplayName: "Living Room TV",
		Disabled:    false,
		VlanID:      2,
	}

	_, err := mergePatch(current, api.NetworkDevicePatch{})
	var validationErr ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("expected validation error, got %v", err)
	}

	name := "  Updated TV  "
	vlanID := int32(3)
	macAddress := "AA-BB-CC-00-11-22"
	disabled := true
	params, err := mergePatch(current, api.NetworkDevicePatch{
		MacAddress:  &macAddress,
		DisplayName: &name,
		Disabled:    &disabled,
		VlanId:      &vlanID,
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if params.MacAddress != "aa:bb:cc:00:11:22" || params.DisplayName != "Updated TV" || params.Disabled != true || params.VlanID != 3 {
		t.Fatalf("unexpected patch params %+v", params)
	}
}

func TestServiceCreateNormalizesMAC(t *testing.T) {
	t.Parallel()

	deviceID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	service := NewService(nil, fakeTransactor{
		q: fakeQuerier{
			getVlanFn: func(ctx context.Context, arg db.GetVlanByVlanIDParams) (db.Vlan, error) {
				return db.Vlan{Name: "iot", VlanID: 20}, nil
			},
			createFn: func(ctx context.Context, arg db.CreateNetworkDeviceParams) (db.NetworkDevice, error) {
				if arg.MacAddress != "aa:bb:cc:dd:ee:ff" {
					t.Fatalf("expected normalized app mac, got %q", arg.MacAddress)
				}
				if arg.Disabled {
					t.Fatal("expected enabled device to stay enabled")
				}
				return db.NetworkDevice{
					ID:          toPgUUID(deviceID),
					MacAddress:  arg.MacAddress,
					DisplayName: arg.DisplayName,
					Disabled:    arg.Disabled,
					VlanID:      arg.VlanID,
				}, nil
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
		VlanId:      20,
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if record.Device.MacAddress != "aa:bb:cc:dd:ee:ff" {
		t.Fatalf("unexpected mac address %q", record.Device.MacAddress)
	}
}

func TestServiceCreatePersistsDisabledState(t *testing.T) {
	t.Parallel()

	deviceID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	service := NewService(nil, fakeTransactor{
		q: fakeQuerier{
			getVlanFn: func(ctx context.Context, arg db.GetVlanByVlanIDParams) (db.Vlan, error) {
				return db.Vlan{Name: "guest", VlanID: 10}, nil
			},
			createFn: func(ctx context.Context, arg db.CreateNetworkDeviceParams) (db.NetworkDevice, error) {
				if !arg.Disabled {
					t.Fatal("expected disabled device to be persisted as disabled")
				}
				return db.NetworkDevice{
					ID:          toPgUUID(deviceID),
					MacAddress:  arg.MacAddress,
					DisplayName: arg.DisplayName,
					Disabled:    arg.Disabled,
					VlanID:      arg.VlanID,
				}, nil
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
		MacAddress:  "AA-BB-CC-DD-EE-44",
		DisplayName: "Spare Phone",
		Disabled:    boolPtrUnit(true),
		VlanId:      10,
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !record.Device.Disabled {
		t.Fatal("expected disabled device record")
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

func boolPtrUnit(value bool) *bool {
	return &value
}
