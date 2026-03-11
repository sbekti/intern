package devices

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"unicode"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sbekti/intern-api/internal/api"
	"github.com/sbekti/intern-api/internal/db"
)

var (
	ErrNotFound          = errors.New("device not found")
	ErrConflict          = errors.New("device conflict")
	ErrTransactorMissing = errors.New("device transactor not configured")
)

type ValidationError struct {
	Message string
}

func (e ValidationError) Error() string {
	return e.Message
}

type DeviceRecord struct {
	Device db.NetworkDevice
	VLAN   db.Vlan
}

type Querier interface {
	ListNetworkDevices(ctx context.Context) ([]db.NetworkDevice, error)
	GetNetworkDeviceByID(ctx context.Context, arg db.GetNetworkDeviceByIDParams) (db.NetworkDevice, error)
	CreateNetworkDevice(ctx context.Context, arg db.CreateNetworkDeviceParams) (db.NetworkDevice, error)
	UpdateNetworkDevice(ctx context.Context, arg db.UpdateNetworkDeviceParams) (db.NetworkDevice, error)
	DeleteNetworkDevice(ctx context.Context, arg db.DeleteNetworkDeviceParams) error
	GetVlanByID(ctx context.Context, arg db.GetVlanByIDParams) (db.Vlan, error)
	UpsertRadcheckCleartextPassword(ctx context.Context, arg db.UpsertRadcheckCleartextPasswordParams) error
	DeleteRadcheckCleartextPasswordByUsername(ctx context.Context, arg db.DeleteRadcheckCleartextPasswordByUsernameParams) error
	DeleteRadusergroupsByUsername(ctx context.Context, arg db.DeleteRadusergroupsByUsernameParams) error
	InsertRadusergroup(ctx context.Context, arg db.InsertRadusergroupParams) error
	CreateAuditLog(ctx context.Context, arg db.CreateAuditLogParams) (db.AuditLog, error)
}

type Transactor interface {
	InTx(ctx context.Context, fn func(q Querier) error) error
}

type Service struct {
	queries Querier
	tx      Transactor
}

type PGXTransactor struct {
	pool *pgxpool.Pool
}

func NewService(queries Querier, tx Transactor) *Service {
	return &Service{
		queries: queries,
		tx:      tx,
	}
}

func NewPGXTransactor(pool *pgxpool.Pool) *PGXTransactor {
	return &PGXTransactor{pool: pool}
}

func (t *PGXTransactor) InTx(ctx context.Context, fn func(q Querier) error) error {
	tx, err := t.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}

	queries := db.New(tx)
	if err := fn(queries); err != nil {
		_ = tx.Rollback(ctx)
		return err
	}

	return tx.Commit(ctx)
}

func (s *Service) List(ctx context.Context) ([]DeviceRecord, error) {
	if s == nil || s.queries == nil {
		return nil, fmt.Errorf("device queries not configured")
	}

	devices, err := s.queries.ListNetworkDevices(ctx)
	if err != nil {
		return nil, err
	}

	records := make([]DeviceRecord, 0, len(devices))
	for _, device := range devices {
		vlan, err := s.queries.GetVlanByID(ctx, db.GetVlanByIDParams{ID: device.VlanID})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, ErrConflict
			}
			return nil, err
		}
		records = append(records, DeviceRecord{Device: device, VLAN: vlan})
	}

	return records, nil
}

func (s *Service) Get(ctx context.Context, id uuid.UUID) (DeviceRecord, error) {
	if s == nil || s.queries == nil {
		return DeviceRecord{}, fmt.Errorf("device queries not configured")
	}

	device, err := s.queries.GetNetworkDeviceByID(ctx, db.GetNetworkDeviceByIDParams{ID: toPgUUID(id)})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return DeviceRecord{}, ErrNotFound
		}
		return DeviceRecord{}, err
	}

	vlan, err := s.queries.GetVlanByID(ctx, db.GetVlanByIDParams{ID: device.VlanID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return DeviceRecord{}, ErrConflict
		}
		return DeviceRecord{}, err
	}

	return DeviceRecord{Device: device, VLAN: vlan}, nil
}

func (s *Service) Create(ctx context.Context, actor db.User, input api.NetworkDeviceWrite) (DeviceRecord, error) {
	if s == nil || s.tx == nil {
		return DeviceRecord{}, ErrTransactorMissing
	}

	params, err := normalizeCreate(input)
	if err != nil {
		return DeviceRecord{}, err
	}

	var created DeviceRecord
	err = s.tx.InTx(ctx, func(q Querier) error {
		vlan, err := q.GetVlanByID(ctx, db.GetVlanByIDParams{ID: params.VlanID})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrConflict
			}
			return err
		}

		device, err := q.CreateNetworkDevice(ctx, db.CreateNetworkDeviceParams{
			MacAddress:      params.MacAddressColon,
			DisplayName:     params.DisplayName,
			VlanID:          params.VlanID,
			CreatedByUserID: nullablePgUUID(actor.ID),
			UpdatedByUserID: nullablePgUUID(actor.ID),
		})
		if err != nil {
			return classifyDBError(err)
		}

		if err := q.UpsertRadcheckCleartextPassword(ctx, db.UpsertRadcheckCleartextPasswordParams{
			Username: params.MacAddressBare,
			Value:    params.MacAddressBare,
		}); err != nil {
			return classifyDBError(err)
		}
		if err := q.DeleteRadusergroupsByUsername(ctx, db.DeleteRadusergroupsByUsernameParams{
			Username: params.MacAddressBare,
		}); err != nil {
			return classifyDBError(err)
		}
		if err := q.InsertRadusergroup(ctx, db.InsertRadusergroupParams{
			Username:  params.MacAddressBare,
			Groupname: vlan.Name,
			Priority:  0,
		}); err != nil {
			return classifyDBError(err)
		}

		metadata, err := json.Marshal(map[string]any{
			"after": map[string]any{
				"id":           deviceIDString(device.ID),
				"mac_address":  device.MacAddress,
				"display_name": device.DisplayName,
				"vlan_id":      device.VlanID,
				"radius_group": vlan.Name,
			},
		})
		if err != nil {
			return err
		}

		_, err = q.CreateAuditLog(ctx, db.CreateAuditLogParams{
			ActorUserID:   actor.ID,
			ActorUsername: actor.Username,
			Action:        "device.create",
			ResourceType:  "network_device",
			ResourceID:    deviceIDString(device.ID),
			Metadata:      metadata,
		})
		if err != nil {
			return err
		}

		created = DeviceRecord{Device: device, VLAN: vlan}
		return nil
	})
	if err != nil {
		return DeviceRecord{}, err
	}

	return created, nil
}

func (s *Service) Update(ctx context.Context, actor db.User, id uuid.UUID, patch api.NetworkDevicePatch) (DeviceRecord, error) {
	if s == nil || s.tx == nil {
		return DeviceRecord{}, ErrTransactorMissing
	}

	var updated DeviceRecord
	err := s.tx.InTx(ctx, func(q Querier) error {
		current, err := q.GetNetworkDeviceByID(ctx, db.GetNetworkDeviceByIDParams{ID: toPgUUID(id)})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrNotFound
			}
			return err
		}

		params, err := mergePatch(current, patch)
		if err != nil {
			return err
		}
		params.ID = current.ID

		vlan, err := q.GetVlanByID(ctx, db.GetVlanByIDParams{ID: params.VlanID})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrConflict
			}
			return err
		}

		device, err := q.UpdateNetworkDevice(ctx, db.UpdateNetworkDeviceParams{
			ID:              params.ID,
			MacAddress:      params.MacAddress,
			DisplayName:     params.DisplayName,
			VlanID:          params.VlanID,
			UpdatedByUserID: nullablePgUUID(actor.ID),
		})
		if err != nil {
			return classifyDBError(err)
		}

		oldRadiusUsername := colonMACToBare(current.MacAddress)
		newRadiusUsername := colonMACToBare(device.MacAddress)
		if oldRadiusUsername != newRadiusUsername {
			if err := q.DeleteRadusergroupsByUsername(ctx, db.DeleteRadusergroupsByUsernameParams{
				Username: oldRadiusUsername,
			}); err != nil {
				return classifyDBError(err)
			}
			if err := q.DeleteRadcheckCleartextPasswordByUsername(ctx, db.DeleteRadcheckCleartextPasswordByUsernameParams{
				Username: oldRadiusUsername,
			}); err != nil {
				return classifyDBError(err)
			}
		}

		if err := q.UpsertRadcheckCleartextPassword(ctx, db.UpsertRadcheckCleartextPasswordParams{
			Username: newRadiusUsername,
			Value:    newRadiusUsername,
		}); err != nil {
			return classifyDBError(err)
		}
		if err := q.DeleteRadusergroupsByUsername(ctx, db.DeleteRadusergroupsByUsernameParams{
			Username: newRadiusUsername,
		}); err != nil {
			return classifyDBError(err)
		}
		if err := q.InsertRadusergroup(ctx, db.InsertRadusergroupParams{
			Username:  newRadiusUsername,
			Groupname: vlan.Name,
			Priority:  0,
		}); err != nil {
			return classifyDBError(err)
		}

		metadataBody := map[string]any{
			"before": map[string]any{
				"id":           deviceIDString(current.ID),
				"mac_address":  current.MacAddress,
				"display_name": current.DisplayName,
				"vlan_id":      current.VlanID,
			},
			"after": map[string]any{
				"id":           deviceIDString(device.ID),
				"mac_address":  device.MacAddress,
				"display_name": device.DisplayName,
				"vlan_id":      device.VlanID,
				"radius_group": vlan.Name,
			},
		}
		if current.MacAddress != device.MacAddress {
			metadataBody["old_mac_address"] = current.MacAddress
			metadataBody["new_mac_address"] = device.MacAddress
		}

		metadata, err := json.Marshal(metadataBody)
		if err != nil {
			return err
		}

		_, err = q.CreateAuditLog(ctx, db.CreateAuditLogParams{
			ActorUserID:   actor.ID,
			ActorUsername: actor.Username,
			Action:        "device.update",
			ResourceType:  "network_device",
			ResourceID:    deviceIDString(device.ID),
			Metadata:      metadata,
		})
		if err != nil {
			return err
		}

		updated = DeviceRecord{Device: device, VLAN: vlan}
		return nil
	})
	if err != nil {
		return DeviceRecord{}, err
	}

	return updated, nil
}

func (s *Service) Delete(ctx context.Context, actor db.User, id uuid.UUID) error {
	if s == nil || s.tx == nil {
		return ErrTransactorMissing
	}

	return s.tx.InTx(ctx, func(q Querier) error {
		current, err := q.GetNetworkDeviceByID(ctx, db.GetNetworkDeviceByIDParams{ID: toPgUUID(id)})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil
			}
			return err
		}

		if err := q.DeleteNetworkDevice(ctx, db.DeleteNetworkDeviceParams{ID: current.ID}); err != nil {
			return classifyDBError(err)
		}

		radiusUsername := colonMACToBare(current.MacAddress)
		if err := q.DeleteRadusergroupsByUsername(ctx, db.DeleteRadusergroupsByUsernameParams{
			Username: radiusUsername,
		}); err != nil {
			return classifyDBError(err)
		}
		if err := q.DeleteRadcheckCleartextPasswordByUsername(ctx, db.DeleteRadcheckCleartextPasswordByUsernameParams{
			Username: radiusUsername,
		}); err != nil {
			return classifyDBError(err)
		}

		metadata, err := json.Marshal(map[string]any{
			"before": map[string]any{
				"id":           deviceIDString(current.ID),
				"mac_address":  current.MacAddress,
				"display_name": current.DisplayName,
				"vlan_id":      current.VlanID,
			},
		})
		if err != nil {
			return err
		}

		_, err = q.CreateAuditLog(ctx, db.CreateAuditLogParams{
			ActorUserID:   actor.ID,
			ActorUsername: actor.Username,
			Action:        "device.delete",
			ResourceType:  "network_device",
			ResourceID:    deviceIDString(current.ID),
			Metadata:      metadata,
		})
		return err
	})
}

type normalizedCreate struct {
	MacAddressColon string
	MacAddressBare  string
	DisplayName     string
	VlanID          int64
}

func normalizeCreate(input api.NetworkDeviceWrite) (normalizedCreate, error) {
	macBare, macColon, err := normalizeMAC(input.MacAddress)
	if err != nil {
		return normalizedCreate{}, err
	}

	displayName := strings.TrimSpace(input.DisplayName)
	if displayName == "" {
		return normalizedCreate{}, ValidationError{Message: "display_name must not be empty"}
	}
	if input.VlanId < 1 {
		return normalizedCreate{}, ValidationError{Message: "vlan_id must be greater than zero"}
	}

	return normalizedCreate{
		MacAddressColon: macColon,
		MacAddressBare:  macBare,
		DisplayName:     displayName,
		VlanID:          input.VlanId,
	}, nil
}

func mergePatch(current db.NetworkDevice, patch api.NetworkDevicePatch) (db.UpdateNetworkDeviceParams, error) {
	if patch.DisplayName == nil && patch.VlanId == nil && patch.MacAddress == nil {
		return db.UpdateNetworkDeviceParams{}, ValidationError{Message: "patch must include at least one field"}
	}

	macAddress := current.MacAddress
	if patch.MacAddress != nil {
		_, normalizedMAC, err := normalizeMAC(*patch.MacAddress)
		if err != nil {
			return db.UpdateNetworkDeviceParams{}, err
		}
		macAddress = normalizedMAC
	}

	displayName := current.DisplayName
	if patch.DisplayName != nil {
		displayName = strings.TrimSpace(*patch.DisplayName)
		if displayName == "" {
			return db.UpdateNetworkDeviceParams{}, ValidationError{Message: "display_name must not be empty"}
		}
	}

	vlanID := current.VlanID
	if patch.VlanId != nil {
		if *patch.VlanId < 1 {
			return db.UpdateNetworkDeviceParams{}, ValidationError{Message: "vlan_id must be greater than zero"}
		}
		vlanID = *patch.VlanId
	}

	return db.UpdateNetworkDeviceParams{
		MacAddress:  macAddress,
		DisplayName: displayName,
		VlanID:      vlanID,
	}, nil
}

func normalizeMAC(raw string) (bare string, colon string, err error) {
	var builder strings.Builder
	for _, r := range strings.TrimSpace(strings.ToLower(raw)) {
		switch {
		case r == ':' || r == '-' || r == '.':
			continue
		case unicode.IsDigit(r) || (r >= 'a' && r <= 'f'):
			builder.WriteRune(r)
		default:
			return "", "", ValidationError{Message: "mac_address must contain 12 hexadecimal characters"}
		}
	}

	bare = builder.String()
	if len(bare) != 12 {
		return "", "", ValidationError{Message: "mac_address must contain 12 hexadecimal characters"}
	}
	if _, err := hex.DecodeString(bare); err != nil {
		return "", "", ValidationError{Message: "mac_address must contain 12 hexadecimal characters"}
	}

	var colonBuilder strings.Builder
	for i := 0; i < len(bare); i += 2 {
		if i > 0 {
			colonBuilder.WriteByte(':')
		}
		colonBuilder.WriteString(bare[i : i+2])
	}

	return bare, colonBuilder.String(), nil
}

func colonMACToBare(value string) string {
	return strings.ReplaceAll(value, ":", "")
}

func classifyDBError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505", "23503":
			return ErrConflict
		}
	}
	return err
}

func toPgUUID(value uuid.UUID) pgtype.UUID {
	var bytes [16]byte
	copy(bytes[:], value[:])
	return pgtype.UUID{Bytes: bytes, Valid: true}
}

func nullablePgUUID(value pgtype.UUID) pgtype.UUID {
	if !value.Valid {
		return pgtype.UUID{}
	}
	return value
}

func deviceIDString(value pgtype.UUID) string {
	if !value.Valid {
		return ""
	}
	return uuid.UUID(value.Bytes).String()
}
