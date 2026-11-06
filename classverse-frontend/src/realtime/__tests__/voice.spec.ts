import { describe, expect, it } from 'vitest'

import { MapData, type TiledMap } from '../mapdata'
import { audible, channelAt } from '../voice'
import { World } from '../world'

const map = new MapData({
  width: 20,
  height: 10,
  tilewidth: 32,
  tileheight: 32,
  tilesets: [],
  layers: [
    { name: 'collision', type: 'tilelayer', data: Array(200).fill(0) },
    {
      name: 'zones',
      type: 'objectgroup',
      objects: [
        { id: 1, name: 'Meja 1', x: 0, y: 0, width: 64, height: 64, properties: [{ name: 'key', value: 'meja-1' }] },
        { id: 2, name: 'Meja 2', x: 500, y: 0, width: 64, height: 64, properties: [{ name: 'key', value: 'meja-2' }] },
      ],
    },
  ],
} as TiledMap)

const p = (id: string, x: number, y: number, role = 'participant') => ({
  id,
  name: id,
  avatar: 'default',
  role,
  x,
  y,
  dir: 'down' as const,
})

function world() {
  const w = new World()
  w.apply(
    {
      t: 'welcome',
      seq: 1,
      d: {
        self_id: 'me',
        room_id: 'r',
        map_key: 'classroom',
        participants: [
          p('me', 10, 10),
          p('mate', 60, 60), // same table, far corner
          p('other-table', 510, 10),
          p('area-far', 600, 300), // outside zones, far away
          p('guru', 300, 300, 'teacher'),
        ],
        all_chat_enabled: true,
      },
    },
    0,
  )
  return w
}

describe('voice channels', () => {
  it('maps points to zones or the general area', () => {
    expect(channelAt(map, 10, 10)).toBe('meja-1')
    expect(channelAt(map, 64, 10)).toBe('') // right edge is open
    expect(channelAt(map, 300, 300)).toBe('')
  })

  it('at a table hears only the same table, at any distance', () => {
    expect([...audible(map, world(), 'meja-1', [])]).toEqual(['mate'])
  })

  it('in the general area hears everyone outside tables, at any distance', () => {
    expect([...audible(map, world(), '', [])].sort()).toEqual(['area-far', 'guru'])
  })

  it('a broadcasting teacher is heard everywhere', () => {
    expect([...audible(map, world(), 'meja-2', ['guru'])].sort()).toEqual(['guru', 'other-table'])
  })

  it('follows the newest snapshot when someone sits down', () => {
    const w = world()
    w.apply({ t: 'snapshot', seq: 2, d: { p: [{ id: 'area-far', x: 20, y: 20, dir: 'down' }] } }, 5)
    expect([...audible(map, w, 'meja-1', [])].sort()).toEqual(['area-far', 'mate'])
  })
})
