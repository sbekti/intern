package httpserver

import (
	"log/slog"

	"github.com/go-chi/chi/v5"
	"github.com/sbekti/intern-api/internal/auth"
)

func registerAPIRoutes(
	router chi.Router,
	logger *slog.Logger,
	authorizer *auth.Authorizer,
	vlanService VLANService,
	deviceService DeviceService,
	clientAuthService ClientAuthService,
	authSpamService AuthSpamService,
	sessionService SessionService,
	auditLogService AuditLogService,
) {
	router.Route("/api/v1", func(r chi.Router) {
		registerSystemProfileRoutes(r, authorizer, sessionService)
		registerAdminRoutes(r, authorizer, sessionService, auditLogService)
		registerNetworkRoutes(r, authorizer, vlanService, deviceService)
		registerDeviceAuthRoutes(r, logger, authorizer, clientAuthService, authSpamService)
	})
}
