import type { MapData } from '@/realtime/mapdata'
import { cssToken } from './tiles'

// One continuous illustration: no tile atlas edges can bleed at fractional zoom.
// Geometry remains sourced from Tiled; this texture never defines collision.
export function campusArt(map: MapData): HTMLCanvasElement {
  const canvas = document.createElement('canvas')
  const scale = 2
  canvas.width = map.pixelWidth * scale
  canvas.height = map.pixelHeight * scale
  const g = canvas.getContext('2d')!
  g.scale(scale, scale)
  const color = cssToken
  const rect = (x: number, y: number, w: number, h: number, r: number, fill: string) => {
    g.fillStyle = color(fill)
    g.beginPath()
    g.roundRect(x, y, w, h, r)
    g.fill()
  }
  rect(0, 0, map.pixelWidth, map.pixelHeight, 0, '--map-grass')
  g.fillStyle = color('--map-grass-dark')
  for (let y = 24; y < map.pixelHeight; y += 48) {
    for (let x = 24; x < map.pixelWidth; x += 48) {
      g.beginPath()
      g.arc(x, y, 3, 0, Math.PI * 2)
      g.fill()
    }
  }
  // Paint contiguous path runs into one canvas rather than an atlas.
  const ground = map.raw.layers.find((l) => l.name === 'ground')?.data ?? []
  const ts = map.tileSize
  for (let y = 0; y < map.height; y++) {
    for (let x = 0; x < map.width; x++) {
      const id = ground[y * map.width + x]
      if (id === 3 || id === 4 || id === 16) rect(x * ts, y * ts, ts, ts, 0, '--map-path')
    }
  }
  for (const b of map.buildings) {
    const { x, y, w, h } = b
    if (b.key === 'taman') {
      rect(x, y, w, h, 40, '--map-grass-dark')
      g.fillStyle = color('--map-water')
      g.beginPath()
      g.ellipse(x + w / 2, y + h / 2, w * 0.25, h * 0.2, 0, 0, Math.PI * 2)
      g.fill()
      for (const [dx, dy] of [
        [0.12, 0.15],
        [0.72, 0.8],
      ])
        rect(x + w * dx!, y + h * dy!, w * 0.18, 32, 10, '--map-wood')
      for (const [dx, dy] of [
        [0.82, 0.16],
        [0.18, 0.84],
      ]) {
        g.fillStyle = color('--color-speaking')
        g.beginPath()
        g.arc(x + w * dx!, y + h * dy!, 28, 0, Math.PI * 2)
        g.fill()
      }
      continue
    }
    if (b.key === 'lapangan') {
      rect(x, y, w, h, 32, '--map-grass-dark')
      g.strokeStyle = color('--color-surface')
      g.lineWidth = 6
      g.beginPath()
      g.roundRect(x + 10, y + 10, w - 20, h - 20, 24)
      g.stroke()
      g.beginPath()
      g.moveTo(x + 10, y + h / 2)
      g.lineTo(x + w - 10, y + h / 2)
      g.stroke()
      g.beginPath()
      g.arc(x + w / 2, y + h / 2, w * 0.18, 0, Math.PI * 2)
      g.stroke()
      continue
    }
    // Silhouette exactly follows building bounds; shadow is visual only.
    g.save()
    g.globalAlpha = 0.12
    rect(x, y + 10, w, h, 28, '--color-ink')
    g.restore()
    rect(x, y, w, h, 28, '--color-surface')
    g.save()
    g.beginPath()
    g.roundRect(x, y, w, h, 28)
    g.clip()
    const roof =
      (
        {
          classroom: '--roof-classroom',
          staff: '--roof-staff',
          library: '--roof-library',
          hall: '--roof-hall',
          canteen: '--roof-canteen',
        } as Record<string, string>
      )[b.roof] ?? '--roof-classroom'
    rect(x, y, w, h * 0.22, 0, roof)
    if (b.key === 'kantin') {
      for (let i = 1; i < 8; i += 2) rect(x + (w * i) / 8, y, w / 8, h * 0.22, 0, '--color-surface')
    }
    const cols = w > 350 ? 4 : 2
    const gap = 18,
      inset = 28,
      ww = (w - 2 * inset - gap * (cols - 1)) / cols
    for (let row = 0; row < (b.roof === 'classroom' ? 2 : 1); row++) {
      for (let col = 0; col < cols; col++)
        rect(x + inset + col * (ww + gap), y + h * 0.29 + row * 48, ww, 30, 10, '--map-water')
    }
    // Door aligns with the server-authored door point, not decorative centre.
    const dw = Math.min(76, w * 0.23)
    rect(b.doorX - dw / 2, y + h - 82, dw, 90, 16, '--map-wood')
    g.restore()
  }
  return canvas
}
