package authspam

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

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
	if !errors.Is(RateLimitedError{RetryAfter: time.Second}, ErrRateLimited) {
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

func TestFixedWindowRejectsAndReportsRetryAfter(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	service := NewService(config.AuthRateLimitConfig{
		DeviceCodeCreate: config.AuthRateLimitRule{Limit: 1, Window: time.Minute},
	})
	service.now = func() time.Time { return now }
	client := requestmeta.ClientInfo{IP: "203.0.113.11"}
	if err := service.CheckDeviceCodeCreate(context.Background(), client); err != nil {
		t.Fatalf("first request returned %v", err)
	}
	err := service.CheckDeviceCodeCreate(context.Background(), client)
	var limited RateLimitedError
	if !errors.As(err, &limited) || !errors.Is(err, ErrRateLimited) {
		t.Fatalf("expected rate limit error, got %v", err)
	}
	if limited.RetryAfter != time.Minute {
		t.Fatalf("retry after = %v, want %v", limited.RetryAfter, time.Minute)
	}
	now = now.Add(time.Minute)
	if err := service.CheckDeviceCodeCreate(context.Background(), client); err != nil {
		t.Fatalf("new window request returned %v", err)
	}
}

func TestFixedWindowKeysUserAndIP(t *testing.T) {
	t.Parallel()
	service := NewService(config.AuthRateLimitConfig{
		DeviceDecision: config.AuthRateLimitRule{Limit: 1, Window: time.Minute},
	})
	client := requestmeta.ClientInfo{IP: "203.0.113.12"}
	if err := service.CheckDeviceDecision(context.Background(), "alice", client); err != nil {
		t.Fatalf("first alice request returned %v", err)
	}
	if err := service.CheckDeviceDecision(context.Background(), "bob", client); err != nil {
		t.Fatalf("bob request returned %v", err)
	}
	if err := service.CheckDeviceDecision(context.Background(), "alice", client); !errors.Is(err, ErrRateLimited) {
		t.Fatalf("expected alice second request to be limited, got %v", err)
	}
}

func TestFixedWindowIsRaceSafeAndCleansExpiredEntries(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	service := NewService(config.AuthRateLimitConfig{
		DeviceCodeCreate: config.AuthRateLimitRule{Limit: 1000, Window: time.Minute},
	})
	service.now = func() time.Time { return now }
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = service.CheckDeviceCodeCreate(context.Background(), requestmeta.ClientInfo{IP: "203.0.113.13"})
		}()
	}
	wg.Wait()
	if got := service.buckets[rateLimitKey("device_code_create", "", "203.0.113.13")].count; got != 100 {
		t.Fatalf("count = %d, want 100", got)
	}
	now = now.Add(time.Minute)
	_ = service.CheckDeviceCodeCreate(context.Background(), requestmeta.ClientInfo{IP: "203.0.113.14"})
	if len(service.buckets) != 1 {
		t.Fatalf("bucket count = %d, want expired entries cleaned", len(service.buckets))
	}
}
