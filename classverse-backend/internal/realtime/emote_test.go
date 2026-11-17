package realtime

import (
	"testing"
	"time"
)

func TestEmoteDeliveryAndLimits(t *testing.T) {
	r, clock := newMapRoom(t, "campus", true)
	sender := r.testJoin(t, "Ani")
	other := r.testJoin(t, "Budi")
	drain(sender)
	drain(other)
	r.handle(emoteCmd{c: sender, key: "wave", seq: 1})
	if len(drain(sender)[TypeEmote]) != 1 || len(drain(other)[TypeEmote]) != 1 {
		t.Fatal("room emote missing")
	}
	r.handle(emoteCmd{c: sender, key: "wave", seq: 2})
	if got := errorsOf(t, sender); len(got) != 1 || got[0] != CodeRateLimited {
		t.Fatal("emote flood allowed", got)
	}
	clock.t = clock.t.Add(2 * time.Second)
	r.handle(emoteCmd{c: sender, key: "<script>", seq: 3})
	if got := errorsOf(t, sender); len(got) != 1 || got[0] != CodeBadMessage {
		t.Fatal("unknown emote allowed", got)
	}
	drain(other)
	stale := newConn(sender.userID)
	r.handle(emoteCmd{c: stale, key: "wave", seq: 4})
	if len(drain(other)[TypeEmote]) != 0 {
		t.Fatal("stale connection sent emote")
	}
	r.muted[sender.userID] = true
	r.handle(emoteCmd{c: sender, key: "hand", seq: 5})
	if got := errorsOf(t, sender); len(got) != 1 || got[0] != CodeMuted {
		t.Fatal("muted actor sent emote", got)
	}
}

func TestCampusEmoteStaysInAreaOfInterest(t *testing.T) {
	r, _ := newMapRoom(t, "campus", true)
	a := r.testJoin(t, "Ani")
	b := r.testJoin(t, "Budi")
	drain(a)
	drain(b)
	r.members[b.userID].p.X = r.members[a.userID].p.X + r.mapd.TileSize*40
	r.handle(emoteCmd{c: a, key: "cheer"})
	if len(drain(b)[TypeEmote]) != 0 {
		t.Fatal("distant campus actor received emote")
	}
}
