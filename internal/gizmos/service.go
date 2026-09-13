package gizmos

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sbekti/intern/internal/api"
	"github.com/sbekti/intern/internal/db"
	"github.com/sbekti/intern/internal/devices"
)

var (
	ErrNotFound          = errors.New("gizmo not found")
	ErrConflict          = errors.New("gizmo conflict")
	ErrTransactorMissing = errors.New("gizmo transactor not configured")
)

type ValidationError struct {
	Message string
}

func (e ValidationError) Error() string {
	return e.Message
}

type Record struct {
	Gizmo  db.Gizmo
	Device db.NetworkDevice
	VLAN   db.Vlan
}

type Querier interface {
	ListGizmos(ctx context.Context) ([]db.ListGizmosRow, error)
	GetGizmoByNetworkDeviceID(ctx context.Context, arg db.GetGizmoByNetworkDeviceIDParams) (db.GetGizmoByNetworkDeviceIDRow, error)
	GetKioskURLByMAC(ctx context.Context, arg db.GetKioskURLByMACParams) (*string, error)
	CreateGizmo(ctx context.Context, arg db.CreateGizmoParams) (db.Gizmo, error)
	UpdateGizmo(ctx context.Context, arg db.UpdateGizmoParams) (db.Gizmo, error)
	DeleteGizmo(ctx context.Context, arg db.DeleteGizmoParams) error
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
	return &Service{queries: queries, tx: tx}
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

func (s *Service) List(ctx context.Context) ([]Record, error) {
	if s == nil || s.queries == nil {
		return nil, fmt.Errorf("gizmo queries not configured")
	}

	rows, err := s.queries.ListGizmos(ctx)
	if err != nil {
		return nil, err
	}

	records := make([]Record, 0, len(rows))
	for _, row := range rows {
		records = append(records, Record{Gizmo: row.Gizmo, Device: row.NetworkDevice, VLAN: row.Vlan})
	}
	return records, nil
}

func (s *Service) Create(ctx context.Context, actor db.User, input api.GizmoWrite) (Record, error) {
	if s == nil || s.tx == nil {
		return Record{}, ErrTransactorMissing
	}

	kioskURL, err := normalizeKioskURL(input.KioskUrl)
	if err != nil {
		return Record{}, err
	}
	networkDeviceID := toPgUUID(uuid.UUID(input.NetworkDeviceId))

	var created Record
	err = s.tx.InTx(ctx, func(q Querier) error {
		if _, err := q.CreateGizmo(ctx, db.CreateGizmoParams{
			NetworkDeviceID: networkDeviceID,
			KioskUrl:        kioskURL,
		}); err != nil {
			return classifyDBError(err)
		}

		row, err := q.GetGizmoByNetworkDeviceID(ctx, db.GetGizmoByNetworkDeviceIDParams{NetworkDeviceID: networkDeviceID})
		if err != nil {
			return err
		}
		created = recordFromRow(row)
		return writeAuditLog(ctx, q, actor, "gizmo.create", networkDeviceID, nil, kioskURL)
	})
	if err != nil {
		return Record{}, err
	}
	return created, nil
}

func (s *Service) Update(ctx context.Context, actor db.User, networkDeviceID uuid.UUID, input api.GizmoUpdate) (Record, error) {
	if s == nil || s.tx == nil {
		return Record{}, ErrTransactorMissing
	}

	kioskURL, err := normalizeKioskURL(input.KioskUrl)
	if err != nil {
		return Record{}, err
	}
	id := toPgUUID(networkDeviceID)

	var updated Record
	err = s.tx.InTx(ctx, func(q Querier) error {
		current, err := q.GetGizmoByNetworkDeviceID(ctx, db.GetGizmoByNetworkDeviceIDParams{NetworkDeviceID: id})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrNotFound
			}
			return err
		}

		if _, err := q.UpdateGizmo(ctx, db.UpdateGizmoParams{NetworkDeviceID: id, KioskUrl: kioskURL}); err != nil {
			return err
		}
		row, err := q.GetGizmoByNetworkDeviceID(ctx, db.GetGizmoByNetworkDeviceIDParams{NetworkDeviceID: id})
		if err != nil {
			return err
		}
		updated = recordFromRow(row)
		return writeAuditLog(ctx, q, actor, "gizmo.update", id, current.Gizmo.KioskUrl, kioskURL)
	})
	if err != nil {
		return Record{}, err
	}
	return updated, nil
}

func (s *Service) Delete(ctx context.Context, actor db.User, networkDeviceID uuid.UUID) error {
	if s == nil || s.tx == nil {
		return ErrTransactorMissing
	}

	id := toPgUUID(networkDeviceID)
	return s.tx.InTx(ctx, func(q Querier) error {
		current, err := q.GetGizmoByNetworkDeviceID(ctx, db.GetGizmoByNetworkDeviceIDParams{NetworkDeviceID: id})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil
			}
			return err
		}
		if err := q.DeleteGizmo(ctx, db.DeleteGizmoParams{NetworkDeviceID: id}); err != nil {
			return err
		}
		return writeAuditLog(ctx, q, actor, "gizmo.delete", id, current.Gizmo.KioskUrl, nil)
	})
}

func (s *Service) ResolveKioskURL(ctx context.Context, macAddress string) (string, error) {
	if s == nil || s.queries == nil {
		return "", fmt.Errorf("gizmo queries not configured")
	}

	normalizedMAC, err := devices.NormalizeMAC(macAddress)
	if err != nil {
		return "", ValidationError{Message: err.Error()}
	}
	url, err := s.queries.GetKioskURLByMAC(ctx, db.GetKioskURLByMACParams{MacAddress: normalizedMAC})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ErrNotFound
		}
		return "", err
	}
	if url == nil {
		return "", ErrNotFound
	}
	return *url, nil
}

func normalizeKioskURL(value *string) (*string, error) {
	if value == nil {
		return nil, nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil, nil
	}
	if utf8.RuneCountInString(trimmed) > 2048 {
		return nil, ValidationError{Message: "kiosk_url must be at most 2048 characters"}
	}
	return &trimmed, nil
}

func recordFromRow(row db.GetGizmoByNetworkDeviceIDRow) Record {
	return Record{Gizmo: row.Gizmo, Device: row.NetworkDevice, VLAN: row.Vlan}
}

func writeAuditLog(ctx context.Context, q Querier, actor db.User, action string, id pgtype.UUID, before, after *string) error {
	metadata, err := json.Marshal(map[string]any{
		"before": map[string]any{"kiosk_url": before},
		"after":  map[string]any{"kiosk_url": after},
	})
	if err != nil {
		return err
	}
	_, err = q.CreateAuditLog(ctx, db.CreateAuditLogParams{
		ActorUserID:   actor.ID,
		ActorUsername: actor.Username,
		Action:        action,
		ResourceType:  "gizmo",
		ResourceID:    uuid.UUID(id.Bytes).String(),
		Metadata:      metadata,
	})
	return err
}

func classifyDBError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && (pgErr.Code == "23505" || pgErr.Code == "23503") {
		return ErrConflict
	}
	return err
}

func toPgUUID(value uuid.UUID) pgtype.UUID {
	return pgtype.UUID{Bytes: [16]byte(value), Valid: true}
}
