import { describe, expect, it } from 'vitest'
import { stackLabels } from './layout'

describe('stackLabels', () => {
  it('keeps separate tags in place', () => {
    expect(
      stackLabels([
        { x: 0, y: 0, w: 40, h: 10 },
        { x: 100, y: 0, w: 40, h: 10 },
      ]),
    ).toEqual([0, 0])
  })

  it('stacks overlapping tags below each other', () => {
    expect(
      stackLabels([
        { x: 10, y: 2, w: 40, h: 10 },
        { x: 0, y: 0, w: 40, h: 10 },
        { x: 5, y: 1, w: 40, h: 10 },
      ]),
    ).toEqual([20, 0, 10])
  })
})
