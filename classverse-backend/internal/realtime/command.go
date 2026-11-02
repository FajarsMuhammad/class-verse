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
	kick    bool
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
	case "kick", "allow":
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
// hub worker. The room never blocks on the database; the target is removed
// only after the ban and its audit row are committed.
func (r *room) moderate(m *member, c commandCmd, reject func(code, message string)) {
	if r.campus {
		reject(CodeForbidden, "Moderasi hanya tersedia di ruang")
		return
	}
	if c.target == uuid.Nil || c.target == m.p.ID {
		reject(CodeBadMessage, "Pilih peserta yang dituju")
		return
	}
	kick := c.name == "kick"
	if kick {
		t := r.members[c.target]
		if t == nil {
			reject(CodeNotFoundTarget, "Peserta tidak ada di ruang ini")
			return
		}
		if t.p.Role != "participant" {
			reject(CodeForbidden, "Pengajar dan admin tidak bisa dikeluarkan")
			return
		}
	}
	actor, seq, target := m.p.ID, c.seq, c.target
	r.hub.moderate(modJob{kick: kick, roomID: r.id, actor: actor, target: target, at: r.now(),
		done: func(changed bool, banned []BannedUser, err error) {
			cmd := moderatedCmd{kick: kick, actor: actor, seq: seq, target: target, changed: changed, banned: banned, err: err}
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
		r.log.Error("moderation write failed", slog.String("room_id", r.id.String()), slog.Any("err", c.err))
		reply(CodeUnavailable, "Aksi gagal disimpan. Coba lagi.")
		return
	}
	if !c.changed {
		if c.kick {
			reply(CodeForbidden, "Peserta tidak dapat dikeluarkan")
		} else {
			reply(CodeNotFoundTarget, "Peserta tidak sedang dikeluarkan")
		}
		return
	}
	if c.kick {
		if t := r.members[c.target]; t != nil {
			if t.conn != nil {
				t.conn.sendError(ErrorMsg{Code: CodeKicked, Message: errKicked.msg})
				t.conn.close(closeForbidden, CodeKicked)
				t.conn = nil
			}
			r.remove(c.target)
		}
	}
	name := "allow"
	if c.kick {
		name = "kick"
	}
	ev := newOut(TypeEvent, ModerationEvent{Name: name, By: c.actor, Target: c.target, Banned: c.banned})
	for _, o := range r.members {
		if o.conn != nil && (o.p.Role == "teacher" || o.p.Role == "admin") {
			o.conn.send(ev)
		}
	}
}
