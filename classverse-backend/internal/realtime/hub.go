package realtime

import (
	"context"
	"errors"
	"log/slog"
	"math"
	"sort"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/google/uuid"
)

var errUnavailable = errors.New("room unavailable")

const sessionQueue = 1024

// SessionRecorder persists sessions and attendance (implemented by Store).
// Calls run on the hub worker, never on a room goroutine.
type SessionRecorder interface {
	OpenSession(ctx context.Context, roomID uuid.UUID, at time.Time) (uuid.UUID, error)
	CloseSession(ctx context.Context, roomID uuid.UUID, at time.Time) error
	Attend(ctx context.Context, sessionID, userID uuid.UUID, at time.Time) error
	Leave(ctx context.Context, sessionID, userID uuid.UUID, at time.Time) error
	// SaveChat stores a filtered message in the room's open session and
	// returns its id; ok is false when the room has no open session.
	SaveChat(ctx context.Context, roomID uuid.UUID, msg ChatOutMsg) (id int64, ok bool, err error)
}

// Moderator persists session bans with their audit row (implemented by Store).
// Calls run on the hub worker so they stay ordered behind session jobs.
type Moderator interface {
	Kick(ctx context.Context, roomID, actorID, targetID uuid.UUID, at time.Time) (bool, error)
	Allow(ctx context.Context, roomID, actorID, targetID uuid.UUID, at time.Time) (bool, error)
	Banned(ctx context.Context, roomID uuid.UUID) ([]BannedUser, error)
	RecordAction(ctx context.Context, roomID, actorID, targetID uuid.UUID, action string, at time.Time) (bool, error)
}

// modJob runs one moderation write; done runs on the worker and must not block.
type modJob struct {
	action string // kick, allow, mute, unmute, lock, unlock
	roomID uuid.UUID
	actor  uuid.UUID
	target uuid.UUID
	at     time.Time
	done   func(ok bool, banned []BannedUser, err error)
}

type jobKind int

const (
	jobOpen jobKind = iota
	jobAttend
	jobLeave
	jobClose
	jobChat
	jobMod
)

type sessionJob struct {
	kind   jobKind
	roomID uuid.UUID
	userID uuid.UUID
	at     time.Time
	chat   *chatJob
	mod    *modJob
}

// chatJob stores one message; deliver runs on the worker with the stored id
// (0 when not stored) and must not block.
type chatJob struct {
	roomID  uuid.UUID
	msg     ChatOutMsg
	deliver func(id int64)
}

// storeChat queues a chat message behind the room's session jobs, so it is
// written after the session that it belongs to was opened. When storing is
// unavailable the message is still delivered, without an id.
func (h *Hub) storeChat(j chatJob) {
	if h.sessions == nil {
		j.deliver(0)
		return
	}
	select {
	case h.jobs <- sessionJob{kind: jobChat, roomID: j.roomID, chat: &j}:
	default:
		h.log.Warn("chat store queue full", slog.String("room_id", j.roomID.String()))
		j.deliver(0)
	}
}

// moderate queues a ban/allow write behind the room's session jobs. Without
// persistence the action fails (an unaudited kick must not succeed).
func (h *Hub) moderate(j modJob) {
	if h.sessions == nil {
		j.done(false, nil, errUnavailable)
		return
	}
	select {
	case h.jobs <- sessionJob{kind: jobMod, roomID: j.roomID, mod: &j}:
	default:
		j.done(false, nil, errUnavailable)
	}
}

// roomStats is what the campus needs to know about a classroom or garden.
type roomStats struct {
	orgID       uuid.UUID
	buildingKey string
	people      int
	live        bool
}

// location is the last committed room of a user, kept for reconnects.
type location struct {
	roomID uuid.UUID
	until  time.Time // zero while connected
}

// Hub is the registry of running rooms and of the one live connection per
// user. Rooms are started on first join and stop themselves when idle.
type Hub struct {
	ctx      context.Context
	cancel   context.CancelFunc
	maps     map[string]*MapData
	now      func() time.Time
	log      *slog.Logger
	sessions SessionRecorder
	jobs     chan sessionJob

	mu            sync.Mutex
	rooms         map[uuid.UUID]*room
	conns         map[uuid.UUID]*conn
	where         map[uuid.UUID]location
	stats         map[uuid.UUID]roomStats
	traveling     map[uuid.UUID]bool
	locked        map[uuid.UUID]bool // rooms being archived
	wg            sync.WaitGroup     // room goroutines
	worker        chan struct{}      // closed when recordLoop returns
	voice         VoiceAdmin         // nil when voice is off; set before serving
	voiceJobs     chan func(context.Context)
	voiceWorker   chan struct{}
	voiceRevoking map[uuid.UUID]int // protected by mu, spans travel between room actors
}

// VoiceAdmin applies room moderation to LiveKit (implemented by voice.LiveKit).
// Calls run on the hub's voice worker, never on a room goroutine.
type VoiceAdmin interface {
	SetCanPublish(ctx context.Context, room, identity string, canPublish bool) error
	Remove(ctx context.Context, room, identity string) error
}

// SetVoice enables LiveKit moderation hooks. Call before serving.
func (h *Hub) SetVoice(v VoiceAdmin) {
	h.voice = v
	h.voiceJobs = make(chan func(context.Context), sessionQueue)
	h.voiceWorker = make(chan struct{})
	go func() {
		defer close(h.voiceWorker)
		for job := range h.voiceJobs {
			job(h.ctx)
		}
	}()
}

// VoiceMember is what the voice token endpoint needs about a room member.
type VoiceMember struct {
	Name         string
	Muted        bool
	Identity     string
	Channel      string
	Broadcasting bool
}

// ErrNotInRoom means the user has no live connection in that classroom or garden.
var ErrNotInRoom = errors.New("not connected to this room")

// ErrVoicePending means an old media admission has not been evicted yet.
var ErrVoicePending = errors.New("voice revocation pending")

// VoiceMember reports whether userID is connected to roomID right now
// (campus excluded). Admission rules were applied when the user entered.
func (h *Hub) VoiceMember(ctx context.Context, roomID, userID uuid.UUID) (VoiceMember, error) {
	h.mu.Lock()
	r := h.rooms[roomID]
	pending := h.voiceRevoking[userID] > 0
	h.mu.Unlock()
	if pending {
		return VoiceMember{}, ErrVoicePending
	}
	if r == nil || r.campus {
		return VoiceMember{}, ErrNotInRoom
	}
	res := make(chan VoiceMember, 1)
	probe := probeCmd{fn: func(r *room) {
		m := r.members[userID]
		if m == nil || m.conn == nil {
			close(res)
			return
		}
		res <- VoiceMember{Name: m.p.Name, Muted: r.muted[userID], Identity: m.voiceIdentity,
			Channel: m.voiceChannel, Broadcasting: r.voiceOn[userID]}
	}}
	select {
	case r.inbox <- probe:
	case <-r.done:
		return VoiceMember{}, ErrNotInRoom
	case <-ctx.Done():
		return VoiceMember{}, ctx.Err()
	}
	select {
	case m, ok := <-res:
		if !ok {
			return VoiceMember{}, ErrNotInRoom
		}
		return m, nil
	case <-r.done:
		return VoiceMember{}, ErrNotInRoom
	case <-ctx.Done():
		return VoiceMember{}, ctx.Err()
	}
}

// NewHub returns an empty hub. sessions may be nil (tests). Call Shutdown to stop it.
func NewHub(maps map[string]*MapData, sessions SessionRecorder, now func() time.Time, log *slog.Logger) *Hub {
	ctx, cancel := context.WithCancel(context.Background())
	h := &Hub{
		ctx: ctx, cancel: cancel, maps: maps, now: now, log: log, sessions: sessions,
		jobs:  make(chan sessionJob, sessionQueue),
		rooms: map[uuid.UUID]*room{}, conns: map[uuid.UUID]*conn{}, where: map[uuid.UUID]location{},
		stats: map[uuid.UUID]roomStats{}, traveling: map[uuid.UUID]bool{}, locked: map[uuid.UUID]bool{},
	}
	h.worker = make(chan struct{})
	go func() {
		defer close(h.worker)
		h.recordLoop()
	}()
	return h
}

// claim makes c the user's only live connection, closing any older one.
func (h *Hub) claim(c *conn) {
	h.mu.Lock()
	if h.ctx.Err() != nil {
		h.mu.Unlock()
		c.close(websocket.StatusGoingAway, "server shutting down")
		return
	}
	old := h.conns[c.userID]
	h.conns[c.userID] = c
	h.mu.Unlock()
	if old != nil && old != c {
		old.close(closeReplaced, "replaced")
	}
}

// release forgets c and starts the reconnect grace period for its room.
func (h *Hub) release(c *conn) {
	h.mu.Lock()
	if h.conns[c.userID] == c {
		delete(h.conns, c.userID)
		if loc, ok := h.where[c.userID]; ok {
			loc.until = h.now().Add(holdDuration)
			h.where[c.userID] = loc
		}
	}
	h.mu.Unlock()
}

// Resume returns the room a reconnecting user should rejoin: their last
// committed room while it is still held, else uuid.Nil.
func (h *Hub) resume(userID uuid.UUID) uuid.UUID {
	h.mu.Lock()
	defer h.mu.Unlock()
	loc, ok := h.where[userID]
	if !ok || (!loc.until.IsZero() && h.now().After(loc.until)) {
		return uuid.Nil
	}
	return loc.roomID
}

// forget is called by a room when the user's avatar is gone from it.
func (h *Hub) forget(userID, roomID uuid.UUID) {
	h.mu.Lock()
	if loc, ok := h.where[userID]; ok && loc.roomID == roomID {
		delete(h.where, userID)
	}
	h.mu.Unlock()
}

func (h *Hub) setLocation(userID, roomID uuid.UUID) {
	h.mu.Lock()
	h.where[userID] = location{roomID: roomID}
	h.mu.Unlock()
}

// ensure returns the running room for adm, starting it if needed, with one
// pending slot counted so it cannot stop before the caller posts.
func (h *Hub) ensure(adm Admission) (*room, error) {
	mapd := h.maps[adm.MapKey]
	if mapd == nil {
		return nil, errUnavailable
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.ctx.Err() != nil {
		return nil, errUnavailable
	}
	if h.locked[adm.RoomID] {
		return nil, ErrNoAccess
	}
	r := h.rooms[adm.RoomID]
	if r == nil {
		r = newRoom(adm, mapd, h, h.now, h.log)
		h.rooms[adm.RoomID] = r
		h.wg.Go(func() { r.run(h.ctx) })
	}
	r.pending.Add(1)
	return r, nil
}

// join adds c to the room of adm, starting the room if needed.
func (h *Hub) join(c *conn, adm Admission) (*room, error) {
	r, err := h.ensure(adm)
	if err != nil {
		return nil, err
	}
	res := make(chan error, 1)
	if !r.post(joinCmd{c: c, adm: adm, result: res}) {
		r.pending.Add(-1)
		return nil, errUnavailable
	}
	select {
	case err := <-res:
		if err != nil {
			return nil, err
		}
		h.setLocation(c.userID, adm.RoomID)
		return r, nil
	case <-r.done:
		return nil, errUnavailable
	}
}

// travel moves c from room `from` to the room of dest. It never leaves the
// user without a room: the destination slot is reserved first, the origin
// releases the avatar only if it stands at the right point, and the
// reservation is cancelled on any failure.
func (h *Hub) travel(c *conn, from *room, dest Admission) (*room, error) {
	h.mu.Lock()
	if h.traveling[c.userID] {
		h.mu.Unlock()
		return nil, &roomError{code: CodeTravelInProgress, msg: "Perpindahan sedang diproses"}
	}
	h.traveling[c.userID] = true
	h.mu.Unlock()
	defer func() {
		h.mu.Lock()
		delete(h.traveling, c.userID)
		h.mu.Unlock()
	}()

	if dest.RoomID == from.id {
		return nil, errNotInRoom
	}
	// Where the avatar must stand, and where it appears.
	var check func(Participant) error
	var x, y int
	destMap := h.maps[dest.MapKey]
	if destMap == nil {
		return nil, errUnavailable
	}
	switch {
	case from.campus && dest.MapKey != campusKey:
		b, ok := from.mapd.Building(dest.BuildingKey)
		if !ok {
			return nil, ErrNoAccess
		}
		check = func(p Participant) error {
			radius := b.InteractRadius
			if radius == 0 {
				radius = 48
			}
			if math.Hypot(float64(p.X-b.DoorX), float64(p.Y-b.DoorY)) > float64(radius) {
				return errNotAtDoor
			}
			return nil
		}
		var err error
		if x, y, err = destMap.SpawnPoint(int(c.userID[0])); err != nil {
			return nil, err
		}
	case !from.campus && dest.MapKey == campusKey:
		check = nearPoint(from.mapd.ExitX, from.mapd.ExitY)
		b, ok := destMap.Building(from.buildingKey)
		if !ok {
			return nil, errUnavailable
		}
		x, y = b.DoorX, b.DoorY
	default:
		return nil, errNotAtDoor // room to room goes through the campus
	}
	if c.list {
		// Mode HP cannot walk; the door is presence UX, not access control
		// (admission, lock, ban and capacity still apply). Design M8-03.
		check = func(Participant) error { return nil }
	}

	to, err := h.ensure(dest)
	if err != nil {
		return nil, err
	}
	res := make(chan error, 1)
	if !to.post(reserveCmd{userID: c.userID, staff: dest.staff(), result: res}) {
		to.pending.Add(-1)
		return nil, errUnavailable
	}
	if err := waitErr(res, to.done); err != nil {
		return nil, err
	}
	// The reservation now holds the pending count until arrive or cancel.

	dep := make(chan departResult, 1)
	if !from.post(departCmd{c: c, check: check, result: dep}) {
		to.post(cancelCmd{userID: c.userID})
		return nil, errUnavailable
	}
	var d departResult
	select {
	case d = <-dep:
	case <-from.done:
		d.err = errUnavailable
	}
	if d.err != nil {
		to.post(cancelCmd{userID: c.userID})
		return nil, d.err
	}

	h.setLocation(c.userID, to.id)
	arr := make(chan error, 1)
	if !to.post(arriveCmd{c: c, adm: dest, x: x, y: y, result: arr}) {
		return nil, errUnavailable
	}
	if err := waitErr(arr, to.done); err != nil {
		return nil, err
	}
	return to, nil
}

func waitErr(ch <-chan error, done <-chan struct{}) error {
	select {
	case err := <-ch:
		return err
	case <-done:
		return errUnavailable
	}
}

// tryRemove unregisters an idle room unless a join is on its way.
func (h *Hub) tryRemove(r *room) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	if r.pending.Load() > 0 {
		return false
	}
	if h.rooms[r.id] == r {
		delete(h.rooms, r.id)
	}
	delete(h.stats, r.id)
	return true
}

func (h *Hub) setStats(roomID uuid.UUID, s roomStats) {
	h.mu.Lock()
	if s == (roomStats{}) {
		delete(h.stats, roomID)
	} else {
		h.stats[roomID] = s
	}
	h.mu.Unlock()
}

// Occupancy reports the live state of one room for the REST API.
func (h *Hub) Occupancy(roomID uuid.UUID) (int, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	s := h.stats[roomID]
	return s.people, s.live
}

// buildingCounts aggregates classroom and garden stats of one organization.
// Ruang Guru is hidden from participants.
func (h *Hub) buildingCounts(orgID uuid.UUID, role string) []BuildingCount {
	staff := role == "teacher" || role == "admin"
	h.mu.Lock()
	byKey := map[string]*BuildingCount{}
	for _, s := range h.stats {
		if s.orgID != orgID || (s.buildingKey == staffBuildingKey && !staff) {
			continue
		}
		b := byKey[s.buildingKey]
		if b == nil {
			b = &BuildingCount{BuildingKey: s.buildingKey}
			byKey[s.buildingKey] = b
		}
		b.People += s.people
		if s.live {
			b.LiveRooms++
		}
	}
	h.mu.Unlock()
	out := make([]BuildingCount, 0, len(byKey))
	for _, b := range byKey {
		out = append(out, *b)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].BuildingKey < out[j].BuildingKey })
	return out
}

// WhileEmpty runs fn while no one can enter roomID, provided nobody is in it.
// It is used to archive a room without racing a travel.
func (h *Hub) WhileEmpty(ctx context.Context, roomID uuid.UUID, fn func() error) error {
	h.mu.Lock()
	if h.locked[roomID] {
		h.mu.Unlock()
		return ErrRoomBusy
	}
	h.locked[roomID] = true
	r := h.rooms[roomID]
	h.mu.Unlock()
	archived := false
	defer func() {
		if !archived {
			h.mu.Lock()
			delete(h.locked, roomID)
			h.mu.Unlock()
		}
	}()
	if r != nil {
		// ensure may have returned before its reservation reached the inbox.
		if r.pending.Load() > 0 {
			return ErrRoomBusy
		}
		res := make(chan int, 1)
		if r.post(probeCmd{fn: func(r *room) { res <- r.people() }}) {
			select {
			case n := <-res:
				if n > 0 {
					return ErrRoomBusy
				}
			case <-r.done:
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}
	err := fn()
	// Keep a tombstone until shutdown so an admission read before the
	// archive cannot enter after its DB transaction commits.
	archived = err == nil
	return err
}

// ErrRoomBusy means people are still inside or travelling into the room.
var ErrRoomBusy = errors.New("room busy")

// record queues a session job; when the queue is full the job is dropped and
// logged rather than blocking a room.
func (h *Hub) record(j sessionJob) {
	if h.sessions == nil {
		return
	}
	select {
	case h.jobs <- j:
	default:
		h.log.Warn("session job dropped", slog.String("room_id", j.roomID.String()))
	}
}

// recordLoop persists session jobs in order until the queue is closed.
func (h *Hub) recordLoop() {
	open := map[uuid.UUID]uuid.UUID{} // room -> session
	do := func(j sessionJob) {
		if h.sessions == nil {
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		var err error
		switch j.kind {
		case jobOpen:
			var id uuid.UUID
			if id, err = h.sessions.OpenSession(ctx, j.roomID, j.at); err == nil {
				open[j.roomID] = id
			}
		case jobAttend:
			if id, ok := open[j.roomID]; ok {
				err = h.sessions.Attend(ctx, id, j.userID, j.at)
			}
		case jobLeave:
			if id, ok := open[j.roomID]; ok {
				err = h.sessions.Leave(ctx, id, j.userID, j.at)
			}
		case jobClose:
			err = h.sessions.CloseSession(ctx, j.roomID, j.at)
			delete(open, j.roomID)
		case jobMod:
			m, ok := h.sessions.(Moderator)
			if !ok {
				j.mod.done(false, nil, errUnavailable)
				return
			}
			var changed bool
			var banned []BannedUser
			switch j.mod.action {
			case "kick":
				changed, err = m.Kick(ctx, j.roomID, j.mod.actor, j.mod.target, j.mod.at)
			case "allow":
				changed, err = m.Allow(ctx, j.roomID, j.mod.actor, j.mod.target, j.mod.at)
			default:
				changed, err = m.RecordAction(ctx, j.roomID, j.mod.actor, j.mod.target, j.mod.action, j.mod.at)
			}
			if err == nil && (j.mod.action == "kick" || j.mod.action == "allow") {
				banned, err = m.Banned(ctx, j.roomID)
			}
			j.mod.done(changed, banned, err)
		case jobChat:
			var id int64
			var ok bool
			id, ok, err = h.sessions.SaveChat(ctx, j.roomID, j.chat.msg)
			if !ok {
				id = 0
			}
			j.chat.deliver(id)
		}
		if err != nil {
			h.log.Error("session record failed", slog.String("room_id", j.roomID.String()), slog.Any("err", err))
		}
	}
	for j := range h.jobs {
		do(j)
	}
}

// SetAllChat tells a running room that its "Semua" tab was toggled. A room
// that is not running reads the value from the database on its next start.
func (h *Hub) SetAllChat(roomID uuid.UUID, enabled bool) {
	h.mu.Lock()
	r := h.rooms[roomID]
	h.mu.Unlock()
	if r != nil {
		r.post(roomStateCmd{allChat: enabled})
	}
}

// PublishProfile sends committed identity through each room owner, including
// disconnected avatars held for reconnect. Never reads room state directly.
func (h *Hub) PublishProfile(ctx context.Context, id uuid.UUID, name, avatar string) error {
	h.mu.Lock()
	rooms := make([]*room, 0, len(h.rooms))
	for _, r := range h.rooms {
		rooms = append(rooms, r)
	}
	h.mu.Unlock()
	for _, r := range rooms {
		done := make(chan struct{})
		select {
		case r.inbox <- profileCmd{p: ProfileUpdatedMsg{ID: id, Name: name, Avatar: avatar}, done: done}:
		case <-r.done:
			continue
		case <-ctx.Done():
			return ctx.Err()
		}
		select {
		case <-done:
		case <-r.done:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}

// Shutdown closes every connection with 1001 (going away), stops all rooms
// (each closes its open session), flushes session writes, and waits until
// ctx expires.
func (h *Hub) Shutdown(ctx context.Context) error {
	h.mu.Lock()
	h.cancel()
	for _, c := range h.conns {
		c.close(websocket.StatusGoingAway, "server shutting down")
	}
	h.mu.Unlock()
	done := make(chan struct{})
	go func() {
		h.wg.Wait()   // no room can record after this
		close(h.jobs) // let the worker drain and stop
		<-h.worker
		if h.voiceJobs != nil {
			close(h.voiceJobs)
			<-h.voiceWorker
		}
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// roomCount is used by tests.
func (h *Hub) roomCount() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.rooms)
}
