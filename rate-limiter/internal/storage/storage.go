// Package storage defines the Strategy interface for rate limiter persistence.
// Swap implementations (Redis, in-memory, etc.) by providing a different Storage.
package storage

import (
	"context"
	"time"
)

// Storage is the persistence strategy used by the rate limiter.
type Storage interface {
	// Increment atomically increments the request counter for key within the
	// given time window and returns the new count.
	Increment(ctx context.Context, key string, window time.Duration) (int64, error)

	// IsBlocked reports whether key is currently blocked.
	IsBlocked(ctx context.Context, key string) (bool, error)

	// Block marks key as blocked for the specified duration.
	Block(ctx context.Context, key string, duration time.Duration) error

	// Reset clears the request counter for key (used in tests / admin).
	Reset(ctx context.Context, key string) error
}
