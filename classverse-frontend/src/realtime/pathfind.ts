import type { MapData } from './mapdata'
import type { Vec } from './movement'

const MAX_EXPANDED = 6000

// findPath returns waypoints (tile centres, in pixels) from `from` to `to`
// using A* on the collision grid with 8-way moves that never cut corners.
// It returns [] when the target is blocked or unreachable.
export function findPath(map: MapData, from: Vec, to: Vec): Vec[] {
  const ts = map.tileSize
  const sx = Math.floor(from.x / ts)
  const sy = Math.floor(from.y / ts)
  const gx = Math.floor(to.x / ts)
  const gy = Math.floor(to.y / ts)
  if (map.isTileBlocked(gx, gy)) return []
  if (sx === gx && sy === gy) return [{ x: to.x, y: to.y }]

  const w = map.width
  const key = (x: number, y: number) => y * w + x
  const g = new Map<number, number>([[key(sx, sy), 0]])
  const came = new Map<number, number>()
  const open: { k: number; f: number }[] = [{ k: key(sx, sy), f: 0 }]
  const closed = new Set<number>()
  const h = (x: number, y: number) => {
    const dx = Math.abs(x - gx)
    const dy = Math.abs(y - gy)
    return Math.max(dx, dy) + (Math.SQRT2 - 1) * Math.min(dx, dy)
  }

  while (open.length > 0 && closed.size < MAX_EXPANDED) {
    let best = 0
    for (let i = 1; i < open.length; i++) if (open[i]!.f < open[best]!.f) best = i
    const { k } = open.splice(best, 1)[0]!
    if (closed.has(k)) continue
    closed.add(k)
    const cx = k % w
    const cy = Math.floor(k / w)
    if (cx === gx && cy === gy) return smooth(map, from, rebuild(came, k, w, ts, to))
    for (let dy = -1; dy <= 1; dy++) {
      for (let dx = -1; dx <= 1; dx++) {
        if (dx === 0 && dy === 0) continue
        const nx = cx + dx
        const ny = cy + dy
        if (map.isTileBlocked(nx, ny)) continue
        if (
          dx !== 0 &&
          dy !== 0 &&
          (map.isTileBlocked(cx + dx, cy) || map.isTileBlocked(cx, cy + dy))
        )
          continue
        const nk = key(nx, ny)
        if (closed.has(nk)) continue
        const cost = (g.get(k) ?? 0) + (dx !== 0 && dy !== 0 ? Math.SQRT2 : 1)
        if (cost < (g.get(nk) ?? Infinity)) {
          g.set(nk, cost)
          came.set(nk, k)
          open.push({ k: nk, f: cost + h(nx, ny) })
        }
      }
    }
  }
  return []
}

function rebuild(came: Map<number, number>, end: number, w: number, ts: number, to: Vec): Vec[] {
  const path: Vec[] = []
  let k: number | undefined = end
  while (k !== undefined && came.has(k)) {
    path.push({ x: (k % w) * ts + ts / 2, y: Math.floor(k / w) * ts + ts / 2 })
    k = came.get(k)
  }
  path.reverse()
  path[path.length - 1] = { x: to.x, y: to.y }
  return path
}

// smooth drops waypoints that can be skipped with a clear straight line.
function smooth(map: MapData, from: Vec, path: Vec[]): Vec[] {
  const out: Vec[] = []
  let anchor = from
  let i = 0
  while (i < path.length) {
    let j = path.length - 1
    while (j > i && !map.pathClear(anchor.x, anchor.y, path[j]!.x, path[j]!.y)) j--
    out.push(path[j]!)
    anchor = path[j]!
    i = j + 1
  }
  return out
}
