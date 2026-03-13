//go:build integration

package authspam

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/sbekti/intern-api/internal/config"
	"github.com/sbekti/intern-api/internal/requestmeta"
	"github.com/sbekti/intern-api/internal/testutil"
)

func TestServiceIntegrationDeviceCodeCreateIPLimit(t *testing.T) {
	t.Parallel()

	redisContainer := testutil.StartRedis(t)
	service := NewService(redisContainer.Client, config.AuthRateLimitConfig{
		DeviceCodeCreate:    config.AuthRateLimitRule{Limit: 1, Window: time.Minute},
		DeviceTokenExchange: config.AuthRateLimitRule{Limit: 10, Window: time.Minute},
		DeviceDecision:      config.AuthRateLimitRule{Limit: 10, Window: time.Minute},
		RefreshToken:        config.AuthRateLimitRule{Limit: 10, Window: time.Minute},
	})

	clientInfo := requestmeta.ClientInfo{IP: "203.0.113.20"}
	if err := service.CheckDeviceCodeCreate(context.Background(), clientInfo); err != nil {
		t.Fatalf("expected first request to pass, got %v", err)
	}

	err := service.CheckDeviceCodeCreate(context.Background(), clientInfo)
	if !errors.Is(err, ErrRateLimited) {
		t.Fatalf("expected rate limited error, got %v", err)
	}

	var limitedErr RateLimitedError
	if !errors.As(err, &limitedErr) {
		t.Fatalf("expected RateLimitedError, got %T", err)
	}
	if limitedErr.RetryAfter <= 0 {
		t.Fatalf("expected positive retry after, got %v", limitedErr.RetryAfter)
	}
}

func TestServiceIntegrationDeviceDecisionUsesUserAndIPKey(t *testing.T) {
	t.Parallel()

	redisContainer := testutil.StartRedis(t)
	service := NewService(redisContainer.Client, config.AuthRateLimitConfig{
		DeviceCodeCreate:    config.AuthRateLimitRule{Limit: 10, Window: time.Minute},
		DeviceTokenExchange: config.AuthRateLimitRule{Limit: 10, Window: time.Minute},
		DeviceDecision:      config.AuthRateLimitRule{Limit: 1, Window: time.Minute},
		RefreshToken:        config.AuthRateLimitRule{Limit: 10, Window: time.Minute},
	})

	clientInfo := requestmeta.ClientInfo{IP: "203.0.113.21"}
	if err := service.CheckDeviceDecision(context.Background(), "alice", clientInfo); err != nil {
		t.Fatalf("expected first alice request to pass, got %v", err)
	}
	if err := service.CheckDeviceDecision(context.Background(), "bob", clientInfo); err != nil {
		t.Fatalf("expected bob request to pass with same IP, got %v", err)
	}

	err := service.CheckDeviceDecision(context.Background(), "alice", clientInfo)
	if !errors.Is(err, ErrRateLimited) {
		t.Fatalf("expected alice second request to be rate limited, got %v", err)
	}
}

func TestServiceIntegrationRefreshTokenIPLimit(t *testing.T) {
	t.Parallel()

	redisContainer := testutil.StartRedis(t)
	service := NewService(redisContainer.Client, config.AuthRateLimitConfig{
		DeviceCodeCreate:    config.AuthRateLimitRule{Limit: 10, Window: time.Minute},
		DeviceTokenExchange: config.AuthRateLimitRule{Limit: 10, Window: time.Minute},
		DeviceDecision:      config.AuthRateLimitRule{Limit: 10, Window: time.Minute},
		RefreshToken:        config.AuthRateLimitRule{Limit: 1, Window: time.Minute},
	})

	clientInfo := requestmeta.ClientInfo{IP: "203.0.113.22"}
	if err := service.CheckRefreshToken(context.Background(), clientInfo); err != nil {
		t.Fatalf("expected first refresh request to pass, got %v", err)
	}

	err := service.CheckRefreshToken(context.Background(), clientInfo)
	if !errors.Is(err, ErrRateLimited) {
		t.Fatalf("expected rate limited error, got %v", err)
	}
}
