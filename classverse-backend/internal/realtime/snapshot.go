package realtime

import "github.com/google/uuid"

// snapshotPosition keeps the wire shape while avoiding UUID.MarshalText per recipient.
type snapshotPosition struct {
	ID  string    `json:"id"`
	X   int       `json:"x"`
	Y   int       `json:"y"`
	Dir Direction `json:"dir"`
}

// sendSnapshots rebuilds a room-local spatial index only when positions or
// membership changed. No index maintenance is needed in every movement path.
func (r *room) sendSnapshots() {
	changed := false
	for _, m := range r.members {
		if m.dirty || !m.viewReady {
			changed = true
			break
		}
	}
	if !changed {
		return
	}
	var grid map[cell][]*member
	if r.campus {
		grid = make(map[cell][]*member)
		for _, m := range r.members {
			c := cellOf(r.mapd, m.p.X, m.p.Y)
			grid[c] = append(grid[c], m)
		}
	}
	// Convert each UUID once per tick, not once per recipient.
	positions := make(map[uuid.UUID]snapshotPosition, len(r.members))
	for _, recv := range r.members {
		if recv.conn == nil {
			continue
		}
		var snap struct {
			P   []snapshotPosition `json:"p"`
			Out []uuid.UUID        `json:"out,omitempty"`
		}
		appendPosition := func(o *member) {
			id := o.p.ID
			if id != recv.p.ID && (!recv.seen[id] || o.dirty) {
				if snap.P == nil {
					snap.P = make([]snapshotPosition, 0, len(recv.seen)+1)
				}
				recv.seen[id] = true
				position, ok := positions[id]
				if !ok {
					position = snapshotPosition{ID: id.String(), X: o.p.X, Y: o.p.Y, Dir: o.p.Dir}
					positions[id] = position
				}
				snap.P = append(snap.P, position)
			}
		}
		if r.campus {
			center := cellOf(r.mapd, recv.p.X, recv.p.Y)
			radius := aoiRadiusTiles / aoiCellTiles
			for y := center.cy - radius; y <= center.cy+radius; y++ {
				for x := center.cx - radius; x <= center.cx+radius; x++ {
					for _, o := range grid[cell{x, y}] {
						appendPosition(o)
					}
				}
			}
			for id := range recv.seen {
				o := r.members[id]
				if o == nil || !inAOI(r.mapd, recv.p.X, recv.p.Y, o.p.X, o.p.Y) {
					delete(recv.seen, id)
					snap.Out = append(snap.Out, id)
				}
			}
		} else {
			for _, o := range r.members {
				appendPosition(o)
			}
		}
		if len(snap.P) > 0 || len(snap.Out) > 0 {
			if snap.P == nil {
				snap.P = []snapshotPosition{}
			}
			recv.conn.send(newOut(TypeSnapshot, snap))
		}
	}
	for _, m := range r.members {
		m.dirty = false
		m.viewReady = true
	}
}
