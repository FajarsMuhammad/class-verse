package realtime

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/platform/db/dbtest"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/platform/db/query"
)

func lastState(t *testing.T, c *conn) RoomStateMsg {
	t.Helper()
	raws := drain(c)[TypeRoomState]
	if len(raws) == 0 {
		t.Fatal("no room_state")
	}
	var s RoomStateMsg
	if err := json.Unmarshal(raws[len(raws)-1], &s); err != nil {
		t.Fatal(err)
	}
	return s
}

func TestMuteBlocksEveryScopeUntilUnmute(t *testing.T) {
	mod := &fakeModerator{banned: map[uuid.UUID]bool{}}
	r := newModRoom(t, mod)
	teacher, student := r.testJoin(t, "Guru"), r.testJoin(t, "Siswa")
	r.members[teacher.userID].p.Role = "teacher"
	drain(teacher)
	drain(student)

	r.handle(commandCmd{c: teacher, seq: 1, name: "mute", target: student.userID})
	if r.muted[student.userID] {
		t.Fatal("muted before the audit write committed")
	}
	r.settle(t)
	if s := lastState(t, student); !s.Muted {
		t.Fatalf("student state = %+v", s)
	}
	for _, scope := range []string{ScopeNear, ScopeRoom, ScopeZone, ScopeBroadcast} {
		r.say(student, scope, "halo")
		if got := errorsOf(t, student); len(got) != 1 || (got[0] != CodeMuted && scope != ScopeBroadcast) || (scope == ScopeBroadcast && got[0] != CodeForbidden) {
			t.Fatalf("%s errors = %v", scope, got)
		}
	}
	if len(chats(t, teacher)) != 0 {
		t.Fatal("muted chat delivered")
	}
	r.handle(commandCmd{c: teacher, seq: 2, name: "unmute", target: student.userID})
	r.settle(t)
	if s := lastState(t, student); s.Muted {
		t.Fatal("still muted after unmute")
	}
	r.say(student, ScopeRoom, "kembali")
	r.settle(t) // stored on the worker, then delivered through the inbox
	if len(chats(t, teacher)) != 1 {
		t.Fatal("unmuted chat not delivered")
	}
	if mod.actions != 2 {
		t.Fatalf("audited actions = %d, want 2", mod.actions)
	}
}

func TestMuteRejectionsAndFailure(t *testing.T) {
	mod := &fakeModerator{banned: map[uuid.UUID]bool{}, fail: true}
	r := newModRoom(t, mod)
	teacher, admin, student := r.testJoin(t, "Guru"), r.testJoin(t, "Admin"), r.testJoin(t, "Siswa")
	r.members[teacher.userID].p.Role = "teacher"
	r.members[admin.userID].p.Role = "admin"
	drain(teacher)
	r.handle(commandCmd{c: teacher, name: "mute", target: admin.userID})
	if got := errorsOf(t, teacher); len(got) != 1 || got[0] != CodeForbidden {
		t.Fatalf("mute staff errors = %v", got)
	}
	r.handle(commandCmd{c: teacher, name: "unmute", target: student.userID})
	if got := errorsOf(t, teacher); len(got) != 1 || got[0] != CodeBadMessage {
		t.Fatalf("unmute not muted errors = %v", got)
	}
	r.handle(commandCmd{c: teacher, name: "mute", target: student.userID})
	r.settle(t)
	if r.muted[student.userID] {
		t.Fatal("muted although audit failed")
	}
	if got := errorsOf(t, teacher); len(got) != 1 || got[0] != CodeUnavailable {
		t.Fatalf("failure errors = %v", got)
	}
}

func TestLockRejectsNewParticipantsOnly(t *testing.T) {
	mod := &fakeModerator{banned: map[uuid.UUID]bool{}}
	r := newModRoom(t, mod)
	teacher, inside := r.testJoin(t, "Guru"), r.testJoin(t, "Di dalam")
	r.members[teacher.userID].p.Role = "teacher"
	drain(inside)
	r.handle(commandCmd{c: teacher, seq: 1, name: "lock"})
	r.settle(t)
	if !r.locked {
		t.Fatal("not locked")
	}
	if s := lastState(t, inside); !s.Locked {
		t.Fatalf("occupant state = %+v", s)
	}
	// New participant through join and through travel reservation: rejected.
	newcomer := newConn(uuid.New())
	if err := r.join(newcomer, Admission{UserID: newcomer.userID, Role: "participant"}); !errors.Is(err, errLocked) {
		t.Fatalf("join = %v, want locked", err)
	}
	if err := r.reserve(uuid.New(), false); !errors.Is(err, errLocked) {
		t.Fatalf("reserve = %v, want locked", err)
	}
	if r.people() != 2 {
		t.Fatalf("people = %d, rejected entries leaked", r.people())
	}
	// Staff may still enter; an occupant reconnecting within grace keeps the seat.
	if err := r.reserve(uuid.New(), true); err != nil {
		t.Fatalf("staff reserve = %v", err)
	}
	r.leave(inside)
	back := newConn(inside.userID)
	if err := r.join(back, Admission{UserID: inside.userID, Role: "participant"}); err != nil {
		t.Fatalf("reconnect = %v", err)
	}
	r.handle(commandCmd{c: teacher, seq: 2, name: "lock"})
	if got := errorsOf(t, teacher); len(got) != 1 || got[0] != CodeBadMessage {
		t.Fatalf("double lock errors = %v", got)
	}
	r.handle(commandCmd{c: teacher, seq: 3, name: "unlock"})
	r.settle(t)
	if r.locked {
		t.Fatal("still locked")
	}
}

func TestLockForbiddenOnCampus(t *testing.T) {
	r, _ := newMapRoom(t, "campus", true)
	a := r.testJoin(t, "Guru")
	r.members[a.userID].p.Role = "teacher"
	drain(a)
	r.handle(commandCmd{c: a, name: "lock"})
	if got := errorsOf(t, a); len(got) != 1 || got[0] != CodeForbidden {
		t.Fatalf("errors = %v", got)
	}
}

func TestStoreRecordActionScoped(t *testing.T) {
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
	teacher, student, outsider := user("g@m.co"), user("s@m.co"), user("x@m.co")
	o, err := q.CreateOrg(ctx, query.CreateOrgParams{Name: "SMA", Slug: "sma-mute", CreatedBy: teacher, CreatedAt: ts})
	if err != nil {
		t.Fatal(err)
	}
	for id, role := range map[uuid.UUID]query.OrgRole{teacher: query.OrgRoleTeacher, student: query.OrgRoleParticipant} {
		if _, err := q.CreateMembership(ctx, query.CreateMembershipParams{OrgID: o.ID, UserID: id, Role: role, JoinedAt: ts}); err != nil {
			t.Fatal(err)
		}
	}
	rm, err := q.CreateRoom(ctx, query.CreateRoomParams{OrgID: o.ID, Name: "Kelas", MapKey: "classroom",
		BuildingKey: pgtype.Text{String: "kelas-a", Valid: true}, CreatedBy: teacher, Now: ts})
	if err != nil {
		t.Fatal(err)
	}
	s := NewStore(pool)
	if ok, err := s.RecordAction(ctx, rm.ID, teacher, uuid.Nil, "lock", at); err != nil || ok {
		t.Fatalf("lock without session = %v %v", ok, err)
	}
	if _, err := s.OpenSession(ctx, rm.ID, at); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name          string
		actor, target uuid.UUID
		action        string
		want          bool
	}{
		{"mute participant", teacher, student, "mute", true},
		{"unmute participant", teacher, student, "unmute", true},
		{"lock", teacher, uuid.Nil, "lock", true},
		{"unlock", teacher, uuid.Nil, "unlock", true},
		{"participant actor", student, uuid.Nil, "lock", false},
		{"mute staff", teacher, teacher, "mute", false},
		{"mute other org", teacher, outsider, "mute", false},
		{"lock with target", teacher, student, "lock", false},
		{"mute without target", teacher, uuid.Nil, "mute", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ok, err := s.RecordAction(ctx, rm.ID, tc.actor, tc.target, tc.action, at)
			if err != nil || ok != tc.want {
				t.Fatalf("ok=%v err=%v want %v", ok, err, tc.want)
			}
		})
	}
	var n int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM moderation_actions WHERE room_id=$1", rm.ID).Scan(&n); err != nil || n != 4 {
		t.Fatalf("audit rows = %d %v", n, err)
	}
}
