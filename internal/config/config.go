package config

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
)

type Config struct {
	Server   ServerConfig
	LogLevel LogLevel
}

type ServerConfig struct {
	Addr string
}

type LogLevel string

const (
	LogLevelDebug LogLevel = "debug"
	LogLevelInfo  LogLevel = "info"
	LogLevelWarn  LogLevel = "warn"
	LogLevelError LogLevel = "error"
)

func Load() (Config, error) {
	cfg := Config{
		Server: ServerConfig{
			Addr: envOrDefault("INTERN_API_ADDR", ":8080"),
		},
		LogLevel: LogLevel(envOrDefault("INTERN_API_LOG_LEVEL", string(LogLevelInfo))),
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

	switch c.LogLevel {
	case LogLevelDebug, LogLevelInfo, LogLevelWarn, LogLevelError:
		return nil
	default:
		return fmt.Errorf("invalid INTERN_API_LOG_LEVEL %q", c.LogLevel)
	}
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
