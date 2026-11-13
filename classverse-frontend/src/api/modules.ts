import { api } from './client'
import type { Level, Module, ModuleInput, ModuleResume, ModuleStep, StepInput } from './types'

const base = (orgID: string) => `/api/orgs/${encodeURIComponent(orgID)}/modules`
const one = (orgID: string, id: string) => `${base(orgID)}/${encodeURIComponent(id)}`

export interface ModuleQuery {
  building_key?: string
  level?: Level | ''
  drafts?: boolean
}

export const moduleApi = {
  list: (orgID: string, query: ModuleQuery = {}) => {
    const p = new URLSearchParams()
    if (query.building_key) p.set('building_key', query.building_key)
    if (query.level) p.set('level', query.level)
    if (query.drafts) p.set('drafts', 'true')
    const qs = p.toString()
    return api<{ items: Module[] }>(qs ? `${base(orgID)}?${qs}` : base(orgID))
  },
  create: (orgID: string, body: ModuleInput) => api<Module>(base(orgID), { method: 'POST', body }),
  update: (orgID: string, id: string, body: ModuleInput) => api<Module>(one(orgID, id), { method: 'PUT', body }),
  archive: (orgID: string, id: string) => api<void>(one(orgID, id), { method: 'DELETE' }),
  addStep: (orgID: string, id: string, body: StepInput) =>
    api<ModuleStep>(`${one(orgID, id)}/steps`, { method: 'POST', body }),
  updateStep: (orgID: string, id: string, stepID: string, body: StepInput) =>
    api<ModuleStep>(`${one(orgID, id)}/steps/${encodeURIComponent(stepID)}`, { method: 'PUT', body }),
  archiveStep: (orgID: string, id: string, stepID: string) =>
    api<void>(`${one(orgID, id)}/steps/${encodeURIComponent(stepID)}`, { method: 'DELETE' }),
  setDone: (orgID: string, id: string, stepID: string, done: boolean) =>
    api<void>(`${one(orgID, id)}/steps/${encodeURIComponent(stepID)}/done`, { method: done ? 'PUT' : 'DELETE' }),
  resume: (orgID: string) => api<{ resume: ModuleResume | null }>(`${base(orgID)}/resume`),
  reorder: (orgID: string, id: string, stepIDs: string[]) =>
    api<void>(`${one(orgID, id)}/order`, { method: 'PUT', body: { step_ids: stepIDs } }),
}

export const levels: Level[] = ['SD', 'SMP', 'SMA', 'Umum']

export const kindLabel: Record<ModuleStep['kind'], string> = { materi: 'Materi', berkas: 'Bacaan', video: 'Video' }

// Privacy-enhanced embed; the server stores only the 11-character ID.
export const embedURL = (videoID: string) => `https://www.youtube-nocookie.com/embed/${encodeURIComponent(videoID)}?rel=0`
