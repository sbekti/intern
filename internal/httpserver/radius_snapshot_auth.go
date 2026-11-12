package httpserver

import (
	"crypto/sha256"
	"crypto/subtle"
	"log/slog"
	"net/http"
	"strings"

	"github.com/sbekti/intern/internal/config"
)

func radiusSnapshotTokenMiddleware(logger *slog.Logger, hashes []config.RADIUSTokenHash) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Cache-Control", "no-store")
			scheme, token, ok := strings.Cut(r.Header.Get("Authorization"), " ")
			if !ok || !strings.EqualFold(scheme, "Bearer") || strings.TrimSpace(token) == "" {
				radiusSnapshotUnauthorized(w)
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
				radiusSnapshotUnauthorized(w)
				return
			}

			logger.Info("RADIUS snapshot request", "site", matchedSite, "path", r.URL.Path)
			next.ServeHTTP(w, r)
		})
	}
}

func radiusSnapshotUnauthorized(w http.ResponseWriter) {
	w.Header().Set("WWW-Authenticate", "Bearer")
	http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
}
