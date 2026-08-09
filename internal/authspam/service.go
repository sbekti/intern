package authspam

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/sbekti/intern-api/internal/config"
	"github.com/sbekti/intern-api/internal/requestmeta"
)

var ErrRateLimited = errors.New("rate limited")

const maxEntries = 10_000

type window struct {
	count   int64
	expires time.Time
}

// Service is a fixed-window limiter for the single API replica. Entries expire
// at the end of their configured window and are removed during checks.
type Service struct {
	mu      sync.Mutex
	buckets map[string]window
	now     func() time.Time
	max     int
	cfg     config.AuthRateLimitConfig
}

type RateLimitedError struct {
	RetryAfter time.Duration
}

func (e RateLimitedError) Error() string { return ErrRateLimited.Error() }
func (e RateLimitedError) Unwrap() error { return ErrRateLimited }

func NewService(cfg config.AuthRateLimitConfig) *Service {
	return &Service{
		buckets: make(map[string]window),
		now:     time.Now,
		max:     maxEntries,
		cfg:     cfg,
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

func (s *Service) check(ctx context.Context, _ string, key string, rule config.AuthRateLimitRule) error {
	if s == nil || rule.Limit <= 0 || rule.Window <= 0 {
		return nil
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	now := s.now()
	expires := now.Truncate(rule.Window).Add(rule.Window)

	s.mu.Lock()
	defer s.mu.Unlock()
	s.cleanupLocked(now)

	entry, ok := s.buckets[key]
	if !ok || !now.Before(entry.expires) {
		if len(s.buckets) >= s.max {
			s.evictOldestLocked()
		}
		entry = window{expires: expires}
	}
	entry.count++
	entry.expires = expires
	s.buckets[key] = entry
	if entry.count <= rule.Limit {
		return nil
	}

	retryAfter := entry.expires.Sub(now)
	if retryAfter <= 0 {
		retryAfter = rule.Window
	}
	return RateLimitedError{RetryAfter: retryAfter}
}

func (s *Service) cleanupLocked(now time.Time) {
	for key, entry := range s.buckets {
		if !now.Before(entry.expires) {
			delete(s.buckets, key)
		}
	}
}

func (s *Service) evictOldestLocked() {
	var oldestKey string
	var oldest time.Time
	for key, entry := range s.buckets {
		if oldestKey == "" || entry.expires.Before(oldest) {
			oldestKey, oldest = key, entry.expires
		}
	}
	if oldestKey != "" {
		delete(s.buckets, oldestKey)
	}
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
