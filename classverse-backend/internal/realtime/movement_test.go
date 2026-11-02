package realtime

import (
	"testing"
	"time"
)

func TestStride(t *testing.T) {
	m := loadCampus(t)
	x, y, _ := m.SpawnPoint(0)
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	a := m.Buildings[0]

	tests := []struct {
		name   string
		steps  [][3]int // dx, dy, ms since previous step
		wantOK []bool
	}{
		{"normal walk", [][3]int{{16, 0, 100}, {16, 0, 100}, {16, 0, 100}}, []bool{true, true, true}},
		{"burst within budget then refill", [][3]int{{80, 0, 0}, {40, 0, 0}, {40, 0, 300}}, []bool{true, false, true}},
		{"teleport", [][3]int{{400, 0, 100}}, []bool{false}},
		{"sustained too fast", [][3]int{{60, 0, 100}, {60, 0, 100}, {60, 0, 100}}, []bool{true, true, false}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newStride(defaultWalkSpeed, t0)
			cx, cy, now := x, y, t0
			for i, st := range tt.steps {
				now = now.Add(time.Duration(st[2]) * time.Millisecond)
				nx, ny := cx+st[0], cy+st[1]
				ok := s.try(m, cx, cy, nx, ny, now)
				if ok != tt.wantOK[i] {
					t.Fatalf("step %d: ok=%v want %v (budget %.1f)", i, ok, tt.wantOK[i], s.budget)
				}
				if ok {
					cx, cy = nx, ny
				}
			}
		})
	}

	t.Run("slack cannot be spent repeatedly without refill", func(t *testing.T) {
		s := newStride(defaultWalkSpeed, t0)
		if !s.try(m, x, y, x+100, y, t0) || !s.try(m, x+100, y, x+108, y, t0) {
			t.Fatal("initial budget plus slack should be accepted")
		}
		if s.try(m, x+108, y, x+116, y, t0) {
			t.Fatal("slack granted free repeated movement")
		}
	})

	t.Run("into a wall", func(t *testing.T) {
		s := newStride(defaultWalkSpeed, t0)
		if s.try(m, a.DoorX, a.DoorY, a.DoorX, a.DoorY-40, t0.Add(time.Second)) {
			t.Error("walked into building")
		}
	})
	t.Run("outside the map", func(t *testing.T) {
		s := newStride(defaultWalkSpeed, t0)
		if s.try(m, 40, 40, -4, 40, t0.Add(time.Second)) {
			t.Error("walked out of map")
		}
	})
}

func TestStrideScalesWithWalkSpeed(t *testing.T) {
	m := loadCampus(t)
	x, y, _ := m.SpawnPoint(0)
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	// 32 px per 100 ms is 320 px/s: legal at walk_speed 320, too fast at 160.
	for _, tt := range []struct {
		name  string
		speed int
		want  bool
	}{
		{"default rejects sustained 320 px/s", defaultWalkSpeed, false},
		{"campus speed accepts 320 px/s", 320, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			s := newStride(tt.speed, t0)
			cx, now, ok := x, t0, true
			for range 10 {
				now = now.Add(100 * time.Millisecond)
				if !s.try(m, cx, y, cx+32, y, now) {
					ok = false
					break
				}
				cx += 32
			}
			if ok != tt.want {
				t.Fatalf("ok=%v want %v", ok, tt.want)
			}
		})
	}
}

func TestMoveBudget(t *testing.T) {
	var b moveBudget
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	ok := 0
	for i := range 20 {
		if b.allow(t0.Add(time.Duration(i) * 10 * time.Millisecond)) {
			ok++
		}
	}
	if ok != maxMovesPerSec {
		t.Errorf("allowed %d in one second, want %d", ok, maxMovesPerSec)
	}
	if !b.allow(t0.Add(time.Second)) {
		t.Error("new window should allow")
	}
}

func TestAOI(t *testing.T) {
	m := loadCampus(t)
	ts := m.TileSize
	if !inAOI(m, 0, 0, 23*ts, 23*ts) {
		t.Error("23 tiles away should be visible")
	}
	if inAOI(m, 0, 0, 33*ts, 0) {
		t.Error("33 tiles away should be hidden")
	}
}
