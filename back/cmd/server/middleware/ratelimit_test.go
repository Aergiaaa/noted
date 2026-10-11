package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// withClientIP injects the key Forwarded would have stored, so limiter
// tests do not run the whole chain.
func withClientIP(r *http.Request, ip string) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), CLIENT_IP_KEY, ip))
}

func TestTokenBucket_allowsLimitThenRefuses(t *testing.T) {
	now := time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC)
	b := NewTokenBucket(5, time.Minute, func() time.Time { return now })

	for i := 0; i < 5; i++ {
		if wait, ok := b.Allow("ip-1"); !ok {
			t.Fatalf("attempt %d refused (wait=%v), want allowed", i+1, wait)
		}
	}
	wait, ok := b.Allow("ip-1")
	if ok {
		t.Fatal("6th attempt inside the window allowed, want 429 path")
	}
	if wait <= 0 || wait > time.Minute {
		t.Fatalf("wait = %v, want in (0, 1m]", wait)
	}

	// One token refills after window/limit = 12s; 13s clears it.
	now = now.Add(13 * time.Second)
	if _, ok := b.Allow("ip-1"); !ok {
		t.Fatal("attempt after refill refused, want allowed")
	}
}

func TestTokenBucket_isolatesKeys(t *testing.T) {
	now := time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC)
	b := NewTokenBucket(1, time.Minute, func() time.Time { return now })

	if _, ok := b.Allow("a"); !ok {
		t.Fatal("first key refused")
	}
	if _, ok := b.Allow("a"); ok {
		t.Fatal("exhausted key allowed")
	}
	if _, ok := b.Allow("b"); !ok {
		t.Fatal("fresh key refused — keys not isolated")
	}
}

func TestTokenBucket_fullCapacityOnFirstUsePerKey(t *testing.T) {
	// The map starts empty: a never-before-seen key must be credited a
	// full allowance at first sight, not at bucket construction time.
	now := time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC)
	b := NewTokenBucket(2, time.Minute, func() time.Time { return now })
	now = now.Add(time.Hour) // idle time before any key appears
	if _, ok := b.Allow("late"); !ok {
		t.Fatal("late first use refused")
	}
	if _, ok := b.Allow("late"); !ok {
		t.Fatal("second use of late key refused")
	}
	if _, ok := b.Allow("late"); ok {
		t.Fatal("third use allowed, want limit 2")
	}
}

func TestTokenBucket_evictsStalestAtCapacity(t *testing.T) {
	now := time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC)
	b := NewTokenBucket(5, time.Minute, func() time.Time { return now })

	for i := 0; i < MAX_BUCKETS; i++ {
		b.Allow(string(rune('a'+i%26)) + "-" + time.Duration(i).String())
	}
	if len(b.buckets) != MAX_BUCKETS {
		t.Fatalf("buckets = %d, want %d", len(b.buckets), MAX_BUCKETS)
	}
	// One more distinct key: capacity holds, the oldest key is gone.
	b.Allow("brand-new-key")
	if len(b.buckets) != MAX_BUCKETS {
		t.Fatalf("buckets after overflow = %d, want %d", len(b.buckets), MAX_BUCKETS)
	}
	if _, ok := b.buckets["brand-new-key"]; !ok {
		t.Fatal("new key missing after eviction")
	}
}

func TestRateLimit_admitsThen429WithRetryAfter(t *testing.T) {
	now := time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC)
	b := NewTokenBucket(2, time.Minute, func() time.Time { return now })

	reached := 0
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { reached++ })
	run := func() *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		req := withClientIP(httptest.NewRequest(http.MethodPost, "/api/auth/login", nil), "9.9.9.9")
		RateLimit(b)(next).ServeHTTP(rec, req)
		return rec
	}

	for i := 0; i < 2; i++ {
		if rec := run(); rec.Code != http.StatusOK {
			t.Fatalf("attempt %d status = %d, want 200 passthrough", i+1, rec.Code)
		}
	}
	rec := run()
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("3rd attempt status = %d, want 429", rec.Code)
	}
	retry := rec.Header().Get("Retry-After")
	if retry == "" || retry == "0" {
		t.Fatalf("Retry-After = %q, want positive integer seconds", retry)
	}
	if reached != 2 {
		t.Fatalf("handler reached %d times, want 2", reached)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", ct)
	}
}

func TestRetryAfter_roundsUpAndNeverZero(t *testing.T) {
	for _, tc := range []struct {
		wait time.Duration
		want string
	}{
		{0, "1"},
		{100 * time.Millisecond, "1"},
		{1500 * time.Millisecond, "2"},
		{12 * time.Second, "12"},
	} {
		if got := retryAfter(tc.wait); got != tc.want {
			t.Fatalf("retryAfter(%v) = %q, want %q", tc.wait, got, tc.want)
		}
	}
}

func TestRateLimit_differentIPs_getIndependentBudgets(t *testing.T) {
	b := NewTokenBucket(1, time.Minute, time.Now)
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	run := func(ip string) int {
		rec := httptest.NewRecorder()
		req := withClientIP(httptest.NewRequest(http.MethodPost, "/api/auth/login", nil), ip)
		RateLimit(b)(next).ServeHTTP(rec, req)
		return rec.Code
	}
	if run("1.1.1.1") != http.StatusOK {
		t.Fatal("first ip refused")
	}
	if run("1.1.1.1") != http.StatusTooManyRequests {
		t.Fatal("exhausted ip not limited")
	}
	if run("2.2.2.2") != http.StatusOK {
		t.Fatal("other ip shares the budget")
	}
}

func TestRateLimit_missingClientIP_usesSharedKey(t *testing.T) {
	// Forwarded not in the chain → ClientIP "" → every request shares one
	// bucket (fail-closed for rate limiting, not fail-open).
	b := NewTokenBucket(1, time.Minute, time.Now)
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	rec := httptest.NewRecorder()
	RateLimit(b)(next).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/auth/login", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("first empty-key request = %d", rec.Code)
	}
	rec = httptest.NewRecorder()
	RateLimit(b)(next).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/auth/login", nil))
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("second empty-key request = %d, want 429", rec.Code)
	}
}
