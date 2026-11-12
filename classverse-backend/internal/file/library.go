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

// List returns ready files with the caller's last position.
// shortcut: no draft/published split yet; every ready file is visible to members until L1/M1 add content access rules.
func (s *Service) List(ctx context.Context, userID, orgID uuid.UUID) ([]query.ListReadyFilesRow, error) {
	var rows []query.ListReadyFilesRow
	err := s.store.inTx(ctx, func(q *query.Queries) error {
		if err := member(ctx, q, userID, orgID, false); err != nil {
			return err
		}
		var err error
		rows, err = q.ListReadyFiles(ctx, query.ListReadyFilesParams{OrgID: orgID, UserID: userID})
		return err
	})
	return rows, err
}

// readable rechecks membership on every request, including every range request.
func (s *Service) readable(ctx context.Context, userID, orgID, id uuid.UUID) (query.File, error) {
	var f query.File
	err := s.store.inTx(ctx, func(q *query.Queries) error {
		if err := member(ctx, q, userID, orgID, false); err != nil {
			return err
		}
		var err error
		f, err = q.GetReadyFile(ctx, query.GetReadyFileParams{OrgID: orgID, ID: id})
		return fileError(err)
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
		if err := u.meta.MarkDeleting(ctx, orgID, id); err != nil {
			return err
		}
		if err := u.storage.Delete(ctx, f.ObjectKey); err != nil {
			return err
		}
		return u.meta.ConfirmDeleted(ctx, orgID, id)
	})
}
