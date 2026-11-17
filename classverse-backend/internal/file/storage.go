// Package file manages private learning files.
package file

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// Storage accesses a pre-provisioned private bucket with application credentials.
type Storage struct {
	client *minio.Client
	bucket string
}

// NewStorage connects to S3-compatible storage; endpoint is host:port, not a URL.
func NewStorage(endpoint, bucket, accessKey, secretKey string, secure bool) (*Storage, error) {
	if endpoint == "" || bucket == "" || accessKey == "" || secretKey == "" || strings.Contains(endpoint, "://") {
		return nil, fmt.Errorf("storage requires host, bucket and credentials")
	}
	client, err := minio.New(endpoint, &minio.Options{
		Creds: credentials.NewStaticV4(accessKey, secretKey, ""), Secure: secure, Region: "us-east-1",
	})
	if err != nil {
		return nil, fmt.Errorf("configure storage: %w", err)
	}
	return &Storage{client: client, bucket: bucket}, nil
}

// Put uploads a known-size, already validated stream. Bucket creation is operator-only.
func (s *Storage) Put(ctx context.Context, key string, body io.Reader, size int64, contentType string) error {
	if size < 0 {
		return fmt.Errorf("upload file: size must be known")
	}
	_, err := s.client.PutObject(ctx, s.bucket, key, body, size, minio.PutObjectOptions{ContentType: contentType})
	if err != nil {
		return fmt.Errorf("upload file: %w", err)
	}
	return nil
}

// Read returns an authenticated object stream. end=-1 reads through EOF.
func (s *Storage) Read(ctx context.Context, key string, start, end int64) (io.ReadCloser, minio.ObjectInfo, http.Header, error) {
	opts := minio.GetObjectOptions{}
	if start != 0 || end != -1 {
		if start < 0 || (end != -1 && end < start) {
			return nil, minio.ObjectInfo{}, nil, fmt.Errorf("read file: invalid range")
		}
		if err := opts.SetRange(start, end); err != nil {
			return nil, minio.ObjectInfo{}, nil, fmt.Errorf("read file range: %w", err)
		}
	}
	body, info, headers, err := (minio.Core{Client: s.client}).GetObject(ctx, s.bucket, key, opts)
	if err != nil {
		return nil, minio.ObjectInfo{}, nil, fmt.Errorf("read file: %w", err)
	}
	return body, info, headers, nil
}

// Delete removes an object; callers release quota only after success.
func (s *Storage) Delete(ctx context.Context, key string) error {
	if err := s.client.RemoveObject(ctx, s.bucket, key, minio.RemoveObjectOptions{}); err != nil {
		return fmt.Errorf("delete file: %w", err)
	}
	return nil
}
