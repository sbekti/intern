package auth

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"strings"
	"time"

	jwt "github.com/golang-jwt/jwt/v5"

	"github.com/sbekti/intern-api/internal/config"
)

var (
	ErrMissingCredentials = errors.New("missing credentials")
	ErrUntrustedProxy     = errors.New("request did not originate from a trusted proxy")
	ErrMissingSubject     = errors.New("token missing subject")
)

type Authenticator struct {
	trustedProxyCIDRs []netip.Prefix
	userHeader        string
	nameHeader        string
	emailHeader       string
	groupsHeader      string
	jwtIssuer         string
	jwtAudience       string
	jwtHMACSecret     []byte
	now               func() time.Time
}

type AccessTokenClaims struct {
	Username     string   `json:"username"`
	Name         string   `json:"name"`
	Email        string   `json:"email"`
	Groups       []string `json:"groups"`
	Scopes       []string `json:"scopes"`
	SessionID    string   `json:"session_id"`
	TokenVersion int      `json:"token_version"`
	jwt.RegisteredClaims
}

func NewAuthenticator(cfg config.Config) *Authenticator {
	return &Authenticator{
		trustedProxyCIDRs: cfg.TrustedProxy.CIDRs,
		userHeader:        cfg.TrustedProxy.UserHeader,
		nameHeader:        cfg.TrustedProxy.NameHeader,
		emailHeader:       cfg.TrustedProxy.EmailHeader,
		groupsHeader:      cfg.TrustedProxy.GroupsHeader,
		jwtIssuer:         cfg.Auth.JWTIssuer,
		jwtAudience:       cfg.Auth.JWTAudience,
		jwtHMACSecret:     []byte(cfg.Auth.JWTHMACSecret),
		now:               time.Now,
	}
}

func (a *Authenticator) OptionalPrincipalMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			principal, err := a.AuthenticateRequest(r)
			if err != nil {
				if errors.Is(err, ErrMissingCredentials) {
					next.ServeHTTP(w, r)
					return
				}
				http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
				return
			}

			next.ServeHTTP(w, r.WithContext(NewContext(r.Context(), principal)))
		})
	}
}

func (a *Authenticator) AuthenticateRequest(r *http.Request) (*Principal, error) {
	authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
	if authHeader != "" {
		return a.authenticateBearer(authHeader)
	}

	if a.hasForwardAuthHeaders(r.Header) {
		if !a.isTrustedProxyRequest(r.RemoteAddr) {
			return nil, ErrUntrustedProxy
		}
		return a.authenticateForwardHeaders(r.Header)
	}

	return nil, ErrMissingCredentials
}

func (a *Authenticator) authenticateBearer(authHeader string) (*Principal, error) {
	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || strings.TrimSpace(parts[1]) == "" {
		return nil, fmt.Errorf("invalid authorization header")
	}

	claims := &AccessTokenClaims{}
	token, err := jwt.ParseWithClaims(parts[1], claims, func(token *jwt.Token) (any, error) {
		if token.Method == nil {
			return nil, fmt.Errorf("missing signing method")
		}
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method %q", token.Method.Alg())
		}
		return a.jwtHMACSecret, nil
	},
		jwt.WithAudience(a.jwtAudience),
		jwt.WithIssuer(a.jwtIssuer),
		jwt.WithTimeFunc(a.now),
	)
	if err != nil {
		return nil, err
	}
	if !token.Valid {
		return nil, fmt.Errorf("token invalid")
	}
	if strings.TrimSpace(claims.Subject) == "" {
		return nil, ErrMissingSubject
	}

	username := strings.TrimSpace(claims.Username)
	if username == "" {
		username = claims.Subject
	}

	return &Principal{
		Subject:      claims.Subject,
		Username:     username,
		Name:         strings.TrimSpace(claims.Name),
		Email:        strings.TrimSpace(claims.Email),
		Groups:       compactStrings(claims.Groups),
		Scopes:       compactStrings(claims.Scopes),
		SessionID:    strings.TrimSpace(claims.SessionID),
		TokenVersion: claims.TokenVersion,
		Source:       PrincipalSourceBearerJWT,
	}, nil
}

func (a *Authenticator) authenticateForwardHeaders(header http.Header) (*Principal, error) {
	username := strings.TrimSpace(header.Get(a.userHeader))
	if username == "" {
		return nil, fmt.Errorf("missing %s header", a.userHeader)
	}

	return &Principal{
		Subject:  username,
		Username: username,
		Name:     strings.TrimSpace(header.Get(a.nameHeader)),
		Email:    strings.TrimSpace(header.Get(a.emailHeader)),
		Groups:   splitCommaSeparated(header.Get(a.groupsHeader)),
		Source:   PrincipalSourceForwardAuth,
	}, nil
}

func (a *Authenticator) hasForwardAuthHeaders(header http.Header) bool {
	return strings.TrimSpace(header.Get(a.userHeader)) != "" ||
		strings.TrimSpace(header.Get(a.nameHeader)) != "" ||
		strings.TrimSpace(header.Get(a.emailHeader)) != "" ||
		strings.TrimSpace(header.Get(a.groupsHeader)) != ""
}

func (a *Authenticator) isTrustedProxyRequest(remoteAddr string) bool {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		host = remoteAddr
	}

	addr, err := netip.ParseAddr(strings.TrimSpace(host))
	if err != nil {
		return false
	}

	for _, prefix := range a.trustedProxyCIDRs {
		if prefix.Contains(addr) {
			return true
		}
	}

	return false
}

func splitCommaSeparated(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}

	parts := strings.Split(raw, ",")
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			values = append(values, trimmed)
		}
	}
	return values
}

func compactStrings(values []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}
