// Package realtime runs the WebSocket gateway and the in-memory rooms
// (one goroutine per room, server-authoritative movement).
package realtime

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
)

// MapData is the server's view of a Tiled map: walkability, spawn area, and
// buildings. It is immutable after parsing and safe to share between rooms.
type MapData struct {
	Key       string
	Width     int // tiles
	Height    int // tiles
	TileSize  int // pixels
	blocked   []bool
	Spawn     Rect // pixels
	Buildings []Building
	// Exit is the point to stand on to leave a classroom or garden; HasExit
	// is false on the campus.
	ExitX, ExitY int
	HasExit      bool
	// Zones are discussion tables (classroom/garden); empty on the campus.
	Zones []Zone
	// WalkSpeed is the maximum walking speed in px per second (map property
	// walk_speed). The client reads the same property for prediction.
	WalkSpeed int
}

// Building returns the building with key.
func (m *MapData) Building(key string) (Building, bool) {
	for _, b := range m.Buildings {
		if b.Key == key {
			return b, true
		}
	}
	return Building{}, false
}

// Rect is an axis-aligned rectangle in pixels.
type Rect struct{ X, Y, W, H int }

// Contains reports whether the pixel point lies inside r (right/bottom open).
func (r Rect) Contains(x, y int) bool { return x >= r.X && y >= r.Y && x < r.X+r.W && y < r.Y+r.H }

// Zone is a discussion area: everyone inside shares zone chat.
type Zone struct {
	Key  string
	Name string
	Area Rect
}

// ZoneAt returns the zone containing (x, y), or the zero Zone.
func (m *MapData) ZoneAt(x, y int) Zone {
	for _, z := range m.Zones {
		if z.Area.Contains(x, y) {
			return z
		}
	}
	return Zone{}
}

// Building is a building on the campus map with its door point.
type Building struct {
	Key       string
	Name      string
	StaffOnly bool
	Area      Rect
	// Door is where travellers stand to enter and where they return.
	DoorX int
	DoorY int
}

type tmjMap struct {
	Properties []tmjProperty `json:"properties"`
	Width      int           `json:"width"`
	Height     int           `json:"height"`
	TileWidth  int           `json:"tilewidth"`
	TileHeight int           `json:"tileheight"`
	Layers     []tmjLayer    `json:"layers"`
}

type tmjLayer struct {
	Name    string      `json:"name"`
	Type    string      `json:"type"`
	Width   int         `json:"width"`
	Height  int         `json:"height"`
	Data    []int       `json:"data"`
	Objects []tmjObject `json:"objects"`
}

type tmjObject struct {
	Name       string        `json:"name"`
	Type       string        `json:"type"`
	X          float64       `json:"x"`
	Y          float64       `json:"y"`
	Width      float64       `json:"width"`
	Height     float64       `json:"height"`
	Properties []tmjProperty `json:"properties"`
}

type tmjProperty struct {
	Name  string          `json:"name"`
	Value json.RawMessage `json:"value"`
}

func (o tmjObject) str(name string) string {
	for _, p := range o.Properties {
		if p.Name == name {
			var s string
			_ = json.Unmarshal(p.Value, &s)
			return s
		}
	}
	return ""
}

func (o tmjObject) boolean(name string) bool {
	for _, p := range o.Properties {
		if p.Name == name {
			var b bool
			_ = json.Unmarshal(p.Value, &b)
			return b
		}
	}
	return false
}

// ParseMap reads a Tiled JSON map (see maps/README.md for the required layers).
func ParseMap(key string, raw []byte) (*MapData, error) {
	var m tmjMap
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("parse map %s: %w", key, err)
	}
	if m.Width <= 0 || m.Height <= 0 || m.TileWidth <= 0 || m.TileWidth != m.TileHeight {
		return nil, fmt.Errorf("parse map %s: invalid size", key)
	}
	md := &MapData{Key: key, Width: m.Width, Height: m.Height, TileSize: m.TileWidth, WalkSpeed: defaultWalkSpeed}
	for _, p := range m.Properties {
		if p.Name != "walk_speed" {
			continue
		}
		if json.Unmarshal(p.Value, &md.WalkSpeed) != nil || md.WalkSpeed < minWalkSpeed || md.WalkSpeed > maxWalkSpeed {
			return nil, fmt.Errorf("parse map %s: walk_speed must be an integer %d-%d", key, minWalkSpeed, maxWalkSpeed)
		}
	}
	var haveCollision, haveSpawn bool
	doors := map[string][2]int{}
	for _, l := range m.Layers {
		switch l.Name {
		case "collision":
			if l.Type != "tilelayer" || len(l.Data) != m.Width*m.Height {
				return nil, fmt.Errorf("parse map %s: collision layer must be a %dx%d tile layer", key, m.Width, m.Height)
			}
			md.blocked = make([]bool, len(l.Data))
			for i, gid := range l.Data {
				md.blocked[i] = gid != 0
			}
			haveCollision = true
		case "spawn":
			if len(l.Objects) == 0 {
				return nil, fmt.Errorf("parse map %s: spawn layer is empty", key)
			}
			o := l.Objects[0]
			md.Spawn = Rect{X: int(o.X), Y: int(o.Y), W: int(o.Width), H: int(o.Height)}
			haveSpawn = true
		case "buildings":
			for _, o := range l.Objects {
				switch o.Type {
				case "building":
					md.Buildings = append(md.Buildings, Building{
						Key: o.str("key"), Name: o.str("name"), StaffOnly: o.boolean("staff_only"),
						Area: Rect{X: int(o.X), Y: int(o.Y), W: int(o.Width), H: int(o.Height)},
					})
				case "door":
					doors[o.str("building")] = [2]int{int(math.Round(o.X)), int(math.Round(o.Y))}
				}
			}
		case "zones":
			for _, o := range l.Objects {
				md.Zones = append(md.Zones, Zone{
					Key: o.str("key"), Name: o.Name,
					Area: Rect{X: int(o.X), Y: int(o.Y), W: int(o.Width), H: int(o.Height)},
				})
			}
		case "exit":
			if len(l.Objects) != 1 {
				return nil, fmt.Errorf("parse map %s: exit layer must have one point", key)
			}
			md.ExitX, md.ExitY, md.HasExit = int(math.Round(l.Objects[0].X)), int(math.Round(l.Objects[0].Y)), true
		}
	}
	if !haveCollision || !haveSpawn {
		return nil, fmt.Errorf("parse map %s: collision and spawn layers are required", key)
	}
	seen := map[string]bool{}
	for i := range md.Buildings {
		b := &md.Buildings[i]
		if b.Key == "" || seen[b.Key] {
			return nil, fmt.Errorf("parse map %s: building key %q missing or duplicated", key, b.Key)
		}
		seen[b.Key] = true
		d, ok := doors[b.Key]
		if !ok {
			return nil, fmt.Errorf("parse map %s: building %q has no door", key, b.Key)
		}
		if md.Blocked(d[0], d[1]) {
			return nil, fmt.Errorf("parse map %s: door of %q is not walkable", key, b.Key)
		}
		b.DoorX, b.DoorY = d[0], d[1]
	}
	zoneKeys := map[string]bool{}
	for _, z := range md.Zones {
		a := z.Area
		if z.Key == "" || zoneKeys[z.Key] || a.W <= 0 || a.H <= 0 || a.X < 0 || a.Y < 0 ||
			a.X+a.W > md.PixelWidth() || a.Y+a.H > md.PixelHeight() {
			return nil, fmt.Errorf("parse map %s: zone %q invalid, duplicated, or outside the map", key, z.Key)
		}
		zoneKeys[z.Key] = true
	}
	if md.HasExit && md.Blocked(md.ExitX, md.ExitY) {
		return nil, fmt.Errorf("parse map %s: exit is not walkable", key)
	}
	if key != campusKey && !md.HasExit {
		return nil, fmt.Errorf("parse map %s: exit layer is required", key)
	}
	if _, _, err := md.SpawnPoint(0); err != nil {
		return nil, fmt.Errorf("parse map %s: %w", key, err)
	}
	return md, nil
}

// PixelWidth is the world width in pixels.
func (m *MapData) PixelWidth() int { return m.Width * m.TileSize }

// PixelHeight is the world height in pixels.
func (m *MapData) PixelHeight() int { return m.Height * m.TileSize }

// Blocked reports whether the pixel point is outside the map or on a
// collision tile.
func (m *MapData) Blocked(x, y int) bool {
	if x < 0 || y < 0 || x >= m.PixelWidth() || y >= m.PixelHeight() {
		return true
	}
	return m.blocked[(y/m.TileSize)*m.Width+x/m.TileSize]
}

// PathClear reports whether the straight segment between two points stays on
// walkable tiles, sampling every 8 px.
func (m *MapData) PathClear(x0, y0, x1, y1 int) bool {
	dx, dy := float64(x1-x0), float64(y1-y0)
	steps := int(math.Ceil(math.Hypot(dx, dy) / 8))
	for i := 0; i <= steps; i++ {
		t := 1.0
		if steps > 0 {
			t = float64(i) / float64(steps)
		}
		if m.Blocked(x0+int(math.Round(dx*t)), y0+int(math.Round(dy*t))) {
			return false
		}
	}
	return true
}

var errNoSpawn = errors.New("spawn area has no walkable tile")

// SpawnPoint returns the centre of a walkable tile inside the spawn area. n
// spreads players over the free tiles so they do not stack on one point.
func (m *MapData) SpawnPoint(n int) (int, int, error) {
	ts := m.TileSize
	var free [][2]int
	for ty := m.Spawn.Y / ts; ty*ts < m.Spawn.Y+m.Spawn.H; ty++ {
		for tx := m.Spawn.X / ts; tx*ts < m.Spawn.X+m.Spawn.W; tx++ {
			x, y := tx*ts+ts/2, ty*ts+ts/2
			if !m.Blocked(x, y) {
				free = append(free, [2]int{x, y})
			}
		}
	}
	if len(free) == 0 {
		return 0, 0, errNoSpawn
	}
	if n < 0 {
		n = -n
	}
	p := free[n%len(free)]
	return p[0], p[1], nil
}
