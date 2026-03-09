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
				Server:   ServerConfig{Addr: ":8080"},
				Database: DatabaseConfig{URL: "postgres://postgres:postgres@127.0.0.1:5432/intern_test?sslmode=disable"},
				Redis:    RedisConfig{URL: "redis://127.0.0.1:6379/0"},
				Weather:  WeatherConfig{BaseURL: "https://weather.example.test", LocationName: "Example Home", Latitude: 40.7128, Longitude: -74.0060, CacheTTL: 15 * time.Minute},
				LogLevel: LogLevelInfo,
				Auth: AuthConfig{
					JWTIssuer:     "intern.corp.example.com",
					JWTAudience:   "internctl",
					JWTHMACSecret: "test-secret",
				},
				TrustedProxy: TrustedProxyConfig{
					CIDRs:      []netip.Prefix{netip.MustParsePrefix("127.0.0.1/32")},
					UserHeader: "Remote-User",
				},
				Authorization: AuthorizationConfig{
					AdminGroups: []string{"Super-Users"},
				},
			},
		},
		{
			name: "empty addr",
			cfg: Config{
				Server:   ServerConfig{Addr: ""},
				Database: DatabaseConfig{URL: "postgres://postgres:postgres@127.0.0.1:5432/intern_test?sslmode=disable"},
				Redis:    RedisConfig{URL: "redis://127.0.0.1:6379/0"},
				Weather:  WeatherConfig{BaseURL: "https://weather.example.test", LocationName: "Example Home", Latitude: 40.7128, Longitude: -74.0060, CacheTTL: 15 * time.Minute},
				LogLevel: LogLevelInfo,
				Auth: AuthConfig{
					JWTIssuer:     "intern.corp.example.com",
					JWTAudience:   "internctl",
					JWTHMACSecret: "test-secret",
				},
				TrustedProxy: TrustedProxyConfig{
					CIDRs:      []netip.Prefix{netip.MustParsePrefix("127.0.0.1/32")},
					UserHeader: "Remote-User",
				},
				Authorization: AuthorizationConfig{
					AdminGroups: []string{"Super-Users"},
				},
			},
			wantErr: true,
		},
		{
			name: "invalid log level",
			cfg: Config{
				Server:   ServerConfig{Addr: ":8080"},
				Database: DatabaseConfig{URL: "postgres://postgres:postgres@127.0.0.1:5432/intern_test?sslmode=disable"},
				Redis:    RedisConfig{URL: "redis://127.0.0.1:6379/0"},
				Weather:  WeatherConfig{BaseURL: "https://weather.example.test", LocationName: "Example Home", Latitude: 40.7128, Longitude: -74.0060, CacheTTL: 15 * time.Minute},
				LogLevel: LogLevel("trace"),
				Auth: AuthConfig{
					JWTIssuer:     "intern.corp.example.com",
					JWTAudience:   "internctl",
					JWTHMACSecret: "test-secret",
				},
				TrustedProxy: TrustedProxyConfig{
					CIDRs:      []netip.Prefix{netip.MustParsePrefix("127.0.0.1/32")},
					UserHeader: "Remote-User",
				},
				Authorization: AuthorizationConfig{
					AdminGroups: []string{"Super-Users"},
				},
			},
			wantErr: true,
		},
		{
			name: "missing jwt secret",
			cfg: Config{
				Server:   ServerConfig{Addr: ":8080"},
				Database: DatabaseConfig{URL: "postgres://postgres:postgres@127.0.0.1:5432/intern_test?sslmode=disable"},
				Redis:    RedisConfig{URL: "redis://127.0.0.1:6379/0"},
				Weather:  WeatherConfig{BaseURL: "https://weather.example.test", LocationName: "Example Home", Latitude: 40.7128, Longitude: -74.0060, CacheTTL: 15 * time.Minute},
				LogLevel: LogLevelInfo,
				Auth: AuthConfig{
					JWTIssuer:   "intern.corp.example.com",
					JWTAudience: "internctl",
				},
				TrustedProxy: TrustedProxyConfig{
					CIDRs:      []netip.Prefix{netip.MustParsePrefix("127.0.0.1/32")},
					UserHeader: "Remote-User",
				},
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
				Auth: AuthConfig{
					JWTIssuer:     "intern.corp.example.com",
					JWTAudience:   "internctl",
					JWTHMACSecret: "test-secret",
				},
				Authorization: AuthorizationConfig{
					AdminGroups: []string{"Super-Users"},
				},
			},
			wantErr: true,
		},
		{
			name: "missing database url",
			cfg: Config{
				Server:   ServerConfig{Addr: ":8080"},
				Database: DatabaseConfig{},
				Redis:    RedisConfig{URL: "redis://127.0.0.1:6379/0"},
				Weather:  WeatherConfig{BaseURL: "https://weather.example.test", LocationName: "Example Home", Latitude: 40.7128, Longitude: -74.0060, CacheTTL: 15 * time.Minute},
				LogLevel: LogLevelInfo,
				Auth: AuthConfig{
					JWTIssuer:     "intern.corp.example.com",
					JWTAudience:   "internctl",
					JWTHMACSecret: "test-secret",
				},
				TrustedProxy: TrustedProxyConfig{
					CIDRs:      []netip.Prefix{netip.MustParsePrefix("127.0.0.1/32")},
					UserHeader: "Remote-User",
				},
				Authorization: AuthorizationConfig{
					AdminGroups: []string{"Super-Users"},
				},
			},
			wantErr: true,
		},
		{
			name: "missing redis url",
			cfg: Config{
				Server:   ServerConfig{Addr: ":8080"},
				Database: DatabaseConfig{URL: "postgres://postgres:postgres@127.0.0.1:5432/intern_test?sslmode=disable"},
				Weather:  WeatherConfig{BaseURL: "https://weather.example.test", LocationName: "Example Home", Latitude: 40.7128, Longitude: -74.0060, CacheTTL: 15 * time.Minute},
				LogLevel: LogLevelInfo,
				Auth: AuthConfig{
					JWTIssuer:     "intern.corp.example.com",
					JWTAudience:   "internctl",
					JWTHMACSecret: "test-secret",
				},
				TrustedProxy: TrustedProxyConfig{
					CIDRs:      []netip.Prefix{netip.MustParsePrefix("127.0.0.1/32")},
					UserHeader: "Remote-User",
				},
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
