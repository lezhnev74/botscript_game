package httpapi

import (
	"net"
	"net/http"
	"net/netip"
	"strings"
)

// proxies are the reverse proxies whose X-Forwarded-For and
// X-Forwarded-Proto headers are believed. Without any, the headers are
// ignored: anyone could set them to dodge a rate limit.
type proxies []netip.Prefix

// clientHost is the address of the client: the TCP peer, or, when that is a
// trusted proxy, the rightmost X-Forwarded-For entry that is not one (the
// entries left of it are whatever the client chose to send).
func (p proxies) clientHost(r *http.Request) string {
	host := peerHost(r)
	if !p.trusts(host) {
		return host
	}
	hops := strings.Split(strings.Join(r.Header.Values("X-Forwarded-For"), ","), ",")
	for i := len(hops) - 1; i >= 0; i-- {
		hop := strings.TrimSpace(hops[i])
		if hop == "" {
			continue
		}
		if !p.trusts(hop) {
			return hop
		}
		host = hop
	}
	return host
}

// https reports whether the client reached us over TLS, directly or through a
// trusted proxy that says so.
func (p proxies) https(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	return p.trusts(peerHost(r)) && strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
}

func (p proxies) trusts(host string) bool {
	addr, err := netip.ParseAddr(host)
	if err != nil {
		return false
	}
	addr = addr.Unmap()
	for _, prefix := range p {
		if prefix.Contains(addr) {
			return true
		}
	}
	return false
}

// peerHost is the TCP peer address without the port.
func peerHost(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
