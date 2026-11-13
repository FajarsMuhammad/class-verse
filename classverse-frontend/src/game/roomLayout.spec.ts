import { expect, it } from 'vitest'
import { roomLayout, shelfAt, stationAt } from './roomLayout'

it('uses the default floor for regular rooms and the larger library floor', () => {
  const kelas = roomLayout('classroom')!
  expect([kelas.width, kelas.height, kelas.floor.w, kelas.floor.h]).toEqual([1044, 684, 1000, 640])
  expect(kelas.shelves).toEqual([])
  expect(roomLayout('campus')).toBeNull()

  const lib = roomLayout('library')!
  expect([lib.width, lib.height]).toEqual([1244, 764])
  // Door gap centred on the bottom wall, matching the server exit at width/2.
  expect(lib.door.x + lib.door.w / 2).toBe(lib.width / 2)
  expect(lib.shelves.map((s) => s.label)).toContain('Matematika')
  // Category shelves and areas are shifted inside the 22 px wall.
  expect(lib.shelves[0]).toMatchObject({ x: 122, y: 106, w: 36, h: 230 })
  expect(lib.areas.map((a) => a.label)).toEqual(['Pojok Anak · SD', 'Bilik hening'])
  expect(lib.quiet).toBe(true)
  expect(kelas.quiet).toBe(false)
})

it('maps science and staff stations to shifted coordinates without quiet mode', () => {
  const science = roomLayout('science')!
  expect([science.width, science.height]).toEqual([1244, 764])
  expect(science.quiet).toBe(false)
  expect(stationAt('science', 100, 500)?.id).toBe('tatasurya')
  expect(stationAt('science', 100, 440)?.id).toBe('tatasurya')
  expect(stationAt('science', 600, 720)).toBeUndefined()
  expect(roomLayout('staff')!.stations.map((s) => s.id)).toEqual(['kerja', 'rapat', 'studio', 'nilai'])
  expect(stationAt('staff', 100, 100)?.id).toBe('kerja')
  expect(stationAt('library', 100, 100)).toBeUndefined()
})

it('shelfAt hits a library shelf and its label, nothing in other rooms', () => {
  const sains = roomLayout('library')!.shelves.find((s) => s.cat === 'sains')!
  expect(shelfAt('library', sains.x + 5, sains.y + 100)).toBe('sains')
  expect(shelfAt('library', sains.x + 5, sains.y - 20)).toBe('sains')
  expect(shelfAt('library', sains.x + 5, sains.y + sains.h + 5)).toBeUndefined()
  expect(shelfAt('classroom', sains.x + 5, sains.y + 100)).toBeUndefined()
})
