package dashboard

import (
	"context"
	"fmt"

	openapi_types "github.com/oapi-codegen/runtime/types"
	"github.com/sbekti/intern-api/internal/api"
	"github.com/sbekti/intern-api/internal/auth"
	"github.com/sbekti/intern-api/internal/db"
)

type Querier interface {
	CountNetworkDevices(ctx context.Context) (int64, error)
	CountVlans(ctx context.Context) (int64, error)
}

type WeatherService interface {
	GetSummary(ctx context.Context) (*api.WeatherSummary, error)
}

type Service struct {
	queries Querier
	weather WeatherService
}

func NewService(queries Querier, weather WeatherService) *Service {
	return &Service{
		queries: queries,
		weather: weather,
	}
}

func (s *Service) Build(ctx context.Context, user db.User, principal *auth.Principal, isAdmin bool) (*api.Dashboard, error) {
	if s == nil || s.queries == nil {
		return nil, fmt.Errorf("dashboard queries not configured")
	}

	deviceCount, err := s.queries.CountNetworkDevices(ctx)
	if err != nil {
		return nil, err
	}

	vlanCount, err := s.queries.CountVlans(ctx)
	if err != nil {
		return nil, err
	}

	var weatherSummary *api.WeatherSummary
	if s.weather != nil {
		weatherSummary, err = s.weather.GetSummary(ctx)
		if err != nil {
			weatherSummary = nil
		}
	}

	return &api.Dashboard{
		WelcomeMessage: fmt.Sprintf("Welcome, %s", displayName(user, principal)),
		Profile: api.Profile{
			Username: user.Username,
			Name:     user.Name,
			Email:    openapi_types.Email(user.Email),
			Groups:   append([]string(nil), user.Groups...),
			IsAdmin:  isAdmin,
		},
		NetworkSummary: api.NetworkSummary{
			DeviceCount: deviceCount,
			VlanCount:   vlanCount,
		},
		Weather: weatherSummary,
	}, nil
}

func displayName(user db.User, principal *auth.Principal) string {
	if user.Name != "" {
		return user.Name
	}
	if principal != nil && principal.Name != "" {
		return principal.Name
	}
	if user.Username != "" {
		return user.Username
	}
	if principal != nil && principal.Username != "" {
		return principal.Username
	}
	return "there"
}
