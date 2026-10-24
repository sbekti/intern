package auditlogs

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/google/uuid"
	"github.com/sbekti/intern/internal/api"
	"github.com/sbekti/intern/internal/db"
)

const (
	DefaultLimit int32 = 50
	MaxLimit     int32 = 200
)

type Querier interface {
	CountAuditLogs(ctx context.Context, arg db.CountAuditLogsParams) (int64, error)
	ListAuditLogs(ctx context.Context, arg db.ListAuditLogsParams) ([]db.AuditLog, error)
	GetAuditLogFilterOptions(ctx context.Context) (db.GetAuditLogFilterOptionsRow, error)
}

type Filter struct {
	Query                 string
	Action                string
	ResourceType          string
	ResourceID            string
	ActorUsername         string
	ActorUsernameContains string
	MetadataContains      string
	SortBy                string
	SortDir               string
	Limit                 int32
	Offset                int32
}

type Page struct {
	Items         []api.AuditLogEntry
	Limit         int32
	Offset        int32
	TotalCount    int64
	FilterOptions api.AuditLogFilterOptions
}

type Service struct {
	queries Querier
}

func NewService(queries Querier) *Service {
	return &Service{queries: queries}
}

func (s *Service) List(ctx context.Context, filter Filter) (*Page, error) {
	limit := filter.Limit
	if limit <= 0 {
		limit = DefaultLimit
	}
	if limit > MaxLimit {
		limit = MaxLimit
	}

	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}
	sortBy, sortDir := filter.SortBy, filter.SortDir
	if sortBy == "" {
		sortBy = "created_at"
	}
	if sortDir == "" {
		sortDir = "desc"
	}

	args := db.ListAuditLogsParams{
		Query:                 strings.TrimSpace(filter.Query),
		Action:                filter.Action,
		ResourceType:          filter.ResourceType,
		ResourceID:            filter.ResourceID,
		ActorUsername:         filter.ActorUsername,
		ActorUsernameContains: strings.TrimSpace(filter.ActorUsernameContains),
		MetadataContains:      strings.TrimSpace(filter.MetadataContains),
		SortBy:                sortBy,
		SortDir:               sortDir,
		LimitCount:            limit,
		OffsetCount:           offset,
	}

	total, err := s.queries.CountAuditLogs(ctx, db.CountAuditLogsParams{
		Query:                 args.Query,
		Action:                args.Action,
		ResourceType:          args.ResourceType,
		ResourceID:            args.ResourceID,
		ActorUsername:         args.ActorUsername,
		ActorUsernameContains: args.ActorUsernameContains,
		MetadataContains:      args.MetadataContains,
	})
	if err != nil {
		return nil, err
	}

	rows, err := s.queries.ListAuditLogs(ctx, args)
	if err != nil {
		return nil, err
	}

	options, err := s.queries.GetAuditLogFilterOptions(ctx)
	if err != nil {
		return nil, err
	}

	items := make([]api.AuditLogEntry, 0, len(rows))
	for _, row := range rows {
		entry, err := toAPIEntry(row)
		if err != nil {
			return nil, err
		}
		items = append(items, entry)
	}

	return &Page{
		Items:      items,
		Limit:      limit,
		Offset:     offset,
		TotalCount: total,
		FilterOptions: api.AuditLogFilterOptions{
			Actions:       options.Actions,
			ResourceTypes: options.ResourceTypes,
		},
	}, nil
}

func toAPIEntry(row db.AuditLog) (api.AuditLogEntry, error) {
	entry := api.AuditLogEntry{
		Action:        row.Action,
		ActorUsername: row.ActorUsername,
		CreatedAt:     row.CreatedAt.Time,
		Id:            uuid.UUID(row.ID.Bytes),
		ResourceId:    row.ResourceID,
		ResourceType:  row.ResourceType,
	}

	if len(row.Metadata) == 0 {
		entry.Metadata = map[string]interface{}{}
		return entry, nil
	}

	var metadata map[string]interface{}
	if err := json.Unmarshal(row.Metadata, &metadata); err != nil {
		return api.AuditLogEntry{}, err
	}
	entry.Metadata = metadata

	return entry, nil
}
