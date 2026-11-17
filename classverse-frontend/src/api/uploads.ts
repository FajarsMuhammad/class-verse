import { api, ApiError } from './client'
import type { UploadProgress } from './types'

export type { UploadProgress }

interface SavedUpload { id: string; sha256: string; size: number }
const maxFileBytes = 50_000_000

// Session IDs are not credentials: server rechecks user ownership and membership.
// After reload, reselect the same file; its hash prevents mixing different files.
export async function uploadFile(
  orgID: string,
  userID: string,
  file: File,
  kind: 'pdf' | 'audio',
  onProgress: (sent: number, total: number) => void,
  signal?: AbortSignal,
): Promise<UploadProgress> {
  if (file.size < 1 || file.size > maxFileBytes) throw new Error('Ukuran file harus 1 byte–50 MB')
  const digest = await crypto.subtle.digest('SHA-256', await file.arrayBuffer())
  const sha256 = Array.from(new Uint8Array(digest), (b) => b.toString(16).padStart(2, '0')).join('')
  const key = `kv-upload:${userID}:${orgID}:${sha256}:${kind}`
  const base = `/api/orgs/${encodeURIComponent(orgID)}/uploads`
  let saved: SavedUpload | null = null
  const raw = localStorage.getItem(key)
  if (raw) {
    try { saved = JSON.parse(raw) as SavedUpload } catch { localStorage.removeItem(key) }
  }
  let state: UploadProgress
  if (saved?.sha256 === sha256 && saved.size === file.size && typeof saved.id === 'string') {
    // Do not create another reservation after an ambiguous network error.
    try {
      state = await api<UploadProgress>(`${base}/${encodeURIComponent(saved.id)}`, { signal })
    } catch (err) {
      if (!(err instanceof ApiError) || err.status !== 404) throw err
      state = await api<UploadProgress>(base, { method: 'POST', body: { id: saved.id, name: file.name, size: file.size, kind, sha256 }, signal })
    }
  } else {
    const id = crypto.randomUUID()
    // Persist before sending: a lost create response retries the same idempotency ID.
    localStorage.setItem(key, JSON.stringify({ id, sha256, size: file.size }))
    state = await api<UploadProgress>(base, { method: 'POST', body: { id, name: file.name, size: file.size, kind, sha256 }, signal })
  }
  const path = `${base}/${encodeURIComponent(state.id)}`
  if (state.state === 'assembled') {
    localStorage.removeItem(key)
    return state
  }
  if (state.chunk_size !== 5 * 1024 * 1024 || state.size !== file.size) throw new Error('Status upload tidak valid')
  const count = Math.ceil(file.size / state.chunk_size)
  const present = new Set(state.parts)
  let sent = state.parts.reduce((n, p) => n + Math.min(state.chunk_size, file.size - (p - 1) * state.chunk_size), 0)
  onProgress(sent, file.size)
  for (let part = 1; part <= count; part++) {
    if (present.has(part)) continue
    signal?.throwIfAborted()
    const start = (part - 1) * state.chunk_size
    const chunk = file.slice(start, Math.min(start + state.chunk_size, file.size))
    await api<void>(`${path}/parts/${part}`, { method: 'PUT', body: chunk, signal })
    sent += chunk.size
    onProgress(sent, file.size)
  }
  try {
    state = await api<UploadProgress>(`${path}/complete`, { method: 'POST', signal })
  } catch (err) {
    // Server discarded an invalid file; a fresh upload must not resume a dead session.
    if (err instanceof ApiError && err.status === 400) localStorage.removeItem(key)
    throw err
  }
  // Done: the next upload of the same bytes starts a new file.
  localStorage.removeItem(key)
  return state
}
