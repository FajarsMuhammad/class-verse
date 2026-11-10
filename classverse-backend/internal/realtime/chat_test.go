package realtime

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/FajarsMuhammad/class-verse/classverse-backend/maps"
)

// newMapRoom builds a goroutine-less room on a real embedded map.
func newMapRoom(t *testing.T, mapKey string, allChat bool) (*room, *fakeClock) {
	t.Helper()
	raw, err := maps.Read(mapKey)
	if err != nil {
		t.Fatal(err)
	}
	md, err := ParseMap(mapKey, raw)
	if err != nil {
		t.Fatal(err)
	}
	c := &fakeClock{t: time.Date(2026, 1, 1, 8, 0, 0, 0, time.UTC)}
	h := NewHub(map[string]*MapData{mapKey: md}, nil, c.Now, slog.New(slog.NewTextHandler(io.Discard, nil)))
	t.Cleanup(func() { _ = h.Shutdown(context.Background()) })
	return newRoom(Admission{RoomID: uuid.New(), Capacity: 40, AllChat: allChat}, md, h, c.Now, h.log), c
}

// place moves a member to (x, y) directly (test only).
func (r *room) place(c *conn, x, y int) { r.members[c.userID].p.X, r.members[c.userID].p.Y = x, y }

// say sends chat and settles any stored-then-delivered message.
func (r *room) say(c *conn, scope, text string) {
	r.handle(chatCmd{c: c, seq: 9, text: text, scope: scope})
	for {
		select {
		case cmd := <-r.inbox:
			r.handle(cmd)
		default:
			return
		}
	}
}

func chats(t *testing.T, c *conn) []ChatOutMsg {
	t.Helper()
	var out []ChatOutMsg
	for _, raw := range drain(c)[TypeChat] {
		var m ChatOutMsg
		if err := json.Unmarshal(raw, &m); err != nil {
			t.Fatal(err)
		}
		out = append(out, m)
	}
	return out
}

func errorsOf(t *testing.T, c *conn) []string {
	t.Helper()
	var codes []string
	for _, raw := range drain(c)[TypeError] {
		var e ErrorMsg
		_ = json.Unmarshal(raw, &e)
		codes = append(codes, e.Code)
	}
	return codes
}

func TestChatNearRadius(t *testing.T) {
	r, _ := newMapRoom(t, "campus", false)
	a, b, far := r.testJoin(t, "Ani"), r.testJoin(t, "Budi"), r.testJoin(t, "Jauh")
	x, y, _ := r.mapd.SpawnPoint(0)
	r.place(a, x, y)
	r.place(b, x+160, y)   // exactly on the radius
	r.place(far, x+161, y) // just outside
	drain(a)
	drain(b)
	drain(far)
	r.say(a, ScopeNear, "halo")
	if got := chats(t, a); len(got) != 1 || got[0].From.Name != "Ani" || got[0].Scope != ScopeNear {
		t.Fatalf("sender copy = %+v", got)
	}
	if len(chats(t, b)) != 1 {
		t.Error("avatar at 160 px did not receive")
	}
	if len(chats(t, far)) != 0 {
		t.Error("avatar at 161 px received")
	}
}

// In a classroom near chat reaches 3 tiles (96 px), not the campus 5.
func TestChatNearRadiusPerMap(t *testing.T) {
	r, _ := newMapRoom(t, "classroom", false)
	a, b, far := r.testJoin(t, "Ani"), r.testJoin(t, "Budi"), r.testJoin(t, "Jauh")
	x, y, _ := r.mapd.SpawnPoint(0)
	r.place(a, x, y)
	r.place(b, x, y+96)    // exactly on the radius
	r.place(far, x, y+128) // inside the campus range, outside the room range
	drain(a)
	drain(b)
	drain(far)
	r.say(a, ScopeNear, "halo")
	if len(chats(t, b)) != 1 {
		t.Error("avatar at 96 px did not receive")
	}
	if len(chats(t, far)) != 0 {
		t.Error("avatar at 128 px received room near chat")
	}
}

func TestChatHeldAvatarDoesNotReceive(t *testing.T) {
	r, _ := newMapRoom(t, "campus", false)
	a, b := r.testJoin(t, "Ani"), r.testJoin(t, "Budi")
	x, y, _ := r.mapd.SpawnPoint(0)
	r.place(a, x, y)
	r.place(b, x, y)
	r.leave(b) // disconnected, held for reconnect
	r.say(a, ScopeNear, "halo")
	if len(chats(t, b)) != 0 {
		t.Error("held avatar received chat")
	}
}

func TestChatZonesAcrossDistance(t *testing.T) {
	r, clk := newMapRoom(t, "classroom", true)
	if len(r.mapd.Zones) < 2 {
		t.Fatal("classroom needs two zones")
	}
	z1, z2 := r.mapd.Zones[0].Area, r.mapd.Zones[1].Area
	a, b, other := r.testJoin(t, "Ani"), r.testJoin(t, "Budi"), r.testJoin(t, "Lain")
	r.place(a, z1.X+2, z1.Y+2)
	r.place(b, z1.X+z1.W-2, z1.Y+z1.H-2) // same zone, far corner
	r.place(other, z2.X+2, z2.Y+2)
	r.tick(clk.Now())
	var zm ZoneMsg
	if raw := drain(a)[TypeZone]; len(raw) != 1 || json.Unmarshal(raw[0], &zm) != nil || zm.Key != r.mapd.Zones[0].Key || zm.Name == "" {
		t.Fatalf("zone message = %s", raw)
	}
	drain(b)
	drain(other)

	r.say(a, ScopeZone, "meja satu")
	if got := chats(t, b); len(got) != 1 || got[0].Zone != r.mapd.Zones[0].Key {
		t.Fatalf("zone mate got %+v", got)
	}
	if len(chats(t, other)) != 0 {
		t.Error("other zone received")
	}

	// Leaving the zone: told on the next tick, and zone chat is refused.
	x, y, _ := r.mapd.SpawnPoint(0)
	r.place(a, x, y)
	r.tick(clk.Now())
	if raw := drain(a)[TypeZone]; len(raw) != 1 || json.Unmarshal(raw[0], &zm) != nil || zm.Key != "" {
		t.Fatalf("leave zone message = %s", raw)
	}
	r.say(a, ScopeZone, "masih?")
	if codes := errorsOf(t, a); len(codes) != 1 || codes[0] != CodeNotInZone {
		t.Fatalf("errors = %v", codes)
	}
}

func TestChatRoomScopeAndToggle(t *testing.T) {
	r, _ := newMapRoom(t, "classroom", true)
	a, b := r.testJoin(t, "Ani"), r.testJoin(t, "Budi")
	drain(a)
	drain(b)
	r.say(a, ScopeRoom, "semua")
	if len(chats(t, b)) != 1 {
		t.Fatal("room chat not delivered")
	}
	r.handle(roomStateCmd{allChat: false})
	var st RoomStateMsg
	if raw := drain(b)[TypeRoomState]; len(raw) != 1 || json.Unmarshal(raw[0], &st) != nil || st.AllChatEnabled {
		t.Fatalf("room_state = %s", raw)
	}
	drain(a)
	r.say(a, ScopeRoom, "masih semua?")
	if codes := errorsOf(t, a); len(codes) != 1 || codes[0] != CodeAllChatDisabled {
		t.Fatalf("errors = %v", codes)
	}
	if len(chats(t, b)) != 0 {
		t.Error("room chat delivered while disabled")
	}
}

func TestChatCampusAlwaysAllowsRoomScope(t *testing.T) {
	r, _ := newMapRoom(t, "campus", false)
	a := r.testJoin(t, "Ani")
	r.handle(roomStateCmd{allChat: false})
	drain(a)
	r.say(a, ScopeRoom, "halo sekolah")
	if len(chats(t, a)) != 1 {
		t.Fatal("campus room chat refused")
	}
}

func TestChatRateLimit(t *testing.T) {
	r, clk := newMapRoom(t, "campus", false)
	a := r.testJoin(t, "Ani")
	drain(a)
	for range 6 {
		r.say(a, ScopeNear, "spam")
	}
	out := drain(a)
	if got := len(out[TypeChat]); got != int(chatBurst) {
		t.Fatalf("burst delivered %d, want %d", got, int(chatBurst))
	}
	var e ErrorMsg
	if len(out[TypeError]) != 1 || json.Unmarshal(out[TypeError][0], &e) != nil || e.Code != CodeRateLimited {
		t.Fatalf("errors = %s", out[TypeError])
	}
	clk.Advance(time.Second) // refills chatRate tokens
	for range 3 {
		r.say(a, ScopeNear, "lagi")
	}
	if got := len(chats(t, a)); got != int(chatRate) {
		t.Fatalf("after refill delivered %d, want %d", got, int(chatRate))
	}
}

func TestChatUnknownScope(t *testing.T) {
	r, _ := newMapRoom(t, "campus", false)
	a := r.testJoin(t, "Ani")
	drain(a)
	r.say(a, "unknown", "semua dengar")
	if codes := errorsOf(t, a); len(codes) != 1 || codes[0] != CodeBadMessage {
		t.Fatalf("errors = %v", codes)
	}
}

func TestWelcomeCarriesAllChatAndZone(t *testing.T) {
	r, clk := newMapRoom(t, "classroom", false)
	a := r.testJoin(t, "Ani")
	z := r.mapd.Zones[0].Area
	r.place(a, z.X+2, z.Y+2)
	r.tick(clk.Now())
	r.leave(a)
	again := newConn(a.userID)
	if err := r.join(again, Admission{UserID: a.userID, Name: "Ani"}); err != nil {
		t.Fatal(err)
	}
	out := drain(again)
	var w WelcomeMsg
	if json.Unmarshal(out[TypeWelcome][0], &w) != nil || w.AllChatEnabled {
		t.Fatalf("welcome all_chat = %v", w.AllChatEnabled)
	}
	if len(out[TypeZone]) != 1 {
		t.Fatal("reconnect did not restate zone")
	}
}
