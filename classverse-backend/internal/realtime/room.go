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
	chatRate         = 3.0  // messages per second
	chatBurst        = 5.0  // messages
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
	AllChat     bool
	Pin         *LearningPin
	UserID      uuid.UUID
	Name        string
	Avatar      string
	Role        string
	// Banned is true when the user was kicked from the room's open session.
	Banned         bool
	GuestExpiresAt time.Time
}

// RoleGuest marks a meeting guest (G1): no account, one room, no travel.
const RoleGuest = "guest"

func (a Admission) staff() bool { return a.Role == "teacher" || a.Role == "admin" }

type joinCmd struct {
	c      *conn
	adm    Admission
	result chan error
}

type leaveCmd struct{ c *conn }
type endGuestCmd struct {
	userID uuid.UUID
	done   chan struct{}
}

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

// chatCmd is a chat message whose text has already been filtered.
type chatCmd struct {
	c     *conn
	seq   uint64
	text  string
	scope string
}

// roomStateCmd changes settings that teachers can toggle during a session.
type roomStateCmd struct{ allChat bool }

// reserveCmd holds one capacity slot for a travelling user.
type reserveCmd struct {
	userID uuid.UUID
	staff  bool
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
	p              Participant
	conn           *conn // nil while held after a disconnect
	heldUntil      time.Time
	guestExpiresAt time.Time
	stride         stride
	moves          moveBudget
	dirty          bool
	viewReady      bool               // visibility has been synchronized since join/reconnect
	seen           map[uuid.UUID]bool // ids the client currently shows (area of interest)
	zone           Zone               // zone the avatar stands in, zero when none
	chat           chatBudget
	emoteAt        time.Time
	seatAt         time.Time // last accepted seat (mode HP), 1 per second
	voiceIdentity  string
	voiceChannel   string
}

// chatBudget is a token bucket: chatRate per second, at most chatBurst.
type chatBudget struct {
	tokens float64
	at     time.Time
}

func (b *chatBudget) allow(now time.Time) bool {
	if b.at.IsZero() {
		b.tokens = chatBurst
	} else if d := now.Sub(b.at).Seconds(); d > 0 {
		b.tokens = math.Min(chatBurst, b.tokens+d*chatRate)
	}
	b.at = now
	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
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
	pin         *LearningPin
	allChat     bool
	// Session moderation state; lives as long as the room actor (see design).
	locked       bool
	muted        map[uuid.UUID]bool
	voiceOn      map[uuid.UUID]bool // staff speaking to the whole room (voice broadcast)
	voicePending map[string]*voiceCleanup
	voiceEpoch   uuid.UUID // prevents stale media sessions hearing a restarted actor
	ticks        int
	lastStats    roomStats
}

func newRoom(adm Admission, mapd *MapData, hub *Hub, now func() time.Time, log *slog.Logger) *room {
	return &room{
		id: adm.RoomID, orgID: adm.OrgID, name: adm.RoomName, buildingKey: adm.BuildingKey,
		mapd: mapd, campus: mapd.Key == campusKey, capacity: adm.Capacity, now: now, log: log, hub: hub, pin: adm.Pin,
		allChat: adm.AllChat || mapd.Key == campusKey,
		inbox:   make(chan any, inboxSize), done: make(chan struct{}),
		members: make(map[uuid.UUID]*member), reserved: make(map[uuid.UUID]bool),
		muted: make(map[uuid.UUID]bool), voiceOn: make(map[uuid.UUID]bool), voiceEpoch: uuid.New(),
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
	defer func() {
		if ctx.Err() != nil {
			for id := range r.members {
				if !r.campus {
					r.hub.record(sessionJob{kind: jobLeave, roomID: r.id, userID: id, at: r.now()})
				}
			}
			return // Preserve session identity across process restarts.
		}
		r.closeSession()
	}()
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
	case endGuestCmd:
		r.endGuest(c.userID)
		if c.done != nil {
			close(c.done)
		}
	case leaveCmd:
		r.leave(c.c)
	case emoteCmd:
		r.emote(c)
	case moveCmd:
		r.move(c.c, c.seq, c.m)
	case reserveCmd:
		err := r.reserve(c.userID, c.staff)
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
	case learningPinResult:
		r.applyLearningPin(c)
	case commandCmd:
		r.command(c)
	case moderatedCmd:
		r.moderated(c)
	case voiceCleanedCmd:
		r.voiceCleaned(c)
	case chatCmd:
		r.chat(c)
	case seatCmd:
		r.seat(c)
	case deliverCmd:
		c.msg.ID = c.id
		r.deliverChat(c.msg, c.to)
	case roomStateCmd:
		if !r.campus && r.allChat != c.allChat {
			r.allChat = c.allChat
			r.sendStates()
		}
	case probeCmd:
		c.fn(r)
	}
}

var (
	errRoomFull  = &roomError{code: CodeRoomFull, msg: "Ruang sudah penuh"}
	errNotAtDoor = &roomError{code: CodeNotAtDoor, msg: "Berjalanlah ke pintu gedung dulu"}
	errNotInRoom = &roomError{code: CodeUnavailable, msg: "Kamu belum berada di ruang ini"}
	errLocked    = &roomError{code: CodeRoomLocked, msg: "Ruang sedang dikunci pengajar"}
	errKicked    = &roomError{code: CodeKicked, msg: "Kamu dikeluarkan dari sesi ini. Minta izin pengajar untuk masuk lagi."}
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
		m.guestExpiresAt = adm.GuestExpiresAt
		m.p.Name, m.p.Avatar, m.p.Role = adm.Name, adm.Avatar, adm.Role
		m.seen = map[uuid.UUID]bool{}
		m.viewReady = false
		m.stride = newStride(r.mapd.WalkSpeed, r.now())
		r.sendWelcome(m, TypeWelcome)
		return nil
	}
	if r.locked && !adm.staff() {
		return errLocked
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
		p:              Participant{ID: c.userID, Name: adm.Name, Avatar: adm.Avatar, Role: adm.Role, X: x, Y: y, Dir: DirDown},
		conn:           c,
		guestExpiresAt: adm.GuestExpiresAt,
		stride:         newStride(r.mapd.WalkSpeed, r.now()),
		seen:           map[uuid.UUID]bool{},
	}
	r.initVoice(m)
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
		Participants: make([]Participant, 0, len(r.members)), AllChatEnabled: r.allChat,
		Locked: r.locked, Muted: r.muted[m.p.ID], VoiceBroadcast: r.broadcasters(),
		VoiceIdentity: m.voiceIdentity, VoiceChannel: m.voiceChannel,
	}
	if m.p.Role == "teacher" || m.p.Role == "admin" {
		w.VoicePending = r.pendingVoice()
	}
	for id, o := range r.members {
		w.Participants = append(w.Participants, o.p)
		if id != m.p.ID {
			m.seen[id] = true
		}
	}
	m.conn.send(newOut(typ, w))
	m.conn.send(newOut(TypeRoomState, r.state(m.p.ID)))
	// A (re)connected client starts without a zone; tell it the current one.
	if m.zone.Key != "" {
		m.conn.send(newOut(TypeZone, ZoneMsg{Key: m.zone.Key, Name: m.zone.Name}))
	}
	if r.campus {
		r.sendBuildings(m)
	}
}

func (r *room) reserve(userID uuid.UUID, staff bool) error {
	if r.locked && !staff {
		return errLocked
	}
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
	if m := r.members[id]; m != nil {
		r.revokeVoice(m, nil)
	}
	delete(r.members, id)
	for _, o := range r.members {
		delete(o.seen, id)
	}
	r.broadcast(newOut(TypeLeft, LeftMsg{ID: id}), uuid.Nil)
	r.hub.forget(id, r.id)
	if !r.campus {
		r.hub.record(sessionJob{kind: jobLeave, roomID: r.id, userID: id, at: r.now()})

	}
	if r.voiceOn[id] {
		delete(r.voiceOn, id)
		r.sendStates()
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
	if c.list {
		x, y := m.p.X, m.p.Y
		c.sendError(ErrorMsg{Code: CodeMoveRejected, Message: "Mode HP tidak berjalan", Seq: seq, X: &x, Y: &y})
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
	if !r.voicePosition(m, mv.X, mv.Y, mv.Dir) {
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
		if m.p.Role == RoleGuest && !m.guestExpiresAt.IsZero() && !now.Before(m.guestExpiresAt) {
			r.endGuest(id)
			continue
		}
		if m.conn == nil && !now.Before(m.heldUntil) {
			r.remove(id)
		}
	}

	r.retryVoice(now)
	r.updateZones()

	r.sendSnapshots()

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

	if r.people() > 0 || len(r.voicePending) > 0 {
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

// updateZones recomputes each avatar's zone and tells its owner on change.
func (r *room) updateZones() {
	if len(r.mapd.Zones) == 0 {
		return
	}
	for _, m := range r.members {
		z := r.mapd.ZoneAt(m.p.X, m.p.Y)
		if z.Key == m.zone.Key {
			continue
		}
		m.zone = z
		if m.conn != nil {
			m.conn.send(newOut(TypeZone, ZoneMsg{Key: z.Key, Name: z.Name}))
		}
	}
}

// chat routes a filtered message by scope. Recipients are decided from the
// authoritative positions at the moment the room handles the command.
func (r *room) chat(c chatCmd) {
	m, ok := r.members[c.c.userID]
	if !ok || m.conn != c.c {
		return
	}
	now := r.now()
	reject := func(code, msg string) {
		c.c.sendError(ErrorMsg{Code: code, Message: msg, Seq: c.seq})
	}
	if c.scope == ScopeBroadcast && m.p.Role != "teacher" && m.p.Role != "admin" {
		reject(CodeForbidden, "Pengumuman hanya dapat dikirim pengajar atau admin")
		return
	}
	if r.muted[m.p.ID] {
		reject(CodeMuted, "Kamu sedang dibisukan pengajar")
		return
	}
	if !m.chat.allow(now) {
		reject(CodeRateLimited, "Terlalu cepat. Tunggu sebentar sebelum mengirim lagi.")
		return
	}
	// The zone may have changed since the last tick.
	if len(r.mapd.Zones) > 0 {
		m.zone = r.mapd.ZoneAt(m.p.X, m.p.Y)
	}
	out := ChatOutMsg{
		From: ChatSender{ID: m.p.ID, Name: m.p.Name, Role: m.p.Role}, Text: c.text, Scope: c.scope, At: now.UTC(),
	}
	var to []*member
	switch c.scope {
	case ScopeNear:
		for _, o := range r.members {
			if o == m || (o.conn != nil && math.Hypot(float64(o.p.X-m.p.X), float64(o.p.Y-m.p.Y)) <= float64(r.mapd.NearRadius)) {
				to = append(to, o)
			}
		}
	case ScopeZone:
		if m.zone.Key == "" {
			reject(CodeNotInZone, "Kamu sedang tidak berada di meja diskusi")
			return
		}
		out.Zone = m.zone.Key
		for _, o := range r.members {
			if o == m || (o.conn != nil && o.zone.Key == m.zone.Key) {
				to = append(to, o)
			}
		}
	case ScopeRoom, ScopeBroadcast:
		if c.scope == ScopeRoom && !r.allChat {
			reject(CodeAllChatDisabled, "Chat semua ruangan sedang dimatikan pengajar")
			return
		}
		for _, o := range r.members {
			if o.conn != nil {
				to = append(to, o)
			}
		}
	default:
		reject(CodeBadMessage, "Jenis chat tidak dikenal")
		return
	}
	if r.campus {
		r.deliverChat(out, to)
		return
	}
	// Store first so recipients get the id; the worker answers on deliver and
	// the room goroutine never blocks on the database.
	r.hub.storeChat(chatJob{roomID: r.id, msg: out, deliver: func(id int64) {
		// Runs on the hub worker: never block it on a busy or stopped room.
		select {
		case r.inbox <- deliverCmd{msg: out, id: id, to: to}:
		case <-r.done:
		default:
			r.log.Warn("chat dropped: room inbox full", slog.String("room_id", r.id.String()))
		}
	}})
}

// deliverCmd sends a chat message once it was stored (or storing failed).
type deliverCmd struct {
	msg ChatOutMsg
	id  int64
	to  []*member
}

func (r *room) deliverChat(msg ChatOutMsg, to []*member) {
	o := newOut(TypeChat, msg)
	for _, m := range to {
		if m.conn != nil && r.members[m.p.ID] == m {
			m.conn.send(o)
		}
	}
}

// seatCmd places a list-mode (mode HP) avatar in a zone or the general area.
type seatCmd struct {
	c    *conn
	seq  uint64
	zone string
}

// seatEvery limits seat to one per second per avatar.
const seatEvery = time.Second

// seat teleports a mode HP avatar into a zone; the next tick sends its zone
// and the snapshot that shows it at the table to everyone else.
func (r *room) seat(c seatCmd) {
	m, ok := r.members[c.c.userID]
	if !ok || m.conn != c.c {
		return
	}
	reject := func(code, msg string) {
		c.c.sendError(ErrorMsg{Code: code, Message: msg, Seq: c.seq})
	}
	if !c.c.list || r.campus {
		reject(CodeForbidden, "Pilih tempat hanya tersedia di mode HP dalam ruang")
		return
	}
	now := r.now()
	if !m.seatAt.IsZero() && now.Sub(m.seatAt) < seatEvery {
		reject(CodeRateLimited, "Terlalu cepat. Tunggu sebentar.")
		return
	}
	var x, y int
	if c.zone == "" {
		var err error
		if x, y, err = r.mapd.SpawnPoint(r.spawnN); err != nil {
			reject(CodeUnavailable, "Area umum tidak tersedia")
			return
		}
	} else if x, y, ok = r.mapd.SeatPoint(c.zone, r.spawnN); !ok {
		reject(CodeBadMessage, "Tempat tidak dikenal")
		return
	}
	r.spawnN++
	m.seatAt = now
	if !r.voicePosition(m, x, y, DirDown) {
		return
	}
	m.p.X, m.p.Y, m.p.Dir = x, y, DirDown
	m.stride = newStride(r.mapd.WalkSpeed, now)
	m.dirty = true
}

// nearPoint returns a check that accepts avatars within radius px of (x, y).
func nearPoint(x, y int, radius float64) func(Participant) error {
	return func(p Participant) error {
		if math.Hypot(float64(p.X-x), float64(p.Y-y)) > radius {
			return errNotAtDoor
		}
		return nil
	}
}

func (r *room) endGuest(id uuid.UUID) {
	m := r.members[id]
	if m == nil || m.p.Role != RoleGuest {
		return
	}
	if m.conn != nil {
		m.conn.sendError(ErrorMsg{Code: CodeForbidden, Message: "Akses tamu sudah berakhir"})
		m.conn.close(closeForbidden, CodeForbidden)
	}
	r.remove(id)
}
