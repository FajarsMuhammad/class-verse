package realtime

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestGuestExpiryRemovesLiveAndHeldPresence(t *testing.T) {
	for _, held := range []bool{false, true} {
		t.Run(map[bool]string{false: "live", true: "held"}[held], func(t *testing.T) {
			r, clock := newTestRoom(t, "classroom-test", 40)
			c := newConn(uuid.New())
			expires := clock.Now().Add(time.Second)
			if err := r.join(c, Admission{UserID: c.userID, Name: "Guest", Role: RoleGuest, GuestExpiresAt: expires}); err != nil {
				t.Fatal(err)
			}
			if held {
				r.leave(c)
			}
			clock.Advance(time.Second)
			r.tick(clock.Now())
			if r.members[c.userID] != nil {
				t.Fatal("expired guest retained presence")
			}
		})
	}
}

func TestGuestRevocationRemovesHeldPresence(t *testing.T) {
	r, _ := newTestRoom(t, "classroom-test", 40)
	c := newConn(uuid.New())
	if err := r.join(c, Admission{UserID: c.userID, Name: "Guest", Role: RoleGuest}); err != nil {
		t.Fatal(err)
	}
	r.leave(c)
	r.handle(endGuestCmd{userID: c.userID})
	if r.members[c.userID] != nil {
		t.Fatal("revoked guest retained presence")
	}
}

func TestGuestTravelAndLearningIsolation(t *testing.T) {
	r, _ := newTestRoom(t, "classroom-test", 40)
	c := newConn(uuid.New())
	c.guest = true
	if err := r.join(c, Admission{UserID: c.userID, Name: "Guest", Role: RoleGuest}); err != nil {
		t.Fatal(err)
	}
	g := &Gateway{}
	if _, err := g.travel(t.Context(), c, r, uuid.New()); err == nil {
		t.Fatal("guest travel accepted")
	}
	r.pin = &LearningPin{ModuleID: uuid.New()}
	if r.state(c.userID).Pin != nil {
		t.Fatal("guest received learning data")
	}
}

func TestGuestModeration(t *testing.T) {
	mod := &fakeModerator{banned: map[uuid.UUID]bool{}}
	r := newModRoom(t, mod)
	teacher, guest := r.testJoin(t, "Host"), r.testJoin(t, "Guest")
	r.members[teacher.userID].p.Role = "teacher"
	r.members[guest.userID].p.Role = RoleGuest
	drain(teacher)
	drain(guest)
	r.handle(commandCmd{c: teacher, seq: 1, name: "mute", target: guest.userID})
	r.settle(t)
	if !r.state(guest.userID).Muted {
		t.Fatal("guest not muted")
	}
	r.say(guest, ScopeRoom, "blocked")
	if got := errorsOf(t, guest); len(got) != 1 || got[0] != CodeMuted {
		t.Fatal("muted guest chat", got)
	}
	r.handle(commandCmd{c: teacher, seq: 2, name: "kick", target: guest.userID})
	r.settle(t)
	if r.members[guest.userID] != nil {
		t.Fatal("kicked guest retained")
	}
	if !mod.banned[guest.userID] {
		t.Fatal("guest ban not persisted")
	}
}

func TestGuestRemovalRevokesHeldVoice(t *testing.T) {
	r, _ := newMapRoom(t, "classroom", true)
	voice := newFakeVoice()
	r.hub.SetVoice(voice)
	guest := r.testJoin(t, "Guest")
	r.members[guest.userID].p.Role = RoleGuest
	r.leave(guest)
	r.endGuest(guest.userID)
	if got := voice.wait(t, 2); len(got) != 2 {
		t.Fatal("zone and broadcast eviction required", got)
	}
	if r.members[guest.userID] != nil {
		t.Fatal("presence retained during voice cleanup")
	}
	r.settle(t)
}
