import { describe, expect, it } from 'vitest'
import { avatarChoices, avatarKey, avatarURL } from './avatars'
import palette from '@/assets/avatars/palette.json'

const svg = (key: string) => atob(avatarURL(key).split(',')[1]!)

describe('idle avatars', () => {
  it('renders independently selected palette values and rejects out-of-range keys', () => {
    const rendered = svg('look-5-4-9')
    expect(avatarKey('look-5-4-9')).toBe('look-5-4-9')
    expect(rendered).toContain(`fill="${palette.skin[5]}"`)
    expect(rendered).toContain(`fill="${palette.hair[4]}"`)
    expect(rendered).toContain(`fill="${palette.shirt[9]}"`)
    for (const key of ['look-6-0-0', 'look-0-5-0', 'look-0-0-10', 'look-01-0-0'])
      expect(avatarKey(key)).toBe('default')
  })
  it('renders all existing profile choices with the supplied idle silhouette', () => {
    expect(avatarChoices).toHaveLength(42)
    for (const { key } of avatarChoices) {
      expect(avatarKey(key)).toBe(key)
      expect(avatarURL(key)).toMatch(/^data:image\/svg\+xml;base64,/)
      expect(svg(key)).toContain('viewBox="0 0 28 38"')
      expect(svg(key)).toContain('clipPath')
      expect(palette.skin.some((color) => svg(key).includes(color))).toBe(true)
      expect(palette.shirt.some((color) => svg(key).includes(color))).toBe(true)
    }
  })
  it('keeps skin and hair unchanged when clothing changes', () => {
    const colors = (key: string) => [...svg(key).matchAll(/fill="([^"]+)"/g)].map((m) => m[1])
    const nila = colors('sporty-headphones-nila')
    const laut = colors('sporty-headphones-laut')
    expect(nila[0]).not.toBe(laut[0])
    expect(nila.slice(1)).toEqual(laut.slice(1))
  })
  it('accepts legacy keys and never interpolates unknown input', () => {
    expect(avatarKey('cap-mangga')).toBe('cap-mangga')
    expect(svg('cap-mangga')).toContain('<svg')
    expect(avatarKey('../../bad.png')).toBe('default')
    expect(avatarURL('"><script>alert(1)</script>')).toBe(avatarURL('default'))
  })
})
