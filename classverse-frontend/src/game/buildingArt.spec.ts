import { describe, expect, it, vi } from 'vitest'
import { drawBuilding, shade } from './buildingArt'
import type { Building } from '@/realtime/mapdata'

describe('building art', () => {
  it('mixes palette shades without losing channel padding', () => {
    expect(shade('#204060', 0)).toBe('#204060')
    expect(shade('#204060', -1)).toBe('#000000')
    expect(shade('#204060', 1)).toBe('#ffffff')
  })

  it('draws two window floors and a sloped roof inside classroom bounds', () => {
    const fills: { x: number; y: number; w: number; h: number; color: string }[] = []
    const context = {
      fillStyle: '', globalAlpha: 1,
      fillRect(x: number, y: number, w: number, h: number) {
        fills.push({ x, y, w, h, color: this.fillStyle })
      },
      save: vi.fn<() => void>(), restore: vi.fn<() => void>(),
      beginPath: vi.fn<() => void>(), moveTo: vi.fn<(x: number, y: number) => void>(),
      lineTo: vi.fn<(x: number, y: number) => void>(), closePath: vi.fn<() => void>(),
      fill: vi.fn<() => void>(), clip: vi.fn<() => void>(),
    }
    const building = { x: 192, y: 128, w: 832, h: 512, doorX: 608, roof: 'classroom' } as Building
    drawBuilding(context as unknown as CanvasRenderingContext2D, building,
      (token) => token === '--map-lamp' ? '#ffcf70' : '#808080', true)
    const windows = fills.filter((r) => r.color === '#ffcf70')
    expect(new Set(windows.map((r) => r.y)).size).toBe(2)
    expect(windows.every((r) => r.x >= building.x && r.x + r.w <= building.x + building.w)).toBe(true)
    expect(context.clip).toHaveBeenCalledOnce()
    expect(context.lineTo).toHaveBeenCalled()
  })
})
