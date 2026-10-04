package middleware

import (
	"math"
	"net/http"
	"strconv"
	"sync"
	"time"

	"noted/cmd/server/handler"
)

// MAX_BUCKETS caps per-client limiter state: once the map holds this many
// keys, the stalest entry is evicted, so a flood of forged client keys
// cannot grow memory without bound.
const MAX_BUCKETS = 4096

// RateLimit admits requests while the shared TokenBucket has a token for
// the client key (ClientIP from Forwarded); the exhausted request gets 429
// with Retry-After and never reaches the handler (SECURITY.md login
// limiting — the login route wraps it, every attempt counts).
func RateLimit(b *TokenBucket) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rateLimit(b, w, r, next)
		})
	}
}

// rateLimit is RateLimit's handler body, split for direct test driving.
func rateLimit(b *TokenBucket, w http.ResponseWriter, r *http.Request, next http.Handler) {
	if wait, ok := b.Allow(ClientIP(r)); !ok {
		w.Header().Set("Retry-After", retryAfter(wait))
		handler.WriteError(w, http.StatusTooManyRequests, "", nil)
		return
	}
	next.ServeHTTP(w, r)
}

// retryAfter renders the backoff as whole seconds, rounded up and never
// below 1 — HTTP Retry-After is integer seconds and "0" would invite an
// immediate retry storm.
func retryAfter(wait time.Duration) string {
	secs := int(math.Ceil(wait.Seconds()))
	if secs < 1 {
		secs = 1
	}
	return strconv.Itoa(secs)
}

// TokenBucket is a lazily-refilled per-key limiter: each key starts full
// (limit tokens), tokens refill continuously at limit/window, and one
// token is consumed per allowed request. now is injectable so tests pin
// time instead of sleeping.
type TokenBucket struct {
	mu      sync.Mutex
	limit   int
	window  time.Duration
	now     func() time.Time
	buckets map[string]*bucket
}

type bucket struct {
	tokens float64
	last   time.Time
}

// NewTokenBucket builds a limiter granting `limit` attempts per `window`
// per key (SECURITY.md: 5 per minute on login).
func NewTokenBucket(limit int, window time.Duration, now func() time.Time) *TokenBucket {
	return &TokenBucket{
		limit:   limit,
		window:  window,
		now:     now,
		buckets: make(map[string]*bucket),
	}
}

// Allow consumes one token for key, reporting success; on exhaustion it
// returns how long the caller should back off until one token has refilled.
func (b *TokenBucket) Allow(key string) (time.Duration, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()

	now := b.now()
	rate := float64(b.limit) / b.window.Seconds() // tokens per second
	st, ok := b.buckets[key]
	if !ok {
		b.evictStalest()
		st = &bucket{tokens: float64(b.limit), last: now}
		b.buckets[key] = st
	}
	st.tokens = math.Min(float64(b.limit), st.tokens+now.Sub(st.last).Seconds()*rate)
	st.last = now

	if st.tokens >= 1 {
		st.tokens--
		return 0, true
	}
	wait := time.Duration((1 - st.tokens) / rate * float64(time.Second))
	return wait, false
}

// evictStalest drops the least-recently-seen key once at capacity; it is
// called only for keys that are not yet present.
func (b *TokenBucket) evictStalest() {
	if len(b.buckets) < MAX_BUCKETS {
		return
	}
	var oldestKey string
	var oldest time.Time
	for k, st := range b.buckets {
		if oldestKey == "" || st.last.Before(oldest) {
			oldestKey, oldest = k, st.last
		}
	}
	delete(b.buckets, oldestKey)
}
