import type { MapData } from '@/realtime/mapdata'
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
  const exit = map.raw.layers.find((l) => l.name === 'exit')?.objects?.[0]
  if (exit) rect(exit.x - 24, exit.y - 12, 48, 24, '--color-primary', 8)
  return canvas
}
