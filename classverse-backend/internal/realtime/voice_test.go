package realtime

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
)

// fakeVoice records LiveKit moderation calls.
type fakeVoice struct {
	mu    sync.Mutex
	calls []string
	seen  chan struct{}
}

func newFakeVoice() *fakeVoice { return &fakeVoice{seen: make(chan struct{}, 16)} }

func (f *fakeVoice) record(s string) {
	f.mu.Lock()
	f.calls = append(f.calls, s)
	f.mu.Unlock()
	f.seen <- struct{}{}
}

func (f *fakeVoice) SetCanPublish(_ context.Context, _, identity string, can bool) error {
	if can {
		f.record("publish:" + identity)
	} else {
		f.record("nopublish:" + identity)
	}
	return nil
}

func (f *fakeVoice) Remove(_ context.Context, _, identity string) error {
	f.record("remove:" + identity)
	return nil
}

// wait returns the calls after n arrived (calls run on the hub voice worker).
func (f *fakeVoice) wait(t *testing.T, n int) []string {
	t.Helper()
	for range n {
		select {
		case <-f.seen:
		case <-time.After(2 * time.Second):
			t.Fatalf("voice calls = %v, want %d", f.calls, n)
		}
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.calls...)
}

func TestVoiceMuteFollowsAudit(t *testing.T) {
	for _, fail := range []bool{false, true} {
		mod := &fakeModerator{banned: map[uuid.UUID]bool{}, fail: fail}
		r := newModRoom(t, mod)
		v := newFakeVoice()
		r.hub.SetVoice(v)
		teacher, student := r.testJoin(t, "Guru"), r.testJoin(t, "Siswa")
		r.members[teacher.userID].p.Role = "teacher"
		r.handle(commandCmd{c: teacher, seq: 1, name: "mute", target: student.userID})
		r.settle(t)
		if fail {
			select {
			case <-v.seen:
				t.Fatalf("voice muted although the audit failed: %v", v.calls)
			case <-time.After(100 * time.Millisecond):
			}
			continue
		}
		if got := v.wait(t, 2); len(got) != 2 || !strings.HasPrefix(got[0], "remove:"+student.userID.String()+":") {
			t.Fatalf("mute calls=%v", got)
		}
		r.settle(t) // removal confirmation restores admission with muted grant
		r.handle(commandCmd{c: teacher, seq: 2, name: "unmute", target: student.userID})
		r.settle(t)
		if got := v.wait(t, 2); len(got) != 4 || !strings.HasPrefix(got[2], "remove:"+student.userID.String()+":") {
			t.Fatalf("unmute calls=%v", got)
		}
		r.settle(t)
	}
}

func TestVoiceRemovedOnKickAndLeave(t *testing.T) {
	mod := &fakeModerator{banned: map[uuid.UUID]bool{}}
	r := newModRoom(t, mod)
	v := newFakeVoice()
	r.hub.SetVoice(v)
	teacher, kicked, leaver := r.testJoin(t, "Guru"), r.testJoin(t, "Siswa"), r.testJoin(t, "Pergi")
	r.members[teacher.userID].p.Role = "teacher"
	r.handle(commandCmd{c: teacher, seq: 1, name: "kick", target: kicked.userID})
	r.settle(t)
	r.remove(leaver.userID) // travel or grace expiry
	got := v.wait(t, 4)
	if !strings.HasPrefix(got[0], "remove:"+kicked.userID.String()+":") || !strings.HasPrefix(got[2], "remove:"+leaver.userID.String()+":") {
		t.Fatalf("calls = %v", got)
	}
}

func TestVoiceBroadcastState(t *testing.T) {
	r, _ := newMapRoom(t, "classroom", true)
	teacher, student := r.testJoin(t, "Guru"), r.testJoin(t, "Siswa")
	r.members[teacher.userID].p.Role = "teacher"
	drain(teacher)
	drain(student)

	r.handle(commandCmd{c: student, seq: 1, name: "voice_broadcast"})
	if got := errorsOf(t, student); len(got) != 1 || got[0] != CodeForbidden || len(r.voiceOn) != 0 {
		t.Fatalf("participant broadcast errors = %v", got)
	}
	r.handle(commandCmd{c: teacher, seq: 2, name: "voice_broadcast"})
	if s := lastState(t, student); len(s.VoiceBroadcast) != 1 || s.VoiceBroadcast[0] != teacher.userID {
		t.Fatalf("student state = %+v", s)
	}
	// A late joiner learns it from welcome.
	late := r.testJoin(t, "Telat")
	var w WelcomeMsg
	if err := json.Unmarshal(drain(late)[TypeWelcome][0], &w); err != nil {
		t.Fatal(err)
	}
	if len(w.VoiceBroadcast) != 1 {
		t.Fatalf("welcome voice_broadcast = %v", w.VoiceBroadcast)
	}
	// Leaving the room ends the broadcast for everyone.
	drain(student)
	r.remove(teacher.userID)
	if s := lastState(t, student); len(s.VoiceBroadcast) != 0 {
		t.Fatalf("broadcast kept after teacher left: %+v", s)
	}
}

func TestVoiceBroadcastForbiddenOnCampus(t *testing.T) {
	r, _ := newMapRoom(t, "campus", true)
	teacher := r.testJoin(t, "Guru")
	r.members[teacher.userID].p.Role = "teacher"
	drain(teacher)
	r.handle(commandCmd{c: teacher, seq: 1, name: "voice_broadcast"})
	if got := errorsOf(t, teacher); len(got) != 1 || got[0] != CodeForbidden {
		t.Fatalf("campus errors = %v", got)
	}
}

func TestHubVoiceMember(t *testing.T) {
	campus := loadCampus(t)
	classroom := *campus
	classroom.Key = "classroom"
	hub := NewHub(map[string]*MapData{"campus": campus, "classroom": &classroom}, nil, time.Now, slog.New(slog.NewTextHandler(io.Discard, nil)))
	t.Cleanup(func() { _ = hub.Shutdown(context.Background()) })

	c := newConn(uuid.New())
	cls := Admission{RoomID: uuid.New(), MapKey: "classroom", Capacity: 40, UserID: c.userID, Name: "Ani", Role: "participant"}
	if _, err := hub.join(c, cls); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if m, err := hub.VoiceMember(ctx, cls.RoomID, c.userID); err != nil || m.Name != "Ani" || m.Muted {
		t.Fatalf("member = %+v %v", m, err)
	}
	if _, err := hub.VoiceMember(ctx, cls.RoomID, uuid.New()); !errors.Is(err, ErrNotInRoom) {
		t.Fatalf("stranger err = %v", err)
	}
	if _, err := hub.VoiceMember(ctx, uuid.New(), c.userID); !errors.Is(err, ErrNotInRoom) {
		t.Fatalf("other room err = %v", err)
	}

	other := newConn(uuid.New())
	camp := Admission{RoomID: uuid.New(), MapKey: "campus", Capacity: 200, UserID: other.userID, Role: "participant"}
	if _, err := hub.join(other, camp); err != nil {
		t.Fatal(err)
	}
	if _, err := hub.VoiceMember(ctx, camp.RoomID, other.userID); !errors.Is(err, ErrNotInRoom) {
		t.Fatalf("campus err = %v", err)
	}
}
