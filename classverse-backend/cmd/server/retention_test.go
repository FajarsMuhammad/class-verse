package main

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"
)

type fakePurger struct {
	cutoffs chan time.Time
	fail    bool
}

func (f fakePurger) PurgeChat(_ context.Context, cutoff time.Time) (int64, error) {
	f.cutoffs <- cutoff
	if f.fail {
		return 0, errors.New("db down")
	}
	return 3, nil
}

func TestRunChatRetention(t *testing.T) {
	for _, tt := range []struct {
		name string
		fail bool
	}{{"ok", false}, {"failure keeps running", true}} {
		t.Run(tt.name, func(t *testing.T) {
			now := time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)
			p := fakePurger{cutoffs: make(chan time.Time, 4), fail: tt.fail}
			ctx, cancel := context.WithCancel(t.Context())
			done := make(chan struct{})
			go func() {
				runChatRetention(ctx, p, func() time.Time { return now }, 10*time.Millisecond, slog.New(slog.NewTextHandler(io.Discard, nil)))
				close(done)
			}()
			for i := range 2 { // once at start, then on the ticker
				select {
				case c := <-p.cutoffs:
					if want := now.Add(-30 * 24 * time.Hour); !c.Equal(want) {
						t.Fatalf("run %d cutoff = %v, want %v", i, c, want)
					}
				case <-time.After(2 * time.Second):
					t.Fatalf("run %d did not happen", i)
				}
			}
			cancel()
			select {
			case <-done:
			case <-time.After(2 * time.Second):
				t.Fatal("did not stop on cancel")
			}
		})
	}
}
