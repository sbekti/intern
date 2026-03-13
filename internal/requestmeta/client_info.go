package requestmeta

import (
	"context"
	"net"
	"net/http"
	"net/netip"
	"strings"
)

const (
	SourceRemoteAddr    = "remote_addr"
	SourceXForwardedFor = "x_forwarded_for"
	SourceXRealIP       = "x_real_ip"
)

type ClientInfo struct {
	IP       string
	IPSource string
}

type contextKey string

const clientInfoContextKey contextKey = "requestmeta.client_info"

type IPResolver struct {
	trustedProxyCIDRs []netip.Prefix
}

func NewIPResolver(trustedProxyCIDRs []netip.Prefix) *IPResolver {
	return &IPResolver{trustedProxyCIDRs: append([]netip.Prefix(nil), trustedProxyCIDRs...)}
}

func WithClientInfo(ctx context.Context, info ClientInfo) context.Context {
	return context.WithValue(ctx, clientInfoContextKey, info)
}

func FromContext(ctx context.Context) (ClientInfo, bool) {
	info, ok := ctx.Value(clientInfoContextKey).(ClientInfo)
	if !ok {
		return ClientInfo{}, false
	}
	return info, true
}

func (r *IPResolver) Resolve(req *http.Request) ClientInfo {
	remoteIP, remoteOK := parseAddr(req.RemoteAddr)
	if !remoteOK || !r.IsTrustedProxyRequest(req.RemoteAddr) {
		if !remoteOK {
			return ClientInfo{}
		}
		return ClientInfo{IP: remoteIP.String(), IPSource: SourceRemoteAddr}
	}

	if ip, ok := r.resolveXForwardedFor(req.Header.Get("X-Forwarded-For")); ok {
		return ClientInfo{IP: ip.String(), IPSource: SourceXForwardedFor}
	}

	if ip, ok := parseAddr(req.Header.Get("X-Real-IP")); ok {
		return ClientInfo{IP: ip.String(), IPSource: SourceXRealIP}
	}

	if remoteOK {
		return ClientInfo{IP: remoteIP.String(), IPSource: SourceRemoteAddr}
	}

	return ClientInfo{}
}

func (r *IPResolver) IsTrustedProxyRequest(remoteAddr string) bool {
	addr, ok := parseAddr(remoteAddr)
	if !ok {
		return false
	}

	for _, prefix := range r.trustedProxyCIDRs {
		if prefix.Contains(addr) {
			return true
		}
	}

	return false
}

func (r *IPResolver) resolveXForwardedFor(headerValue string) (netip.Addr, bool) {
	parts := strings.Split(headerValue, ",")
	parsed := make([]netip.Addr, 0, len(parts))
	for _, part := range parts {
		addr, ok := parseAddr(part)
		if ok {
			parsed = append(parsed, addr)
		}
	}
	if len(parsed) == 0 {
		return netip.Addr{}, false
	}

	for i := len(parsed) - 1; i >= 0; i-- {
		if !r.isTrustedAddr(parsed[i]) {
			return parsed[i], true
		}
	}

	return parsed[0], true
}

func (r *IPResolver) isTrustedAddr(addr netip.Addr) bool {
	for _, prefix := range r.trustedProxyCIDRs {
		if prefix.Contains(addr) {
			return true
		}
	}
	return false
}

func parseAddr(raw string) (netip.Addr, bool) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return netip.Addr{}, false
	}

	if addr, err := netip.ParseAddr(trimmed); err == nil {
		return addr, true
	}

	host, _, err := net.SplitHostPort(trimmed)
	if err != nil {
		return netip.Addr{}, false
	}

	addr, err := netip.ParseAddr(strings.TrimSpace(host))
	if err != nil {
		return netip.Addr{}, false
	}
	return addr, true
}
