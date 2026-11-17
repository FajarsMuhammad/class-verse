package invite

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/org"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/platform/db/dbtest"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/platform/db/query"
)

func TestCodeHelpers(t *testing.T) {
	c, err := newCode()
	if err != nil || len(c) != codeLen {
		t.Fatalf("newCode() = %q, %v", c, err)
	}
	tests := []struct {
		in   string
		want string
		ok   bool
	}{
		{"abcd-efgh", "ABCDEFGH", true},
		{" ABCD EFGH ", "ABCDEFGH", true},
		{"ABCD-EFG0", "", false}, // 0 is not in the alphabet
		{"ABC", "", false},
	}
	for _, tt := range tests {
		got, ok := normalizeCode(tt.in)
		if ok != tt.ok || (ok && got != tt.want) {
			t.Errorf("normalizeCode(%q) = %q, %v", tt.in, got, ok)
		}
	}
	if displayCode("ABCDEFGH") != "ABCD-EFGH" {
		t.Error("displayCode")
	}
}

type clock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *clock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *clock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

type fixture struct {
	svc                            *Service
	orgSvc                         *org.Service
	clock                          *clock
	orgID                          uuid.UUID
	admin, teacher, member, joiner uuid.UUID
	noBirth, outsider              uuid.UUID
	q                              *query.Queries
}

func setup(t *testing.T) *fixture {
	t.Helper()
	pool := dbtest.New(t)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	c := &clock{t: time.Date(2026, 3, 1, 8, 0, 0, 0, time.UTC)}
	orgSvc := org.NewService(org.NewStore(pool), c.Now, log)
	q := query.New(pool)
	ctx := context.Background()

	user := func(email, birth string) uuid.UUID {
		var bd pgtype.Date
		if birth != "" {
			d, _ := time.Parse(time.DateOnly, birth)
			bd = pgtype.Date{Time: d, Valid: true}
		}
		u, err := q.CreateTestUser(ctx, query.CreateTestUserParams{Email: email, DisplayName: "User", BirthDate: bd})
		if err != nil {
			t.Fatal(err)
		}
		return u.ID
	}
	f := &fixture{
		svc: NewService(NewStore(pool), orgSvc, c.Now, log), orgSvc: orgSvc, clock: c, q: q,
		admin: user("admin@b.co", "1985-01-01"), teacher: user("teacher@b.co", "1990-01-01"),
		member: user("member@b.co", "2009-01-01"), joiner: user("joiner@b.co", "2010-05-05"),
		noBirth: user("nobirth@b.co", ""), outsider: user("out@b.co", "1990-01-01"),
	}
	o, err := orgSvc.Create(ctx, f.admin, org.CreateInput{Name: "SMA Satu", Slug: "sma-satu"})
	if err != nil {
		t.Fatal(err)
	}
	f.orgID = o.ID
	for id, role := range map[uuid.UUID]query.OrgRole{f.teacher: query.OrgRoleTeacher, f.member: query.OrgRoleParticipant} {
		if _, err := q.CreateMembership(ctx, query.CreateMembershipParams{OrgID: o.ID, UserID: id, Role: role, JoinedAt: pgTime(c.Now())}); err != nil {
			t.Fatal(err)
		}
	}
	return f
}

func TestCreatePermissions(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	tests := []struct {
		name    string
		user    uuid.UUID
		in      CreateInput
		want    error
		invalid bool
	}{
		{"admin invites teacher", f.admin, CreateInput{Role: org.RoleTeacher}, nil, false},
		{"teacher invites participant", f.teacher, CreateInput{Role: org.RoleParticipant}, nil, false},
		{"teacher cannot invite teacher", f.teacher, CreateInput{Role: org.RoleTeacher}, ErrForbidden, false},
		{"participant cannot invite", f.member, CreateInput{Role: org.RoleParticipant}, org.ErrForbidden, false},
		{"outsider sees not member", f.outsider, CreateInput{Role: org.RoleParticipant}, org.ErrNotMember, false},
		{"admin role not allowed", f.admin, CreateInput{Role: org.RoleAdmin}, nil, true},
		{"expiry too long", f.admin, CreateInput{Role: org.RoleParticipant, ExpiresInDays: ptr(8)}, nil, true},
		{"max uses zero", f.admin, CreateInput{Role: org.RoleParticipant, MaxUses: ptr(0)}, nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inv, err := f.svc.Create(ctx, tt.user, f.orgID, tt.in)
			if tt.invalid {
				var verr *ValidationError
				if !errors.As(err, &verr) {
					t.Fatalf("err = %v, want ValidationError", err)
				}
				return
			}
			if !errors.Is(err, tt.want) {
				t.Fatalf("err = %v, want %v", err, tt.want)
			}
			if err == nil && (len(inv.Code) != 9 || inv.Code[4] != '-') {
				t.Errorf("code = %q", inv.Code)
			}
		})
	}
}

func TestAcceptRules(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	inv, err := f.svc.Create(ctx, f.admin, f.orgID, CreateInput{Role: org.RoleParticipant, ExpiresInDays: ptr(1)})
	if err != nil {
		t.Fatal(err)
	}
	lower := strings.ToLower(strings.ReplaceAll(inv.Code, "-", " "))

	p, err := f.svc.Preview(ctx, lower)
	if err != nil || p.OrgName != "SMA Satu" || p.Role != org.RoleParticipant {
		t.Fatalf("Preview = %+v, %v", p, err)
	}
	if _, err := f.svc.Accept(ctx, f.noBirth, inv.Code); !errors.Is(err, ErrProfileIncomplete) {
		t.Errorf("no birth date = %v", err)
	}
	m, err := f.svc.Accept(ctx, f.joiner, lower) // 15-year-old may join via school invite
	if err != nil || m.Role != org.RoleParticipant || m.OrgID != f.orgID {
		t.Fatalf("Accept = %+v, %v", m, err)
	}
	if _, err := f.svc.Accept(ctx, f.joiner, inv.Code); !errors.Is(err, ErrAlreadyMember) {
		t.Errorf("second accept = %v", err)
	}
	if _, err := f.svc.Accept(ctx, f.joiner, "ZZZZ-ZZZZ"); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown code = %v", err)
	}

	page, err := f.svc.List(ctx, f.admin, f.orgID, 0, "", nil)
	if err != nil || len(page.Items) != 1 || page.Items[0].UsedCount != 1 {
		t.Fatalf("List = %+v, %v (already-member accept must not count)", page, err)
	}

	f.clock.Advance(24 * time.Hour)
	if _, err := f.svc.Preview(ctx, inv.Code); !errors.Is(err, ErrExpired) {
		t.Errorf("expired preview = %v", err)
	}
	if _, err := f.svc.Accept(ctx, f.outsider, inv.Code); !errors.Is(err, ErrExpired) {
		t.Errorf("expired accept = %v", err)
	}
}

func TestRevoke(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	byAdmin, _ := f.svc.Create(ctx, f.admin, f.orgID, CreateInput{Role: org.RoleParticipant})
	byTeacher, _ := f.svc.Create(ctx, f.teacher, f.orgID, CreateInput{Role: org.RoleParticipant})

	if err := f.svc.Revoke(ctx, f.teacher, f.orgID, byAdmin.ID); !errors.Is(err, ErrForbidden) {
		t.Errorf("teacher revokes admin's invite = %v", err)
	}
	if err := f.svc.Revoke(ctx, f.teacher, f.orgID, byTeacher.ID); err != nil {
		t.Errorf("teacher revokes own = %v", err)
	}
	if err := f.svc.Revoke(ctx, f.admin, f.orgID, byAdmin.ID); err != nil {
		t.Errorf("admin revokes = %v", err)
	}
	if _, err := f.svc.Accept(ctx, f.outsider, byAdmin.Code); !errors.Is(err, ErrExpired) {
		t.Errorf("revoked accept = %v", err)
	}

	if err := f.svc.Revoke(ctx, f.admin, uuid.New(), byTeacher.ID); !errors.Is(err, org.ErrNotMember) {
		t.Errorf("revoke via unknown org = %v", err)
	}
	// The admin also runs org B; an invite of org A must not be reachable through org B's path.
	orgB, err := f.orgSvc.Create(ctx, f.admin, org.CreateInput{Name: "SMA Dua", Slug: "sma-dua"})
	if err != nil {
		t.Fatal(err)
	}
	fresh, _ := f.svc.Create(ctx, f.admin, f.orgID, CreateInput{Role: org.RoleParticipant})
	if err := f.svc.Revoke(ctx, f.admin, orgB.ID, fresh.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("revoke org A invite via org B = %v, want ErrNotFound", err)
	}
	if _, err := f.svc.Preview(ctx, fresh.Code); err != nil {
		t.Errorf("invite must stay usable: %v", err)
	}
}

func TestAcceptConcurrentMaxUses(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	inv, err := f.svc.Create(ctx, f.admin, f.orgID, CreateInput{Role: org.RoleParticipant, MaxUses: ptr(1)})
	if err != nil {
		t.Fatal(err)
	}
	users := []uuid.UUID{f.joiner, f.outsider, f.noBirth}
	// noBirth would fail on profile; give it a birth date so all three race fairly.
	if _, err := f.q.UpdateProfile(ctx, query.UpdateProfileParams{
		BirthDate: pgtype.Date{Time: time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC), Valid: true},
		Now:       pgTime(f.clock.Now()), ID: f.noBirth,
	}); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	errs := make([]error, len(users))
	for i, u := range users {
		wg.Go(func() { _, errs[i] = f.svc.Accept(ctx, u, inv.Code) })
	}
	wg.Wait()
	ok, expired := 0, 0
	for _, err := range errs {
		switch {
		case err == nil:
			ok++
		case errors.Is(err, ErrExpired):
			expired++
		default:
			t.Errorf("unexpected error %v", err)
		}
	}
	if ok != 1 || expired != len(users)-1 {
		t.Errorf("ok=%d expired=%d, want 1 and %d", ok, expired, len(users)-1)
	}
}

func TestListPagination(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	for range 5 {
		if _, err := f.svc.Create(ctx, f.admin, f.orgID, CreateInput{Role: org.RoleParticipant}); err != nil {
			t.Fatal(err)
		}
		f.clock.Advance(time.Second)
	}
	seen := map[uuid.UUID]bool{}
	cursor := ""
	for pages := 0; ; pages++ {
		page, err := f.svc.List(ctx, f.admin, f.orgID, 2, cursor, nil)
		if err != nil {
			t.Fatal(err)
		}
		for _, inv := range page.Items {
			if seen[inv.ID] {
				t.Fatalf("duplicate %s", inv.ID)
			}
			seen[inv.ID] = true
		}
		if page.NextCursor == "" {
			break
		}
		cursor = page.NextCursor
		if pages > 5 {
			t.Fatal("pagination does not terminate")
		}
	}
	if len(seen) != 5 {
		t.Errorf("saw %d invites, want 5", len(seen))
	}
	if _, err := f.svc.List(ctx, f.admin, f.orgID, 2, "garbage!", nil); !errors.Is(err, ErrInvalidCursor) {
		t.Errorf("bad cursor = %v", err)
	}
	if _, err := f.svc.List(ctx, f.member, f.orgID, 2, "", nil); !errors.Is(err, org.ErrForbidden) {
		t.Errorf("participant list = %v", err)
	}
}

func ptr[T any](v T) *T { return &v }
