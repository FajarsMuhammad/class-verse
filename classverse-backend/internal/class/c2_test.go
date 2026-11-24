package class_test

import (
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/google/uuid"

	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/class"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/org"
)

// C2: wali kelas manages only their class; guru mapel and other teachers read; admin manages everything.
func TestClassTeachersAndRights(t *testing.T) {
	f := setup(t)
	ctx := t.Context()
	year, _ := f.svc.CreateYear(ctx, f.admin, f.school, "2026/2027")
	a, _ := f.svc.CreateClass(ctx, f.admin, f.school, year.ID, "X IPA 1")
	b, _ := f.svc.CreateClass(ctx, f.admin, f.school, year.ID, "X IPA 2")
	var v *class.ValidationError
	if err := f.svc.SetTeacher(ctx, f.teacher, f.school, a.ID, f.teacher, "homeroom", ""); !errors.Is(err, org.ErrForbidden) {
		t.Fatalf("teacher sets teacher: %v", err)
	}
	if err := f.svc.SetTeacher(ctx, f.admin, f.school, a.ID, f.ani, "homeroom", ""); !errors.Is(err, class.ErrNotStaff) {
		t.Fatalf("participant as homeroom: %v", err)
	}
	if err := f.svc.SetTeacher(ctx, f.admin, f.school, a.ID, f.stray, "homeroom", ""); !errors.Is(err, class.ErrNotStaff) {
		t.Fatalf("foreign school staff: %v", err)
	}
	if err := f.svc.SetTeacher(ctx, f.admin, f.school, a.ID, f.teacher, "boss", ""); !errors.As(err, &v) {
		t.Fatalf("bad role: %v", err)
	}
	if err := f.svc.SetTeacher(ctx, f.admin, f.school, a.ID, f.teacher, "homeroom", "Fisika"); !errors.As(err, &v) {
		t.Fatalf("homeroom with subject: %v", err)
	}
	if err := f.svc.SetTeacher(ctx, f.admin, f.school, a.ID, f.teacher, "homeroom", ""); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.SetTeacher(ctx, f.admin, f.school, b.ID, f.teacher, "subject", " Fisika "); err != nil {
		t.Fatal(err)
	}
	classes, err := f.svc.Classes(ctx, f.teacher, f.school, year.ID)
	if err != nil || len(classes) != 2 || classes[0].MyRole != "homeroom" || classes[0].Homeroom == nil || classes[0].Homeroom.ID != f.teacher ||
		classes[1].MyRole != "subject" || classes[1].SubjectTeachers != 1 {
		t.Fatalf("classes=%+v err=%v", classes, err)
	}
	teachers, err := f.svc.Teachers(ctx, f.teacher, f.school, b.ID)
	if err != nil || len(teachers) != 1 || teachers[0].Subject != "Fisika" {
		t.Fatalf("teachers=%+v err=%v", teachers, err)
	}
	// Wali kelas: add a student without a class to own class; remove own students.
	if err := f.svc.Place(ctx, f.teacher, f.school, year.ID, f.ani, &a.ID); err != nil {
		t.Fatalf("homeroom adds unplaced: %v", err)
	}
	// Guru mapel of B cannot place into B.
	if err := f.svc.Place(ctx, f.teacher, f.school, year.ID, f.budi, &b.ID); !errors.Is(err, org.ErrForbidden) {
		t.Fatalf("guru mapel places: %v", err)
	}
	if err := f.svc.Place(ctx, f.admin, f.school, year.ID, f.budi, &b.ID); err != nil {
		t.Fatal(err)
	}
	// Wali kelas of A cannot pull a student out of B, nor remove B's student.
	if err := f.svc.Place(ctx, f.teacher, f.school, year.ID, f.budi, &a.ID); !errors.Is(err, class.ErrHasClass) {
		t.Fatalf("homeroom steals from other class: %v", err)
	}
	if err := f.svc.Place(ctx, f.teacher, f.school, year.ID, f.budi, nil); !errors.Is(err, org.ErrForbidden) {
		t.Fatalf("homeroom removes other class student: %v", err)
	}
	if err := f.svc.Place(ctx, f.teacher, f.school, year.ID, f.ani, nil); err != nil {
		t.Fatalf("homeroom removes own: %v", err)
	}
	// Replacing the wali kelas ends the previous one; a removed wali loses rights.
	if err := f.svc.SetTeacher(ctx, f.admin, f.school, a.ID, f.admin, "homeroom", ""); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.Place(ctx, f.teacher, f.school, year.ID, f.ani, &a.ID); !errors.Is(err, org.ErrForbidden) {
		t.Fatalf("former homeroom places: %v", err)
	}
	if err := f.svc.RemoveTeacher(ctx, f.admin, f.school, b.ID, f.teacher); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.RemoveTeacher(ctx, f.admin, f.school, b.ID, f.teacher); !errors.Is(err, class.ErrNotFound) {
		t.Fatalf("remove twice: %v", err)
	}
	if _, err := f.svc.Teachers(ctx, f.stray, f.other, a.ID); !errors.Is(err, class.ErrNotFound) {
		t.Fatalf("cross-school teachers: %v", err)
	}
	if _, err := f.svc.Teachers(ctx, f.ani, f.school, a.ID); !errors.Is(err, org.ErrForbidden) {
		t.Fatalf("participant reads teachers: %v", err)
	}
	staff, err := f.svc.Staff(ctx, f.teacher, f.school)
	if err != nil || len(staff) != 2 {
		t.Fatalf("staff=%+v err=%v", staff, err)
	}
	// Archive ends teacher assignments.
	if err := f.svc.Place(ctx, f.admin, f.school, year.ID, f.budi, nil); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.SetTeacher(ctx, f.admin, f.school, b.ID, f.teacher, "homeroom", ""); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.ArchiveClass(ctx, f.admin, f.school, b.ID); err != nil {
		t.Fatal(err)
	}
	var active int
	if err := f.pool.QueryRow(ctx, "SELECT count(*) FROM class_teachers WHERE class_id=$1 AND ended_at IS NULL", b.ID).Scan(&active); err != nil || active != 0 {
		t.Fatalf("archive must end teachers: %d %v", active, err)
	}
}

// Concurrent wali kelas assignment to one class leaves exactly one active wali.
func TestConcurrentHomeroom(t *testing.T) {
	f := setup(t)
	ctx := t.Context()
	year, _ := f.svc.CreateYear(ctx, f.admin, f.school, "2026/2027")
	a, _ := f.svc.CreateClass(ctx, f.admin, f.school, year.ID, "X")
	var wg sync.WaitGroup
	for i := range 12 {
		who := []uuid.UUID{f.admin, f.teacher}[i%2]
		wg.Go(func() {
			if err := f.svc.SetTeacher(ctx, f.admin, f.school, a.ID, who, "homeroom", ""); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	var n int
	if err := f.pool.QueryRow(ctx, "SELECT count(*) FROM class_teachers WHERE class_id=$1 AND role='homeroom' AND ended_at IS NULL", a.ID).Scan(&n); err != nil || n != 1 {
		t.Fatalf("active homerooms=%d err=%v", n, err)
	}
}

func TestRosterPageAndMove(t *testing.T) {
	f := setup(t)
	ctx := t.Context()
	for i := range 7 {
		email := fmt.Sprintf("siswa%02d@class.test", i)
		if _, err := f.pool.Exec(ctx, `WITH u AS (INSERT INTO users (email, display_name, birth_date, password_hash) VALUES ($1, $1, '1990-01-01', 'x') RETURNING id)
			INSERT INTO memberships (org_id, user_id, role, joined_at) SELECT $2, id, 'participant', now() FROM u`, email, f.school); err != nil {
			t.Fatal(err)
		}
	}
	year, _ := f.svc.CreateYear(ctx, f.admin, f.school, "2026/2027")
	a, _ := f.svc.CreateClass(ctx, f.admin, f.school, year.ID, "X IPA 1")
	b, _ := f.svc.CreateClass(ctx, f.admin, f.school, year.ID, "X IPA 2")
	var v *class.ValidationError
	for _, bad := range []class.RosterFilter{{Limit: 0}, {Limit: 51}, {Limit: 5, Offset: -1}, {Limit: 5, Unplaced: true, ClassID: &a.ID}} {
		if _, err := f.svc.Roster(ctx, f.teacher, f.school, year.ID, bad); !errors.As(err, &v) {
			t.Fatalf("filter %+v: %v", bad, err)
		}
	}
	p1, err := f.svc.Roster(ctx, f.teacher, f.school, year.ID, class.RosterFilter{Limit: 4})
	p2, err2 := f.svc.Roster(ctx, f.teacher, f.school, year.ID, class.RosterFilter{Limit: 4, Offset: 4})
	if err != nil || err2 != nil || p1.Total != 9 || len(p1.Items) != 4 || len(p2.Items) != 4 || p1.Items[3].UserID == p2.Items[0].UserID {
		t.Fatalf("pages %+v %+v %v %v", p1, p2, err, err2)
	}
	search, _ := f.svc.Roster(ctx, f.teacher, f.school, year.ID, class.RosterFilter{Limit: 50, Query: "SISWA0"})
	if search.Total != 7 {
		t.Fatalf("search total=%d", search.Total)
	}
	ids := []uuid.UUID{f.ani, f.budi, f.teacher, uuid.New()}
	if _, err := f.svc.Move(ctx, f.teacher, f.school, year.ID, ids, &a.ID); !errors.Is(err, org.ErrForbidden) {
		t.Fatalf("teacher bulk move: %v", err)
	}
	res, err := f.svc.Move(ctx, f.admin, f.school, year.ID, ids, &a.ID)
	if err != nil || res[0].Error != "" || res[1].Error != "" || res[2].Error != "not_student" || res[3].Error != "not_student" {
		t.Fatalf("move=%+v err=%v", res, err)
	}
	in, _ := f.svc.Roster(ctx, f.teacher, f.school, year.ID, class.RosterFilter{Limit: 50, ClassID: &a.ID})
	none, _ := f.svc.Roster(ctx, f.teacher, f.school, year.ID, class.RosterFilter{Limit: 50, Unplaced: true})
	if in.Total != 2 || none.Total != 7 {
		t.Fatalf("class=%d unplaced=%d", in.Total, none.Total)
	}
	// Wali kelas bulk add: students of another class are reported, unplaced ones join.
	if err := f.svc.SetTeacher(ctx, f.admin, f.school, b.ID, f.teacher, "homeroom", ""); err != nil {
		t.Fatal(err)
	}
	res, err = f.svc.Move(ctx, f.teacher, f.school, year.ID, []uuid.UUID{f.ani, none.Items[0].UserID}, &b.ID)
	if err != nil || res[0].Error != "has_class" || res[1].Error != "" {
		t.Fatalf("homeroom move=%+v err=%v", res, err)
	}
	if _, err := f.svc.Move(ctx, f.admin, f.school, year.ID, make([]uuid.UUID, 51), &a.ID); !errors.As(err, &v) {
		t.Fatalf("too many: %v", err)
	}
	if _, err := f.svc.Move(ctx, f.admin, f.school, year.ID, ids[:1], &uuid.Nil); !errors.Is(err, class.ErrNotFound) {
		t.Fatalf("unknown class: %v", err)
	}
}

func TestCopyYearAndPromote(t *testing.T) {
	f := setup(t)
	ctx := t.Context()
	old, _ := f.svc.CreateYear(ctx, f.admin, f.school, "2026/2027")
	next, _ := f.svc.CreateYear(ctx, f.admin, f.school, "2027/2028")
	x1, _ := f.svc.CreateClass(ctx, f.admin, f.school, old.ID, "X IPA 1")
	x2, _ := f.svc.CreateClass(ctx, f.admin, f.school, old.ID, "X IPA 2")
	gone, _ := f.svc.CreateClass(ctx, f.admin, f.school, old.ID, "Lama")
	_ = f.svc.ArchiveClass(ctx, f.admin, f.school, gone.ID)
	_ = f.svc.SetTeacher(ctx, f.admin, f.school, x1.ID, f.teacher, "homeroom", "")
	_ = f.svc.SetTeacher(ctx, f.admin, f.school, x2.ID, f.teacher, "subject", "Kimia")
	_ = f.svc.Place(ctx, f.admin, f.school, old.ID, f.ani, &x1.ID)
	_ = f.svc.Place(ctx, f.admin, f.school, old.ID, f.budi, &x1.ID)
	if _, err := f.svc.CreateClass(ctx, f.admin, f.school, next.ID, "x ipa 2"); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.CopyYear(ctx, f.teacher, f.school, next.ID, old.ID, true); !errors.Is(err, org.ErrForbidden) {
		t.Fatalf("teacher copies: %v", err)
	}
	res, err := f.svc.CopyYear(ctx, f.admin, f.school, next.ID, old.ID, true)
	if err != nil || res.Created != 1 || len(res.Skipped) != 1 || res.Skipped[0] != "X IPA 2" {
		t.Fatalf("copy=%+v err=%v", res, err)
	}
	classes, _ := f.svc.Classes(ctx, f.admin, f.school, next.ID)
	var xi1 class.Class
	for _, c := range classes {
		if c.Name == "X IPA 1" {
			xi1 = c
		}
		if c.Students != 0 {
			t.Fatalf("students must not be copied: %+v", c)
		}
	}
	if xi1.Homeroom == nil || xi1.Homeroom.ID != f.teacher {
		t.Fatalf("homeroom not copied: %+v", xi1)
	}
	if _, err := f.svc.CopyYear(ctx, f.admin, f.school, next.ID, next.ID, false); err == nil {
		t.Fatal("copy into itself must fail")
	}
	// Promote X IPA 1 (old) → X IPA 1 (next). Budi already placed in next year is skipped, not moved.
	other := classes[0].ID
	if other == xi1.ID {
		other = classes[1].ID
	}
	_ = f.svc.Place(ctx, f.admin, f.school, next.ID, f.budi, &other)
	if _, err := f.svc.Promote(ctx, f.teacher, f.school, next.ID, old.ID, []class.Promotion{{From: x1.ID, To: xi1.ID}}); !errors.Is(err, org.ErrForbidden) {
		t.Fatalf("teacher promotes: %v", err)
	}
	var v *class.ValidationError
	if _, err := f.svc.Promote(ctx, f.admin, f.school, next.ID, old.ID, []class.Promotion{{From: xi1.ID, To: x1.ID}}); !errors.As(err, &v) {
		t.Fatalf("reversed years: %v", err)
	}
	pr, err := f.svc.Promote(ctx, f.admin, f.school, next.ID, old.ID, []class.Promotion{{From: x1.ID, To: xi1.ID}})
	if err != nil || pr.Moved != 1 || len(pr.Skipped) != 1 || pr.Skipped[0].ID != f.budi {
		t.Fatalf("promote=%+v err=%v", pr, err)
	}
	// Old-year history is untouched.
	page, _ := f.svc.Roster(ctx, f.admin, f.school, old.ID, class.RosterFilter{Limit: 50, ClassID: &x1.ID})
	if page.Total != 2 {
		t.Fatalf("old year roster changed: %d", page.Total)
	}
}
