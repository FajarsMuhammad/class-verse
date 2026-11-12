import { expect, it } from 'vitest'
import { MapData } from './mapdata'

it('uses exact pixel bounds, interaction radius and circular-zone metadata', () => {
  const map = new MapData({ width: 2, height: 2, tilewidth: 32, tileheight: 32,
    properties: [
      { name: 'world_width', value: 60 }, { name: 'world_height', value: 50 },
      { name: 'near_radius', value: 85 },
    ], tilesets: [], layers: [
      { name: 'collision', type: 'tilelayer', data: [0, 0, 0, 0] },
      { name: 'zones', type: 'objectgroup', objects: [{ id: 1, x: 0, y: 0, width: 40, height: 40,
        properties: [{ name: 'key', value: 'table-1' }, { name: 'shape', value: 'circle' }] }] },
      { name: 'buildings', type: 'objectgroup', objects: [{ id: 2, type: 'building', x: 0, y: 0, width: 20, height: 20,
        properties: [{ name: 'key', value: 'arena' }, { name: 'interact_radius', value: 180 }] }] },
    ] })
  expect([map.pixelWidth, map.pixelHeight, map.nearRadius]).toEqual([60, 50, 85])
  expect(map.isBlocked(60, 20)).toBe(true)
  expect(map.isBlocked(20, 50)).toBe(true)
  expect(map.zones[0]?.circle).toBe(true)
  expect(map.buildings[0]?.interactRadius).toBe(180)
  expect(map.quiet).toBe(false)
})

it('reads the quiet map property (library: chat only)', () => {
  const map = new MapData({ width: 1, height: 1, tilewidth: 32, tileheight: 32, tilesets: [],
    properties: [{ name: 'quiet', value: true }],
    layers: [{ name: 'collision', type: 'tilelayer', data: [0] }] })
  expect(map.quiet).toBe(true)
})
