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

// TEST_PROXY_PEER is the trusted proxy's address — also httptest's
// default RemoteAddr — the only peer whose X-Forwarded-* count.
// TEST_CLIENT_IP is the client that proxy forwards. Both are RFC 5737
// documentation addresses.
const (
	TEST_PROXY_PEER = "192.0.2.1"
	TEST_CLIENT_IP  = "203.0.113.9"
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

	trusted := parseTrustedProxies("not-an-ip," + TEST_PROXY_PEER)

	if !isTrusted(net.ParseIP(TEST_PROXY_PEER), trusted) {
		t.Error("valid entry next to invalid one was dropped")
	}
	if got := buf.String(); !strings.Contains(got, `TRUSTED_PROXIES: invalid IP "not-an-ip"`) {
		t.Fatalf("log = %q, want invalid-IP warning naming the entry", got)
	}
}

func TestParseTrustedProxies_invalidCIDR_skippedWithUnderlyingCause(t *testing.T) {
	var buf bytes.Buffer
	old := log.Writer()
	log.SetOutput(&buf)
	defer log.SetOutput(old)

	trusted := parseTrustedProxies("999.0.0.0/8")

	if len(trusted) != 0 {
		t.Fatalf("len = %d, want 0", len(trusted))
	}
	got := buf.String()
	for _, want := range []string{`invalid CIDR "999.0.0.0/8"`, "invalid CIDR address"} {
		if !strings.Contains(got, want) {
			t.Errorf("log = %q, missing %q", got, want)
		}
	}
}

// --- clientIPAndProto ---

func TestClientIPAndProto_untrustedRemote_ignoresForwardedHeaders(t *testing.T) {
	trusted := parseTrustedProxies(TEST_PROXY_PEER)

	ip, secure, err := clientIPAndProto("198.51.100.7:44321", TEST_CLIENT_IP, "https", trusted)

	if err != nil {
		t.Fatalf("err = %v, want nil for well-formed remote", err)
	}
	if ip != "198.51.100.7" {
		t.Fatalf("ip = %q, want remote addr (headers untrusted)", ip)
	}
	if secure {
		t.Fatal("secure = true, want false without trusted proxy")
	}
}

func TestClientIPAndProto_trustedRemote_usesRightmostForwardedFor(t *testing.T) {
	trusted := parseTrustedProxies(TEST_PROXY_PEER)

	ip, _, err := clientIPAndProto(TEST_PROXY_PEER+":9999", TEST_CLIENT_IP+", 198.51.100.7", "", trusted)

	if err != nil {
		t.Fatalf("err = %v, want nil for parsable hops", err)
	}
	if ip != "198.51.100.7" {
		t.Fatalf("ip = %q, want rightmost (nearest-to-proxy) hop", ip)
	}
}

func TestClientIPAndProto_trustedRemote_fallsBackWhenForwardedForUnusable(t *testing.T) {
	trusted := parseTrustedProxies(TEST_PROXY_PEER)

	for _, xff := range []string{"", "not-an-ip", TEST_CLIENT_IP + ","} {
		ip, _, err := clientIPAndProto(TEST_PROXY_PEER+":9999", xff, "", trusted)
		if ip != TEST_PROXY_PEER {
			t.Errorf("xff %q → ip = %q, want proxy addr", xff, ip)
		}
		if xff == "" && err != nil {
			t.Errorf("xff %q → err = %v, want nil (header absent)", xff, err)
		}
		if xff != "" && err == nil {
			t.Errorf("xff %q → err = nil, want unparsable-hop error", xff)
		}
	}
}

func TestClientIPAndProto_trustedRemote_noPortOnRemoteAddr(t *testing.T) {
	trusted := parseTrustedProxies(TEST_PROXY_PEER)

	ip, _, err := clientIPAndProto(TEST_PROXY_PEER, "", "", trusted)

	if ip != TEST_PROXY_PEER {
		t.Fatalf("ip = %q, want bare addr fallback", ip)
	}
	if err == nil || !strings.Contains(err.Error(), "malformed remote addr") {
		t.Fatalf("err = %v, want malformed-remote-addr error", err)
	}
}

func TestClientIPAndProto_earlierErrorWinsOverUnparsableHop(t *testing.T) {
	trusted := parseTrustedProxies(TEST_PROXY_PEER)

	_, _, err := clientIPAndProto(TEST_PROXY_PEER, "not-an-ip", "", trusted)

	if err == nil || !strings.Contains(err.Error(), "malformed remote addr") {
		t.Fatalf("err = %v, want the first (remote addr) cause kept", err)
	}
}

func TestClientIPAndProto_forwardedProto_decidesSecure(t *testing.T) {
	trusted := parseTrustedProxies(TEST_PROXY_PEER)

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
			_, secure, err := clientIPAndProto(TEST_PROXY_PEER+":1", "", tt.xfp, trusted)
			if err != nil {
				t.Fatalf("err = %v, want nil", err)
			}
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
	req.Header.Set("X-Forwarded-For", TEST_CLIENT_IP)
	req.Header.Set("X-Forwarded-Proto", "https")
	Forwarded(TEST_PROXY_PEER)(next).ServeHTTP(httptest.NewRecorder(), req)

	if ip != TEST_CLIENT_IP {
		t.Fatalf("ClientIP = %q, want %s", ip, TEST_CLIENT_IP)
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

// --- resolveForwarded error handling ---

func TestForwarded_malformedRemoteAddr_loggedWithRIDAndFailsClosed(t *testing.T) {
	var got string
	next := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		got = ClientIP(r)
	})

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req.RemoteAddr = TEST_PROXY_PEER // bare, no port: SplitHostPort fails

	rec := httptest.NewRecorder()
	var out string
	out = captureLog(t, func() {
		RequestID(Forwarded("192.0.0.0/8")(next)).ServeHTTP(rec, req)
	})

	if got != TEST_PROXY_PEER {
		t.Fatalf("ClientIP = %q, want raw peer (fail-closed)", got)
	}
	rid := rec.Header().Get(REQUEST_ID_HEADER)
	for _, want := range []string{"FORWARDED rid=" + rid, "malformed remote addr"} {
		if !strings.Contains(out, want) {
			t.Errorf("log = %q, missing %q", out, want)
		}
	}
}

func TestForwarded_unparsableForwardedFor_loggedWithoutClientBytes(t *testing.T) {
	var got string
	next := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		got = ClientIP(r)
	})

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil) // RemoteAddr 192.0.2.1:1234
	req.Header.Set("X-Forwarded-For", "not-an-ip")

	var out string
	out = captureLog(t, func() {
		RequestID(Forwarded("192.0.0.0/8")(next)).ServeHTTP(httptest.NewRecorder(), req)
	})

	if got != TEST_PROXY_PEER {
		t.Fatalf("ClientIP = %q, want peer kept (fail-closed)", got)
	}
	if !strings.Contains(out, "rightmost X-Forwarded-For hop is not an IP") {
		t.Errorf("log = %q, want specific hop error", out)
	}
	if strings.Contains(out, "not-an-ip") {
		t.Errorf("log = %q, client-supplied bytes must not be echoed", out)
	}
}
