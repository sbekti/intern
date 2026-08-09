package httpserver

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/sbekti/intern/internal/authspam"
	"github.com/sbekti/intern/internal/requestmeta"
)

type deviceFlowRateLimitScope string

const (
	deviceFlowRateLimitCreate   deviceFlowRateLimitScope = "create"
	deviceFlowRateLimitExchange deviceFlowRateLimitScope = "exchange"
	deviceFlowRateLimitDecision deviceFlowRateLimitScope = "decision"
	deviceFlowRateLimitRefresh  deviceFlowRateLimitScope = "refresh"
	deviceFlowRateLimitLogout   deviceFlowRateLimitScope = "logout"
)

func enforceDeviceFlowRateLimit(r *http.Request, limiter AuthSpamService, scope deviceFlowRateLimitScope, username string) error {
	if limiter == nil {
		return nil
	}

	clientInfo, _ := requestmeta.FromContext(r.Context())

	switch scope {
	case deviceFlowRateLimitCreate:
		return limiter.CheckDeviceCodeCreate(r.Context(), clientInfo)
	case deviceFlowRateLimitExchange:
		return limiter.CheckDeviceTokenExchange(r.Context(), clientInfo)
	case deviceFlowRateLimitDecision:
		return limiter.CheckDeviceDecision(r.Context(), username, clientInfo)
	case deviceFlowRateLimitRefresh:
		return limiter.CheckRefreshToken(r.Context(), clientInfo)
	case deviceFlowRateLimitLogout:
		return limiter.CheckLogout(r.Context(), clientInfo)
	default:
		return nil
	}
}

func handleAuthSpamError(w http.ResponseWriter, logger *slog.Logger, r *http.Request, scope, username string, err error) {
	var limitedErr authspam.RateLimitedError
	if errors.As(err, &limitedErr) {
		retryAfterSeconds := int(limitedErr.RetryAfter.Round(time.Second) / time.Second)
		if retryAfterSeconds < 1 {
			retryAfterSeconds = 1
		}
		logAuthRateLimit(logger, r, scope, username, retryAfterSeconds)
		w.Header().Set("Retry-After", strconv.Itoa(retryAfterSeconds))
		writeAPIError(w, http.StatusTooManyRequests, "too_many_requests", "too many requests")
		return
	}

	logger.Error("auth rate limiter failed", "path", r.URL.Path, "method", r.Method, "scope", scope, "username", username, "error", err)
	writeAPIError(w, http.StatusInternalServerError, "internal_error", "client auth rate limiter failed")
}

func logAuthRateLimit(logger *slog.Logger, r *http.Request, scope, username string, retryAfterSeconds int) {
	clientInfo, _ := requestmeta.FromContext(r.Context())
	logger.Warn(
		"auth rate limit exceeded",
		"scope", scope,
		"method", r.Method,
		"path", r.URL.Path,
		"username", username,
		"client_ip", clientInfo.IP,
		"client_ip_source", clientInfo.IPSource,
		"retry_after_seconds", retryAfterSeconds,
	)
}
