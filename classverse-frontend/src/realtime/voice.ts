import type { MapData } from './mapdata'
import type { World } from './world'

// channelAt is the voice channel of a point: its zone key, or '' for the
// general area outside every zone (docs/design.md, slice 9, V9-03).
export function channelAt(map: MapData, x: number, y: number): string {
  return map.zones.find((z) => x >= z.x && y >= z.y && x < z.x + z.w && y < z.y + z.h)?.key ?? ''
}

// audible lists the remote avatars the local user should hear: everyone in
// the same channel, plus staff broadcasting to the whole room. Positions are
// the newest server samples (mode HP never runs world.sample()).
export function audible(map: MapData, world: World, selfZone: string, broadcasters: string[]): Set<string> {
  const out = new Set<string>()
  for (const a of world.avatars.values()) {
    const p = a.samples[a.samples.length - 1] ?? a
    if (broadcasters.includes(a.id) || channelAt(map, p.x, p.y) === selfZone) out.add(a.id)
  }
  return out
}
