package realtime

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/FajarsMuhammad/class-verse/classverse-backend/maps"
)

// snapshotFixture uses the production map, not the smaller MVP1 fixture.
func snapshotFixture(tb testing.TB, n int, campus, crowded bool) *room {
	tb.Helper()
	raw, err := maps.Read("campus")
	if err != nil {
		tb.Fatal(err)
	}
	md, err := ParseMap("campus", raw)
	if err != nil {
		tb.Fatal(err)
	}
	if !campus {
		md.Key = "classroom"
	}
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	hub := NewHub(map[string]*MapData{md.Key: md}, nil, func() time.Time { return now }, log)
	tb.Cleanup(func() { _ = hub.Shutdown(context.Background()) })
	r := newRoom(Admission{RoomID: uuid.New(), Capacity: n}, md, hub, func() time.Time { return now }, log)
	for i := 0; i < n; i++ {
		id := uuid.New()
		x, y := md.PixelWidth()/2, md.PixelHeight()/2
		if !crowded {
			x = (i % 20) * md.PixelWidth() / 20
			y = (i / 20) * md.PixelHeight() / ((n + 19) / 20)
		}
		r.members[id] = &member{p: Participant{ID: id, X: x, Y: y, Dir: DirDown}, conn: newConn(id), seen: map[uuid.UUID]bool{}}
	}
	// Warm visibility sets; transport is drained synchronously to isolate tick cost.
	r.tick(now)
	drainSnapshots(r)
	return r
}

func drainSnapshots(r *room) int {
	bytes := 0
	for _, m := range r.members {
		for len(m.conn.out) > 0 {
			bytes += len((<-m.conn.out).d)
		}
	}
	return bytes
}

func BenchmarkRoomTick(b *testing.B) {
	for _, tc := range []struct {
		name                    string
		n                       int
		campus, crowded, moving bool
	}{
		{"campus-spread-idle-500", 500, true, false, false},
		{"campus-spread-moving-500", 500, true, false, true},
		{"campus-crowded-moving-500", 500, true, true, true},
		{"classroom-moving-50", 50, false, true, true},
	} {
		b.Run(tc.name, func(b *testing.B) {
			r := snapshotFixture(b, tc.n, tc.campus, tc.crowded)
			b.ReportAllocs()
			for b.Loop() {
				if tc.moving {
					for _, m := range r.members {
						m.dirty = true
					}
				}
				r.tick(r.now())
				drainSnapshots(r)
			}
		})
	}
}

func TestSnapshotScale(t *testing.T) {
	for _, n := range []int{50, 100, 500} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			r := snapshotFixture(t, n, true, false)
			for _, m := range r.members {
				m.dirty = true
			}
			r.tick(r.now())
			bytes := drainSnapshots(r)
			if bytes == 0 {
				t.Fatal("moving participants produced no snapshots")
			}
			for _, m := range r.members {
				select {
				case <-m.conn.done:
					t.Fatal("healthy receiver disconnected")
				default:
				}
			}
			t.Logf("%d people, JSON payload=%d bytes/tick (no envelope/transport)", n, bytes)
		})
	}
}
