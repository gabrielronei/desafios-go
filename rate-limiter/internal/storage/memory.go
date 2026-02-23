package storage

import (
	"context"
	"sync"
	"time"
)

// MemoryStorage is an in-memory Storage implementation intended for testing.
// It is NOT safe to use in a multi-process environment.
type MemoryStorage struct {
	mu       sync.Mutex
	counters map[string]counterEntry
	blocked  map[string]time.Time

	// nowFn is injectable for deterministic testing.
	nowFn func() time.Time
}

type counterEntry struct {
	count     int64
	resetTime time.Time
}

// NewMemoryStorage returns an empty MemoryStorage.
func NewMemoryStorage() *MemoryStorage {
	return &MemoryStorage{
		counters: make(map[string]counterEntry),
		blocked:  make(map[string]time.Time),
		nowFn:    time.Now,
	}
}

// WithNow overrides the clock used by this storage (useful in tests).
func (m *MemoryStorage) WithNow(fn func() time.Time) *MemoryStorage {
	m.nowFn = fn
	return m
}

func (m *MemoryStorage) Increment(_ context.Context, key string, window time.Duration) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := m.nowFn()
	entry, exists := m.counters[key]

	if !exists || now.After(entry.resetTime) {
		// Start a fresh window.
		m.counters[key] = counterEntry{count: 1, resetTime: now.Add(window)}
		return 1, nil
	}

	entry.count++
	m.counters[key] = entry
	return entry.count, nil
}

func (m *MemoryStorage) IsBlocked(_ context.Context, key string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	expiry, exists := m.blocked[key]
	if !exists {
		return false, nil
	}
	if m.nowFn().After(expiry) {
		delete(m.blocked, key)
		return false, nil
	}
	return true, nil
}

func (m *MemoryStorage) Block(_ context.Context, key string, duration time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.blocked[key] = m.nowFn().Add(duration)
	return nil
}

func (m *MemoryStorage) Reset(_ context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.counters, key)
	delete(m.blocked, key)
	return nil
}
