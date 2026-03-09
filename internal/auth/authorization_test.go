package auth

import (
	"testing"

	"github.com/sbekti/intern-api/internal/config"
)

func TestIsAdmin(t *testing.T) {
	t.Parallel()

	authorizer := NewAuthorizer(config.Config{
		Authorization: config.AuthorizationConfig{
			AdminGroups: []string{"Super-Users"},
		},
	})

	if !authorizer.IsAdmin(&Principal{Groups: []string{"Users", "Super-Users"}}) {
		t.Fatal("expected principal to be admin")
	}

	if authorizer.IsAdmin(&Principal{Groups: []string{"Users"}}) {
		t.Fatal("expected principal not to be admin")
	}
}
