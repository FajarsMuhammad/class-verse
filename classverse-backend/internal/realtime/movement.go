package realtime

import (
	"math"
	"time"
)

// Movement limits (docs/design.md, slice 3). Speed is enforced as a distance
// budget (token bucket) so two moves that arrive close together because of
// network jitter are not rejected, while the average speed stays bounded.
const (
	defaultWalkSpeed = 160  // px per second when a map sets no walk_speed
	minWalkSpeed     = 80   // bounds for the walk_speed map property
	maxWalkSpeed     = 400  //
	speedTolerance   = 1.25 //
	burstSeconds     = 0.5  // at most half a second of movement in one burst
	slackPx          = 8.0  // rounding and prediction slack
	maxMovesPerSec   = 15   // extra moves are dropped
)

// stride tracks the movement budget of one avatar.
type stride struct {
	budget float64
	rate   float64 // px per second, walk speed with tolerance
	at     time.Time
}

// newStride starts a full budget for a map walked at speed px per second.
func newStride(speed int, now time.Time) stride {
	rate := float64(speed) * speedTolerance
	return stride{budget: rate * burstSeconds, rate: rate, at: now}
}

// try reports whether moving from (fromX, fromY) to (toX, toY) at now is
// allowed on m, and spends the budget if so.
func (s *stride) try(m *MapData, fromX, fromY, toX, toY int, now time.Time) bool {
	if elapsed := now.Sub(s.at); elapsed > 0 {
		s.budget = math.Min(s.rate*burstSeconds, s.budget+elapsed.Seconds()*s.rate)
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
