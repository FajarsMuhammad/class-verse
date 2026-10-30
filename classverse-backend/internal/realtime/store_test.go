package realtime

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/platform/db/dbtest"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/platform/db/query"
)

func TestStoreAdmit(t *testing.T) {
	pool := dbtest.New(t)
	q := query.New(pool)
	ctx := context.Background()
	now := pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true}
	user := func(email string) uuid.UUID {
		u, err := q.CreateTestUser(ctx, query.CreateTestUserParams{Email: email, DisplayName: "U " + email})
		if err != nil {
			t.Fatal(err)
		}
		return u.ID
	}
	member, outsider := user("m@b.co"), user("o@b.co")
	o, err := q.CreateOrg(ctx, query.CreateOrgParams{Name: "SMA", Slug: "sma", CreatedBy: member, CreatedAt: now})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := q.CreateMembership(ctx, query.CreateMembershipParams{OrgID: o.ID, UserID: member, Role: query.OrgRoleTeacher, JoinedAt: now}); err != nil {
		t.Fatal(err)
	}
	room, err := q.CreateCampusRoom(ctx, query.CreateCampusRoomParams{OrgID: o.ID, CreatedBy: member, CreatedAt: now})
	if err != nil {
		t.Fatal(err)
	}

	s := NewStore(pool)
	adm, err := s.Admit(ctx, room, member)
	if err != nil || adm.MapKey != "campus" || adm.Capacity != 200 || adm.Role != "teacher" || adm.OrgID != o.ID {
		t.Fatalf("Admit(member) = %+v, %v", adm, err)
	}
	if _, err := s.Admit(ctx, room, outsider); !errors.Is(err, ErrNoAccess) {
		t.Errorf("Admit(outsider) = %v", err)
	}
	if _, err := s.Admit(ctx, uuid.New(), member); !errors.Is(err, ErrNoAccess) {
		t.Errorf("Admit(unknown room) = %v", err)
	}
	if _, err := pool.Exec(ctx, "UPDATE rooms SET archived_at = now() WHERE id = $1", room); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Admit(ctx, room, member); !errors.Is(err, ErrNoAccess) {
		t.Errorf("Admit(archived) = %v", err)
	}
}
