package devices

import (
	"bytes"
	"context"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/google/uuid"
	"github.com/sbekti/intern/internal/db"
)

func TestRADIUSPolicy(t *testing.T) {
	cipher, _ := NewPasswordCipher(bytes.Repeat([]byte{7}, 32))
	id := uuid.New()
	password := `private-$ENV{PATH}-%{User-Name}-'"\`
	sealed, _ := cipher.encrypt(id, password)
	rows := []db.ListNetworkDevicesRow{
		{NetworkDevice: db.NetworkDevice{MacAddress: "02:00:00:00:00:03", Disabled: true, PrivateWifiPasswordEnabled: true, WifiPasswordCiphertext: []byte("invalid")}},
		{NetworkDevice: db.NetworkDevice{MacAddress: "02:00:00:00:00:02", VlanID: 20, WifiPasswordCiphertext: []byte("retained-but-not-active")}},
		{NetworkDevice: db.NetworkDevice{ID: toPgUUID(id), MacAddress: "02:00:00:00:00:01", VlanID: 100, PrivateWifiPasswordEnabled: true, WifiPasswordCiphertext: sealed}},
	}
	service := NewService(fakeQuerier{listFn: func(context.Context) ([]db.ListNetworkDevicesRow, error) {
		return rows, nil
	}}, nil, cipher)
	snapshot, err := service.RADIUSPolicy(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	expected := []RADIUSPolicyRecord{
		{MACAddress: "02:00:00:00:00:02", VLANID: 20},
		{MACAddress: "02:00:00:00:00:01", VLANID: 100, PrivatePassword: password},
	}
	if !reflect.DeepEqual(snapshot, expected) {
		t.Fatal("active policy or private-password selection is incorrect")
	}
	serialized, err := json.Marshal(snapshot)
	if err != nil || bytes.Contains(serialized, []byte("PrivatePassword")) || bytes.Contains(serialized, []byte(password)) {
		t.Fatal("internal policy records expose passwords through ordinary JSON serialization")
	}
	// Nonces and inventory-only edits must not trigger a network restart.
	rows[2].NetworkDevice.WifiPasswordCiphertext, _ = cipher.encrypt(id, password)
	rows[2].NetworkDevice.DisplayName = "Renamed device"
	unchanged, err := service.RADIUSPolicy(context.Background())
	if err != nil || !reflect.DeepEqual(snapshot, unchanged) {
		t.Fatal("unchanged policy produced a new revision")
	}
	for _, change := range []func(){
		func() { rows[2].NetworkDevice.VlanID = 20 },
		func() { rows[2].NetworkDevice.PrivateWifiPasswordEnabled = false },
		func() { rows[2].NetworkDevice.Disabled = true },
	} {
		change()
		changed, err := service.RADIUSPolicy(context.Background())
		if err != nil || reflect.DeepEqual(snapshot, changed) {
			t.Fatal("authorization change did not update policy")
		}
		snapshot = changed
	}
}

func TestRADIUSPolicyFailsWithoutReturningPartialSecrets(t *testing.T) {
	for _, ciphertext := range [][]byte{nil, {1, 2, 3}} {
		cipher, _ := NewPasswordCipher(bytes.Repeat([]byte{7}, 32))
		validID := uuid.New()
		sealed, _ := cipher.encrypt(validID, "forest-river-cloud-moon")
		service := NewService(fakeQuerier{listFn: func(context.Context) ([]db.ListNetworkDevicesRow, error) {
			return []db.ListNetworkDevicesRow{
				{NetworkDevice: db.NetworkDevice{ID: toPgUUID(validID), MacAddress: "02:00:00:00:00:01", VlanID: 100, PrivateWifiPasswordEnabled: true, WifiPasswordCiphertext: sealed}},
				{NetworkDevice: db.NetworkDevice{ID: toPgUUID(uuid.New()), MacAddress: "02:00:00:00:00:02", VlanID: 20, PrivateWifiPasswordEnabled: true, WifiPasswordCiphertext: ciphertext}},
			}, nil
		}}, nil, cipher)
		snapshot, err := service.RADIUSPolicy(context.Background())
		if err == nil || len(snapshot) != 0 {
			t.Fatal("bad active ciphertext returned a partial policy")
		}
	}
}
