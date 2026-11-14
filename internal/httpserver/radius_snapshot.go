package httpserver

import (
	"crypto/sha256"
	"fmt"
	"sort"
	"strings"

	"github.com/sbekti/intern/internal/devices"
)

func renderRADIUSSnapshot(records []devices.RADIUSPolicyRecord) []byte {
	sort.Slice(records, func(i, j int) bool {
		return records[i].MACAddress < records[j].MACAddress
	})
	var entries strings.Builder
	for _, record := range records {
		mac := strings.ToLower(strings.ReplaceAll(record.MACAddress, ":", ""))
		fmt.Fprintf(&entries, "%s Cleartext-Password := \"%s\"", mac, mac)
		if record.PrivatePassword != "" {
			fmt.Fprintf(&entries, ", Tunnel-Password := %s", quoteRADIUSLiteral(record.PrivatePassword))
		}
		fmt.Fprintf(&entries, "\n\tTunnel-Type := VLAN,\n\tTunnel-Medium-Type := IEEE-802,\n\tTunnel-Private-Group-Id := \"%d\"\n\n", record.VLANID)
	}
	entryText := entries.String()
	revision := sha256.Sum256([]byte(entryText))
	return []byte(fmt.Sprintf("# radius-policy-v1\n# revision=%x\n%s", revision, entryText))
}

// Escape expansions before FreeRADIUS parses the literal value.
func quoteRADIUSLiteral(value string) string {
	escaped := strings.NewReplacer(`\`, `\\`, `"`, `\"`, "$", `\044`, "%", `\045`).Replace(value)
	return `"` + escaped + `"`
}
