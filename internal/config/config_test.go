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
				Presence:     PresenceConfig{PollIntervalDefault: 5 * time.Minute, DisconnectGraceDefault: 15 * time.Minute},
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
				Presence:     PresenceConfig{PollIntervalDefault: 5 * time.Minute, DisconnectGraceDefault: 15 * time.Minute},
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
				Presence:     PresenceConfig{PollIntervalDefault: 5 * time.Minute, DisconnectGraceDefault: 15 * time.Minute},
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
				Presence:     PresenceConfig{PollIntervalDefault: 5 * time.Minute, DisconnectGraceDefault: 15 * time.Minute},
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
				Presence: PresenceConfig{PollIntervalDefault: 5 * time.Minute, DisconnectGraceDefault: 15 * time.Minute},
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
				Presence: PresenceConfig{PollIntervalDefault: 5 * time.Minute, DisconnectGraceDefault: 15 * time.Minute},
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
				Presence: PresenceConfig{PollIntervalDefault: 5 * time.Minute, DisconnectGraceDefault: 15 * time.Minute},
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
				Presence: PresenceConfig{PollIntervalDefault: 5 * time.Minute, DisconnectGraceDefault: 15 * time.Minute},
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
				Presence:     PresenceConfig{PollIntervalDefault: 5 * time.Minute, DisconnectGraceDefault: 15 * time.Minute},
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
				Presence:     PresenceConfig{PollIntervalDefault: 5 * time.Minute, DisconnectGraceDefault: 15 * time.Minute},
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
			name: "valid presence source config",
			cfg: Config{
				Server:   ServerConfig{Addr: ":8080"},
				Database: DatabaseConfig{URL: "postgres://postgres:postgres@127.0.0.1:5432/intern_test?sslmode=disable"},
				Redis:    RedisConfig{URL: "redis://127.0.0.1:6379/0"},
				Weather:  WeatherConfig{BaseURL: "https://weather.example.test", LocationName: "Example Home", Latitude: 40.7128, Longitude: -74.0060, CacheTTL: 15 * time.Minute},
				Presence: PresenceConfig{
					Enabled:                true,
					PollIntervalDefault:    5 * time.Minute,
					DisconnectGraceDefault: 15 * time.Minute,
					Sources: []PresenceSourceConfig{
						{
							Key:                "unifi-site-a",
							Type:               PresenceSourceTypeUnifi,
							DisplayName:        "Site A UniFi",
							Host:               "controller.internal.example",
							Port:               443,
							PollInterval:       5 * time.Minute,
							DisconnectGrace:    15 * time.Minute,
							InsecureSkipVerify: true,
							Site:               "default",
							CredentialEnv: PresenceSourceCredentialEnvConfig{
								Username: "INTERN_PRESENCE_SOURCE_UNIFI_SITE_A_USERNAME",
								Password: "INTERN_PRESENCE_SOURCE_UNIFI_SITE_A_PASSWORD",
							},
						},
						{
							Key:             "juniper-switch-a",
							Type:            PresenceSourceTypeJuniperSNMP,
							DisplayName:     "switch-a",
							Host:            "192.0.2.10",
							Port:            161,
							PollInterval:    5 * time.Minute,
							DisconnectGrace: 15 * time.Minute,
							CredentialEnv: PresenceSourceCredentialEnvConfig{
								SNMPUsername:     "INTERN_PRESENCE_SOURCE_JUNIPER_SWITCH_A_SNMP_USERNAME",
								SNMPAuthProtocol: "INTERN_PRESENCE_SOURCE_JUNIPER_SWITCH_A_SNMP_AUTH_PROTOCOL",
								SNMPAuthPassword: "INTERN_PRESENCE_SOURCE_JUNIPER_SWITCH_A_SNMP_AUTH_PASSWORD",
								SNMPPrivProtocol: "INTERN_PRESENCE_SOURCE_JUNIPER_SWITCH_A_SNMP_PRIV_PROTOCOL",
								SNMPPrivPassword: "INTERN_PRESENCE_SOURCE_JUNIPER_SWITCH_A_SNMP_PRIV_PASSWORD",
							},
						},
					},
				},
				LogLevel:     LogLevelInfo,
				Auth:         testAuthConfig("test-secret"),
				TrustedProxy: testTrustedProxyConfig(),
				Authorization: AuthorizationConfig{
					AdminGroups: []string{"Super-Users"},
				},
			},
		},
		{
			name: "duplicate presence source key",
			cfg: Config{
				Server:   ServerConfig{Addr: ":8080"},
				Database: DatabaseConfig{URL: "postgres://postgres:postgres@127.0.0.1:5432/intern_test?sslmode=disable"},
				Redis:    RedisConfig{URL: "redis://127.0.0.1:6379/0"},
				Weather:  WeatherConfig{BaseURL: "https://weather.example.test", LocationName: "Example Home", Latitude: 40.7128, Longitude: -74.0060, CacheTTL: 15 * time.Minute},
				Presence: PresenceConfig{
					Enabled:                true,
					PollIntervalDefault:    5 * time.Minute,
					DisconnectGraceDefault: 15 * time.Minute,
					Sources: []PresenceSourceConfig{
						{
							Key:             "dup",
							Type:            PresenceSourceTypeUnifi,
							DisplayName:     "A",
							Host:            "unifi-a.example.test",
							Port:            443,
							PollInterval:    5 * time.Minute,
							DisconnectGrace: 15 * time.Minute,
							Site:            "default",
							CredentialEnv: PresenceSourceCredentialEnvConfig{
								Username: "A",
								Password: "B",
							},
						},
						{
							Key:             "dup",
							Type:            PresenceSourceTypeJuniperSNMP,
							DisplayName:     "B",
							Host:            "10.20.0.11",
							Port:            161,
							PollInterval:    5 * time.Minute,
							DisconnectGrace: 15 * time.Minute,
							CredentialEnv: PresenceSourceCredentialEnvConfig{
								SNMPUsername:     "A",
								SNMPAuthProtocol: "B",
								SNMPAuthPassword: "C",
								SNMPPrivProtocol: "D",
								SNMPPrivPassword: "E",
							},
						},
					},
				},
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

func TestEnvPresenceSources(t *testing.T) {

	t.Setenv("INTERN_PRESENCE_SOURCES_JSON", `[{"key":"juniper-switch-a","type":"juniper-snmp","displayName":"switch-a","host":"192.0.2.10","port":161,"credentialEnv":{"snmpUsername":"JUNIPER_USERNAME","snmpAuthProtocol":"JUNIPER_AUTH_PROTOCOL","snmpAuthPassword":"JUNIPER_AUTH_PASSWORD","snmpPrivProtocol":"JUNIPER_PRIV_PROTOCOL","snmpPrivPassword":"JUNIPER_PRIV_PASSWORD"}}]`)

	sources, err := envPresenceSources("INTERN_PRESENCE_SOURCES_JSON", 5*time.Minute, 15*time.Minute)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(sources) != 1 {
		t.Fatalf("expected 1 source, got %d", len(sources))
	}
	if sources[0].PollInterval != 5*time.Minute {
		t.Fatalf("expected default poll interval to be applied, got %s", sources[0].PollInterval)
	}
	if sources[0].DisconnectGrace != 15*time.Minute {
		t.Fatalf("expected default disconnect grace to be applied, got %s", sources[0].DisconnectGrace)
	}
	if sources[0].CredentialEnv.SNMPPrivProtocol != "JUNIPER_PRIV_PROTOCOL" {
		t.Fatalf("expected snmp priv protocol env var name to be preserved, got %q", sources[0].CredentialEnv.SNMPPrivProtocol)
	}
}

func TestEnvPresenceSourcesDisconnectGraceOverride(t *testing.T) {
	t.Setenv("INTERN_PRESENCE_SOURCES_JSON", `[{"key":"unifi-site-a","type":"unifi","displayName":"Site A UniFi","host":"controller.internal.example","port":443,"site":"default","disconnectGrace":"2m","insecureSkipVerify":true,"credentialEnv":{"username":"UNIFI_USERNAME","password":"UNIFI_PASSWORD"}}]`)

	sources, err := envPresenceSources("INTERN_PRESENCE_SOURCES_JSON", 5*time.Minute, 15*time.Minute)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(sources) != 1 {
		t.Fatalf("expected 1 source, got %d", len(sources))
	}
	if sources[0].DisconnectGrace != 2*time.Minute {
		t.Fatalf("expected explicit disconnect grace to win, got %s", sources[0].DisconnectGrace)
	}
	if !sources[0].InsecureSkipVerify {
		t.Fatal("expected insecureSkipVerify to be preserved")
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
