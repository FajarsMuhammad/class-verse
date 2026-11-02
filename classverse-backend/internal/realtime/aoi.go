package realtime

// Area of interest on the campus map: a grid of 8×8-tile cells; a receiver
// sees avatars within 24 tiles (3 cells) in each direction.
const (
	aoiCellTiles   = 8
	aoiRadiusTiles = 24
)

type cell struct{ cx, cy int }

func cellOf(m *MapData, x, y int) cell {
	size := aoiCellTiles * m.TileSize
	return cell{x / size, y / size}
}

// inAOI reports whether b is within the receiver a's area of interest.
// Using cells (not exact distance) keeps the set stable while walking.
func inAOI(m *MapData, ax, ay, bx, by int) bool {
	a, b := cellOf(m, ax, ay), cellOf(m, bx, by)
	r := aoiRadiusTiles / aoiCellTiles
	return abs(a.cx-b.cx) <= r && abs(a.cy-b.cy) <= r
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
