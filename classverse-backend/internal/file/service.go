package file

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/org"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/platform/db/query"
)

// Storage limits use decimal bytes to match the UI's MB/GB units.
const (
	MaxFileBytes      int64 = 50_000_000
	DefaultQuotaBytes int64 = 5_000_000_000
)

// Domain errors do not expose bucket names, object keys or credentials.
var (
	ErrInvalidUpload = errors.New("invalid file upload")
	ErrQuota         = errors.New("storage quota exceeded")
	ErrNotFound      = errors.New("file not found")
	ErrState         = errors.New("file state conflict")
)

// Service reserves capacity before storage I/O and releases it only after deletion.
type Service struct {
	store *Store
	now   func() time.Time
}

// NewService wires file metadata operations.
func NewService(store *Store, now func() time.Time) *Service { return &Service{store: store, now: now} }

func validateUpload(size int64, kind string) error {
	if size < 1 || size > MaxFileBytes || (kind != "pdf" && kind != "audio") {
		return ErrInvalidUpload
	}
	return nil
}

func member(ctx context.Context, q *query.Queries, userID, orgID uuid.UUID, write bool) error {
	staff, err := isStaff(ctx, q, userID, orgID)
	if err != nil {
		return err
	}
	if write && !staff {
		return org.ErrForbidden
	}
	return nil
}

// isStaff reports teacher/admin; non-members get org.ErrNotMember.
func isStaff(ctx context.Context, q *query.Queries, userID, orgID uuid.UUID) (bool, error) {
	role, err := q.GetMemberRole(ctx, query.GetMemberRoleParams{OrgID: orgID, UserID: userID})
	if errors.Is(err, pgx.ErrNoRows) {
		return false, org.ErrNotMember
	}
	if err != nil {
		return false, fmt.Errorf("file membership: %w", err)
	}
	return role == query.OrgRoleTeacher || role == query.OrgRoleAdmin, nil
}

// Reserve creates a pending upload and charges its whole declared size atomically.
func (s *Service) Reserve(ctx context.Context, userID, orgID uuid.UUID, size int64, kind string) (query.File, error) {
	return s.reserve(ctx, userID, orgID, uuid.New(), size, kind, "file")
}

func (s *Service) reserve(ctx context.Context, userID, orgID, id uuid.UUID, size int64, kind, name string) (query.File, error) {
	if err := validateUpload(size, kind); err != nil {
		return query.File{}, err
	}
	var result query.File
	err := s.store.inTx(ctx, func(q *query.Queries) error {
		if err := member(ctx, q, userID, orgID, true); err != nil {
			return err
		}
		if err := q.EnsureFileStorage(ctx, orgID); err != nil {
			return fmt.Errorf("ensure file quota: %w", err)
		}
		if _, err := q.ReserveFileBytes(ctx, query.ReserveFileBytesParams{OrgID: orgID, SizeBytes: size}); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrQuota
			}
			return fmt.Errorf("reserve file quota: %w", err)
		}
		var err error
		result, err = q.CreatePendingFile(ctx, query.CreatePendingFileParams{
			ID: id, OrgID: orgID, UploadedBy: userID, Kind: kind, Name: name, SizeBytes: size,
			ObjectKey: orgID.String() + "/" + id.String(), CreatedAt: pgtype.Timestamptz{Time: s.now().UTC(), Valid: true},
		})
		if err != nil {
			return fmt.Errorf("create pending file: %w", err)
		}
		return nil
	})
	return result, err
}

// Usage is visible only to members and includes unfinished uploads and deletes.
func (s *Service) Usage(ctx context.Context, userID, orgID uuid.UUID) (query.FileStorage, error) {
	var result query.FileStorage
	err := s.store.inTx(ctx, func(q *query.Queries) error {
		if err := member(ctx, q, userID, orgID, false); err != nil {
			return err
		}
		if err := q.EnsureFileStorage(ctx, orgID); err != nil {
			return fmt.Errorf("ensure file quota: %w", err)
		}
		var err error
		result, err = q.GetFileUsage(ctx, orgID)
		if err != nil {
			return fmt.Errorf("get file quota: %w", err)
		}
		return nil
	})
	return result, err
}

// Complete is internal to the upload worker, after format, byte count and digest validation.
func (s *Service) Complete(ctx context.Context, orgID, id uuid.UUID, size int64, digest []byte) error {
	return s.complete(ctx, orgID, id, size, digest, "application/pdf")
}

func (s *Service) complete(ctx context.Context, orgID, id uuid.UUID, size int64, digest []byte, contentType string) error {
	if size < 1 || size > MaxFileBytes || len(digest) != 32 || contentType == "" {
		return ErrInvalidUpload
	}
	return s.store.inTx(ctx, func(q *query.Queries) error {
		f, err := q.LockFile(ctx, query.LockFileParams{OrgID: orgID, ID: id})
		if err != nil {
			return fileError(err)
		}
		if size != f.SizeBytes {
			return ErrInvalidUpload
		}
		if f.Status == "ready" && bytes.Equal(f.Sha256, digest) {
			return nil
		}
		if f.Status != "pending" {
			return ErrState
		}
		return q.CompleteFile(ctx, query.CompleteFileParams{OrgID: orgID, ID: id, Sha256: digest, ContentType: pgtype.Text{String: contentType, Valid: true}, UpdatedAt: pgtype.Timestamptz{Time: s.now().UTC(), Valid: true}})
	})
}

// MarkDeleting is internal to cleanup; it deliberately keeps quota charged.
func (s *Service) MarkDeleting(ctx context.Context, orgID, id uuid.UUID) error {
	return s.store.inTx(ctx, func(q *query.Queries) error {
		f, err := q.LockFile(ctx, query.LockFileParams{OrgID: orgID, ID: id})
		if err != nil {
			return fileError(err)
		}
		if f.Status == "deleted" || f.Status == "deleting" {
			return nil
		}
		return q.MarkFileDeleting(ctx, query.MarkFileDeletingParams{OrgID: orgID, ID: id, UpdatedAt: pgtype.Timestamptz{Time: s.now().UTC(), Valid: true}})
	})
}

// ConfirmDeleted is internal to cleanup, after object removal has succeeded.
func (s *Service) ConfirmDeleted(ctx context.Context, orgID, id uuid.UUID) error {
	return s.store.inTx(ctx, func(q *query.Queries) error {
		f, err := q.LockFile(ctx, query.LockFileParams{OrgID: orgID, ID: id})
		if err != nil {
			return fileError(err)
		}
		if f.Status == "deleted" {
			return nil
		}
		if f.Status != "deleting" {
			return ErrState
		}
		n, err := q.ReleaseFileBytes(ctx, query.ReleaseFileBytesParams{OrgID: orgID, SizeBytes: f.SizeBytes})
		if err != nil {
			return fmt.Errorf("release file quota: %w", err)
		}
		if n != 1 {
			return ErrState
		}
		return q.ConfirmFileDeleted(ctx, query.ConfirmFileDeletedParams{OrgID: orgID, ID: id, DeletedAt: pgtype.Timestamptz{Time: s.now().UTC(), Valid: true}})
	})
}
