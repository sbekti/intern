package weather

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/sbekti/intern-api/internal/config"
)

type fakeCache struct {
	getFn func(ctx context.Context, key string) (string, error)
	setFn func(ctx context.Context, key string, value string, ttl time.Duration) error
}

func (f fakeCache) Get(ctx context.Context, key string) (string, error) {
	return f.getFn(ctx, key)
}

func (f fakeCache) Set(ctx context.Context, key string, value string, ttl time.Duration) error {
	return f.setFn(ctx, key, value, ttl)
}

func TestServiceGetSummaryUsesCache(t *testing.T) {
	t.Parallel()

	service := NewService(testConfig(), fakeCache{
		getFn: func(ctx context.Context, key string) (string, error) {
			return `{"location_name":"Example Home","timezone":"America/New_York","current":{"temperature_c":21.5,"wind_speed_kph":8.4,"weather_code":1}}`, nil
		},
		setFn: func(ctx context.Context, key string, value string, ttl time.Duration) error {
			t.Fatal("expected cache set not to be called")
			return nil
		},
	}, nil)

	summary, err := service.GetSummary(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if summary.LocationName != "Example Home" {
		t.Fatalf("expected cached location, got %q", summary.LocationName)
	}
}

func TestServiceGetSummaryFetchesAndCaches(t *testing.T) {
	t.Parallel()

	serverCalls := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serverCalls++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"timezone":"America/New_York","current":{"temperature_2m":19.25,"wind_speed_10m":11.5,"weather_code":3}}`))
	}))
	defer upstream.Close()

	setCalled := false
	service := NewService(config.Config{
		Weather: config.WeatherConfig{
			BaseURL:      upstream.URL,
			LocationName: "Example Home",
			Latitude:     40.7128,
			Longitude:    -74.0060,
			CacheTTL:     15 * time.Minute,
		},
	}, fakeCache{
		getFn: func(ctx context.Context, key string) (string, error) {
			return "", redis.Nil
		},
		setFn: func(ctx context.Context, key string, value string, ttl time.Duration) error {
			setCalled = true
			if ttl != 15*time.Minute {
				t.Fatalf("expected 15m ttl, got %s", ttl)
			}
			return nil
		},
	}, upstream.Client())

	summary, err := service.GetSummary(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if summary.LocationName != "Example Home" {
		t.Fatalf("expected Example Home, got %q", summary.LocationName)
	}
	if summary.Timezone != "America/New_York" {
		t.Fatalf("expected timezone America/New_York, got %q", summary.Timezone)
	}
	if serverCalls != 1 {
		t.Fatalf("expected exactly one upstream call, got %d", serverCalls)
	}
	if !setCalled {
		t.Fatal("expected cache set to be called")
	}
}

func testConfig() config.Config {
	return config.Config{
		Weather: config.WeatherConfig{
			BaseURL:      "https://weather.example.test",
			LocationName: "Example Home",
			Latitude:     40.7128,
			Longitude:    -74.0060,
			CacheTTL:     15 * time.Minute,
		},
	}
}
