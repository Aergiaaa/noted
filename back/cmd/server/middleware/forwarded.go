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

// IPV4_BITS/IPV6_BITS: host bits in each address family; a full-length
// mask (CIDRMask(bits, bits)) matches that one address and nothing else.
const (
	IPV4_BITS = 32
	IPV6_BITS = 128
)

// Forwarded resolves the client IP and HTTPS status from X-Forwarded-*
// headers, but only when the direct peer is listed in trustedProxies
// (comma-separated IPs and CIDRs, e.g. "127.0.0.1,10.0.0.0/8"). The
// result lands in the request context for ClientIP/IsSecure; without a
// trusted peer the headers are ignored entirely (SECURITY.md).
func Forwarded(trustedProxies string) Middleware {
	trusted := parseTrustedProxies(trustedProxies)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			resolveForwarded(w, r, next, trusted)
		})
	}
}

// resolveForwarded is Forwarded's handler body: resolve the client IP
// and scheme, stash both in the context, then pass the request on. A
// malformed remote address or forwarded hop is handled here, where the
// request id is available; the returned values stay fail-closed either
// way, so the request continues.
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

// parseTrustedProxies turns the env list into matchers. Unparsable
// entries are skipped: silently trusting nothing is safe, silently
// trusting something is not, and a typo must not go unnoticed.
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

// parseTrustedProxy parses one TRUSTED_PROXIES entry — IP or CIDR —
// and returns nil, with a warning naming the entry and its cause, when
// it is malformed.
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

// clientIPAndProto picks the client IP: the remote address unless the
// remote is a trusted proxy, in which case the rightmost X-Forwarded-For
// hop wins (the one the nearest proxy observed; anything to its left is
// client-supplied). X-Forwarded-Proto only counts from a trusted peer,
// and an "https" first element marks the hop secure. A malformed remote
// address or an unparsable forwarded hop is reported as an error for the
// caller to log; the returned values stay fail-closed regardless (peer
// address, no HTTPS).
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

// applyForwardedFor adopts the rightmost X-Forwarded-For hop when it
// parses as an IP; an unparsable hop is reported (unless an earlier
// error already names a cause) and the peer address kept. Client bytes
// stay out of the log.
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
