package realtime

import (
	"context"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
)

const (
	tickInterval = 100 * time.Millisecond
	holdDuration = 30 * time.Second // reconnect grace period
	idleTimeout  = 60 * time.Second // empty room lifetime
	inboxSize    = 256
)

// Admission is what the gateway learned about a user and the room they want.
type Admission struct {
	RoomID   uuid.UUID
	OrgID    uuid.UUID
	MapKey   string
	Capacity int
	UserID   uuid.UUID
	Name     string
	Avatar   string
	Role     string
}

type joinCmd struct {
	c      *conn
	adm    Admission
	result chan error
}

type leaveCmd struct{ c *conn }

// probeCmd runs fn inside the room goroutine (tests only inspect state this way).
type probeCmd struct{ fn func(*room) }

type moveCmd struct {
	c   *conn
	seq uint64
	m   MoveMsg
}

type member struct {
	p         Participant
	conn      *conn // nil while held after a disconnect
	heldUntil time.Time
	stride    stride
	moves     moveBudget
	dirty     bool
	seen      map[uuid.UUID]bool // ids the client currently shows (area of interest)
}

// room owns the state of one room. Only its goroutine touches the fields
// below inbox; other goroutines talk to it through post.
type room struct {
	id       uuid.UUID
	mapd     *MapData
	aoi      bool
	capacity int
	now      func() time.Time
	log      *slog.Logger
	hub      *Hub

	inbox   chan any
	done    chan struct{}
	pending atomic.Int32 // joins posted by the hub but not yet handled

	members    map[uuid.UUID]*member
	spawnN     int
	emptySince time.Time
}

func newRoom(id uuid.UUID, mapd *MapData, capacity int, hub *Hub, now func() time.Time, log *slog.Logger) *room {
	return &room{
		id: id, mapd: mapd, aoi: mapd.Key == "campus", capacity: capacity, now: now, log: log, hub: hub,
		inbox: make(chan any, inboxSize), done: make(chan struct{}), members: make(map[uuid.UUID]*member),
	}
}

// post delivers cmd to the room; it returns false if the room has stopped.
func (r *room) post(cmd any) bool {
	select {
	case r.inbox <- cmd:
		return true
	case <-r.done:
		return false
	}
}

func (r *room) run(ctx context.Context) {
	defer close(r.done)
	t := time.NewTicker(tickInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case cmd := <-r.inbox:
			r.handle(cmd)
		case <-t.C:
			if r.tick(r.now()) {
				return
			}
		}
	}
}

func (r *room) handle(cmd any) {
	switch c := cmd.(type) {
	case joinCmd:
		r.pending.Add(-1)
		c.result <- r.join(c.c, c.adm)
	case leaveCmd:
		r.leave(c.c)
	case moveCmd:
		r.move(c.c, c.seq, c.m)
	case probeCmd:
		c.fn(r)
	}
}

var errRoomFull = &roomError{code: CodeRoomFull, msg: "Ruang sudah penuh"}

type roomError struct{ code, msg string }

func (e *roomError) Error() string { return e.code }

func (r *room) join(c *conn, adm Admission) error {
	select {
	case <-c.done:
		return errUnavailable
	default:
	}
	now := r.now()
	if m, ok := r.members[c.userID]; ok {
		// Reconnect within the grace period, or a replacing tab: keep the
		// avatar where it is and tell nobody else.
		if m.conn != nil && m.conn != c {
			m.conn.close(closeReplaced, "replaced")
		}
		m.conn, m.heldUntil = c, time.Time{}
		m.p.Name, m.p.Avatar, m.p.Role = adm.Name, adm.Avatar, adm.Role
		m.seen = map[uuid.UUID]bool{}
		m.stride = newStride(now)
		r.sendWelcome(m)
		return nil
	}
	if len(r.members) >= r.capacity {
		return errRoomFull
	}
	x, y, err := r.mapd.SpawnPoint(r.spawnN)
	if err != nil {
		return err
	}
	r.spawnN++
	m := &member{
		p:      Participant{ID: c.userID, Name: adm.Name, Avatar: adm.Avatar, Role: adm.Role, X: x, Y: y, Dir: DirDown},
		conn:   c,
		stride: newStride(now),
		seen:   map[uuid.UUID]bool{},
	}
	r.members[c.userID] = m
	r.broadcast(newOut(TypeJoined, m.p), c.userID)
	// joined includes a position. Track it as seen even outside AOI so the
	// next tick explicitly hides it with out.
	for id, other := range r.members {
		if id != c.userID && other.conn != nil {
			other.seen[c.userID] = true
		}
	}
	r.sendWelcome(m)
	return nil
}

func (r *room) sendWelcome(m *member) {
	w := WelcomeMsg{SelfID: m.p.ID, RoomID: r.id, MapKey: r.mapd.Key, Participants: make([]Participant, 0, len(r.members))}
	for id, o := range r.members {
		w.Participants = append(w.Participants, o.p)
		if id != m.p.ID {
			m.seen[id] = true
		}
	}
	m.conn.send(newOut(TypeWelcome, w))
}

func (r *room) leave(c *conn) {
	m, ok := r.members[c.userID]
	if !ok || m.conn != c {
		return // a replaced connection; the new one owns the avatar
	}
	m.conn = nil
	m.heldUntil = r.now().Add(holdDuration)
}

func (r *room) move(c *conn, seq uint64, mv MoveMsg) {
	m, ok := r.members[c.userID]
	if !ok || m.conn != c {
		return
	}
	now := r.now()
	if !m.moves.allow(now) {
		return // flood: drop silently
	}
	if !mv.Dir.valid() {
		c.sendError(ErrorMsg{Code: CodeBadMessage, Message: "Arah tidak valid", Seq: seq})
		return
	}
	if !m.stride.try(r.mapd, m.p.X, m.p.Y, mv.X, mv.Y, now) {
		x, y := m.p.X, m.p.Y
		c.sendError(ErrorMsg{Code: CodeMoveRejected, Message: "Posisi ditolak", Seq: seq, X: &x, Y: &y})
		return
	}
	if mv.X != m.p.X || mv.Y != m.p.Y || mv.Dir != m.p.Dir {
		m.p.X, m.p.Y, m.p.Dir = mv.X, mv.Y, mv.Dir
		m.dirty = true
	}
}

// tick expires held avatars, sends snapshots, and reports whether the room
// stopped because it stayed empty.
func (r *room) tick(now time.Time) (stopped bool) {
	for id, m := range r.members {
		if m.conn == nil && !now.Before(m.heldUntil) {
			delete(r.members, id)
			for _, o := range r.members {
				delete(o.seen, id)
			}
			r.broadcast(newOut(TypeLeft, LeftMsg{ID: id}), uuid.Nil)
		}
	}

	for _, recv := range r.members {
		if recv.conn == nil {
			continue
		}
		var snap SnapshotMsg
		for id, o := range r.members {
			if id == recv.p.ID {
				continue
			}
			visible := !r.aoi || inAOI(r.mapd, recv.p.X, recv.p.Y, o.p.X, o.p.Y)
			switch {
			case visible && (!recv.seen[id] || o.dirty):
				recv.seen[id] = true
				snap.P = append(snap.P, Position{ID: id, X: o.p.X, Y: o.p.Y, Dir: o.p.Dir})
			case !visible && recv.seen[id]:
				delete(recv.seen, id)
				snap.Out = append(snap.Out, id)
			}
		}
		if len(snap.P) > 0 || len(snap.Out) > 0 {
			if snap.P == nil {
				snap.P = []Position{}
			}
			recv.conn.send(newOut(TypeSnapshot, snap))
		}
	}
	for _, m := range r.members {
		m.dirty = false
	}

	if len(r.members) > 0 {
		r.emptySince = time.Time{}
		return false
	}
	if r.emptySince.IsZero() {
		r.emptySince = now
		return false
	}
	if now.Sub(r.emptySince) >= idleTimeout && r.hub.tryRemove(r) {
		return true
	}
	return false
}

// broadcast sends m to every connected member except skip.
func (r *room) broadcast(m outMsg, skip uuid.UUID) {
	for id, o := range r.members {
		if id != skip && o.conn != nil {
			o.conn.send(m)
		}
	}
}
