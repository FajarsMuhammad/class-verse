package realtime

import "testing"

func TestBroadcastAuthorization(t *testing.T) {
	for _, role := range []string{"participant", "teacher", "admin"} {
		t.Run(role, func(t *testing.T) {
			r, _ := newMapRoom(t, "classroom", false)
			a, b := r.testJoin(t, "Sender"), r.testJoin(t, "Receiver")
			r.members[a.userID].p.Role = role
			drain(a)
			drain(b)
			r.say(a, ScopeBroadcast, "Pengumuman")
			if role == "participant" {
				codes := errorsOf(t, a)
				if len(codes) != 1 || codes[0] != CodeForbidden {
					t.Fatalf("errors = %v", codes)
				}
				if len(chats(t, b)) != 0 {
					t.Fatal("unauthorized broadcast delivered")
				}
				return
			}
			for _, c := range []*conn{a, b} {
				got := chats(t, c)
				if len(got) != 1 || got[0].Scope != ScopeBroadcast {
					t.Fatalf("chat = %v", got)
				}
			}
		})
	}
}
