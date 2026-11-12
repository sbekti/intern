//go:build integration

package devices

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/sbekti/intern/internal/api"
	"github.com/sbekti/intern/internal/db"
	"github.com/sbekti/intern/internal/testutil"
)

func TestWiFiPasswordLifecycleAndSecretFreeAudit(t *testing.T) {
	pg := testutil.StartPostgres(t)
	ctx := context.Background()
	q := db.New(pg.Pool)
	actor, err := q.UpsertUserByUsername(ctx, db.UpsertUserByUsernameParams{Username: "alice", Name: "Alice", Groups: []string{"Super-Users"}})
	if err != nil {
		t.Fatal(err)
	}
	cipher, _ := NewPasswordCipher(bytes.Repeat([]byte{7}, 32))
	service := NewService(q, NewPGXTransactor(pg.Pool), cipher)
	enabled := true
	password := "forest-river-cloud-moon"
	created, err := service.Create(ctx, actor, api.NetworkDeviceWrite{
		DisplayName: "Test device", MacAddress: "02:00:00:00:00:01",
		VlanId:                     vlanNumberByName(t, ctx, pg.Pool, "iot"),
		PrivateWifiPasswordEnabled: &enabled, WifiPassword: &password,
	})
	if err != nil {
		t.Fatal(err)
	}
	id := deviceID(created.Device)
	sealed := bytes.Clone(created.Device.WifiPasswordCiphertext)
	if bytes.Contains(sealed, []byte(password)) || len(sealed) == 0 {
		t.Fatal("password not encrypted")
	}
	serialized, _ := json.Marshal(created.Device)
	if bytes.Contains(serialized, []byte("ciphertext")) || bytes.Contains(serialized, []byte(password)) {
		t.Fatal("database model exposes the credential")
	}
	if got, err := service.RevealWiFiPassword(ctx, actor, id); err != nil || got != password {
		t.Fatal("reveal failed")
	}
	blank := ""
	updated, err := service.Update(ctx, actor, id, api.NetworkDevicePatch{WifiPassword: &blank})
	if err != nil || !bytes.Equal(sealed, updated.Device.WifiPasswordCiphertext) {
		t.Fatal("blank password replaced the saved key")
	}
	enabled = false
	updated, err = service.Update(ctx, actor, id, api.NetworkDevicePatch{PrivateWifiPasswordEnabled: &enabled})
	if err != nil || updated.Device.PrivateWifiPasswordEnabled || !bytes.Equal(sealed, updated.Device.WifiPasswordCiphertext) {
		t.Fatal("disabling access discarded the key")
	}
	enabled = true
	updated, err = service.Update(ctx, actor, id, api.NetworkDevicePatch{PrivateWifiPasswordEnabled: &enabled})
	if err != nil || !updated.Device.PrivateWifiPasswordEnabled {
		t.Fatal("could not re-enable the saved key")
	}
	invalid, changedName := "short", "Must roll back"
	if _, err := service.Update(ctx, actor, id, api.NetworkDevicePatch{DisplayName: &changedName, WifiPassword: &invalid}); err == nil {
		t.Fatal("accepted invalid password")
	}
	current, err := service.Get(ctx, id)
	if err != nil || current.Device.DisplayName != "Test device" {
		t.Fatal("invalid password partially committed the device edit")
	}
	newMAC := "02:00:00:00:00:02"
	if _, err := service.Update(ctx, actor, id, api.NetworkDevicePatch{MacAddress: &newMAC}); err != nil {
		t.Fatal(err)
	}
	if got, err := service.RevealWiFiPassword(ctx, actor, id); err != nil || got != password {
		t.Fatal("MAC edit broke UUID-bound encryption")
	}
	rows, err := pg.Pool.Query(ctx, `SELECT action, metadata::text FROM audit_logs WHERE resource_id=$1`, id.String())
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	reveals := 0
	for rows.Next() {
		var action, metadata string
		if err := rows.Scan(&action, &metadata); err != nil {
			t.Fatal(err)
		}
		if action == "device.wifi_password.reveal" {
			reveals++
		}
		if bytes.Contains([]byte(metadata), []byte(password)) || bytes.Contains([]byte(metadata), []byte(base64.StdEncoding.EncodeToString(sealed))) {
			t.Fatal("audit contains credential material")
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if reveals != 2 {
		t.Fatalf("got %d reveal audit records, want 2", reveals)
	}
}
