import world from '@/assets/world/world.json'
import type { ModuleStep } from '@/api/types'

// placeInfo is a building's name, subject and map colour (world.json), so tasks and modules share the map's colours.
export function placeInfo(key: string) {
  const p = world.places.find((place) => place.id === key)
  return { name: p?.name ?? key, subject: p?.subject ?? 'Umum', color: p?.uiColor ?? 'var(--kv-primary)' }
}

// Fields a step needs to be played; module steps and class-task steps both have them.
export type PlayerStep = Pick<ModuleStep, 'id' | 'kind' | 'title' | 'body' | 'rich_content' | 'video_id' | 'file_id' | 'file_kind' | 'quiz_id' | 'minutes' | 'done'>
  & { unlocked?: boolean }

// firstOpen is the step to open first: the first unfinished step the learner may open, else the first step.
export function firstOpen<T extends PlayerStep>(steps: T[]): T | undefined {
  return steps.find((s) => !s.done && s.unlocked !== false) ?? steps[0]
}
