//go:build integration

package weather

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sbekti/intern-api/internal/config"
	"github.com/sbekti/intern-api/internal/testutil"
)

func TestServiceCachesSummaryInRedis(t *testing.T) {
	t.Parallel()

	redisContainer := testutil.StartRedis(t)
	var requests atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"timezone": "America/New_York",
			"current": map[string]any{
				"temperature_2m": 21.5,
				"wind_speed_10m": 8.2,
				"weather_code":   2,
			},
		})
	}))
	defer server.Close()

	service := NewService(config.Config{
		Weather: config.WeatherConfig{
			BaseURL:      server.URL,
			LocationName: "Example Home",
			Latitude:     40.7128,
			Longitude:    -74.0060,
			CacheTTL:     time.Minute,
		},
	}, NewRedisCache(redisContainer.Client), nil)

	ctx := context.Background()
	first, err := service.GetSummary(ctx)
	if err != nil {
		t.Fatalf("expected first fetch to succeed, got %v", err)
	}
	second, err := service.GetSummary(ctx)
	if err != nil {
		t.Fatalf("expected cached fetch to succeed, got %v", err)
	}

	if requests.Load() != 1 {
		t.Fatalf("expected exactly 1 upstream request, got %d", requests.Load())
	}
	if first.Current.TemperatureC != second.Current.TemperatureC {
		t.Fatalf("expected cached payload to match first payload: first=%#v second=%#v", first, second)
	}

	cached, err := redisContainer.Client.Get(ctx, service.cacheKey()).Result()
	if err != nil {
		t.Fatalf("expected cached redis entry, got %v", err)
	}
	if cached == "" {
		t.Fatal("expected non-empty cached redis payload")
	}
}
