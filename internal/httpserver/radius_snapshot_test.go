package httpserver

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"testing"

	"github.com/sbekti/intern/internal/devices"
)

func TestRADIUSSnapshotLiteralPasswordsAndStableRevisions(t *testing.T) {
	records := []devices.RADIUSPolicyRecord{
		{MACAddress: "02:00:00:00:00:02", VLANID: 20},
		{MACAddress: "02:00:00:00:00:01", VLANID: 100, PrivatePassword: `private-$ENV{PATH}-%{User-Name}-'"\`},
	}
	snapshot := renderRADIUSSnapshot("radius-policy-v1", records)
	entries := "020000000001 Cleartext-Password := \"020000000001\", Tunnel-Password := \"private-\\044ENV{PATH}-\\045{User-Name}-'\\\"\\\\\"\n" +
		"\tTunnel-Type := VLAN,\n\tTunnel-Medium-Type := IEEE-802,\n\tTunnel-Private-Group-Id := \"100\"\n\n" +
		"020000000002 Cleartext-Password := \"020000000002\"\n" +
		"\tTunnel-Type := VLAN,\n\tTunnel-Medium-Type := IEEE-802,\n\tTunnel-Private-Group-Id := \"20\"\n\n"
	revision := sha256.Sum256([]byte(entries))
	if string(snapshot) != fmt.Sprintf("# radius-policy-v1\n# revision=%x\n%s", revision, entries) {
		t.Fatal("snapshot does not contain exactly the sorted policy with literal password quoting")
	}
	if !bytes.Equal(snapshot, renderRADIUSSnapshot("radius-policy-v1", records)) {
		t.Fatal("unchanged policy changed its revision")
	}
	for _, change := range []func(){
		func() { records[0].VLANID = 20 },
		func() { records[0].PrivatePassword = "" },
		func() { records = records[1:] },
	} {
		change()
		changed := renderRADIUSSnapshot("radius-policy-v1", records)
		if bytes.Equal(snapshot, changed) {
			t.Fatal("authorization change did not update the snapshot revision")
		}
		snapshot = changed
	}
}
