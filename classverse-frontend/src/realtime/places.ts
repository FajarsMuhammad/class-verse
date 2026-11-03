import type { MapData } from './mapdata'
import type { Avatar, World } from './world'

// Mode HP has no Phaser, so nothing calls world.sample(); use the newest
// authoritative position of each avatar instead of the rendered one.
function latest(a: Avatar) {
  return a.samples[a.samples.length - 1] ?? a
}

export interface Place {
  key: string // '' = Area umum
  name: string
  people: Avatar[]
}

// places lists the general area and every zone with the other avatars in it.
// Snapshots never include self, so count() adds self from the server's
// `zone` message instead.
export function places(map: MapData, world: World): Place[] {
  const list: Place[] = [
    { key: '', name: 'Area umum', people: [] },
    ...map.zones.map((z) => ({ key: z.key, name: z.name, people: [] as Avatar[] })),
  ]
  for (const a of world.avatars.values()) {
    const p = latest(a)
    const z = map.zones.find((z) => p.x >= z.x && p.y >= z.y && p.x < z.x + z.w && p.y < z.y + z.h)
    list.find((pl) => pl.key === (z?.key ?? ''))!.people.push(a)
  }
  for (const pl of list) pl.people.sort((a, b) => a.name.localeCompare(b.name, 'id'))
  return list
}

// count is the number of people at a place, self included.
export function count(place: Place, selfZone: string): number {
  return place.people.length + (place.key === selfZone ? 1 : 0)
}

// nearby lists avatars within radius px of self (campus "Di dekatmu").
export function nearby(world: World, radius: number): Avatar[] {
  const self = world.self
  if (!self) return []
  return [...world.avatars.values()].filter((a) => {
    const p = latest(a)
    return !a.hidden && Math.hypot(p.x - self.x, p.y - self.y) <= radius
  })
}
