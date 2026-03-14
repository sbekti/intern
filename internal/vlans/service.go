package vlans

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sbekti/intern-api/internal/api"
	"github.com/sbekti/intern-api/internal/db"
)

var (
	ErrNotFound          = errors.New("vlan not found")
	ErrConflict          = errors.New("vlan conflict")
	ErrTransactorMissing = errors.New("vlan transactor not configured")
)

type ValidationError struct {
	Message string
}

func (e ValidationError) Error() string {
	return e.Message
}

type Querier interface {
	ListVlans(ctx context.Context) ([]db.Vlan, error)
	GetVlanByVlanID(ctx context.Context, arg db.GetVlanByVlanIDParams) (db.Vlan, error)
	CreateVlan(ctx context.Context, arg db.CreateVlanParams) (db.Vlan, error)
	UpdateVlan(ctx context.Context, arg db.UpdateVlanParams) (db.Vlan, error)
	DeleteVlan(ctx context.Context, arg db.DeleteVlanParams) error
	DeleteRadgrouprepliesByGroupname(ctx context.Context, arg db.DeleteRadgrouprepliesByGroupnameParams) error
	InsertRadgroupreply(ctx context.Context, arg db.InsertRadgroupreplyParams) error
	UpdateRadusergroupsGroupname(ctx context.Context, arg db.UpdateRadusergroupsGroupnameParams) error
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

func (s *Service) List(ctx context.Context) ([]db.Vlan, error) {
	if s == nil || s.queries == nil {
		return nil, fmt.Errorf("vlan queries not configured")
	}
	return s.queries.ListVlans(ctx)
}

func (s *Service) Get(ctx context.Context, vlanID int32) (db.Vlan, error) {
	if s == nil || s.queries == nil {
		return db.Vlan{}, fmt.Errorf("vlan queries not configured")
	}

	vlan, err := s.queries.GetVlanByVlanID(ctx, db.GetVlanByVlanIDParams{VlanID: vlanID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.Vlan{}, ErrNotFound
		}
		return db.Vlan{}, err
	}

	return vlan, nil
}

func (s *Service) Create(ctx context.Context, actor db.User, input api.VlanWrite) (db.Vlan, error) {
	if s == nil || s.tx == nil {
		return db.Vlan{}, ErrTransactorMissing
	}

	params, err := normalizeCreate(input)
	if err != nil {
		return db.Vlan{}, err
	}

	var created db.Vlan
	err = s.tx.InTx(ctx, func(q Querier) error {
		created, err = q.CreateVlan(ctx, params)
		if err != nil {
			return classifyDBError(err)
		}

		if err := syncRadiusGroupReplies(ctx, q, created.VlanID); err != nil {
			return classifyDBError(err)
		}

		metadata, err := json.Marshal(map[string]any{
			"after": map[string]any{
				"name":        created.Name,
				"vlan_id":     created.VlanID,
				"description": created.Description,
			},
		})
		if err != nil {
			return err
		}

		_, err = q.CreateAuditLog(ctx, db.CreateAuditLogParams{
			ActorUserID:   actor.ID,
			ActorUsername: actor.Username,
			Action:        "vlan.create",
			ResourceType:  "vlan",
			ResourceID:    fmt.Sprintf("%d", created.VlanID),
			Metadata:      metadata,
		})
		return err
	})
	if err != nil {
		return db.Vlan{}, err
	}

	return created, nil
}

func (s *Service) Update(ctx context.Context, actor db.User, currentVlanID int32, patch api.VlanPatch) (db.Vlan, error) {
	if s == nil || s.tx == nil {
		return db.Vlan{}, ErrTransactorMissing
	}

	var updated db.Vlan
	err := s.tx.InTx(ctx, func(q Querier) error {
		current, err := q.GetVlanByVlanID(ctx, db.GetVlanByVlanIDParams{VlanID: currentVlanID})
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
		params.CurrentVlanID = currentVlanID

		updated, err = q.UpdateVlan(ctx, params)
		if err != nil {
			return classifyDBError(err)
		}

		oldGroup := radiusGroupName(current.VlanID)
		newGroup := radiusGroupName(updated.VlanID)
		if oldGroup != newGroup {
			if err := q.UpdateRadusergroupsGroupname(ctx, db.UpdateRadusergroupsGroupnameParams{
				OldGroupname: oldGroup,
				NewGroupname: newGroup,
			}); err != nil {
				return classifyDBError(err)
			}
			if err := q.DeleteRadgrouprepliesByGroupname(ctx, db.DeleteRadgrouprepliesByGroupnameParams{
				Groupname: oldGroup,
			}); err != nil {
				return classifyDBError(err)
			}
		}
		if err := syncRadiusGroupReplies(ctx, q, updated.VlanID); err != nil {
			return classifyDBError(err)
		}

		metadata, err := json.Marshal(map[string]any{
			"before": map[string]any{
				"name":        current.Name,
				"vlan_id":     current.VlanID,
				"description": current.Description,
			},
			"after": map[string]any{
				"name":        updated.Name,
				"vlan_id":     updated.VlanID,
				"description": updated.Description,
			},
		})
		if err != nil {
			return err
		}

		_, err = q.CreateAuditLog(ctx, db.CreateAuditLogParams{
			ActorUserID:   actor.ID,
			ActorUsername: actor.Username,
			Action:        "vlan.update",
			ResourceType:  "vlan",
			ResourceID:    fmt.Sprintf("%d", updated.VlanID),
			Metadata:      metadata,
		})
		return err
	})
	if err != nil {
		return db.Vlan{}, err
	}

	return updated, nil
}

func (s *Service) Delete(ctx context.Context, actor db.User, vlanID int32) error {
	if s == nil || s.tx == nil {
		return ErrTransactorMissing
	}

	return s.tx.InTx(ctx, func(q Querier) error {
		current, err := q.GetVlanByVlanID(ctx, db.GetVlanByVlanIDParams{VlanID: vlanID})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil
			}
			return err
		}

		if err := q.DeleteVlan(ctx, db.DeleteVlanParams{VlanID: vlanID}); err != nil {
			return classifyDBError(err)
		}
		if err := q.DeleteRadgrouprepliesByGroupname(ctx, db.DeleteRadgrouprepliesByGroupnameParams{
			Groupname: radiusGroupName(current.VlanID),
		}); err != nil {
			return classifyDBError(err)
		}

		metadata, err := json.Marshal(map[string]any{
			"before": map[string]any{
				"name":        current.Name,
				"vlan_id":     current.VlanID,
				"description": current.Description,
			},
		})
		if err != nil {
			return err
		}

		_, err = q.CreateAuditLog(ctx, db.CreateAuditLogParams{
			ActorUserID:   actor.ID,
			ActorUsername: actor.Username,
			Action:        "vlan.delete",
			ResourceType:  "vlan",
			ResourceID:    fmt.Sprintf("%d", current.VlanID),
			Metadata:      metadata,
		})
		return err
	})
}

func normalizeCreate(input api.VlanWrite) (db.CreateVlanParams, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return db.CreateVlanParams{}, ValidationError{Message: "name must not be empty"}
	}
	if input.VlanId < 1 || input.VlanId > 4094 {
		return db.CreateVlanParams{}, ValidationError{Message: "vlan_id must be between 1 and 4094"}
	}

	description := ""
	if input.Description != nil {
		description = strings.TrimSpace(*input.Description)
	}

	return db.CreateVlanParams{
		VlanID:      input.VlanId,
		Name:        name,
		Description: description,
	}, nil
}

func mergePatch(current db.Vlan, patch api.VlanPatch) (db.UpdateVlanParams, error) {
	if patch.Name == nil && patch.VlanId == nil && patch.Description == nil {
		return db.UpdateVlanParams{}, ValidationError{Message: "patch must include at least one field"}
	}

	name := current.Name
	if patch.Name != nil {
		name = strings.TrimSpace(*patch.Name)
		if name == "" {
			return db.UpdateVlanParams{}, ValidationError{Message: "name must not be empty"}
		}
	}

	vlanID := current.VlanID
	if patch.VlanId != nil {
		if *patch.VlanId < 1 || *patch.VlanId > 4094 {
			return db.UpdateVlanParams{}, ValidationError{Message: "vlan_id must be between 1 and 4094"}
		}
		vlanID = *patch.VlanId
	}

	description := current.Description
	if patch.Description != nil {
		description = strings.TrimSpace(*patch.Description)
	}

	return db.UpdateVlanParams{
		VlanID:      vlanID,
		Name:        name,
		Description: description,
	}, nil
}

func radiusGroupName(vlanID int32) string {
	return fmt.Sprintf("vlan-%d", vlanID)
}

func syncRadiusGroupReplies(ctx context.Context, q Querier, vlanID int32) error {
	groupName := radiusGroupName(vlanID)
	if err := q.DeleteRadgrouprepliesByGroupname(ctx, db.DeleteRadgrouprepliesByGroupnameParams{
		Groupname: groupName,
	}); err != nil {
		return err
	}

	rows := []db.InsertRadgroupreplyParams{
		{Groupname: groupName, Attribute: "Tunnel-Type", Op: ":=", Value: "VLAN"},
		{Groupname: groupName, Attribute: "Tunnel-Medium-Type", Op: ":=", Value: "IEEE-802"},
		{Groupname: groupName, Attribute: "Tunnel-Private-Group-ID", Op: ":=", Value: fmt.Sprintf("%d", vlanID)},
	}
	for _, row := range rows {
		if err := q.InsertRadgroupreply(ctx, row); err != nil {
			return err
		}
	}

	return nil
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
