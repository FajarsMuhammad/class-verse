import type { MapData } from './mapdata'
import { Movement, type Direction } from './messages'

// Local walking speed stays a little under the server limit so prediction
// never outruns what the server accepts.
export const WALK_SPEED = Movement.maxSpeed * 0.95
// Minimum interval between move messages (server accepts 15 per second).
export const SEND_INTERVAL_MS = 80

export interface Vec {
  x: number
  y: number
}

// step moves from `from` toward input direction (dx, dy) for dtMs, sliding
// along walls: if the diagonal is blocked it tries each axis alone.
export function step(map: MapData, from: Vec, dx: number, dy: number, dtMs: number): Vec {
  if (dx === 0 && dy === 0) return from
  const len = Math.hypot(dx, dy)
  const dist = (WALK_SPEED * dtMs) / 1000
  const nx = from.x + (dx / len) * dist
  const ny = from.y + (dy / len) * dist
  const candidates: Vec[] = [
    { x: nx, y: ny },
    { x: nx, y: from.y },
    { x: from.x, y: ny },
  ]
  for (const c of candidates) {
    if ((c.x !== from.x || c.y !== from.y) && map.pathClear(from.x, from.y, c.x, c.y)) return c
  }
  return from
}

// moveToward walks along a straight line toward target, at most dtMs worth.
export function moveToward(map: MapData, from: Vec, target: Vec, dtMs: number): Vec {
  const dist = Math.hypot(target.x - from.x, target.y - from.y)
  const max = (WALK_SPEED * dtMs) / 1000
  if (dist <= max) return map.pathClear(from.x, from.y, target.x, target.y) ? target : from
  const t = max / dist
  const next = {
    x: from.x + (target.x - from.x) * t,
    y: from.y + (target.y - from.y) * t,
  }
  return map.pathClear(from.x, from.y, next.x, next.y) ? next : from
}

export function directionOf(dx: number, dy: number, fallback: Direction): Direction {
  if (dx === 0 && dy === 0) return fallback
  if (Math.abs(dx) >= Math.abs(dy)) return dx > 0 ? 'right' : 'left'
  return dy > 0 ? 'down' : 'up'
}
