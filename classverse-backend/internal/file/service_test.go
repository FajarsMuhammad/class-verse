package file

import (
	"crypto/sha256"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/org"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/platform/db/dbtest"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/platform/db/query"
)

func TestValidateUpload(t *testing.T) {
	for _, tc := range []struct {
		name string
		size int64
		kind string
	}{
		{"empty", 0, "pdf"}, {"negative", -1, "pdf"}, {"oversize", MaxFileBytes + 1, "pdf"},
		{"unsupported", 12, "epub"}, {"unknown", 12, "application/pdf"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := validateUpload(tc.size, tc.kind); !errors.Is(err, ErrInvalidUpload) {
				t.Fatalf("got %v", err)
			}
		})
	}
	for _, kind := range []string{"pdf", "audio"} {
		if err := validateUpload(MaxFileBytes, kind); err != nil {
			t.Fatal(err)
		}
	}
}

func TestReservationLifecycle(t *testing.T) {
	pool := dbtest.New(t)
	ctx := t.Context()
	now := time.Date(2026, 11, 12, 8, 0, 0, 0, time.UTC)
	clock := func() time.Time { return now }
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	q := query.New(pool)
	user, err := q.CreateTestUser(ctx, query.CreateTestUserParams{Email: "file-teacher@example.test", DisplayName: "Teacher", BirthDate: pgtype.Date{Time: time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC), Valid: true}})
	if err != nil {
		t.Fatal(err)
	}
	orgs := org.NewService(org.NewStore(pool), clock, log)
	school, err := orgs.Create(ctx, user.ID, org.CreateInput{Name: "File School", Slug: "file-school"})
	if err != nil {
		t.Fatal(err)
	}
	svc := NewService(NewStore(pool), clock)

	if _, err := svc.Reserve(ctx, uuid.New(), school.ID, 10, "pdf"); !errors.Is(err, org.ErrNotMember) {
		t.Fatalf("nonmember: %v", err)
	}
	// Make the same member a participant; content authorship must be checked server-side.
	if _, err := pool.Exec(ctx, "UPDATE memberships SET role='participant' WHERE org_id=$1 AND user_id=$2", school.ID, user.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Reserve(ctx, user.ID, school.ID, 10, "pdf"); !errors.Is(err, org.ErrForbidden) {
		t.Fatalf("participant: %v", err)
	}
	if _, err := pool.Exec(ctx, "UPDATE memberships SET role='teacher' WHERE org_id=$1 AND user_id=$2", school.ID, user.ID); err != nil {
		t.Fatal(err)
	}

	reserved, err := svc.Reserve(ctx, user.ID, school.ID, MaxFileBytes, "pdf")
	if err != nil {
		t.Fatal(err)
	}
	if reserved.Status != "pending" || reserved.OrgID != school.ID || reserved.SizeBytes != MaxFileBytes {
		t.Fatalf("invalid reservation: %+v", reserved)
	}
	usage, err := svc.Usage(ctx, user.ID, school.ID)
	if err != nil || usage.UsedBytes != MaxFileBytes || usage.QuotaBytes != DefaultQuotaBytes {
		t.Fatalf("usage=%+v err=%v", usage, err)
	}
	if _, err := svc.Usage(ctx, uuid.New(), school.ID); !errors.Is(err, org.ErrNotMember) {
		t.Fatalf("usage leaked: %v", err)
	}

	digest := sha256.Sum256([]byte("example"))
	if err := svc.Complete(ctx, uuid.New(), reserved.ID, MaxFileBytes, digest[:]); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross org complete: %v", err)
	}
	if err := svc.Complete(ctx, school.ID, reserved.ID, MaxFileBytes-1, digest[:]); !errors.Is(err, ErrInvalidUpload) {
		t.Fatalf("short upload accepted: %v", err)
	}
	if err := svc.Complete(ctx, school.ID, reserved.ID, MaxFileBytes, []byte("bad")); !errors.Is(err, ErrInvalidUpload) {
		t.Fatalf("bad digest accepted: %v", err)
	}
	if err := svc.Complete(ctx, school.ID, reserved.ID, MaxFileBytes, digest[:]); err != nil {
		t.Fatal(err)
	}
	if err := svc.Complete(ctx, school.ID, reserved.ID, MaxFileBytes, digest[:]); err != nil {
		t.Fatalf("retry: %v", err)
	}
	other := sha256.Sum256([]byte("different"))
	if err := svc.Complete(ctx, school.ID, reserved.ID, MaxFileBytes, other[:]); !errors.Is(err, ErrState) {
		t.Fatalf("different finalize accepted: %v", err)
	}

	if err := svc.MarkDeleting(ctx, school.ID, reserved.ID); err != nil {
		t.Fatal(err)
	}
	usage, err = svc.Usage(ctx, user.ID, school.ID)
	if err != nil || usage.UsedBytes != MaxFileBytes {
		t.Fatalf("quota freed before object deletion: %+v %v", usage, err)
	}
	if err := svc.ConfirmDeleted(ctx, school.ID, reserved.ID); err != nil {
		t.Fatal(err)
	}
	if err := svc.ConfirmDeleted(ctx, school.ID, reserved.ID); err != nil {
		t.Fatalf("delete retry: %v", err)
	}
	usage, err = svc.Usage(ctx, user.ID, school.ID)
	if err != nil || usage.UsedBytes != 0 {
		t.Fatalf("usage after delete: %+v %v", usage, err)
	}
	if err := svc.Complete(ctx, school.ID, reserved.ID, MaxFileBytes, digest[:]); !errors.Is(err, ErrState) {
		t.Fatalf("deleted file revived: %v", err)
	}

	// Interrupted uploads retain capacity until cleanup confirms object removal.
	pending, err := svc.Reserve(ctx, user.ID, school.ID, 25, "audio")
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.ConfirmDeleted(ctx, school.ID, pending.ID); !errors.Is(err, ErrState) {
		t.Fatalf("pending quota released without cleanup: %v", err)
	}
	if err := svc.MarkDeleting(ctx, uuid.New(), pending.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-org deletion: %v", err)
	}
	if err := svc.MarkDeleting(ctx, school.ID, pending.ID); err != nil {
		t.Fatal(err)
	}
	if err := svc.MarkDeleting(ctx, school.ID, pending.ID); err != nil {
		t.Fatalf("cleanup retry: %v", err)
	}
	if err := svc.Complete(ctx, school.ID, pending.ID, 25, digest[:]); !errors.Is(err, ErrState) {
		t.Fatalf("canceled upload completed: %v", err)
	}
	if err := svc.ConfirmDeleted(ctx, school.ID, pending.ID); err != nil {
		t.Fatal(err)
	}

	// A metadata write failure must roll back the earlier quota reservation.
	if _, err := pool.Exec(ctx, "ALTER TABLE files ADD CONSTRAINT test_reject_audio CHECK (kind <> 'audio') NOT VALID"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Reserve(ctx, user.ID, school.ID, 25, "audio"); err == nil {
		t.Fatal("expected metadata insertion failure")
	}
	usage, err = svc.Usage(ctx, user.ID, school.ID)
	if err != nil || usage.UsedBytes != 0 {
		t.Fatalf("failed insert leaked quota: %+v %v", usage, err)
	}
	if _, err := pool.Exec(ctx, "ALTER TABLE files DROP CONSTRAINT test_reject_audio"); err != nil {
		t.Fatal(err)
	}

	// Reduce this disposable school's quota to make contention cheap to exercise.
	if _, err := pool.Exec(ctx, "UPDATE file_storage SET quota_bytes=100 WHERE org_id=$1", school.ID); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for range 2 {
		wg.Go(func() { _, e := svc.Reserve(ctx, user.ID, school.ID, 60, "audio"); results <- e })
	}
	wg.Wait()
	close(results)
	successes, denied := 0, 0
	for e := range results {
		switch {
		case e == nil:
			successes++
		case errors.Is(e, ErrQuota):
			denied++
		default:
			t.Fatal(e)
		}
	}
	if successes != 1 || denied != 1 {
		t.Fatalf("concurrent results: success=%d quota=%d", successes, denied)
	}
	if _, err := svc.Reserve(ctx, user.ID, school.ID, 40, "pdf"); err != nil {
		t.Fatalf("exact quota: %v", err)
	}
	if _, err := svc.Reserve(ctx, user.ID, school.ID, 1, "pdf"); !errors.Is(err, ErrQuota) {
		t.Fatalf("quota overflow: %v", err)
	}
	usage, err = svc.Usage(ctx, user.ID, school.ID)
	if err != nil || usage.UsedBytes != 100 {
		t.Fatalf("quota accounting: %+v %v", usage, err)
	}
}
