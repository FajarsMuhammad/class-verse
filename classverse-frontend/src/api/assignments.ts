import { api } from './client'
import type { components } from './schema'
export type Assignment = components['schemas']['Assignment']
export type AssignmentStep = components['schemas']['AssignmentStep']
export type AssignmentRecipient = components['schemas']['AssignmentRecipient']
export type AssignmentInput = components['schemas']['AssignmentInput']
const base = (org: string) => `/api/orgs/${encodeURIComponent(org)}/assignments`
const one = (org: string, id: string) => `${base(org)}/${encodeURIComponent(id)}`
export const assignmentApi = {
  list: (org: string, offset = 0, search = '', classID = '') => api<{ items: Assignment[]; total?: number }>(`${base(org)}?limit=20&offset=${offset}&search=${encodeURIComponent(search)}${classID ? `&class_id=${encodeURIComponent(classID)}` : ''}`),
  issue: (org: string, body: AssignmentInput) => api<{ id: string }>(base(org), { method: 'POST', body }),
  cancel: (org: string, id: string) => api<void>(one(org, id), { method: 'DELETE' }),
  steps: (org: string, id: string) => api<{ items: AssignmentStep[] }>(`${one(org, id)}/steps`),
  roster: (org: string, id: string) => api<{ items: AssignmentRecipient[] }>(`${one(org, id)}/roster`),
  complete: (org: string, id: string, step: string) => api<void>(`${one(org, id)}/steps/${encodeURIComponent(step)}/done`, { method: 'PUT' }),
}
export function deadline(value: string | null) {
  return value ? new Date(value).toLocaleString('id-ID', { timeZone: 'Asia/Jakarta', dateStyle: 'medium', timeStyle: 'short' }) + ' WIB' : 'Tanpa tenggat'
}
