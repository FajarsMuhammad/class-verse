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

func TestStoreChatAndPurge(t *testing.T) {
	pool := dbtest.New(t)
	q := query.New(pool)
	ctx := context.Background()
	at := time.Date(2026, 3, 1, 8, 0, 0, 0, time.UTC)
	ts := pgtype.Timestamptz{Time: at, Valid: true}
	u, err := q.CreateTestUser(ctx, query.CreateTestUserParams{Email: "c@b.co", DisplayName: "Ani"})
	if err != nil {
		t.Fatal(err)
	}
	o, err := q.CreateOrg(ctx, query.CreateOrgParams{Name: "SMA", Slug: "sma-chat", CreatedBy: u.ID, CreatedAt: ts})
	if err != nil {
		t.Fatal(err)
	}
	room, err := q.CreateRoom(ctx, query.CreateRoomParams{OrgID: o.ID, Name: "Biologi", MapKey: "classroom",
		BuildingKey: pgtype.Text{String: "kelas-a", Valid: true}, CreatedBy: u.ID, Now: ts})
	if err != nil {
		t.Fatal(err)
	}
	s := NewStore(pool)
	msg := ChatOutMsg{From: ChatSender{ID: u.ID}, Text: "halo", Scope: ScopeZone, Zone: "meja-1", At: at}

	if _, ok, err := s.SaveChat(ctx, room.ID, msg); err != nil || ok {
		t.Fatalf("save without session: ok=%v err=%v", ok, err)
	}
	if _, err := s.OpenSession(ctx, room.ID, at); err != nil {
		t.Fatal(err)
	}
	oldID, ok, err := s.SaveChat(ctx, room.ID, msg)
	if err != nil || !ok || oldID == 0 {
		t.Fatalf("save = %d %v %v", oldID, ok, err)
	}
	var zone, body string
	if err := pool.QueryRow(ctx, "SELECT zone_key, body FROM chat_messages WHERE id=$1", oldID).Scan(&zone, &body); err != nil || zone != "meja-1" || body != "halo" {
		t.Fatalf("row = %q %q %v", zone, body, err)
	}
	msg.At = at.Add(31 * 24 * time.Hour)
	msg.Zone = ""
	newID, _, err := s.SaveChat(ctx, room.ID, msg)
	if err != nil {
		t.Fatal(err)
	}
	// A report pointing at the old message must survive the purge.
	if _, err := pool.Exec(ctx, `INSERT INTO reports (org_id, room_id, reporter_id, target_user_id, message_id, reason)
		VALUES ($1, $2, $3, $3, $4, 'uji')`, o.ID, room.ID, u.ID, oldID); err != nil {
		t.Fatal(err)
	}
	n, err := s.PurgeChat(ctx, msg.At.Add(-30*24*time.Hour))
	if err != nil || n != 1 {
		t.Fatalf("purge removed %d, err %v", n, err)
	}
	var left int
	var reportMsg *int64
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM chat_messages").Scan(&left); err != nil || left != 1 {
		t.Fatalf("remaining = %d %v", left, err)
	}
	if err := pool.QueryRow(ctx, "SELECT message_id FROM reports").Scan(&reportMsg); err != nil || reportMsg != nil {
		t.Fatalf("report message_id = %v %v", reportMsg, err)
	}
	_ = newID
}
