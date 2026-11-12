import { describe, expect, it } from 'vitest'
import rooms from '@/assets/world/rooms.json'
import { shelves } from './library'

describe('library shelves', () => {
  it('match the map shelves so a shelf click selects a catalog tab', () => {
    const map = rooms.rooms.perpustakaan.shelves.map((s) => ({ key: s.cat, label: s.label }))
    expect(shelves).toEqual(map)
  })
})
