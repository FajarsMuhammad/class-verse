package realtime

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

type failingVoice struct {
	fail  bool
	calls int
}

func (f *failingVoice) Remove(context.Context, string, string) error {
	f.calls++
	if f.fail {
		return errors.New("offline")
	}
	return nil
}
func (*failingVoice) SetCanPublish(context.Context, string, string, bool) error { return nil }

// zoneSeat returns a walkable point inside the first zone.
func zoneSeat(t *testing.T, r *room) (int, int) {
	t.Helper()
	x, y, ok := r.mapd.SeatPoint(r.mapd.Zones[0].Key, 0)
	if !ok {
		t.Fatal("zone seat missing")
	}
	return x, y
}

func TestVoiceTransitionCommitsAfterConfirmation(t *testing.T) {
	r, _ := newMapRoom(t, "classroom", true)
	r.hub.SetVoice(&failingVoice{})
	c := r.testJoin(t, "Ani")
	m := r.members[c.userID]
	oldIdentity, oldChannel := m.voiceIdentity, m.voiceChannel
	x, y := zoneSeat(t, r)
	beforeX, beforeY := m.p.X, m.p.Y
	if r.voicePosition(m, x, y, DirDown) {
		t.Fatal("cross-zone move committed before removal")
	}
	if m.voiceIdentity != "" || m.p.X != beforeX || m.p.Y != beforeY {
		t.Fatal("old media admission still live or position already changed")
	}
	r.settle(t)
	if len(r.voicePending) != 0 || m.voiceIdentity == "" || m.voiceIdentity == oldIdentity || m.voiceChannel == oldChannel {
		t.Fatal("admission not rotated after confirmation")
	}
	if m.p.X != x || m.p.Y != y {
		t.Fatal("confirmed destination not committed")
	}
	var correction ErrorMsg
	for _, raw := range drain(c)[TypeError] {
		if err := json.Unmarshal(raw, &correction); err != nil {
			t.Fatal(err)
		}
	}
	if !correction.ContinuePath || correction.X == nil || *correction.X != x || correction.Y == nil || *correction.Y != y {
		t.Fatal("successful zone correction must preserve the click path")
	}
}

// A LiveKit outage must not freeze the avatar (review finding 1): moves in the
// old zone keep working, cross-zone moves are refused with a resync, and the
// stale destination is dropped instead of teleporting the avatar later.
func TestVoicePendingEvictionDoesNotFreezeAvatar(t *testing.T) {
	r, _ := newMapRoom(t, "classroom", true)
	v := &failingVoice{fail: true}
	r.hub.SetVoice(v)
	c := r.testJoin(t, "Ani")
	m := r.members[c.userID]
	drain(c)
	oldChannel := m.voiceChannel
	x, y := zoneSeat(t, r)
	beforeX, beforeY := m.p.X, m.p.Y

	r.voicePosition(m, x, y, DirDown)
	r.settle(t) // removal fails: eviction stays pending
	if len(r.voicePending) != 1 || m.voiceIdentity != "" {
		t.Fatal("failure did not remain pending")
	}
	if got := errorsOf(t, c); len(got) != 1 || got[0] != CodeMoveRejected {
		t.Fatalf("failed transition did not resync the client: %v", got)
	}

	// Same-zone move while pending: allowed.
	if !r.voicePosition(m, beforeX, beforeY, DirLeft) {
		t.Fatal("same-zone move frozen while eviction pending")
	}
	// Cross-zone move while pending: refused, client told where it really is.
	if r.voicePosition(m, x, y, DirDown) {
		t.Fatal("cross-zone move allowed while old audio still connected")
	}
	if got := errorsOf(t, c); len(got) != 1 || got[0] != CodeMoveRejected {
		t.Fatalf("blocked cross-zone move not rejected: %v", got)
	}

	v.fail = false
	r.retryVoice(r.now().Add(2 * time.Second))
	r.settle(t)
	if len(r.voicePending) != 0 || m.voiceIdentity == "" || m.voiceChannel != oldChannel {
		t.Fatalf("recovery: pending=%d identity=%q channel changed=%v", len(r.voicePending), m.voiceIdentity, m.voiceChannel != oldChannel)
	}
	if m.p.X != beforeX || m.p.Y != beforeY {
		t.Fatal("dropped destination applied after recovery")
	}
}

func TestVoiceQueueFullRetainsRevocation(t *testing.T) {
	r, _ := newMapRoom(t, "classroom", true)
	// No worker: force queue backpressure deterministically.
	r.hub.voice = &failingVoice{}
	r.hub.voiceJobs = make(chan func(context.Context))
	c := r.testJoin(t, "Ani")
	m := r.members[c.userID]
	old := m.voiceIdentity
	r.revokeVoice(m, nil)
	if m.voiceIdentity != "" || r.voicePending[old] == nil || r.voicePending[old].busy {
		t.Fatal("queue-full revocation lost")
	}
	r.hub.mu.Lock()
	pending := r.hub.voiceRevoking[c.userID]
	r.hub.mu.Unlock()
	if pending != 1 {
		t.Fatal("travel not blocked while eviction pending")
	}
	// Test fixture has no worker to await at cleanup.
	r.hub.voiceJobs = nil
}

func TestRejoinedMemberDoesNotReuseOldIdentity(t *testing.T) {
	r, _ := newMapRoom(t, "classroom", true)
	c := r.testJoin(t, "Ani")
	first := r.members[c.userID].voiceIdentity
	r.remove(c.userID)
	// Reuse same user ID, not an unrelated participant.
	adm := Admission{RoomID: r.id, MapKey: "classroom", UserID: c.userID, Name: "Ani", Role: "participant", Capacity: 40}
	if err := r.add(newConn(c.userID), adm, 64, 64, TypeWelcome); err != nil {
		t.Fatal(err)
	}
	if got := r.members[c.userID].voiceIdentity; got == first || got == "" {
		t.Fatal("old admission reused")
	}
}
