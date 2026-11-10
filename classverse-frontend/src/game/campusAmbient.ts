import type Phaser from 'phaser'
import type { MapData } from '@/realtime/mapdata'
import { cssToken } from './tiles'

// Sparse overlays animate decoration without rebuilding the world texture.
export function drawCampusAmbient(g: Phaser.GameObjects.Graphics, map: MapData, frame: number) {
  g.clear()
  const ground = map.raw.layers.find((layer) => layer.name === 'ground')?.data ?? []
  const color = (token: string) => Number.parseInt(cssToken(token).slice(1), 16)
  const water = color('--color-surface')
  const leaves = color('--color-speaking')
  const lamp = color('--map-lamp')
  const flag = color('--roof-hall')
  ground.forEach((id, index) => {
    const x = (index % map.width) * map.tileSize
    const y = Math.floor(index / map.width) * map.tileSize
    const phase = (frame + index) % 4
    if (id === 13 && index % 3 === 0) {
      g.fillStyle(water, 0.65).fillRect(x + 5 + phase * 3, y + 10, 10, 2)
    } else if (id === 17) {
      g.fillStyle(leaves, 0.75).fillRect(x + 4 + phase, y - 10, 12, 4)
    } else if (id === 23) {
      g.fillStyle(lamp, 0.08 + phase * 0.025).fillCircle(x + 16, y + 4, 22)
    } else if (id === 16) {
      g.fillStyle(flag).fillRect(x + 18, y - 38, 20 + phase * 2, 8)
    }
  })
}
