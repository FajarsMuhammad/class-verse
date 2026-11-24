import world from '@/assets/world/world.json'

// Campus zoom is a factor of the "fit" zoom, where every building and its label fills the map frame, as in the
// map-revamp prototype's overview. 1 = fit (shown as 100%); the user zooms in from there to walk around.
export const ZOOM_MIN = 0.7
export const ZOOM_MAX = 2
export const ZOOM_DEFAULT = 1
// Absolute camera limits. Scenery is baked at ART_SCALE 1 (GPU memory, see campusPaint.ts), so past ~1.2 it blurs.
const CAMERA_MIN = 0.3
const CAMERA_MAX = 1.2

export const clampZoom = (z: number) => Math.max(ZOOM_MIN, Math.min(ZOOM_MAX, Math.round(z * 100) / 100))

// CONTENT is the box around every building, open place and label, with a margin for the trees at the edge.
const MARGIN = 40
const boxes = [
  ...world.places.map((p) => ({ x: p.sprite_pos.x, y: Math.min(p.sprite_pos.y, p.label_anchor.y - 20), r: p.sprite_pos.x + p.size.w, b: p.sprite_pos.y + p.size.h })),
  ...world.zones.map((z) => ({ x: z.rect.x, y: z.rect.y - 20, r: z.rect.x + z.rect.w, b: z.rect.y + z.rect.h })),
]
const left = Math.min(...boxes.map((b) => b.x)) - MARGIN, top = Math.min(...boxes.map((b) => b.y)) - MARGIN
export const CONTENT = {
  x: left, y: top,
  w: Math.max(...boxes.map((b) => b.r)) + MARGIN - left,
  h: Math.max(...boxes.map((b) => b.b)) + MARGIN - top,
}

// cameraZoom turns the user's factor into a camera zoom for a view of viewW x viewH screen pixels.
export function cameraZoom(factor: number, viewW: number, viewH: number) {
  const fit = Math.min(viewW / CONTENT.w, viewH / CONTENT.h)
  return Math.max(CAMERA_MIN, Math.min(CAMERA_MAX, fit * clampZoom(factor)))
}
