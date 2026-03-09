package weather

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/sbekti/intern-api/internal/api"
	"github.com/sbekti/intern-api/internal/config"
)

type Cache interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key string, value string, ttl time.Duration) error
}

type HTTPEr interface {
	Do(req *http.Request) (*http.Response, error)
}

type Service struct {
	baseURL      string
	locationName string
	latitude     float64
	longitude    float64
	cacheTTL     time.Duration
	cache        Cache
	httpClient   HTTPEr
}

type RedisCache struct {
	client *redis.Client
}

type openMeteoResponse struct {
	Timezone string `json:"timezone"`
	Current  struct {
		Temperature2M float64 `json:"temperature_2m"`
		WindSpeed10M  float64 `json:"wind_speed_10m"`
		WeatherCode   int32   `json:"weather_code"`
	} `json:"current"`
}

func NewService(cfg config.Config, cache Cache, httpClient HTTPEr) *Service {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 5 * time.Second}
	}

	return &Service{
		baseURL:      cfg.Weather.BaseURL,
		locationName: cfg.Weather.LocationName,
		latitude:     cfg.Weather.Latitude,
		longitude:    cfg.Weather.Longitude,
		cacheTTL:     cfg.Weather.CacheTTL,
		cache:        cache,
		httpClient:   httpClient,
	}
}

func NewRedisCache(client *redis.Client) *RedisCache {
	return &RedisCache{client: client}
}

func (c *RedisCache) Get(ctx context.Context, key string) (string, error) {
	return c.client.Get(ctx, key).Result()
}

func (c *RedisCache) Set(ctx context.Context, key string, value string, ttl time.Duration) error {
	return c.client.Set(ctx, key, value, ttl).Err()
}

func (s *Service) GetSummary(ctx context.Context) (*api.WeatherSummary, error) {
	if s.cache != nil {
		cached, err := s.cache.Get(ctx, s.cacheKey())
		if err == nil {
			var summary api.WeatherSummary
			if unmarshalErr := json.Unmarshal([]byte(cached), &summary); unmarshalErr == nil {
				return &summary, nil
			}
		}
	}

	summary, err := s.fetch(ctx)
	if err != nil {
		return nil, err
	}

	if s.cache != nil {
		encoded, err := json.Marshal(summary)
		if err != nil {
			return nil, err
		}
		_ = s.cache.Set(ctx, s.cacheKey(), string(encoded), s.cacheTTL)
	}

	return summary, nil
}

func (s *Service) fetch(ctx context.Context) (*api.WeatherSummary, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.requestURL(), nil)
	if err != nil {
		return nil, err
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("weather upstream returned status %d", resp.StatusCode)
	}

	var payload openMeteoResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}

	return &api.WeatherSummary{
		LocationName: s.locationName,
		Timezone:     payload.Timezone,
		Current: api.WeatherCurrent{
			TemperatureC: float32(payload.Current.Temperature2M),
			WindSpeedKph: float32(payload.Current.WindSpeed10M),
			WeatherCode:  payload.Current.WeatherCode,
		},
	}, nil
}

func (s *Service) cacheKey() string {
	return fmt.Sprintf("weather:summary:%s:%0.4f:%0.4f", s.locationName, s.latitude, s.longitude)
}

func (s *Service) requestURL() string {
	values := url.Values{}
	values.Set("latitude", strconv.FormatFloat(s.latitude, 'f', 4, 64))
	values.Set("longitude", strconv.FormatFloat(s.longitude, 'f', 4, 64))
	values.Set("current", "temperature_2m,wind_speed_10m,weather_code")

	return s.baseURL + "?" + values.Encode()
}
