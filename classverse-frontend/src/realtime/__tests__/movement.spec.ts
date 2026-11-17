import { describe, expect, it } from 'vitest'

import { directionOf, moveToward, PREDICTION_FACTOR, step } from '../movement'
import { findPath, nearestOpen } from '../pathfind'
import { smallMap } from './fixtures'
import { MapData } from '../mapdata'

const map = smallMap(['#####', '#...#', '#.#.#', '#...#', '#####'])

describe('MapData', () => {
  it('blocks walls and outside', () => {
    expect(map.isBlocked(48, 48)).toBe(false)
    expect(map.isBlocked(80, 80)).toBe(true)
    expect(map.isBlocked(-1, 40)).toBe(true)
    expect(map.pathClear(48, 48, 112, 48)).toBe(true)
    expect(map.pathClear(48, 80, 112, 80)).toBe(false)
  })

  it('parses buildings with doors', () => {
    expect(map.buildings).toEqual([
      expect.objectContaining({ key: 'kelas-a', name: 'Gedung Kelas A', doorX: 64, doorY: 48 }),
    ])
  })
})

describe('movement', () => {
  it('reads near_tiles like the server: 5 tiles by default, per map otherwise', () => {
    expect(map.nearRadius).toBe(160)
    const room = new MapData({
      ...map.raw,
      properties: [{ name: 'near_tiles', type: 'int', value: 3 }],
    })
    expect(room.nearRadius).toBe(96)
  })

  it('uses the map walk_speed so campus walks faster than default', () => {
    const fast = smallMap(['......', '......'], 320)
    expect(fast.walkSpeed).toBe(320)
    expect(map.walkSpeed).toBe(160)
    const next = step(fast, { x: 16, y: 16 }, 1, 0, 100)
    expect(next.x - 16).toBeCloseTo((320 * PREDICTION_FACTOR) / 10)
  })

  it('walks at WALK_SPEED and slides along walls', () => {
    const from = { x: 48, y: 48 }
    const next = step(map, from, 1, 0, 100)
    expect(next.x - from.x).toBeCloseTo((160 * PREDICTION_FACTOR) / 10)
    // Diagonal into the wall below-right slides horizontally.
    const slid = step(map, { x: 112, y: 48 }, 1, 1, 100)
    expect(slid.y).toBeGreaterThan(48)
    expect(map.isBlocked(slid.x, slid.y)).toBe(false)
  })

  it('keeps speed stable at high frame rates', () => {
    const free = smallMap(['....................'])
    let pos = { x: 16, y: 16 }
    for (let i = 0; i < 144; i++) pos = step(free, pos, 1, 0, 1000 / 144)
    expect(pos.x - 16).toBeCloseTo(160 * PREDICTION_FACTOR)
  })

  it('stays put when the only way is a wall', () => {
    expect(step(map, { x: 48, y: 48 }, -1, 0, 1000)).toEqual({ x: 48, y: 48 })
  })

  it('moves toward a target without overshooting', () => {
    expect(moveToward(map, { x: 48, y: 48 }, { x: 52, y: 48 }, 100)).toEqual({ x: 52, y: 48 })
  })

  it('picks a facing direction', () => {
    expect(directionOf(1, 0.2, 'down')).toBe('right')
    expect(directionOf(0, -1, 'down')).toBe('up')
    expect(directionOf(0, 0, 'left')).toBe('left')
  })
})

describe('findPath', () => {
  it('goes around a wall', () => {
    const path = findPath(map, { x: 48, y: 80 }, { x: 112, y: 80 })
    expect(path.length).toBeGreaterThan(1)
    let at = { x: 48, y: 80 }
    for (const wp of path) {
      expect(map.pathClear(at.x, at.y, wp.x, wp.y)).toBe(true)
      at = wp
    }
    expect(at).toEqual({ x: 112, y: 80 })
  })

  it('walks around penalised tiles when a detour exists, through them otherwise', () => {
    const open = smallMap(['.....', '.....', '.....'])
    // Middle column of the middle row is a "table" to avoid.
    const table = (tx: number, ty: number) => (tx === 2 && ty === 1 ? 20 : 0)
    const tiles = (p: { x: number; y: number }[]) => {
      // Sample every 8 px like the scene walks, from the start.
      let at = { x: 16, y: 48 }
      const out = new Set<string>()
      for (const wp of p) {
        const steps = Math.ceil(Math.hypot(wp.x - at.x, wp.y - at.y) / 8)
        for (let s = 0; s <= steps; s++) {
          const t = steps ? s / steps : 1
          out.add(`${Math.floor((at.x + (wp.x - at.x) * t) / 32)},${Math.floor((at.y + (wp.y - at.y) * t) / 32)}`)
        }
        at = wp
      }
      return out
    }
    const around = findPath(open, { x: 16, y: 48 }, { x: 144, y: 48 }, table)
    expect(tiles(around).has('2,1')).toBe(false)
    expect(around[around.length - 1]).toEqual({ x: 144, y: 48 })

    const corridor = smallMap(['#####', '.....', '#####'])
    const through = findPath(corridor, { x: 16, y: 48 }, { x: 144, y: 48 }, table)
    expect(through[through.length - 1]).toEqual({ x: 144, y: 48 })
  })

  it('returns nothing for a blocked target', () => {
    expect(findPath(map, { x: 48, y: 48 }, { x: 80, y: 80 })).toEqual([])
  })
})

describe('nearestOpen', () => {
  it('keeps an open point and moves a blocked one to the closest open tile centre', () => {
    expect(nearestOpen(map, { x: 48, y: 48 })).toEqual({ x: 48, y: 48 })
    expect(nearestOpen(map, { x: 80, y: 84 })).toEqual({ x: 80, y: 112 })
    expect(nearestOpen(smallMap(['###', '###']), { x: 16, y: 16 })).toBeNull()
  })
})
