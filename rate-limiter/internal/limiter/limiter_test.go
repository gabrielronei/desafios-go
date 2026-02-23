package limiter_test

import (
	"context"
	"testing"
	"time"

	"github.com/gaboliveirap/rate-limiter/internal/limiter"
	"github.com/gaboliveirap/rate-limiter/internal/storage"
)

const (
	testIP    = "192.168.1.1"
	testToken = "abc123"
)

func newLimiter(ipMax, tokenMax int) (*limiter.RateLimiter, *storage.MemoryStorage) {
	store := storage.NewMemoryStorage()
	l := limiter.New(
		store,
		limiter.LimitConfig{MaxRequests: ipMax, BlockDuration: 5 * time.Minute},
		limiter.LimitConfig{MaxRequests: tokenMax, BlockDuration: 5 * time.Minute},
		nil,
	)
	return l, store
}

// ---------------------------------------------------------------------------
// IP-based tests
// ---------------------------------------------------------------------------

func TestAllowByIP_UnderLimit(t *testing.T) {
	l, _ := newLimiter(5, 100)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		ok, err := l.Allow(ctx, testIP, "")
		if err != nil {
			t.Fatalf("unexpected error on request %d: %v", i+1, err)
		}
		if !ok {
			t.Fatalf("request %d should be allowed but was denied", i+1)
		}
	}
}

func TestAllowByIP_OverLimit(t *testing.T) {
	l, _ := newLimiter(5, 100)
	ctx := context.Background()

	// Consume all allowed requests.
	for i := 0; i < 5; i++ {
		if _, err := l.Allow(ctx, testIP, ""); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}

	// The 6th request must be denied.
	ok, err := l.Allow(ctx, testIP, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Fatal("6th request should be denied but was allowed")
	}
}

func TestAllowByIP_StaysBlockedAfterLimit(t *testing.T) {
	l, _ := newLimiter(3, 100)
	ctx := context.Background()

	// Exhaust the limit.
	for i := 0; i < 3; i++ {
		l.Allow(ctx, testIP, "") //nolint:errcheck
	}

	// Next N requests must all be blocked.
	for i := 0; i < 5; i++ {
		ok, err := l.Allow(ctx, testIP, "")
		if err != nil {
			t.Fatalf("unexpected error on post-block request %d: %v", i+1, err)
		}
		if ok {
			t.Fatalf("post-block request %d should be denied", i+1)
		}
	}
}

// ---------------------------------------------------------------------------
// Token-based tests
// ---------------------------------------------------------------------------

func TestAllowByToken_UnderLimit(t *testing.T) {
	l, _ := newLimiter(5, 10)
	ctx := context.Background()

	for i := 0; i < 10; i++ {
		ok, err := l.Allow(ctx, testIP, testToken)
		if err != nil {
			t.Fatalf("unexpected error on request %d: %v", i+1, err)
		}
		if !ok {
			t.Fatalf("request %d should be allowed but was denied", i+1)
		}
	}
}

func TestAllowByToken_OverLimit(t *testing.T) {
	l, _ := newLimiter(5, 10)
	ctx := context.Background()

	for i := 0; i < 10; i++ {
		l.Allow(ctx, testIP, testToken) //nolint:errcheck
	}

	ok, err := l.Allow(ctx, testIP, testToken)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Fatal("11th token request should be denied")
	}
}

// ---------------------------------------------------------------------------
// Token overrides IP tests
// ---------------------------------------------------------------------------

func TestTokenOverridesIPLimit(t *testing.T) {
	// IP limit = 3, token limit = 10.
	l, _ := newLimiter(3, 10)
	ctx := context.Background()

	// 10 requests with the same IP but a token should all succeed.
	for i := 0; i < 10; i++ {
		ok, err := l.Allow(ctx, testIP, testToken)
		if err != nil {
			t.Fatalf("unexpected error on request %d: %v", i+1, err)
		}
		if !ok {
			t.Fatalf("request %d should be allowed (token limit=10) but was denied", i+1)
		}
	}
}

func TestIPLimitDoesNotAffectToken(t *testing.T) {
	// IP limit = 3, token limit = 100.
	l, _ := newLimiter(3, 100)
	ctx := context.Background()

	// Exhaust the IP limit.
	for i := 0; i < 3; i++ {
		l.Allow(ctx, testIP, "") //nolint:errcheck
	}

	// Token requests from the same IP should still be allowed (different key).
	ok, err := l.Allow(ctx, testIP, testToken)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("token request should be allowed even though IP is blocked")
	}
}

// ---------------------------------------------------------------------------
// Per-token configuration tests
// ---------------------------------------------------------------------------

func TestPerTokenConfig(t *testing.T) {
	store := storage.NewMemoryStorage()
	perToken := map[string]limiter.LimitConfig{
		"vip-token": {MaxRequests: 20, BlockDuration: time.Minute},
	}
	l := limiter.New(
		store,
		limiter.LimitConfig{MaxRequests: 5, BlockDuration: 5 * time.Minute},
		limiter.LimitConfig{MaxRequests: 10, BlockDuration: 5 * time.Minute},
		perToken,
	)
	ctx := context.Background()

	// vip-token should allow 20 requests.
	for i := 0; i < 20; i++ {
		ok, err := l.Allow(ctx, testIP, "vip-token")
		if err != nil {
			t.Fatalf("unexpected error on request %d: %v", i+1, err)
		}
		if !ok {
			t.Fatalf("vip request %d should be allowed but was denied", i+1)
		}
	}

	// 21st request must be denied.
	ok, _ := l.Allow(ctx, testIP, "vip-token")
	if ok {
		t.Fatal("21st vip request should be denied")
	}
}

// ---------------------------------------------------------------------------
// Window reset test (uses injectable clock)
// ---------------------------------------------------------------------------

func TestCounterResetsAfterWindow(t *testing.T) {
	store := storage.NewMemoryStorage()

	// Start at t=0.
	fakeNow := time.Unix(1_000_000, 0)
	store.WithNow(func() time.Time { return fakeNow })

	l := limiter.New(
		store,
		limiter.LimitConfig{MaxRequests: 3, BlockDuration: 5 * time.Minute},
		limiter.LimitConfig{MaxRequests: 100, BlockDuration: 5 * time.Minute},
		nil,
	)
	ctx := context.Background()

	// Exhaust the limit.
	for i := 0; i < 3; i++ {
		l.Allow(ctx, testIP, "") //nolint:errcheck
	}

	// Advance clock by 2 seconds (new window).
	fakeNow = fakeNow.Add(2 * time.Second)

	// Counter should have reset; request must be allowed.
	ok, err := l.Allow(ctx, testIP, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("request after window reset should be allowed")
	}
}

// ---------------------------------------------------------------------------
// Block expiry test (uses injectable clock)
// ---------------------------------------------------------------------------

func TestBlockExpiresAfterDuration(t *testing.T) {
	store := storage.NewMemoryStorage()

	fakeNow := time.Unix(1_000_000, 0)
	store.WithNow(func() time.Time { return fakeNow })

	blockDuration := 5 * time.Second
	l := limiter.New(
		store,
		limiter.LimitConfig{MaxRequests: 2, BlockDuration: blockDuration},
		limiter.LimitConfig{MaxRequests: 100, BlockDuration: 5 * time.Minute},
		nil,
	)
	ctx := context.Background()

	// Exhaust the limit and trigger a block.
	for i := 0; i < 3; i++ {
		l.Allow(ctx, testIP, "") //nolint:errcheck
	}

	// Still within block window → denied.
	ok, _ := l.Allow(ctx, testIP, "")
	if ok {
		t.Fatal("request during block should be denied")
	}

	// Advance past block duration.
	fakeNow = fakeNow.Add(blockDuration + time.Second)

	// Also advance the counter window so it resets.
	fakeNow = fakeNow.Add(time.Second)

	ok, err := l.Allow(ctx, testIP, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("request after block expiry should be allowed")
	}
}
