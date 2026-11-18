package gradebook_test

import (
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/gradebook"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/module"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/org"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/platform/db/dbtest"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/platform/db/query"
)

type buildings struct{}

func (buildings) Building(key string) (bool, bool) { return false, key == "sains" || key == "angka" }
func TestReportsOwnershipMetricsAndFilters(t *testing.T) {
	p := dbtest.New(t)
	q := query.New(p)
	ctx := t.Context()
	now := pgtype.Timestamptz{Time: time.Now(), Valid: true}
	user := func(name string) uuid.UUID {
		u, err := q.CreateTestUser(ctx, query.CreateTestUserParams{Email: uuid.NewString() + "@test.invalid", DisplayName: name})
		if err != nil {
			t.Fatal(err)
		}
		return u.ID
	}
	admin, ani, budi, outsider := user("Guru"), user("=Ani"), user("Budi"), user("Luar")
	school, err := q.CreateOrg(ctx, query.CreateOrgParams{Name: "Sekolah", Slug: "gradebook", CreatedBy: admin, CreatedAt: now})
	if err != nil {
		t.Fatal(err)
	}
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := p.Exec(ctx, sql, args...); err != nil {
			t.Fatal(err)
		}
	}
	exec(`INSERT INTO memberships(org_id,user_id,role) VALUES($1,$2,'admin'),($1,$3,'participant'),($1,$4,'participant')`, school.ID, admin, ani, budi)
	m, step, quizID, year, cls := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	exec(`INSERT INTO academic_years(id,org_id,label,active,created_at) VALUES($1,$2,'2026/2027',true,now())`, year, school.ID)
	exec(`INSERT INTO classes(id,org_id,academic_year_id,name,created_by,created_at) VALUES($1,$2,$3,'XI A',$4,now())`, cls, school.ID, year, admin)
	exec(`INSERT INTO class_enrollments(id,org_id,academic_year_id,class_id,user_id,started_at,created_by) VALUES($1,$2,$3,$4,$5,now(),$6)`, uuid.New(), school.ID, year, cls, ani, admin)
	exec(`INSERT INTO modules(id,org_id,building_key,level,title,created_by,created_at,updated_at,published_at) VALUES($1,$2,'sains','SMA','Modul',$3,now(),now(),now())`, m, school.ID, admin)
	exec(`INSERT INTO module_steps(id,org_id,module_id,position,kind,title,body,minutes,created_at,updated_at) VALUES($1,$2,$3,0,'materi','Baca','Isi',3,now(),now())`, step, school.ID, m)
	exec(`INSERT INTO module_progress(org_id,user_id,step_id,completed_at) VALUES($1,$2,$3,now())`, school.ID, ani, step)
	exec(`INSERT INTO quizzes(id,org_id,building_key,level,title,pass_score,created_by,created_at,updated_at,published_at) VALUES($1,$2,'sains','SMA','Kuis',60,$3,now(),now(),now())`, quizID, school.ID, admin)
	for i, score := range []int{100, 50} {
		exec(`INSERT INTO quiz_attempts(id,org_id,quiz_id,user_id,questions,answers,pass_score,started_at,finished_at,correct,score) VALUES($1,$2,$3,$4,'[]','{}',60,now(),now()+$5*interval '1 second',0,$6)`, uuid.New(), school.ID, quizID, ani, i, score)
	}
	auth := org.NewService(org.NewStore(p), time.Now, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if _, err := module.NewService(p, auth, buildings{}, time.Now).Publish(ctx, admin, school.ID, m, 1); err != nil {
		t.Fatal(err)
	}
	s := gradebook.NewService(p, auth, buildings{})
	report, err := s.Student(ctx, ani, school.ID, ani, gradebook.Filter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Modules) != 1 || report.Modules[0].Done != 1 || len(report.Quizzes) != 1 || report.Quizzes[0].Best == nil || *report.Quizzes[0].Best != 100 || *report.Quizzes[0].Last != 50 || report.AverageBest == nil || *report.AverageBest != 100 {
		t.Fatalf("report=%+v", report)
	}
	blank, err := s.Student(ctx, budi, school.ID, budi, gradebook.Filter{})
	if err != nil || blank.Quizzes[0].Best != nil || blank.AverageBest != nil {
		t.Fatalf("unattempted must be null: %+v %v", blank, err)
	}
	if _, err := s.Student(ctx, budi, school.ID, ani, gradebook.Filter{}); !errors.Is(err, org.ErrForbidden) {
		t.Fatalf("other student's report=%v", err)
	}
	if _, err := s.Student(ctx, outsider, school.ID, ani, gradebook.Filter{}); !errors.Is(err, org.ErrNotMember) {
		t.Fatalf("outsider=%v", err)
	}
	if _, err := s.Roster(ctx, ani, school.ID, gradebook.Filter{}, 0, 20); !errors.Is(err, org.ErrForbidden) {
		t.Fatalf("participant roster=%v", err)
	}
	page, err := s.Roster(ctx, admin, school.ID, gradebook.Filter{YearID: &year, ClassID: &cls}, 0, 20)
	if err != nil || len(page.Items) != 1 || page.Items[0].ModulesDone != 1 || page.Items[0].QuizzesAttempted != 1 {
		t.Fatalf("roster=%+v %v", page, err)
	}
	// Current roster filter is not historical attribution; module/quiz activity remains after transfer out.
	exec(`UPDATE class_enrollments SET ended_at=now()`)
	page, err = s.Roster(ctx, admin, school.ID, gradebook.Filter{YearID: &year, ClassID: &cls}, 0, 20)
	if err != nil || len(page.Items) != 0 {
		t.Fatalf("current roster=%+v %v", page, err)
	}
	report, err = s.Student(ctx, admin, school.ID, ani, gradebook.Filter{})
	if err != nil || *report.Quizzes[0].Best != 100 {
		t.Fatalf("history changed: %+v %v", report, err)
	}
	exec(`UPDATE modules SET archived_at=now(); UPDATE quizzes SET archived_at=now()`)
	report, err = s.Student(ctx, ani, school.ID, ani, gradebook.Filter{})
	if err != nil || !report.Modules[0].Archived || !report.Quizzes[0].Archived {
		t.Fatalf("archive history=%+v %v", report, err)
	}
	fake := uuid.New()
	if _, err := s.Student(ctx, admin, school.ID, ani, gradebook.Filter{ModuleID: &fake}); !errors.Is(err, gradebook.ErrNotFound) {
		t.Fatalf("foreign module=%v", err)
	}
	if _, err := s.Roster(ctx, admin, school.ID, gradebook.Filter{ClassID: &cls}, 0, 20); !errors.Is(err, gradebook.ErrInvalid) {
		t.Fatalf("class without year=%v", err)
	}
	page, err = s.Roster(ctx, admin, school.ID, gradebook.Filter{}, 1, 1)
	if err != nil || len(page.Items) != 1 || page.Items[0].Name != "Budi" || page.Total != 2 {
		t.Fatalf("page=%+v %v", page, err)
	}
}
