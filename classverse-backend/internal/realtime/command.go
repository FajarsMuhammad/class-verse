package realtime

import (
	"log/slog"

	"github.com/google/uuid"
)

type commandCmd struct {
	c      *conn
	seq    uint64
	name   string
	target uuid.UUID
}

// moderatedCmd comes back from the hub worker after a kick/allow was written.
type moderatedCmd struct {
	action  string
	actor   uuid.UUID
	seq     uint64
	target  uuid.UUID
	changed bool
	banned  []BannedUser
	err     error
}

func (r *room) command(c commandCmd) {
	m := r.members[c.c.userID]
	if m == nil || m.conn != c.c {
		return
	}
	reject := func(code, message string) {
		c.c.sendError(ErrorMsg{Code: code, Message: message, Seq: c.seq})
	}
	if m.p.Role != "teacher" && m.p.Role != "admin" {
		reject(CodeForbidden, "Perintah hanya untuk pengajar dan admin")
		return
	}
	switch c.name {
	case "gather":
		r.gather(m, c.seq, reject)
	case "kick", "allow", "mute", "unmute", "lock", "unlock":
		r.moderate(m, c, reject)
	default:
		reject(CodeBadMessage, "Perintah tidak dikenal")
	}
}

func (r *room) gather(m *member, _ uint64, reject func(code, message string)) {
	if r.campus {
		reject(CodeForbidden, "Kumpulkan hanya tersedia di ruang")
		return
	}
	x, y, err := r.mapd.SpawnPoint(0)
	if err != nil {
		reject(CodeBadMessage, "Titik kumpul tidak tersedia")
		return
	}
	now := r.now()
	participants := make([]Participant, 0, len(r.members))
	for _, member := range r.members {
		member.p.X, member.p.Y = x, y
		member.stride = newStride(r.mapd.WalkSpeed, now)
		member.dirty = true
		participants = append(participants, member.p)
	}
	r.broadcast(newOut(TypeEvent, GatherEvent{Name: "gather", By: m.p.ID, Participants: participants}), uuid.Nil)
}

// moderate validates against live room state, then hands the write to the
// hub worker. The room never blocks on the database; the change takes effect
// only after the audit row (and, for kick, the ban) is committed.
func (r *room) moderate(m *member, c commandCmd, reject func(code, message string)) {
	if r.campus {
		reject(CodeForbidden, "Moderasi hanya tersedia di ruang")
		return
	}
	target := c.target
	switch c.name {
	case "lock", "unlock":
		if (c.name == "lock") == r.locked {
			reject(CodeBadMessage, map[bool]string{true: "Ruang sudah terkunci", false: "Ruang tidak terkunci"}[r.locked])
			return
		}
		target = uuid.Nil
	default:
		if target == uuid.Nil || target == m.p.ID {
			reject(CodeBadMessage, "Pilih peserta yang dituju")
			return
		}
		if c.name == "allow" {
			break // the banned user is not in the room
		}
		t := r.members[target]
		if t == nil {
			reject(CodeNotFoundTarget, "Peserta tidak ada di ruang ini")
			return
		}
		if t.p.Role != "participant" {
			reject(CodeForbidden, "Pengajar dan admin tidak bisa dimoderasi")
			return
		}
		if c.name == "mute" && r.muted[target] {
			reject(CodeBadMessage, "Peserta sudah dibisukan")
			return
		}
		if c.name == "unmute" && !r.muted[target] {
			reject(CodeBadMessage, "Peserta tidak sedang dibisukan")
			return
		}
	}
	action, actor, seq := c.name, m.p.ID, c.seq
	r.hub.moderate(modJob{action: action, roomID: r.id, actor: actor, target: target, at: r.now(),
		done: func(changed bool, banned []BannedUser, err error) {
			cmd := moderatedCmd{action: action, actor: actor, seq: seq, target: target, changed: changed, banned: banned, err: err}
			select {
			case r.inbox <- cmd:
			case <-r.done:
			default:
				r.log.Warn("moderation result dropped: room inbox full", slog.String("room_id", r.id.String()))
			}
		}})
}

func (r *room) moderated(c moderatedCmd) {
	actor := r.members[c.actor]
	reply := func(code, message string) {
		if actor != nil && actor.conn != nil {
			actor.conn.sendError(ErrorMsg{Code: code, Message: message, Seq: c.seq})
		}
	}
	if c.err != nil {
		r.log.Error("moderation write failed", slog.String("room_id", r.id.String()),
			slog.String("action", c.action), slog.Any("err", c.err))
		reply(CodeUnavailable, "Aksi gagal disimpan. Coba lagi.")
		return
	}
	if !c.changed {
		if c.action == "allow" {
			reply(CodeNotFoundTarget, "Peserta tidak sedang dikeluarkan")
		} else {
			reply(CodeForbidden, "Aksi tidak diizinkan")
		}
		return
	}
	switch c.action {
	case "kick":
		delete(r.muted, c.target)
		if t := r.members[c.target]; t != nil {
			if t.conn != nil {
				t.conn.sendError(ErrorMsg{Code: CodeKicked, Message: errKicked.msg})
				t.conn.close(closeForbidden, CodeKicked)
				t.conn = nil
			}
			r.remove(c.target)
		}
	case "mute", "unmute":
		muted := c.action == "mute"
		if muted {
			r.muted[c.target] = true
		} else {
			delete(r.muted, c.target)
		}
		if t := r.members[c.target]; t != nil && t.conn != nil {
			t.conn.send(newOut(TypeRoomState, r.state(c.target)))
		}
	case "lock", "unlock":
		r.locked = c.action == "lock"
		for id, o := range r.members {
			if o.conn != nil {
				o.conn.send(newOut(TypeRoomState, r.state(id)))
			}
		}
	}
	ev := newOut(TypeEvent, ModerationEvent{Name: c.action, By: c.actor, Target: c.target, Banned: c.banned, Muted: r.mutedIDs()})
	for _, o := range r.members {
		if o.conn != nil && (o.p.Role == "teacher" || o.p.Role == "admin") {
			o.conn.send(ev)
		}
	}
}

// state is the room_state a member sees: shared settings plus their own mute.
func (r *room) state(id uuid.UUID) RoomStateMsg {
	return RoomStateMsg{AllChatEnabled: r.allChat, Locked: r.locked, Muted: r.muted[id]}
}

func (r *room) mutedIDs() []uuid.UUID {
	out := make([]uuid.UUID, 0, len(r.muted))
	for id := range r.muted {
		out = append(out, id)
	}
	return out
}
