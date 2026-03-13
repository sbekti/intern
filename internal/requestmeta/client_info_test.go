package requestmeta

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"
)

func TestResolveUsesRemoteAddrForUntrustedProxy(t *testing.T) {
	t.Parallel()

	resolver := NewIPResolver([]netip.Prefix{netip.MustParsePrefix("127.0.0.1/32")})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "198.51.100.10:12345"
	req.Header.Set("X-Forwarded-For", "203.0.113.20")
	req.Header.Set("X-Real-IP", "203.0.113.21")

	info := resolver.Resolve(req)
	if info.IP != "198.51.100.10" || info.IPSource != SourceRemoteAddr {
		t.Fatalf("unexpected client info %#v", info)
	}
}

func TestResolveUsesFirstNonTrustedXForwardedForFromRight(t *testing.T) {
	t.Parallel()

	resolver := NewIPResolver([]netip.Prefix{
		netip.MustParsePrefix("127.0.0.1/32"),
		netip.MustParsePrefix("10.0.0.0/8"),
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	req.Header.Set("X-Forwarded-For", "198.51.100.20, 10.0.0.10, 127.0.0.1")

	info := resolver.Resolve(req)
	if info.IP != "198.51.100.20" || info.IPSource != SourceXForwardedFor {
		t.Fatalf("unexpected client info %#v", info)
	}
}

func TestResolveUsesLeftmostXForwardedForWhenAllAreTrusted(t *testing.T) {
	t.Parallel()

	resolver := NewIPResolver([]netip.Prefix{
		netip.MustParsePrefix("127.0.0.1/32"),
		netip.MustParsePrefix("10.0.0.0/8"),
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	req.Header.Set("X-Forwarded-For", "10.0.0.11, 127.0.0.1")

	info := resolver.Resolve(req)
	if info.IP != "10.0.0.11" || info.IPSource != SourceXForwardedFor {
		t.Fatalf("unexpected client info %#v", info)
	}
}

func TestResolveFallsBackToXRealIP(t *testing.T) {
	t.Parallel()

	resolver := NewIPResolver([]netip.Prefix{netip.MustParsePrefix("127.0.0.1/32")})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	req.Header.Set("X-Forwarded-For", "not-an-ip")
	req.Header.Set("X-Real-IP", "203.0.113.44")

	info := resolver.Resolve(req)
	if info.IP != "203.0.113.44" || info.IPSource != SourceXRealIP {
		t.Fatalf("unexpected client info %#v", info)
	}
}

func TestResolveFallsBackToRemoteAddrWhenHeadersInvalid(t *testing.T) {
	t.Parallel()

	resolver := NewIPResolver([]netip.Prefix{netip.MustParsePrefix("127.0.0.1/32")})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	req.Header.Set("X-Forwarded-For", "bad")
	req.Header.Set("X-Real-IP", "also-bad")

	info := resolver.Resolve(req)
	if info.IP != "127.0.0.1" || info.IPSource != SourceRemoteAddr {
		t.Fatalf("unexpected client info %#v", info)
	}
}

func TestContextRoundTrip(t *testing.T) {
	t.Parallel()

	info := ClientInfo{IP: "203.0.113.10", IPSource: SourceXForwardedFor}
	ctx := WithClientInfo(context.Background(), info)

	got, ok := FromContext(ctx)
	if !ok {
		t.Fatal("expected client info in context")
	}
	if got != info {
		t.Fatalf("client info = %#v, want %#v", got, info)
	}
}
