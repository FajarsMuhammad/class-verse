package realtime

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestHubArchiveAdmissionRace(t *testing.T) {
	hub := NewHub(map[string]*MapData{"campus": loadCampus(t)}, nil, time.Now, slog.New(slog.NewTextHandler(io.Discard, nil)))
	t.Cleanup(func() { _ = hub.Shutdown(context.Background()) })
	adm := Admission{RoomID: uuid.New(), MapKey: "campus", Capacity: 40}
	r, err := hub.ensure(adm)
	if err != nil {
		t.Fatal(err)
	}
	called := false
	if err := hub.WhileEmpty(t.Context(), adm.RoomID, func() error { called = true; return nil }); !errors.Is(err, ErrRoomBusy) || called {
		t.Fatalf("pending admission archive: %v called=%v", err, called)
	}
	r.pending.Add(-1)
	if err := hub.WhileEmpty(t.Context(), adm.RoomID, func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	if _, err := hub.ensure(adm); !errors.Is(err, ErrNoAccess) {
		t.Fatalf("stale admission after archive: %v", err)
	}
}

func TestHubTravelLastSlotRace(t *testing.T) {
	campus := loadCampus(t)
	interior := *campus
	interior.Key = "classroom"
	hub := NewHub(map[string]*MapData{"campus": campus, "classroom": &interior}, nil, time.Now, slog.New(slog.NewTextHandler(io.Discard, nil)))
	t.Cleanup(func() { _ = hub.Shutdown(context.Background()) })
	origin := Admission{RoomID: uuid.New(), MapKey: "campus", Capacity: 200, Role: "participant"}
	dest := Admission{RoomID: uuid.New(), OrgID: origin.OrgID, MapKey: "classroom", Capacity: 2, BuildingKey: "kelas-a", Role: "participant"}
	door, _ := campus.Building("kelas-a")

	// One seat already taken, one left.
	first := newConn(uuid.New())
	adm := dest
	adm.UserID = first.userID
	if _, err := hub.join(first, adm); err != nil {
		t.Fatal(err)
	}

	const racers = 8
	conns := make([]*conn, racers)
	var from *room
	for i := range conns {
		c := newConn(uuid.New())
		a := origin
		a.UserID = c.userID
		r, err := hub.join(c, a)
		if err != nil {
			t.Fatal(err)
		}
		from = r
		conns[i] = c
	}
	done := make(chan struct{})
	from.post(probeCmd{fn: func(r *room) {
		for _, c := range conns {
			r.members[c.userID].p.X, r.members[c.userID].p.Y = door.DoorX, door.DoorY
		}
		close(done)
	}})
	<-done

	errs := make(chan error, racers)
	for _, c := range conns {
		go func() {
			d := dest
			d.UserID = c.userID
			_, err := hub.travel(c, from, d)
			errs <- err
		}()
	}
	ok, full := 0, 0
	for range racers {
		err := <-errs
		switch {
		case err == nil:
			ok++
		case errors.Is(err, errRoomFull):
			full++
		default:
			t.Errorf("unexpected travel error: %v", err)
		}
	}
	if ok != 1 || full != racers-1 {
		t.Fatalf("ok=%d full=%d, want 1 and %d", ok, full, racers-1)
	}
	counts := make(chan [2]int, 1)
	from.post(probeCmd{fn: func(r *room) {
		var c [2]int
		c[0] = r.people()
		counts <- c
	}})
	if got := (<-counts)[0]; got != racers-1 {
		t.Fatalf("campus keeps %d avatars, want %d (losers must stay)", got, racers-1)
	}
	if n, _ := hub.Occupancy(dest.RoomID); n > 2 {
		t.Fatalf("destination over capacity: %d", n)
	}
}

// failingSessions fails every write, counting calls; it must never block rooms.
type failingSessions struct{ calls chan string }

func (f failingSessions) OpenSession(context.Context, uuid.UUID, time.Time) (uuid.UUID, error) {
	f.calls <- "open"
	return uuid.Nil, errors.New("db down")
}
func (f failingSessions) CloseSession(context.Context, uuid.UUID, time.Time) error {
	f.calls <- "close"
	return errors.New("db down")
}
func (f failingSessions) Attend(context.Context, uuid.UUID, uuid.UUID, time.Time) error {
	f.calls <- "attend"
	return errors.New("db down")
}
func (f failingSessions) Leave(context.Context, uuid.UUID, uuid.UUID, time.Time) error {
	f.calls <- "leave"
	return errors.New("db down")
}

func TestHubSessionWriteFailureDoesNotBlock(t *testing.T) {
	campus := loadCampus(t)
	interior := *campus
	interior.Key = "classroom"
	calls := make(chan string, 16)
	hub := NewHub(map[string]*MapData{"classroom": &interior}, failingSessions{calls}, time.Now, slog.New(slog.NewTextHandler(io.Discard, nil)))
	c := newConn(uuid.New())
	adm := Admission{RoomID: uuid.New(), MapKey: "classroom", Capacity: 40, UserID: c.userID}
	r, err := hub.join(c, adm)
	if err != nil {
		t.Fatal(err)
	}
	if got := <-calls; got != "open" {
		t.Fatalf("first job = %s", got)
	}
	// Room keeps serving commands after the failed write.
	alive := make(chan struct{})
	if !r.post(probeCmd{fn: func(*room) { close(alive) }}) {
		t.Fatal("room stopped after session failure")
	}
	select {
	case <-alive:
	case <-time.After(time.Second):
		t.Fatal("room loop blocked by session failure")
	}
	// Without a session id attendance is skipped; shutdown still closes and drains.
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := hub.Shutdown(ctx); err != nil {
		t.Fatalf("shutdown: %v", err)
	}
	if got := <-calls; got != "close" {
		t.Fatalf("shutdown job = %s, want close", got)
	}
}

func TestHubTravelDoorAndReturn(t *testing.T) {
	campus := loadCampus(t)
	interior := *campus
	interior.Key = "classroom"
	interior.ExitX, interior.ExitY = 480, 592
	hub := NewHub(map[string]*MapData{"campus": campus, "classroom": &interior}, nil, time.Now, slog.New(slog.NewTextHandler(io.Discard, nil)))
	t.Cleanup(func() { _ = hub.Shutdown(context.Background()) })
	c := newConn(uuid.New())
	origin := Admission{RoomID: uuid.New(), MapKey: "campus", Capacity: 200, UserID: c.userID, Role: "participant"}
	from, err := hub.join(c, origin)
	if err != nil {
		t.Fatal(err)
	}
	dest := origin
	dest.RoomID, dest.MapKey, dest.Capacity = uuid.New(), "classroom", 40
	dest.BuildingKey = "kelas-a"
	door, ok := campus.Building(dest.BuildingKey)
	if !ok {
		t.Fatal("missing test building")
	}
	if _, err := hub.travel(c, from, dest); !errors.Is(err, errNotAtDoor) {
		t.Fatalf("distant travel error=%v", err)
	}
	probe := func(r *room, fn func(*room)) {
		t.Helper()
		done := make(chan struct{})
		if !r.post(probeCmd{fn: func(r *room) { fn(r); close(done) }}) {
			t.Fatal("room stopped")
		}
		<-done
	}
	probe(from, func(r *room) {
		m := r.members[c.userID]
		if m == nil {
			t.Error("failed travel removed origin presence")
			return
		}
		m.p.X, m.p.Y = door.DoorX, door.DoorY
	})
	to, err := hub.travel(c, from, dest)
	if err != nil {
		t.Fatal(err)
	}
	probe(from, func(r *room) {
		if r.members[c.userID] != nil {
			t.Error("origin presence retained")
		}
	})
	probe(to, func(r *room) {
		m := r.members[c.userID]
		if m == nil {
			t.Error("destination presence missing")
			return
		}
		m.p.X, m.p.Y = interior.ExitX, interior.ExitY
	})
	returned, err := hub.travel(c, to, origin)
	if err != nil {
		t.Fatal(err)
	}
	if returned != from {
		t.Fatal("returned to wrong room")
	}
	probe(returned, func(r *room) {
		m := r.members[c.userID]
		if m == nil || m.p.X != door.DoorX || m.p.Y != door.DoorY {
			t.Error("return did not restore building door position")
		}
	})
}
