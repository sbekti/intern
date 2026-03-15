package netnormalize

import (
	"encoding/hex"
	"fmt"
	"strings"
	"unicode"
)

func NormalizeMAC(raw string) (bare string, colon string, err error) {
	var builder strings.Builder
	for _, r := range strings.TrimSpace(strings.ToLower(raw)) {
		switch {
		case r == ':' || r == '-' || r == '.':
			continue
		case unicode.IsDigit(r) || (r >= 'a' && r <= 'f'):
			builder.WriteRune(r)
		default:
			return "", "", fmt.Errorf("mac_address must contain 12 hexadecimal characters")
		}
	}

	bare = builder.String()
	if len(bare) != 12 {
		return "", "", fmt.Errorf("mac_address must contain 12 hexadecimal characters")
	}
	if _, err := hex.DecodeString(bare); err != nil {
		return "", "", fmt.Errorf("mac_address must contain 12 hexadecimal characters")
	}

	var colonBuilder strings.Builder
	for i := 0; i < len(bare); i += 2 {
		if i > 0 {
			colonBuilder.WriteByte(':')
		}
		colonBuilder.WriteString(bare[i : i+2])
	}

	return bare, colonBuilder.String(), nil
}
