package presence

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sbekti/intern-api/internal/api"
	"github.com/sbekti/intern-api/internal/db"
)

var (
	ErrObservationPointNotFound = errors.New("presence observation point not found")
	ErrCatalogPoolMissing       = errors.New("presence catalog pool not configured")
)

type ValidationError struct {
	Message string
}

func (e ValidationError) Error() string {
	return e.Message
}

type ManagedPresenceSummary struct {
	DeviceID               uuid.UUID
	Status                 string
	LastSeenAt             time.Time
	SourceKey              string
	SourceType             string
	Medium                 string
	ObservationExternalID  string
	ObservationDisplayName string
	LocationLabel          string
	SSID                   string
}

type ObservedClientFilter struct {
	Query         string
	Status        string
	SourceType    string
	SourceKey     string
	Medium        string
	LocationQuery string
	Limit         int32
	Offset        int32
}

type ObservedClientRecord struct {
	ID                     uuid.UUID
	MacAddress             string
	ManagedDeviceID        *uuid.UUID
	ManagedDeviceName      string
	Status                 string
	FirstSeenAt            time.Time
	LastSeenAt             time.Time
	SourceKey              string
	SourceType             string
	Medium                 string
	ObservationPointID     *uuid.UUID
	ObservationExternalID  string
	ObservationDisplayName string
	LocationLabel          string
	SSID                   string
}

type ObservedClientPage struct {
	Items      []ObservedClientRecord
	Pagination Page
}

type ObservationPointFilter struct {
	Query      string
	SourceType string
	SourceKey  string
	Medium     string
	Limit      int32
	Offset     int32
}

type ObservationPointRecord struct {
	ID               uuid.UUID
	SourceKey        string
	SourceType       string
	Medium           string
	ExternalID       string
	ParentExternalID string
	DisplayName      string
	LocationLabel    string
	Notes            string
	SSID             string
	LastSeenAt       *time.Time
}

type ObservationPointPage struct {
	Items      []ObservationPointRecord
	Pagination Page
}

type ObservationPointPatch struct {
	LocationLabel *string
	Notes         *string
}

type Page struct {
	Limit  int32
	Offset int32
	Total  int64
}

type CatalogService struct {
	queries *db.Queries
	pool    *pgxpool.Pool
}

func NewCatalogService(queries *db.Queries, pool *pgxpool.Pool) *CatalogService {
	return &CatalogService{
		queries: queries,
		pool:    pool,
	}
}

func (s *CatalogService) ListManagedPresence(ctx context.Context) (map[uuid.UUID]ManagedPresenceSummary, error) {
	if s == nil || s.queries == nil {
		return map[uuid.UUID]ManagedPresenceSummary{}, nil
	}

	rows, err := s.queries.ListManagedPresenceClients(ctx)
	if err != nil {
		return nil, err
	}

	items := make(map[uuid.UUID]ManagedPresenceSummary, len(rows))
	for _, row := range rows {
		deviceID, ok := uuidFromPg(row.NetworkDeviceID)
		if !ok {
			continue
		}
		items[deviceID] = ManagedPresenceSummary{
			DeviceID:               deviceID,
			Status:                 row.Status,
			LastSeenAt:             row.LastSeenAt.Time.UTC(),
			SourceKey:              row.LastSourceKey,
			SourceType:             row.LastSourceType,
			Medium:                 row.LastMedium,
			ObservationExternalID:  row.ObservationExternalID,
			ObservationDisplayName: row.ObservationDisplayName,
			LocationLabel:          row.ObservationLocationLabel,
			SSID:                   derefString(row.LastSsid),
		}
	}

	return items, nil
}

func (s *CatalogService) GetManagedPresence(ctx context.Context, deviceID uuid.UUID) (*ManagedPresenceSummary, error) {
	if s == nil || s.queries == nil {
		return nil, nil
	}

	row, err := s.queries.GetManagedPresenceClientByDeviceID(ctx, db.GetManagedPresenceClientByDeviceIDParams{
		NetworkDeviceID: uuidToPg(deviceID),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	return &ManagedPresenceSummary{
		DeviceID:               deviceID,
		Status:                 row.Status,
		LastSeenAt:             row.LastSeenAt.Time.UTC(),
		SourceKey:              row.LastSourceKey,
		SourceType:             row.LastSourceType,
		Medium:                 row.LastMedium,
		ObservationExternalID:  row.ObservationExternalID,
		ObservationDisplayName: row.ObservationDisplayName,
		LocationLabel:          row.ObservationLocationLabel,
		SSID:                   derefString(row.LastSsid),
	}, nil
}

func (s *CatalogService) ListObservedClients(ctx context.Context, filter ObservedClientFilter) (*ObservedClientPage, error) {
	if s == nil || s.queries == nil {
		return &ObservedClientPage{Items: []ObservedClientRecord{}, Pagination: Page{Limit: filter.Limit, Offset: filter.Offset}}, nil
	}

	total, err := s.queries.CountObservedPresenceClients(ctx, db.CountObservedPresenceClientsParams{
		Status:        filter.Status,
		SourceType:    filter.SourceType,
		SourceKey:     filter.SourceKey,
		Medium:        filter.Medium,
		LocationQuery: filter.LocationQuery,
		Query:         filter.Query,
	})
	if err != nil {
		return nil, err
	}

	rows, err := s.queries.ListObservedPresenceClients(ctx, db.ListObservedPresenceClientsParams{
		Status:        filter.Status,
		SourceType:    filter.SourceType,
		SourceKey:     filter.SourceKey,
		Medium:        filter.Medium,
		LocationQuery: filter.LocationQuery,
		Query:         filter.Query,
		LimitCount:    filter.Limit,
		OffsetCount:   filter.Offset,
	})
	if err != nil {
		return nil, err
	}

	items := make([]ObservedClientRecord, 0, len(rows))
	for _, row := range rows {
		items = append(items, ObservedClientRecord{
			ID:                     pgUUIDString(row.ID),
			MacAddress:             row.MacAddress,
			ManagedDeviceID:        optionalUUID(row.NetworkDeviceID),
			ManagedDeviceName:      row.ManagedDeviceDisplayName,
			Status:                 row.Status,
			FirstSeenAt:            row.FirstSeenAt.Time.UTC(),
			LastSeenAt:             row.LastSeenAt.Time.UTC(),
			SourceKey:              row.LastSourceKey,
			SourceType:             row.LastSourceType,
			Medium:                 row.LastMedium,
			ObservationPointID:     optionalUUID(row.LastObservationPointID),
			ObservationExternalID:  row.ObservationExternalID,
			ObservationDisplayName: row.ObservationDisplayName,
			LocationLabel:          row.ObservationLocationLabel,
			SSID:                   derefString(row.LastSsid),
		})
	}

	return &ObservedClientPage{
		Items: items,
		Pagination: Page{
			Limit:  filter.Limit,
			Offset: filter.Offset,
			Total:  total,
		},
	}, nil
}

func (s *CatalogService) ListObservationPoints(ctx context.Context, filter ObservationPointFilter) (*ObservationPointPage, error) {
	if s == nil || s.queries == nil {
		return &ObservationPointPage{Items: []ObservationPointRecord{}, Pagination: Page{Limit: filter.Limit, Offset: filter.Offset}}, nil
	}

	total, err := s.queries.CountPresenceObservationPoints(ctx, db.CountPresenceObservationPointsParams{
		SourceType: filter.SourceType,
		SourceKey:  filter.SourceKey,
		Medium:     filter.Medium,
		Query:      filter.Query,
	})
	if err != nil {
		return nil, err
	}

	rows, err := s.queries.ListPresenceObservationPoints(ctx, db.ListPresenceObservationPointsParams{
		SourceType: filter.SourceType,
		SourceKey:  filter.SourceKey,
		Medium:     filter.Medium,
		Query:      filter.Query,
		LimitCount:  filter.Limit,
		OffsetCount: filter.Offset,
	})
	if err != nil {
		return nil, err
	}

	items := make([]ObservationPointRecord, 0, len(rows))
	for _, row := range rows {
		item := ObservationPointRecord{
			ID:               pgUUIDString(row.ID),
			SourceKey:        row.SourceKey,
			SourceType:       row.SourceType,
			Medium:           row.Medium,
			ExternalID:       row.ExternalID,
			ParentExternalID: row.ParentExternalID,
			DisplayName:      row.DisplayName,
			LocationLabel:    row.LocationLabel,
			Notes:            row.Notes,
			SSID:             derefString(row.Ssid),
		}
		if row.LastSeenAt.Valid {
			value := row.LastSeenAt.Time.UTC()
			item.LastSeenAt = &value
		}
		items = append(items, item)
	}

	return &ObservationPointPage{
		Items: items,
		Pagination: Page{
			Limit:  filter.Limit,
			Offset: filter.Offset,
			Total:  total,
		},
	}, nil
}

func (s *CatalogService) UpdateObservationPoint(ctx context.Context, actor db.User, id uuid.UUID, patch api.PresenceObservationPointPatch) (ObservationPointRecord, error) {
	if s == nil || s.pool == nil {
		return ObservationPointRecord{}, ErrCatalogPoolMissing
	}
	if patch.LocationLabel == nil && patch.Notes == nil {
		return ObservationPointRecord{}, ValidationError{Message: "patch must include at least one field"}
	}

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return ObservationPointRecord{}, err
	}
	defer tx.Rollback(ctx)

	q := db.New(tx)
	current, err := q.GetPresenceObservationPointByID(ctx, db.GetPresenceObservationPointByIDParams{
		ID: uuidToPg(id),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ObservationPointRecord{}, ErrObservationPointNotFound
		}
		return ObservationPointRecord{}, err
	}

	nextLocationLabel := current.LocationLabel
	if patch.LocationLabel != nil {
		nextLocationLabel = *patch.LocationLabel
	}
	nextNotes := current.Notes
	if patch.Notes != nil {
		nextNotes = *patch.Notes
	}

	updated, err := q.UpdatePresenceObservationPointAdmin(ctx, db.UpdatePresenceObservationPointAdminParams{
		ID:            uuidToPg(id),
		LocationLabel: nextLocationLabel,
		Notes:         nextNotes,
	})
	if err != nil {
		return ObservationPointRecord{}, err
	}

	metadata, err := json.Marshal(map[string]any{
		"before": map[string]any{
			"id":             id.String(),
			"source_key":     current.SourceKey,
			"source_type":    current.SourceType,
			"medium":         current.Medium,
			"external_id":    current.ExternalID,
			"location_label": current.LocationLabel,
			"notes":          current.Notes,
		},
		"after": map[string]any{
			"id":             id.String(),
			"source_key":     updated.SourceKey,
			"source_type":    updated.SourceType,
			"medium":         updated.Medium,
			"external_id":    updated.ExternalID,
			"location_label": updated.LocationLabel,
			"notes":          updated.Notes,
		},
	})
	if err != nil {
		return ObservationPointRecord{}, err
	}

	if _, err := q.CreateAuditLog(ctx, db.CreateAuditLogParams{
		ActorUserID:   actor.ID,
		ActorUsername: actor.Username,
		Action:        "presence.observation_point.update",
		ResourceType:  "presence_observation_point",
		ResourceID:    id.String(),
		Metadata:      metadata,
	}); err != nil {
		return ObservationPointRecord{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return ObservationPointRecord{}, err
	}

	return observationPointRecordFromDB(updated), nil
}

func observationPointRecordFromDB(row db.PresenceObservationPoint) ObservationPointRecord {
	item := ObservationPointRecord{
		ID:               pgUUIDString(row.ID),
		SourceKey:        row.SourceKey,
		SourceType:       row.SourceType,
		Medium:           row.Medium,
		ExternalID:       row.ExternalID,
		ParentExternalID: row.ParentExternalID,
		DisplayName:      row.DisplayName,
		LocationLabel:    row.LocationLabel,
		Notes:            row.Notes,
		SSID:             derefString(row.Ssid),
	}
	if row.LastSeenAt.Valid {
		value := row.LastSeenAt.Time.UTC()
		item.LastSeenAt = &value
	}
	return item
}

func optionalUUID(value pgtype.UUID) *uuid.UUID {
	if !value.Valid {
		return nil
	}
	result := uuid.UUID(value.Bytes)
	return &result
}

func uuidFromPg(value pgtype.UUID) (uuid.UUID, bool) {
	if !value.Valid {
		return uuid.UUID{}, false
	}
	return uuid.UUID(value.Bytes), true
}

func uuidToPg(value uuid.UUID) pgtype.UUID {
	return pgtype.UUID{
		Bytes: value,
		Valid: true,
	}
}

func pgUUIDString(value pgtype.UUID) uuid.UUID {
	if !value.Valid {
		return uuid.UUID{}
	}
	return uuid.UUID(value.Bytes)
}

func derefString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
