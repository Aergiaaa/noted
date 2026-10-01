package middleware

import (
	"context"
	"log"
	"net"
	"net/http"
	"strings"
)

// Forwarded resolves the client IP and HTTPS status from X-Forwarded-*
// headers, but only when the direct peer is listed in trustedProxies
// (comma-separated IPs and CIDRs, e.g. "127.0.0.1,10.0.0.0/8"). The
// result lands in the request context for ClientIP/IsSecure; without a
// trusted peer the headers are ignored entirely (SECURITY.md).
func Forwarded(trustedProxies string) func(http.Handler) http.Handler {
	trusted := parseTrustedProxies(trustedProxies)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip, secure := clientIPAndProto(r.RemoteAddr, r.Header.Get("X-Forwarded-For"), r.Header.Get("X-Forwarded-Proto"), trusted)
			ctx := context.WithValue(r.Context(), clientIPKey, ip)
			ctx = context.WithValue(ctx, secureKey, secure)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// parseTrustedProxies turns the env list into matchers. Unparsable
// entries are skipped with a warning: silently trusting nothing is safe,
// silently trusting something is not, and a typo must not go unnoticed.
func parseTrustedProxies(csv string) []*net.IPNet {
	var nets []*net.IPNet
	for _, entry := range strings.Split(csv, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		if strings.Contains(entry, "/") {
			if _, n, err := net.ParseCIDR(entry); err == nil {
				nets = append(nets, n)
				continue
			}
		} else if ip := net.ParseIP(entry); ip != nil {
			bits := 128
			if v4 := ip.To4(); v4 != nil {
				ip, bits = v4, 32
			}
			nets = append(nets, &net.IPNet{IP: ip, Mask: net.CIDRMask(bits, bits)})
			continue
		}
		log.Printf("TRUSTED_PROXIES: ignoring invalid entry %q", entry)
	}
	return nets
}

// isTrusted reports whether ip matches any trusted network (nil-safe:
// unparsable input matches nothing).
func isTrusted(ip net.IP, trusted []*net.IPNet) bool {
	for _, n := range trusted {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}

// clientIPAndProto picks the client IP: the remote address unless the
// remote is a trusted proxy, in which case the rightmost X-Forwarded-For
// hop wins (the one the nearest proxy observed; anything to its left is
// client-supplied). X-Forwarded-Proto only counts from a trusted peer,
// and an "https" first element marks the hop secure.
func clientIPAndProto(remoteAddr, xff, xfp string, trusted []*net.IPNet) (ip string, secure bool) {
	ip, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		ip = remoteAddr
	}
	if !isTrusted(net.ParseIP(ip), trusted) {
		return ip, false
	}

	if xff != "" {
		hops := strings.Split(xff, ",")
		if cand := strings.TrimSpace(hops[len(hops)-1]); net.ParseIP(cand) != nil {
			ip = cand
		}
	}
	proto := strings.TrimSpace(strings.Split(xfp, ",")[0])
	return ip, strings.EqualFold(proto, "https")
}
