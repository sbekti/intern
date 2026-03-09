package auth

import "context"

type PrincipalSource string

const (
	PrincipalSourceForwardAuth PrincipalSource = "forward_auth"
	PrincipalSourceBearerJWT   PrincipalSource = "bearer_jwt"
)

type Principal struct {
	Subject      string
	Username     string
	Name         string
	Email        string
	Groups       []string
	Scopes       []string
	SessionID    string
	TokenVersion int
	Source       PrincipalSource
}

type contextKey string

const principalContextKey contextKey = "principal"

func NewContext(ctx context.Context, principal *Principal) context.Context {
	return context.WithValue(ctx, principalContextKey, principal)
}

func FromContext(ctx context.Context) (*Principal, bool) {
	principal, ok := ctx.Value(principalContextKey).(*Principal)
	if !ok || principal == nil {
		return nil, false
	}
	return principal, true
}
