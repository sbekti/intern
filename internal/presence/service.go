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
	for _, client := range activeClients {
		if err := s.applyUniFiClient(ctx, txQueries, source, client); err != nil {
			_ = tx.Rollback(ctx)
			return err
		}
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
		"radacct_id":           row.Radacctid,
		"acct_session_id":      row.Acctsessionid,
		"called_station_id":    ptrString(row.Calledstationid),
		"calling_station_id":   ptrString(row.Callingstationid),
		"acct_terminate_cause": ptrString(row.Acctterminatecause),
		"nas_ip_address":       row.Nasipaddress.String(),
		"nas_port_id":          ptrString(row.Nasportid),
		"username":             ptrString(row.Username),
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

func (s *Service) applyUniFiClient(ctx context.Context, q db.Querier, source config.PresenceSourceConfig, client UniFiActiveClient) error {
	clientMAC, err := normalizedColonMAC(client.MAC)
	if err != nil {
		s.logger.Warn("skipping unifi client with invalid mac", "source_key", source.Key, "mac", client.MAC, "error", err)
		return nil
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

	if observationExternalID != "" {
		parentExternalID := ""
		if apMAC != "" && apMAC != observationExternalID {
			parentExternalID = apMAC
		}

		observationPoint, err := q.UpsertPresenceObservationPoint(ctx, db.UpsertPresenceObservationPointParams{
			SourceKey:        source.Key,
			SourceType:       sourceTypeUnifi,
			Medium:           mediumWireless,
			ExternalID:       observationExternalID,
			ParentExternalID: parentExternalID,
			DisplayName:      strings.TrimSpace(client.Hostname),
			Ssid:             nullableString(client.ESSID),
			Metadata: mustJSON(map[string]any{
				"ap_mac":   apMAC,
				"bssid":    bssid,
				"hostname": client.Hostname,
			}),
			LastSeenAt: timestamptz(lastSeen),
		})
		if err != nil {
			return err
		}
		observationPointID = observationPoint.ID
	}

	clientMetadata := mustJSON(map[string]any{
		"ap_mac":     apMAC,
		"bssid":      bssid,
		"hostname":   client.Hostname,
		"assoc_time": firstSeen.Format(time.RFC3339),
		"last_seen":  lastSeen.Format(time.RFC3339),
	})

	_, err = q.UpsertPresenceClient(ctx, db.UpsertPresenceClientParams{
		MacAddress:             clientMAC,
		NetworkDeviceID:        s.lookupNetworkDeviceID(ctx, q, clientMAC),
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

func (s *Service) lookupNetworkDeviceID(ctx context.Context, q db.Querier, macAddress string) pgtype.UUID {
	device, err := q.GetNetworkDeviceByMACAddress(ctx, db.GetNetworkDeviceByMACAddressParams{MacAddress: macAddress})
	if err != nil {
		return pgtype.UUID{}
	}
	return device.ID
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
