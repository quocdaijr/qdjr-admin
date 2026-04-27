package contact

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestLimiter_AllowsBurstThenRejects(t *testing.T) {
	l := NewLimiter()
	ip := "1.2.3.4"

	for i := 0; i < limiterBurst; i++ {
		assert.Truef(t, l.Allow(ip), "request %d within burst should be allowed", i+1)
	}
	// 6th request within the same instant must be rejected.
	assert.False(t, l.Allow(ip), "burst+1 request should be rate-limited")
}

func TestLimiter_PerIPIndependent(t *testing.T) {
	l := NewLimiter()

	// Exhaust IP A.
	for i := 0; i < limiterBurst; i++ {
		require_(t, l.Allow("10.0.0.1"))
	}
	assert.False(t, l.Allow("10.0.0.1"), "A should be limited")

	// IP B must be unaffected.
	for i := 0; i < limiterBurst; i++ {
		assert.Truef(t, l.Allow("10.0.0.2"), "B request %d should pass", i+1)
	}
	assert.False(t, l.Allow("10.0.0.2"), "B exhausted independently")
}

func TestLimiter_Evict(t *testing.T) {
	l := NewLimiter()

	// Pin a fake clock.
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	l.now = func() time.Time { return base }

	require_(t, l.Allow("a"))
	require_(t, l.Allow("b"))

	// Advance past the idle TTL.
	l.now = func() time.Time { return base.Add(2 * limiterIdleTTL) }
	l.evict()

	l.mu.Lock()
	defer l.mu.Unlock()
	assert.Empty(t, l.entries, "all idle entries should be swept")
}

// tiny helper: gin/testify's require uses *T's FailNow; we want a plain
// assertion that boolean is true without importing require here.
func require_(t *testing.T, ok bool) {
	t.Helper()
	if !ok {
		t.Fatalf("expected true")
	}
}
