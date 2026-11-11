package realtime

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/FajarsMuhammad/class-verse/classverse-backend/maps"
)

// TestRevampDoorRadius uses the production campus: Rumah Angka accepts
// entry within its 60 px interact_radius, not the 48 px fixture default.
func TestRevampDoorRadius(t *testing.T) {
	mapData, err := LoadMaps("campus", "classroom")
	if err != nil {
		t.Fatal(err)
	}
	b, _ := mapData["campus"].Building("angka")
	if b.InteractRadius != 60 {
		t.Fatalf("angka radius %d", b.InteractRadius)
	}
	for _, tc := range []struct {
		dy   int
		want error
	}{{-59, nil}, {-61, errNotAtDoor}} {
		hub := NewHub(mapData, nil, time.Now, slog.New(slog.NewTextHandler(io.Discard, nil)))
		c := newConn(uuid.New())
		origin := Admission{RoomID: uuid.New(), MapKey: campusKey, Capacity: 200, UserID: c.userID, Role: "participant"}
		from, err := hub.join(c, origin)
		if err != nil {
			t.Fatal(err)
		}
		done := make(chan struct{})
		from.post(probeCmd{fn: func(r *room) { r.members[c.userID].p.X, r.members[c.userID].p.Y = b.DoorX, b.DoorY+tc.dy; close(done) }})
		<-done
		dest := origin
		dest.RoomID, dest.MapKey, dest.Capacity, dest.BuildingKey = uuid.New(), "classroom", 40, "angka"
		if _, err := hub.travel(c, from, dest); !errors.Is(err, tc.want) {
			t.Fatalf("dy %d: got %v want %v", tc.dy, err, tc.want)
		}
		_ = hub.Shutdown(context.Background())
	}
}

func TestParseMapRejectsInvalidInteractRadius(t *testing.T) {
	raw, err := maps.Read("campus")
	if err != nil {
		t.Fatal(err)
	}
	bad := strings.Replace(string(raw), `{"name":"interact_radius","type":"int","value":60}`, `{"name":"interact_radius","type":"int","value":250}`, 1)
	if bad == string(raw) {
		t.Fatal("fixture text not found")
	}
	if _, err := ParseMap("campus", []byte(bad)); err == nil {
		t.Fatal("interact_radius 250 must fail like near_radius, not fall back silently")
	}
}

func TestRevampMaps(t *testing.T) {
	for _, key := range []string{"campus", "classroom", "garden", "lab", "baca"} {
		raw, err := maps.Read(key)
		if err != nil {
			t.Fatal(err)
		}
		m, err := ParseMap(key, raw)
		if err != nil {
			t.Fatal(err)
		}
		if m.NearRadius != 85 {
			t.Fatalf("%s radius %d", key, m.NearRadius)
		}
		if key == "campus" {
			if m.PixelWidth() != 2200 || m.PixelHeight() != 1500 || len(m.Buildings) != 10 {
				t.Fatal("campus geometry mismatch")
			}
			b, ok := m.Building("ruang-guru")
			if !ok || !b.StaffOnly {
				t.Fatal("staff building must remain restricted")
			}
		}
		for _, z := range m.Zones {
			if !z.Circle || z.Contains(z.Area.X+1, z.Area.Y+1) {
				t.Fatal("zone must exclude circle corners")
			}
			if !z.Contains(z.Area.X+z.Area.W/2, z.Area.Y+z.Area.H/2) {
				t.Fatal("zone center missing")
			}
		}
	}
}
