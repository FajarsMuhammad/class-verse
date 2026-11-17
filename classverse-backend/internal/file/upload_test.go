package file

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"io"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/org"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/platform/db/dbtest"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/platform/db/query"
)

func TestResumableUpload(t *testing.T) {
	endpoint := os.Getenv("TEST_STORAGE_ENDPOINT")
	if endpoint == "" {
		t.Skip("TEST_STORAGE_* not configured")
	}
	pool := dbtest.New(t)
	ctx := t.Context()
	now := time.Now().UTC()
	clock := func() time.Time { return now }
	q := query.New(pool)
	user, err := q.CreateTestUser(ctx, query.CreateTestUserParams{Email: "chunks@example.test", DisplayName: "Teacher", BirthDate: pgtype.Date{Time: time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC), Valid: true}})
	if err != nil {
		t.Fatal(err)
	}
	school, err := org.NewService(org.NewStore(pool), clock, slog.New(slog.NewTextHandler(io.Discard, nil))).Create(ctx, user.ID, org.CreateInput{Name: "Chunk School", Slug: "chunk-school"})
	if err != nil {
		t.Fatal(err)
	}
	storage, err := NewStorage(endpoint, os.Getenv("TEST_STORAGE_BUCKET"), os.Getenv("TEST_STORAGE_ACCESS_KEY"), os.Getenv("TEST_STORAGE_SECRET_KEY"), false)
	if err != nil {
		t.Fatal(err)
	}
	meta := NewService(NewStore(pool), clock)
	u := NewUploader(meta, storage)
	data := []byte("%PDF-1.7\n" + strings.Repeat("x", int(ChunkBytes)) + "\n%%EOF\n")
	digest := sha256.Sum256(data)
	s, err := u.Start(ctx, user.ID, school.ID, "buku.pdf", int64(len(data)), "pdf", digest[:])
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = u.Cancel(ctx, user.ID, school.ID, s.ID) }()
	if err := u.Part(ctx, uuid.New(), school.ID, s.ID, 1, bytes.NewReader(data[:ChunkBytes])); !errors.Is(err, ErrNotFound) {
		t.Fatalf("ownership: %v", err)
	}
	if err := u.Part(ctx, user.ID, uuid.New(), s.ID, 1, bytes.NewReader(data[:ChunkBytes])); !errors.Is(err, ErrNotFound) {
		t.Fatalf("org: %v", err)
	}
	if _, err := u.Assemble(ctx, user.ID, school.ID, s.ID); !errors.Is(err, ErrState) {
		t.Fatalf("incomplete: %v", err)
	}
	if err := u.Part(ctx, user.ID, school.ID, s.ID, 1, strings.NewReader("broken")); !errors.Is(err, ErrInvalidUpload) {
		t.Fatalf("short: %v", err)
	}
	if err := u.Part(ctx, user.ID, school.ID, s.ID, 1, bytes.NewReader(data[:ChunkBytes])); err != nil {
		t.Fatal(err)
	}
	// Simulate process restart and a lost chunk response: request authoritative status.
	u = NewUploader(NewService(NewStore(pool), clock), storage)
	resumed, err := u.Status(ctx, user.ID, school.ID, s.ID)
	if err != nil || len(resumed.Parts) != 1 || resumed.Parts[0] != 1 {
		t.Fatalf("resume: %+v %v", resumed, err)
	}
	if err := u.Part(ctx, user.ID, school.ID, s.ID, 2, bytes.NewReader(data[ChunkBytes:])); err != nil {
		t.Fatal(err)
	}
	// Crash after storage completes but before metadata commit must recover without new chunks.
	row, err := q.GetOwnedFileUpload(ctx, query.GetOwnedFileUploadParams{OrgID: school.ID, ID: s.ID, UploadedBy: user.ID})
	if err != nil {
		t.Fatal(err)
	}
	if err := storage.FinishMultipart(ctx, row.ObjectKey, row.MultipartID); err != nil {
		t.Fatal(err)
	}
	assembled, err := u.Assemble(ctx, user.ID, school.ID, s.ID)
	if err != nil || assembled.State != "assembled" {
		t.Fatalf("complete: %+v %v", assembled, err)
	}
	if _, err := u.Assemble(ctx, user.ID, school.ID, s.ID); err != nil {
		t.Fatal(err)
	}
	usage, err := meta.Usage(ctx, user.ID, school.ID)
	if err != nil || usage.UsedBytes != int64(len(data)) {
		t.Fatalf("usage %+v %v", usage, err)
	}
	f, err := q.LockFile(ctx, query.LockFileParams{OrgID: school.ID, ID: s.ID})
	if err != nil || f.Status != "ready" || f.ContentType.String != "application/pdf" || f.Name != "buku.pdf" {
		t.Fatalf("not published: %+v %v", f, err)
	}
	if err := u.Cancel(ctx, user.ID, school.ID, s.ID); !errors.Is(err, ErrState) {
		t.Fatalf("cancel ready: %v", err)
	}
	ready, err := meta.readable(ctx, user.ID, school.ID, s.ID)
	if err != nil {
		t.Fatal(err)
	}
	c, err := u.Open(ctx, ready, 0, 7)
	if err != nil {
		t.Fatal(err)
	}
	head, _ := io.ReadAll(c.Body)
	_ = c.Body.Close()
	if string(head) != "%PDF-1.7" || !c.Partial {
		t.Fatalf("range %q partial=%v", head, c.Partial)
	}
	if err := meta.SavePosition(ctx, user.ID, school.ID, s.ID, 2); err != nil {
		t.Fatal(err)
	}
	if err := meta.SavePosition(ctx, user.ID, school.ID, s.ID, -1); !errors.Is(err, ErrInvalidUpload) {
		t.Fatalf("negative position: %v", err)
	}
	if err := meta.SavePosition(ctx, uuid.New(), school.ID, s.ID, 1); !errors.Is(err, org.ErrNotMember) {
		t.Fatalf("outsider position: %v", err)
	}
	list, err := meta.List(ctx, user.ID, school.ID)
	if err != nil || len(list) != 1 || list[0].Position != 2 {
		t.Fatalf("list %+v %v", list, err)
	}
	if err := u.Delete(ctx, user.ID, school.ID, s.ID); err != nil {
		t.Fatal(err)
	}
	if err := u.Delete(ctx, user.ID, school.ID, s.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := meta.readable(ctx, user.ID, school.ID, s.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("deleted still readable: %v", err)
	}
	usage, err = meta.Usage(ctx, user.ID, school.ID)
	if err != nil || usage.UsedBytes != 0 {
		t.Fatalf("release %+v %v", usage, err)
	}
	// Checksum-valid but fake content is rejected and its quota released immediately.
	fake := []byte("<html>not a pdf</html>")
	fakeDigest := sha256.Sum256(fake)
	bad, err := u.Start(ctx, user.ID, school.ID, "palsu.pdf", int64(len(fake)), "pdf", fakeDigest[:])
	if err != nil {
		t.Fatal(err)
	}
	if err := u.Part(ctx, user.ID, school.ID, bad.ID, 1, bytes.NewReader(fake)); err != nil {
		t.Fatal(err)
	}
	if _, err := u.Assemble(ctx, user.ID, school.ID, bad.ID); !errors.Is(err, ErrInvalidUpload) {
		t.Fatalf("fake pdf: %v", err)
	}
	usage, err = meta.Usage(ctx, user.ID, school.ID)
	if err != nil || usage.UsedBytes != 0 {
		t.Fatalf("fake quota %+v %v", usage, err)
	}
	expiring, err := u.Start(ctx, user.ID, school.ID, "buku.pdf", int64(len(data)), "pdf", digest[:])
	if err != nil {
		t.Fatal(err)
	}
	if err := u.Part(ctx, user.ID, school.ID, expiring.ID, 1, bytes.NewReader(data[:ChunkBytes])); err != nil {
		t.Fatal(err)
	}
	if err := u.CleanupExpired(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := u.Status(ctx, user.ID, school.ID, expiring.ID); err != nil {
		t.Fatal(err)
	}
	now = now.Add(25 * time.Hour)
	if err := u.CleanupExpired(ctx); err != nil {
		t.Fatal(err)
	}
	if err := u.CleanupExpired(ctx); err != nil {
		t.Fatal(err)
	}
	usage, err = meta.Usage(ctx, user.ID, school.ID)
	if err != nil || usage.UsedBytes != 0 {
		t.Fatalf("expired quota %+v %v", usage, err)
	}
}
