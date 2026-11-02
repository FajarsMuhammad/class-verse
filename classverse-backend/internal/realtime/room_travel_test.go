package realtime

import (
	"errors"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/google/uuid"
)

func TestArrivalAfterDisconnectRetainsGrace(t *testing.T) {
	r, clock := newTestRoom(t, "classroom-test", 40)
	c := newConn(uuid.New())
	c.close(websocket.StatusNormalClosure, "")
	if err := r.add(c, Admission{UserID: c.userID, Name: "Ani", Role: "participant"}, 100, 100, TypeTraveled); err != nil {
		t.Fatal(err)
	}
	m := r.members[c.userID]
	if m == nil || m.conn != nil || !m.heldUntil.Equal(clock.Now().Add(holdDuration)) {
		t.Fatal("committed arrival not held for reconnect")
	}
	replacement := newConn(c.userID)
	if err := r.join(replacement, Admission{UserID: c.userID, Name: "Ani", Role: "participant"}); err != nil {
		t.Fatal(err)
	}
	if r.members[c.userID].p.X != 100 || r.people() != 1 {
		t.Fatal("reconnect lost position or duplicated presence")
	}
}

func TestRoomSessionEmptyTimeout(t *testing.T) {
	r, clock := newTestRoom(t, "classroom-test", 40)
	c := r.testJoin(t, "Ani")
	if !r.sessionOpen {
		t.Fatal("session not opened")
	}
	r.leave(c)
	clock.Advance(holdDuration - time.Second)
	r.tick(clock.Now())
	if r.people() != 1 {
		t.Fatal("grace avatar released early")
	}
	clock.Advance(2 * time.Second)
	r.tick(clock.Now())
	if r.people() != 0 || !r.sessionOpen {
		t.Fatal("session closed before empty timeout")
	}
	clock.Advance(sessionIdle - time.Second)
	r.tick(clock.Now())
	if !r.sessionOpen {
		t.Fatal("session closed early")
	}
	clock.Advance(time.Second)
	r.tick(clock.Now())
	if r.sessionOpen {
		t.Fatal("empty session not closed")
	}
}

func TestRoomCapacityReservations(t *testing.T) {
	r, _ := newTestRoom(t, "classroom-test", 40)
	for range 39 {
		r.testJoin(t, "Participant")
	}
	winner := uuid.New()
	if err := r.reserve(winner); err != nil {
		t.Fatal(err)
	}
	if err := r.reserve(uuid.New()); !errors.Is(err, errRoomFull) {
		t.Fatalf("41st reservation: %v", err)
	}
	if r.people() != 40 {
		t.Fatalf("people=%d", r.people())
	}
	r.pending.Add(1)
	r.handle(cancelCmd{userID: winner})
	if err := r.reserve(uuid.New()); err != nil {
		t.Fatalf("cancel did not free slot: %v", err)
	}
}
