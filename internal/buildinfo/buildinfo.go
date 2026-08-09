package buildinfo

import "fmt"

var (
	Version = "dev"
	Commit  = "unknown"
	Date    = "unknown"
)

func Short() string {
	return Version
}

func Summary() string {
	return fmt.Sprintf("internctl %s", Version)
}

func Details(defaultServerURL string) string {
	return fmt.Sprintf(
		"version: %s\ncommit: %s\nbuilt: %s\ndefault_server: %s",
		Version,
		Commit,
		Date,
		defaultServerURL,
	)
}
