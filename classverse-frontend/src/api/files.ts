import { api, ApiError, getAccessToken, refreshAccessToken } from './client'
import type { FileList } from './types'

const base = (orgID: string) => `/api/orgs/${encodeURIComponent(orgID)}/files`

export const fileApi = {
  list: (orgID: string, offset?: number) => api<FileList>(base(orgID) + (offset === undefined ? '' : `?limit=20&offset=${offset}`)),
  remove: (orgID: string, id: string) =>
    api<void>(`${base(orgID)}/${encodeURIComponent(id)}`, { method: 'DELETE' }),
  savePosition: (orgID: string, id: string, position: number) =>
    api<void>(`${base(orgID)}/${encodeURIComponent(id)}/position`, {
      method: 'PUT',
      body: { position: Math.max(0, Math.min(1_000_000, Math.floor(position))) },
    }),
}

export const contentURL = (orgID: string, id: string) =>
  `${base(orgID)}/${encodeURIComponent(id)}/content`

// fetchContent loads the whole private file with the in-memory Bearer token.
// Native <audio>/<object> cannot send that header, so callers wrap the result in a Blob URL.
// shortcut: whole-file fetch (≤50 MB); switch PDF.js to range transport if large PDFs feel slow on phones.
export async function fetchContent(orgID: string, id: string, signal?: AbortSignal): Promise<Blob> {
  const send = () =>
    fetch(contentURL(orgID, id), {
      headers: { Authorization: `Bearer ${getAccessToken() ?? ''}` },
      credentials: 'same-origin',
      signal,
    })
  let res = await send()
  if (res.status === 401 && (await refreshAccessToken())) res = await send()
  if (!res.ok) {
    let body = { code: 'internal_error', message: 'Terjadi kesalahan' }
    try {
      body = ((await res.json()) as { error: typeof body }).error ?? body
    } catch {
      // non-JSON error
    }
    throw new ApiError(res.status, body)
  }
  return res.blob()
}
