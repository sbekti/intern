package httpserver

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/sbekti/intern/internal/auth"
	"github.com/sbekti/intern/internal/config"
	"github.com/sbekti/intern/internal/identity"
	"github.com/sbekti/intern/internal/requestmeta"
)

func NewHandler(logger *slog.Logger, cfg config.Config, deps Dependencies) http.Handler {
	if logger == nil {
		logger = slog.Default()
	}

	authenticator := auth.NewAuthenticator(cfg)
	authorizer := auth.NewAuthorizer(cfg)
	clientIPResolver := requestmeta.NewIPResolver(cfg.TrustedProxy.CIDRs)
	userSyncer := identity.NewSyncer(deps.UserStore)
	vlanService := deps.VLANService
	deviceService := deps.DeviceService
	clientAuthService := deps.ClientAuthService
	authSpamService := deps.AuthSpamService
	sessionService := deps.SessionService
	auditLogService := deps.AuditLogService

	router := chi.NewRouter()
	router.Use(middleware.RequestID)
	router.Use(requestLogger(logger))
	router.Use(middleware.Recoverer)

	registerRADIUSMABRoutes(router, logger, cfg.RADIUSMAB, deviceService)

	router.Group(func(r chi.Router) {
		r.Use(clientInfoMiddleware(clientIPResolver))
		r.Use(authenticator.OptionalPrincipalMiddleware())
		r.Use(auth.RequireActiveBearerSession(deps.SessionService))
		r.Use(userSyncer.Middleware())

		r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, http.StatusOK, response{Status: "ok"})
		})

		r.Get("/readyz", func(w http.ResponseWriter, r *http.Request) {
			if deps.DatabasePinger == nil {
				writeJSON(w, http.StatusServiceUnavailable, response{Status: "unavailable"})
				return
			}
			if err := deps.DatabasePinger.Ping(r.Context()); err != nil {
				writeJSON(w, http.StatusServiceUnavailable, response{Status: "unavailable"})
				return
			}
			writeJSON(w, http.StatusOK, response{Status: "ok"})
		})

		registerAPIRoutes(r, logger, authorizer, vlanService, deviceService, clientAuthService, authSpamService, sessionService, auditLogService)
	})

	return router
}
