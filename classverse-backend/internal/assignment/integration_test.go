package assignment_test

import (
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/assignment"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/module"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/org"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/platform/db/dbtest"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/platform/db/query"
)

type fixture struct {
	pool                                                           *pgxpool.Pool
	svc                                                            *assignment.Service
	org, admin, student, other, year, class, module, first, second uuid.UUID
}

func setup(t *testing.T) fixture {
	t.Helper()
	p := dbtest.New(t)
	ctx := t.Context()
	f := fixture{pool: p, org: uuid.New(), admin: uuid.New(), student: uuid.New(), other: uuid.New(), year: uuid.New(), class: uuid.New(), module: uuid.New(), first: uuid.New(), second: uuid.New()}
	for _, u := range []uuid.UUID{f.admin, f.student, f.other} {
		if _, err := p.Exec(ctx, `INSERT INTO users(id,email,display_name) VALUES($1,$2,'Siswa')`, u, u.String()+"@test.invalid"); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := p.Exec(ctx, `INSERT INTO organizations(id,name,slug,created_by) VALUES($1,'Sekolah','assignment-test',$2)`, f.org, f.admin); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Exec(ctx, `INSERT INTO memberships(org_id,user_id,role) VALUES($1,$2,'admin'),($1,$3,'participant')`, f.org, f.admin, f.student); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Exec(ctx, `INSERT INTO academic_years(id,org_id,label,active,created_at) VALUES($1,$2,'2026/2027',true,now())`, f.year, f.org); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Exec(ctx, `INSERT INTO classes(id,org_id,academic_year_id,name,created_by,created_at) VALUES($1,$2,$3,'XI A',$4,now())`, f.class, f.org, f.year, f.admin); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Exec(ctx, `INSERT INTO class_enrollments(id,org_id,academic_year_id,class_id,user_id,started_at,created_by) VALUES($1,$2,$3,$4,$5,now(),$6)`, uuid.New(), f.org, f.year, f.class, f.student, f.admin); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Exec(ctx, `INSERT INTO modules(id,org_id,building_key,level,title,created_by,created_at,updated_at,published_at) VALUES($1,$2,'sains','SMA','Modul awal',$3,now(),now(),now())`, f.module, f.org, f.admin); err != nil {
		t.Fatal(err)
	}
	for i, id := range []uuid.UUID{f.first, f.second} {
		if _, err := p.Exec(ctx, `INSERT INTO module_steps(id,org_id,module_id,position,kind,title,body,minutes,created_at,updated_at) VALUES($1,$2,$3,$4,'materi','Langkah','Isi lama',5,now(),now())`, id, f.org, f.module, i); err != nil {
			t.Fatal(err)
		}
	}
	auth := org.NewService(org.NewStore(p), time.Now, slog.New(slog.NewTextHandler(io.Discard, nil)))
	f.svc = assignment.NewService(p, auth, time.Now)
	f.publish(t)
	return f
}
func TestIssueFrozenRecipientsAndContent(t *testing.T) {
	f := setup(t)
	ctx := t.Context()
	request := uuid.New()
	in := assignment.Input{ID: request, ClassID: f.class, ModuleID: f.module}
	a, err := f.svc.Issue(ctx, f.admin, f.org, in)
	if err != nil {
		t.Fatal(err)
	}
	if a.ID != request || a.Title != "Modul awal" {
		t.Fatalf("assignment=%+v", a)
	}
	// Author edits and student leaves class: neither changes the already issued task.
	for _, sql := range []string{`UPDATE module_steps SET body='Isi baru',archived_at=now()`, `UPDATE modules SET title='Judul baru',archived_at=now()`, `UPDATE class_enrollments SET ended_at=now()`} {
		if _, err := f.pool.Exec(ctx, sql); err != nil {
			t.Fatal(err)
		}
	}
	tasks, err := f.svc.List(ctx, f.student, f.org)
	if err != nil || len(tasks) != 1 {
		t.Fatalf("tasks=%+v %v", tasks, err)
	}
	steps, err := f.svc.Steps(ctx, f.student, f.org, a.ID)
	if err != nil || len(steps) != 2 || steps[0].Body != "Isi lama" || steps[0].Done || steps[1].Unlocked {
		t.Fatalf("steps=%+v %v", steps, err)
	}
	// Retry remains the same issuance, even after author changes and the roster empties.
	again, err := f.svc.Issue(ctx, f.admin, f.org, in)
	if err != nil || again.ID != a.ID {
		t.Fatalf("retry=%+v %v", again, err)
	}
	if err := f.svc.Complete(ctx, f.student, f.org, a.ID, steps[1].ID); !errors.Is(err, assignment.ErrLocked) {
		t.Fatalf("out of order: %v", err)
	}
	if err := f.svc.Complete(ctx, f.student, f.org, a.ID, steps[0].ID); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.Complete(ctx, f.student, f.org, a.ID, steps[1].ID); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.Complete(ctx, f.student, f.org, a.ID, steps[1].ID); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := f.pool.QueryRow(ctx, `SELECT count(*) FROM module_progress`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("catalog progress altered: %d %v", count, err)
	}
	if _, err := f.svc.Steps(ctx, f.other, f.org, a.ID); !errors.Is(err, org.ErrNotMember) {
		t.Fatalf("outsider=%v", err)
	}
	if err := f.svc.Cancel(ctx, f.admin, f.org, a.ID); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.Complete(ctx, f.student, f.org, a.ID, steps[0].ID); !errors.Is(err, assignment.ErrCancelled) {
		t.Fatalf("cancelled=%v", err)
	}
}
func TestIssuePermissionsRetriesAndConcurrency(t *testing.T) {
	f := setup(t)
	ctx := t.Context()
	in := assignment.Input{ID: uuid.New(), ClassID: f.class, ModuleID: f.module}
	if _, err := f.svc.Issue(ctx, f.student, f.org, in); !errors.Is(err, org.ErrForbidden) {
		t.Fatalf("student issues=%v", err)
	}
	var wg sync.WaitGroup
	for range 12 {
		wg.Go(func() {
			if _, err := f.svc.Issue(ctx, f.admin, f.org, in); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	tasks, err := f.svc.List(ctx, f.admin, f.org)
	if err != nil || len(tasks) != 1 {
		t.Fatalf("retry tasks=%+v %v", tasks, err)
	}
	progress, err := f.svc.Roster(ctx, f.admin, f.org, in.ID)
	if err != nil || len(progress) != 1 || progress[0].Done != 0 {
		t.Fatalf("progress=%+v %v", progress, err)
	}
	if _, err := f.svc.Roster(ctx, f.student, f.org, in.ID); !errors.Is(err, org.ErrForbidden) {
		t.Fatalf("student sees roster=%v", err)
	}
	in.ID = uuid.New()
	if _, err := f.svc.Issue(ctx, f.admin, f.org, in); err != nil {
		t.Fatal(err)
	}
	tasks, _ = f.svc.List(ctx, f.student, f.org)
	if len(tasks) != 2 {
		t.Fatalf("new task overwrote old: %d", len(tasks))
	}
	in.ModuleID = uuid.New()
	if _, err := f.svc.Issue(ctx, f.admin, f.org, in); !errors.Is(err, assignment.ErrConflict) {
		t.Fatalf("reuse request ID changed payload=%v", err)
	}
	in.ID = uuid.New()
	if _, err := f.svc.Issue(ctx, f.admin, f.org, in); !errors.Is(err, assignment.ErrNotFound) {
		t.Fatalf("missing module=%v", err)
	}
}

// Direct SQL fixtures explicitly publish after mutation, like the author API does in CS.
func (f fixture) publish(t *testing.T) {
	t.Helper()
	ctx := t.Context()
	auth := org.NewService(org.NewStore(f.pool), time.Now, slog.New(slog.NewTextHandler(io.Discard, nil)))
	svc := module.NewService(f.pool, auth, buildings{}, time.Now)
	if _, err := f.pool.Exec(ctx, "UPDATE modules SET draft_version=draft_version+1 WHERE id=$1", f.module); err != nil {
		t.Fatal(err)
	}
	m, err := query.New(f.pool).LockModule(ctx, query.LockModuleParams{OrgID: f.org, ID: f.module})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Publish(ctx, f.admin, f.org, f.module, m.DraftVersion); err != nil {
		t.Fatal(err)
	}
}
