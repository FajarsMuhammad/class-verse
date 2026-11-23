import { describe, expect, it } from 'vitest'
import { NAME_RADIUS, badgeFor, visibleNames } from './mapLabels'

describe('visibleNames', () => {
  const self = { x: 0, y: 0 }
  const others = [{ id: 'near', x: 100, y: 0 }, { id: 'edge', x: NAME_RADIUS, y: 0 }, { id: 'far', x: 400, y: 0 }]

  it('shows only people within the radius', () => {
    expect([...visibleNames(self, others, null)]).toEqual(['near', 'edge'])
  })
  it('also shows the hovered person, wherever they are', () => {
    expect(visibleNames(self, others, 'far').has('far')).toBe(true)
  })
  it('shows nobody (except hover) before the local avatar exists', () => {
    expect(visibleNames(null, others, null).size).toBe(0)
    expect([...visibleNames(null, others, 'far')]).toEqual(['far'])
  })
})

describe('badgeFor', () => {
  it('uses the server count and never invents one', () => {
    const counts = [{ building_key: 'sains', people: 11, live_rooms: 2 }, { building_key: 'perpus', people: 0, live_rooms: 0 }]
    expect(badgeFor(counts, 'sains')).toBe('11')
    expect(badgeFor(counts, 'perpus')).toBe('0')
    expect(badgeFor(counts, 'arena')).toBe('')
    expect(badgeFor([], 'sains')).toBe('')
  })
})
