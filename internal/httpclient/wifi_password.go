package httpclient

import (
	"context"
	"fmt"
	"net/http"

	"github.com/google/uuid"
	"github.com/sbekti/intern/internal/api"
)

func (c *Client) GenerateWiFiPassword(ctx context.Context) (string, error) {
	response, err := c.authenticated.GenerateWiFiPasswordWithResponse(ctx)
	if err != nil {
		return "", err
	}
	return wifiPasswordResult(response.StatusCode(), response.JSON200)
}

func (c *Client) RevealWiFiPassword(ctx context.Context, id string) (string, error) {
	deviceID, err := uuid.Parse(id)
	if err != nil {
		return "", fmt.Errorf("parse device id: %w", err)
	}
	response, err := c.authenticated.RevealWiFiPasswordWithResponse(ctx, deviceID)
	if err != nil {
		return "", err
	}
	return wifiPasswordResult(response.StatusCode(), response.JSON200)
}

func wifiPasswordResult(status int, result *api.WiFiPassword) (string, error) {
	switch status {
	case http.StatusOK:
		if result == nil {
			return "", fmt.Errorf("Wi-Fi password response is missing")
		}
		return result.Password, nil
	case http.StatusUnauthorized:
		return "", ErrUnauthorized
	case http.StatusForbidden:
		return "", ErrForbidden
	case http.StatusNotFound:
		return "", fmt.Errorf("device or saved Wi-Fi password not found")
	case http.StatusServiceUnavailable:
		return "", fmt.Errorf("private Wi-Fi passwords are not configured")
	default:
		// Never include a response body which could contain a password.
		return "", fmt.Errorf("Wi-Fi password request failed with status %d", status)
	}
}
