import { expect, it } from 'vitest'
import { MapData } from './mapdata'

it('reads seat feet positions from the served map and ignores blocked or invalid points', () => {
  const map = new MapData({ width: 3, height: 2, tilewidth: 32, tileheight: 32, tilesets: [], layers: [
    { name: 'collision', type: 'tilelayer', data: [0, 1, 0, 0, 0, 0] },
    { name: 'seats', type: 'objectgroup', objects: [
      { id: 1, name: '0', x: 16, y: 48, width: 0, height: 0 },
      { id: 2, name: '1', x: 48, y: 16, width: 0, height: 0 },
      { id: 3, name: '2', x: -1, y: 16, width: 0, height: 0 },
      { id: 4, name: '3', x: Number.NaN, y: 16, width: 0, height: 0 },
    ] },
  ] })
  expect(map.seats).toEqual([{ key: '0', x: 16, y: 48 }])
})
