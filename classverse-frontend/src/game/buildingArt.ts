import type { Building } from '@/realtime/mapdata'

// Pixel-art style building facades (W10-03). Geometry still comes from Tiled
// bounds; this is only how a building looks. `color` resolves design tokens.
type Color = (token: string) => string

const ROOF_TOKEN: Record<string, string> = {
  classroom: '--roof-classroom',
  staff: '--roof-staff',
  library: '--roof-library',
  hall: '--roof-hall',
  canteen: '--roof-canteen',
  lab: '--roof-lab',
  studio: '--roof-studio',
  mosque: '--roof-mosque',
  clinic: '--roof-clinic',
}

// shade mixes a hex colour toward black (<0) or white (>0).
export function shade(hex: string, amount: number): string {
  const n = parseInt(hex.slice(1), 16)
  const mix = (c: number) =>
    Math.round(amount < 0 ? c * (1 + amount) : c + (255 - c) * amount)
  const r = mix((n >> 16) & 255), g = mix((n >> 8) & 255), b = mix(n & 255)
  return `#${((r << 16) | (g << 8) | b).toString(16).padStart(6, '0')}`
}

export function drawBuilding(g: CanvasRenderingContext2D, b: Building, color: Color, lit = false) {
  const { x, y, w, h } = b
  const roof = color(ROOF_TOKEN[b.roof] ?? '--roof-classroom')
  const wall = color('--color-surface')
  const ink = color('--color-ink')
  const px = (px: number, py: number, pw: number, ph: number, fill: string) => {
    g.fillStyle = fill
    g.fillRect(Math.round(px), Math.round(py), Math.round(pw), Math.round(ph))
  }
  const roofH = Math.round(h * 0.3)
  const floors = h >= 480 ? 2 : 1
  // Facade details scale with the building so they read at overview zoom.
  const u = Math.max(1, Math.min(w, h) / 320)

  // Ground shadow, then the body, then a darker plinth so it sits on the ground.
  g.globalAlpha = 0.16
  px(x + 6, y + 10, w, h, ink)
  g.globalAlpha = 1
  px(x, y + roofH, w, h - roofH, wall)
  px(x, y + h - 12, w, 12, shade(wall, -0.18))
  // Side wall shading gives a 3/4 volume read on a flat top-down map.
  px(x + w - 14, y + roofH, 14, h - roofH, shade(wall, -0.1))

  // A hipped roof has two visible slopes; clipped tile courses preserve its silhouette.
  const hip = Math.min(w * 0.16, roofH * 0.65)
  g.save()
  g.beginPath()
  g.moveTo(x + hip, y)
  g.lineTo(x + w - hip, y)
  g.lineTo(x + w, y + roofH - 12)
  g.lineTo(x, y + roofH - 12)
  g.closePath()
  g.fillStyle = roof
  g.fill()
  g.clip()
  for (let row = 0; row < roofH; row += 16 * u) {
    px(x, y + row, w, 3 * u, shade(roof, -0.18))
    for (let col = (row / (16 * u)) % 2 ? 16 * u : 0; col < w; col += 32 * u)
      px(x + col, y + row, 2 * u, 16 * u, shade(roof, -0.12))
  }
  g.beginPath()
  g.moveTo(x + w - hip, y)
  g.lineTo(x + w, y + roofH - 12)
  g.lineTo(x + w - hip * 1.6, y + roofH - 12)
  g.closePath()
  g.fillStyle = shade(roof, -0.28)
  g.fill()
  g.restore()
  px(x + hip, y, w - hip * 2, 6 * u, shade(roof, 0.25))
  px(x, y + roofH - 12, w, 12, shade(roof, -0.35))
  // Deep eave shadow and floor band make the facade read as a building, not a card.
  px(x, y + roofH, w, 10 * u, shade(wall, -0.22))
  px(x, y + h - 116 * u, w, 8 * u, shade(wall, -0.18))
  for (const cx of [x + 12 * u, x + w - 24 * u]) {
    px(cx, y + roofH + 10 * u, 12 * u, h - roofH - 24 * u, shade(wall, -0.12))
    px(cx, y + h - 20 * u, 16 * u, 8 * u, shade(wall, -0.25))
  }
  if (b.roof === 'mosque') {
    g.fillStyle = shade(roof, 0.1)
    g.beginPath()
    g.arc(x + w / 2, y + roofH - 10, w * 0.28, Math.PI, 0)
    g.fill()
    px(x + w / 2 - 3, y - 18, 6, 20, shade(roof, -0.25))
  }
  if (b.roof === 'hall') px(x + w * 0.1, y - 10, w * 0.8, 14, shade(roof, -0.1))
  if (b.roof === 'library') px(x + 12, y - 8, 28, roofH, shade(roof, -0.2))
  if (b.roof === 'lab') {
    px(x + w - 48, y - 14, 22, roofH, shade(roof, -0.2))
    px(x + w - 44, y - 20, 14, 8, color('--map-water'))
  }
  if (b.roof === 'canteen') {
    for (let i = 0; i < Math.floor(w / 24); i++)
      px(x + i * 24, y + roofH - 10, 12, 10, i % 2 ? color('--color-surface') : shade(roof, -0.3))
  }
  if (b.roof === 'studio') px(x + 16, y + 10, w - 32, roofH - 28, color('--map-water'))
  if (b.roof === 'clinic') {
    px(x + w / 2 - 4, y + 6, 8, roofH - 22, wall)
    px(x + w / 2 - (roofH - 22) / 2, y + 6 + (roofH - 22) / 2 - 4, roofH - 22, 8, wall)
  }

  // Windows per floor; lit windows tell the map there are people inside.
  const glass = lit ? color('--map-lamp') : color('--map-water')
  const ww = 56 * u, wh = 36 * u, margin = 32 * u
  const cols = Math.max(2, Math.floor((w - margin * 2) / (ww + 28 * u)))
  const gap = (w - margin * 2 - cols * ww) / Math.max(1, cols - 1)
  const doorTop = y + h - 110 * u
  for (let f = 0; f < floors; f++) {
    const wy = y + roofH + 18 * u + f * (wh + 14 * u)
    if (wy + wh > doorTop - 8 * u) break
    for (let c = 0; c < cols; c++) {
      const wx = x + margin + c * (ww + gap)
      px(wx, wy, ww, wh, shade(wall, -0.35))
      px(wx + 4 * u, wy + 4 * u, ww - 8 * u, wh - 8 * u, glass)
      px(wx + ww / 2 - u, wy + 4 * u, 2 * u, wh - 8 * u, shade(wall, -0.35))
      px(wx, wy + wh, ww, 6 * u, shade(wall, -0.25))
    }
  }

  // Door aligned with the Tiled door point, with an awning and steps.
  const dw = 72 * u
  const dh = 96 * u
  const dx = b.doorX - dw / 2
  px(dx - 12 * u, doorTop - 14 * u, dw + 24 * u, 14 * u, shade(roof, -0.2))
  px(dx, doorTop, dw, dh, color('--map-wood'))
  px(dx + dw / 2 - u, doorTop, 2 * u, dh, shade(color('--map-wood'), -0.35))
  px(dx + 6 * u, doorTop + 10 * u, dw / 2 - 10 * u, dh * 0.35, shade(color('--map-water'), 0.1))
  px(dx + dw / 2 + 4 * u, doorTop + 10 * u, dw / 2 - 10 * u, dh * 0.35, shade(color('--map-water'), 0.1))
  px(dx + dw / 2 - 12 * u, doorTop + dh * 0.55, 5 * u, 5 * u, ink)
  px(dx + dw / 2 + 7 * u, doorTop + dh * 0.55, 5 * u, 5 * u, ink)
  px(dx - 16 * u, y + h - 14 * u, dw + 32 * u, 14 * u, shade(wall, -0.3))
  if (b.staffOnly) px(dx - 30 * u, doorTop + dh * 0.5, 16 * u, 20 * u, shade(roof, -0.1))
}
