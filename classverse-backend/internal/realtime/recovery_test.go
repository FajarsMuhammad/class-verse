package realtime

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/platform/db/dbtest"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/platform/db/query"
)

func TestStoreRecoverSessions(t *testing.T) {
	pool := dbtest.New(t)
	q := query.New(pool)
	ctx := context.Background()
	at := time.Date(2026, 3, 1, 8, 0, 0, 0, time.UTC)
	ts := pgtype.Timestamptz{Time: at, Valid: true}
	u, err := q.CreateTestUser(ctx, query.CreateTestUserParams{Email: "r@b.co", DisplayName: "Ani"})
	if err != nil {
		t.Fatal(err)
	}
	o, err := q.CreateOrg(ctx, query.CreateOrgParams{Name: "SMA", Slug: "sma-recover", CreatedBy: u.ID, CreatedAt: ts})
	if err != nil {
		t.Fatal(err)
	}
	mk := func(name string) uuid.UUID {
		r, err := q.CreateRoom(ctx, query.CreateRoomParams{OrgID: o.ID, Name: name, MapKey: "classroom",
			BuildingKey: pgtype.Text{String: "kelas-a", Valid: true}, CreatedBy: u.ID, Now: ts})
		if err != nil {
			t.Fatal(err)
		}
		return r.ID
	}
	live, archived, idle := mk("Live"), mk("Arsip"), mk("Idle")
	s := NewStore(pool)
	sess, err := s.OpenSession(ctx, live, at)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Attend(ctx, sess, u.ID, at); err != nil {
		t.Fatal(err)
	}
	if _, err := s.OpenSession(ctx, archived, at); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, "UPDATE rooms SET archived_at=$2 WHERE id=$1", archived, at); err != nil {
		t.Fatal(err)
	}
	_ = idle

	restart := at.Add(time.Minute)
	got, err := s.RecoverSessions(ctx, restart)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].RoomID != live || got[0].OrgID != o.ID || got[0].MapKey != "classroom" || got[0].BuildingKey != "kelas-a" {
		t.Fatalf("recovered = %+v", got)
	}
	var ended *time.Time
	if err := pool.QueryRow(ctx, "SELECT ended_at FROM room_sessions WHERE id=$1", sess).Scan(&ended); err != nil || ended != nil {
		t.Fatalf("session ended_at = %v %v", ended, err)
	}
	var left time.Time
	if err := pool.QueryRow(ctx, "SELECT left_at FROM session_attendances WHERE session_id=$1", sess).Scan(&left); err != nil || !left.Equal(restart) {
		t.Fatalf("attendance left_at = %v %v", left, err)
	}
	// Reopening after restart reuses the same session id.
	again, err := s.OpenSession(ctx, live, restart)
	if err != nil || again != sess {
		t.Fatalf("reopen = %v %v, want %v", again, err, sess)
	}
}

func TestHubRestoreSessionsIdleClose(t *testing.T) {
	campus := loadCampus(t)
	interior := *campus
	interior.Key = "classroom"
	clk := &fakeClock{t: time.Date(2026, 1, 1, 8, 0, 0, 0, time.UTC)}
	calls := make(chan string, 8)
	hub := NewHub(map[string]*MapData{"classroom": &interior}, failingSessions{calls}, clk.Now, slog.New(slog.NewTextHandler(io.Discard, nil)))
	roomID := uuid.New()
	if err := hub.RestoreSessions([]Admission{{RoomID: roomID, MapKey: "classroom", Capacity: 40}}); err != nil {
		t.Fatal(err)
	}
	if got := <-calls; got != "open" {
		t.Fatalf("first job = %s, want open", got)
	}
	if hub.roomCount() != 1 {
		t.Fatal("room not restored")
	}
	if err := hub.RestoreSessions([]Admission{{RoomID: uuid.New(), MapKey: "nope"}}); err == nil {
		t.Fatal("unknown map accepted")
	}
	clk.Advance(sessionIdle + time.Second)
	select {
	case got := <-calls:
		if got != "close" {
			t.Fatalf("idle job = %s, want close", got)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("restored empty room did not close its session")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := hub.Shutdown(ctx); err != nil {
		t.Fatal(err)
	}
}
