package httpserver

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/sbekti/intern/internal/config"
	"github.com/sbekti/intern/internal/devices"
)

const radiusMABSnapshotPath = "/api/v1/radius/mab-snapshot"

func registerRADIUSMABRoutes(router chi.Router, logger *slog.Logger, cfg config.RADIUSSnapshotConfig, deviceService DeviceService) {
	authenticate := radiusSnapshotTokenMiddleware(logger, cfg.TokenHashes)
	handler := radiusMABSnapshotHandler(logger, deviceService)
	router.With(authenticate).Get(radiusMABSnapshotPath, handler)
}

func radiusMABSnapshotHandler(logger *slog.Logger, deviceService DeviceService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if deviceService == nil {
			http.Error(w, http.StatusText(http.StatusServiceUnavailable), http.StatusServiceUnavailable)
			return
		}

		records, err := deviceService.List(r.Context())
		if err != nil {
			logger.Error("failed to build RADIUS MAB snapshot", "error", err)
			http.Error(w, http.StatusText(http.StatusServiceUnavailable), http.StatusServiceUnavailable)
			return
		}

		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write(renderRADIUSMABSnapshot(records))
	}
}

func renderRADIUSMABSnapshot(records []devices.DeviceRecord) []byte {
	active := make([]devices.RADIUSPolicyRecord, 0, len(records))
	for _, record := range records {
		if !record.Device.Disabled {
			active = append(active, devices.RADIUSPolicyRecord{MACAddress: record.Device.MacAddress, VLANID: record.Device.VlanID})
		}
	}
	return renderRADIUSSnapshot("radius-mab-v1", active)
}
