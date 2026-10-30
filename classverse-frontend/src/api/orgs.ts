import { api } from './client'
import type {
  CreateInviteRequest,
  CreateOrgRequest,
  Invite,
  InviteList,
  InvitePreview,
  Membership,
  Organization,
} from './types'

const enc = encodeURIComponent

export const orgApi = {
  create: (input: CreateOrgRequest) =>
    api<Organization>('/api/orgs', { method: 'POST', body: input }),
  get: (orgId: string) => api<Organization>(`/api/orgs/${enc(orgId)}`),
}

export const inviteApi = {
  list: (orgId: string, cursor?: string) =>
    api<InviteList>(
      `/api/orgs/${enc(orgId)}/invites` + (cursor ? `?cursor=${enc(cursor)}` : ''),
    ),
  create: (orgId: string, input: CreateInviteRequest) =>
    api<Invite>(`/api/orgs/${enc(orgId)}/invites`, { method: 'POST', body: input }),
  revoke: (orgId: string, inviteId: string) =>
    api<void>(`/api/orgs/${enc(orgId)}/invites/${enc(inviteId)}`, { method: 'DELETE' }),
  preview: (code: string) => api<InvitePreview>(`/api/invites/${enc(code)}`),
  accept: (code: string) =>
    api<Membership>(`/api/invites/${enc(code)}/accept`, { method: 'POST' }),
}

const CODE_ALPHABET = 'ABCDEFGHJKMNPQRSTUVWXYZ23456789'

// normalizeInviteCode mirrors the server: uppercase, drop dashes and spaces.
// It returns null when the input cannot be a valid code.
export function normalizeInviteCode(input: string): string | null {
  const s = input.toUpperCase().replace(/[-\s]/g, '')
  if (s.length !== 8) return null
  for (const ch of s) if (!CODE_ALPHABET.includes(ch)) return null
  return `${s.slice(0, 4)}-${s.slice(4)}`
}

// slugify turns a school name into a slug suggestion that matches the
// server pattern ^[a-z0-9](?:[a-z0-9-]{1,46}[a-z0-9])$.
export function slugify(name: string): string {
  return name
    .normalize('NFKD')
    .replace(/[\u0300-\u036f]/g, '')
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '')
    .slice(0, 48)
    .replace(/-+$/g, '')
}
