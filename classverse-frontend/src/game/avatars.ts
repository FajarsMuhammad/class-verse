import round from '@/assets/avatars/round.svg?raw'
import cap from '@/assets/avatars/cap.svg?raw'
import glasses from '@/assets/avatars/glasses.svg?raw'
import headset from '@/assets/avatars/headset.svg?raw'
import bun from '@/assets/avatars/bun.svg?raw'
import beret from '@/assets/avatars/beret.svg?raw'
import { cssToken } from './tiles'

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
export const avatarChoices = Object.keys(accessories).flatMap((accessory) =>
  ['nila', 'mangga'].map((color) => ({
    key: `${accessory}-${color}`,
    label: `${names[accessory as keyof typeof names]} ${color === 'nila' ? 'Nila' : 'Mangga'}`,
  })),
)

export function avatarKey(key: string): string {
  return avatarChoices.some((a) => a.key === key) ? key : 'default'
}

export function avatarURL(key: string): string {
  key = avatarKey(key)
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
