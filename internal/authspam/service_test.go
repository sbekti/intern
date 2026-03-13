package authspam

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/sbekti/intern-api/internal/config"
	"github.com/sbekti/intern-api/internal/requestmeta"
)

func TestRateLimitKey(t *testing.T) {
	t.Parallel()

	key := rateLimitKey("device_decision", "alice example", "2001:db8::1")
	if key != "authspam:device_decision:user:alice+example:ip:2001%3Adb8%3A%3A1" {
		t.Fatalf("unexpected key %q", key)
	}
}

func TestRateLimitedErrorUnwrap(t *testing.T) {
	t.Parallel()

	err := RateLimitedError{RetryAfter: 5 * time.Second}
	if !errors.Is(err, ErrRateLimited) {
		t.Fatal("expected RateLimitedError to unwrap to ErrRateLimited")
	}
}

func TestNilServiceAllowsRequests(t *testing.T) {
	t.Parallel()

	var service *Service
	if err := service.CheckDeviceCodeCreate(context.Background(), requestmeta.ClientInfo{IP: "203.0.113.10"}); err != nil {
		t.Fatalf("expected nil service to allow request, got %v", err)
	}
}

func TestServiceAllowsRequestsWhenRedisMissing(t *testing.T) {
	t.Parallel()

	service := NewService(nil, config.AuthRateLimitConfig{})
	if err := service.CheckDeviceTokenExchange(context.Background(), requestmeta.ClientInfo{IP: "203.0.113.11"}); err != nil {
		t.Fatalf("expected nil redis client to allow request, got %v", err)
	}
}

func TestParseInt64RejectsUnexpectedType(t *testing.T) {
	t.Parallel()

	if _, err := parseInt64(true); err == nil {
		t.Fatal("expected parseInt64 to reject unexpected type")
	}
}

func TestServicePropagatesRedisErrors(t *testing.T) {
	t.Parallel()

	service := NewService(redis.NewClient(&redis.Options{Addr: "127.0.0.1:0"}), config.AuthRateLimitConfig{
		DeviceCodeCreate: config.AuthRateLimitRule{Limit: 1, Window: time.Minute},
	})

	err := service.CheckDeviceCodeCreate(context.Background(), requestmeta.ClientInfo{IP: "203.0.113.12"})
	if err == nil {
		t.Fatal("expected redis error")
	}
}
