package file

import (
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"testing"
	"time"
)

func TestChunkBounds(t *testing.T) {
	for _, tc := range []struct {
		name string
		size int64
		part int
		want int64
	}{
		{"first", ChunkBytes + 3, 1, ChunkBytes}, {"last", ChunkBytes + 3, 2, 3},
		{"single", 3, 1, 3}, {"exact", ChunkBytes, 1, ChunkBytes},
		{"zero part", 10, 0, 0}, {"past last", 10, 2, 0}, {"empty", 0, 1, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			n, err := chunkSize(tc.size, tc.part)
			if tc.want == 0 {
				if !errors.Is(err, ErrInvalidUpload) {
					t.Fatalf("got %d %v", n, err)
				}
				return
			}
			if err != nil || n != tc.want {
				t.Fatalf("got %d %v want %d", n, err, tc.want)
			}
		})
	}
}

func TestMultipartIntegration(t *testing.T) {
	endpoint := os.Getenv("TEST_STORAGE_ENDPOINT")
	if endpoint == "" {
		t.Skip("TEST_STORAGE_* not configured")
	}
	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()
	s, err := NewStorage(endpoint, os.Getenv("TEST_STORAGE_BUCKET"), os.Getenv("TEST_STORAGE_ACCESS_KEY"), os.Getenv("TEST_STORAGE_SECRET_KEY"), false)
	if err != nil {
		t.Fatal(err)
	}
	key := "multipart/" + time.Now().UTC().Format("20060102T150405.000000000")
	id, err := s.BeginMultipart(ctx, key)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.AbortMultipart(ctx, key, id); _ = s.Delete(ctx, key) }()
	data := strings.Repeat("x", int(ChunkBytes))
	if err := s.UploadPart(ctx, key, id, 1, strings.NewReader(data), ChunkBytes); err != nil {
		t.Fatal(err)
	}
	// Recreate adapter to prove progress is in storage, not process memory.
	resumed, err := NewStorage(endpoint, os.Getenv("TEST_STORAGE_BUCKET"), os.Getenv("TEST_STORAGE_ACCESS_KEY"), os.Getenv("TEST_STORAGE_SECRET_KEY"), false)
	if err != nil {
		t.Fatal(err)
	}
	parts, err := resumed.Parts(ctx, key, id)
	if err != nil || len(parts) != 1 || parts[0].Size != ChunkBytes {
		t.Fatalf("parts=%v err=%v", parts, err)
	}
	if err := resumed.UploadPart(ctx, key, id, 2, strings.NewReader("end"), 3); err != nil {
		t.Fatal(err)
	}
	if err := resumed.FinishMultipart(ctx, key, id); err != nil {
		t.Fatal(err)
	}
	body, _, _, err := s.Read(ctx, key, ChunkBytes, ChunkBytes+2)
	if err != nil {
		t.Fatal(err)
	}
	defer body.Close()
	got, err := io.ReadAll(body)
	if err != nil || string(got) != "end" {
		t.Fatalf("tail=%q err=%v", got, err)
	}
}
