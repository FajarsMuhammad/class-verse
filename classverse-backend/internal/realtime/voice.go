package realtime

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
)

const voiceRetry = time.Second

// voiceCleanup is retained by the room until LiveKit confirms absence.
// Admissions are revoked before work is queued; failed/full queues retry on tick.
type voiceCleanup struct {
	identity, channel string
	target            uuid.UUID
	at                time.Time
	busy              bool
	position          *Position
}

type voiceCleanedCmd struct {
	identity string
	err      error
}

func (r *room) initVoice(m *member) {
	if r.campus {
		return
	}
	m.voiceIdentity = m.p.ID.String() + ":" + uuid.NewString()
	m.voiceChannel = r.voiceChannel(m.p.X, m.p.Y)
}

func (r *room) voiceChannel(x, y int) string {
	key := r.mapd.ZoneAt(x, y).Key
	if key == "" {
		key = "general"
	}
	return r.id.String() + ":" + r.voiceEpoch.String() + ":zone:" + key
}

// revokeVoice also rotates identity, so refreshed LiveKit tokens cannot rejoin.
func (r *room) revokeVoice(m *member, position *Position) {
	if m.voiceIdentity == "" {
		return
	}
	identity := m.voiceIdentity
	if r.voicePending == nil {
		r.voicePending = make(map[string]*voiceCleanup)
	}
	r.hub.mu.Lock()
	if r.hub.voiceRevoking == nil {
		r.hub.voiceRevoking = map[uuid.UUID]int{}
	}
	r.hub.voiceRevoking[m.p.ID]++
	r.hub.mu.Unlock()
	r.voicePending[identity] = &voiceCleanup{identity: identity, channel: m.voiceChannel, target: m.p.ID, position: position}
	m.voiceIdentity = ""
	r.sendStates()
	r.retryVoice(r.now())
}

func (r *room) retryVoice(now time.Time) {
	for _, job := range r.voicePending {
		if job.busy || now.Before(job.at) {
			continue
		}
		if r.hub.voice == nil {
			r.voiceCleaned(voiceCleanedCmd{identity: job.identity})
			continue
		}
		identity, channel := job.identity, job.channel
		work := func(ctx context.Context) {
			err := r.hub.voice.Remove(ctx, channel, identity)
			if err == nil {
				base, _, _ := strings.Cut(channel, ":zone:")
				err = r.hub.voice.Remove(ctx, base+":broadcast", identity)
			}
			select {
			case r.inbox <- voiceCleanedCmd{identity: identity, err: err}:
			case <-r.done:
			case <-ctx.Done():
			}
		}
		select {
		case r.hub.voiceJobs <- work:
			job.busy = true
		default:
			job.at = now.Add(voiceRetry)
		}
	}
}

func (r *room) voiceCleaned(c voiceCleanedCmd) {
	job := r.voicePending[c.identity]
	if job == nil {
		return
	}
	if c.err != nil {
		job.busy = false
		job.at = r.now().Add(voiceRetry)
		r.log.Warn("voice eviction pending", slog.String("room_id", r.id.String()), slog.Any("err", c.err))
		r.sendStates()
		return
	}
	delete(r.voicePending, c.identity)
	r.hub.mu.Lock()
	r.hub.voiceRevoking[job.target]--
	if r.hub.voiceRevoking[job.target] == 0 {
		delete(r.hub.voiceRevoking, job.target)
	}
	r.hub.mu.Unlock()
	if m := r.members[job.target]; m != nil && m.voiceIdentity == "" {
		if job.position != nil {
			p := job.position
			m.p.X, m.p.Y, m.p.Dir = p.X, p.Y, p.Dir
			m.stride = newStride(r.mapd.WalkSpeed, r.now())
			m.dirty = true
			m.zone = r.mapd.ZoneAt(m.p.X, m.p.Y)
			if m.conn != nil {
				m.conn.send(newOut(TypeZone, ZoneMsg{Key: m.zone.Key, Name: m.zone.Name}))
				x, y := m.p.X, m.p.Y
				m.conn.sendError(ErrorMsg{Code: CodeMoveRejected, Message: "Perpindahan zona dikonfirmasi", X: &x, Y: &y})
			}
		}
		r.initVoice(m)
	}
	r.sendStates()
}

// voicePosition holds cross-zone movement until old media is gone.
func (r *room) voicePosition(m *member, x, y int, dir Direction) bool {
	if r.campus || r.hub.voice == nil {
		return true
	}
	if m.voiceIdentity == "" {
		return false
	}
	if r.voiceChannel(x, y) == m.voiceChannel {
		return true
	}
	r.revokeVoice(m, &Position{ID: m.p.ID, X: x, Y: y, Dir: dir})
	return false
}

func (r *room) pendingVoice() []uuid.UUID {
	ids := make([]uuid.UUID, 0, len(r.voicePending))
	seen := map[uuid.UUID]bool{}
	for _, job := range r.voicePending {
		if !seen[job.target] {
			ids = append(ids, job.target)
			seen[job.target] = true
		}
	}
	return ids
}
