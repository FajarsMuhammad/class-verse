package class_test

import (
	"errors"
	"io"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/class"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/org"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/platform/db/dbtest"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/platform/db/query"
)

type fixture struct {
	pool                                  *pgxpool.Pool
	svc                                   *class.Service
	school, other                         uuid.UUID
	admin, teacher, ani, budi, stray, out uuid.UUID
}

func setup(t *testing.T) fixture {
	t.Helper()
	pool := dbtest.New(t)
	ctx := t.Context()
	base := time.Date(2026, 7, 1, 8, 0, 0, 0, time.UTC)
	var ticks atomic.Int64
	clock := func() time.Time { return base.Add(time.Duration(ticks.Add(1)) * time.Second) }
	q := query.New(pool)
	user := func(email string) uuid.UUID {
		u, err := q.CreateTestUser(ctx, query.CreateTestUserParams{Email: email, DisplayName: email, BirthDate: pgtype.Date{Time: time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC), Valid: true}})
		if err != nil {
			t.Fatal(err)
		}
		return u.ID
	}
	f := fixture{pool: pool, admin: user("admin@class.test"), teacher: user("teacher@class.test"), ani: user("ani@class.test"),
		budi: user("budi@class.test"), stray: user("stray@class.test"), out: user("out@class.test")}
	orgs := org.NewService(org.NewStore(pool), clock, slog.New(slog.NewTextHandler(io.Discard, nil)))
	school, err := orgs.Create(ctx, f.admin, org.CreateInput{Name: "Class School", Slug: "class-school"})
	if err != nil {
		t.Fatal(err)
	}
	other, err := orgs.Create(ctx, f.stray, org.CreateInput{Name: "Other", Slug: "class-other"})
	if err != nil {
		t.Fatal(err)
	}
	join := func(orgID, userID uuid.UUID, role string) {
		if _, err := pool.Exec(ctx, "INSERT INTO memberships (org_id, user_id, role, joined_at) VALUES ($1, $2, $3, now())", orgID, userID, role); err != nil {
			t.Fatal(err)
		}
	}
	join(school.ID, f.teacher, "teacher")
	join(school.ID, f.ani, "participant")
	join(school.ID, f.budi, "participant")
	join(other.ID, f.ani, "participant") // Ani also studies elsewhere.
	f.svc, f.school, f.other = class.NewService(pool, orgs, clock), school.ID, other.ID
	return f
}

func TestYears(t *testing.T) {
	f := setup(t)
	ctx := t.Context()
	if _, err := f.svc.CreateYear(ctx, f.teacher, f.school, "2026/2027"); !errors.Is(err, org.ErrForbidden) {
		t.Fatalf("teacher creates year: %v", err)
	}
	var v *class.ValidationError
	for _, bad := range []string{"2026", "2026/2028", "26/27", " "} {
		if _, err := f.svc.CreateYear(ctx, f.admin, f.school, bad); !errors.As(err, &v) {
			t.Fatalf("label %q: %v", bad, err)
		}
	}
	y1, err := f.svc.CreateYear(ctx, f.admin, f.school, "2026/2027")
	if err != nil || !y1.Active {
		t.Fatalf("first year must become active: %+v %v", y1, err)
	}
	if _, err := f.svc.CreateYear(ctx, f.admin, f.school, "2026/2027"); !errors.As(err, &v) {
		t.Fatalf("duplicate label: %v", err)
	}
	y2, err := f.svc.CreateYear(ctx, f.admin, f.school, "2027/2028")
	if err != nil || y2.Active {
		t.Fatalf("second year stays inactive: %+v %v", y2, err)
	}
	if err := f.svc.Activate(ctx, f.teacher, f.school, y2.ID); !errors.Is(err, org.ErrForbidden) {
		t.Fatalf("teacher activates: %v", err)
	}
	if err := f.svc.Activate(ctx, f.admin, f.school, y2.ID); err != nil {
		t.Fatal(err)
	}
	years, err := f.svc.Years(ctx, f.teacher, f.school)
	if err != nil || len(years) != 2 || years[0].Label != "2027/2028" || !years[0].Active || years[1].Active {
		t.Fatalf("years=%+v err=%v", years, err)
	}
	if err := f.svc.Activate(ctx, f.stray, f.other, y1.ID); !errors.Is(err, class.ErrNotFound) {
		t.Fatalf("cross-org activate: %v", err)
	}
	if _, err := f.svc.Years(ctx, f.out, f.school); !errors.Is(err, org.ErrNotMember) {
		t.Fatalf("outsider years: %v", err)
	}
	// Only staff see the roster; participants cannot read or change it.
	if _, err := f.svc.Years(ctx, f.ani, f.school); !errors.Is(err, org.ErrForbidden) {
		t.Fatalf("participant years: %v", err)
	}
}

func TestClassesAndPlacement(t *testing.T) {
	f := setup(t)
	ctx := t.Context()
	year, _ := f.svc.CreateYear(ctx, f.admin, f.school, "2026/2027")
	next, _ := f.svc.CreateYear(ctx, f.admin, f.school, "2027/2028")
	a, err := f.svc.CreateClass(ctx, f.teacher, f.school, year.ID, "XI IPA 1")
	if err != nil {
		t.Fatal(err)
	}
	b, _ := f.svc.CreateClass(ctx, f.teacher, f.school, year.ID, "XI IPA 2")
	var v *class.ValidationError
	if _, err := f.svc.CreateClass(ctx, f.teacher, f.school, year.ID, " xi ipa 1 "); !errors.As(err, &v) {
		t.Fatalf("duplicate class name (case-insensitive): %v", err)
	}
	if _, err := f.svc.CreateClass(ctx, f.ani, f.school, year.ID, "X"); !errors.Is(err, org.ErrForbidden) {
		t.Fatalf("participant creates class: %v", err)
	}
	if _, err := f.svc.CreateClass(ctx, f.stray, f.other, year.ID, "X"); !errors.Is(err, class.ErrNotFound) {
		t.Fatalf("class in another school's year: %v", err)
	}
	// Only participants can be placed; teachers and outsiders cannot.
	if err := f.svc.Place(ctx, f.teacher, f.school, year.ID, f.teacher, &a.ID); !errors.Is(err, class.ErrNotStudent) {
		t.Fatalf("place teacher: %v", err)
	}
	if err := f.svc.Place(ctx, f.teacher, f.school, year.ID, f.out, &a.ID); !errors.Is(err, class.ErrNotStudent) {
		t.Fatalf("place outsider: %v", err)
	}
	if err := f.svc.Place(ctx, f.ani, f.school, year.ID, f.ani, &a.ID); !errors.Is(err, org.ErrForbidden) {
		t.Fatalf("student places self: %v", err)
	}
	if err := f.svc.Place(ctx, f.teacher, f.school, next.ID, f.ani, &a.ID); !errors.Is(err, class.ErrNotFound) {
		t.Fatalf("class from another year: %v", err)
	}
	if err := f.svc.Place(ctx, f.teacher, f.school, year.ID, f.ani, &a.ID); err != nil {
		t.Fatal(err)
	}
	// Final DB guards remain correct even when service locks/validation are bypassed.
	for _, tc := range []struct {
		sql  string
		code string
	}{
		{`INSERT INTO class_enrollments (id,org_id,academic_year_id,class_id,user_id,started_at,created_by)
		 SELECT gen_random_uuid(),org_id,academic_year_id,class_id,user_id,started_at,created_by FROM class_enrollments`, "23505"},
		{`INSERT INTO academic_years (id,org_id,label,active,created_at)
		 SELECT gen_random_uuid(),org_id,'2028/2029',true,created_at FROM academic_years WHERE active`, "23505"},
		{`INSERT INTO class_enrollments (id,org_id,academic_year_id,class_id,user_id,started_at,created_by)
		 SELECT gen_random_uuid(),e.org_id,y.id,e.class_id,e.user_id,e.started_at,e.created_by
		 FROM class_enrollments e JOIN academic_years y ON y.org_id=e.org_id AND y.id<>e.academic_year_id`, "23503"},
	} {
		_, err := f.pool.Exec(ctx, tc.sql)
		var pg *pgconn.PgError
		if !errors.As(err, &pg) || pg.Code != tc.code {
			t.Fatalf("DB constraint: want %s, got %v", tc.code, err)
		}
	}
	if renamed, err := f.svc.RenameClass(ctx, f.teacher, f.school, a.ID, a.Name); err != nil || renamed.Students != 1 {
		t.Fatalf("rename must return real student count: %+v %v", renamed, err)
	}
	if err := f.svc.Place(ctx, f.teacher, f.school, year.ID, f.ani, &a.ID); err != nil {
		t.Fatalf("same class again is a no-op: %v", err)
	}
	// Move: one active row, history kept.
	if err := f.svc.Place(ctx, f.teacher, f.school, year.ID, f.ani, &b.ID); err != nil {
		t.Fatal(err)
	}
	roster, err := f.svc.Roster(ctx, f.teacher, f.school, year.ID)
	if err != nil || len(roster) != 2 {
		t.Fatalf("roster=%+v err=%v", roster, err)
	}
	for _, r := range roster {
		switch r.UserID {
		case f.ani:
			if r.ClassID == nil || *r.ClassID != b.ID {
				t.Fatalf("ani class=%v", r.ClassID)
			}
		case f.budi:
			if r.ClassID != nil {
				t.Fatalf("budi must be unplaced: %v", r.ClassID)
			}
		default:
			t.Fatalf("roster includes non-participant %v", r.UserID)
		}
	}
	hist, err := f.svc.History(ctx, f.teacher, f.school, f.ani)
	if err != nil || len(hist) != 2 || hist[0].ClassName != "XI IPA 2" || hist[0].EndedAt != nil || hist[1].ClassName != "XI IPA 1" || hist[1].EndedAt == nil {
		t.Fatalf("history=%+v err=%v", hist, err)
	}
	// A different year is independent; a non-empty class cannot be archived.
	c, _ := f.svc.CreateClass(ctx, f.teacher, f.school, next.ID, "XII IPA 1")
	if err := f.svc.Place(ctx, f.teacher, f.school, next.ID, f.ani, &c.ID); err != nil {
		t.Fatalf("second year placement: %v", err)
	}
	otherYear, err := f.svc.CreateYear(ctx, f.stray, f.other, "2026/2027")
	if err != nil {
		t.Fatal(err)
	}
	otherClass, err := f.svc.CreateClass(ctx, f.stray, f.other, otherYear.ID, "Kelas lain")
	if err != nil {
		t.Fatal(err)
	}
	if err := f.svc.Place(ctx, f.stray, f.other, otherYear.ID, f.ani, &otherClass.ID); err != nil {
		t.Fatalf("same student/year in another school: %v", err)
	}
	if err := f.svc.Place(ctx, f.teacher, f.school, year.ID, f.ani, &otherClass.ID); !errors.Is(err, class.ErrNotFound) {
		t.Fatalf("cross-school class: %v", err)
	}
	if err := f.svc.ArchiveClass(ctx, f.teacher, f.school, b.ID); !errors.Is(err, class.ErrClassNotEmpty) {
		t.Fatalf("archive non-empty: %v", err)
	}
	if err := f.svc.Place(ctx, f.teacher, f.school, year.ID, f.ani, nil); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.ArchiveClass(ctx, f.teacher, f.school, b.ID); err != nil {
		t.Fatal(err)
	}
	classes, _ := f.svc.Classes(ctx, f.teacher, f.school, year.ID)
	if len(classes) != 1 || classes[0].ID != a.ID || classes[0].Students != 0 {
		t.Fatalf("classes after archive=%+v", classes)
	}
	if _, err := f.svc.CreateClass(ctx, f.teacher, f.school, year.ID, "XI IPA 2"); err != nil {
		t.Fatalf("name of archived class is free again: %v", err)
	}
	if err := f.svc.Place(ctx, f.teacher, f.school, year.ID, f.ani, &b.ID); !errors.Is(err, class.ErrNotFound) {
		t.Fatalf("place into archived class: %v", err)
	}
	if _, err := f.svc.RenameClass(ctx, f.teacher, f.school, a.ID, "XI IPA Satu"); err != nil {
		t.Fatal(err)
	}
}

// Concurrent placements of one student must end with exactly one active class.
func TestConcurrentPlacement(t *testing.T) {
	f := setup(t)
	ctx := t.Context()
	year, _ := f.svc.CreateYear(ctx, f.admin, f.school, "2026/2027")
	ids := make([]uuid.UUID, 6)
	for i := range ids {
		c, err := f.svc.CreateClass(ctx, f.teacher, f.school, year.ID, "Kelas "+string(rune('A'+i)))
		if err != nil {
			t.Fatal(err)
		}
		ids[i] = c.ID
	}
	var wg sync.WaitGroup
	for i := range 24 {
		wg.Go(func() {
			id := ids[i%len(ids)]
			if err := f.svc.Place(ctx, f.teacher, f.school, year.ID, f.ani, &id); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	classes, _ := f.svc.Classes(ctx, f.teacher, f.school, year.ID)
	total := 0
	for _, c := range classes {
		total += int(c.Students)
	}
	if total != 1 {
		t.Fatalf("active placements=%d, want exactly 1", total)
	}
	// Two first-year creations at once: exactly one active year.
	g := setup(t)
	var created atomic.Int32
	for _, label := range []string{"2030/2031", "2031/2032", "2032/2033", "2033/2034"} {
		wg.Go(func() {
			if _, err := g.svc.CreateYear(ctx, g.admin, g.school, label); err == nil {
				created.Add(1)
			}
		})
	}
	wg.Wait()
	years, _ := g.svc.Years(ctx, g.admin, g.school)
	active := 0
	for _, y := range years {
		if y.Active {
			active++
		}
	}
	if created.Load() != 4 || active != 1 {
		t.Fatalf("created=%d active=%d", created.Load(), active)
	}
}
