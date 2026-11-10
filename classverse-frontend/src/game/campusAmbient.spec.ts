import { expect, it, vi } from 'vitest'
import type Phaser from 'phaser'
import type { MapData } from '@/realtime/mapdata'
import { drawCampusAmbient } from './campusAmbient'

it('changes sparse decoration overlays without touching collision data', () => {
  const ground = [13, 17, 23, 16]
  const map = { width: 4, tileSize: 32, raw: { layers: [{ name: 'ground', data: ground }] } } as unknown as MapData
  const g = {
    clear: vi.fn<() => void>(),
    fillStyle: vi.fn<() => unknown>(),
    fillRect: vi.fn<() => unknown>(),
    fillCircle: vi.fn<() => unknown>(),
  }
  g.fillStyle.mockReturnValue(g)
  drawCampusAmbient(g as unknown as Phaser.GameObjects.Graphics, map, 0)
  const first = JSON.stringify(g.fillRect.mock.calls)
  g.fillRect.mockClear()
  drawCampusAmbient(g as unknown as Phaser.GameObjects.Graphics, map, 1)
  expect(JSON.stringify(g.fillRect.mock.calls)).not.toBe(first)
  expect(ground).toEqual([13, 17, 23, 16])
  expect(g.clear).toHaveBeenCalledTimes(2)
})
