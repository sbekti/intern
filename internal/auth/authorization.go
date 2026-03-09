package auth

import (
	"strings"

	"github.com/sbekti/intern-api/internal/config"
)

type Authorizer struct {
	adminGroups []string
}

func NewAuthorizer(cfg config.Config) *Authorizer {
	groups := make([]string, 0, len(cfg.Authorization.AdminGroups))
	for _, group := range cfg.Authorization.AdminGroups {
		trimmed := strings.TrimSpace(group)
		if trimmed != "" {
			groups = append(groups, trimmed)
		}
	}
	return &Authorizer{adminGroups: groups}
}

func (a *Authorizer) IsAdmin(principal *Principal) bool {
	if principal == nil {
		return false
	}

	for _, group := range principal.Groups {
		trimmed := strings.TrimSpace(group)
		for _, adminGroup := range a.adminGroups {
			if trimmed == adminGroup {
				return true
			}
		}
	}

	return false
}
