package config

import "testing"

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
				LogLevel: LogLevelInfo,
			},
		},
		{
			name: "empty addr",
			cfg: Config{
				Server:   ServerConfig{Addr: ""},
				LogLevel: LogLevelInfo,
			},
			wantErr: true,
		},
		{
			name: "invalid log level",
			cfg: Config{
				Server:   ServerConfig{Addr: ":8080"},
				LogLevel: LogLevel("trace"),
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
