package realtime

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
)

// TestLibraryIsQuiet: the production library map disables voice (no LiveKit
// identity, no broadcast) while chat keeps working.
func TestLibraryIsQuiet(t *testing.T) {
	mapData, err := LoadMaps("library")
	if err != nil {
		t.Fatal(err)
	}
	md := mapData["library"]
	if !md.Quiet || md.PixelWidth() != 1244 || md.PixelHeight() != 764 || !md.HasExit {
		t.Fatalf("library geometry: quiet=%v %dx%d exit=%v", md.Quiet, md.PixelWidth(), md.PixelHeight(), md.HasExit)
	}
	c := &fakeClock{t: time.Date(2026, 1, 1, 8, 0, 0, 0, time.UTC)}
	h := NewHub(mapData, nil, c.Now, slog.New(slog.NewTextHandler(io.Discard, nil)))
	h.SetVoice(newFakeVoice())
	t.Cleanup(func() { _ = h.Shutdown(context.Background()) })
	r := newRoom(Admission{RoomID: uuid.New(), Capacity: 40, AllChat: true}, md, h, c.Now, h.log)

	teacher, student := r.testJoin(t, "Guru"), r.testJoin(t, "Siswa")
	r.members[teacher.userID].p.Role = "teacher"
	if id := r.members[student.userID].voiceIdentity; id != "" {
		t.Fatalf("quiet room issued voice identity %q", id)
	}
	drain(teacher)
	drain(student)
	r.handle(commandCmd{c: teacher, seq: 1, name: "voice_broadcast"})
	if got := errorsOf(t, teacher); len(got) != 1 || got[0] != CodeForbidden || len(r.voiceOn) != 0 {
		t.Fatalf("broadcast in quiet room: errors=%v on=%v", got, r.voiceOn)
	}
	r.say(student, ScopeRoom, "halo")
	if got := chats(t, teacher); len(got) != 1 || got[0].Text != "halo" {
		t.Fatalf("chat in quiet room: %+v", got)
	}
}
