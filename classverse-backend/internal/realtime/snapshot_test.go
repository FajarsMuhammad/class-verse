package realtime

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
)

// Compare the optimized path with the original all-pairs visibility rules,
// including cell crossings, stationary receivers, reconnect and held avatars.
func TestSnapshotsMatchAllPairs(t *testing.T) {
	r := snapshotFixture(t, 100, true, false)
	for tick := 0; tick < 12; tick++ {
		i := 0
		for _, m := range r.members {
			if i%3 == tick%3 {
				m.p.X = (m.p.X + 300) % r.mapd.PixelWidth()
				m.p.Y = (m.p.Y + 200) % r.mapd.PixelHeight()
				m.dirty = true
			}
			i++
		}
		if tick == 3 {
			for _, m := range r.members {
				m.conn = nil
				m.heldUntil = r.now().Add(holdDuration)
				break
			}
		}
		if tick == 4 {
			for _, m := range r.members {
				m.seen = map[uuid.UUID]bool{}
				m.viewReady = false
				break
			}
		}
		expected := map[uuid.UUID]SnapshotMsg{}
		for id, recv := range r.members {
			if recv.conn == nil {
				continue
			}
			var snap SnapshotMsg
			for oid, o := range r.members {
				if id == oid {
					continue
				}
				visible := inAOI(r.mapd, recv.p.X, recv.p.Y, o.p.X, o.p.Y)
				if visible && (!recv.seen[oid] || o.dirty) {
					snap.P = append(snap.P, Position{ID: oid, X: o.p.X, Y: o.p.Y, Dir: o.p.Dir})
				}
				if !visible && recv.seen[oid] {
					snap.Out = append(snap.Out, oid)
				}
			}
			expected[id] = snap
		}
		r.tick(r.now())
		for id, m := range r.members {
			if m.conn == nil {
				continue
			}
			msgs := drain(m.conn)[TypeSnapshot]
			var got SnapshotMsg
			if len(msgs) > 1 {
				t.Fatal("more than one snapshot per tick")
			}
			if len(msgs) == 1 {
				if err := json.Unmarshal(msgs[0], &got); err != nil {
					t.Fatal(err)
				}
			}
			want := expected[id]
			if len(got.P) != len(want.P) || len(got.Out) != len(want.Out) {
				t.Fatalf("tick %d lengths got=%+v want=%+v", tick, got, want)
			}
			positions := map[uuid.UUID]Position{}
			for _, p := range got.P {
				positions[p.ID] = p
			}
			for _, p := range want.P {
				if positions[p.ID] != p {
					t.Fatalf("tick %d missing position %v", tick, p)
				}
			}
			out := map[uuid.UUID]bool{}
			for _, id := range got.Out {
				out[id] = true
			}
			for _, id := range want.Out {
				if !out[id] {
					t.Fatal("missing out")
				}
			}
		}
	}
}
