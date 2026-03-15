package presence

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/sbekti/intern-api/internal/config"
)

type HTTPEr interface {
	Do(req *http.Request) (*http.Response, error)
}

type UniFiHTTPClient struct {
	httpClient HTTPEr
}

type unifiLoginRequest struct {
	Username   string `json:"username"`
	Password   string `json:"password"`
	RememberMe bool   `json:"rememberMe"`
	Token      string `json:"token"`
}

type unifiResponse[T any] struct {
	Data []T `json:"data"`
}

type unifiActiveClientPayload struct {
	MAC       string `json:"mac"`
	Hostname  string `json:"hostname"`
	ESSID     string `json:"essid"`
	APMAC     string `json:"ap_mac"`
	BSSID     string `json:"bssid"`
	AssocTime any    `json:"assoc_time"`
	LastSeen  any    `json:"last_seen"`
}

func NewUniFiHTTPClient(httpClient HTTPEr) *UniFiHTTPClient {
	if httpClient == nil {
		jar, _ := cookiejar.New(nil)
		httpClient = &http.Client{
			Timeout: 10 * time.Second,
			Jar:     jar,
		}
	}

	return &UniFiHTTPClient{httpClient: httpClient}
}

func (c *UniFiHTTPClient) ListActiveClients(ctx context.Context, source config.PresenceSourceConfig) ([]UniFiActiveClient, error) {
	baseURL := unifiBaseURL(source)
	loginPayload := mustJSON(unifiLoginRequest{
		Username:   source.CredentialEnv.Username,
		Password:   source.CredentialEnv.Password,
		RememberMe: true,
		Token:      "",
	})

	loginEndpoints := []string{"/api/auth/login", "/api/login"}
	var loginErr error
	for _, endpoint := range loginEndpoints {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+endpoint, bytes.NewReader(loginPayload))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			loginErr = err
			continue
		}
		resp.Body.Close()

		if resp.StatusCode == http.StatusNotFound {
			loginErr = fmt.Errorf("unifi login endpoint %s returned 404", endpoint)
			continue
		}
		if resp.StatusCode >= 300 {
			return nil, fmt.Errorf("unifi login failed with status %d", resp.StatusCode)
		}

		loginErr = nil
		break
	}
	if loginErr != nil {
		return nil, loginErr
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/api/s/%s/stat/sta", baseURL, url.PathEscape(source.Site)), nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unifi active clients request returned status %d", resp.StatusCode)
	}

	var payload unifiResponse[unifiActiveClientPayload]
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}

	clients := make([]UniFiActiveClient, 0, len(payload.Data))
	for _, client := range payload.Data {
		clients = append(clients, UniFiActiveClient{
			MAC:       client.MAC,
			Hostname:  client.Hostname,
			ESSID:     client.ESSID,
			APMAC:     client.APMAC,
			BSSID:     client.BSSID,
			AssocTime: parseUniFiTimestamp(client.AssocTime),
			LastSeen:  parseUniFiTimestamp(client.LastSeen),
		})
	}

	return clients, nil
}

func unifiBaseURL(source config.PresenceSourceConfig) string {
	host := strings.TrimSpace(source.Host)
	if strings.HasPrefix(host, "http://") || strings.HasPrefix(host, "https://") {
		return strings.TrimRight(host, "/")
	}

	scheme := "https"
	if source.Port == 80 {
		scheme = "http"
	}

	if source.Port > 0 {
		return fmt.Sprintf("%s://%s:%d", scheme, host, source.Port)
	}
	return fmt.Sprintf("%s://%s", scheme, host)
}

func parseUniFiTimestamp(value any) time.Time {
	switch typed := value.(type) {
	case float64:
		return time.Unix(int64(typed), 0).UTC()
	case int64:
		return time.Unix(typed, 0).UTC()
	case int:
		return time.Unix(int64(typed), 0).UTC()
	case json.Number:
		if seconds, err := typed.Int64(); err == nil {
			return time.Unix(seconds, 0).UTC()
		}
	case string:
		trimmed := strings.TrimSpace(typed)
		if trimmed == "" {
			return time.Time{}
		}
		if seconds, err := strconv.ParseInt(trimmed, 10, 64); err == nil {
			return time.Unix(seconds, 0).UTC()
		}
	}
	return time.Time{}
}
