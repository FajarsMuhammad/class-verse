package ratelimit

import (
	"testing"
	"time"
)

func TestLimiter(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	l := New(2, time.Minute, func() time.Time { return now })

	steps := []struct {
		name      string
		advance   time.Duration
		key       string
		wantOK    bool
		wantRetry time.Duration
	}{
		{"first", 0, "a", true, 0},
		{"second", 10 * time.Second, "a", true, 0},
		{"third blocked", 10 * time.Second, "a", false, 40 * time.Second},
		{"other key independent", 0, "b", true, 0},
		{"new window", 40 * time.Second, "a", true, 0},
	}
	for _, s := range steps {
		now = now.Add(s.advance)
		ok, retry := l.Allow(s.key)
		if ok != s.wantOK || retry != s.wantRetry {
			t.Errorf("%s: Allow() = %v, %v; want %v, %v", s.name, ok, retry, s.wantOK, s.wantRetry)
		}
	}
}
