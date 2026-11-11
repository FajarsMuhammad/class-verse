import palette from '@/assets/avatars/palette.json'

// Keep persisted profile keys valid while replacing their presentation with MVP 2 art.
export const avatarStyles = [
  { key: 'runner', label: 'Profil 1' }, { key: 'cap', label: 'Profil 2' },
  { key: 'headphones', label: 'Profil 3' }, { key: 'varsity', label: 'Profil 4' },
  { key: 'ponytail', label: 'Profil 5' }, { key: 'hoodie', label: 'Profil 6' },
  { key: 'glasses', label: 'Profil 7' },
]
export const avatarColors = [
  { key: 'nila', label: 'Nila', color: palette.shirt[0]! },
  { key: 'mangga', label: 'Mangga', color: palette.shirt[4]! },
  { key: 'zamrud', label: 'Zamrud', color: palette.shirt[2]! },
  { key: 'laut', label: 'Biru laut', color: palette.shirt[3]! },
  { key: 'koral', label: 'Koral', color: palette.shirt[1]! },
  { key: 'grafit', label: 'Grafit', color: palette.shirt[8]! },
]
export const avatarChoices = avatarStyles.flatMap((style) => avatarColors.map((color) => ({
  key: `sporty-${style.key}-${color.key}`, label: `${style.label} ${color.label}`,
})))
export const avatarPalette = palette
const legacyStyles = ['round', 'cap', 'glasses', 'headset', 'bun', 'beret']
export const avatarKeys = ['default', ...avatarChoices.map(({ key }) => key),
  ...legacyStyles.flatMap((style) => ['nila', 'mangga'].map((color) => `${style}-${color}`))]

export function avatarKey(key: string): string {
  return /^look-[0-5]-[0-4]-[0-9]$/.test(key) || avatarKeys.includes(key) ? key : 'default'
}

export function avatarLook(value: string): { skin: number; hair: number; shirt: number } {
  const key = avatarKey(value)
  if (key.startsWith('look-')) {
    const [, skin, hair, shirt] = key.split('-').map(Number)
    return { skin: skin!, hair: hair!, shirt: shirt! }
  }
  const sporty = key.startsWith('sporty-'), parts = key.split('-')
  const index = Math.max(0, sporty ? avatarStyles.findIndex((s) => s.key === parts[1]) : legacyStyles.indexOf(parts[0]!))
  const color = avatarColors.find((c) => c.key === parts[sporty ? 2 : 1])?.color ?? palette.shirt[0]!
  return { skin: index % palette.skin.length, hair: index % palette.hair.length, shirt: palette.shirt.indexOf(color) }
}

export function avatarURL(value: string): string {
  const look = avatarLook(value)
  const skin = palette.skin[look.skin]!
  const hair = palette.hair[look.hair]!
  const shirt = palette.shirt[look.shirt]!
  // Adapted from the reference avatar renderer; interpolate only allowlisted palette values.
  const svg = `<svg xmlns="http://www.w3.org/2000/svg" width="28" height="38" viewBox="0 0 28 38">
<clipPath id="av-head"><circle cx="14" cy="12" r="9"/></clipPath>
<rect x="3" y="20" width="22" height="16" rx="6" fill="${shirt}" stroke="#1d2140" stroke-width="2.5"/>
<circle cx="14" cy="12" r="9" fill="${skin}"/>
<rect x="0" y="0" width="28" height="9" fill="${hair}" clip-path="url(#av-head)"/>
<circle cx="14" cy="12" r="9" fill="none" stroke="#1d2140" stroke-width="2.5"/>
</svg>`
  // Phaser's SVG loader decodes all data URLs as Base64.
  return 'data:image/svg+xml;base64,' + btoa(svg)
}
