package realtime

import (
	"context"
	"fmt"
	"time"
)

// RecoverSessions closes interrupted attendance but preserves session identity.
// Startup must call this before serving traffic.
func (s *Store) RecoverSessions(ctx context.Context, at time.Time) ([]Admission, error) {
	if err := s.q.EndInterruptedAttendance(ctx, pgTime(at)); err != nil {
		return nil, fmt.Errorf("end interrupted attendance: %w", err)
	}
	rows, err := s.q.ListRecoverableRooms(ctx)
	if err != nil {
		return nil, fmt.Errorf("list recoverable sessions: %w", err)
	}
	rooms := make([]Admission, 0, len(rows))
	for _, r := range rows {
		rooms = append(rooms, Admission{RoomID: r.ID, OrgID: r.OrgID, RoomName: r.Name, BuildingKey: r.BuildingKey.String, MapKey: r.MapKey, Capacity: int(r.Capacity), AllChat: r.AllChatEnabled})
	}
	return rooms, nil
}

// RestoreSessions starts empty actors with a fresh idle deadline. A reconnect
// reuses the database session; abandoned sessions close after sessionIdle.
func (h *Hub) RestoreSessions(rooms []Admission) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, adm := range rooms {
		if h.maps[adm.MapKey] == nil {
			return fmt.Errorf("recover session: unknown map %q", adm.MapKey)
		}
	}
	for _, adm := range rooms {
		if h.rooms[adm.RoomID] != nil {
			continue
		}
		r := newRoom(adm, h.maps[adm.MapKey], h, h.now, h.log)
		r.emptySince = h.now()
		// Queue session lookup now so future attendance jobs can reuse its id.
		r.sessionOpen = true
		h.record(sessionJob{kind: jobOpen, roomID: adm.RoomID, at: h.now()})
		h.rooms[adm.RoomID] = r
		h.wg.Go(func() { r.run(h.ctx) })
	}
	return nil
}
