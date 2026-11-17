package file

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/platform/db/query"
)

// CleanupExpired reclaims at most 100 expired sessions. Live uploads keep their chunks.
func (u *Uploader) CleanupExpired(ctx context.Context) error {
	rows, err := query.New(u.meta.store.pool).ListExpiredUploads(ctx, pgtype.Timestamptz{Time: u.meta.now().UTC(), Valid: true})
	if err != nil {
		return err
	}
	var failures []error
	for _, f := range rows {
		err := u.withLock(ctx, f.ID, func(q *query.Queries) error {
			current, err := q.LockFile(ctx, query.LockFileParams{OrgID: f.OrgID, ID: f.ID})
			if err != nil {
				return err
			}
			if current.Status != "pending" && current.Status != "deleting" {
				return nil
			}
			if err := u.meta.MarkDeleting(ctx, f.OrgID, f.ID); err != nil {
				return err
			}
			if err := u.storage.AbortMultipart(ctx, f.ObjectKey, f.MultipartID); err != nil {
				return err
			}
			if err := u.storage.Delete(ctx, f.ObjectKey); err != nil {
				return err
			}
			if err := u.meta.ConfirmDeleted(ctx, f.OrgID, f.ID); err != nil {
				return err
			}
			return q.MarkUploadCanceled(ctx, query.MarkUploadCanceledParams{OrgID: f.OrgID, FileID: f.ID})
		})
		if err != nil && !errors.Is(err, ErrState) {
			failures = append(failures, err)
		}
	}
	return errors.Join(failures...)
}

// RunCleanup retries persisted expired sessions at startup and once per minute.
func (u *Uploader) RunCleanup(ctx context.Context, log *slog.Logger) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		timeout, cancel := context.WithTimeout(ctx, 45*time.Second)
		err := u.CleanupExpired(timeout)
		cancel()
		if err != nil && ctx.Err() == nil {
			log.Warn("expired upload cleanup failed")
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
