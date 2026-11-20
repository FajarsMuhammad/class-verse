// Campus scenery layout: roads, set pieces, trees, and decor. Pure data + seeded placement (no Phaser, no DOM),
// so every player sees the same trees and the rules can be unit tested. Pure visuals: nothing here is collision;
// the server's campus.tmj still comes only from world.json places/zones (see scripts/generate_world.py).
import world from '@/assets/world/world.json'

export type Pt = [number, number]
export type Seg = [Pt, Pt]
export type Rect = { x: number; y: number; w: number; h: number }
export type TreeKind = 'round' | 'bloom' | 'pine' | 'palm' | 'beri' | 'bush'
export type TreePalette = 'g1' | 'g2' | 'pink' | 'orange' | 'beri'
export interface Tree { x: number; y: number; type: TreeKind; r: number; pal: TreePalette; ph: number; sw: number }
export type DecorKind = 'lamp' | 'bench' | 'pendopo' | 'flag' | 'board' | 'score' | 'aflag' | 'fountain' | 'easel' | 'sign' | 'scope' | 'bikes'
export interface Decor { t: DecorKind; x: number; y: number; cl?: string }

export const W = world.size.w
export const H = world.size.h

// Main roads (centre lines). Chosen to avoid every building footprint in world.json.
export const ROADS: Seg[] = [
  [[200, 440], [2000, 440]], [[200, 440], [200, 640]], [[200, 940], [200, 1415]], [[2000, 440], [2000, 628]], [[2000, 898], [2000, 1415]],
  [[200, 1110], [2000, 1110]], [[200, 1415], [2000, 1415]],
  [[395, 440], [395, 640]], [[395, 940], [395, 1110]], [[1100, 440], [1100, 640]], [[1100, 1020], [1100, 1110]],
  [[1780, 440], [1780, 628]], [[1780, 898], [1780, 1110]], [[600, 830], [800, 830]], [[1400, 830], [1600, 830]],
  [[755, 1110], [755, 1415]], [[1430, 1110], [1430, 1415]],
]

const places = world.places
const placeById = (id: string) => places.find((p) => p.id === id)!
const doorOf = (id: string): Pt => { const e = placeById(id).entrance; return [e.x, e.y] }

// Short spur from each entrance down/up to its road (the road a building faces).
export const SPURS: Seg[] = [
  [doorOf('angka'), [doorOf('angka')[0], 440]],
  [doorOf('ruang-guru'), [doorOf('ruang-guru')[0], 440]],
  [doorOf('sains'), [doorOf('sains')[0], 440]],
  [doorOf('perpus'), [doorOf('perpus')[0], 440]],
  [doorOf('galeri'), [doorOf('galeri')[0], 1415]],
  [doorOf('studio'), [doorOf('studio')[0], 1415]],
  [doorOf('kelas'), [doorOf('kelas')[0], 1415]],
]

export const footprints: Rect[] = places.map((p) => ({ x: p.sprite_pos.x, y: p.sprite_pos.y, w: p.size.w, h: p.size.h }))
export const ARENA = { ...placeById('arena').sprite_pos, ...placeById('arena').size, cx: placeById('arena').entrance.x, cy: placeById('arena').entrance.y - 7 }
export const arenaRect: Rect = { x: ARENA.x, y: ARENA.y, w: ARENA.w, h: ARENA.h }
const zoneRect = (id: string) => world.zones.find((z) => z.id === id || z.name.toLowerCase().includes(id))!.rect
export const TAMAN: Rect = zoneRect('taman')
export const ALUN: Rect = zoneRect('alun')
export const FOUNT = { x: ALUN.x + ALUN.w / 2, y: ALUN.y + 160 }
export const POND = { x: TAMAN.x + 130, y: TAMAN.y + 160, rx: 95, ry: 55 }
const SPOTS = places.map((p) => doorOf(p.id))

export function mulberry32(a: number) {
  return () => {
    a = (a + 0x6d2b79f5) | 0
    let t = Math.imul(a ^ (a >>> 15), 1 | a)
    t = (t + Math.imul(t ^ (t >>> 7), 61 | t)) ^ t
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296
  }
}

export function segDist(px: number, py: number, a: Pt, b: Pt) {
  const dx = b[0] - a[0], dy = b[1] - a[1], l = dx * dx + dy * dy
  const t = l ? Math.max(0, Math.min(1, ((px - a[0]) * dx + (py - a[1]) * dy) / l)) : 0
  return Math.hypot(px - a[0] - t * dx, py - a[1] - t * dy)
}
export const roadDist = (x: number, y: number) => Math.min(...ROADS.concat(SPURS).map((s) => segDist(x, y, s[0], s[1])))
export const inRect = (x: number, y: number, r: Rect, m = 0) => x > r.x - m && x < r.x + r.w + m && y > r.y - m && y < r.y + r.h + m

const T_TYPES: [TreeKind, TreePalette][] = [['round', 'g1'], ['round', 'g1'], ['round', 'g2'], ['pine', 'g1'], ['bloom', 'pink'], ['bloom', 'orange'], ['round', 'g1'], ['pine', 'g1']]

// setPieces: how many leading trees are hand-placed (banyans, arena palms, garden shrubs); the rest follow the placement rules.
export interface CampusScenery { trees: Tree[]; decor: Decor[]; setPieces: number }

let cached: CampusScenery | null = null
// campusScenery places everything once with a fixed seed; the result is identical for every player and every call.
export function campusScenery(): CampusScenery {
  if (cached) return cached
  const rnd = mulberry32(7)
  const R = (a: number, b: number) => a + rnd() * (b - a)
  const pick = <T,>(a: T[]) => a[Math.floor(rnd() * a.length)]!
  const trees: Tree[] = []
  const decor: Decor[] = []

  // A tree never sits on a road, in front of a facade, on a door, or on top of another tree.
  const treeOK = (x: number, y: number, r: number, strict = true) => {
    if (roadDist(x, y) < 36 + r * 0.42) return false
    for (const b of footprints) {
      if (x > b.x - r * 0.8 && x < b.x + b.w + r * 0.8 && y > b.y - 30 && y < b.y + b.h + r * 2.3) return false
      if (x > b.x - r && x < b.x + b.w + r && y > b.y - r * 0.3 && y < b.y + b.h) return false
    }
    if (strict) {
      if (inRect(x, y, TAMAN, r * 0.4) || inRect(x, y, ALUN, r * 0.5)) return false
      if (inRect(x, y + r * 1.2, arenaRect, r * 0.3) || inRect(x, y, arenaRect, r * 0.6)) return false
    }
    for (const s of SPOTS) if (Math.hypot(x - s[0], y - s[1]) < 80) return false
    for (const t of trees) if (Math.hypot(x - t.x, y - t.y) < (r + t.r) * 0.72) return false
    return true
  }
  const addTree = (x: number, y: number, type: TreeKind, r: number, pal: TreePalette) =>
    trees.push({ x, y, type, r, pal, ph: rnd() * 6.28, sw: type === 'bush' ? 0.012 : type === 'palm' ? 0.045 : 0.026 })

  // Set pieces first: twin banyans on the plaza, palms around the arena, shrubs around the garden.
  addTree(FOUNT.x - 218, FOUNT.y - 10, 'beri', 62, 'beri')
  addTree(FOUNT.x + 218, FOUNT.y - 10, 'beri', 62, 'beri')
  for (const [x, y] of [[arenaRect.x - 15, 640], [arenaRect.x + arenaRect.w + 18, 640], [arenaRect.x - 15, 925], [arenaRect.x + arenaRect.w + 18, 925]] as Pt[]) addTree(x, y, 'palm', 38, 'g1')
  for (const [x, y, t, r, p] of [[185, 700, 'bush', 18, 'pink'], [575, 925, 'bloom', 36, 'pink'], [178, 930, 'round', 40, 'g1'], [560, 680, 'bush', 20, 'g1'], [460, 935, 'bush', 16, 'orange'], [330, 690, 'bush', 16, 'g2']] as [number, number, TreeKind, number, TreePalette][]) addTree(x, y, t, r, p)

  const setPieces = trees.length
  // Avenues along the long roads.
  for (let x = 300; x < 2000; x += 300) if (treeOK(x, 1052, 34, false) && !inRect(x, 1052, ALUN, 30)) addTree(x, 1052, 'bloom', 34, Math.floor(x / 300) % 2 ? 'pink' : 'orange')
  for (let x = 280; x < 2000; x += 340) if (treeOK(x, 515, 38)) addTree(x, 515, 'round', 38, 'g1')

  // Thin forest border, then rare interior groves; the middle stays open on purpose.
  for (let i = 0; i < 520; i++) {
    const side = Math.floor(rnd() * 4)
    let x: number, y: number
    if (side === 0) { x = R(-10, W + 10); y = R(30, 150) } else if (side === 1) { x = R(-10, W + 10); y = R(1452, 1530) } else if (side === 2) { x = R(-10, 140); y = R(60, 1500) } else { x = R(W - 140, W + 10); y = R(60, 1500) }
    const [ty, pl] = pick(T_TYPES)
    const r = side === 1 ? R(26, 36) : R(42, 64)
    if (treeOK(x, y, r)) addTree(x, y, side === 1 && rnd() < 0.4 ? 'bush' : ty, side === 1 ? Math.round(r * 0.7) : Math.round(r), pl)
  }
  for (let i = 0; i < 900; i++) {
    const x = R(240, W - 240), y = R(470, 1400)
    if (rnd() < 0.9) continue
    const bush = rnd() < 0.35
    const r = bush ? Math.round(R(14, 22)) : Math.round(R(30, 46))
    const [ty, pl] = pick(T_TYPES)
    if (treeOK(x, y, r)) addTree(x, y, bush ? 'bush' : ty, r, bush ? pick<TreePalette>(['g1', 'g2', 'pink', 'orange']) : pl)
  }

  const decorOK = (x: number, y: number) => {
    for (const b of footprints) if (inRect(x, y, b, 24)) return false
    for (const t of trees) if (Math.hypot(x - t.x, y - t.y) < t.r * 0.6 + 14) return false
    for (const d of decor) if (Math.hypot(x - d.x, y - d.y) < 46) return false
    for (const s of SPOTS) if (Math.hypot(x - s[0], y - s[1]) < 60) return false
    return !inRect(x, y, arenaRect, 10) && !inRect(x, y, TAMAN, 0) && !inRect(x, y, ALUN, 0)
  }
  // Lamps alternate sides along every long road.
  for (const s of ROADS) {
    const [a, b] = s
    const L = Math.hypot(b[0] - a[0], b[1] - a[1])
    if (L < 260) continue
    const ux = (b[0] - a[0]) / L, uy = (b[1] - a[1]) / L
    let side = 1
    for (let d = 110; d < L - 60; d += 230) {
      const x = a[0] + ux * d - uy * 38 * side, y = a[1] + uy * d + ux * 38 * side
      if (decorOK(x, y) && !ROADS.some((o) => o !== s && segDist(x, y, o[0], o[1]) < 60)) decor.push({ t: 'lamp', x, y })
      side = -side
    }
  }
  for (const [x, y] of [[480, 1150], [1010, 1150], [1520, 1150], [930, 480], [1290, 480], [1600, 1380], [560, 1380], [1260, 1380], [1960, 1180], [240, 1180]] as Pt[]) if (decorOK(x, y)) decor.push({ t: 'bench', x, y })
  for (const [x, y] of [[240, 770], [340, 880], [230, 860], [FOUNT.x - 220, FOUNT.y - 100], [FOUNT.x + 220, FOUNT.y - 100], [980, 980], [1220, 980], [1460, 870], [740, 870]] as Pt[]) decor.push({ t: 'bench', x, y })
  decor.push(
    { t: 'pendopo', x: TAMAN.x + 335, y: TAMAN.y + 120 }, { t: 'flag', x: FOUNT.x + 70, y: FOUNT.y - 112 }, { t: 'board', x: FOUNT.x - 82, y: FOUNT.y - 110 },
    { t: 'score', x: ARENA.cx, y: ARENA.y + 24 }, { t: 'fountain', x: FOUNT.x, y: FOUNT.y + 30 },
  )
  for (const [x, y, cl] of [[1640, 700, '#ef4444'], [2020, 700, '#3b82f6'], [1640, 850, '#10b981'], [2020, 850, '#fbbf24']] as [number, number, string][]) decor.push({ t: 'aflag', x, y, cl })
  decor.push({ t: 'easel', x: 1262, y: 1352 }, { t: 'sign', x: 1150, y: 490 }, { t: 'sign', x: 445, y: 1160 }, { t: 'sign', x: 1830, y: 1160 }, { t: 'scope', x: 1240, y: 395 }, { t: 'bikes', x: 1560, y: 1360 })
  cached = { trees, decor, setPieces }
  return cached
}
