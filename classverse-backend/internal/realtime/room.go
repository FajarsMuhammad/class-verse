package realtime

import (
	"context"
	"log/slog"
	"math"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
)

const (
	tickInterval     = 100 * time.Millisecond
	holdDuration     = 30 * time.Second // reconnect grace period
	idleTimeout      = 60 * time.Second // empty campus room lifetime
	sessionIdle      = 5 * time.Minute  // empty classroom/garden: session ends, room stops
	buildingsEvery   = 50               // ticks (5 s)
	inboxSize        = 256
	campusKey        = "campus"
	doorRadius       = 48.0 // px from a door or exit point
	staffBuildingKey = "ruang-guru"
)

// Admission is what the gateway learned about a user and the room they want.
type Admission struct {
	RoomID      uuid.UUID
	OrgID       uuid.UUID
	MapKey      string
	Capacity    int
	RoomName    string
	BuildingKey string
	ScheduledAt *time.Time
	UserID      uuid.UUID
	Name        string
	Avatar      string
	Role        string
}

func (a Admission) staff() bool { return a.Role == "teacher" || a.Role == "admin" }

type joinCmd struct {
	c      *conn
	adm    Admission
	result chan error
}

type leaveCmd struct{ c *conn }

type profileCmd struct {
	p    ProfileUpdatedMsg
	done chan struct{}
}

// probeCmd runs fn inside the room goroutine (tests and the hub inspect state this way).
type probeCmd struct{ fn func(*room) }

type moveCmd struct {
	c   *conn
	seq uint64
	m   MoveMsg
}

// reserveCmd holds one capacity slot for a travelling user.
type reserveCmd struct {
	userID uuid.UUID
	result chan error
}

// cancelCmd releases a reservation after a failed travel.
type cancelCmd struct{ userID uuid.UUID }

// departCmd removes the user's avatar if check accepts its authoritative position.
type departCmd struct {
	c      *conn
	check  func(Participant) error
	result chan departResult
}

type departResult struct {
	p   Participant
	err error
}

// arriveCmd adds a travelling user at (x, y), consuming the reservation.
type arriveCmd struct {
	c      *conn
	adm    Admission
	x, y   int
	result chan error
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
	id          uuid.UUID
	orgID       uuid.UUID
	name        string
	buildingKey string
	mapd        *MapData
	campus      bool
	capacity    int
	now         func() time.Time
	log         *slog.Logger
	hub         *Hub

	inbox   chan any
	done    chan struct{}
	pending atomic.Int32 // joins and reservations posted by the hub but not yet settled

	members     map[uuid.UUID]*member
	reserved    map[uuid.UUID]bool
	spawnN      int
	emptySince  time.Time
	sessionOpen bool
	ticks       int
	lastStats   roomStats
}

func newRoom(adm Admission, mapd *MapData, hub *Hub, now func() time.Time, log *slog.Logger) *room {
	return &room{
		id: adm.RoomID, orgID: adm.OrgID, name: adm.RoomName, buildingKey: adm.BuildingKey,
		mapd: mapd, campus: mapd.Key == campusKey, capacity: adm.Capacity, now: now, log: log, hub: hub,
		inbox: make(chan any, inboxSize), done: make(chan struct{}),
		members: make(map[uuid.UUID]*member), reserved: make(map[uuid.UUID]bool),
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
	defer r.closeSession()
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
	case reserveCmd:
		err := r.reserve(c.userID)
		if err != nil {
			r.pending.Add(-1)
		}
		c.result <- err
	case cancelCmd:
		if r.reserved[c.userID] {
			delete(r.reserved, c.userID)
			r.pending.Add(-1)
		}
	case departCmd:
		p, err := r.depart(c.c, c.check)
		c.result <- departResult{p: p, err: err}
	case arriveCmd:
		if r.reserved[c.c.userID] {
			delete(r.reserved, c.c.userID)
			r.pending.Add(-1)
		}
		c.result <- r.add(c.c, c.adm, c.x, c.y, TypeTraveled)
	case profileCmd:
		if m := r.members[c.p.ID]; m != nil {
			m.p.Name, m.p.Avatar = c.p.Name, c.p.Avatar
			r.broadcast(newOut(TypeProfileUpdated, c.p), uuid.Nil)
		}
		close(c.done)
	case probeCmd:
		c.fn(r)
	}
}

var (
	errRoomFull  = &roomError{code: CodeRoomFull, msg: "Ruang sudah penuh"}
	errNotAtDoor = &roomError{code: CodeNotAtDoor, msg: "Berjalanlah ke pintu gedung dulu"}
	errNotInRoom = &roomError{code: CodeUnavailable, msg: "Kamu belum berada di ruang ini"}
)

type roomError struct{ code, msg string }

func (e *roomError) Error() string { return e.code }

// people counts avatars (connected and held) plus reservations.
func (r *room) people() int { return len(r.members) + len(r.reserved) }

func (r *room) join(c *conn, adm Admission) error {
	select {
	case <-c.done:
		return errUnavailable
	default:
	}
	if m, ok := r.members[c.userID]; ok {
		// Reconnect within the grace period, or a replacing tab: keep the
		// avatar where it is and tell nobody else.
		if m.conn != nil && m.conn != c {
			m.conn.close(closeReplaced, "replaced")
		}
		m.conn, m.heldUntil = c, time.Time{}
		m.p.Name, m.p.Avatar, m.p.Role = adm.Name, adm.Avatar, adm.Role
		m.seen = map[uuid.UUID]bool{}
		m.stride = newStride(r.mapd.WalkSpeed, r.now())
		r.sendWelcome(m, TypeWelcome)
		return nil
	}
	if r.people() >= r.capacity {
		return errRoomFull
	}
	x, y, err := r.mapd.SpawnPoint(r.spawnN)
	if err != nil {
		return err
	}
	r.spawnN++
	return r.add(c, adm, x, y, TypeWelcome)
}

// add places a new avatar, announces it, and greets its client with typ
// (welcome or traveled).
func (r *room) add(c *conn, adm Admission, x, y int, typ string) error {
	// A committed travel must retain its destination even if the socket
	// disconnects after departure. Keep the avatar for normal reconnect grace.
	m := &member{
		p:      Participant{ID: c.userID, Name: adm.Name, Avatar: adm.Avatar, Role: adm.Role, X: x, Y: y, Dir: DirDown},
		conn:   c,
		stride: newStride(r.mapd.WalkSpeed, r.now()),
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
	r.sendWelcome(m, typ)
	r.attend(c.userID)
	select {
	case <-c.done:
		r.leave(c)
	default:
	}
	return nil
}

func (r *room) sendWelcome(m *member, typ string) {
	w := WelcomeMsg{
		SelfID: m.p.ID, RoomID: r.id, MapKey: r.mapd.Key, RoomName: r.name, BuildingKey: r.buildingKey,
		Participants: make([]Participant, 0, len(r.members)),
	}
	for id, o := range r.members {
		w.Participants = append(w.Participants, o.p)
		if id != m.p.ID {
			m.seen[id] = true
		}
	}
	m.conn.send(newOut(typ, w))
	if r.campus {
		r.sendBuildings(m)
	}
}

func (r *room) reserve(userID uuid.UUID) error {
	if r.reserved[userID] {
		return &roomError{code: CodeTravelInProgress, msg: "Perpindahan sedang diproses"}
	}
	if _, ok := r.members[userID]; ok {
		return errNotInRoom
	}
	if r.people() >= r.capacity {
		return errRoomFull
	}
	r.reserved[userID] = true
	return nil
}

func (r *room) depart(c *conn, check func(Participant) error) (Participant, error) {
	m, ok := r.members[c.userID]
	if !ok || m.conn != c {
		return Participant{}, errNotInRoom
	}
	if err := check(m.p); err != nil {
		return Participant{}, err
	}
	p := m.p
	r.remove(c.userID)
	return p, nil
}

// remove deletes an avatar and tells everyone else.
func (r *room) remove(id uuid.UUID) {
	delete(r.members, id)
	for _, o := range r.members {
		delete(o.seen, id)
	}
	r.broadcast(newOut(TypeLeft, LeftMsg{ID: id}), uuid.Nil)
	r.hub.forget(id, r.id)
	if !r.campus {
		r.hub.record(sessionJob{kind: jobLeave, roomID: r.id, userID: id, at: r.now()})
	}
}

func (r *room) attend(userID uuid.UUID) {
	if r.campus {
		return
	}
	now := r.now()
	if !r.sessionOpen {
		r.sessionOpen = true
		r.hub.record(sessionJob{kind: jobOpen, roomID: r.id, at: now})
	}
	r.hub.record(sessionJob{kind: jobAttend, roomID: r.id, userID: userID, at: now})
}

func (r *room) closeSession() {
	if r.sessionOpen {
		r.sessionOpen = false
		r.hub.record(sessionJob{kind: jobClose, roomID: r.id, at: r.now()})
	}
	r.hub.setStats(r.id, roomStats{})
}

func (r *room) leave(c *conn) {
	m, ok := r.members[c.userID]
	if !ok || m.conn != c {
		return // a replaced or travelled connection; nothing to hold here
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

// tick expires held avatars, sends snapshots and building counts, and
// reports whether the room stopped because it stayed empty.
func (r *room) tick(now time.Time) (stopped bool) {
	for id, m := range r.members {
		if m.conn == nil && !now.Before(m.heldUntil) {
			r.remove(id)
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
			visible := !r.campus || inAOI(r.mapd, recv.p.X, recv.p.Y, o.p.X, o.p.Y)
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

	r.ticks++
	if r.campus && r.ticks%buildingsEvery == 0 {
		for _, m := range r.members {
			r.sendBuildings(m)
		}
	}

	stats := roomStats{orgID: r.orgID, buildingKey: r.buildingKey, people: r.people()}
	stats.live = r.sessionOpen && stats.people > 0
	if r.campus {
		stats = roomStats{} // the campus is not counted as a building
	}
	if stats != r.lastStats {
		r.lastStats = stats
		r.hub.setStats(r.id, stats)
	}

	if r.people() > 0 {
		r.emptySince = time.Time{}
		return false
	}
	if r.emptySince.IsZero() {
		r.emptySince = now
		return false
	}
	limit := idleTimeout
	if !r.campus {
		limit = sessionIdle
	}
	if now.Sub(r.emptySince) < limit {
		return false
	}
	r.closeSession()
	return r.hub.tryRemove(r)
}

func (r *room) sendBuildings(m *member) {
	if m.conn == nil {
		return
	}
	m.conn.send(newOut(TypeBuildings, BuildingsMsg{Items: r.hub.buildingCounts(r.orgID, m.p.Role)}))
}

// broadcast sends m to every connected member except skip.
func (r *room) broadcast(m outMsg, skip uuid.UUID) {
	for id, o := range r.members {
		if id != skip && o.conn != nil {
			o.conn.send(m)
		}
	}
}

// nearPoint returns a check that accepts avatars within doorRadius of (x, y).
func nearPoint(x, y int) func(Participant) error {
	return func(p Participant) error {
		if math.Hypot(float64(p.X-x), float64(p.Y-y)) > doorRadius {
			return errNotAtDoor
		}
		return nil
	}
}
