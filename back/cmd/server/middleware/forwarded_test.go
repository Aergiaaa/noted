package middleware

import (
	"bytes"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// --- parseTrustedProxies ---

func TestParseTrustedProxies_empty_noneTrusted(t *testing.T) {
	trusted := parseTrustedProxies("")

	if len(trusted) != 0 {
		t.Fatalf("len = %d, want 0", len(trusted))
	}
}

func TestParseTrustedProxies_ipsAndCIDRs_matched(t *testing.T) {
	trusted := parseTrustedProxies("127.0.0.1, 10.0.0.0/8, ::1")

	for _, ip := range []string{"127.0.0.1", "10.1.2.3", "::1"} {
		if !isTrusted(net.ParseIP(ip), trusted) {
			t.Errorf("%s trusted = false, want true", ip)
		}
	}
	for _, ip := range []string{"127.0.0.2", "11.0.0.1", "2001:db8::1"} {
		if isTrusted(net.ParseIP(ip), trusted) {
			t.Errorf("%s trusted = true, want false", ip)
		}
	}
}

func TestParseTrustedProxies_invalidEntry_skippedWithWarning(t *testing.T) {
	var buf bytes.Buffer
	old := log.Writer()
	log.SetOutput(&buf)
	defer log.SetOutput(old)

	trusted := parseTrustedProxies("not-an-ip,192.0.2.1")

	if !isTrusted(net.ParseIP("192.0.2.1"), trusted) {
		t.Error("valid entry next to invalid one was dropped")
	}
	if got := buf.String(); !strings.Contains(got, "ignoring invalid entry") {
		t.Fatalf("log = %q, want invalid-entry warning", got)
	}
}

// --- clientIPAndProto ---

func TestClientIPAndProto_untrustedRemote_ignoresForwardedHeaders(t *testing.T) {
	trusted := parseTrustedProxies("192.0.2.1")

	ip, secure := clientIPAndProto("198.51.100.7:44321", "203.0.113.9", "https", trusted)

	if ip != "198.51.100.7" {
		t.Fatalf("ip = %q, want remote addr (headers untrusted)", ip)
	}
	if secure {
		t.Fatal("secure = true, want false without trusted proxy")
	}
}

func TestClientIPAndProto_trustedRemote_usesRightmostForwardedFor(t *testing.T) {
	trusted := parseTrustedProxies("192.0.2.1")

	ip, _ := clientIPAndProto("192.0.2.1:9999", "203.0.113.9, 198.51.100.7", "", trusted)

	if ip != "198.51.100.7" {
		t.Fatalf("ip = %q, want rightmost (nearest-to-proxy) hop", ip)
	}
}

func TestClientIPAndProto_trustedRemote_fallsBackWhenForwardedForUnusable(t *testing.T) {
	trusted := parseTrustedProxies("192.0.2.1")

	for _, xff := range []string{"", "not-an-ip", "203.0.113.9,"} {
		ip, _ := clientIPAndProto("192.0.2.1:9999", xff, "", trusted)
		if ip != "192.0.2.1" {
			t.Errorf("xff %q → ip = %q, want proxy addr", xff, ip)
		}
	}
}

func TestClientIPAndProto_trustedRemote_noPortOnRemoteAddr(t *testing.T) {
	trusted := parseTrustedProxies("192.0.2.1")

	ip, _ := clientIPAndProto("192.0.2.1", "", "", trusted)

	if ip != "192.0.2.1" {
		t.Fatalf("ip = %q, want bare addr fallback", ip)
	}
}

func TestClientIPAndProto_forwardedProto_decidesSecure(t *testing.T) {
	trusted := parseTrustedProxies("192.0.2.1")

	for _, tt := range []struct {
		xfp  string
		want bool
	}{
		{"https", true},
		{"https, http", true},
		{"http", false},
		{"", false},
	} {
		t.Run(tt.xfp, func(t *testing.T) {
			_, secure := clientIPAndProto("192.0.2.1:1", "", tt.xfp, trusted)
			if secure != tt.want {
				t.Fatalf("secure(%q) = %v, want %v", tt.xfp, secure, tt.want)
			}
		})
	}
}

// --- Forwarded middleware + context accessors ---

func TestForwarded_exposesClientIPAndSecureInContext(t *testing.T) {
	var ip string
	var secure bool
	next := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		ip, secure = ClientIP(r), IsSecure(r)
	})

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil) // RemoteAddr 192.0.2.1:1234
	req.Header.Set("X-Forwarded-For", "203.0.113.9")
	req.Header.Set("X-Forwarded-Proto", "https")
	Forwarded("192.0.2.1")(next).ServeHTTP(httptest.NewRecorder(), req)

	if ip != "203.0.113.9" {
		t.Fatalf("ClientIP = %q, want 203.0.113.9", ip)
	}
	if !secure {
		t.Fatal("IsSecure = false, want true behind trusted https proxy")
	}
}

func TestAccessors_withoutMiddleware_defaults(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	if got := ClientIP(req); got != "" {
		t.Errorf("ClientIP = %q, want empty", got)
	}
	if IsSecure(req) {
		t.Error("IsSecure = true, want false")
	}
}
