import { api } from './client'
import type { components } from './schema'

export type AcademicYear = components['schemas']['AcademicYear']
export type SchoolClass = components['schemas']['SchoolClass']
export type RosterEntry = components['schemas']['RosterEntry']
export type Placement = components['schemas']['Placement']
const base = (org: string) => `/api/orgs/${encodeURIComponent(org)}`
const year = (org: string, id: string) => `${base(org)}/academic-years/${encodeURIComponent(id)}`
const cls = (org: string, id: string) => `${base(org)}/classes/${encodeURIComponent(id)}`

export const classApi = {
  years: (org: string) => api<{ items: AcademicYear[] }>(`${base(org)}/academic-years`),
  createYear: (org: string, label: string) => api<AcademicYear>(`${base(org)}/academic-years`, { method: 'POST', body: { label } }),
  activate: (org: string, id: string) => api<void>(`${year(org, id)}/active`, { method: 'PUT' }),
  classes: (org: string, id: string) => api<{ items: SchoolClass[] }>(`${year(org, id)}/classes`),
  create: (org: string, id: string, name: string) => api<SchoolClass>(`${year(org, id)}/classes`, { method: 'POST', body: { name } }),
  rename: (org: string, id: string, name: string) => api<SchoolClass>(cls(org, id), { method: 'PUT', body: { name } }),
  archive: (org: string, id: string) => api<void>(cls(org, id), { method: 'DELETE' }),
  roster: (org: string, id: string) => api<{ items: RosterEntry[] }>(`${year(org, id)}/roster`),
  place: (org: string, id: string, user: string, classID: string | null) =>
    api<void>(`${year(org, id)}/roster/${encodeURIComponent(user)}`, { method: 'PUT', body: { class_id: classID } }),
  history: (org: string, user: string) => api<{ items: Placement[] }>(`${base(org)}/members/${encodeURIComponent(user)}/placements`),
}
