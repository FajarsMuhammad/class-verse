import { api } from './client'
import type { Level, Module, ModuleInput, ModuleResume, ModuleStep, StepInput } from './types'

const base = (orgID: string) => `/api/orgs/${encodeURIComponent(orgID)}/modules`
const one = (orgID: string, id: string) => `${base(orgID)}/${encodeURIComponent(id)}`

export interface ModuleQuery {
  building_key?: string
  level?: Level | ''
  drafts?: boolean
  offset?: number
  limit?: number
  module_id?: string
  search?: string
  status?: string
  archived?: boolean
}

export const moduleApi = {
  list: async (orgID: string, query: ModuleQuery = {}) => {
    const p = new URLSearchParams()
    if (query.building_key) p.set('building_key', query.building_key)
    if (query.level) p.set('level', query.level)
    if (query.drafts) p.set('drafts', 'true')
    if (query.archived) p.set('archived', 'true')
    if (query.offset !== undefined) p.set('offset', String(query.offset))
    if (query.limit !== undefined) p.set('limit', String(query.limit))
    if (query.module_id) p.set('module_id', query.module_id)
    if (query.search) p.set('search', query.search)
    if (query.status) p.set('status', query.status)
    return api<{ items: Module[]; total?: number }>(`${base(orgID)}?${p}`)
  },
  create: (orgID: string, body: ModuleInput) => api<Module>(base(orgID), { method: 'POST', body }),
  update: (orgID: string, id: string, body: ModuleInput) => api<Module>(one(orgID, id), { method: 'PUT', body }),
  publish: (orgID: string, id: string, expected_version: number) => api<Module>(`${one(orgID, id)}/publish`, { method: 'POST', body: { expected_version } }),
  history: (orgID: string, id: string) => api<{items: {id: string; revision: number}[]}>(`${one(orgID,id)}/revisions`),
  archive: (orgID: string, id: string) => api<void>(one(orgID, id), { method: 'DELETE' }),
  addStep: (orgID: string, id: string, body: StepInput) =>
    api<ModuleStep>(`${one(orgID, id)}/steps`, { method: 'POST', body }),
  updateStep: (orgID: string, id: string, stepID: string, body: StepInput) =>
    api<ModuleStep>(`${one(orgID, id)}/steps/${encodeURIComponent(stepID)}`, { method: 'PUT', body }),
  archiveStep: (orgID: string, id: string, stepID: string, expected_version?: number) =>
    api<void>(`${one(orgID, id)}/steps/${encodeURIComponent(stepID)}?expected_version=${expected_version ?? 0}`, { method: 'DELETE' }),
  setDone: (orgID: string, id: string, stepID: string, done: boolean) =>
    api<void>(`${one(orgID, id)}/steps/${encodeURIComponent(stepID)}/done`, { method: done ? 'PUT' : 'DELETE' }),
  resume: (orgID: string) => api<{ resume: ModuleResume | null }>(`${base(orgID)}/resume`),
  reorder: (orgID: string, id: string, stepIDs: string[], expected_version?: number) =>
    api<void>(`${one(orgID, id)}/order`, { method: 'PUT', body: { step_ids: stepIDs, expected_version } }),
}

export const levels: Level[] = ['SD', 'SMP', 'SMA', 'Umum']

export const kindLabel: Record<ModuleStep['kind'], string> = { materi: 'Materi', berkas: 'Bacaan', video: 'Video', kuis: 'Kuis' }

// Privacy-enhanced embed; the server stores only the 11-character ID.
export const embedURL = (videoID: string) => `https://www.youtube-nocookie.com/embed/${encodeURIComponent(videoID)}?rel=0`
