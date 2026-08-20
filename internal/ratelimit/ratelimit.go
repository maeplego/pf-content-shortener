package ratelimit

import (
	"sync"
	"time"
)

// Limiter is an in-process per-key limiter for the redirect hot path.
type Limiter struct {
	mu      sync.Mutex
	limit   int
	window  time.Duration
	entries map[string]*bucket
}

type bucket struct {
	count int
	reset time.Time
}

func New(limit int, window time.Duration) *Limiter {
	if limit <= 0 {
		limit = 60
	}
	if window <= 0 {
		window = time.Minute
	}
	return &Limiter{limit: limit, window: window, entries: map[string]*bucket{}}
}

func (l *Limiter) Allow(key string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	b, ok := l.entries[key]
	if !ok || now.After(b.reset) {
		l.entries[key] = &bucket{count: 1, reset: now.Add(l.window)}
		return true
	}
	if b.count >= l.limit {
		return false
	}
	b.count++
	return true
}
