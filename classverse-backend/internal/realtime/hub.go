package realtime

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/google/uuid"
)

var errUnavailable = errors.New("room unavailable")

// Hub is the registry of running rooms and of the one live connection per
// user. Rooms are started on first join and stop themselves when idle.
type Hub struct {
	ctx    context.Context
	cancel context.CancelFunc
	maps   map[string]*MapData
	now    func() time.Time
	log    *slog.Logger

	mu    sync.Mutex
	rooms map[uuid.UUID]*room
	conns map[uuid.UUID]*conn
	wg    sync.WaitGroup
}

// NewHub returns an empty hub. Call Shutdown to stop it.
func NewHub(maps map[string]*MapData, now func() time.Time, log *slog.Logger) *Hub {
	ctx, cancel := context.WithCancel(context.Background())
	return &Hub{ctx: ctx, cancel: cancel, maps: maps, now: now, log: log, rooms: map[uuid.UUID]*room{}, conns: map[uuid.UUID]*conn{}}
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

func (h *Hub) release(c *conn) {
	h.mu.Lock()
	if h.conns[c.userID] == c {
		delete(h.conns, c.userID)
	}
	h.mu.Unlock()
}

// join adds c to the room of adm, starting the room if needed.
func (h *Hub) join(c *conn, adm Admission) (*room, error) {
	mapd := h.maps[adm.MapKey]
	if mapd == nil {
		return nil, errUnavailable
	}
	h.mu.Lock()
	if h.ctx.Err() != nil {
		h.mu.Unlock()
		return nil, errUnavailable
	}
	r := h.rooms[adm.RoomID]
	if r == nil {
		r = newRoom(adm.RoomID, mapd, adm.Capacity, h, h.now, h.log)
		h.rooms[adm.RoomID] = r
		h.wg.Go(func() { r.run(h.ctx) })
	}
	// Counted under the hub lock so an idle room cannot stop with a join in flight.
	r.pending.Add(1)
	h.mu.Unlock()

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
		return r, nil
	case <-r.done:
		return nil, errUnavailable
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
	return true
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

// Shutdown closes every connection with 1001 (going away), stops all rooms,
// and waits for them until ctx expires.
func (h *Hub) Shutdown(ctx context.Context) error {
	h.mu.Lock()
	h.cancel()
	for _, c := range h.conns {
		c.close(websocket.StatusGoingAway, "server shutting down")
	}
	h.mu.Unlock()
	done := make(chan struct{})
	go func() {
		h.wg.Wait()
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
