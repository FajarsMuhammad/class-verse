package realtime

import (
	"testing"

	"github.com/FajarsMuhammad/class-verse/classverse-backend/maps"
)

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
