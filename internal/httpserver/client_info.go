package httpserver

import (
	"net/http"

	"github.com/sbekti/intern-api/internal/requestmeta"
)

func clientInfoMiddleware(resolver *requestmeta.IPResolver) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if resolver == nil {
				next.ServeHTTP(w, r)
				return
			}

			info := resolver.Resolve(r)
			next.ServeHTTP(w, r.WithContext(requestmeta.WithClientInfo(r.Context(), info)))
		})
	}
}
