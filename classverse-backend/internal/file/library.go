package file

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/org"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/platform/db/query"
)

// maxPosition matches the reader_positions check constraint.
const maxPosition = 1_000_000

// List returns ready files with the caller's last position. Staff only: it is the Kelola file manager.
func (s *Service) List(ctx context.Context, userID, orgID uuid.UUID) ([]query.ListReadyFilesRow, error) {
	var rows []query.ListReadyFilesRow
	err := s.store.inTx(ctx, func(q *query.Queries) error {
		if err := member(ctx, q, userID, orgID, true); err != nil {
			return err
		}
		var err error
		rows, err = q.ListReadyFiles(ctx, query.ListReadyFilesParams{OrgID: orgID, UserID: userID})
		return err
	})
	return rows, err
}

// readable rechecks access on every request, including every range request.
// Staff read any ready file; participants only files of published library books.
func (s *Service) readable(ctx context.Context, userID, orgID, id uuid.UUID) (query.File, error) {
	var f query.File
	err := s.store.inTx(ctx, func(q *query.Queries) error {
		staff, err := isStaff(ctx, q, userID, orgID)
		if err != nil {
			return err
		}
		f, err = q.GetReadyFile(ctx, query.GetReadyFileParams{OrgID: orgID, ID: id})
		if err != nil {
			return fileError(err)
		}
		if staff {
			return nil
		}
		published, err := q.FilePublished(ctx, query.FilePublishedParams{OrgID: orgID, FileID: uuid.NullUUID{UUID: id, Valid: true}})
		if err != nil {
			return fmt.Errorf("check published file: %w", err)
		}
		if !published {
			published, err = q.SnapshotFileReadable(ctx, query.SnapshotFileReadableParams{OrgID: orgID, UserID: userID, FileID: uuid.NullUUID{UUID: id, Valid: true}})
			if err != nil {
				return fmt.Errorf("check revision file: %w", err)
			}
		}
		if !published {
			published, err = q.AssignmentFileReadable(ctx, query.AssignmentFileReadableParams{OrgID: orgID, UserID: userID, FileID: uuid.NullUUID{UUID: id, Valid: true}})
			if err != nil {
				return fmt.Errorf("check assignment file: %w", err)
			}
		}
		if !published {
			// Same answer as a missing file: drafts must not be discoverable by ID.
			return ErrNotFound
		}
		return nil
	})
	return f, err
}

// SavePosition stores a PDF page or audio second for the caller.
func (s *Service) SavePosition(ctx context.Context, userID, orgID, id uuid.UUID, position int32) error {
	if position < 0 || position > maxPosition {
		return ErrInvalidUpload
	}
	if _, err := s.readable(ctx, userID, orgID, id); err != nil {
		return err
	}
	err := s.store.inTx(ctx, func(q *query.Queries) error {
		return q.SaveReaderPosition(ctx, query.SaveReaderPositionParams{
			OrgID: orgID, FileID: id, UserID: userID, Position: position,
			UpdatedAt: pgtype.Timestamptz{Time: s.now().UTC(), Valid: true},
		})
	})
	if err != nil {
		return fmt.Errorf("save reader position: %w", err)
	}
	return nil
}

// Content is an authenticated, same-origin object stream with Range support.
type Content struct {
	Body    io.ReadCloser
	Header  http.Header
	Partial bool
}

// Open streams an already-authorized ready file. start/end follow Storage.Read; end=-1 means EOF.
func (u *Uploader) Open(ctx context.Context, f query.File, start, end int64) (Content, error) {
	body, _, header, err := u.storage.Read(ctx, f.ObjectKey, start, end)
	if err != nil {
		return Content{}, err
	}
	return Content{Body: body, Header: header, Partial: start != 0 || end != f.SizeBytes-1}, nil
}

// ErrInUse protects files still referenced by an active library book or module step.
var ErrInUse = errors.New("file in use")

// ErrRange maps to 416 so the browser can retry without Range.
var ErrRange = errors.New("unsatisfiable range")

// Delete removes a ready file. Only the uploader or an admin may delete; quota is released after removal.
func (u *Uploader) Delete(ctx context.Context, userID, orgID, id uuid.UUID) error {
	return u.withLock(ctx, id, func(q *query.Queries) error {
		role, err := q.GetMemberRole(ctx, query.GetMemberRoleParams{OrgID: orgID, UserID: userID})
		if err != nil {
			return org.ErrNotMember
		}
		f, err := q.LockFile(ctx, query.LockFileParams{OrgID: orgID, ID: id})
		if err != nil {
			return fileError(err)
		}
		if f.Status == "deleted" {
			return nil
		}
		if f.UploadedBy != userID && role != query.OrgRoleAdmin {
			return org.ErrForbidden
		}
		if f.Status != "ready" && f.Status != "deleting" {
			return ErrState
		}
		inUse, err := q.FileInUse(ctx, query.FileInUseParams{OrgID: orgID, FileID: uuid.NullUUID{UUID: id, Valid: true}})
		if err != nil {
			return fmt.Errorf("check file use: %w", err)
		}
		if inUse {
			return ErrInUse
		}
		if err := u.meta.MarkDeleting(ctx, orgID, id); err != nil {
			return err
		}
		if err := u.storage.Delete(ctx, f.ObjectKey); err != nil {
			return err
		}
		return u.meta.ConfirmDeleted(ctx, orgID, id)
	})
}
