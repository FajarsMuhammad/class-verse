import type Phaser from 'phaser'
import { expect, it, vi } from 'vitest'
import { freezeGround } from './staticGround'

it('bakes static graphics once and removes per-frame geometry', () => {
  const image = { setOrigin: vi.fn<(origin: number) => void>().mockReturnThis(), setDepth: vi.fn<(depth: number) => void>().mockReturnThis() }
  const scene = { add: { image: vi.fn<(...args: unknown[]) => typeof image>(() => image) } }
  const graphics = { generateTexture: vi.fn<(...args: unknown[]) => void>().mockReturnThis(), destroy: vi.fn<() => void>() }
  freezeGround(scene as unknown as Phaser.Scene, graphics as unknown as Phaser.GameObjects.Graphics, 2200, 1500)
  expect(graphics.generateTexture).toHaveBeenCalledExactlyOnceWith('world-ground', 2200, 1500)
  expect(scene.add.image).toHaveBeenCalledExactlyOnceWith(0, 0, 'world-ground')
  expect(image.setOrigin).toHaveBeenCalledWith(0)
  expect(image.setDepth).toHaveBeenCalledWith(-10)
  expect(graphics.destroy).toHaveBeenCalledOnce()
})
