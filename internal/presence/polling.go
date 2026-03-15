package presence

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/sbekti/intern-api/internal/config"
	"github.com/sbekti/intern-api/internal/db"
)

type polledPresenceClient struct {
	MAC                         string
	DisplayName                 string
	ObservationExternalID       string
	ObservationParentExternalID string
	SSID                        string
	FirstSeen                   time.Time
	LastSeen                    time.Time
	Metadata                    map[string]any
}

type polledSourceSpec struct {
	WorkerName         string
	SourceType         string
	Medium             string
	SessionPrefix      string
	SessionOrigin      string
	MoveCloseReason    string
	TimeoutCloseReason string
}

func (s *Service) syncPolledSource(ctx context.Context, source config.PresenceSourceConfig, spec polledSourceSpec, clients []polledPresenceClient) error {
	if s == nil || s.pool == nil {
		return fmt.Errorf("presence pool not configured")
	}

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}

	pollTime := s.now().UTC()
	txQueries := db.New(tx)
	seenClientMACs := make(map[string]struct{}, len(clients))
	for _, client := range clients {
		clientMAC, err := s.applyPolledClient(ctx, txQueries, source, spec, client, pollTime)
		if err != nil {
			_ = tx.Rollback(ctx)
			return err
		}
		if clientMAC != "" {
			seenClientMACs[clientMAC] = struct{}{}
		}
	}

	if err := s.reconcilePolledDisappearances(ctx, txQueries, source, spec, pollTime, seenClientMACs); err != nil {
		_ = tx.Rollback(ctx)
		return err
	}

	state := map[string]any{"last_client_count": len(clients)}
	if _, err := txQueries.UpsertPresenceWorkerState(ctx, db.UpsertPresenceWorkerStateParams{
		WorkerName:      spec.WorkerName,
		SourceKey:       source.Key,
		State:           mustJSON(state),
		LastPolledAt:    timestamptz(pollTime),
		LastSucceededAt: timestamptz(pollTime),
	}); err != nil {
		_ = tx.Rollback(ctx)
		return err
	}

	return tx.Commit(ctx)
}

func (s *Service) applyPolledClient(ctx context.Context, q db.Querier, source config.PresenceSourceConfig, spec polledSourceSpec, client polledPresenceClient, pollTime time.Time) (string, error) {
	clientMAC, err := normalizedColonMAC(client.MAC)
	if err != nil {
		s.logger.Warn("skipping polled client with invalid mac", "source_key", source.Key, "source_type", spec.SourceType, "mac", client.MAC, "error", err)
		return "", nil
	}

	lastSeen := client.LastSeen.UTC()
	if lastSeen.IsZero() {
		lastSeen = client.FirstSeen.UTC()
	}
	if lastSeen.IsZero() {
		lastSeen = pollTime
	}

	firstSeen := client.FirstSeen.UTC()
	if firstSeen.IsZero() {
		firstSeen = lastSeen
	}

	observationPointID := pgtype.UUID{}
	if client.ObservationExternalID != "" {
		observationPoint, err := q.UpsertPresenceObservationPoint(ctx, db.UpsertPresenceObservationPointParams{
			SourceKey:        source.Key,
			SourceType:       spec.SourceType,
			Medium:           spec.Medium,
			ExternalID:       client.ObservationExternalID,
			ParentExternalID: client.ObservationParentExternalID,
			DisplayName:      client.DisplayName,
			Ssid:             nullableString(client.SSID),
			Metadata:         mustJSON(client.Metadata),
			LastSeenAt:       timestamptz(lastSeen),
		})
		if err != nil {
			return "", err
		}
		observationPointID = observationPoint.ID
	}

	deviceID := s.lookupNetworkDeviceID(ctx, q, clientMAC)
	if err := s.reconcilePolledActiveSession(ctx, q, source, spec, clientMAC, deviceID, observationPointID, client.ObservationExternalID, client.ObservationParentExternalID, client.SSID, firstSeen, lastSeen, pollTime, client.Metadata); err != nil {
		return "", err
	}

	_, err = q.UpsertPresenceClient(ctx, db.UpsertPresenceClientParams{
		MacAddress:             clientMAC,
		NetworkDeviceID:        deviceID,
		Status:                 clientStatusOnline,
		FirstSeenAt:            timestamptz(firstSeen),
		LastSeenAt:             timestamptz(lastSeen),
		LastSourceKey:          source.Key,
		LastSourceType:         spec.SourceType,
		LastMedium:             spec.Medium,
		LastObservationPointID: observationPointID,
		LastSsid:               nullableString(client.SSID),
		LastMetadata:           mustJSON(client.Metadata),
	})
	return clientMAC, err
}

func (s *Service) reconcilePolledActiveSession(
	ctx context.Context,
	q db.Querier,
	source config.PresenceSourceConfig,
	spec polledSourceSpec,
	clientMAC string,
	deviceID pgtype.UUID,
	observationPointID pgtype.UUID,
	observationExternalID string,
	observationParentExternalID string,
	ssid string,
	firstSeen time.Time,
	lastSeen time.Time,
	pollTime time.Time,
	clientMetadata map[string]any,
) error {
	openSession, found, err := s.lookupOpenPresenceSession(ctx, q, clientMAC, spec.Medium)
	if err != nil {
		return err
	}

	if !found {
		return s.createSyntheticPolledSession(ctx, q, source, spec, clientMAC, deviceID, observationPointID, ssid, firstSeen, lastSeen, clientMetadata)
	}

	if sameObservationIdentity(openSession.ObservationExternalID, openSession.ObservationParentExternalID, observationExternalID, observationParentExternalID) {
		updateObservationPointID := pgtype.UUID{}
		if !openSession.ObservationPointID.Valid && observationPointID.Valid {
			updateObservationPointID = observationPointID
		}
		_, err := q.UpdatePresenceSessionActivity(ctx, db.UpdatePresenceSessionActivityParams{
			ID:                 openSession.ID,
			NetworkDeviceID:    deviceID,
			ObservationPointID: updateObservationPointID,
			SourceUpdatedAt:    timestamptz(lastSeen),
			LastSeenAt:         timestamptz(lastSeen),
			Ssid:               nullableString(ssid),
			Metadata:           mustJSON(clientMetadata),
		})
		return err
	}

	closeAt := openSession.LastSeenAt.Time.UTC()
	if closeAt.IsZero() {
		closeAt = lastSeen
	}
	if err := s.closePresenceSession(ctx, q, openPresenceSessionRecordToSession(openSession), deviceID, pollTime, closeAt, ssid, map[string]any{
		"ended_by":            spec.MoveCloseReason,
		"inferred_at":         pollTime.Format(time.RFC3339),
		"inferred_source_key": source.Key,
		"next_observation_id": observationExternalID,
	}); err != nil {
		return err
	}

	return s.createSyntheticPolledSession(ctx, q, source, spec, clientMAC, deviceID, observationPointID, ssid, maxTime(firstSeen, closeAt), lastSeen, clientMetadata)
}

func (s *Service) reconcilePolledDisappearances(ctx context.Context, q db.Querier, source config.PresenceSourceConfig, spec polledSourceSpec, pollTime time.Time, seenClientMACs map[string]struct{}) error {
	cutoff := pollTime.Add(-s.disconnectGrace(source))
	staleClients, err := q.ListStaleOnlinePresenceClientsBySource(ctx, db.ListStaleOnlinePresenceClientsBySourceParams{
		SourceKey:  source.Key,
		SourceType: spec.SourceType,
		Medium:     spec.Medium,
		SeenBefore: timestamptz(cutoff),
	})
	if err != nil {
		return err
	}

	for _, client := range staleClients {
		if _, seen := seenClientMACs[client.MacAddress]; seen {
			continue
		}
		if err := s.markPolledClientOffline(ctx, q, source, spec, pollTime, client); err != nil {
			return err
		}
	}

	return nil
}

func (s *Service) markPolledClientOffline(ctx context.Context, q db.Querier, source config.PresenceSourceConfig, spec polledSourceSpec, pollTime time.Time, client db.PresenceClient) error {
	openSession, found, err := s.lookupOpenPresenceSession(ctx, q, client.MacAddress, spec.Medium)
	if err != nil {
		return err
	}

	closeAt := client.LastSeenAt.Time.UTC()
	if closeAt.IsZero() {
		closeAt = pollTime
	}

	if found {
		if err := s.closePresenceSession(ctx, q, openPresenceSessionRecordToSession(openSession), client.NetworkDeviceID, pollTime, closeAt, ptrString(client.LastSsid), map[string]any{
			"ended_by":            spec.TimeoutCloseReason,
			"inferred_at":         pollTime.Format(time.RFC3339),
			"inferred_source_key": source.Key,
			"disconnect_grace":    s.disconnectGrace(source).String(),
		}); err != nil {
			return err
		}
	}

	_, err = q.UpsertPresenceClient(ctx, db.UpsertPresenceClientParams{
		MacAddress:             client.MacAddress,
		NetworkDeviceID:        client.NetworkDeviceID,
		Status:                 clientStatusOffline,
		FirstSeenAt:            client.FirstSeenAt,
		LastSeenAt:             client.LastSeenAt,
		LastSourceKey:          source.Key,
		LastSourceType:         spec.SourceType,
		LastMedium:             spec.Medium,
		LastObservationPointID: client.LastObservationPointID,
		LastSsid:               client.LastSsid,
		LastMetadata: mustJSON(map[string]any{
			"ended_by":            spec.TimeoutCloseReason,
			"inferred_at":         pollTime.Format(time.RFC3339),
			"inferred_source_key": source.Key,
			"disconnect_grace":    s.disconnectGrace(source).String(),
		}),
	})
	return err
}

func (s *Service) createSyntheticPolledSession(
	ctx context.Context,
	q db.Querier,
	source config.PresenceSourceConfig,
	spec polledSourceSpec,
	clientMAC string,
	deviceID pgtype.UUID,
	observationPointID pgtype.UUID,
	ssid string,
	firstSeen time.Time,
	lastSeen time.Time,
	clientMetadata map[string]any,
) error {
	startedAt := maxTime(firstSeen, time.Time{})
	if startedAt.IsZero() {
		startedAt = lastSeen
	}

	_, err := q.UpsertPresenceSession(ctx, db.UpsertPresenceSessionParams{
		SourceKey:          source.Key,
		SourceType:         spec.SourceType,
		Medium:             spec.Medium,
		SourceSessionKey:   syntheticPresenceSessionKey(spec.SessionPrefix, clientMAC, startedAt),
		ClientMacAddress:   clientMAC,
		NetworkDeviceID:    deviceID,
		ObservationPointID: observationPointID,
		StartedAt:          timestamptz(startedAt),
		SourceUpdatedAt:    timestamptz(lastSeen),
		LastSeenAt:         timestamptz(lastSeen),
		EndedAt:            pgtype.Timestamptz{},
		Ssid:               nullableString(ssid),
		Metadata:           mergeSyntheticSessionMetadata(mustJSON(clientMetadata), spec.SessionOrigin),
	})
	return err
}
