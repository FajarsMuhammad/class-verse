import { api } from './client'
import type { components } from './schema'
export type StudentGradebook = components['schemas']['StudentGradebook']
export type GradebookPage = components['schemas']['GradebookPage']
export interface ReportFilter { year_id?: string; class_id?: string; module_id?: string; quiz_id?: string; building_key?: string }
const base = (org: string) => `/api/orgs/${encodeURIComponent(org)}/gradebook`
function query(filter: ReportFilter, offset?: number) {
  const p = new URLSearchParams()
  for (const [k, v] of Object.entries(filter)) if (v) p.set(k, v)
  if (offset !== undefined) { p.set('offset', String(offset)); p.set('limit', '20') }
  return `?${p}`
}
export const gradebookApi = {
  me: (org: string, filter: ReportFilter) => api<StudentGradebook>(`${base(org)}/me${query(filter)}`),
  student: (org: string, user: string, filter: ReportFilter) => api<StudentGradebook>(`${base(org)}/students/${encodeURIComponent(user)}${query(filter)}`),
  roster: (org: string, filter: ReportFilter, offset = 0) => api<GradebookPage>(`${base(org)}${query(filter, offset)}`),
  csv: (org: string, filter: ReportFilter) => api<Blob>(`${base(org)}/csv${query(filter)}`, { responseType: 'blob' }),
}
