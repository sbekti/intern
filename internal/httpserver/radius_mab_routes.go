package httpserver

import (
	"crypto/sha256"
	"crypto/subtle"
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/sbekti/intern/internal/config"
	"github.com/sbekti/intern/internal/devices"
)

const radiusMABSnapshotPath = "/api/v1/radius/mab-snapshot"

func registerRADIUSMABRoutes(router chi.Router, logger *slog.Logger, cfg config.RADIUSMABConfig, deviceService DeviceService) {
	authenticate := radiusMABTokenMiddleware(logger, cfg.TokenHashes)
	handler := radiusMABSnapshotHandler(logger, deviceService)
	router.With(authenticate).Get(radiusMABSnapshotPath, handler)
}

func radiusMABTokenMiddleware(logger *slog.Logger, hashes []config.RADIUSMABTokenHash) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			scheme, token, ok := strings.Cut(r.Header.Get("Authorization"), " ")
			if !ok || !strings.EqualFold(scheme, "Bearer") || strings.TrimSpace(token) == "" {
				radiusMABUnauthorized(w)
				return
			}

			candidate := sha256.Sum256([]byte(strings.TrimSpace(token)))
			matchedSite := ""
			matched := 0
			for _, configured := range hashes {
				equal := subtle.ConstantTimeCompare(candidate[:], configured.SHA256[:])
				if equal == 1 {
					matchedSite = configured.Site
				}
				matched |= equal
			}
			if matched == 0 {
				radiusMABUnauthorized(w)
				return
			}

			logger.Info("RADIUS MAB snapshot request", "site", matchedSite)
			next.ServeHTTP(w, r)
		})
	}
}

func radiusMABUnauthorized(w http.ResponseWriter) {
	w.Header().Set("WWW-Authenticate", "Bearer")
	http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
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
	active := make([]devices.DeviceRecord, 0, len(records))
	for _, record := range records {
		if !record.Device.Disabled {
			active = append(active, record)
		}
	}
	sort.Slice(active, func(i, j int) bool {
		return active[i].Device.MacAddress < active[j].Device.MacAddress
	})

	var entries strings.Builder
	for _, record := range active {
		mac := strings.ToLower(strings.ReplaceAll(record.Device.MacAddress, ":", ""))
		fmt.Fprintf(&entries, "%s Cleartext-Password := \"%s\"\n", mac, mac)
		fmt.Fprintf(&entries, "\tTunnel-Type := VLAN,\n\tTunnel-Medium-Type := IEEE-802,\n\tTunnel-Private-Group-Id := \"%d\"\n\n", record.Device.VlanID)
	}

	entryText := entries.String()
	revision := sha256.Sum256([]byte(entryText))
	return []byte(fmt.Sprintf("# radius-mab-v1\n# revision=%x\n%s", revision, entryText))
}
