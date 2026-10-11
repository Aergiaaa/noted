package middleware

import (
	"math"
	"net/http"
	"strconv"
	"sync"
	"time"

	"noted/cmd/server/handler"
)

// MAX_BUCKETS caps limiter state: once the map is full the stalest entry
// is evicted, so a flood of forged client keys cannot grow memory without
// bound.
const MAX_BUCKETS = 4096

func RateLimit(b *TokenBucket) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rateLimit(b, w, r, next)
		})
	}
}

func rateLimit(b *TokenBucket, w http.ResponseWriter, r *http.Request, next http.Handler) {
	if wait, ok := b.Allow(ClientIP(r)); !ok {
		w.Header().Set("Retry-After", retryAfter(wait))
		handler.WriteError(w, http.StatusTooManyRequests, "", nil)
		return
	}
	next.ServeHTTP(w, r)
}

// retryAfter rounds up (Retry-After is integer seconds) and floors at 1 —
// "0" would invite an immediate retry storm.
func retryAfter(wait time.Duration) string {
	secs := int(math.Ceil(wait.Seconds()))
	if secs < 1 {
		secs = 1
	}
	return strconv.Itoa(secs)
}

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

func NewTokenBucket(limit int, window time.Duration, now func() time.Time) *TokenBucket {
	return &TokenBucket{
		limit:   limit,
		window:  window,
		now:     now,
		buckets: make(map[string]*bucket),
	}
}

// Allow consumes one token for key; on exhaustion it returns how long to
// wait until one token has refilled.
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
