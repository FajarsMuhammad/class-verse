package realtime

import (
	"math"
	"time"
)

// Movement limits (docs/design.md, slice 3). Speed is enforced as a distance
// budget (token bucket) so two moves that arrive close together because of
// network jitter are not rejected, while the average speed stays bounded.
const (
	maxSpeed       = 160.0                     // px per second
	speedTolerance = 1.25                      //
	budgetRate     = maxSpeed * speedTolerance // px per second
	budgetCap      = budgetRate * 0.5          // at most half a second of movement in one burst
	slackPx        = 8.0                       // rounding and prediction slack
	maxMovesPerSec = 15                        // extra moves are dropped
)

// stride tracks the movement budget of one avatar.
type stride struct {
	budget float64
	at     time.Time
}

func newStride(now time.Time) stride {
	return stride{budget: budgetCap, at: now}
}

// try reports whether moving from (fromX, fromY) to (toX, toY) at now is
// allowed on m, and spends the budget if so.
func (s *stride) try(m *MapData, fromX, fromY, toX, toY int, now time.Time) bool {
	if elapsed := now.Sub(s.at); elapsed > 0 {
		s.budget = math.Min(budgetCap, s.budget+elapsed.Seconds()*budgetRate)
	}
	s.at = now
	if m.Blocked(toX, toY) {
		return false
	}
	dist := math.Hypot(float64(toX-fromX), float64(toY-fromY))
	if dist > s.budget+slackPx {
		return false
	}
	if !m.PathClear(fromX, fromY, toX, toY) {
		return false
	}
	// Preserve slack debt; resetting to zero would grant 8 free pixels on
	// every message once the budget is exhausted.
	s.budget -= dist
	return true
}

// moveBudget is a one-second fixed window counter for move messages.
type moveBudget struct {
	windowStart time.Time
	count       int
}

func (b *moveBudget) allow(now time.Time) bool {
	if now.Sub(b.windowStart) >= time.Second {
		b.windowStart = now
		b.count = 0
	}
	b.count++
	return b.count <= maxMovesPerSec
}
