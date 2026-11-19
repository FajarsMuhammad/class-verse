import { api } from './client'
import type { Level, QuestionInput, Quiz, QuizAttempt, QuizInput, QuizQuestion } from './types'

export type QuizResult = { user_id: string; display_name: string; best_score: number; attempts: number; last_finished_at: string }

const base = (orgID: string) => `/api/orgs/${encodeURIComponent(orgID)}/quizzes`
const one = (orgID: string, id: string) => `${base(orgID)}/${encodeURIComponent(id)}`
const attempt = (orgID: string, id: string, attemptID: string) => `${one(orgID, id)}/attempts/${encodeURIComponent(attemptID)}`

export interface QuizQuery {
  building_key?: string
  level?: Level | ''
  drafts?: boolean
  offset?: number
  limit?: number
  search?: string
  status?: string
  counts?: boolean
}

export const quizApi = {
  list: async (orgID: string, query: QuizQuery = {}) => {
    const p = new URLSearchParams()
    if (query.building_key) p.set('building_key', query.building_key)
    if (query.level) p.set('level', query.level)
    if (query.drafts) p.set('drafts', 'true')
    if (query.offset !== undefined) p.set('offset', String(query.offset))
    if (query.limit !== undefined) p.set('limit', String(query.limit))
    if (query.search) p.set('search', query.search)
    if (query.status) p.set('status', query.status)
    if (query.counts) p.set('counts', 'true')
    return api<{ items: Quiz[]; total?: number; counts?: { all: number; published: number; draft: number; empty: number } }>(`${base(orgID)}?${p}`)
  },
  create: (orgID: string, body: QuizInput) => api<Quiz>(base(orgID), { method: 'POST', body }),
  update: (orgID: string, id: string, body: QuizInput) => api<Quiz>(one(orgID, id), { method: 'PUT', body }),
  publish: (orgID: string, id: string, expected_version: number) => api<Quiz>(`${one(orgID,id)}/publish`, { method: 'POST', body: { expected_version } }),
  history: (orgID: string, id: string) => api<{items: {id: string; revision: number}[]}>(`${one(orgID,id)}/revisions`),
  archive: (orgID: string, id: string) => api<void>(one(orgID, id), { method: 'DELETE' }),
  results: (orgID: string, id: string, offset = 0) =>
    api<{ items: QuizResult[]; total: number }>(`${one(orgID, id)}/results?limit=20&offset=${offset}`),
  addQuestion: (orgID: string, id: string, body: QuestionInput) =>
    api<QuizQuestion>(`${one(orgID, id)}/questions`, { method: 'POST', body }),
  updateQuestion: (orgID: string, id: string, questionID: string, body: QuestionInput) =>
    api<QuizQuestion>(`${one(orgID, id)}/questions/${encodeURIComponent(questionID)}`, { method: 'PUT', body }),
  deleteQuestion: (orgID: string, id: string, questionID: string, expected_version?: number) =>
    api<void>(`${one(orgID, id)}/questions/${encodeURIComponent(questionID)}?expected_version=${expected_version ?? 0}`, { method: 'DELETE' }),
  reorder: (orgID: string, id: string, questionIDs: string[], expected_version?: number) =>
    api<void>(`${one(orgID, id)}/order`, { method: 'PUT', body: { question_ids: questionIDs, expected_version } }),
  start: (orgID: string, id: string, assignmentStepID?: string, moduleID?: string, stepID?: string) => api<QuizAttempt>(`${one(orgID, id)}/attempts${assignmentStepID ? `?assignment_step_id=${encodeURIComponent(assignmentStepID)}` : moduleID ? `?module_id=${encodeURIComponent(moduleID)}&step_id=${encodeURIComponent(stepID ?? '')}` : ''}`, { method: 'POST' }),
  answer: (orgID: string, id: string, attemptID: string, index: number, choice: number) =>
    api<void>(`${attempt(orgID, id, attemptID)}/answers/${index}`, { method: 'PUT', body: { choice } }),
  finish: (orgID: string, id: string, attemptID: string) =>
    api<QuizAttempt>(`${attempt(orgID, id, attemptID)}/finish`, { method: 'POST' }),
}

// Four shapes so an option never depends on colour alone (docs/tasks/mvp2-learning-plan.md, Q1 UI).
export const shapes = [
  { symbol: '●', name: 'Lingkaran' },
  { symbol: '▲', name: 'Segitiga' },
  { symbol: '■', name: 'Kotak' },
  { symbol: '◆', name: 'Belah ketupat' },
] as const
