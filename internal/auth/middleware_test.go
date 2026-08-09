package auth

import (
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"
	"time"

	jwt "github.com/golang-jwt/jwt/v5"

	"github.com/sbekti/intern/internal/config"
)

func TestAuthenticateRequestForwardHeaders(t *testing.T) {
	t.Parallel()

	authenticator := NewAuthenticator(config.Config{
		Auth: config.AuthConfig{
			JWTIssuer:     "intern.corp.example.com",
			JWTAudience:   "internctl",
			JWTHMACSecret: "test-secret",
		},
		TrustedProxy: config.TrustedProxyConfig{
			CIDRs:        []netip.Prefix{netip.MustParsePrefix("127.0.0.1/32")},
			UserHeader:   "Remote-User",
			NameHeader:   "Remote-Name",
			EmailHeader:  "Remote-Email",
			GroupsHeader: "Remote-Groups",
			MarkerHeader: "X-Intern-Forward-Auth",
			MarkerValue:  "authenticated-ingress",
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	req.Header.Set("Remote-User", "alice")
	req.Header.Set("Remote-Name", "Alice Example")
	req.Header.Set("Remote-Email", "alice@example.com")
	req.Header.Set("Remote-Groups", "Super-Users, Operators")
	req.Header.Set("X-Intern-Forward-Auth", "authenticated-ingress")

	principal, err := authenticator.AuthenticateRequest(req)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if principal.Username != "alice" {
		t.Fatalf("expected username alice, got %q", principal.Username)
	}
	if len(principal.Groups) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(principal.Groups))
	}
	if principal.Source != PrincipalSourceForwardAuth {
		t.Fatalf("expected forward auth source, got %q", principal.Source)
	}
}

func TestAuthenticateRequestRejectsUntrustedForwardHeaders(t *testing.T) {
	t.Parallel()

	authenticator := NewAuthenticator(config.Config{
		Auth: config.AuthConfig{
			JWTIssuer:     "intern.corp.example.com",
			JWTAudience:   "internctl",
			JWTHMACSecret: "test-secret",
		},
		TrustedProxy: config.TrustedProxyConfig{
			CIDRs:        []netip.Prefix{netip.MustParsePrefix("127.0.0.1/32")},
			UserHeader:   "Remote-User",
			NameHeader:   "Remote-Name",
			EmailHeader:  "Remote-Email",
			GroupsHeader: "Remote-Groups",
			MarkerHeader: "X-Intern-Forward-Auth",
			MarkerValue:  "authenticated-ingress",
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.10:12345"
	req.Header.Set("Remote-User", "alice")
	req.Header.Set("X-Intern-Forward-Auth", "authenticated-ingress")

	_, err := authenticator.AuthenticateRequest(req)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err != ErrUntrustedProxy {
		t.Fatalf("expected ErrUntrustedProxy, got %v", err)
	}
}

func TestAuthenticateRequestRejectsMissingTrustedMarker(t *testing.T) {
	t.Parallel()

	authenticator := NewAuthenticator(config.Config{
		Auth: config.AuthConfig{
			JWTIssuer:     "intern.corp.example.com",
			JWTAudience:   "internctl",
			JWTHMACSecret: "test-secret",
		},
		TrustedProxy: config.TrustedProxyConfig{
			CIDRs:        []netip.Prefix{netip.MustParsePrefix("127.0.0.1/32")},
			UserHeader:   "Remote-User",
			NameHeader:   "Remote-Name",
			EmailHeader:  "Remote-Email",
			GroupsHeader: "Remote-Groups",
			MarkerHeader: "X-Intern-Forward-Auth",
			MarkerValue:  "authenticated-ingress",
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	req.Header.Set("Remote-User", "alice")

	_, err := authenticator.AuthenticateRequest(req)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err != ErrMissingTrustedMarker {
		t.Fatalf("expected ErrMissingTrustedMarker, got %v", err)
	}
}

func TestAuthenticateRequestBearerToken(t *testing.T) {
	t.Parallel()

	authenticator := NewAuthenticator(config.Config{
		Auth: config.AuthConfig{
			JWTIssuer:     "intern.corp.example.com",
			JWTAudience:   "internctl",
			JWTHMACSecret: "test-secret",
		},
		TrustedProxy: config.TrustedProxyConfig{
			CIDRs:        []netip.Prefix{netip.MustParsePrefix("127.0.0.1/32")},
			UserHeader:   "Remote-User",
			NameHeader:   "Remote-Name",
			EmailHeader:  "Remote-Email",
			GroupsHeader: "Remote-Groups",
			MarkerHeader: "X-Intern-Forward-Auth",
			MarkerValue:  "authenticated-ingress",
		},
	})
	authenticator.now = func() time.Time {
		return time.Date(2026, 3, 8, 12, 0, 0, 0, time.UTC)
	}

	claims := AccessTokenClaims{
		Username:  "alice",
		Name:      "Alice Example",
		Email:     "alice@example.com",
		Groups:    []string{"Super-Users"},
		Scopes:    []string{"devices:write"},
		SessionID: "session-1",
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "alice",
			Issuer:    "intern.corp.example.com",
			Audience:  []string{"internctl"},
			ExpiresAt: jwt.NewNumericDate(time.Date(2026, 3, 8, 13, 0, 0, 0, time.UTC)),
			IssuedAt:  jwt.NewNumericDate(time.Date(2026, 3, 8, 11, 0, 0, 0, time.UTC)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, err := token.SignedString([]byte("test-secret"))
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+signedToken)

	principal, err := authenticator.AuthenticateRequest(req)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if principal.Source != PrincipalSourceBearerJWT {
		t.Fatalf("expected bearer source, got %q", principal.Source)
	}
	if principal.SessionID != "session-1" {
		t.Fatalf("expected session ID session-1, got %q", principal.SessionID)
	}
}
