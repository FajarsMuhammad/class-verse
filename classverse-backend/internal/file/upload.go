package file

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/platform/db/query"
)

const uploadLifetime = 24 * time.Hour

// UploadProgress exposes only safe metadata, never S3 upload IDs or object keys.
type UploadProgress struct {
	ID        uuid.UUID `json:"id"`
	Size      int64     `json:"size"`
	ChunkSize int64     `json:"chunk_size"`
	Parts     []int     `json:"parts"`
	State     string    `json:"state"`
	ExpiresAt time.Time `json:"expires_at"`
}

// Uploader coordinates durable Postgres ownership/quota with S3 multipart state.
type Uploader struct {
	meta    *Service
	storage *Storage
}

// NewUploader wires resumable uploads. Assembled files remain pending content validation.
func NewUploader(meta *Service, storage *Storage) *Uploader {
	return &Uploader{meta: meta, storage: storage}
}

// Start reserves quota once; retry subsequent operations using the returned file ID.
func (u *Uploader) Start(ctx context.Context, user, orgID uuid.UUID, name string, size int64, kind string, digest []byte) (UploadProgress, error) {
	return u.StartWithID(ctx, user, orgID, uuid.New(), name, size, kind, digest)
}

// cleanName keeps a display label only; object keys never use it.
func cleanName(name string) (string, error) {
	name = strings.TrimSpace(strings.ToValidUTF8(name, ""))
	if n := utf8.RuneCountInString(name); n < 1 || n > 200 || strings.ContainsFunc(name, unicode.IsControl) {
		return "", ErrInvalidUpload
	}
	return name, nil
}

// StartWithID makes a lost creation response retryable without reserving quota twice.
func (u *Uploader) StartWithID(ctx context.Context, user, orgID, id uuid.UUID, name string, size int64, kind string, digest []byte) (UploadProgress, error) {
	name, err := cleanName(name)
	if err != nil {
		return UploadProgress{}, err
	}
	var result UploadProgress
	err = u.withLock(ctx, id, func(q *query.Queries) error {
		f, err := q.GetOwnedFileUpload(ctx, query.GetOwnedFileUploadParams{OrgID: orgID, ID: id, UploadedBy: user})
		if err == nil {
			if f.SizeBytes != size || f.Kind != kind || !bytes.Equal(f.ExpectedSha256, digest) {
				return ErrInvalidUpload
			}
			result = progress(f)
			return nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		result, err = u.start(ctx, user, orgID, id, name, size, kind, digest)
		return err
	})
	return result, err
}

func (u *Uploader) start(ctx context.Context, user, orgID, id uuid.UUID, name string, size int64, kind string, digest []byte) (UploadProgress, error) {
	if id == uuid.Nil || len(digest) != sha256.Size {
		return UploadProgress{}, ErrInvalidUpload
	}
	f, err := u.meta.reserve(ctx, user, orgID, id, size, kind, name)
	if err != nil {
		return UploadProgress{}, err
	}
	multipartID, err := u.storage.BeginMultipart(ctx, f.ObjectKey)
	if err == nil {
		err = query.New(u.meta.store.pool).CreateFileUpload(ctx, query.CreateFileUploadParams{
			OrgID: orgID, FileID: f.ID, ExpectedSha256: digest, MultipartID: multipartID,
			ExpiresAt: pgtype.Timestamptz{Time: u.meta.now().Add(uploadLifetime).UTC(), Valid: true},
		})
	}
	if err != nil {
		cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 15*time.Second)
		defer cancel()
		if e := u.meta.MarkDeleting(cleanup, orgID, f.ID); e != nil {
			return UploadProgress{}, errors.Join(err, e)
		}
		if multipartID != "" {
			if e := u.storage.AbortMultipart(cleanup, f.ObjectKey, multipartID); e != nil {
				return UploadProgress{}, errors.Join(err, e)
			}
		}
		if e := u.meta.ConfirmDeleted(cleanup, orgID, f.ID); e != nil {
			return UploadProgress{}, errors.Join(err, e)
		}
		return UploadProgress{}, fmt.Errorf("start file upload: %w", err)
	}
	return UploadProgress{ID: f.ID, Size: size, ChunkSize: ChunkBytes, Parts: []int{}, State: "uploading", ExpiresAt: u.meta.now().Add(uploadLifetime).UTC()}, nil
}

// withOwned serializes storage operations using a connection-scoped lock, not an open DB transaction.
func (u *Uploader) withOwned(ctx context.Context, user, orgID, id uuid.UUID, fn func(*query.Queries, query.GetOwnedFileUploadRow) error) error {
	return u.withLock(ctx, id, func(q *query.Queries) error {
		f, err := q.GetOwnedFileUpload(ctx, query.GetOwnedFileUploadParams{OrgID: orgID, ID: id, UploadedBy: user})
		if err != nil {
			return fileError(err)
		}
		return fn(q, f)
	})
}

func (u *Uploader) withLock(ctx context.Context, id uuid.UUID, fn func(*query.Queries) error) error {
	conn, err := u.meta.store.pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("acquire upload connection: %w", err)
	}
	defer conn.Release()
	var locked bool
	// UUID is hashed by PostgreSQL; collisions only serialize unrelated requests.
	if err := conn.QueryRow(ctx, "SELECT pg_try_advisory_lock(hashtextextended($1, 0))", id.String()).Scan(&locked); err != nil {
		return err
	}
	if !locked {
		return ErrState
	}
	defer func() {
		unlock, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		var ok bool
		if err := conn.QueryRow(unlock, "SELECT pg_advisory_unlock(hashtextextended($1, 0))", id.String()).Scan(&ok); err != nil || !ok {
			_ = conn.Conn().Close(unlock)
		}
	}()
	return fn(query.New(conn))
}

func progress(f query.GetOwnedFileUploadRow) UploadProgress {
	return UploadProgress{ID: f.ID, Size: f.SizeBytes, ChunkSize: ChunkBytes, Parts: []int{}, State: f.State, ExpiresAt: f.ExpiresAt.Time}
}

// Status asks storage which parts persisted, including a part whose response was lost.
func (u *Uploader) Status(ctx context.Context, user, orgID, id uuid.UUID) (UploadProgress, error) {
	var result UploadProgress
	err := u.withOwned(ctx, user, orgID, id, func(q *query.Queries, f query.GetOwnedFileUploadRow) error {
		result = progress(f)
		if f.State == "canceled" || f.Status == "deleted" || f.Status == "deleting" {
			return ErrState
		}
		if f.State == "assembled" {
			return nil
		}
		if !u.meta.now().Before(f.ExpiresAt.Time) {
			return ErrState
		}
		parts, err := u.storage.Parts(ctx, f.ObjectKey, f.MultipartID)
		if err != nil {
			// Complete may have succeeded just before the process or response was lost.
			if storageErrorCode(err) == "NoSuchUpload" {
				return u.recoverAssembly(ctx, q, f, &result)
			}
			return err
		}
		for _, p := range parts {
			result.Parts = append(result.Parts, p.PartNumber)
		}
		return nil
	})
	return result, err
}

// Part accepts one exact-sized chunk; no partial chunk is committed on a broken request.
func (u *Uploader) Part(ctx context.Context, user, orgID, id uuid.UUID, part int, body io.Reader) error {
	return u.withOwned(ctx, user, orgID, id, func(_ *query.Queries, f query.GetOwnedFileUploadRow) error {
		if f.State != "uploading" || f.Status != "pending" || !u.meta.now().Before(f.ExpiresAt.Time) {
			return ErrState
		}
		size, err := chunkSize(f.SizeBytes, part)
		if err != nil {
			return err
		}
		data, err := io.ReadAll(io.LimitReader(body, size+1))
		if err != nil {
			return fmt.Errorf("read upload chunk: %w", err)
		}
		if int64(len(data)) != size {
			return ErrInvalidUpload
		}
		return u.storage.UploadPart(ctx, f.ObjectKey, f.MultipartID, part, bytes.NewReader(data), size)
	})
}

// Assemble checks all chunks and full-file SHA-256. It does not publish unvalidated content.
func (u *Uploader) Assemble(ctx context.Context, user, orgID, id uuid.UUID) (UploadProgress, error) {
	var result UploadProgress
	err := u.withOwned(ctx, user, orgID, id, func(q *query.Queries, f query.GetOwnedFileUploadRow) error {
		result = progress(f)
		if f.State == "canceled" || f.Status == "deleted" || f.Status == "deleting" {
			return ErrState
		}
		if f.State == "assembled" {
			return nil
		}
		if !u.meta.now().Before(f.ExpiresAt.Time) {
			return ErrState
		}
		parts, err := u.storage.Parts(ctx, f.ObjectKey, f.MultipartID)
		if err != nil {
			if storageErrorCode(err) == "NoSuchUpload" {
				return u.recoverAssembly(ctx, q, f, &result)
			}
			return err
		}
		if int64(len(parts)) != (f.SizeBytes+ChunkBytes-1)/ChunkBytes {
			return ErrState
		}
		for i, p := range parts {
			size, e := chunkSize(f.SizeBytes, i+1)
			if e != nil || p.PartNumber != i+1 || p.Size != size {
				return ErrState
			}
		}
		if err := u.storage.FinishMultipart(ctx, f.ObjectKey, f.MultipartID); err != nil {
			return err
		}
		return u.recoverAssembly(ctx, q, f, &result)
	})
	return result, err
}

func (u *Uploader) recoverAssembly(ctx context.Context, q *query.Queries, f query.GetOwnedFileUploadRow, result *UploadProgress) error {
	body, _, _, err := u.storage.Read(ctx, f.ObjectKey, 0, -1)
	if err != nil {
		return err
	}
	defer body.Close()
	digest := sha256.New()
	n, err := io.Copy(digest, io.LimitReader(body, f.SizeBytes+1))
	if err != nil {
		return fmt.Errorf("verify assembled file: %w", err)
	}
	contentType, verr := "", ErrInvalidUpload
	if n == f.SizeBytes && bytes.Equal(digest.Sum(nil), f.ExpectedSha256) {
		contentType, verr = u.sniffObject(ctx, f)
	}
	if verr != nil {
		// Parts are gone after assembly; drop the object so a bad file never holds quota.
		if err := u.discard(ctx, q, f); err != nil {
			return errors.Join(verr, err)
		}
		return verr
	}
	// Ready first: a crash before the next line is repaired by the next Assemble/Status call.
	if err := u.meta.complete(ctx, f.OrgID, f.ID, f.SizeBytes, f.ExpectedSha256, contentType); err != nil {
		return err
	}
	if err := q.MarkUploadAssembled(ctx, query.MarkUploadAssembledParams{OrgID: f.OrgID, FileID: f.ID}); err != nil {
		return err
	}
	result.State = "assembled"
	return nil
}

func (u *Uploader) sniffObject(ctx context.Context, f query.GetOwnedFileUploadRow) (string, error) {
	part := func(start, end int64) ([]byte, error) {
		body, _, _, err := u.storage.Read(ctx, f.ObjectKey, start, end)
		if err != nil {
			return nil, err
		}
		defer body.Close()
		return io.ReadAll(io.LimitReader(body, sniffBytes))
	}
	head, err := part(0, min(f.SizeBytes, sniffBytes)-1)
	if err != nil {
		return "", err
	}
	tail, err := part(max(0, f.SizeBytes-sniffBytes), f.SizeBytes-1)
	if err != nil {
		return "", err
	}
	return sniff(f.Kind, head, tail)
}

// Cancel aborts chunks and removes any assembled object before releasing quota.
func (u *Uploader) Cancel(ctx context.Context, user, orgID, id uuid.UUID) error {
	return u.withOwned(ctx, user, orgID, id, func(q *query.Queries, f query.GetOwnedFileUploadRow) error {
		if f.State == "canceled" || f.Status == "deleted" {
			return nil
		}
		if f.Status == "ready" {
			return ErrState
		}
		return u.discard(ctx, q, f)
	})
}

// discard removes chunks and object, then releases quota; every step is retry-safe.
func (u *Uploader) discard(ctx context.Context, q *query.Queries, f query.GetOwnedFileUploadRow) error {
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
}

// UploadSample stores small trusted bytes through the same path as browser uploads.
// Retrying with the same id reuses the session and skips parts that already exist.
func (u *Uploader) UploadSample(ctx context.Context, userID, orgID, id uuid.UUID, name, kind string, data []byte) error {
	digest := sha256.Sum256(data)
	s, err := u.StartWithID(ctx, userID, orgID, id, name, int64(len(data)), kind, digest[:])
	if err != nil {
		return err
	}
	if s.State == "assembled" {
		return nil
	}
	done := map[int]bool{}
	for _, p := range s.Parts {
		done[p] = true
	}
	for part := 1; int64(part-1)*ChunkBytes < int64(len(data)); part++ {
		if done[part] {
			continue
		}
		start := int64(part-1) * ChunkBytes
		if err := u.Part(ctx, userID, orgID, id, part, bytes.NewReader(data[start:min(start+ChunkBytes, int64(len(data)))])); err != nil {
			return err
		}
	}
	_, err = u.Assemble(ctx, userID, orgID, id)
	return err
}
