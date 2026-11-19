import { api } from './client'
import type { components } from './schema'

export type AcademicYear = components['schemas']['AcademicYear']
export type SchoolClass = components['schemas']['SchoolClass']
export type RosterEntry = components['schemas']['RosterEntry']
export type Placement = components['schemas']['Placement']
export type ClassTeacher = components['schemas']['ClassTeacher']
export type Person = components['schemas']['Person']
export type Staff = { user_id: string; display_name: string; role: 'teacher' | 'admin' }
export type MoveResult = { user_id: string; error: 'not_student' | 'has_class' | null }
/** Roster filter: class_id is a class UUID or 'none' (no class yet). */
export type RosterQuery = { class_id?: string; q?: string; offset?: number; limit?: number }
const base = (org: string) => `/api/orgs/${encodeURIComponent(org)}`
const year = (org: string, id: string) => `${base(org)}/academic-years/${encodeURIComponent(id)}`
const cls = (org: string, id: string) => `${base(org)}/classes/${encodeURIComponent(id)}`

export const classApi = {
  years: (org: string, offset?: number) => api<{ items: AcademicYear[] }>(`${base(org)}/academic-years${offset === undefined ? '' : `?limit=20&offset=${offset}`}`),
  createYear: (org: string, label: string) => api<AcademicYear>(`${base(org)}/academic-years`, { method: 'POST', body: { label } }),
  activate: (org: string, id: string) => api<void>(`${year(org, id)}/active`, { method: 'PUT' }),
  classes: (org: string, id: string, offset?: number) => api<{ items: SchoolClass[] }>(`${year(org, id)}/classes${offset === undefined ? '' : `?limit=20&offset=${offset}`}`),
  create: (org: string, id: string, name: string) => api<SchoolClass>(`${year(org, id)}/classes`, { method: 'POST', body: { name } }),
  rename: (org: string, id: string, name: string) => api<SchoolClass>(cls(org, id), { method: 'PUT', body: { name } }),
  archive: (org: string, id: string) => api<void>(cls(org, id), { method: 'DELETE' }),
  roster: (org: string, id: string, f: RosterQuery = {}) => {
    const qs = new URLSearchParams(Object.entries(f).filter(([, v]) => v !== undefined && v !== '').map(([k, v]) => [k, String(v)]))
    return api<{ items: RosterEntry[]; total: number }>(`${year(org, id)}/roster${qs.size ? `?${qs}` : ''}`)
  },
  move: (org: string, id: string, users: string[], classID: string | null) =>
    api<{ items: MoveResult[] }>(`${year(org, id)}/moves`, { method: 'POST', body: { user_ids: users, class_id: classID } }),
  copy: (org: string, id: string, from: string, teachers: boolean) =>
    api<{ created: number; skipped: string[] }>(`${year(org, id)}/copy`, { method: 'POST', body: { from_year_id: from, teachers } }),
  promote: (org: string, id: string, from: string, mappings: { from_class_id: string; to_class_id: string }[]) =>
    api<{ moved: number; skipped: Person[] }>(`${year(org, id)}/promotions`, { method: 'POST', body: { from_year_id: from, mappings } }),
  staff: (org: string) => api<{ items: Staff[] }>(`${base(org)}/staff`),
  teachers: (org: string, id: string) => api<{ items: ClassTeacher[] }>(`${cls(org, id)}/teachers`),
  setTeacher: (org: string, id: string, user: string, role: 'homeroom' | 'subject', subject = '') =>
    api<void>(`${cls(org, id)}/teachers/${encodeURIComponent(user)}`, { method: 'PUT', body: { role, subject } }),
  removeTeacher: (org: string, id: string, user: string) =>
    api<void>(`${cls(org, id)}/teachers/${encodeURIComponent(user)}`, { method: 'DELETE' }),
  place: (org: string, id: string, user: string, classID: string | null) =>
    api<void>(`${year(org, id)}/roster/${encodeURIComponent(user)}`, { method: 'PUT', body: { class_id: classID } }),
  history: (org: string, user: string) => api<{ items: Placement[] }>(`${base(org)}/members/${encodeURIComponent(user)}/placements`),
}
