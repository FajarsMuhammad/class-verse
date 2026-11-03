package realtime

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/platform/db/dbtest"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/platform/db/query"
)

// fakeModerator records bans in memory; fail makes every write error.
type fakeModerator struct {
	mu      sync.Mutex
	banned  map[uuid.UUID]bool
	fail    bool
	actions int
}

func (f *fakeModerator) OpenSession(context.Context, uuid.UUID, time.Time) (uuid.UUID, error) {
	return uuid.New(), nil
}
func (f *fakeModerator) CloseSession(context.Context, uuid.UUID, time.Time) error { return nil }
func (f *fakeModerator) Attend(context.Context, uuid.UUID, uuid.UUID, time.Time) error {
	return nil
}
func (f *fakeModerator) Leave(context.Context, uuid.UUID, uuid.UUID, time.Time) error { return nil }
func (f *fakeModerator) SaveChat(context.Context, uuid.UUID, ChatOutMsg) (int64, bool, error) {
	return 0, false, nil
}
func (f *fakeModerator) Kick(_ context.Context, _, _, target uuid.UUID, _ time.Time) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.fail {
		return false, errors.New("db down")
	}
	f.banned[target] = true
	return true, nil
}
func (f *fakeModerator) Allow(_ context.Context, _, _, target uuid.UUID, _ time.Time) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.fail {
		return false, errors.New("db down")
	}
	was := f.banned[target]
	delete(f.banned, target)
	return was, nil
}
func (f *fakeModerator) RecordAction(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, string, time.Time) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.fail {
		return false, errors.New("db down")
	}
	f.actions++
	return true, nil
}

func (f *fakeModerator) Banned(context.Context, uuid.UUID) ([]BannedUser, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := []BannedUser{}
	for id := range f.banned {
		out = append(out, BannedUser{ID: id})
	}
	return out, nil
}

// newModRoom builds a goroutine-less classroom backed by a fake moderator.
func newModRoom(t *testing.T, mod *fakeModerator) *room {
	t.Helper()
	r, _ := newMapRoom(t, "classroom", true)
	h := NewHub(r.hub.maps, mod, r.now, slog.New(slog.NewTextHandler(io.Discard, nil)))
	t.Cleanup(func() { _ = h.Shutdown(context.Background()) })
	r.hub = h
	return r
}

// settle waits for the worker answer and handles it on the test goroutine.
func (r *room) settle(t *testing.T) {
	t.Helper()
	select {
	case cmd := <-r.inbox:
		r.handle(cmd)
	case <-time.After(2 * time.Second):
		t.Fatal("no moderation result")
	}
}

func TestKickRemovesParticipantAfterAudit(t *testing.T) {
	mod := &fakeModerator{banned: map[uuid.UUID]bool{}}
	r := newModRoom(t, mod)
	teacher, student := r.testJoin(t, "Guru"), r.testJoin(t, "Siswa")
	r.members[teacher.userID].p.Role = "teacher"
	drain(teacher)
	drain(student)

	r.handle(commandCmd{c: teacher, seq: 3, name: "kick", target: student.userID})
	r.settle(t)

	if _, ok := r.members[student.userID]; ok {
		t.Fatal("kicked participant still in room")
	}
	select {
	case <-student.done:
	default:
		t.Fatal("kicked connection not closed")
	}
	if student.code != closeForbidden {
		t.Fatalf("close code = %d, want %d", student.code, closeForbidden)
	}
	evs := drain(teacher)[TypeEvent]
	if len(evs) != 1 {
		t.Fatalf("teacher events = %d", len(evs))
	}
	var ev ModerationEvent
	if err := json.Unmarshal(evs[0], &ev); err != nil || ev.Name != "kick" || len(ev.Banned) != 1 || ev.Banned[0].ID != student.userID {
		t.Fatalf("event = %+v %v", ev, err)
	}

	r.handle(commandCmd{c: teacher, seq: 4, name: "allow", target: student.userID})
	r.settle(t)
	if mod.banned[student.userID] {
		t.Fatal("allow did not revoke ban")
	}
}

func TestKickFailsClosedWhenAuditFails(t *testing.T) {
	mod := &fakeModerator{banned: map[uuid.UUID]bool{}, fail: true}
	r := newModRoom(t, mod)
	teacher, student := r.testJoin(t, "Guru"), r.testJoin(t, "Siswa")
	r.members[teacher.userID].p.Role = "teacher"
	drain(teacher)
	r.handle(commandCmd{c: teacher, seq: 3, name: "kick", target: student.userID})
	r.settle(t)
	if _, ok := r.members[student.userID]; !ok {
		t.Fatal("participant removed although the audit write failed")
	}
	if got := errorsOf(t, teacher); len(got) != 1 || got[0] != CodeUnavailable {
		t.Fatalf("teacher errors = %v", got)
	}
}

func TestKickRejections(t *testing.T) {
	mod := &fakeModerator{banned: map[uuid.UUID]bool{}}
	r := newModRoom(t, mod)
	teacher, other, student := r.testJoin(t, "Guru"), r.testJoin(t, "Guru 2"), r.testJoin(t, "Siswa")
	r.members[teacher.userID].p.Role = "teacher"
	r.members[other.userID].p.Role = "admin"
	for _, tc := range []struct {
		name string
		by   *conn
		cmd  commandCmd
		want string
	}{
		{"participant cannot kick", student, commandCmd{name: "kick", target: teacher.userID}, CodeForbidden},
		{"staff cannot be kicked", teacher, commandCmd{name: "kick", target: other.userID}, CodeForbidden},
		{"no self kick", teacher, commandCmd{name: "kick", target: teacher.userID}, CodeBadMessage},
		{"target must be in room", teacher, commandCmd{name: "kick", target: uuid.New()}, CodeNotFoundTarget},
	} {
		t.Run(tc.name, func(t *testing.T) {
			drain(tc.by)
			tc.cmd.c = tc.by
			r.handle(tc.cmd)
			if got := errorsOf(t, tc.by); len(got) != 1 || got[0] != tc.want {
				t.Fatalf("errors = %v, want %s", got, tc.want)
			}
		})
	}
	if len(mod.banned) != 0 {
		t.Fatal("rejected command wrote a ban")
	}
}

func TestStoreKickAllowAudited(t *testing.T) {
	pool := dbtest.New(t)
	q := query.New(pool)
	ctx := context.Background()
	at := time.Date(2026, 3, 1, 8, 0, 0, 0, time.UTC)
	ts := pgtype.Timestamptz{Time: at, Valid: true}
	user := func(email string) uuid.UUID {
		u, err := q.CreateTestUser(ctx, query.CreateTestUserParams{Email: email, DisplayName: email})
		if err != nil {
			t.Fatal(err)
		}
		return u.ID
	}
	teacher, student, outsider := user("g@b.co"), user("s@b.co"), user("x@b.co")
	org := func(slug string, members map[uuid.UUID]query.OrgRole) uuid.UUID {
		o, err := q.CreateOrg(ctx, query.CreateOrgParams{Name: slug, Slug: slug, CreatedBy: teacher, CreatedAt: ts})
		if err != nil {
			t.Fatal(err)
		}
		for id, role := range members {
			if _, err := q.CreateMembership(ctx, query.CreateMembershipParams{OrgID: o.ID, UserID: id, Role: role, JoinedAt: ts}); err != nil {
				t.Fatal(err)
			}
		}
		return o.ID
	}
	o := org("sma-kick", map[uuid.UUID]query.OrgRole{teacher: query.OrgRoleTeacher, student: query.OrgRoleParticipant})
	org("sma-other", map[uuid.UUID]query.OrgRole{outsider: query.OrgRoleParticipant})
	rm, err := q.CreateRoom(ctx, query.CreateRoomParams{OrgID: o, Name: "Kelas", MapKey: "classroom",
		BuildingKey: pgtype.Text{String: "kelas-a", Valid: true}, CreatedBy: teacher, Now: ts})
	if err != nil {
		t.Fatal(err)
	}
	s := NewStore(pool)
	if ok, err := s.Kick(ctx, rm.ID, teacher, student, at); err != nil || ok {
		t.Fatalf("kick without session = %v %v", ok, err)
	}
	if _, err := s.OpenSession(ctx, rm.ID, at); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name          string
		actor, target uuid.UUID
	}{
		{"participant actor", student, teacher},
		{"staff target", teacher, teacher},
		{"other organization target", teacher, outsider},
	} {
		if ok, err := s.Kick(ctx, rm.ID, tc.actor, tc.target, at); err != nil || ok {
			t.Fatalf("%s: kick = %v %v", tc.name, ok, err)
		}
	}
	if ok, err := s.Kick(ctx, rm.ID, teacher, student, at); err != nil || !ok {
		t.Fatalf("kick = %v %v", ok, err)
	}
	adm, err := s.Admit(ctx, rm.ID, student)
	if err != nil || !adm.Banned {
		t.Fatalf("admit banned = %+v %v", adm, err)
	}
	if list, err := s.Banned(ctx, rm.ID); err != nil || len(list) != 1 || list[0].ID != student {
		t.Fatalf("banned list = %v %v", list, err)
	}
	// Survives a restart: recovery keeps the session, so the ban still applies.
	if _, err := s.RecoverSessions(ctx, at.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if adm, _ := s.Admit(ctx, rm.ID, student); !adm.Banned {
		t.Fatal("ban lost after recovery")
	}
	if ok, err := s.Allow(ctx, rm.ID, teacher, student, at); err != nil || !ok {
		t.Fatalf("allow = %v %v", ok, err)
	}
	if adm, _ := s.Admit(ctx, rm.ID, student); adm.Banned {
		t.Fatal("ban kept after allow")
	}
	var actions []string
	rows, err := pool.Query(ctx, "SELECT action::text FROM moderation_actions WHERE room_id=$1 ORDER BY created_at, action", rm.ID)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var a string
		_ = rows.Scan(&a)
		actions = append(actions, a)
	}
	rows.Close()
	if len(actions) != 2 {
		t.Fatalf("audit = %v, want kick and allow", actions)
	}
	// A new session starts clean.
	if err := s.CloseSession(ctx, rm.ID, at.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if ok, _ := s.Kick(ctx, rm.ID, teacher, student, at); ok {
		t.Fatal("kick succeeded on a closed session")
	}
}
