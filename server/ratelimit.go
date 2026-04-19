package server

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// RateLimiter is a per-key token-bucket rate limiter. Keys are typically
// IP addresses or player IDs. Idle buckets are evicted by a background janitor.
type RateLimiter struct {
	mu      sync.Mutex
	buckets map[string]*rlBucket
	rps     rate.Limit
	burst   int
	idleTTL time.Duration
}

type rlBucket struct {
	lim  *rate.Limiter
	seen time.Time
}

// NewRateLimiter creates a RateLimiter with the given tokens-per-second rate,
// burst size, and idle eviction TTL.
func NewRateLimiter(rps rate.Limit, burst int, idleTTL time.Duration) *RateLimiter {
	return &RateLimiter{
		buckets: make(map[string]*rlBucket),
		rps:     rps,
		burst:   burst,
		idleTTL: idleTTL,
	}
}

// Allow returns true if the key is within its rate limit, false otherwise.
func (rl *RateLimiter) Allow(key string) bool {
	rl.mu.Lock()
	b, ok := rl.buckets[key]
	if !ok {
		b = &rlBucket{lim: rate.NewLimiter(rl.rps, rl.burst)}
		rl.buckets[key] = b
	}
	b.seen = time.Now()
	allowed := b.lim.Allow()
	rl.mu.Unlock()
	return allowed
}

// cleanup removes buckets that have been idle longer than idleTTL.
// now is taken as a parameter so tests can drive it without sleeping.
func (rl *RateLimiter) cleanup(now time.Time) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	for key, b := range rl.buckets {
		if now.Sub(b.seen) > rl.idleTTL {
			delete(rl.buckets, key)
		}
	}
}

// RunJanitor starts a background goroutine that calls cleanup periodically
// (every idleTTL/2). It stops when done is closed.
func (rl *RateLimiter) RunJanitor(done <-chan struct{}) {
	interval := rl.idleTTL / 2
	if interval < time.Second {
		interval = time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-done:
			return
		case t := <-ticker.C:
			rl.cleanup(t)
		}
	}
}

// clientIP returns the best-effort remote IP for the request.
//
// If trustProxy is false (the default), the IP is taken directly from
// r.RemoteAddr. If trustProxy is true, the first value of the
// X-Forwarded-For header is used instead. Only set trustProxy=true when
// the server sits behind a trusted reverse proxy that (a) strips any
// client-supplied X-Forwarded-For header and (b) appends the real client IP
// itself — otherwise a malicious client can spoof the header and bypass
// rate limits.
func clientIP(r *http.Request, trustProxy bool) string {
	if trustProxy {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			// May be comma-separated; take only the first entry.
			if idx := strings.Index(xff, ","); idx != -1 {
				xff = xff[:idx]
			}
			if ip := strings.TrimSpace(xff); ip != "" {
				return ip
			}
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// sendRateLimited sends a rate-limit error message to the player.
func sendRateLimited(p *Player, action string) {
	p.sendJSON(ErrorMessage{
		Type:    "error",
		V:       ProtocolVersion,
		Message: "rate limited: " + action,
	})
}
