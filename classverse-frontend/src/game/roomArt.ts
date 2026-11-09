import type { MapData, TiledMap } from '@/realtime/mapdata'
import { cssToken } from './tiles'

// Continuous artwork follows Tiled ground/collision geometry; no atlas seams.
export function roomArt(map: MapData, key: string): HTMLCanvasElement {
  const canvas = document.createElement('canvas')
  canvas.width = map.pixelWidth * 2
  canvas.height = map.pixelHeight * 2
  const g = canvas.getContext('2d')!
  g.scale(2, 2)
  const garden = key === 'garden'
  const rect = (x: number, y: number, w: number, h: number, token: string, radius = 0) => {
    g.fillStyle = cssToken(token)
    g.beginPath()
    g.roundRect(x, y, w, h, radius)
    g.fill()
  }
  rect(0, 0, map.pixelWidth, map.pixelHeight, garden ? '--map-grass' : '--map-path')
  const ground = map.raw.layers.find((l) => l.name === 'ground')?.data ?? []
  const ts = map.tileSize
  const tokens: Record<number, string> = {
    3: '--map-path',
    7: '--map-wood',
    13: '--map-water',
    14: '--map-grass-dark',
    16: '--map-board',
    17: '--map-wood',
  }
  for (let y = 0; y < map.height; y++) {
    for (let x = 0; x < map.width; x++) {
      const id = ground[y * map.width + x]!
      if (tokens[id]) rect(x * ts, y * ts, ts, ts, !garden && id === 14 ? '--map-wood' : tokens[id])
    }
  }
  if (!garden) {
    g.strokeStyle = cssToken('--map-wood')
    g.globalAlpha = 0.25
    g.lineWidth = 1
    for (let x = 0; x <= map.pixelWidth; x += ts) {
      g.beginPath()
      g.moveTo(x, 0)
      g.lineTo(x, map.pixelHeight)
      g.stroke()
    }
    for (let y = 0; y <= map.pixelHeight; y += ts) {
      g.beginPath()
      g.moveTo(0, y)
      g.lineTo(map.pixelWidth, y)
      g.stroke()
    }
    g.globalAlpha = 1
  }
  const exit = exitPoint(map.raw)
  if (exit) rect(exit.x - 24, exit.y - 12, 48, 24, '--color-primary', 8)
  return canvas
}

const BOARD_TILE = 16

function exitPoint(raw: TiledMap) {
  return raw.layers.find((l) => l.name === 'exit')?.objects?.[0]
}

// roomLandmarks names the board and the exit, so the hint "jalan ke pintu
// keluar" points at something visible. Positions in map pixels (label top).
export function roomLandmarks(map: MapData): { text: string; x: number; y: number; dark: boolean }[] {
  const out: { text: string; x: number; y: number; dark: boolean }[] = []
  const ground = map.raw.layers.find((l) => l.name === 'ground')?.data ?? []
  const board = ground.flatMap((id, i) => (id === BOARD_TILE ? [i] : []))
  if (board.length) {
    const xs = board.map((i) => i % map.width)
    const y = Math.floor(board[0]! / map.width)
    const x = ((Math.min(...xs) + Math.max(...xs) + 1) / 2) * map.tileSize
    out.push({ text: 'Papan tulis', x, y: y * map.tileSize + 8, dark: true })
  }
  const exit = exitPoint(map.raw)
  if (exit) out.push({ text: 'Pintu keluar', x: exit.x, y: exit.y - 44, dark: false })
  return out
}
