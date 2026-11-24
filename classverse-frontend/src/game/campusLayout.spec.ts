import { describe, expect, it } from 'vitest'
import { ALUN, ARENA, TAMAN, campusScenery, footprints, inRect, roadDist } from './campusLayout'
import world from '@/assets/world/world.json'

const doors = world.places.map((p) => [p.entrance.x, p.entrance.y] as const)

describe('campus scenery', () => {
  const { trees, decor, setPieces } = campusScenery()

  it('is identical on every call, so all players see the same trees', () => {
    expect(campusScenery()).toBe(campusScenery())
    expect(trees.length).toBeGreaterThan(60)
    expect(decor.some((d) => d.t === 'fountain')).toBe(true)
  })

  // Palms around the arena and the twin banyans are hand-placed set pieces; every random tree follows the rules.
  const scattered = trees.slice(setPieces)

  it('keeps trees off roads, doors, and building facades', () => {
    for (const t of scattered) {
      expect(roadDist(t.x, t.y), `road ${t.x},${t.y}`).toBeGreaterThanOrEqual(36)
      for (const [dx, dy] of doors) expect(Math.hypot(t.x - dx, t.y - dy), `door ${t.x},${t.y}`).toBeGreaterThanOrEqual(80)
      for (const b of footprints) {
        const inFront = t.x > b.x - t.r * 0.8 && t.x < b.x + b.w + t.r * 0.8 && t.y > b.y - 30 && t.y < b.y + b.h + t.r * 2.3
        expect(inFront, `facade ${t.x},${t.y}`).toBe(false)
      }
    }
  })

  it('leaves the garden, plaza (except the twin banyans), and arena field open', () => {
    const banyans = trees.filter((t) => t.type === 'beri')
    expect(banyans).toHaveLength(2)
    expect(setPieces).toBe(12)
    const field = [ALUN, { x: ARENA.x, y: ARENA.y, w: ARENA.w, h: ARENA.h }]
    // Garden shrubs are intentional; scattered trees never land inside the plaza or the arena.
    for (const t of scattered.filter((t) => t.type !== 'bush' && !inRect(t.x, t.y, TAMAN)))
      expect(field.some((r) => inRect(t.x, t.y, r)), `${t.type} ${t.x},${t.y}`).toBe(false)
  })

  it('never stacks props on top of each other', () => {
    for (let i = 0; i < decor.length; i++)
      for (let j = i + 1; j < decor.length; j++) {
        const a = decor[i]!, b = decor[j]!
        if (['fountain', 'pendopo', 'board', 'flag', 'score'].includes(a.t) || ['fountain', 'pendopo', 'board', 'flag', 'score'].includes(b.t)) continue
        expect(Math.hypot(a.x - b.x, a.y - b.y), `${a.t}/${b.t}`).toBeGreaterThanOrEqual(26)
      }
  })
})
