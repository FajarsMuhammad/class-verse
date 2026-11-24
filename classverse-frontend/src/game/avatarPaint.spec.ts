import { describe, expect, it } from 'vitest'
import { STEP_PX, WALK_FRAMES, frameIndex, walkFrame } from './avatarPaint'

describe('walkFrame', () => {
  it('stands when not moving, whatever the distance', () => {
    expect(walkFrame(false, 0)).toBe(0)
    expect(walkFrame(false, 500)).toBe(0)
  })
  it('alternates the two stride poses as distance grows', () => {
    expect(walkFrame(true, 0)).toBe(1)
    expect(walkFrame(true, STEP_PX)).toBe(2)
    expect(walkFrame(true, STEP_PX * 2)).toBe(1)
    expect(walkFrame(true, STEP_PX * 2 - 1)).toBe(2) // still in the second step
    expect(walkFrame(true, STEP_PX * 3)).toBe(2)
  })
})

describe('frameIndex', () => {
  it('lays frames out row per direction, column per pose', () => {
    expect(frameIndex('down', 0)).toBe(0)
    expect(frameIndex('left', 0)).toBe(WALK_FRAMES)
    expect(frameIndex('up', 2)).toBe(3 * WALK_FRAMES + 2)
  })
})
