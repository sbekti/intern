package presence

import (
	"os"
	"strings"
)

func resolveCredentialValue(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	if resolved := strings.TrimSpace(os.Getenv(trimmed)); resolved != "" {
		return resolved
	}
	return trimmed
}
