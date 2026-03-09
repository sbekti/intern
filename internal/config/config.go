package config

import (
	"fmt"
	"log/slog"
	"net/netip"
	"os"
	"strings"
)

type Config struct {
	Server        ServerConfig
	Database      DatabaseConfig
	LogLevel      LogLevel
	Auth          AuthConfig
	TrustedProxy  TrustedProxyConfig
	Authorization AuthorizationConfig
}

type ServerConfig struct {
	Addr string
}

type DatabaseConfig struct {
	URL string
}

type AuthConfig struct {
	JWTIssuer     string
	JWTAudience   string
	JWTHMACSecret string
}

type TrustedProxyConfig struct {
	CIDRs        []netip.Prefix
	UserHeader   string
	NameHeader   string
	EmailHeader  string
	GroupsHeader string
}

type AuthorizationConfig struct {
	AdminGroups []string
}

type LogLevel string

const (
	LogLevelDebug LogLevel = "debug"
	LogLevelInfo  LogLevel = "info"
	LogLevelWarn  LogLevel = "warn"
	LogLevelError LogLevel = "error"
)

func Load() (Config, error) {
	trustedProxyCIDRs, err := envPrefixesOrDefault("TRUSTED_PROXY_CIDRS", []string{"127.0.0.1/32", "::1/128"})
	if err != nil {
		return Config{}, err
	}

	cfg := Config{
		Server: ServerConfig{
			Addr: envOrDefault("INTERN_API_ADDR", ":8080"),
		},
		Database: DatabaseConfig{
			URL: envOrDefault("INTERN_API_DATABASE_URL", ""),
		},
		LogLevel: LogLevel(envOrDefault("INTERN_API_LOG_LEVEL", string(LogLevelInfo))),
		Auth: AuthConfig{
			JWTIssuer:     envOrDefault("AUTH_JWT_ISSUER", "intern.corp.example.com"),
			JWTAudience:   envOrDefault("AUTH_JWT_AUDIENCE", "internctl"),
			JWTHMACSecret: envOrDefault("AUTH_JWT_HMAC_SECRET", "dev-insecure-jwt-secret"),
		},
		TrustedProxy: TrustedProxyConfig{
			CIDRs:        trustedProxyCIDRs,
			UserHeader:   envOrDefault("AUTH_REMOTE_USER_HEADER", "Remote-User"),
			NameHeader:   envOrDefault("AUTH_REMOTE_NAME_HEADER", "Remote-Name"),
			EmailHeader:  envOrDefault("AUTH_REMOTE_EMAIL_HEADER", "Remote-Email"),
			GroupsHeader: envOrDefault("AUTH_REMOTE_GROUPS_HEADER", "Remote-Groups"),
		},
		Authorization: AuthorizationConfig{
			AdminGroups: envCSVOrDefault("AUTH_ADMIN_GROUPS", []string{"Super-Users"}),
		},
	}

	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func (c Config) Validate() error {
	if strings.TrimSpace(c.Server.Addr) == "" {
		return fmt.Errorf("INTERN_API_ADDR must not be empty")
	}
	if strings.TrimSpace(c.Database.URL) == "" {
		return fmt.Errorf("INTERN_API_DATABASE_URL must not be empty")
	}

	switch c.LogLevel {
	case LogLevelDebug, LogLevelInfo, LogLevelWarn, LogLevelError:
	default:
		return fmt.Errorf("invalid INTERN_API_LOG_LEVEL %q", c.LogLevel)
	}

	if strings.TrimSpace(c.Auth.JWTIssuer) == "" {
		return fmt.Errorf("AUTH_JWT_ISSUER must not be empty")
	}
	if strings.TrimSpace(c.Auth.JWTAudience) == "" {
		return fmt.Errorf("AUTH_JWT_AUDIENCE must not be empty")
	}
	if strings.TrimSpace(c.Auth.JWTHMACSecret) == "" {
		return fmt.Errorf("AUTH_JWT_HMAC_SECRET must not be empty")
	}
	if len(c.TrustedProxy.CIDRs) == 0 {
		return fmt.Errorf("TRUSTED_PROXY_CIDRS must contain at least one CIDR")
	}
	if strings.TrimSpace(c.TrustedProxy.UserHeader) == "" {
		return fmt.Errorf("AUTH_REMOTE_USER_HEADER must not be empty")
	}
	if len(c.Authorization.AdminGroups) == 0 {
		return fmt.Errorf("AUTH_ADMIN_GROUPS must contain at least one group")
	}

	return nil
}

func (l LogLevel) Leveler() slog.Leveler {
	switch l {
	case LogLevelDebug:
		return slog.LevelDebug
	case LogLevelWarn:
		return slog.LevelWarn
	case LogLevelError:
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func (l LogLevel) String() string {
	return string(l)
}

func envOrDefault(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func envCSVOrDefault(key string, fallback []string) []string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return append([]string(nil), fallback...)
	}

	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	if len(result) == 0 {
		return append([]string(nil), fallback...)
	}
	return result
}

func envPrefixesOrDefault(key string, fallback []string) ([]netip.Prefix, error) {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		raw = strings.Join(fallback, ",")
	}

	parts := strings.Split(raw, ",")
	prefixes := make([]netip.Prefix, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		prefix, err := netip.ParsePrefix(trimmed)
		if err != nil {
			return nil, fmt.Errorf("invalid CIDR in %s: %w", key, err)
		}
		prefixes = append(prefixes, prefix)
	}

	if len(prefixes) == 0 {
		return nil, fmt.Errorf("%s must contain at least one CIDR", key)
	}

	return prefixes, nil
}
