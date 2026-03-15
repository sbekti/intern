package config

import (
	"net/netip"
	"testing"
	"time"
)

func TestValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{
			name: "valid defaults",
			cfg: Config{
				Server:       ServerConfig{Addr: ":8080"},
				Database:     DatabaseConfig{URL: "postgres://postgres:postgres@127.0.0.1:5432/intern_test?sslmode=disable"},
				Redis:        RedisConfig{URL: "redis://127.0.0.1:6379/0"},
				Weather:      WeatherConfig{BaseURL: "https://weather.example.test", LocationName: "Example Home", Latitude: 40.7128, Longitude: -74.0060, CacheTTL: 15 * time.Minute},
				LogLevel:     LogLevelInfo,
				Auth:         testAuthConfig("test-secret"),
				TrustedProxy: testTrustedProxyConfig(),
				Authorization: AuthorizationConfig{
					AdminGroups: []string{"Super-Users"},
				},
			},
		},
		{
			name: "empty addr",
			cfg: Config{
				Server:       ServerConfig{Addr: ""},
				Database:     DatabaseConfig{URL: "postgres://postgres:postgres@127.0.0.1:5432/intern_test?sslmode=disable"},
				Redis:        RedisConfig{URL: "redis://127.0.0.1:6379/0"},
				Weather:      WeatherConfig{BaseURL: "https://weather.example.test", LocationName: "Example Home", Latitude: 40.7128, Longitude: -74.0060, CacheTTL: 15 * time.Minute},
				LogLevel:     LogLevelInfo,
				Auth:         testAuthConfig("test-secret"),
				TrustedProxy: testTrustedProxyConfig(),
				Authorization: AuthorizationConfig{
					AdminGroups: []string{"Super-Users"},
				},
			},
			wantErr: true,
		},
		{
			name: "invalid log level",
			cfg: Config{
				Server:       ServerConfig{Addr: ":8080"},
				Database:     DatabaseConfig{URL: "postgres://postgres:postgres@127.0.0.1:5432/intern_test?sslmode=disable"},
				Redis:        RedisConfig{URL: "redis://127.0.0.1:6379/0"},
				Weather:      WeatherConfig{BaseURL: "https://weather.example.test", LocationName: "Example Home", Latitude: 40.7128, Longitude: -74.0060, CacheTTL: 15 * time.Minute},
				LogLevel:     LogLevel("trace"),
				Auth:         testAuthConfig("test-secret"),
				TrustedProxy: testTrustedProxyConfig(),
				Authorization: AuthorizationConfig{
					AdminGroups: []string{"Super-Users"},
				},
			},
			wantErr: true,
		},
		{
			name: "missing jwt secret",
			cfg: Config{
				Server:       ServerConfig{Addr: ":8080"},
				Database:     DatabaseConfig{URL: "postgres://postgres:postgres@127.0.0.1:5432/intern_test?sslmode=disable"},
				Redis:        RedisConfig{URL: "redis://127.0.0.1:6379/0"},
				Weather:      WeatherConfig{BaseURL: "https://weather.example.test", LocationName: "Example Home", Latitude: 40.7128, Longitude: -74.0060, CacheTTL: 15 * time.Minute},
				LogLevel:     LogLevelInfo,
				Auth:         testAuthConfig(""),
				TrustedProxy: testTrustedProxyConfig(),
				Authorization: AuthorizationConfig{
					AdminGroups: []string{"Super-Users"},
				},
			},
			wantErr: true,
		},
		{
			name: "missing auth public base url",
			cfg: Config{
				Server:   ServerConfig{Addr: ":8080"},
				Database: DatabaseConfig{URL: "postgres://postgres:postgres@127.0.0.1:5432/intern_test?sslmode=disable"},
				Redis:    RedisConfig{URL: "redis://127.0.0.1:6379/0"},
				Weather:  WeatherConfig{BaseURL: "https://weather.example.test", LocationName: "Example Home", Latitude: 40.7128, Longitude: -74.0060, CacheTTL: 15 * time.Minute},
				LogLevel: LogLevelInfo,
				Auth: AuthConfig{
					PublicBaseURL:      "",
					JWTIssuer:          "intern.corp.example.com",
					JWTAudience:        "internctl",
					JWTHMACSecret:      "test-secret",
					AccessTokenTTL:     15 * time.Minute,
					RefreshIdleTTL:     30 * 24 * time.Hour,
					RefreshAbsoluteTTL: 90 * 24 * time.Hour,
					DeviceCodeTTL:      10 * time.Minute,
					DevicePollInterval: 5 * time.Second,
				},
				TrustedProxy: testTrustedProxyConfig(),
				Authorization: AuthorizationConfig{
					AdminGroups: []string{"Super-Users"},
				},
			},
			wantErr: true,
		},
		{
			name: "invalid auth public base url",
			cfg: Config{
				Server:   ServerConfig{Addr: ":8080"},
				Database: DatabaseConfig{URL: "postgres://postgres:postgres@127.0.0.1:5432/intern_test?sslmode=disable"},
				Redis:    RedisConfig{URL: "redis://127.0.0.1:6379/0"},
				Weather:  WeatherConfig{BaseURL: "https://weather.example.test", LocationName: "Example Home", Latitude: 40.7128, Longitude: -74.0060, CacheTTL: 15 * time.Minute},
				LogLevel: LogLevelInfo,
				Auth: AuthConfig{
					PublicBaseURL:      "not-a-url",
					JWTIssuer:          "intern.corp.example.com",
					JWTAudience:        "internctl",
					JWTHMACSecret:      "test-secret",
					AccessTokenTTL:     15 * time.Minute,
					RefreshIdleTTL:     30 * 24 * time.Hour,
					RefreshAbsoluteTTL: 90 * 24 * time.Hour,
					DeviceCodeTTL:      10 * time.Minute,
					DevicePollInterval: 5 * time.Second,
				},
				TrustedProxy: testTrustedProxyConfig(),
				Authorization: AuthorizationConfig{
					AdminGroups: []string{"Super-Users"},
				},
			},
			wantErr: true,
		},
		{
			name: "missing trusted proxy",
			cfg: Config{
				Server:   ServerConfig{Addr: ":8080"},
				Database: DatabaseConfig{URL: "postgres://postgres:postgres@127.0.0.1:5432/intern_test?sslmode=disable"},
				Redis:    RedisConfig{URL: "redis://127.0.0.1:6379/0"},
				Weather:  WeatherConfig{BaseURL: "https://weather.example.test", LocationName: "Example Home", Latitude: 40.7128, Longitude: -74.0060, CacheTTL: 15 * time.Minute},
				LogLevel: LogLevelInfo,
				Auth:     testAuthConfig("test-secret"),
				Authorization: AuthorizationConfig{
					AdminGroups: []string{"Super-Users"},
				},
			},
			wantErr: true,
		},
		{
			name: "invalid auth rate limit",
			cfg: Config{
				Server:   ServerConfig{Addr: ":8080"},
				Database: DatabaseConfig{URL: "postgres://postgres:postgres@127.0.0.1:5432/intern_test?sslmode=disable"},
				Redis:    RedisConfig{URL: "redis://127.0.0.1:6379/0"},
				Weather:  WeatherConfig{BaseURL: "https://weather.example.test", LocationName: "Example Home", Latitude: 40.7128, Longitude: -74.0060, CacheTTL: 15 * time.Minute},
				LogLevel: LogLevelInfo,
				Auth: func() AuthConfig {
					cfg := testAuthConfig("test-secret")
					cfg.RateLimit.DeviceDecision.Limit = 0
					return cfg
				}(),
				TrustedProxy: testTrustedProxyConfig(),
				Authorization: AuthorizationConfig{
					AdminGroups: []string{"Super-Users"},
				},
			},
			wantErr: true,
		},
		{
			name: "missing database url",
			cfg: Config{
				Server:       ServerConfig{Addr: ":8080"},
				Database:     DatabaseConfig{},
				Redis:        RedisConfig{URL: "redis://127.0.0.1:6379/0"},
				Weather:      WeatherConfig{BaseURL: "https://weather.example.test", LocationName: "Example Home", Latitude: 40.7128, Longitude: -74.0060, CacheTTL: 15 * time.Minute},
				LogLevel:     LogLevelInfo,
				Auth:         testAuthConfig("test-secret"),
				TrustedProxy: testTrustedProxyConfig(),
				Authorization: AuthorizationConfig{
					AdminGroups: []string{"Super-Users"},
				},
			},
			wantErr: true,
		},
		{
			name: "missing redis url",
			cfg: Config{
				Server:       ServerConfig{Addr: ":8080"},
				Database:     DatabaseConfig{URL: "postgres://postgres:postgres@127.0.0.1:5432/intern_test?sslmode=disable"},
				Weather:      WeatherConfig{BaseURL: "https://weather.example.test", LocationName: "Example Home", Latitude: 40.7128, Longitude: -74.0060, CacheTTL: 15 * time.Minute},
				LogLevel:     LogLevelInfo,
				Auth:         testAuthConfig("test-secret"),
				TrustedProxy: testTrustedProxyConfig(),
				Authorization: AuthorizationConfig{
					AdminGroups: []string{"Super-Users"},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.cfg.Validate()
			if tt.wantErr && err == nil {
				t.Fatal("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
		})
	}
}

func TestEnvPrefixesOrDefault(t *testing.T) {
	t.Parallel()

	prefixes, err := envPrefixesOrDefault("TEST_PREFIXES_DOES_NOT_EXIST", []string{"127.0.0.1/32", "::1/128"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(prefixes) != 2 {
		t.Fatalf("expected 2 prefixes, got %d", len(prefixes))
	}
}

func testAuthConfig(secret string) AuthConfig {
	return AuthConfig{
		PublicBaseURL:      "https://intern.corp.example.com",
		JWTIssuer:          "intern.corp.example.com",
		JWTAudience:        "internctl",
		JWTHMACSecret:      secret,
		AccessTokenTTL:     15 * time.Minute,
		RefreshIdleTTL:     30 * 24 * time.Hour,
		RefreshAbsoluteTTL: 90 * 24 * time.Hour,
		DeviceCodeTTL:      10 * time.Minute,
		DevicePollInterval: 5 * time.Second,
		RateLimit: AuthRateLimitConfig{
			DeviceCodeCreate:    AuthRateLimitRule{Limit: 10, Window: time.Minute},
			DeviceTokenExchange: AuthRateLimitRule{Limit: 120, Window: time.Minute},
			DeviceDecision:      AuthRateLimitRule{Limit: 30, Window: time.Minute},
			RefreshToken:        AuthRateLimitRule{Limit: 60, Window: time.Minute},
			Logout:              AuthRateLimitRule{Limit: 60, Window: time.Minute},
		},
	}
}

func testTrustedProxyConfig() TrustedProxyConfig {
	return TrustedProxyConfig{
		CIDRs:        []netip.Prefix{netip.MustParsePrefix("127.0.0.1/32")},
		UserHeader:   "Remote-User",
		NameHeader:   "Remote-Name",
		EmailHeader:  "Remote-Email",
		GroupsHeader: "Remote-Groups",
		MarkerHeader: "X-Intern-Forward-Auth",
		MarkerValue:  "authenticated-ingress",
	}
}
