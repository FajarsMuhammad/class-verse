package realtime

import "time"

type emoteCmd struct {
	c   *conn
	seq uint64
	key string
}

func (r *room) emote(cmd emoteCmd) {
	m, ok := r.members[cmd.c.userID]
	if !ok || m.conn != cmd.c {
		return
	}
	if r.muted[m.p.ID] {
		cmd.c.sendError(ErrorMsg{Code: CodeMuted, Message: "Kamu dibisukan oleh pengajar", Seq: cmd.seq})
		return
	}
	switch cmd.key {
	case "wave", "hand", "quiz", "cheer":
	default:
		cmd.c.sendError(ErrorMsg{Code: CodeBadMessage, Message: "Emote tidak dikenal", Seq: cmd.seq})
		return
	}
	now := r.now()
	if !m.emoteAt.IsZero() && now.Sub(m.emoteAt) < time.Second {
		cmd.c.sendError(ErrorMsg{Code: CodeRateLimited, Message: "Tunggu sebentar sebelum emote lagi", Seq: cmd.seq})
		return
	}
	m.emoteAt = now
	out := newOut(TypeEmote, EmoteOut{ID: m.p.ID, Key: cmd.key})
	for _, recipient := range r.members {
		if recipient.conn != nil && (!r.campus || inAOI(r.mapd, m.p.X, m.p.Y, recipient.p.X, recipient.p.Y)) {
			recipient.conn.send(out)
		}
	}
}
