package realtime

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/platform/db/dbtest"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/platform/db/query"
)

func TestLearningPinPersistsOnlyForSession(t *testing.T) {
	p := dbtest.New(t)
	q := query.New(p)
	ctx := t.Context()
	at := time.Now().UTC()
	ts := pgtype.Timestamptz{Time: at, Valid: true}
	u, err := q.CreateTestUser(ctx, query.CreateTestUserParams{Email: "pin@test.invalid", DisplayName: "Guru"})
	if err != nil {
		t.Fatal(err)
	}
	o, err := q.CreateOrg(ctx, query.CreateOrgParams{Name: "Sekolah", Slug: "pin-test", CreatedBy: u.ID, CreatedAt: ts})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := q.CreateMembership(ctx, query.CreateMembershipParams{OrgID: o.ID, UserID: u.ID, Role: query.OrgRoleTeacher, JoinedAt: ts}); err != nil {
		t.Fatal(err)
	}
	room, err := q.CreateRoom(ctx, query.CreateRoomParams{OrgID: o.ID, Name: "Kelas", MapKey: "classroom", BuildingKey: pgtype.Text{String: "kelas", Valid: true}, CreatedBy: u.ID, Now: ts})
	if err != nil {
		t.Fatal(err)
	}
	m := uuid.New()
	if _, err := p.Exec(ctx, `INSERT INTO modules(id,org_id,building_key,level,title,created_by,created_at,updated_at,published_at) VALUES($1,$2,'sains','SMA','Belajar',$3,now(),now(),now())`, m, o.ID, u.ID); err != nil {
		t.Fatal(err)
	}
	rev, err := q.InsertModuleRevision(ctx, query.InsertModuleRevisionParams{OrgID: o.ID, ID: m, PublishedAt: ts})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := q.PublishModulePointer(ctx, query.PublishModulePointerParams{OrgID: o.ID, ID: m, PublishedRevisionID: uuid.NullUUID{UUID: rev.ID, Valid: true}, PublishedAt: ts}); err != nil {
		t.Fatal(err)
	}
	s := NewStore(p)
	if _, err := s.OpenSession(ctx, room.ID, at); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SaveLearningPin(ctx, room.ID, u.ID, m, uuid.New()); err == nil {
		t.Fatal("wrong step accepted")
	}
	if _, err := s.SaveLearningPin(ctx, room.ID, u.ID, m, uuid.Nil); err != nil {
		t.Fatal(err)
	}
	restored, err := NewStore(p).Admit(ctx, room.ID, u.ID)
	if err != nil || restored.Pin == nil || restored.Pin.ModuleID != m {
		t.Fatalf("restart restore=%+v %v", restored.Pin, err)
	}
	if err := s.CloseSession(ctx, room.ID, at.Add(5*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if _, err := s.OpenSession(ctx, room.ID, at.Add(6*time.Minute)); err != nil {
		t.Fatal(err)
	}
	restored, err = s.Admit(ctx, room.ID, u.ID)
	if err != nil || restored.Pin != nil {
		t.Fatalf("new session retained pin=%+v %v", restored.Pin, err)
	}
}
