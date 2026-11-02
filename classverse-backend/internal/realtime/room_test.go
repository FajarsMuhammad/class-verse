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
)

// fakeClock is advanced by tests; rooms read it on every command and tick.
type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

// newTestRoom builds a room without its goroutine; tests call handle/tick
// directly, which keeps them deterministic.
func newTestRoom(t *testing.T, mapKey string, capacity int) (*room, *fakeClock) {
	t.Helper()
	md := loadCampus(t)
	md2 := *md
	md2.Key = mapKey
	c := &fakeClock{t: time.Date(2026, 1, 1, 8, 0, 0, 0, time.UTC)}
	h := NewHub(map[string]*MapData{mapKey: &md2}, nil, c.Now, slog.New(slog.NewTextHandler(io.Discard, nil)))
	t.Cleanup(func() { _ = h.Shutdown(context.Background()) })
	return newRoom(Admission{RoomID: uuid.New(), Capacity: capacity}, &md2, h, c.Now, h.log), c
}

func (r *room) testJoin(t *testing.T, name string) *conn {
	t.Helper()
	c := newConn(uuid.New())
	if err := r.join(c, Admission{UserID: c.userID, Name: name, Avatar: "default", Role: "participant"}); err != nil {
		t.Fatal(err)
	}
	return c
}

// drain returns the queued messages of c by type.
func drain(c *conn) map[string][]json.RawMessage {
	out := map[string][]json.RawMessage{}
	for {
		select {
		case m := <-c.out:
			out[m.t] = append(out[m.t], m.d)
		default:
			return out
		}
	}
}

func TestRoomJoinMoveSnapshot(t *testing.T) {
	r, clk := newTestRoom(t, "classroom-test", 40)
	a := r.testJoin(t, "Ani")
	b := r.testJoin(t, "Budi")

	ma, mb := drain(a), drain(b)
	if len(ma[TypeJoined]) != 1 || len(mb[TypeJoined]) != 0 {
		t.Fatalf("joined: a=%d b=%d", len(ma[TypeJoined]), len(mb[TypeJoined]))
	}
	var w WelcomeMsg
	_ = json.Unmarshal(mb[TypeWelcome][0], &w)
	if w.SelfID != b.userID || len(w.Participants) != 2 {
		t.Fatalf("welcome = %+v", w)
	}

	// Welcome/joined already contain positions; unchanged avatars need no snapshot.
	r.tick(clk.Now())
	if len(drain(a)[TypeSnapshot]) != 0 {
		t.Error("first tick duplicated joined position")
	}
	drain(b)
	// Nothing changed: no snapshot.
	clk.Advance(tickInterval)
	r.tick(clk.Now())
	if len(drain(a)[TypeSnapshot]) != 0 {
		t.Error("snapshot sent without changes")
	}

	mbr := r.members[b.userID]
	clk.Advance(100 * time.Millisecond)
	r.handle(moveCmd{c: b, seq: 1, m: MoveMsg{X: mbr.p.X + 10, Y: mbr.p.Y, Dir: DirRight}})
	r.tick(clk.Now())
	snaps := drain(a)[TypeSnapshot]
	if len(snaps) != 1 {
		t.Fatalf("snapshots after move = %d", len(snaps))
	}
	var s SnapshotMsg
	_ = json.Unmarshal(snaps[0], &s)
	if len(s.P) != 1 || s.P[0].ID != b.userID || s.P[0].Dir != DirRight {
		t.Errorf("snapshot = %+v", s)
	}

	// A rejected move returns the authoritative position to the sender only.
	r.handle(moveCmd{c: b, seq: 2, m: MoveMsg{X: mbr.p.X + 500, Y: mbr.p.Y, Dir: DirRight}})
	errs := drain(b)[TypeError]
	var e ErrorMsg
	if len(errs) != 1 || json.Unmarshal(errs[0], &e) != nil || e.Code != CodeMoveRejected || e.Seq != 2 || *e.X != mbr.p.X {
		t.Errorf("reject = %s", errs)
	}
}

func TestRoomReconnectAndHold(t *testing.T) {
	r, clk := newTestRoom(t, "classroom-test", 40)
	a := r.testJoin(t, "Ani")
	b := r.testJoin(t, "Budi")
	drain(a)

	r.handle(leaveCmd{c: b})
	clk.Advance(holdDuration - time.Second)
	r.tick(clk.Now())
	if len(drain(a)[TypeLeft]) != 0 {
		t.Fatal("left sent during grace period")
	}

	// Reconnect with a new connection for the same user: no joined for others.
	b2 := newConn(b.userID)
	if err := r.join(b2, Admission{UserID: b.userID, Name: "Budi", Avatar: "default", Role: "participant"}); err != nil {
		t.Fatal(err)
	}
	if len(drain(a)[TypeJoined]) != 0 {
		t.Error("joined sent on reconnect")
	}
	if len(drain(b2)[TypeWelcome]) != 1 {
		t.Error("reconnected client should get welcome")
	}

	// A late leave from the old connection is ignored.
	r.handle(leaveCmd{c: b})
	if r.members[b.userID].conn != b2 {
		t.Error("stale leave detached new connection")
	}

	r.handle(leaveCmd{c: b2})
	clk.Advance(holdDuration)
	r.tick(clk.Now())
	if len(drain(a)[TypeLeft]) != 1 {
		t.Error("left not sent after grace period")
	}
}

func TestRoomReplacedConnection(t *testing.T) {
	r, _ := newTestRoom(t, "classroom-test", 40)
	a := r.testJoin(t, "Ani")
	a2 := newConn(a.userID)
	if err := r.join(a2, Admission{UserID: a.userID, Name: "Ani"}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-a.done:
		if a.code != closeReplaced {
			t.Errorf("code = %d", a.code)
		}
	default:
		t.Error("old connection not closed")
	}
}

func TestRoomCapacity(t *testing.T) {
	r, _ := newTestRoom(t, "classroom-test", 1)
	r.testJoin(t, "Ani")
	c := newConn(uuid.New())
	if err := r.join(c, Admission{UserID: c.userID}); !errors.Is(err, errRoomFull) {
		t.Errorf("join full room = %v", err)
	}
}

func TestRoomAOI(t *testing.T) {
	r, clk := newTestRoom(t, "campus", 200)
	a := r.testJoin(t, "Ani")
	b := r.testJoin(t, "Budi")
	r.tick(clk.Now())
	drain(a)
	// Teleport b far away directly in state (as if it had walked there).
	mb := r.members[b.userID]
	mb.p.X, mb.p.Y, mb.dirty = 60*32, 2*32, true
	clk.Advance(tickInterval)
	r.tick(clk.Now())
	var s SnapshotMsg
	snaps := drain(a)[TypeSnapshot]
	if len(snaps) != 1 || json.Unmarshal(snaps[0], &s) != nil || len(s.Out) != 1 || len(s.P) != 0 {
		t.Fatalf("expected b to leave a's AOI, got %s", snaps)
	}
	// While far away, b's moves are not sent to a.
	mb.p.X, mb.dirty = mb.p.X+8, true
	clk.Advance(tickInterval)
	r.tick(clk.Now())
	if len(drain(a)[TypeSnapshot]) != 0 {
		t.Error("far avatar leaked into snapshot")
	}
}

func TestWelcomeHidesStationaryAvatarOutsideAOI(t *testing.T) {
	r, clk := newTestRoom(t, "campus", 200)
	a := r.testJoin(t, "Ani")
	r.members[a.userID].p.X, r.members[a.userID].p.Y = 60*32, 2*32
	b := r.testJoin(t, "Budi")
	drain(a)
	drain(b)
	r.tick(clk.Now())
	for _, c := range []*conn{a, b} {
		msgs := drain(c)[TypeSnapshot]
		var s SnapshotMsg
		if len(msgs) != 1 || json.Unmarshal(msgs[0], &s) != nil || len(s.Out) != 1 {
			t.Fatalf("stationary far participant not hidden: %s", msgs)
		}
	}
}

func TestRoomProfileUpdate(t *testing.T) {
	r, _ := newTestRoom(t, "campus", 200)
	a, b := r.testJoin(t, "Ani"), r.testJoin(t, "Budi")
	drain(a)
	drain(b)
	before := r.members[a.userID].p
	done := make(chan struct{})
	r.handle(profileCmd{p: ProfileUpdatedMsg{ID: a.userID, Name: "Ani Baru", Avatar: "bun-nila"}, done: done})
	<-done
	for _, c := range []*conn{a, b} {
		var p ProfileUpdatedMsg
		msgs := drain(c)[TypeProfileUpdated]
		if len(msgs) != 1 || json.Unmarshal(msgs[0], &p) != nil || p.Name != "Ani Baru" || p.Avatar != "bun-nila" {
			t.Fatal("identity event missing")
		}
	}
	if r.members[a.userID].p.X != before.X || r.members[a.userID].p.Y != before.Y {
		t.Fatal("profile moved participant")
	}
}

func TestSlowConsumerClosed(t *testing.T) {
	c := newConn(uuid.New())
	for range outBuffer {
		if !c.send(newOut(TypePong, PongMsg{})) {
			t.Fatal("buffer smaller than expected")
		}
	}
	if c.send(newOut(TypePong, PongMsg{})) {
		t.Fatal("send on full buffer succeeded")
	}
	select {
	case <-c.done:
	default:
		t.Error("slow consumer not closed")
	}
}
