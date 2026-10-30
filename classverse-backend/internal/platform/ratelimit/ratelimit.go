// Package ratelimit is an in-memory fixed-window limiter keyed by string.
// One server instance is enough for the pilot (ADR-002), so no shared store.
package ratelimit

import (
	"sync"
	"time"
)

type window struct {
	start time.Time
	count int
}

// Limiter allows at most limit events per key in each window.
type Limiter struct {
	limit  int
	period time.Duration
	now    func() time.Time

	mu        sync.Mutex
	windows   map[string]window
	lastPrune time.Time
}

// New returns a limiter. now is usually time.Now; tests inject a fake clock.
func New(limit int, period time.Duration, now func() time.Time) *Limiter {
	return &Limiter{limit: limit, period: period, now: now, windows: make(map[string]window)}
}

// Allow records one event for key. When the limit is exceeded it returns
// false and how long until the current window ends.
func (l *Limiter) Allow(key string) (ok bool, retryAfter time.Duration) {
	now := l.now()

	l.mu.Lock()
	defer l.mu.Unlock()

	l.prune(now)
	w := l.windows[key]
	if now.Sub(w.start) >= l.period {
		w = window{start: now}
	}
	if w.count >= l.limit {
		return false, w.start.Add(l.period).Sub(now)
	}
	w.count++
	l.windows[key] = w
	return true, 0
}

// prune drops expired windows at most once per period so the map does not
// grow without bound.
func (l *Limiter) prune(now time.Time) {
	if now.Sub(l.lastPrune) < l.period {
		return
	}
	l.lastPrune = now
	for k, w := range l.windows {
		if now.Sub(w.start) >= l.period {
			delete(l.windows, k)
		}
	}
}
