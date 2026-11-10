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
	// Slice 10: the campus is a 128x80 world (W10-01).
	if m.Width != 128 || m.Height != 80 || m.TileSize != 32 {
		t.Errorf("size = %dx%d@%d", m.Width, m.Height, m.TileSize)
	}
	keys := map[string]Building{}
	for _, b := range m.Buildings {
		keys[b.Key] = b
	}
	for _, k := range []string{"kelas-a", "kelas-b", "kelas-c", "ruang-guru", "perpustakaan", "kantin", "taman", "lapangan"} {
		if _, ok := keys[k]; !ok {
			t.Errorf("missing building %q", k)
		}
	}
	if len(keys) != 8 {
		t.Errorf("building count = %d, want 8", len(keys))
	}
	for _, key := range []string{"kebun", "lab", "aula", "uks", "studio", "mushola"} {
		if _, ok := keys[key]; ok {
			t.Errorf("removed building %q still present", key)
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
	// Every door is a walkable stand reachable on foot from the spawn.
	sx, sy, _ := m.SpawnPoint(0)
	for _, b := range m.Buildings {
		if m.Blocked(b.DoorX, b.DoorY) {
			t.Errorf("%s door %d,%d is blocked", b.Key, b.DoorX, b.DoorY)
		}
		if !reachable(m, sx, sy, b.DoorX, b.DoorY) {
			t.Errorf("%s door unreachable from spawn", b.Key)
		}
	}
}

// reachable floods walkable tiles (4-neighbour) from one pixel point to another.
func reachable(m *MapData, x0, y0, x1, y1 int) bool {
	ts := m.TileSize
	start, goal := [2]int{x0 / ts, y0 / ts}, [2]int{x1 / ts, y1 / ts}
	seen := map[[2]int]bool{start: true}
	queue := [][2]int{start}
	for len(queue) > 0 {
		c := queue[0]
		queue = queue[1:]
		if c == goal {
			return true
		}
		for _, d := range [][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
			n := [2]int{c[0] + d[0], c[1] + d[1]}
			if seen[n] || m.Blocked(n[0]*ts+ts/2, n[1]*ts+ts/2) {
				continue
			}
			seen[n] = true
			queue = append(queue, n)
		}
	}
	return false
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

func TestMapNearRadius(t *testing.T) {
	// Rooms use 3 tiles so near chat stays at one table; the campus keeps 5.
	for key, want := range map[string]int{"campus": 160, "classroom": 96, "garden": 96} {
		raw, err := maps.Read(key)
		if err != nil {
			t.Fatal(err)
		}
		m, err := ParseMap(key, raw)
		if err != nil {
			t.Fatal(err)
		}
		if m.NearRadius != want {
			t.Errorf("%s near radius = %d, want %d", key, m.NearRadius, want)
		}
	}
}

func TestMapZones(t *testing.T) {
	raw, err := maps.Read("classroom")
	if err != nil {
		t.Fatal(err)
	}
	m, err := ParseMap("classroom", raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Zones) != 6 {
		t.Fatalf("zones = %d, want 6", len(m.Zones))
	}
	z := m.Zones[0]
	if got := m.ZoneAt(z.Area.X+1, z.Area.Y+1); got.Key != z.Key || got.Name == "" {
		t.Errorf("ZoneAt inside = %+v", got)
	}
	if got := m.ZoneAt(z.Area.X+z.Area.W, z.Area.Y); got.Key == z.Key {
		t.Error("right edge must be outside")
	}
	if got := loadCampus(t).ZoneAt(1000, 600); got.Key != "" {
		t.Errorf("campus has zone %+v", got)
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
		"duplicate zone": `{"width":4,"height":4,"tilewidth":32,"tileheight":32,"layers":[
			{"name":"collision","type":"tilelayer","data":[0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0]},
			{"name":"spawn","objects":[{"x":0,"y":0,"width":32,"height":32}]},
			{"name":"zones","objects":[{"x":0,"y":0,"width":32,"height":32,"properties":[{"name":"key","value":"m"}]},
			{"x":32,"y":0,"width":32,"height":32,"properties":[{"name":"key","value":"m"}]}]}]}`,
		"zone outside": `{"width":1,"height":1,"tilewidth":32,"tileheight":32,"layers":[
			{"name":"collision","type":"tilelayer","data":[0]},
			{"name":"spawn","objects":[{"x":0,"y":0,"width":32,"height":32}]},
			{"name":"zones","objects":[{"x":0,"y":0,"width":64,"height":32,"properties":[{"name":"key","value":"m"}]}]}]}`,
		"walk_speed too fast": `{"properties":[{"name":"walk_speed","value":900}],"width":1,"height":1,"tilewidth":32,"tileheight":32,"layers":[
			{"name":"collision","type":"tilelayer","data":[0]},
			{"name":"spawn","objects":[{"x":0,"y":0,"width":32,"height":32}]}]}`,
		"near_tiles too big": `{"properties":[{"name":"near_tiles","value":11}],"width":1,"height":1,"tilewidth":32,"tileheight":32,"layers":[
			{"name":"collision","type":"tilelayer","data":[0]},
			{"name":"spawn","objects":[{"x":0,"y":0,"width":32,"height":32}]}]}`,
		"near_tiles not a number": `{"properties":[{"name":"near_tiles","value":"near"}],"width":1,"height":1,"tilewidth":32,"tileheight":32,"layers":[
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
