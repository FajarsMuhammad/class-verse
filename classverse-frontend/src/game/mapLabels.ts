// Pure rules for campus map labels, kept out of Phaser so they can be unit tested.
import type { BuildingCount } from '@/realtime/messages'

export interface NearbyAvatar { id: string; x: number; y: number }

// NAME_RADIUS: other players' names appear only this close to you (world px), or while hovered.
export const NAME_RADIUS = 120

// visibleNames picks whose name tag shows. Your own tag is always shown (the caller adds it); others only when near
// you or hovered, so a busy campus stays readable instead of a pile of overlapping names.
export function visibleNames(self: { x: number; y: number } | null, others: NearbyAvatar[], hoveredId: string | null): Set<string> {
  const out = new Set<string>()
  for (const o of others) {
    if (o.id === hoveredId || (self && Math.hypot(o.x - self.x, o.y - self.y) <= NAME_RADIUS)) out.add(o.id)
  }
  return out
}

// badgeFor is the people count shown on a building label, or '' when the server has no figure for it yet.
// It never invents a number: unknown buildings show no badge.
export function badgeFor(counts: BuildingCount[], key: string): string {
  const c = counts.find((b) => b.building_key === key)
  return c ? String(c.people) : ''
}
