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

// Accessories drawn over the base look; 0 = none. Same ids as the server allowlist (internal/auth/validate.go).
export const avatarAccessories = [
  { id: 0, label: 'Tanpa' }, { id: 1, label: 'Hijab' }, { id: 2, label: 'Topi' }, { id: 3, label: 'Kacamata' },
] as const

export interface AvatarLook { skin: number; hair: number; shirt: number; accessory: number; accessoryColor: number }

// lookKey is the saved key for a look; without an accessory it stays the 3-part key, so old keys keep their meaning.
export function lookKey(l: AvatarLook): string {
  const base = `look-${l.skin}-${l.hair}-${l.shirt}`
  return l.accessory ? `${base}-${l.accessory}-${l.accessoryColor}` : base
}

export function avatarKey(key: string): string {
  return /^look-[0-5]-[0-4]-[0-9](?:-[1-3]-[0-7])?$/.test(key) || avatarKeys.includes(key) ? key : 'default'
}

export function avatarLook(value: string): AvatarLook {
  const key = avatarKey(value)
  if (key.startsWith('look-')) {
    const [, skin, hair, shirt, accessory = 0, accessoryColor = 0] = key.split('-').map(Number)
    return { skin: skin!, hair: hair!, shirt: shirt!, accessory, accessoryColor }
  }
  const sporty = key.startsWith('sporty-'), parts = key.split('-')
  const index = Math.max(0, sporty ? avatarStyles.findIndex((s) => s.key === parts[1]) : legacyStyles.indexOf(parts[0]!))
  const color = avatarColors.find((c) => c.key === parts[sporty ? 2 : 1])?.color ?? palette.shirt[0]!
  return { skin: index % palette.skin.length, hair: index % palette.hair.length, shirt: palette.shirt.indexOf(color), accessory: 0, accessoryColor: 0 }
}

export function avatarURL(value: string): string {
  const look = avatarLook(value)
  const skin = palette.skin[look.skin]!
  const hair = palette.hair[look.hair]!
  const shirt = palette.shirt[look.shirt]!
  const acc = palette.accessory[look.accessoryColor]!
  // Hijab replaces the hair and frames the face; cap sits on the hair; glasses go over the eyes.
  const head = look.accessory === 1
    ? `<path d="M3 13a11 11 0 0 1 22 0v9H3z" fill="${acc}" stroke="#1d2140" stroke-width="2.5"/>
<circle cx="14" cy="13" r="6.5" fill="${skin}" stroke="#1d2140" stroke-width="1.5"/>`
    : `<circle cx="14" cy="12" r="9" fill="${skin}"/>
<rect x="0" y="0" width="28" height="9" fill="${hair}" clip-path="url(#av-head)"/>
<circle cx="14" cy="12" r="9" fill="none" stroke="#1d2140" stroke-width="2.5"/>`
  const extra = look.accessory === 2
    ? `<path d="M5 9a9 7 0 0 1 18 0z" fill="${acc}" stroke="#1d2140" stroke-width="2"/><rect x="13" y="7" width="12" height="3" rx="1.5" fill="${acc}" stroke="#1d2140" stroke-width="1.5"/>`
    : look.accessory === 3
      ? `<g fill="none" stroke="${acc}" stroke-width="1.8"><circle cx="10.5" cy="13" r="2.6"/><circle cx="17.5" cy="13" r="2.6"/><path d="M13.1 13h1.8"/></g>`
      : ''
  // Adapted from the reference avatar renderer; interpolate only allowlisted palette values.
  const svg = `<svg xmlns="http://www.w3.org/2000/svg" width="28" height="38" viewBox="0 0 28 38">
<clipPath id="av-head"><circle cx="14" cy="12" r="9"/></clipPath>
<rect x="3" y="20" width="22" height="16" rx="6" fill="${shirt}" stroke="#1d2140" stroke-width="2.5"/>
${head}${extra}
</svg>`
  // Phaser's SVG loader decodes all data URLs as Base64.
  return 'data:image/svg+xml;base64,' + btoa(svg)
}
