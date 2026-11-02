package main

import (
	"context"
	"log/slog"
	"time"
)

const (
	chatRetention = 30 * 24 * time.Hour // docs/spec.md, Keamanan 5
	purgeInterval = 24 * time.Hour
)

// chatPurger deletes chat messages older than a cutoff.
type chatPurger interface {
	PurgeChat(ctx context.Context, cutoff time.Time) (int64, error)
}

// runChatRetention purges once at start and then every interval until ctx
// ends. Failures are logged and retried on the next run.
func runChatRetention(ctx context.Context, p chatPurger, now func() time.Time, every time.Duration, log *slog.Logger) {
	purge := func() {
		pctx, cancel := context.WithTimeout(ctx, time.Minute)
		defer cancel()
		n, err := p.PurgeChat(pctx, now().Add(-chatRetention))
		if err != nil {
			log.Error("chat purge failed", slog.Any("err", err))
			return
		}
		log.Info("chat purged", slog.Int64("deleted", n))
	}
	purge()
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			purge()
		}
	}
}
