import { describe, expect, it } from 'vitest'

import { directionOf, moveToward, step, WALK_SPEED } from '../movement'
import { findPath } from '../pathfind'
import { smallMap } from './fixtures'

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
  it('walks at WALK_SPEED and slides along walls', () => {
    const from = { x: 48, y: 48 }
    const next = step(map, from, 1, 0, 100)
    expect(next.x - from.x).toBeCloseTo(WALK_SPEED / 10)
    // Diagonal into the wall below-right slides horizontally.
    const slid = step(map, { x: 112, y: 48 }, 1, 1, 100)
    expect(slid.y).toBeGreaterThan(48)
    expect(map.isBlocked(slid.x, slid.y)).toBe(false)
  })

  it('keeps speed stable at high frame rates', () => {
    const free = smallMap(['....................'])
    let pos = { x: 16, y: 16 }
    for (let i = 0; i < 144; i++) pos = step(free, pos, 1, 0, 1000 / 144)
    expect(pos.x - 16).toBeCloseTo(WALK_SPEED)
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

  it('returns nothing for a blocked target', () => {
    expect(findPath(map, { x: 48, y: 48 }, { x: 80, y: 80 })).toEqual([])
  })
})
