import { describe, expect, it } from 'vitest'

import { MapData, type TiledMap } from '../mapdata'
import { count, nearby, places } from '../places'
import { World } from '../world'

const map = new MapData({
  width: 10,
  height: 10,
  tilewidth: 32,
  tileheight: 32,
  tilesets: [],
  layers: [
    { name: 'collision', type: 'tilelayer', data: Array(100).fill(0) },
    {
      name: 'zones',
      type: 'objectgroup',
      objects: [
        { id: 1, name: 'Meja 1', x: 0, y: 0, width: 64, height: 64, properties: [{ name: 'key', value: 'meja-1' }] },
      ],
    },
  ],
} as TiledMap)

const p = (id: string, x: number, y: number) => ({
  id,
  name: id,
  avatar: 'default',
  role: 'participant',
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
        participants: [p('me', 200, 200), p('Sari', 16, 16), p('Dimas', 300, 300)],
        all_chat_enabled: true,
      },
    },
    0,
  )
  return w
}

describe('places', () => {
  it('groups others by zone and counts self from the server zone', () => {
    const w = world()
    const list = places(map, w)
    expect(list.map((pl) => [pl.key, pl.people.map((a) => a.id)])).toEqual([
      ['', ['Dimas']],
      ['meja-1', ['Sari']],
    ])
    expect(count(list[1]!, 'meja-1')).toBe(2)
    expect(count(list[0]!, 'meja-1')).toBe(1)
  })

  it('uses the newest snapshot, not the stale rendered position', () => {
    const w = world()
    w.apply({ t: 'snapshot', seq: 2, d: { p: [{ id: 'Dimas', x: 40, y: 40, dir: 'down' }] } }, 10)
    expect(places(map, w)[1]!.people.map((a) => a.id)).toEqual(['Dimas', 'Sari'])
  })

  it('nearby keeps avatars within the radius of self', () => {
    const w = world()
    expect(nearby(w, 160).map((a) => a.id)).toEqual(['Dimas'])
    expect(nearby(w, 100)).toEqual([])
  })
})
