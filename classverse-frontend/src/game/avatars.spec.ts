import { describe, expect, it } from 'vitest'
import { avatarChoices, avatarKey, avatarURL, avatarSheetURL, avatarFrame } from './avatars'

describe('sporty avatars', () => {
  it('provides 42 unique style/color choices with matching portraits and sheets', () => {
    expect(avatarChoices).toHaveLength(42)
    expect(new Set(avatarChoices.map((a) => a.key)).size).toBe(42)
    for (const a of avatarChoices) {
      expect(avatarURL(a.key)).toContain('-idle.png')
      expect(avatarSheetURL(a.key)).toContain('.png')
      expect(avatarKey(a.key)).toBe(a.key)
    }
  })
  it('preserves legacy keys and rejects unknown asset paths', () => {
    for (const style of ['round', 'cap', 'glasses', 'headset', 'bun', 'beret']) {
      for (const color of ['nila', 'mangga']) {
        const key = `${style}-${color}`
        expect(avatarKey(key)).toBe(key)
        expect(avatarURL(key)).toMatch(/^data:image\/svg\+xml/)
        expect(avatarSheetURL(key)).toBeNull()
      }
    }
    expect(avatarKey('../../bad.png')).toBe('default')
    expect(avatarURL('unknown')).toBe(avatarURL('default'))
  })
  it('selects direction rows and idle/walk frames, respecting reduced motion', () => {
    expect(avatarFrame('down', false, 0)).toBe(0)
    expect(avatarFrame('left', false, 0)).toBe(5)
    expect(avatarFrame('right', false, 0)).toBe(10)
    expect(avatarFrame('up', false, 0)).toBe(15)
    expect([0, 125, 250, 375, 500].map((time) => avatarFrame('down', true, time))).toEqual([1, 2, 3, 4, 1])
    expect(avatarFrame('up', true, 250, true)).toBe(15)
  })
})
