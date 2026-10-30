import { describe, expect, it } from 'vitest'
import { avatarChoices, avatarKey, avatarURL } from './avatars'

describe('avatar catalogue', () => {
  it('provides 12 unique safe keys, renderable SVGs and both colors', () => {
    document.documentElement.style.setProperty('--avatar-nila', '#7c83fd')
    document.documentElement.style.setProperty('--avatar-mangga', '#ffb547')
    expect(avatarChoices).toHaveLength(12)
    expect(new Set(avatarChoices.map((a) => a.key)).size).toBe(12)
    for (const a of avatarChoices) {
      const svg = atob(avatarURL(a.key).split(',')[1]!)
      expect(svg).toContain('<svg')
      expect(svg).toContain(a.key.endsWith('nila') ? '#7c83fd' : '#ffb547')
      expect(svg).not.toContain('<script')
    }
  })
  it('keeps legacy and unknown values safe', () => {
    expect(avatarKey('../../bad.svg')).toBe('default')
    expect(avatarURL('unknown')).toBe(avatarURL('default'))
  })
})
