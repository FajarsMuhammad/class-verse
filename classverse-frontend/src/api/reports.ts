import { api } from './client'
import type { CreateReportRequest, ModerationLog, Report, ReportList, ReportStatus } from './types'

const enc = encodeURIComponent

export const reportApi = {
  create: (input: CreateReportRequest) =>
    api<Report>('/api/reports', { method: 'POST', body: input }),
  list: (orgId: string, options: { status?: ReportStatus; cursor?: string } = {}) => {
    const query = new URLSearchParams()
    if (options.status) query.set('status', options.status)
    if (options.cursor) query.set('cursor', options.cursor)
    return api<ReportList>(`/api/orgs/${enc(orgId)}/reports${query.size ? `?${query}` : ''}`)
  },
  review: (id: string, status: 'reviewed' | 'dismissed') =>
    api<Report>(`/api/reports/${enc(id)}`, { method: 'PATCH', body: { status } }),
  moderationLog: (orgId: string, cursor?: string) =>
    api<ModerationLog>(
      `/api/orgs/${enc(orgId)}/moderation-log${cursor ? `?cursor=${enc(cursor)}` : ''}`,
    ),
}
