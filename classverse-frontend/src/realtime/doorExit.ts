// Entering a room spawns at its door. Arm only after walking away; one return to
// the door sends one travel request. The server still validates the position.
export function doorExit(armed: boolean, distance: number, enabled: boolean) {
  if (!enabled || !Number.isFinite(distance)) return { armed: false, exit: false }
  if (distance > 56) return { armed: true, exit: false }
  if (armed && distance <= 40) return { armed: false, exit: true }
  return { armed, exit: false }
}
