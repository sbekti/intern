package config

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/netip"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Server        ServerConfig
	Database      DatabaseConfig
	Redis         RedisConfig
	Weather       WeatherConfig
	Presence      PresenceConfig
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

type RedisConfig struct {
	URL string
}

type WeatherConfig struct {
	BaseURL      string
	LocationName string
	Latitude     float64
	Longitude    float64
	CacheTTL     time.Duration
}

type PresenceConfig struct {
	Enabled             bool
	PollIntervalDefault time.Duration
	Sources             []PresenceSourceConfig
}

type PresenceSourceType string

const (
	PresenceSourceTypeUnifi       PresenceSourceType = "unifi"
	PresenceSourceTypeJuniperSNMP PresenceSourceType = "juniper-snmp"
)

type PresenceSourceConfig struct {
	Key           string
	Type          PresenceSourceType
	DisplayName   string
	Host          string
	Port          int
	PollInterval  time.Duration
	Site          string
	CredentialEnv PresenceSourceCredentialEnvConfig
}

type PresenceSourceCredentialEnvConfig struct {
	Username         string
	Password         string
	SNMPUsername     string
	SNMPAuthProtocol string
	SNMPAuthPassword string
	SNMPPrivProtocol string
	SNMPPrivPassword string
}

type rawPresenceSourceConfig struct {
	Key           string                               `json:"key"`
	Type          PresenceSourceType                   `json:"type"`
	DisplayName   string                               `json:"displayName"`
	Host          string                               `json:"host"`
	Port          int                                  `json:"port"`
	PollInterval  string                               `json:"pollInterval"`
	Site          string                               `json:"site"`
	CredentialEnv rawPresenceSourceCredentialEnvConfig `json:"credentialEnv"`
}

type rawPresenceSourceCredentialEnvConfig struct {
	Username         string `json:"username"`
	Password         string `json:"password"`
	SNMPUsername     string `json:"snmpUsername"`
	SNMPAuthProtocol string `json:"snmpAuthProtocol"`
	SNMPAuthPassword string `json:"snmpAuthPassword"`
	SNMPPrivProtocol string `json:"snmpPrivProtocol"`
	SNMPPrivPassword string `json:"snmpPrivPassword"`
}

type AuthConfig struct {
	PublicBaseURL      string
	JWTIssuer          string
	JWTAudience        string
	JWTHMACSecret      string
	AccessTokenTTL     time.Duration
	RefreshIdleTTL     time.Duration
	RefreshAbsoluteTTL time.Duration
	DeviceCodeTTL      time.Duration
	DevicePollInterval time.Duration
	RateLimit          AuthRateLimitConfig
}

type AuthRateLimitConfig struct {
	DeviceCodeCreate    AuthRateLimitRule
	DeviceTokenExchange AuthRateLimitRule
	DeviceDecision      AuthRateLimitRule
	RefreshToken        AuthRateLimitRule
	Logout              AuthRateLimitRule
}

type AuthRateLimitRule struct {
	Limit  int64
	Window time.Duration
}

type TrustedProxyConfig struct {
	CIDRs        []netip.Prefix
	UserHeader   string
	NameHeader   string
	EmailHeader  string
	GroupsHeader string
	MarkerHeader string
	MarkerValue  string
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
	presenceEnabled, err := envBoolOrDefault("INTERN_PRESENCE_ENABLED", false)
	if err != nil {
		return Config{}, err
	}
	presencePollIntervalDefault, err := envDurationOrDefault("INTERN_PRESENCE_POLL_INTERVAL_DEFAULT", 5*time.Minute)
	if err != nil {
		return Config{}, err
	}
	presenceSources, err := envPresenceSources("INTERN_PRESENCE_SOURCES_JSON", presencePollIntervalDefault)
	if err != nil {
		return Config{}, err
	}

	trustedProxyCIDRs, err := envPrefixesOrDefault("TRUSTED_PROXY_CIDRS", []string{"127.0.0.1/32", "::1/128"})
	if err != nil {
		return Config{}, err
	}

	weatherLatitude, err := envFloatOrDefault("WEATHER_LATITUDE", 0)
	if err != nil {
		return Config{}, err
	}
	weatherLongitude, err := envFloatOrDefault("WEATHER_LONGITUDE", 0)
	if err != nil {
		return Config{}, err
	}
	weatherCacheTTL, err := envDurationOrDefault("WEATHER_CACHE_TTL", 15*time.Minute)
	if err != nil {
		return Config{}, err
	}
	accessTokenTTL, err := envDurationOrDefault("AUTH_ACCESS_TOKEN_TTL", 15*time.Minute)
	if err != nil {
		return Config{}, err
	}
	refreshIdleTTL, err := envDurationOrDefault("AUTH_REFRESH_IDLE_TTL", 30*24*time.Hour)
	if err != nil {
		return Config{}, err
	}
	refreshAbsoluteTTL, err := envDurationOrDefault("AUTH_REFRESH_ABSOLUTE_TTL", 90*24*time.Hour)
	if err != nil {
		return Config{}, err
	}
	deviceCodeTTL, err := envDurationOrDefault("AUTH_DEVICE_CODE_TTL", 10*time.Minute)
	if err != nil {
		return Config{}, err
	}
	devicePollInterval, err := envDurationOrDefault("AUTH_DEVICE_POLL_INTERVAL", 5*time.Second)
	if err != nil {
		return Config{}, err
	}
	deviceCodeCreateLimit, err := envInt64OrDefault("AUTH_DEVICE_CODE_CREATE_RATE_LIMIT", 10)
	if err != nil {
		return Config{}, err
	}
	deviceCodeCreateWindow, err := envDurationOrDefault("AUTH_DEVICE_CODE_CREATE_RATE_WINDOW", time.Minute)
	if err != nil {
		return Config{}, err
	}
	deviceTokenExchangeLimit, err := envInt64OrDefault("AUTH_DEVICE_TOKEN_EXCHANGE_RATE_LIMIT", 120)
	if err != nil {
		return Config{}, err
	}
	deviceTokenExchangeWindow, err := envDurationOrDefault("AUTH_DEVICE_TOKEN_EXCHANGE_RATE_WINDOW", time.Minute)
	if err != nil {
		return Config{}, err
	}
	deviceDecisionLimit, err := envInt64OrDefault("AUTH_DEVICE_DECISION_RATE_LIMIT", 30)
	if err != nil {
		return Config{}, err
	}
	deviceDecisionWindow, err := envDurationOrDefault("AUTH_DEVICE_DECISION_RATE_WINDOW", time.Minute)
	if err != nil {
		return Config{}, err
	}
	refreshTokenLimit, err := envInt64OrDefault("AUTH_REFRESH_TOKEN_RATE_LIMIT", 60)
	if err != nil {
		return Config{}, err
	}
	refreshTokenWindow, err := envDurationOrDefault("AUTH_REFRESH_TOKEN_RATE_WINDOW", time.Minute)
	if err != nil {
		return Config{}, err
	}
	logoutLimit, err := envInt64OrDefault("AUTH_LOGOUT_RATE_LIMIT", 60)
	if err != nil {
		return Config{}, err
	}
	logoutWindow, err := envDurationOrDefault("AUTH_LOGOUT_RATE_WINDOW", time.Minute)
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
		Redis: RedisConfig{
			URL: envOrDefault("INTERN_API_REDIS_URL", ""),
		},
		Weather: WeatherConfig{
			BaseURL:      envOrDefault("WEATHER_BASE_URL", "https://api.open-meteo.com/v1/forecast"),
			LocationName: envOrDefault("WEATHER_LOCATION_NAME", "Configured Location"),
			Latitude:     weatherLatitude,
			Longitude:    weatherLongitude,
			CacheTTL:     weatherCacheTTL,
		},
		Presence: PresenceConfig{
			Enabled:             presenceEnabled,
			PollIntervalDefault: presencePollIntervalDefault,
			Sources:             presenceSources,
		},
		LogLevel: LogLevel(envOrDefault("INTERN_API_LOG_LEVEL", string(LogLevelInfo))),
		Auth: AuthConfig{
			PublicBaseURL:      envOrDefault("AUTH_PUBLIC_BASE_URL", ""),
			JWTIssuer:          envOrDefault("AUTH_JWT_ISSUER", "intern.corp.example.com"),
			JWTAudience:        envOrDefault("AUTH_JWT_AUDIENCE", "internctl"),
			JWTHMACSecret:      envOrDefault("AUTH_JWT_HMAC_SECRET", "dev-insecure-jwt-secret"),
			AccessTokenTTL:     accessTokenTTL,
			RefreshIdleTTL:     refreshIdleTTL,
			RefreshAbsoluteTTL: refreshAbsoluteTTL,
			DeviceCodeTTL:      deviceCodeTTL,
			DevicePollInterval: devicePollInterval,
			RateLimit: AuthRateLimitConfig{
				DeviceCodeCreate: AuthRateLimitRule{
					Limit:  deviceCodeCreateLimit,
					Window: deviceCodeCreateWindow,
				},
				DeviceTokenExchange: AuthRateLimitRule{
					Limit:  deviceTokenExchangeLimit,
					Window: deviceTokenExchangeWindow,
				},
				DeviceDecision: AuthRateLimitRule{
					Limit:  deviceDecisionLimit,
					Window: deviceDecisionWindow,
				},
				RefreshToken: AuthRateLimitRule{
					Limit:  refreshTokenLimit,
					Window: refreshTokenWindow,
				},
				Logout: AuthRateLimitRule{
					Limit:  logoutLimit,
					Window: logoutWindow,
				},
			},
		},
		TrustedProxy: TrustedProxyConfig{
			CIDRs:        trustedProxyCIDRs,
			UserHeader:   envOrDefault("AUTH_REMOTE_USER_HEADER", "Remote-User"),
			NameHeader:   envOrDefault("AUTH_REMOTE_NAME_HEADER", "Remote-Name"),
			EmailHeader:  envOrDefault("AUTH_REMOTE_EMAIL_HEADER", "Remote-Email"),
			GroupsHeader: envOrDefault("AUTH_REMOTE_GROUPS_HEADER", "Remote-Groups"),
			MarkerHeader: envOrDefault("AUTH_FORWARD_AUTH_MARKER_HEADER", "X-Intern-Forward-Auth"),
			MarkerValue:  envOrDefault("AUTH_FORWARD_AUTH_MARKER_VALUE", "authenticated-ingress"),
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
	if strings.TrimSpace(c.Redis.URL) == "" {
		return fmt.Errorf("INTERN_API_REDIS_URL must not be empty")
	}
	if strings.TrimSpace(c.Weather.BaseURL) == "" {
		return fmt.Errorf("WEATHER_BASE_URL must not be empty")
	}
	if strings.TrimSpace(c.Weather.LocationName) == "" {
		return fmt.Errorf("WEATHER_LOCATION_NAME must not be empty")
	}
	if c.Weather.CacheTTL <= 0 {
		return fmt.Errorf("WEATHER_CACHE_TTL must be greater than zero")
	}
	if (c.Presence.Enabled || len(c.Presence.Sources) > 0) && c.Presence.PollIntervalDefault <= 0 {
		return fmt.Errorf("INTERN_PRESENCE_POLL_INTERVAL_DEFAULT must be greater than zero")
	}
	if err := validatePresenceSources(c.Presence); err != nil {
		return err
	}

	switch c.LogLevel {
	case LogLevelDebug, LogLevelInfo, LogLevelWarn, LogLevelError:
	default:
		return fmt.Errorf("invalid INTERN_API_LOG_LEVEL %q", c.LogLevel)
	}

	if strings.TrimSpace(c.Auth.JWTIssuer) == "" {
		return fmt.Errorf("AUTH_JWT_ISSUER must not be empty")
	}
	if strings.TrimSpace(c.Auth.PublicBaseURL) == "" {
		return fmt.Errorf("AUTH_PUBLIC_BASE_URL must not be empty")
	}
	publicBaseURL, err := url.Parse(strings.TrimSpace(c.Auth.PublicBaseURL))
	if err != nil || !publicBaseURL.IsAbs() || strings.TrimSpace(publicBaseURL.Host) == "" {
		return fmt.Errorf("AUTH_PUBLIC_BASE_URL must be an absolute URL")
	}
	if strings.TrimSpace(c.Auth.JWTAudience) == "" {
		return fmt.Errorf("AUTH_JWT_AUDIENCE must not be empty")
	}
	if strings.TrimSpace(c.Auth.JWTHMACSecret) == "" {
		return fmt.Errorf("AUTH_JWT_HMAC_SECRET must not be empty")
	}
	if c.Auth.AccessTokenTTL <= 0 {
		return fmt.Errorf("AUTH_ACCESS_TOKEN_TTL must be greater than zero")
	}
	if c.Auth.RefreshIdleTTL <= 0 {
		return fmt.Errorf("AUTH_REFRESH_IDLE_TTL must be greater than zero")
	}
	if c.Auth.RefreshAbsoluteTTL <= 0 {
		return fmt.Errorf("AUTH_REFRESH_ABSOLUTE_TTL must be greater than zero")
	}
	if c.Auth.DeviceCodeTTL <= 0 {
		return fmt.Errorf("AUTH_DEVICE_CODE_TTL must be greater than zero")
	}
	if c.Auth.DevicePollInterval <= 0 {
		return fmt.Errorf("AUTH_DEVICE_POLL_INTERVAL must be greater than zero")
	}
	if err := validateAuthRateLimitRule("AUTH_DEVICE_CODE_CREATE_RATE", c.Auth.RateLimit.DeviceCodeCreate); err != nil {
		return err
	}
	if err := validateAuthRateLimitRule("AUTH_DEVICE_TOKEN_EXCHANGE_RATE", c.Auth.RateLimit.DeviceTokenExchange); err != nil {
		return err
	}
	if err := validateAuthRateLimitRule("AUTH_DEVICE_DECISION_RATE", c.Auth.RateLimit.DeviceDecision); err != nil {
		return err
	}
	if err := validateAuthRateLimitRule("AUTH_REFRESH_TOKEN_RATE", c.Auth.RateLimit.RefreshToken); err != nil {
		return err
	}
	if err := validateAuthRateLimitRule("AUTH_LOGOUT_RATE", c.Auth.RateLimit.Logout); err != nil {
		return err
	}
	if len(c.TrustedProxy.CIDRs) == 0 {
		return fmt.Errorf("TRUSTED_PROXY_CIDRS must contain at least one CIDR")
	}
	if strings.TrimSpace(c.TrustedProxy.UserHeader) == "" {
		return fmt.Errorf("AUTH_REMOTE_USER_HEADER must not be empty")
	}
	if strings.TrimSpace(c.TrustedProxy.MarkerHeader) == "" {
		return fmt.Errorf("AUTH_FORWARD_AUTH_MARKER_HEADER must not be empty")
	}
	if strings.TrimSpace(c.TrustedProxy.MarkerValue) == "" {
		return fmt.Errorf("AUTH_FORWARD_AUTH_MARKER_VALUE must not be empty")
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

func envInt64OrDefault(key string, fallback int64) (int64, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback, nil
	}

	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%s must be a valid integer: %w", key, err)
	}

	return parsed, nil
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

func validatePresenceSources(cfg PresenceConfig) error {
	seenKeys := make(map[string]struct{}, len(cfg.Sources))
	for _, source := range cfg.Sources {
		if strings.TrimSpace(source.Key) == "" {
			return fmt.Errorf("INTERN_PRESENCE_SOURCES_JSON contains a source with an empty key")
		}
		if _, exists := seenKeys[source.Key]; exists {
			return fmt.Errorf("INTERN_PRESENCE_SOURCES_JSON contains duplicate source key %q", source.Key)
		}
		seenKeys[source.Key] = struct{}{}

		if strings.TrimSpace(source.DisplayName) == "" {
			return fmt.Errorf("INTERN_PRESENCE_SOURCES_JSON source %q must include displayName", source.Key)
		}
		if strings.TrimSpace(source.Host) == "" {
			return fmt.Errorf("INTERN_PRESENCE_SOURCES_JSON source %q must include host", source.Key)
		}
		if source.Port <= 0 || source.Port > 65535 {
			return fmt.Errorf("INTERN_PRESENCE_SOURCES_JSON source %q has invalid port %d", source.Key, source.Port)
		}
		if source.PollInterval <= 0 {
			return fmt.Errorf("INTERN_PRESENCE_SOURCES_JSON source %q must include a positive pollInterval", source.Key)
		}

		switch source.Type {
		case PresenceSourceTypeUnifi:
			if strings.TrimSpace(source.Site) == "" {
				return fmt.Errorf("INTERN_PRESENCE_SOURCES_JSON source %q must include site for type %q", source.Key, source.Type)
			}
			if strings.TrimSpace(source.CredentialEnv.Username) == "" {
				return fmt.Errorf("INTERN_PRESENCE_SOURCES_JSON source %q must include credentialEnv.username for type %q", source.Key, source.Type)
			}
			if strings.TrimSpace(source.CredentialEnv.Password) == "" {
				return fmt.Errorf("INTERN_PRESENCE_SOURCES_JSON source %q must include credentialEnv.password for type %q", source.Key, source.Type)
			}
		case PresenceSourceTypeJuniperSNMP:
			if strings.TrimSpace(source.CredentialEnv.SNMPUsername) == "" {
				return fmt.Errorf("INTERN_PRESENCE_SOURCES_JSON source %q must include credentialEnv.snmpUsername for type %q", source.Key, source.Type)
			}
			if strings.TrimSpace(source.CredentialEnv.SNMPAuthProtocol) == "" {
				return fmt.Errorf("INTERN_PRESENCE_SOURCES_JSON source %q must include credentialEnv.snmpAuthProtocol for type %q", source.Key, source.Type)
			}
			if strings.TrimSpace(source.CredentialEnv.SNMPAuthPassword) == "" {
				return fmt.Errorf("INTERN_PRESENCE_SOURCES_JSON source %q must include credentialEnv.snmpAuthPassword for type %q", source.Key, source.Type)
			}
			if strings.TrimSpace(source.CredentialEnv.SNMPPrivProtocol) == "" {
				return fmt.Errorf("INTERN_PRESENCE_SOURCES_JSON source %q must include credentialEnv.snmpPrivProtocol for type %q", source.Key, source.Type)
			}
			if strings.TrimSpace(source.CredentialEnv.SNMPPrivPassword) == "" {
				return fmt.Errorf("INTERN_PRESENCE_SOURCES_JSON source %q must include credentialEnv.snmpPrivPassword for type %q", source.Key, source.Type)
			}
		default:
			return fmt.Errorf("INTERN_PRESENCE_SOURCES_JSON source %q has unsupported type %q", source.Key, source.Type)
		}
	}

	return nil
}

func validateAuthRateLimitRule(prefix string, rule AuthRateLimitRule) error {
	if rule.Limit <= 0 {
		return fmt.Errorf("%s_LIMIT must be greater than zero", prefix)
	}
	if rule.Window <= 0 {
		return fmt.Errorf("%s_WINDOW must be greater than zero", prefix)
	}
	return nil
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

func envBoolOrDefault(key string, fallback bool) (bool, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback, nil
	}

	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return false, fmt.Errorf("%s must be a valid boolean: %w", key, err)
	}

	return parsed, nil
}

func envFloatOrDefault(key string, fallback float64) (float64, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback, nil
	}

	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid %s: %w", key, err)
	}
	return parsed, nil
}

func envPresenceSources(key string, defaultPollInterval time.Duration) ([]PresenceSourceConfig, error) {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return nil, nil
	}

	var decoded []rawPresenceSourceConfig
	if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
		return nil, fmt.Errorf("invalid %s: %w", key, err)
	}

	sources := make([]PresenceSourceConfig, 0, len(decoded))
	for _, source := range decoded {
		pollInterval := defaultPollInterval
		if strings.TrimSpace(source.PollInterval) != "" {
			parsed, err := time.ParseDuration(strings.TrimSpace(source.PollInterval))
			if err != nil {
				return nil, fmt.Errorf("invalid %s pollInterval for source %q: %w", key, source.Key, err)
			}
			pollInterval = parsed
		}

		sources = append(sources, PresenceSourceConfig{
			Key:          strings.TrimSpace(source.Key),
			Type:         source.Type,
			DisplayName:  strings.TrimSpace(source.DisplayName),
			Host:         strings.TrimSpace(source.Host),
			Port:         source.Port,
			PollInterval: pollInterval,
			Site:         strings.TrimSpace(source.Site),
			CredentialEnv: PresenceSourceCredentialEnvConfig{
				Username:         strings.TrimSpace(source.CredentialEnv.Username),
				Password:         strings.TrimSpace(source.CredentialEnv.Password),
				SNMPUsername:     strings.TrimSpace(source.CredentialEnv.SNMPUsername),
				SNMPAuthProtocol: strings.TrimSpace(source.CredentialEnv.SNMPAuthProtocol),
				SNMPAuthPassword: strings.TrimSpace(source.CredentialEnv.SNMPAuthPassword),
				SNMPPrivProtocol: strings.TrimSpace(source.CredentialEnv.SNMPPrivProtocol),
				SNMPPrivPassword: strings.TrimSpace(source.CredentialEnv.SNMPPrivPassword),
			},
		})
	}

	return sources, nil
}

func envDurationOrDefault(key string, fallback time.Duration) (time.Duration, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback, nil
	}

	parsed, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("invalid %s: %w", key, err)
	}
	return parsed, nil
}
