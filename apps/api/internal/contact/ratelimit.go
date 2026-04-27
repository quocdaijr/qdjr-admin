package contact

import (
	"context"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// Per-IP rate-limit budget. 5 requests / hour with burst 5 means a fresh IP
// can submit 5 in quick succession but then must wait ~12m for the next slot.
const (
	limiterRate     = rate.Limit(float64(5) / float64(time.Hour/time.Second)) // 5/hr in 1/sec terms
	limiterBurst    = 5
	limiterIdleTTL  = time.Hour       // entry untouched this long → evictable
	limiterEvictGap = 10 * time.Minute // sweep interval
)

// limiterEntry pairs a token-bucket with its last-seen timestamp so the
// background sweeper can drop idle IPs without holding the global lock.
type limiterEntry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// Limiter is an in-memory per-IP rate limiter.
//
// It is safe for concurrent use. Callers must invoke Allow per request and
// (optionally) call StartEvictor in a goroutine to keep memory bounded.
type Limiter struct {
	mu      sync.Mutex
	entries map[string]*limiterEntry
	now     func() time.Time // injectable clock for tests
}

// NewLimiter returns a Limiter with default 5/hour budget per IP.
func NewLimiter() *Limiter {
	return &Limiter{
		entries: make(map[string]*limiterEntry),
		now:     time.Now,
	}
}

// Allow consumes one token for ip and reports whether the request may proceed.
//
// Empty ip strings still get a slot (tracked under the empty key) — callers
// that want to reject empty IPs should do so before calling Allow.
func (l *Limiter) Allow(ip string) bool {
	l.mu.Lock()
	e, ok := l.entries[ip]
	if !ok {
		e = &limiterEntry{
			limiter: rate.NewLimiter(limiterRate, limiterBurst),
		}
		l.entries[ip] = e
	}
	e.lastSeen = l.now()
	l.mu.Unlock()
	return e.limiter.Allow()
}

// StartEvictor runs a background sweep that drops idle IPs every
// limiterEvictGap. It returns when ctx is cancelled.
func (l *Limiter) StartEvictor(ctx context.Context) {
	t := time.NewTicker(limiterEvictGap)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			l.evict()
		}
	}
}

// evict drops entries whose lastSeen is older than limiterIdleTTL.
func (l *Limiter) evict() {
	cutoff := l.now().Add(-limiterIdleTTL)
	l.mu.Lock()
	defer l.mu.Unlock()
	for ip, e := range l.entries {
		if e.lastSeen.Before(cutoff) {
			delete(l.entries, ip)
		}
	}
}
