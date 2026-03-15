package presence

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sbekti/intern-api/internal/config"
	"github.com/sbekti/intern-api/internal/db"
	"github.com/sbekti/intern-api/internal/netnormalize"
)

const (
	radiusSourceKey = "radius"

	sourceTypeRadius = "radius"
	sourceTypeUnifi  = "unifi"

	mediumWireless = "wireless"

	clientStatusOnline  = "online"
	clientStatusOffline = "offline"

	workerNameRadiusSync = "wireless-radius-sync"
	workerNameUnifiSync  = "wireless-unifi-sync"
)

type UniFiClient interface {
	ListActiveClients(ctx context.Context, source config.PresenceSourceConfig) ([]UniFiActiveClient, error)
}

type Service struct {
	logger           *slog.Logger
	pool             *pgxpool.Pool
	cfg              config.PresenceConfig
	unifiClient      UniFiClient
	now              func() time.Time
	radacctBatchSize int32
}

type radiusWorkerState struct {
	LastRadacctID int64 `json:"last_radacct_id"`
}

type UniFiActiveClient struct {
	MAC       string
	Hostname  string
	ESSID     string
	APMAC     string
	BSSID     string
	AssocTime time.Time
	LastSeen  time.Time
}

func NewService(logger *slog.Logger, pool *pgxpool.Pool, cfg config.PresenceConfig, unifiClient UniFiClient) *Service {
	if logger == nil {
		logger = slog.Default()
	}
	if unifiClient == nil {
		unifiClient = NewUniFiHTTPClient(nil)
	}

	return &Service{
		logger:           logger,
		pool:             pool,
		cfg:              cfg,
		unifiClient:      unifiClient,
		now:              time.Now,
		radacctBatchSize: 250,
	}
}

func (s *Service) SyncWirelessOnce(ctx context.Context) error {
	if err := s.SyncRadius(ctx); err != nil {
		return err
	}

	for _, source := range s.cfg.Sources {
		if source.Type != config.PresenceSourceTypeUnifi {
			continue
		}
		if err := s.SyncUniFiSource(ctx, source); err != nil {
			return err
		}
	}

	return nil
}

func (s *Service) SyncRadius(ctx context.Context) error {
	if s == nil || s.pool == nil {
		return fmt.Errorf("presence pool not configured")
	}

	queries := db.New(s.pool)
	pollTime := s.now().UTC()

	state, err := s.loadRadiusState(ctx, queries)
	if err != nil {
		return err
	}

	lastID := state.LastRadacctID
	for {
		rows, err := queries.ListRadacctRowsAfterID(ctx, db.ListRadacctRowsAfterIDParams{
			AfterRadacctID: lastID,
			LimitCount:     s.radacctBatchSize,
		})
		if err != nil {
			return err
		}
		if len(rows) == 0 {
			_, err := queries.UpsertPresenceWorkerState(ctx, db.UpsertPresenceWorkerStateParams{
				WorkerName:      workerNameRadiusSync,
				SourceKey:       radiusSourceKey,
				State:           mustJSON(state),
				LastPolledAt:    timestamptz(pollTime),
				LastSucceededAt: timestamptz(pollTime),
			})
			return err
		}

		tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
		if err != nil {
			return err
		}

		txQueries := db.New(tx)
		for _, row := range rows {
			if err := s.applyRadiusRow(ctx, txQueries, row); err != nil {
				_ = tx.Rollback(ctx)
				return err
			}
			lastID = row.Radacctid
		}

		state.LastRadacctID = lastID
		if _, err := txQueries.UpsertPresenceWorkerState(ctx, db.UpsertPresenceWorkerStateParams{
			WorkerName:      workerNameRadiusSync,
			SourceKey:       radiusSourceKey,
			State:           mustJSON(state),
			LastPolledAt:    timestamptz(pollTime),
			LastSucceededAt: timestamptz(pollTime),
		}); err != nil {
			_ = tx.Rollback(ctx)
			return err
		}

		if err := tx.Commit(ctx); err != nil {
			return err
		}

		if int32(len(rows)) < s.radacctBatchSize {
			return nil
		}
	}
}

func (s *Service) SyncUniFiSource(ctx context.Context, source config.PresenceSourceConfig) error {
	if s == nil || s.pool == nil {
		return fmt.Errorf("presence pool not configured")
	}

	activeClients, err := s.unifiClient.ListActiveClients(ctx, source)
	if err != nil {
		return err
	}

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}

	pollTime := s.now().UTC()
	txQueries := db.New(tx)
	seenClientMACs := make(map[string]struct{}, len(activeClients))
	for _, client := range activeClients {
		clientMAC, err := s.applyUniFiClient(ctx, txQueries, source, client, pollTime)
		if err != nil {
			_ = tx.Rollback(ctx)
			return err
		}
		if clientMAC != "" {
			seenClientMACs[clientMAC] = struct{}{}
		}
	}

	if err := s.reconcileUniFiDisappearances(ctx, txQueries, source, pollTime, seenClientMACs); err != nil {
		_ = tx.Rollback(ctx)
		return err
	}

	state := map[string]any{"last_client_count": len(activeClients)}
	if _, err := txQueries.UpsertPresenceWorkerState(ctx, db.UpsertPresenceWorkerStateParams{
		WorkerName:      workerNameUnifiSync,
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

func (s *Service) applyRadiusRow(ctx context.Context, q db.Querier, row db.Radacct) error {
	clientMAC, err := normalizedColonMAC(ptrString(row.Callingstationid))
	if err != nil {
		s.logger.Warn("skipping radius row with invalid callingstationid", "radacct_id", row.Radacctid, "callingstationid", ptrString(row.Callingstationid), "error", err)
		return nil
	}

	startedAt := firstValidTime(row.Acctstarttime, row.Acctupdatetime, row.Acctstoptime, timestamptz(s.now().UTC()))
	lastSeenAt := firstValidTime(row.Acctstoptime, row.Acctupdatetime, row.Acctstarttime, timestamptz(s.now().UTC()))
	sourceUpdatedAt := firstValidTime(row.Acctupdatetime, row.Acctstoptime, row.Acctstarttime, timestamptz(s.now().UTC()))

	bssid, ssid := parseCalledStationID(ptrString(row.Calledstationid))
	observationPointID := pgtype.UUID{}
	if bssid != "" {
		observationPoint, err := q.UpsertPresenceObservationPoint(ctx, db.UpsertPresenceObservationPointParams{
			SourceKey:        radiusSourceKey,
			SourceType:       sourceTypeRadius,
			Medium:           mediumWireless,
			ExternalID:       bssid,
			ParentExternalID: "",
			DisplayName:      bssid,
			Ssid:             nullableString(ssid),
			Metadata: mustJSON(map[string]any{
				"called_station_id": ptrString(row.Calledstationid),
				"nas_ip_address":    row.Nasipaddress.String(),
			}),
			LastSeenAt: lastSeenAt,
		})
		if err != nil {
			return err
		}
		observationPointID = observationPoint.ID
	}

	deviceID := s.lookupNetworkDeviceID(ctx, q, clientMAC)
	sessionMetadata := mustJSON(map[string]any{
		"session_origin":        "radius",
		"accounting_backed":     true,
		"radacct_id":            row.Radacctid,
		"acct_session_id":       row.Acctsessionid,
		"called_station_id":     ptrString(row.Calledstationid),
		"calling_station_id":    ptrString(row.Callingstationid),
		"acct_terminate_cause":  ptrString(row.Acctterminatecause),
		"nas_ip_address":        row.Nasipaddress.String(),
		"nas_port_id":           ptrString(row.Nasportid),
		"last_seen_source_key":  radiusSourceKey,
		"last_seen_source_type": sourceTypeRadius,
		"username":              ptrString(row.Username),
	})

	if _, err := q.UpsertPresenceSession(ctx, db.UpsertPresenceSessionParams{
		SourceKey:          radiusSourceKey,
		SourceType:         sourceTypeRadius,
		Medium:             mediumWireless,
		SourceSessionKey:   row.Acctuniqueid,
		ClientMacAddress:   clientMAC,
		NetworkDeviceID:    deviceID,
		ObservationPointID: observationPointID,
		StartedAt:          startedAt,
		SourceUpdatedAt:    sourceUpdatedAt,
		LastSeenAt:         lastSeenAt,
		EndedAt:            row.Acctstoptime,
		Ssid:               nullableString(ssid),
		Metadata:           sessionMetadata,
	}); err != nil {
		return err
	}

	status := clientStatusOnline
	if row.Acctstoptime.Valid {
		status = clientStatusOffline
	}

	_, err = q.UpsertPresenceClient(ctx, db.UpsertPresenceClientParams{
		MacAddress:             clientMAC,
		NetworkDeviceID:        deviceID,
		Status:                 status,
		FirstSeenAt:            startedAt,
		LastSeenAt:             lastSeenAt,
		LastSourceKey:          radiusSourceKey,
		LastSourceType:         sourceTypeRadius,
		LastMedium:             mediumWireless,
		LastObservationPointID: observationPointID,
		LastSsid:               nullableString(ssid),
		LastMetadata:           sessionMetadata,
	})
	return err
}

func (s *Service) applyUniFiClient(ctx context.Context, q db.Querier, source config.PresenceSourceConfig, client UniFiActiveClient, pollTime time.Time) (string, error) {
	clientMAC, err := normalizedColonMAC(client.MAC)
	if err != nil {
		s.logger.Warn("skipping unifi client with invalid mac", "source_key", source.Key, "mac", client.MAC, "error", err)
		return "", nil
	}

	lastSeen := client.LastSeen.UTC()
	if lastSeen.IsZero() {
		lastSeen = client.AssocTime.UTC()
	}
	if lastSeen.IsZero() {
		lastSeen = s.now().UTC()
	}

	firstSeen := client.AssocTime.UTC()
	if firstSeen.IsZero() {
		firstSeen = lastSeen
	}

	bssid, _ := normalizedOptionalMAC(client.BSSID)
	apMAC, _ := normalizedOptionalMAC(client.APMAC)

	observationPointID := pgtype.UUID{}
	observationExternalID := bssid
	if observationExternalID == "" {
		observationExternalID = apMAC
	}
	observationParentExternalID := ""

	if observationExternalID != "" {
		if apMAC != "" && apMAC != observationExternalID {
			observationParentExternalID = apMAC
		}

		observationPoint, err := q.UpsertPresenceObservationPoint(ctx, db.UpsertPresenceObservationPointParams{
			SourceKey:        source.Key,
			SourceType:       sourceTypeUnifi,
			Medium:           mediumWireless,
			ExternalID:       observationExternalID,
			ParentExternalID: observationParentExternalID,
			DisplayName:      strings.TrimSpace(client.Hostname),
			Ssid:             nullableString(client.ESSID),
			Metadata:         mustJSON(buildUniFiMetadata(source.Key, client, bssid, apMAC, firstSeen, lastSeen)),
			LastSeenAt:       timestamptz(lastSeen),
		})
		if err != nil {
			return "", err
		}
		observationPointID = observationPoint.ID
	}

	clientMetadata := mustJSON(buildUniFiMetadata(source.Key, client, bssid, apMAC, firstSeen, lastSeen))
	deviceID := s.lookupNetworkDeviceID(ctx, q, clientMAC)

	if err := s.reconcileUniFiActiveSession(ctx, q, source, clientMAC, deviceID, observationPointID, observationExternalID, observationParentExternalID, client.ESSID, firstSeen, lastSeen, pollTime, clientMetadata); err != nil {
		return "", err
	}

	_, err = q.UpsertPresenceClient(ctx, db.UpsertPresenceClientParams{
		MacAddress:             clientMAC,
		NetworkDeviceID:        deviceID,
		Status:                 clientStatusOnline,
		FirstSeenAt:            timestamptz(firstSeen),
		LastSeenAt:             timestamptz(lastSeen),
		LastSourceKey:          source.Key,
		LastSourceType:         sourceTypeUnifi,
		LastMedium:             mediumWireless,
		LastObservationPointID: observationPointID,
		LastSsid:               nullableString(client.ESSID),
		LastMetadata:           clientMetadata,
	})
	return clientMAC, err
}

func (s *Service) reconcileUniFiActiveSession(
	ctx context.Context,
	q db.Querier,
	source config.PresenceSourceConfig,
	clientMAC string,
	deviceID pgtype.UUID,
	observationPointID pgtype.UUID,
	observationExternalID string,
	observationParentExternalID string,
	ssid string,
	firstSeen time.Time,
	lastSeen time.Time,
	pollTime time.Time,
	clientMetadata []byte,
) error {
	openSession, found, err := s.lookupOpenPresenceSession(ctx, q, clientMAC, mediumWireless)
	if err != nil {
		return err
	}

	if !found {
		return s.createSyntheticUniFiSession(ctx, q, source, clientMAC, deviceID, observationPointID, ssid, firstSeen, lastSeen, clientMetadata)
	}

	if sameWirelessObservation(openSession.ObservationExternalID, openSession.ObservationParentExternalID, observationExternalID, observationParentExternalID) {
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
			Metadata:           clientMetadata,
		})
		return err
	}

	closeAt := openSession.LastSeenAt.Time.UTC()
	if closeAt.IsZero() {
		closeAt = lastSeen
	}
	if err := s.closePresenceSession(ctx, q, openPresenceSessionRecordToSession(openSession), deviceID, pollTime, closeAt, ssid, map[string]any{
		"ended_by":            "unifi_roam",
		"inferred_at":         pollTime.Format(time.RFC3339),
		"inferred_source_key": source.Key,
		"next_observation_id": observationExternalID,
	}); err != nil {
		return err
	}

	return s.createSyntheticUniFiSession(ctx, q, source, clientMAC, deviceID, observationPointID, ssid, maxTime(firstSeen, closeAt), lastSeen, clientMetadata)
}

func (s *Service) reconcileUniFiDisappearances(ctx context.Context, q db.Querier, source config.PresenceSourceConfig, pollTime time.Time, seenClientMACs map[string]struct{}) error {
	cutoff := pollTime.Add(-s.disconnectGrace(source))
	staleClients, err := q.ListStaleOnlinePresenceClientsBySource(ctx, db.ListStaleOnlinePresenceClientsBySourceParams{
		SourceKey:  source.Key,
		SourceType: sourceTypeUnifi,
		Medium:     mediumWireless,
		SeenBefore: timestamptz(cutoff),
	})
	if err != nil {
		return err
	}

	for _, client := range staleClients {
		if _, seen := seenClientMACs[strings.ToLower(client.MacAddress)]; seen {
			continue
		}
		if err := s.markUniFiClientOffline(ctx, q, source, pollTime, client); err != nil {
			return err
		}
	}

	return nil
}

func (s *Service) markUniFiClientOffline(ctx context.Context, q db.Querier, source config.PresenceSourceConfig, pollTime time.Time, client db.PresenceClient) error {
	openSession, found, err := s.lookupOpenPresenceSession(ctx, q, client.MacAddress, mediumWireless)
	if err != nil {
		return err
	}

	closeAt := client.LastSeenAt.Time.UTC()
	if closeAt.IsZero() {
		closeAt = pollTime
	}

	if found {
		if err := s.closePresenceSession(ctx, q, openPresenceSessionRecordToSession(openSession), client.NetworkDeviceID, pollTime, closeAt, ptrString(client.LastSsid), map[string]any{
			"ended_by":            "unifi_poll_timeout",
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
		LastSourceType:         sourceTypeUnifi,
		LastMedium:             mediumWireless,
		LastObservationPointID: client.LastObservationPointID,
		LastSsid:               client.LastSsid,
		LastMetadata: mustJSON(map[string]any{
			"ended_by":            "unifi_poll_timeout",
			"inferred_at":         pollTime.Format(time.RFC3339),
			"inferred_source_key": source.Key,
			"disconnect_grace":    s.disconnectGrace(source).String(),
		}),
	})
	return err
}

func (s *Service) loadRadiusState(ctx context.Context, q db.Querier) (radiusWorkerState, error) {
	record, err := q.GetPresenceWorkerState(ctx, db.GetPresenceWorkerStateParams{
		WorkerName: workerNameRadiusSync,
		SourceKey:  radiusSourceKey,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return radiusWorkerState{}, nil
		}
		return radiusWorkerState{}, err
	}

	var state radiusWorkerState
	if len(record.State) == 0 {
		return state, nil
	}
	if err := json.Unmarshal(record.State, &state); err != nil {
		return radiusWorkerState{}, err
	}
	return state, nil
}

func (s *Service) lookupOpenPresenceSession(ctx context.Context, q db.Querier, clientMAC string, medium string) (db.GetLatestOpenPresenceSessionForClientMediumRow, bool, error) {
	record, err := q.GetLatestOpenPresenceSessionForClientMedium(ctx, db.GetLatestOpenPresenceSessionForClientMediumParams{
		ClientMacAddress: clientMAC,
		Medium:           medium,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.GetLatestOpenPresenceSessionForClientMediumRow{}, false, nil
		}
		return db.GetLatestOpenPresenceSessionForClientMediumRow{}, false, err
	}
	return record, true, nil
}

func (s *Service) lookupNetworkDeviceID(ctx context.Context, q db.Querier, macAddress string) pgtype.UUID {
	device, err := q.GetNetworkDeviceByMACAddress(ctx, db.GetNetworkDeviceByMACAddressParams{MacAddress: macAddress})
	if err != nil {
		return pgtype.UUID{}
	}
	return device.ID
}

func (s *Service) createSyntheticUniFiSession(
	ctx context.Context,
	q db.Querier,
	source config.PresenceSourceConfig,
	clientMAC string,
	deviceID pgtype.UUID,
	observationPointID pgtype.UUID,
	ssid string,
	firstSeen time.Time,
	lastSeen time.Time,
	clientMetadata []byte,
) error {
	startedAt := maxTime(firstSeen, time.Time{})
	if startedAt.IsZero() {
		startedAt = lastSeen
	}

	_, err := q.UpsertPresenceSession(ctx, db.UpsertPresenceSessionParams{
		SourceKey:          source.Key,
		SourceType:         sourceTypeUnifi,
		Medium:             mediumWireless,
		SourceSessionKey:   syntheticPresenceSessionKey("unifi", clientMAC, startedAt),
		ClientMacAddress:   clientMAC,
		NetworkDeviceID:    deviceID,
		ObservationPointID: observationPointID,
		StartedAt:          timestamptz(startedAt),
		SourceUpdatedAt:    timestamptz(lastSeen),
		LastSeenAt:         timestamptz(lastSeen),
		EndedAt:            pgtype.Timestamptz{},
		Ssid:               nullableString(ssid),
		Metadata:           mergeSyntheticSessionMetadata(clientMetadata, "unifi_poll"),
	})
	return err
}

func (s *Service) closePresenceSession(
	ctx context.Context,
	q db.Querier,
	session db.PresenceSession,
	deviceID pgtype.UUID,
	pollTime time.Time,
	closeAt time.Time,
	ssid string,
	metadata map[string]any,
) error {
	_, err := q.ClosePresenceSession(ctx, db.ClosePresenceSessionParams{
		ID:                 session.ID,
		NetworkDeviceID:    deviceID,
		ObservationPointID: pgtype.UUID{},
		SourceUpdatedAt:    timestamptz(pollTime),
		LastSeenAt:         timestamptz(closeAt),
		EndedAt:            timestamptz(closeAt),
		Ssid:               nullableString(ssid),
		Metadata:           mustJSON(metadata),
	})
	return err
}

func (s *Service) disconnectGrace(source config.PresenceSourceConfig) time.Duration {
	if source.DisconnectGrace > 0 {
		return source.DisconnectGrace
	}
	if s.cfg.DisconnectGraceDefault > 0 {
		return s.cfg.DisconnectGraceDefault
	}
	return 15 * time.Minute
}

func parseCalledStationID(raw string) (bssid string, ssid string) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", ""
	}

	head, tail, found := strings.Cut(raw, ":")
	normalized, err := normalizedColonMAC(head)
	if err != nil {
		return "", ""
	}

	if !found {
		return normalized, ""
	}
	return normalized, strings.TrimSpace(tail)
}

func normalizedOptionalMAC(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	return normalizedColonMAC(raw)
}

func normalizedColonMAC(raw string) (string, error) {
	_, colon, err := netnormalize.NormalizeMAC(raw)
	return colon, err
}

func ptrString(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

func nullableString(value string) *string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func mustJSON(value any) []byte {
	payload, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return payload
}

func buildUniFiMetadata(sourceKey string, client UniFiActiveClient, bssid string, apMAC string, firstSeen time.Time, lastSeen time.Time) map[string]any {
	return map[string]any{
		"ap_mac":                apMAC,
		"bssid":                 bssid,
		"hostname":              client.Hostname,
		"assoc_time":            firstSeen.Format(time.RFC3339),
		"last_seen":             lastSeen.Format(time.RFC3339),
		"last_seen_source_key":  sourceKey,
		"last_seen_source_type": sourceTypeUnifi,
	}
}

func mergeSyntheticSessionMetadata(clientMetadata []byte, sessionOrigin string) []byte {
	payload := map[string]any{
		"session_origin":    sessionOrigin,
		"accounting_backed": false,
	}
	if len(clientMetadata) > 0 {
		var decoded map[string]any
		if err := json.Unmarshal(clientMetadata, &decoded); err == nil {
			for key, value := range decoded {
				payload[key] = value
			}
		}
	}
	return mustJSON(payload)
}

func sameWirelessObservation(existingExternalID string, existingParentExternalID string, currentExternalID string, currentParentExternalID string) bool {
	existingIDs := []string{strings.TrimSpace(existingExternalID), strings.TrimSpace(existingParentExternalID)}
	currentIDs := []string{strings.TrimSpace(currentExternalID), strings.TrimSpace(currentParentExternalID)}

	for _, existing := range existingIDs {
		if existing == "" {
			continue
		}
		for _, current := range currentIDs {
			if current != "" && strings.EqualFold(existing, current) {
				return true
			}
		}
	}

	return existingIDs[0] == "" && existingIDs[1] == "" && currentIDs[0] == "" && currentIDs[1] == ""
}

func syntheticPresenceSessionKey(prefix string, clientMAC string, startedAt time.Time) string {
	return fmt.Sprintf("%s:%s:%s", prefix, strings.ToLower(clientMAC), startedAt.UTC().Format(time.RFC3339Nano))
}

func maxTime(a time.Time, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
}

func openPresenceSessionRecordToSession(record db.GetLatestOpenPresenceSessionForClientMediumRow) db.PresenceSession {
	return db.PresenceSession{
		ID:                 record.ID,
		SourceKey:          record.SourceKey,
		SourceType:         record.SourceType,
		Medium:             record.Medium,
		SourceSessionKey:   record.SourceSessionKey,
		ClientMacAddress:   record.ClientMacAddress,
		NetworkDeviceID:    record.NetworkDeviceID,
		ObservationPointID: record.ObservationPointID,
		StartedAt:          record.StartedAt,
		SourceUpdatedAt:    record.SourceUpdatedAt,
		LastSeenAt:         record.LastSeenAt,
		EndedAt:            record.EndedAt,
		Ssid:               record.Ssid,
		Metadata:           record.Metadata,
		CreatedAt:          record.CreatedAt,
		UpdatedAt:          record.UpdatedAt,
	}
}

func timestamptz(value time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: value.UTC(), Valid: true}
}

func firstValidTime(values ...pgtype.Timestamptz) pgtype.Timestamptz {
	for _, value := range values {
		if value.Valid {
			return value
		}
	}
	return pgtype.Timestamptz{}
}
