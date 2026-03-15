package presence

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/gosnmp/gosnmp"
	"github.com/sbekti/intern-api/internal/config"
)

type fakeGoSNMPClient struct {
	pdus map[string][]gosnmp.SnmpPDU
	errs map[string]error
}

func (f *fakeGoSNMPClient) Connect() error { return nil }
func (f *fakeGoSNMPClient) Close() error   { return nil }
func (f *fakeGoSNMPClient) WalkAll(rootOID string) ([]gosnmp.SnmpPDU, error) {
	if err := f.errs[rootOID]; err != nil {
		return nil, err
	}
	return append([]gosnmp.SnmpPDU(nil), f.pdus[rootOID]...), nil
}

func TestJuniperSNMPClientListActiveClientsFiltersToDirectAccessPorts(t *testing.T) {
	t.Parallel()

	client := &JuniperSNMPClient{
		newClient: func(source config.PresenceSourceConfig) (gosnmpClient, error) {
			return &fakeGoSNMPClient{
				pdus: map[string][]gosnmp.SnmpPDU{
					ifNameOID: {
						stringPDU(ifNameOID+".515", "ge-0/0/0"),
						stringPDU(ifNameOID+".516", "ge-0/0/1"),
						stringPDU(ifNameOID+".519", "ge-0/0/4"),
						stringPDU(ifNameOID+".520", "ge-0/0/5"),
						stringPDU(ifNameOID+".527", "ge-0/0/0.0"),
						stringPDU(ifNameOID+".528", "ge-0/0/1.0"),
						stringPDU(ifNameOID+".531", "ge-0/0/4.0"),
						stringPDU(ifNameOID+".532", "ge-0/0/5.0"),
					},
					bridgePortIfIndexOID: {
						intPDU(bridgePortIfIndexOID+".490", 515),
						intPDU(bridgePortIfIndexOID+".491", 516),
						intPDU(bridgePortIfIndexOID+".494", 519),
						intPDU(bridgePortIfIndexOID+".495", 520),
					},
					qBridgeFdbPortOID: {
						intPDU(qBridgeFdbPortOID+".131072.220.166.50.247.83.251", 490),
						intPDU(qBridgeFdbPortOID+".196608.220.166.50.247.83.251", 490),
						intPDU(qBridgeFdbPortOID+".131072.24.232.41.73.203.91", 491),
						intPDU(qBridgeFdbPortOID+".131072.128.185.137.48.157.99", 491),
						intPDU(qBridgeFdbPortOID+".131072.236.181.250.176.194.0", 494),
						intPDU(qBridgeFdbPortOID+".262144.12.112.67.13.161.78", 495),
					},
					qBridgeFdbStatusOID: {
						intPDU(qBridgeFdbStatusOID+".131072.220.166.50.247.83.251", 3),
						intPDU(qBridgeFdbStatusOID+".196608.220.166.50.247.83.251", 3),
						intPDU(qBridgeFdbStatusOID+".131072.24.232.41.73.203.91", 3),
						intPDU(qBridgeFdbStatusOID+".131072.128.185.137.48.157.99", 3),
						intPDU(qBridgeFdbStatusOID+".131072.236.181.250.176.194.0", 3),
						intPDU(qBridgeFdbStatusOID+".262144.12.112.67.13.161.78", 3),
					},
					lldpRemoteTableOID: {
						stringPDU(lldpRemoteTableOID+".8.12186086.516.3", "eth0"),
						stringPDU(lldpRemoteTableOID+".9.12186086.516.3", "uplink-switch"),
					},
					dot1qVlanCurrentOID: {
						stringPDU(dot1qVlanCurrentOID+".1.1", "default+1"),
						stringPDU(dot1qVlanCurrentOID+".1.10", "guest+10"),
						stringPDU(dot1qVlanCurrentOID+".1.20", "iot+20"),
					},
				},
			}, nil
		},
	}

	pollTime := time.Date(2026, 3, 15, 20, 0, 0, 0, time.UTC)
	clients, err := client.ListActiveClients(context.Background(), config.PresenceSourceConfig{
		Key: "juniper-switch-a",
	}, pollTime)
	if err != nil {
		t.Fatalf("expected list active clients to succeed, got %v", err)
	}

	if len(clients) != 2 {
		t.Fatalf("expected exactly two direct-access clients, got %d", len(clients))
	}

	if clients[0].MAC != "0c:70:43:0d:a1:4e" || clients[0].ObservationExternalID != "ge-0/0/5" {
		t.Fatalf("unexpected first client %#v", clients[0])
	}
	if clients[1].MAC != "ec:b5:fa:b0:c2:00" || clients[1].ObservationExternalID != "ge-0/0/4" {
		t.Fatalf("unexpected second client %#v", clients[1])
	}

	if clients[0].Metadata["selection_reason"] != "single_mac_non_lldp_port" {
		t.Fatalf("expected selection reason metadata, got %#v", clients[0].Metadata)
	}
	if clients[0].Metadata["fdb_vlan_index"] != "262144" {
		t.Fatalf("expected raw fdb vlan index enrichment, got %#v", clients[0].Metadata)
	}
}

func TestJuniperSNMPClientListActiveClientsPropagatesWalkErrors(t *testing.T) {
	t.Parallel()

	client := &JuniperSNMPClient{
		newClient: func(source config.PresenceSourceConfig) (gosnmpClient, error) {
			return &fakeGoSNMPClient{
				errs: map[string]error{
					ifNameOID: errors.New("boom"),
				},
			}, nil
		},
	}

	_, err := client.ListActiveClients(context.Background(), config.PresenceSourceConfig{
		Key: "juniper-switch-a",
	}, time.Now().UTC())
	if err == nil || err.Error() != "boom" {
		t.Fatalf("expected walk error to be returned, got %v", err)
	}
}

func stringPDU(name string, value string) gosnmp.SnmpPDU {
	return gosnmp.SnmpPDU{Name: name, Type: gosnmp.OctetString, Value: []byte(value)}
}

func intPDU(name string, value int) gosnmp.SnmpPDU {
	return gosnmp.SnmpPDU{Name: name, Type: gosnmp.Integer, Value: value}
}
