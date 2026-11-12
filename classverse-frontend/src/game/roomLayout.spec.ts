import { expect, it } from 'vitest'
import { roomLayout } from './roomLayout'

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
