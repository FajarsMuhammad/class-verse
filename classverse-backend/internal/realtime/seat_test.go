package realtime

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
)

func (r *room) listJoin(t *testing.T, name string) *conn {
	t.Helper()
	c := newConn(uuid.New())
	c.list = true
	if err := r.join(c, Admission{UserID: c.userID, Name: name, Avatar: "default", Role: "participant"}); err != nil {
		t.Fatal(err)
	}
	return c
}

func lastZone(t *testing.T, c *conn) ZoneMsg {
	t.Helper()
	zs := drain(c)[TypeZone]
	if len(zs) == 0 {
		return ZoneMsg{Key: "<none>"}
	}
	var z ZoneMsg
	if err := json.Unmarshal(zs[len(zs)-1], &z); err != nil {
		t.Fatal(err)
	}
	return z
}

func TestSeatEveryZone(t *testing.T) {
	for _, key := range []string{"classroom", "garden"} {
		t.Run(key, func(t *testing.T) {
			r, clk := newMapRoom(t, key, true)
			phone := r.listJoin(t, "Ani")
			desk := r.testJoin(t, "Budi")
			for _, z := range r.mapd.Zones {
				drain(phone)
				drain(desk)
				clk.Advance(seatEvery)
				r.handle(seatCmd{c: phone, seq: 1, zone: z.Key})
				r.tick(clk.Now())
				if got := lastZone(t, phone); got.Key != z.Key {
					t.Fatalf("zone after seat %s = %+v", z.Key, got)
				}
				p := r.members[phone.userID].p
				if r.mapd.Blocked(p.X, p.Y) || !z.Area.Contains(p.X, p.Y) {
					t.Fatalf("seat %s at blocked/outside point %d,%d", z.Key, p.X, p.Y)
				}
				if len(drain(desk)[TypeSnapshot]) != 1 {
					t.Fatalf("map user did not see seat %s", z.Key)
				}
			}
			// Empty zone returns to the general area (spawn, no zone).
			clk.Advance(seatEvery)
			r.handle(seatCmd{c: phone, seq: 2})
			r.tick(clk.Now())
			if got := lastZone(t, phone); got.Key != "" {
				t.Fatalf("area umum zone = %+v", got)
			}
			p := r.members[phone.userID].p
			if !r.mapd.Spawn.Contains(p.X, p.Y) {
				t.Fatalf("area umum at %d,%d outside spawn", p.X, p.Y)
			}
		})
	}
}

func TestSeatRejections(t *testing.T) {
	r, clk := newMapRoom(t, "classroom", true)
	phone := r.listJoin(t, "Ani")
	desk := r.testJoin(t, "Budi")
	drain(phone)
	drain(desk)

	tests := []struct {
		name string
		c    *conn
		zone string
		want string
	}{
		{"map connection", desk, "meja-1", CodeForbidden},
		{"unknown zone", phone, "meja-99", CodeBadMessage},
		{"accepted", phone, "meja-1", ""},
		{"rate limited", phone, "meja-2", CodeRateLimited},
	}
	for _, tt := range tests {
		r.handle(seatCmd{c: tt.c, seq: 3, zone: tt.zone})
		got := errorsOf(t, tt.c)
		if (tt.want == "" && len(got) != 0) || (tt.want != "" && (len(got) != 1 || got[0] != tt.want)) {
			t.Errorf("%s: errors=%v want %q", tt.name, got, tt.want)
		}
	}
	clk.Advance(seatEvery)
	r.handle(seatCmd{c: phone, seq: 4, zone: "meja-2"})
	if got := errorsOf(t, phone); len(got) != 0 {
		t.Errorf("seat after a second: %v", got)
	}

	// List connections never move.
	m := r.members[phone.userID]
	x := m.p.X
	r.handle(moveCmd{c: phone, seq: 5, m: MoveMsg{X: x + 4, Y: m.p.Y, Dir: DirRight}})
	if got := errorsOf(t, phone); len(got) != 1 || got[0] != CodeMoveRejected || m.p.X != x {
		t.Errorf("list move: errors=%v x=%d", got, m.p.X)
	}
}

func TestSeatForbiddenOnCampus(t *testing.T) {
	r, _ := newMapRoom(t, "campus", true)
	phone := r.listJoin(t, "Ani")
	drain(phone)
	r.handle(seatCmd{c: phone, seq: 1, zone: ""})
	if got := errorsOf(t, phone); len(got) != 1 || got[0] != CodeForbidden {
		t.Errorf("campus seat errors=%v", got)
	}
}

func TestSeatZoneChatAndGather(t *testing.T) {
	r, clk := newMapRoom(t, "classroom", true)
	phone := r.listJoin(t, "Ani")
	desk := r.testJoin(t, "Budi")
	x, y, _ := r.mapd.SeatPoint("meja-3", 7)
	r.place(desk, x, y)
	r.handle(seatCmd{c: phone, seq: 1, zone: "meja-3"})
	r.tick(clk.Now())
	drain(desk)
	r.say(phone, ScopeZone, "halo meja")
	if got := chats(t, desk); len(got) != 1 || got[0].Zone != "meja-3" {
		t.Fatalf("zone chat from phone = %+v", got)
	}

	teacher := newConn(uuid.New())
	if err := r.join(teacher, Admission{UserID: teacher.userID, Name: "Bu Rina", Role: "teacher"}); err != nil {
		t.Fatal(err)
	}
	r.handle(commandCmd{c: teacher, seq: 1, name: "gather"})
	p := r.members[phone.userID].p
	if !r.mapd.Spawn.Contains(p.X, p.Y) {
		t.Errorf("gather left phone at %d,%d", p.X, p.Y)
	}
}

// TestHubTravelListModeSkipsDoor checks M8-03: list connections travel from
// anywhere, map connections still need the door, and capacity still applies.
func TestHubTravelListModeSkipsDoor(t *testing.T) {
	campus := loadCampus(t)
	interior := *campus
	interior.Key = "classroom"
	interior.ExitX, interior.ExitY = 480, 592
	hub := NewHub(map[string]*MapData{"campus": campus, "classroom": &interior}, nil, time.Now, slog.New(slog.NewTextHandler(io.Discard, nil)))
	t.Cleanup(func() { _ = hub.Shutdown(context.Background()) })

	origin := Admission{RoomID: uuid.New(), MapKey: "campus", Capacity: 200, Role: "participant"}
	dest := Admission{RoomID: uuid.New(), MapKey: "classroom", Capacity: 1, BuildingKey: "kelas-a", Role: "participant"}

	desk := newConn(uuid.New())
	deskOrigin := origin
	deskOrigin.UserID = desk.userID
	from, err := hub.join(desk, deskOrigin)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := hub.travel(desk, from, dest); !errors.Is(err, errNotAtDoor) {
		t.Fatalf("map travel away from door: %v", err)
	}

	phone := newConn(uuid.New())
	phone.list = true
	phoneOrigin := origin
	phoneOrigin.UserID = phone.userID
	if _, err := hub.join(phone, phoneOrigin); err != nil {
		t.Fatal(err)
	}
	phoneDest := dest
	phoneDest.UserID = phone.userID
	to, err := hub.travel(phone, from, phoneDest)
	if err != nil {
		t.Fatalf("list travel: %v", err)
	}

	second := newConn(uuid.New())
	second.list = true
	secondOrigin := origin
	secondOrigin.UserID = second.userID
	if _, err := hub.join(second, secondOrigin); err != nil {
		t.Fatal(err)
	}
	if _, err := hub.travel(second, from, dest); !errors.Is(err, errRoomFull) {
		t.Fatalf("list travel into full room: %v", err)
	}

	back, err := hub.travel(phone, to, phoneOrigin)
	if err != nil || back != from {
		t.Fatalf("list return: room=%v err=%v", back, err)
	}
}
