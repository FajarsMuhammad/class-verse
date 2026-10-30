// Flat placeholder tileset drawn from design tokens (maps/README.md in the
// backend lists the tile ids). Final tile art can replace this texture
// without changing the maps.

export const TILE = 32

interface TileStyle {
  token: string
  shade?: number // darken (negative) or lighten (positive), -1..1
  draw?: 'window' | 'door' | 'tree' | 'flag' | 'line' | 'dots' | 'plank'
}

// Index = gid - 1.
const STYLES: TileStyle[] = [
  { token: '--map-grass', draw: 'dots' }, // 1 grass
  { token: '--map-grass-dark' }, // 2 grass dark
  { token: '--map-path' }, // 3 path
  { token: '--map-path', shade: -0.06 }, // 4 plaza
  { token: '--color-surface' }, // 5 wall
  { token: '--color-surface', draw: 'window' }, // 6 window
  { token: '--color-surface', draw: 'door' }, // 7 door
  { token: '--roof-classroom' }, // 8
  { token: '--roof-library' }, // 9
  { token: '--roof-hall' }, // 10
  { token: '--roof-canteen' }, // 11
  { token: '--roof-staff' }, // 12
  { token: '--map-water' }, // 13 water
  { token: '--map-grass-dark', draw: 'plank' }, // 14 bench
  { token: '--map-grass-dark', draw: 'line' }, // 15 field line
  { token: '--map-path', shade: -0.06, draw: 'flag' }, // 16 flag pole
  { token: '--map-grass', draw: 'tree' }, // 17 tree
  { token: '--map-grass' }, // 18 collision marker (never drawn)
]

export const TILE_COUNT = STYLES.length

export function cssToken(name: string, el: Element = document.documentElement): string {
  return getComputedStyle(el).getPropertyValue(name).trim() || '#000000'
}

function shade(hex: string, amount: number): string {
  const m = /^#?([0-9a-f]{6})$/i.exec(hex)
  if (!m) return hex
  const n = parseInt(m[1]!, 16)
  const ch = (v: number) =>
    Math.round(amount < 0 ? v * (1 + amount) : v + (255 - v) * amount)
      .toString(16)
      .padStart(2, '0')
  return `#${ch((n >> 16) & 255)}${ch((n >> 8) & 255)}${ch(n & 255)}`
}

// drawTileset paints all tiles side by side on a canvas.
export function drawTileset(canvas: HTMLCanvasElement, token = cssToken) {
  canvas.width = TILE * TILE_COUNT
  canvas.height = TILE
  const g = canvas.getContext('2d')
  if (!g) return
  STYLES.forEach((s, i) => {
    const x = i * TILE
    const base = s.shade ? shade(token(s.token), s.shade) : token(s.token)
    g.fillStyle = base
    g.fillRect(x, 0, TILE, TILE)
    switch (s.draw) {
      case 'dots':
        g.fillStyle = shade(base, -0.08)
        g.fillRect(x + 8, 8, 2, 2)
        g.fillRect(x + 22, 20, 2, 2)
        break
      case 'window':
        g.fillStyle = token('--map-water')
        roundRect(g, x + 3, 6, TILE - 6, TILE - 12, 6)
        break
      case 'door':
        g.fillStyle = token('--map-wood')
        g.fillRect(x, 0, TILE, TILE)
        break
      case 'tree':
        g.fillStyle = token('--color-speaking')
        g.beginPath()
        g.arc(x + TILE / 2, TILE / 2, TILE / 2 - 3, 0, Math.PI * 2)
        g.fill()
        break
      case 'flag':
        g.fillStyle = token('--color-ink')
        g.fillRect(x + 14, 2, 3, TILE - 4)
        g.fillStyle = token('--roof-hall')
        g.fillRect(x + 17, 2, 12, 9)
        break
      case 'line':
        g.fillStyle = token('--color-surface')
        g.fillRect(x, 13, TILE, 6)
        g.fillRect(x + 13, 0, 6, TILE)
        break
      case 'plank':
        g.fillStyle = token('--map-wood')
        roundRect(g, x + 2, 8, TILE - 4, TILE - 16, 4)
        break
    }
  })
}

function roundRect(
  g: CanvasRenderingContext2D,
  x: number,
  y: number,
  w: number,
  h: number,
  r: number,
) {
  g.beginPath()
  g.roundRect(x, y, w, h, r)
  g.fill()
}
