// Package limiter provides the core rate limiting logic, decoupled from HTTP concerns.
package limiter

import (
	"context"
	"time"

	"github.com/gaboliveirap/rate-limiter/internal/storage"
)

// LimitConfig defines the rate limit policy for a single subject (IP or token).
type LimitConfig struct {
	MaxRequests   int
	BlockDuration time.Duration
}

// RateLimiter evaluates whether a request should be allowed or denied.
type RateLimiter struct {
	storage      storage.Storage
	ipConfig     LimitConfig
	tokenDefault LimitConfig
	tokenCfgs    map[string]LimitConfig
}

// New creates a RateLimiter.
//   - store          — persistence backend (Redis, memory, …)
//   - ipConfig       — default policy applied when no token is present
//   - tokenDefault   — default policy applied to any token not listed in tokenCfgs
//   - tokenCfgs      — per-token policy overrides (may be nil)
func New(
	store storage.Storage,
	ipConfig LimitConfig,
	tokenDefault LimitConfig,
	tokenCfgs map[string]LimitConfig,
) *RateLimiter {
	if tokenCfgs == nil {
		tokenCfgs = make(map[string]LimitConfig)
	}
	return &RateLimiter{
		storage:      store,
		ipConfig:     ipConfig,
		tokenDefault: tokenDefault,
		tokenCfgs:    tokenCfgs,
	}
}

// Allow decides whether the request identified by (ip, token) is allowed.
// When a non-empty token is supplied it takes precedence over the IP.
// Returns (true, nil) when the request should be allowed.
func (rl *RateLimiter) Allow(ctx context.Context, ip, token string) (bool, error) {
	if token != "" {
		return rl.allowKey(ctx, "token:"+token, rl.tokenConfig(token))
	}
	return rl.allowKey(ctx, "ip:"+ip, rl.ipConfig)
}

// tokenConfig returns the effective config for a token.
func (rl *RateLimiter) tokenConfig(token string) LimitConfig {
	if cfg, ok := rl.tokenCfgs[token]; ok {
		return cfg
	}
	return rl.tokenDefault
}

// allowKey is the shared implementation for both IP and token limiting.
func (rl *RateLimiter) allowKey(ctx context.Context, key string, cfg LimitConfig) (bool, error) {
	// 1. Check if the key is already blocked (previous limit breach).
	blocked, err := rl.storage.IsBlocked(ctx, key)
	if err != nil {
		return false, err
	}
	if blocked {
		return false, nil
	}

	// 2. Increment the fixed-window counter.
	count, err := rl.storage.Increment(ctx, key, time.Second)
	if err != nil {
		return false, err
	}

	// 3. Deny and block when the counter exceeds the limit.
	if count > int64(cfg.MaxRequests) {
		if err := rl.storage.Block(ctx, key, cfg.BlockDuration); err != nil {
			return false, err
		}
		return false, nil
	}

	return true, nil
}
