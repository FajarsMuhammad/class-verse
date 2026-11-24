import { expect, it } from 'vitest'
import { ART_SCALE, GROUND_SCALE } from './campusPaint'
import world from '@/assets/world/world.json'

// Scenery textures stay resident on the GPU for the whole campus visit. 2x art plus a 1.5x floor cost ~90 MB
// and lagged real devices; zoom stays <= 1.2 and the canvas is CSS-sized, so 1 texel per world pixel is enough.
it('keeps campus scenery textures at one texel per world pixel', () => {
  expect(ART_SCALE).toBe(1)
  expect(GROUND_SCALE).toBe(1)
  const groundMB = (world.size.w * GROUND_SCALE * world.size.h * GROUND_SCALE * 4) / 1e6
  expect(groundMB).toBeLessThan(15)
})
