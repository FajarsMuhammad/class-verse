package file

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/minio/minio-go/v7"
)

func storageErrorCode(err error) string {
	var response minio.ErrorResponse
	if errors.As(err, &response) {
		return response.Code
	}
	return ""
}

// ChunkBytes respects S3's 5 MiB minimum for every part except the last.
const ChunkBytes int64 = 5 * 1024 * 1024

func chunkSize(size int64, part int) (int64, error) {
	if size < 1 || size > MaxFileBytes || part < 1 || int64(part) > (size+ChunkBytes-1)/ChunkBytes {
		return 0, ErrInvalidUpload
	}
	remaining := size - int64(part-1)*ChunkBytes
	return min(ChunkBytes, remaining), nil
}

// BeginMultipart allocates durable S3 upload state without uploading file bytes.
func (s *Storage) BeginMultipart(ctx context.Context, key string) (string, error) {
	id, err := (minio.Core{Client: s.client}).NewMultipartUpload(ctx, s.bucket, key, minio.PutObjectOptions{})
	if err != nil {
		return "", fmt.Errorf("begin multipart: %w", err)
	}
	return id, nil
}

// UploadPart replaces a single numbered part; a lost response can be retried safely.
func (s *Storage) UploadPart(ctx context.Context, key, id string, part int, body io.Reader, size int64) error {
	_, err := (minio.Core{Client: s.client}).PutObjectPart(ctx, s.bucket, key, id, part, body, size, minio.PutObjectPartOptions{})
	if err != nil {
		return fmt.Errorf("upload part: %w", err)
	}
	return nil
}

// Parts reads authoritative progress; at 50 MB there can be at most ten parts.
func (s *Storage) Parts(ctx context.Context, key, id string) ([]minio.ObjectPart, error) {
	r, err := (minio.Core{Client: s.client}).ListObjectParts(ctx, s.bucket, key, id, 0, 100)
	if err != nil {
		return nil, fmt.Errorf("list upload parts: %w", err)
	}
	if r.IsTruncated {
		return nil, ErrState
	}
	return r.ObjectParts, nil
}

// FinishMultipart combines persisted parts; caller verifies completeness and digest.
func (s *Storage) FinishMultipart(ctx context.Context, key, id string) error {
	parts, err := s.Parts(ctx, key, id)
	if err != nil {
		return err
	}
	complete := make([]minio.CompletePart, 0, len(parts))
	for _, p := range parts {
		complete = append(complete, minio.CompletePart{PartNumber: p.PartNumber, ETag: p.ETag})
	}
	_, err = (minio.Core{Client: s.client}).CompleteMultipartUpload(ctx, s.bucket, key, id, complete, minio.PutObjectOptions{})
	if err != nil {
		return fmt.Errorf("complete multipart: %w", err)
	}
	return nil
}

// AbortMultipart removes incomplete chunks; already-removed uploads are harmless.
func (s *Storage) AbortMultipart(ctx context.Context, key, id string) error {
	err := (minio.Core{Client: s.client}).AbortMultipartUpload(ctx, s.bucket, key, id)
	if storageErrorCode(err) == "NoSuchUpload" {
		return nil
	}
	if err != nil {
		return fmt.Errorf("abort multipart: %w", err)
	}
	return nil
}
