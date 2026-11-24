package invite

import (
	"errors"
	"sync"
	"testing"

	"github.com/google/uuid"

	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/org"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/platform/db/query"
)

// Different codes for the same student's year must serialize with normal class placements too.
func TestConcurrentClassInvites(t *testing.T) {
	f := setup(t)
	ctx := t.Context()
	y, err := f.q.CreateAcademicYear(ctx, query.CreateAcademicYearParams{ID: uuid.New(), OrgID: f.orgID, Label: "2026/2027", CreatedAt: pgTime(f.clock.Now())})
	if err != nil {
		t.Fatal(err)
	}
	var invites []Invite
	for _, name := range []string{"A", "B"} {
		c, err := f.q.CreateClass(ctx, query.CreateClassParams{ID: uuid.New(), OrgID: f.orgID, AcademicYearID: y.ID, Name: name, CreatedBy: f.admin, CreatedAt: pgTime(f.clock.Now())})
		if err != nil {
			t.Fatal(err)
		}
		inv, err := f.svc.Create(ctx, f.admin, f.orgID, CreateInput{Role: "participant", ClassID: &c.ID})
		if err != nil {
			t.Fatal(err)
		}
		invites = append(invites, inv)
	}
	results := make(chan string, 2)
	var wg sync.WaitGroup
	for _, inv := range invites {
		wg.Go(func() {
			m, err := f.svc.Accept(ctx, f.joiner, inv.Code)
			if err != nil {
				t.Error(err)
				return
			}
			results <- m.PlacementStatus
		})
	}
	wg.Wait()
	close(results)
	counts := map[string]int{}
	for s := range results {
		counts[s]++
	}
	if counts["placed"] != 1 || counts["has_class"] != 1 {
		t.Fatalf("outcomes %v", counts)
	}
}

func TestClassInvite(t *testing.T) {
	f := setup(t)
	ctx := t.Context()
	y, err := f.q.CreateAcademicYear(ctx, query.CreateAcademicYearParams{ID: uuid.New(), OrgID: f.orgID, Label: "2026/2027", CreatedAt: pgTime(f.clock.Now())})
	if err != nil {
		t.Fatal(err)
	}
	c, err := f.q.CreateClass(ctx, query.CreateClassParams{ID: uuid.New(), OrgID: f.orgID, AcademicYearID: y.ID, Name: "X IPA", CreatedBy: f.admin, CreatedAt: pgTime(f.clock.Now())})
	if err != nil {
		t.Fatal(err)
	}
	in := CreateInput{Role: "participant", ClassID: &c.ID}
	if _, err := f.svc.Create(ctx, f.teacher, f.orgID, in); !errors.Is(err, ErrForbidden) {
		t.Fatalf("other teacher: %v", err)
	}
	if err := f.q.CreateClassTeacher(ctx, query.CreateClassTeacherParams{ID: uuid.New(), OrgID: f.orgID, ClassID: c.ID, UserID: f.teacher, Role: query.ClassTeacherRoleHomeroom, StartedAt: pgTime(f.clock.Now()), CreatedBy: f.admin}); err != nil {
		t.Fatal(err)
	}
	bad := in
	bad.Role = "teacher"
	if _, err := f.svc.Create(ctx, f.admin, f.orgID, bad); err == nil {
		t.Fatal("class invite must be participant only")
	}
	inv, err := f.svc.Create(ctx, f.teacher, f.orgID, in)
	if err != nil {
		t.Fatal(err)
	}
	preview, err := f.svc.Preview(ctx, inv.Code)
	if err != nil || preview.ClassName != "X IPA" || preview.YearLabel != "2026/2027" {
		t.Fatalf("preview %+v %v", preview, err)
	}
	m, err := f.svc.Accept(ctx, f.joiner, inv.Code)
	if err != nil || m.PlacementStatus != "placed" {
		t.Fatalf("accept %+v %v", m, err)
	}
	e, err := f.q.ActiveEnrollment(ctx, query.ActiveEnrollmentParams{OrgID: f.orgID, AcademicYearID: y.ID, UserID: f.joiner})
	if err != nil || e.ClassID != c.ID {
		t.Fatalf("placement %+v %v", e, err)
	}
	// Existing participants may accept class invites without changing their membership or consuming another use on repeat.
	m, err = f.svc.Accept(ctx, f.member, inv.Code)
	if err != nil || m.PlacementStatus != "placed" {
		t.Fatalf("existing participant %+v %v", m, err)
	}
	m, err = f.svc.Accept(ctx, f.member, inv.Code)
	if err != nil || m.PlacementStatus != "placed" {
		t.Fatalf("repeat %+v %v", m, err)
	}
	c2, err := f.q.CreateClass(ctx, query.CreateClassParams{ID: uuid.New(), OrgID: f.orgID, AcademicYearID: y.ID, Name: "X IPS", CreatedBy: f.admin, CreatedAt: pgTime(f.clock.Now())})
	if err != nil {
		t.Fatal(err)
	}
	inv2, err := f.svc.Create(ctx, f.admin, f.orgID, CreateInput{Role: "participant", ClassID: &c2.ID})
	if err != nil {
		t.Fatal(err)
	}
	m, err = f.svc.Accept(ctx, f.joiner, inv2.Code)
	if err != nil || m.PlacementStatus != "has_class" {
		t.Fatalf("conflict %+v %v", m, err)
	}
	e, _ = f.q.ActiveEnrollment(ctx, query.ActiveEnrollmentParams{OrgID: f.orgID, AcademicYearID: y.ID, UserID: f.joiner})
	if e.ClassID != c.ID {
		t.Fatal("conflict silently moved student")
	}
	page, err := f.svc.List(ctx, f.admin, f.orgID, 20, "", &c2.ID)
	if err != nil || len(page.Items) != 1 || page.Items[0].ClassID == nil || *page.Items[0].ClassID != c2.ID {
		t.Fatalf("filtered invites %+v %v", page, err)
	}
	// Existing staff never become participants through a class invite.
	staff, err := f.svc.Accept(ctx, f.admin, inv.Code)
	if err != nil || staff.PlacementStatus != "not_student" || staff.Role != "admin" {
		t.Fatalf("staff accept %+v %v", staff, err)
	}
	// Cross-school class IDs cannot be invited, even by a valid admin.
	foreign, err := f.orgSvc.Create(ctx, f.outsider, org.CreateInput{Name: "Other school", Slug: "other-school"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.Create(ctx, f.outsider, foreign.ID, in); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross school: %v", err)
	}
	// Removing wali authority makes their old class invite fail safe, but membership still joins.
	_, err = f.q.EndClassTeacher(ctx, query.EndClassTeacherParams{ClassID: c.ID, UserID: f.teacher, EndedAt: pgTime(f.clock.Now())})
	if err != nil {
		t.Fatal(err)
	}
	m, err = f.svc.Accept(ctx, f.outsider, inv.Code)
	if err != nil || m.OrgID != f.orgID || m.PlacementStatus != "class_unavailable" {
		t.Fatalf("inactive wali %+v %v", m, err)
	}
	if err := f.q.ArchiveClass(ctx, query.ArchiveClassParams{OrgID: f.orgID, ID: c2.ID, ArchivedAt: pgTime(f.clock.Now())}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.Create(ctx, f.admin, f.orgID, CreateInput{Role: "participant", ClassID: &c2.ID}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("archived class create: %v", err)
	}
	m, err = f.svc.Accept(ctx, f.member, inv2.Code)
	if err != nil || m.PlacementStatus != "class_unavailable" {
		t.Fatalf("archived class accept %+v %v", m, err)
	}
}
