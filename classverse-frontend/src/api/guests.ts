import { api, toError } from './client'
import type { components } from './schema'
import type { VoiceToken } from './types'

export type GuestSession = components['schemas']['GuestSession']
export type GuestLink = components['schemas']['GuestLink']
export type GuestRequest = components['schemas']['GuestRequest']
export type GuestPreview = components['schemas']['GuestLinkPreview']
const enc = encodeURIComponent
const hostPath = (id: string) => `/api/rooms/${enc(id)}`

export const guestHostApi = {
  links: (id: string) => api<{ items: GuestLink[] }>(`${hostPath(id)}/guest-links`),
  create: (id: string, hours: number, max: number) => api<GuestLink>(`${hostPath(id)}/guest-links`, {
    method: 'POST', body: { expires_in_hours: hours, max_guests: max },
  }),
  revoke: (id: string, link: string) => api<void>(`${hostPath(id)}/guest-links/${enc(link)}`, { method: 'DELETE' }),
  requests: (id: string) => api<{ items: GuestRequest[] }>(`${hostPath(id)}/guest-requests`),
  decide: (id: string, request: string, decision: 'approve' | 'deny') => api<GuestRequest>(`${hostPath(id)}/guest-requests/${enc(request)}`, { method: 'POST', body: { decision } }),
}

// Guest credentials never enter account auth, refresh, storage, cookies or URLs.
export function guestClient() {
  let token: string | null = null
  async function call<T>(path: string, body?: unknown): Promise<T> {
    const res = await fetch(`/api/guest${path}`, {
      method: body === undefined ? 'GET' : 'POST', credentials: 'omit', cache: 'no-store',
      headers: { ...(token ? { Authorization: `Bearer ${token}` } : {}), ...(body === undefined ? {} : { 'Content-Type': 'application/json' }) },
      body: body === undefined ? undefined : JSON.stringify(body),
    })
    if (!res.ok) throw await toError(res)
    return res.json() as Promise<T>
  }
  async function session(path: string, body?: unknown) {
    const value = await call<GuestSession>(path, body)
    token = value.token
    return value
  }
  return {
    preview: (code: string) => call<GuestPreview>(`/links/${enc(code)}`),
    request: (code: string, name: string, requestKey: string) => session(`/links/${enc(code)}/requests`, { display_name: name, request_key: requestKey }),
    status: () => session('/me'),
    getToken: async () => token,
    voiceToken: (id: string) => call<VoiceToken>(`/rooms/${enc(id)}/voice-token`, {}),
    clear: () => { token = null },
  }
}
export type GuestClient = ReturnType<typeof guestClient>
