package presence

import (
	"context"
	"fmt"
	"net"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/gosnmp/gosnmp"
	"github.com/sbekti/intern-api/internal/config"
)

const (
	ifNameOID            = ".1.3.6.1.2.1.31.1.1.1.1"
	bridgePortIfIndexOID = ".1.3.6.1.2.1.17.1.4.1.2"
	qBridgeFdbPortOID    = ".1.3.6.1.2.1.17.7.1.2.2.1.2"
	qBridgeFdbStatusOID  = ".1.3.6.1.2.1.17.7.1.2.2.1.3"
	lldpRemoteTableOID   = ".1.0.8802.1.1.2.1.4.1.1"
	dot1qVlanCurrentOID  = ".1.3.6.1.2.1.17.7.1.4.3.1"
)

type JuniperClient interface {
	ListActiveClients(ctx context.Context, source config.PresenceSourceConfig, pollTime time.Time) ([]polledPresenceClient, error)
}

type gosnmpClient interface {
	Connect() error
	WalkAll(rootOid string) ([]gosnmp.SnmpPDU, error)
	Close() error
}

type JuniperSNMPClient struct {
	newClient func(config.PresenceSourceConfig) (gosnmpClient, error)
}

type juniperPortInventory struct {
	PhysicalName string
	LogicalName  string
	LLDPNeighbor map[string]any
}

type juniperFdbEntry struct {
	RawVlanIndex string
	MAC          string
	BridgePort   int
	Status       int
}

func NewJuniperSNMPClient() *JuniperSNMPClient {
	return &JuniperSNMPClient{
		newClient: func(source config.PresenceSourceConfig) (gosnmpClient, error) {
			host := strings.TrimSpace(source.Host)
			if host == "" {
				return nil, fmt.Errorf("juniper source %q missing host", source.Key)
			}
			port := uint16(source.Port)
			if port == 0 {
				port = 161
			}

			authProtocol, err := juniperAuthProtocol(resolveCredentialValue(source.CredentialEnv.SNMPAuthProtocol))
			if err != nil {
				return nil, err
			}
			privProtocol, err := juniperPrivProtocol(resolveCredentialValue(source.CredentialEnv.SNMPPrivProtocol))
			if err != nil {
				return nil, err
			}

			client := &gosnmp.GoSNMP{
				Target:        host,
				Port:          port,
				Version:       gosnmp.Version3,
				Timeout:       15 * time.Second,
				Retries:       1,
				MaxOids:       gosnmp.MaxOids,
				SecurityModel: gosnmp.UserSecurityModel,
				MsgFlags:      gosnmp.AuthPriv,
				SecurityParameters: &gosnmp.UsmSecurityParameters{
					UserName:                 resolveCredentialValue(source.CredentialEnv.SNMPUsername),
					AuthenticationProtocol:   authProtocol,
					AuthenticationPassphrase: resolveCredentialValue(source.CredentialEnv.SNMPAuthPassword),
					PrivacyProtocol:          privProtocol,
					PrivacyPassphrase:        resolveCredentialValue(source.CredentialEnv.SNMPPrivPassword),
				},
			}
			return client, nil
		},
	}
}

func (c *JuniperSNMPClient) ListActiveClients(ctx context.Context, source config.PresenceSourceConfig, pollTime time.Time) ([]polledPresenceClient, error) {
	if c == nil {
		return nil, fmt.Errorf("juniper client not configured")
	}
	client, err := c.newClient(source)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = client.Close()
	}()

	if err := client.Connect(); err != nil {
		return nil, err
	}

	ifNamePDUs, err := client.WalkAll(ifNameOID)
	if err != nil {
		return nil, err
	}
	bridgePortPDUs, err := client.WalkAll(bridgePortIfIndexOID)
	if err != nil {
		return nil, err
	}
	fdbPortPDUs, err := client.WalkAll(qBridgeFdbPortOID)
	if err != nil {
		return nil, err
	}
	fdbStatusPDUs, err := client.WalkAll(qBridgeFdbStatusOID)
	if err != nil {
		return nil, err
	}
	lldpPDUs, err := client.WalkAll(lldpRemoteTableOID)
	if err != nil {
		return nil, err
	}
	vlanPDUs, err := client.WalkAll(dot1qVlanCurrentOID)
	if err != nil {
		return nil, err
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	ifNames := parseIfNamePDUs(ifNamePDUs)
	bridgePorts := parseBridgePortIfIndexPDUs(bridgePortPDUs)
	vlanNames := parseDot1qVlanNames(vlanPDUs)
	lldpNeighbors := parseLLDPRemotePDUs(lldpPDUs)
	fdbEntries := parseJuniperFdbEntries(fdbPortPDUs, fdbStatusPDUs)

	portInventory := buildJuniperPortInventory(ifNames, lldpNeighbors)
	fdbByIfIndex := groupFdbEntriesByIfIndex(fdbEntries, bridgePorts)

	clients := make([]polledPresenceClient, 0, len(fdbByIfIndex))
	for ifIndex, entries := range fdbByIfIndex {
		port, ok := portInventory[ifIndex]
		if !ok {
			continue
		}
		if strings.TrimSpace(port.PhysicalName) == "" {
			continue
		}
		if len(port.LLDPNeighbor) > 0 {
			continue
		}

		macSet := make(map[string]struct{}, len(entries))
		vlanSet := make(map[string]struct{}, len(entries))
		for _, entry := range entries {
			macSet[entry.MAC] = struct{}{}
			vlanSet[entry.RawVlanIndex] = struct{}{}
		}
		if len(macSet) != 1 || len(vlanSet) != 1 {
			continue
		}

		macAddress := firstSortedKey(macSet)
		rawVlanIndex := firstSortedKey(vlanSet)
		vlanName := vlanNames[rawVlanIndex]
		metadata := map[string]any{
			"interface_name":         port.PhysicalName,
			"logical_interface_name": port.LogicalName,
			"selection_reason":       "single_mac_non_lldp_port",
			"candidate_mac_count":    len(macSet),
			"candidate_vlan_count":   len(vlanSet),
			"fdb_vlan_index":         rawVlanIndex,
			"vlan_name":              vlanName,
			"last_seen_source_key":   source.Key,
			"last_seen_source_type":  sourceTypeJuniperSNMP,
		}

		clients = append(clients, polledPresenceClient{
			MAC:                   macAddress,
			DisplayName:           port.PhysicalName,
			ObservationExternalID: port.PhysicalName,
			FirstSeen:             pollTime,
			LastSeen:              pollTime,
			Metadata:              metadata,
		})
	}

	slices.SortFunc(clients, func(a polledPresenceClient, b polledPresenceClient) int {
		return strings.Compare(strings.ToLower(a.MAC), strings.ToLower(b.MAC))
	})
	return clients, nil
}

func parseIfNamePDUs(pdus []gosnmp.SnmpPDU) map[int]string {
	result := make(map[int]string, len(pdus))
	for _, pdu := range pdus {
		indexes, ok := oidSuffixInts(pdu.Name, ifNameOID, 1)
		if !ok {
			continue
		}
		value := strings.TrimSpace(pduValueString(pdu))
		if value == "" {
			continue
		}
		result[indexes[0]] = value
	}
	return result
}

func parseBridgePortIfIndexPDUs(pdus []gosnmp.SnmpPDU) map[int]int {
	result := make(map[int]int, len(pdus))
	for _, pdu := range pdus {
		indexes, ok := oidSuffixInts(pdu.Name, bridgePortIfIndexOID, 1)
		if !ok {
			continue
		}
		ifIndex, ok := pduValueInt(pdu)
		if !ok {
			continue
		}
		result[indexes[0]] = ifIndex
	}
	return result
}

func parseJuniperFdbEntries(portPDUs []gosnmp.SnmpPDU, statusPDUs []gosnmp.SnmpPDU) []juniperFdbEntry {
	statusByKey := make(map[string]int, len(statusPDUs))
	for _, pdu := range statusPDUs {
		indexes, ok := oidSuffixInts(pdu.Name, qBridgeFdbStatusOID, 7)
		if !ok {
			continue
		}
		status, ok := pduValueInt(pdu)
		if !ok {
			continue
		}
		statusByKey[fdbKey(indexes)] = status
	}

	entries := make([]juniperFdbEntry, 0, len(portPDUs))
	for _, pdu := range portPDUs {
		indexes, ok := oidSuffixInts(pdu.Name, qBridgeFdbPortOID, 7)
		if !ok {
			continue
		}
		bridgePort, ok := pduValueInt(pdu)
		if !ok || bridgePort <= 0 {
			continue
		}
		status := statusByKey[fdbKey(indexes)]
		if status != 3 {
			continue
		}

		macAddress := bytesToMAC(indexes[1:])
		if macAddress == "" {
			continue
		}

		entries = append(entries, juniperFdbEntry{
			RawVlanIndex: strconv.Itoa(indexes[0]),
			MAC:          macAddress,
			BridgePort:   bridgePort,
			Status:       status,
		})
	}
	return entries
}

func parseLLDPRemotePDUs(pdus []gosnmp.SnmpPDU) map[int]map[string]any {
	result := make(map[int]map[string]any)
	for _, pdu := range pdus {
		indexes, ok := oidSuffixIntsAfterColumn(pdu.Name, lldpRemoteTableOID, 3)
		if !ok {
			continue
		}
		column := indexes[0]
		localIfIndex := indexes[1]
		record := result[localIfIndex]
		if record == nil {
			record = map[string]any{}
			result[localIfIndex] = record
		}

		switch column {
		case 5:
			record["lldp_remote_chassis_id"] = strings.ToLower(net.HardwareAddr(pduValueBytes(pdu)).String())
		case 8:
			record["lldp_remote_port_id"] = strings.TrimSpace(pduValueString(pdu))
		case 9:
			record["lldp_remote_system_name"] = strings.TrimSpace(pduValueString(pdu))
		case 10:
			record["lldp_remote_system_description"] = strings.TrimSpace(pduValueString(pdu))
		}
	}
	return result
}

func parseDot1qVlanNames(pdus []gosnmp.SnmpPDU) map[string]string {
	result := make(map[string]string)
	for _, pdu := range pdus {
		indexes, ok := oidSuffixIntsAfterColumn(pdu.Name, dot1qVlanCurrentOID, 2)
		if !ok {
			continue
		}
		column := indexes[0]
		if column != 1 {
			continue
		}
		result[strconv.Itoa(indexes[1])] = strings.TrimSpace(pduValueString(pdu))
	}
	return result
}

func buildJuniperPortInventory(ifNames map[int]string, lldpNeighbors map[int]map[string]any) map[int]juniperPortInventory {
	logicalByBase := make(map[string]string)
	for _, name := range ifNames {
		if strings.HasSuffix(name, ".0") {
			logicalByBase[strings.TrimSuffix(name, ".0")] = name
		}
	}

	result := make(map[int]juniperPortInventory)
	for ifIndex, name := range ifNames {
		if strings.Contains(name, ".") {
			continue
		}
		result[ifIndex] = juniperPortInventory{
			PhysicalName: name,
			LogicalName:  logicalByBase[name],
			LLDPNeighbor: lldpNeighbors[ifIndex],
		}
	}
	return result
}

func groupFdbEntriesByIfIndex(entries []juniperFdbEntry, bridgePorts map[int]int) map[int][]juniperFdbEntry {
	result := make(map[int][]juniperFdbEntry)
	for _, entry := range entries {
		ifIndex, ok := bridgePorts[entry.BridgePort]
		if !ok {
			continue
		}
		result[ifIndex] = append(result[ifIndex], entry)
	}
	return result
}

func juniperAuthProtocol(raw string) (gosnmp.SnmpV3AuthProtocol, error) {
	switch strings.ToUpper(strings.TrimSpace(raw)) {
	case "MD5":
		return gosnmp.MD5, nil
	case "SHA", "SHA1":
		return gosnmp.SHA, nil
	case "SHA224":
		return gosnmp.SHA224, nil
	case "SHA256":
		return gosnmp.SHA256, nil
	case "SHA384":
		return gosnmp.SHA384, nil
	case "SHA512":
		return gosnmp.SHA512, nil
	default:
		return gosnmp.NoAuth, fmt.Errorf("unsupported juniper snmp auth protocol %q", raw)
	}
}

func juniperPrivProtocol(raw string) (gosnmp.SnmpV3PrivProtocol, error) {
	switch strings.ToUpper(strings.TrimSpace(raw)) {
	case "DES":
		return gosnmp.DES, nil
	case "AES", "AES128":
		return gosnmp.AES, nil
	case "AES192":
		return gosnmp.AES192, nil
	case "AES256":
		return gosnmp.AES256, nil
	default:
		return gosnmp.NoPriv, fmt.Errorf("unsupported juniper snmp privacy protocol %q", raw)
	}
}

func oidSuffixInts(oid string, prefix string, expected int) ([]int, bool) {
	trimmed := strings.TrimPrefix(strings.TrimSpace(oid), prefix)
	trimmed = strings.TrimPrefix(trimmed, ".")
	if trimmed == "" {
		return nil, false
	}
	parts := strings.Split(trimmed, ".")
	if len(parts) != expected {
		return nil, false
	}
	values := make([]int, 0, len(parts))
	for _, part := range parts {
		parsed, err := strconv.Atoi(part)
		if err != nil {
			return nil, false
		}
		values = append(values, parsed)
	}
	return values, true
}

func oidSuffixIntsAfterColumn(oid string, prefix string, minimum int) ([]int, bool) {
	trimmed := strings.TrimPrefix(strings.TrimSpace(oid), prefix)
	trimmed = strings.TrimPrefix(trimmed, ".")
	if trimmed == "" {
		return nil, false
	}
	parts := strings.Split(trimmed, ".")
	if len(parts) < minimum {
		return nil, false
	}
	values := make([]int, 0, len(parts))
	for _, part := range parts {
		parsed, err := strconv.Atoi(part)
		if err != nil {
			return nil, false
		}
		values = append(values, parsed)
	}
	return values, true
}

func fdbKey(indexes []int) string {
	parts := make([]string, 0, len(indexes))
	for _, value := range indexes {
		parts = append(parts, strconv.Itoa(value))
	}
	return strings.Join(parts, ".")
}

func pduValueInt(pdu gosnmp.SnmpPDU) (int, bool) {
	switch value := gosnmp.ToBigInt(pdu.Value); {
	case value == nil:
		return 0, false
	default:
		return int(value.Int64()), true
	}
}

func pduValueString(pdu gosnmp.SnmpPDU) string {
	switch value := pdu.Value.(type) {
	case string:
		return value
	case []byte:
		return string(value)
	default:
		return fmt.Sprint(value)
	}
}

func pduValueBytes(pdu gosnmp.SnmpPDU) []byte {
	switch value := pdu.Value.(type) {
	case string:
		return []byte(value)
	case []byte:
		return value
	default:
		return nil
	}
}

func bytesToMAC(values []int) string {
	if len(values) != 6 {
		return ""
	}
	buffer := make([]byte, 0, len(values))
	for _, value := range values {
		buffer = append(buffer, byte(value))
	}
	return strings.ToLower(net.HardwareAddr(buffer).String())
}

func firstSortedKey(values map[string]struct{}) string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	if len(keys) == 0 {
		return ""
	}
	return keys[0]
}
