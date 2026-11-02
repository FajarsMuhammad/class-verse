package realtime

import (
	"strings"
	"testing"

	"github.com/FajarsMuhammad/class-verse/classverse-backend/maps"
)

func loadCampus(t *testing.T) *MapData {
	t.Helper()
	raw, err := maps.Read("campus")
	if err != nil {
		t.Fatal(err)
	}
	m, err := ParseMap("campus", raw)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func TestParseCampus(t *testing.T) {
	m := loadCampus(t)
	if m.Width != 64 || m.Height != 40 || m.TileSize != 32 {
		t.Errorf("size = %dx%d@%d", m.Width, m.Height, m.TileSize)
	}
	keys := map[string]Building{}
	for _, b := range m.Buildings {
		keys[b.Key] = b
	}
	for _, k := range []string{"kelas-a", "kelas-b", "ruang-guru", "perpustakaan", "aula", "kantin", "taman", "lapangan"} {
		if _, ok := keys[k]; !ok {
			t.Errorf("missing building %q", k)
		}
	}
	if !keys["ruang-guru"].StaffOnly || keys["kelas-a"].StaffOnly {
		t.Error("staff_only flags wrong")
	}
	x, y, err := m.SpawnPoint(0)
	if err != nil || m.Blocked(x, y) {
		t.Fatalf("spawn %d,%d blocked or %v", x, y, err)
	}
	// Building walls block; outside the map blocks.
	a := keys["kelas-a"]
	if !m.Blocked(a.Area.X+a.Area.W/2, a.Area.Y+a.Area.H/2) {
		t.Error("inside a building should be blocked")
	}
	if !m.Blocked(-1, 10) || !m.Blocked(10, m.PixelHeight()) {
		t.Error("outside the map should be blocked")
	}
	// Walking straight up from the door into the building is not clear.
	if m.PathClear(a.DoorX, a.DoorY, a.DoorX, a.DoorY-3*m.TileSize) {
		t.Error("path through a wall should not be clear")
	}
}

func TestMapWalkSpeeds(t *testing.T) {
	for key, want := range map[string]int{"campus": 320, "classroom": 160, "garden": 160} {
		raw, err := maps.Read(key)
		if err != nil {
			t.Fatal(err)
		}
		m, err := ParseMap(key, raw)
		if err != nil {
			t.Fatal(err)
		}
		if m.WalkSpeed != want {
			t.Errorf("%s walk_speed = %d, want %d", key, m.WalkSpeed, want)
		}
	}
}

func TestParseMapErrors(t *testing.T) {
	tests := map[string]string{
		"not json":     `{`,
		"bad size":     `{"width":0,"height":1,"tilewidth":32,"tileheight":32,"layers":[]}`,
		"no collision": `{"width":1,"height":1,"tilewidth":32,"tileheight":32,"layers":[{"name":"spawn","objects":[{"x":0,"y":0,"width":32,"height":32}]}]}`,
		"spawn blocked": `{"width":1,"height":1,"tilewidth":32,"tileheight":32,"layers":[
			{"name":"collision","type":"tilelayer","data":[1]},
			{"name":"spawn","objects":[{"x":0,"y":0,"width":32,"height":32}]}]}`,
		"door missing": `{"width":2,"height":1,"tilewidth":32,"tileheight":32,"layers":[
			{"name":"collision","type":"tilelayer","data":[0,0]},
			{"name":"spawn","objects":[{"x":0,"y":0,"width":32,"height":32}]},
			{"name":"buildings","objects":[{"type":"building","x":32,"y":0,"width":32,"height":32,"properties":[{"name":"key","value":"a"}]}]}]}`,
		"walk_speed too fast": `{"properties":[{"name":"walk_speed","value":900}],"width":1,"height":1,"tilewidth":32,"tileheight":32,"layers":[
			{"name":"collision","type":"tilelayer","data":[0]},
			{"name":"spawn","objects":[{"x":0,"y":0,"width":32,"height":32}]}]}`,
		"walk_speed not a number": `{"properties":[{"name":"walk_speed","value":"fast"}],"width":1,"height":1,"tilewidth":32,"tileheight":32,"layers":[
			{"name":"collision","type":"tilelayer","data":[0]},
			{"name":"spawn","objects":[{"x":0,"y":0,"width":32,"height":32}]}]}`,
	}
	for name, raw := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := ParseMap("x", []byte(strings.TrimSpace(raw))); err == nil {
				t.Error("ParseMap() = nil error")
			}
		})
	}
	if _, err := maps.Read("../etc/passwd"); err == nil {
		t.Error("Read accepted path traversal")
	}
	if _, err := maps.Read("nope"); err == nil {
		t.Error("Read accepted unknown map")
	}
}
