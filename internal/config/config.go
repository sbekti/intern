package config

import (
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
