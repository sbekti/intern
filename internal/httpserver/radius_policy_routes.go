package httpserver

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/sbekti/intern/internal/config"
)

const radiusPolicySnapshotPath = "/api/v1/radius/policy-snapshot"

func registerRADIUSPolicyRoutes(router chi.Router, logger *slog.Logger, cfg config.RADIUSSnapshotConfig, service DeviceService, maxBytes int64) {
	router.With(radiusSnapshotTokenMiddleware(logger, cfg.TokenHashes)).Get(radiusPolicySnapshotPath, func(w http.ResponseWriter, r *http.Request) {
		if service == nil {
			http.Error(w, http.StatusText(http.StatusServiceUnavailable), http.StatusServiceUnavailable)
			return
		}
		records, err := service.RADIUSPolicy(r.Context())
		if err != nil {
			logger.Error("failed to build RADIUS policy snapshot", "error", err)
			http.Error(w, http.StatusText(http.StatusServiceUnavailable), http.StatusServiceUnavailable)
			return
		}
		snapshot := renderRADIUSSnapshot(records)
		if int64(len(snapshot)) >= maxBytes {
			logger.Error("RADIUS policy exceeds the configured snapshot size limit", "max_bytes", maxBytes)
			http.Error(w, http.StatusText(http.StatusServiceUnavailable), http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write(snapshot)
	})
}
