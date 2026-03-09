package dashboard

import (
	"context"
	"errors"
	"testing"

	"github.com/sbekti/intern-api/internal/api"
	"github.com/sbekti/intern-api/internal/auth"
	"github.com/sbekti/intern-api/internal/db"
)

type fakeQuerier struct {
	deviceCount int64
	vlanCount   int64
	err         error
}

func (f fakeQuerier) CountNetworkDevices(ctx context.Context) (int64, error) {
	if f.err != nil {
		return 0, f.err
	}
	return f.deviceCount, nil
}

func (f fakeQuerier) CountVlans(ctx context.Context) (int64, error) {
	if f.err != nil {
		return 0, f.err
	}
	return f.vlanCount, nil
}

type fakeWeatherService struct {
	summary *api.WeatherSummary
	err     error
}

func (f fakeWeatherService) GetSummary(ctx context.Context) (*api.WeatherSummary, error) {
	return f.summary, f.err
}

func TestServiceBuild(t *testing.T) {
	t.Parallel()

	service := NewService(fakeQuerier{
		deviceCount: 12,
		vlanCount:   3,
	}, fakeWeatherService{
		summary: &api.WeatherSummary{
			LocationName: "Example Home",
			Timezone:     "America/New_York",
			Current: api.WeatherCurrent{
				TemperatureC: 20,
				WindSpeedKph: 10,
				WeatherCode:  2,
			},
		},
	})

	dashboard, err := service.Build(context.Background(), db.User{
		Username: "alice",
		Name:     "Alice Example",
		Email:    "alice@example.com",
		Groups:   []string{"Users", "Super-Users"},
	}, &auth.Principal{
		Username: "alice",
		Name:     "Alice Example",
	}, true)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if dashboard.WelcomeMessage != "Welcome, Alice Example" {
		t.Fatalf("unexpected welcome message %q", dashboard.WelcomeMessage)
	}
	if dashboard.NetworkSummary.DeviceCount != 12 {
		t.Fatalf("expected 12 devices, got %d", dashboard.NetworkSummary.DeviceCount)
	}
	if dashboard.NetworkSummary.VlanCount != 3 {
		t.Fatalf("expected 3 vlans, got %d", dashboard.NetworkSummary.VlanCount)
	}
	if dashboard.Weather == nil {
		t.Fatal("expected weather summary")
	}
	if !dashboard.Profile.IsAdmin {
		t.Fatal("expected profile is_admin to be true")
	}
}

func TestServiceBuildPropagatesErrors(t *testing.T) {
	t.Parallel()

	service := NewService(fakeQuerier{err: errors.New("db failed")}, fakeWeatherService{})
	if _, err := service.Build(context.Background(), db.User{}, nil, false); err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestServiceBuildIgnoresWeatherErrors(t *testing.T) {
	t.Parallel()

	service := NewService(fakeQuerier{
		deviceCount: 1,
		vlanCount:   2,
	}, fakeWeatherService{
		err: errors.New("weather failed"),
	})

	dashboard, err := service.Build(context.Background(), db.User{
		Username: "alice",
	}, &auth.Principal{Username: "alice"}, false)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if dashboard.Weather != nil {
		t.Fatal("expected nil weather when weather service fails")
	}
}
