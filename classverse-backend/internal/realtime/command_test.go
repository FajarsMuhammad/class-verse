package realtime

import "testing"

func TestGather(t *testing.T) {
	for _, role := range []string{"participant", "teacher", "admin"} {
		t.Run(role, func(t *testing.T) {
			r, _ := newMapRoom(t, "classroom", true)
			a, b := r.testJoin(t, "Actor"), r.testJoin(t, "Participant")
			r.members[a.userID].p.Role = role
			r.place(b, 100, 100)
			drain(a)
			drain(b)
			r.handle(commandCmd{c: a, seq: 7, name: "gather"})
			if role == "participant" {
				if got := errorsOf(t, a); len(got) != 1 || got[0] != CodeForbidden {
					t.Fatalf("errors: %v", got)
				}
				if r.members[b.userID].p.X != 100 {
					t.Fatal("unauthorized gather moved target")
				}
				return
			}
			x, y, err := r.mapd.SpawnPoint(0)
			if err != nil {
				t.Fatal(err)
			}
			for _, c := range []*conn{a, b} {
				p := r.members[c.userID].p
				if p.X != x || p.Y != y {
					t.Fatalf("position: %v", p)
				}
				if len(drain(c)[TypeEvent]) != 1 {
					t.Fatal("missing gather event")
				}
			}
		})
	}
}

func TestGatherForbiddenOnCampus(t *testing.T) {
	r, _ := newMapRoom(t, "campus", true)
	a := r.testJoin(t, "Teacher")
	r.members[a.userID].p.Role = "teacher"
	drain(a)
	r.handle(commandCmd{c: a, name: "gather"})
	if got := errorsOf(t, a); len(got) != 1 || got[0] != CodeForbidden {
		t.Fatalf("errors: %v", got)
	}
}
