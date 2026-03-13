package authspam

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/sbekti/intern-api/internal/config"
	"github.com/sbekti/intern-api/internal/requestmeta"
)

var (
	ErrRateLimited = errors.New("rate limited")

	incrWindowScript = redis.NewScript(`
local current = redis.call("INCR", KEYS[1])
if current == 1 then
  redis.call("PEXPIRE", KEYS[1], ARGV[1])
end
local ttl = redis.call("PTTL", KEYS[1])
return {current, ttl}
`)
)

type Service struct {
	client redis.Cmdable
	cfg    config.AuthRateLimitConfig
}

type RateLimitedError struct {
	RetryAfter time.Duration
}

func (e RateLimitedError) Error() string {
	return ErrRateLimited.Error()
}

func (e RateLimitedError) Unwrap() error {
	return ErrRateLimited
}

func NewService(client redis.Cmdable, cfg config.AuthRateLimitConfig) *Service {
	return &Service{
		client: client,
		cfg:    cfg,
	}
}

func (s *Service) CheckDeviceCodeCreate(ctx context.Context, clientInfo requestmeta.ClientInfo) error {
	if s == nil {
		return nil
	}
	return s.check(ctx, "device_code_create", rateLimitKey("device_code_create", "", clientInfo.IP), s.cfg.DeviceCodeCreate)
}

func (s *Service) CheckDeviceTokenExchange(ctx context.Context, clientInfo requestmeta.ClientInfo) error {
	if s == nil {
		return nil
	}
	return s.check(ctx, "device_token_exchange", rateLimitKey("device_token_exchange", "", clientInfo.IP), s.cfg.DeviceTokenExchange)
}

func (s *Service) CheckDeviceDecision(ctx context.Context, username string, clientInfo requestmeta.ClientInfo) error {
	if s == nil {
		return nil
	}
	return s.check(ctx, "device_decision", rateLimitKey("device_decision", username, clientInfo.IP), s.cfg.DeviceDecision)
}

func (s *Service) CheckRefreshToken(ctx context.Context, clientInfo requestmeta.ClientInfo) error {
	if s == nil {
		return nil
	}
	return s.check(ctx, "refresh_token", rateLimitKey("refresh_token", "", clientInfo.IP), s.cfg.RefreshToken)
}

func (s *Service) CheckLogout(ctx context.Context, clientInfo requestmeta.ClientInfo) error {
	if s == nil {
		return nil
	}
	return s.check(ctx, "logout", rateLimitKey("logout", "", clientInfo.IP), s.cfg.Logout)
}

func (s *Service) check(ctx context.Context, scope, key string, rule config.AuthRateLimitRule) error {
	if s == nil || s.client == nil {
		return nil
	}

	result, err := incrWindowScript.Run(ctx, s.client, []string{key}, rule.Window.Milliseconds()).Result()
	if err != nil {
		return fmt.Errorf("%s rate limit: %w", scope, err)
	}

	values, ok := result.([]any)
	if !ok || len(values) != 2 {
		return fmt.Errorf("%s rate limit: unexpected redis result %T", scope, result)
	}

	current, err := parseInt64(values[0])
	if err != nil {
		return fmt.Errorf("%s rate limit: %w", scope, err)
	}
	if current <= rule.Limit {
		return nil
	}

	ttlMillis, err := parseInt64(values[1])
	if err != nil {
		return fmt.Errorf("%s rate limit: %w", scope, err)
	}

	retryAfter := time.Duration(ttlMillis) * time.Millisecond
	if retryAfter <= 0 {
		retryAfter = rule.Window
	}

	return RateLimitedError{RetryAfter: retryAfter}
}

func rateLimitKey(scope, username, ip string) string {
	parts := []string{"authspam", scope}
	if trimmedUser := strings.TrimSpace(username); trimmedUser != "" {
		parts = append(parts, "user", url.QueryEscape(trimmedUser))
	}

	trimmedIP := strings.TrimSpace(ip)
	if trimmedIP == "" {
		trimmedIP = "unknown"
	}
	parts = append(parts, "ip", url.QueryEscape(trimmedIP))

	return strings.Join(parts, ":")
}

func parseInt64(value any) (int64, error) {
	switch cast := value.(type) {
	case int64:
		return cast, nil
	case string:
		return strconv.ParseInt(cast, 10, 64)
	default:
		return 0, fmt.Errorf("unexpected value type %T", value)
	}
}
