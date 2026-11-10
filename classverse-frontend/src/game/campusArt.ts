import type { MapData } from '@/realtime/mapdata'
import { drawBuilding, shade } from './buildingArt'
import { cssToken } from './tiles'

// Ground tile ids from classverse-backend/maps/README.md.
const PATH = 3, PLAZA = 4, WATER = 13, WOOD = 14, LINE = 15, FLAG = 16, TREE = 17
const LAMP = 23, FLOWER = 24, SAND = 25

// One continuous illustration: no tile atlas edges can bleed at fractional zoom.
// Geometry remains sourced from Tiled; this texture never defines collision.
// `lit` lists building keys that currently have people inside (W10-03).
export function campusArt(map: MapData, lit = new Set<string>()): HTMLCanvasElement {
  const canvas = document.createElement('canvas')
  // Overview downsamples this world; 1× avoids an 8192×5120 GPU texture.
  const scale = 1
  canvas.width = map.pixelWidth * scale
  canvas.height = map.pixelHeight * scale
  const g = canvas.getContext('2d')!
  g.scale(scale, scale)
  const color = cssToken
  const ts = map.tileSize
  const ground = map.raw.layers.find((l) => l.name === 'ground')?.data ?? []
  const at = (x: number, y: number) =>
    x < 0 || y < 0 || x >= map.width || y >= map.height ? 0 : ground[y * map.width + x]!
  const px = (x: number, y: number, w: number, h: number, fill: string) => {
    g.fillStyle = fill
    g.fillRect(x, y, w, h)
  }

  // Grass with a quiet two-tone checker, like mown stripes.
  const grass = color('--map-grass')
  px(0, 0, map.pixelWidth, map.pixelHeight, grass)
  g.fillStyle = shade(grass, -0.035)
  for (let y = 0; y < map.height; y++)
    for (let x = 0; x < map.width; x++) if ((x + y) % 2 === 0) g.fillRect(x * ts, y * ts, ts, ts)

  // Paths and plaza with a darker kerb where they meet grass.
  for (let y = 0; y < map.height; y++) {
    for (let x = 0; x < map.width; x++) {
      const id = at(x, y)
      const paved = id === PATH || id === PLAZA || id === FLAG || id === LAMP
      if (!paved) continue
      const base = color('--map-path')
      px(x * ts, y * ts, ts, ts, id === PLAZA || id === FLAG ? shade(base, -0.06) : base)
      const open = (nx: number, ny: number) => {
        const n = at(nx, ny)
        return n === PATH || n === PLAZA || n === FLAG || n === LAMP
      }
      if (!open(x, y - 1)) px(x * ts, y * ts, ts, 2, shade(base, -0.18))
      if (!open(x, y + 1)) px(x * ts, y * ts + ts - 2, ts, 2, shade(base, -0.18))
      if (!open(x - 1, y)) px(x * ts, y * ts, 2, ts, shade(base, -0.18))
      if (!open(x + 1, y)) px(x * ts + ts - 2, y * ts, 2, ts, shade(base, -0.18))
    }
  }
  // Plaza paving pattern.
  for (let y = 0; y < map.height; y++)
    for (let x = 0; x < map.width; x++)
      if (at(x, y) === PLAZA && (x + y) % 2 === 0) px(x * ts + 4, y * ts + 4, ts - 8, ts - 8, shade(color('--map-path'), -0.1))

  for (const b of map.buildings) {
    const { x, y, w, h } = b
    if (b.key === 'taman' || b.key === 'kebun') {
      px(x, y, w, h, shade(color('--map-grass-dark'), 0.05))
      px(x, y, w, 4, shade(color('--map-grass-dark'), -0.2))
      px(x, y + h - 4, w, 4, shade(color('--map-grass-dark'), -0.2))
      px(x, y, 4, h, shade(color('--map-grass-dark'), -0.2))
      px(x + w - 4, y, 4, h, shade(color('--map-grass-dark'), -0.2))
      continue
    }
    if (b.key === 'lapangan') {
      px(x, y, w, h, shade(color('--map-grass-dark'), -0.05))
      continue
    }
    drawBuilding(g, b, color, lit.has(b.key))
  }

  // Props from the ground layer: water, benches, lines, trees, lamps, flowers, sand.
  for (let y = 0; y < map.height; y++) {
    for (let x = 0; x < map.width; x++) {
      const id = at(x, y)
      const X = x * ts, Y = y * ts
      if (id === WATER) {
        px(X, Y, ts, ts, color('--map-water'))
        if (at(x, y - 1) !== WATER) px(X, Y, ts, 3, shade(color('--map-water'), -0.2))
        if ((x + y) % 3 === 0) px(X + 8, Y + 14, 12, 2, shade(color('--map-water'), 0.5))
      } else if (id === SAND) px(X, Y, ts, ts, color('--map-sand'))
      else if (id === LINE) {
        px(X, Y, ts, ts, shade(color('--map-grass-dark'), -0.05))
        const v = at(x, y - 1) === LINE || at(x, y + 1) === LINE
        const hz = at(x - 1, y) === LINE || at(x + 1, y) === LINE
        if (hz || !v) px(X, Y + 13, ts, 6, color('--color-surface'))
        if (v || !hz) px(X + 13, Y, 6, ts, color('--color-surface'))
      } else if (id === WOOD) {
        px(X, Y + 8, ts, 14, color('--map-wood'))
        px(X, Y + 12, ts, 2, shade(color('--map-wood'), -0.25))
        px(X + 4, Y + 22, 4, 6, shade(color('--map-wood'), -0.35))
        px(X + ts - 8, Y + 22, 4, 6, shade(color('--map-wood'), -0.35))
      } else if (id === FLOWER) {
        px(X, Y + 6, ts, ts - 12, shade(color('--map-grass-dark'), -0.1))
        for (const [fx, fy] of [[6, 10], [18, 8], [12, 18], [24, 18]])
          px(X + fx!, Y + fy!, 5, 5, (x + fx!) % 2 ? color('--map-flower') : color('--map-lamp'))
      } else if (id === TREE) {
        g.globalAlpha = 0.18
        px(X + 4, Y + 20, ts, 10, color('--color-ink'))
        g.globalAlpha = 1
        px(X + 13, Y + 14, 6, 16, color('--map-trunk'))
        px(X + 2, Y - 8, ts - 4, 24, shade(color('--color-speaking'), -0.15))
        px(X + 6, Y - 12, ts - 12, 10, color('--color-speaking'))
        px(X + 10, Y - 6, 8, 6, shade(color('--color-speaking'), 0.25))
      } else if (id === LAMP) {
        px(X + 14, Y + 4, 4, 24, color('--color-ink'))
        px(X + 9, Y, 14, 8, color('--map-lamp'))
        px(X + 11, Y + 2, 10, 4, shade(color('--map-lamp'), 0.4))
      } else if (id === FLAG) {
        px(X + 14, Y - 40, 4, 68, color('--color-ink'))
        px(X + 18, Y - 38, 24, 8, color('--roof-hall'))
        px(X + 18, Y - 30, 24, 8, color('--color-surface'))
        px(X + 4, Y + 20, 24, 10, shade(color('--map-path'), -0.25))
      }
    }
  }
  return canvas
}
