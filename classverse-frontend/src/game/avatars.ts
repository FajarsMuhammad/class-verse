import round from '@/assets/avatars/round.svg?raw'
import cap from '@/assets/avatars/cap.svg?raw'
import glasses from '@/assets/avatars/glasses.svg?raw'
import headset from '@/assets/avatars/headset.svg?raw'
import bun from '@/assets/avatars/bun.svg?raw'
import beret from '@/assets/avatars/beret.svg?raw'
import { cssToken } from './tiles'
import type { Direction } from '@/realtime/messages'

const portraits = import.meta.glob<string>('../assets/avatars/sporty/*-idle.png', {
  eager: true, query: '?url', import: 'default',
})
const sheets = import.meta.glob<string>('../assets/avatars/sporty/*.png', {
  eager: true, query: '?url', import: 'default',
})
export const avatarStyles = [
  { key: 'runner', label: 'Pelari' }, { key: 'cap', label: 'Topi sport' },
  { key: 'headphones', label: 'Headphone' }, { key: 'varsity', label: 'Varsity' },
  { key: 'ponytail', label: 'Kuncir' }, { key: 'hoodie', label: 'Hoodie' },
  { key: 'glasses', label: 'Kacamata' },
]
export const avatarColors = [
  { key: 'nila', label: 'Nila' }, { key: 'mangga', label: 'Mangga' },
  { key: 'zamrud', label: 'Zamrud' }, { key: 'laut', label: 'Biru laut' },
  { key: 'koral', label: 'Koral' }, { key: 'grafit', label: 'Grafit' },
]
export const avatarChoices = avatarStyles.flatMap((style) => avatarColors.map((color) => ({
  key: `sporty-${style.key}-${color.key}`, label: `${style.label} ${color.label}`,
})))

// Row order and frame timing match assets/avatars/sporty/manifest.json.
export function avatarFrame(dir: Direction, moving: boolean, time: number, reducedMotion = false) {
  const row = ['down', 'left', 'right', 'up'].indexOf(dir)
  return row * 5 + (moving && !reducedMotion ? 1 + Math.floor(time / 125) % 4 : 0)
}

export function avatarSheetURL(key: string): string | null {
  if (!avatarChoices.some((a) => a.key === key)) return null
  return sheets[`../assets/avatars/sporty/${key.slice(7)}.png`] ?? null
}

const accessories = { round, cap, glasses, headset, bun, beret }
const names = {
  round: 'Bulat',
  cap: 'Topi',
  glasses: 'Kacamata',
  headset: 'Headset',
  bun: 'Sanggul',
  beret: 'Baret',
}
const bodyColors = {
  round: '#7C83FD',
  cap: '#FFB547',
  glasses: '#2FB58A',
  headset: '#5BB8F0',
  bun: '#F48FB1',
  beret: '#A78BFA',
}
const legacyChoices = Object.keys(accessories).flatMap((accessory) =>
  ['nila', 'mangga'].map((color) => ({
    key: `${accessory}-${color}`,
    label: `${names[accessory as keyof typeof names]} ${color === 'nila' ? 'Nila' : 'Mangga'}`,
  })),
)

export function avatarKey(key: string): string {
  return [...avatarChoices, ...legacyChoices].some((a) => a.key === key) ? key : 'default'
}

export function avatarURL(key: string): string {
  key = avatarKey(key)
  if (key.startsWith('sporty-')) return portraits[`../assets/avatars/sporty/${key.slice(7)}-idle.png`]!
  if (key === 'default') return `data:image/svg+xml;base64,${btoa(round)}`
  const [accessory, color] = key.split('-')
  const a = accessory as keyof typeof accessories
  // Replace the body fill only; preserve accessory and facial detail colors.
  const svg = accessories[a].replace(
    new RegExp(`fill="${bodyColors[a]}"`, 'i'),
    `fill="${cssToken(`--avatar-${color}`)}"`,
  )
  return `data:image/svg+xml;base64,${btoa(svg)}`
}
