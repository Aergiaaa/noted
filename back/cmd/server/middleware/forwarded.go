package middleware

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
)

// A full-length mask (CIDRMask(bits, bits)) matches that one address and
// nothing else.
const (
	IPV4_BITS = 32
	IPV6_BITS = 128
)

// Forwarded resolves the client IP and HTTPS status from X-Forwarded-*
// headers only when the direct peer is in trustedProxies; without a
// trusted peer the headers are ignored entirely (SECURITY.md).
func Forwarded(trustedProxies string) Middleware {
	trusted := parseTrustedProxies(trustedProxies)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			resolveForwarded(w, r, next, trusted)
		})
	}
}

func resolveForwarded(w http.ResponseWriter, r *http.Request, next http.Handler, trusted []*net.IPNet) {
	ip, secure, err := clientIPAndProto(
		r.RemoteAddr,
		r.Header.Get("X-Forwarded-For"),
		r.Header.Get("X-Forwarded-Proto"),
		trusted,
	)
	if err != nil {
		log.Printf("FORWARDED rid=%s: %v; keeping peer address", RequestIDFrom(r), err)
	}

	ctx := context.WithValue(r.Context(), CLIENT_IP_KEY, ip)
	ctx = context.WithValue(ctx, SECURE_KEY, secure)

	next.ServeHTTP(w, r.WithContext(ctx))
}

// parseTrustedProxies skips unparsable entries: silently trusting nothing
// is safe, silently trusting something is not — and a typo must not go
// unnoticed.
func parseTrustedProxies(csv string) []*net.IPNet {
	var nets []*net.IPNet

	for entry := range strings.SplitSeq(csv, ",") {
		entry = strings.TrimSpace(entry)

		if entry == "" {
			continue
		}

		if n := parseTrustedProxy(entry); n != nil {
			nets = append(nets, n)
		}
	}

	return nets
}

func parseTrustedProxy(entry string) *net.IPNet {
	if strings.Contains(entry, "/") {
		_, n, err := net.ParseCIDR(entry)
		if err != nil {
			log.Printf("TRUSTED_PROXIES: invalid CIDR %q: %v", entry, err)
			return nil
		}

		return n
	}

	ip := net.ParseIP(entry)
	if ip == nil {
		log.Printf("TRUSTED_PROXIES: invalid IP %q", entry)
		return nil
	}

	bits := IPV6_BITS
	if v4 := ip.To4(); v4 != nil {
		ip, bits = v4, IPV4_BITS
	}
	return &net.IPNet{IP: ip, Mask: net.CIDRMask(bits, bits)}
}

// clientIPAndProto takes the rightmost X-Forwarded-For hop — the one the
// nearest proxy observed; anything to its left is client-supplied — and
// counts X-Forwarded-Proto only from a trusted peer. Malformed input is
// reported for the caller to log; the values stay fail-closed regardless
// (peer address, no HTTPS).
func clientIPAndProto(remoteAddr, xff, xfp string, trusted []*net.IPNet) (string, bool, error) {
	ip, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		// net/http always sends host:port; a bare/garbage value keeps the
		// whole string as candidate so isTrusted fails closed on it.
		ip = remoteAddr
		err = fmt.Errorf("malformed remote addr: %w", err)
	}
	if !isTrusted(net.ParseIP(ip), trusted) {
		return ip, false, err
	}

	ip, err = applyForwardedFor(ip, xff, err)
	proto := strings.TrimSpace(strings.Split(xfp, ",")[0])
	return ip, strings.EqualFold(proto, "https"), err
}

// applyForwardedFor reports a malformed rightmost hop only when no earlier
// cause exists, so client bytes stay out of the log; the peer address is
// kept either way.
func applyForwardedFor(ip, xff string, err error) (string, error) {
	if xff == "" {
		return ip, err
	}

	hops := strings.Split(xff, ",")
	cand := strings.TrimSpace(hops[len(hops)-1])
	if net.ParseIP(cand) == nil {
		if err != nil {
			return ip, err
		}
		return ip, errors.New("rightmost X-Forwarded-For hop is not an IP")
	}
	return cand, err
}

// isTrusted is nil-safe: unparsable input matches nothing.
func isTrusted(ip net.IP, trusted []*net.IPNet) bool {
	for _, n := range trusted {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}
