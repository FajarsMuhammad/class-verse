// API client for the Go backend. Access tokens live only in memory (never in
// localStorage); the refresh token is an HttpOnly cookie the browser sends to
// /api/auth automatically.

export interface ApiErrorBody {
  code: string
  message: string
  fields?: Record<string, string>
}

export class ApiError extends Error {
  readonly status: number
  readonly code: string
  readonly fields: Record<string, string>

  constructor(status: number, body: ApiErrorBody) {
    super(body.message)
    this.status = status
    this.code = body.code
    this.fields = body.fields ?? {}
  }
}

type TokenListener = (token: string | null) => void

let accessToken: string | null = null
let inflight: Promise<string | null> | null = null
const listeners = new Set<TokenListener>()

export function getAccessToken(): string | null {
  return accessToken
}

export function setAccessToken(token: string | null): void {
  accessToken = token
  for (const l of listeners) l(token)
}

// onTokenChange lets the auth store react when the client logs the user out
// (for example after a failed refresh).
export function onTokenChange(listener: TokenListener): () => void {
  listeners.add(listener)
  return () => listeners.delete(listener)
}

// refreshAccessToken rotates the refresh cookie and stores a new access token.
// Calls are single-flight inside a tab and serialized across tabs with the Web
// Locks API, so two tabs never present the same refresh token (which the
// server would treat as reuse and revoke the session).
export function refreshAccessToken(): Promise<string | null> {
  if (!inflight) {
    const seen = accessToken
    inflight = withCrossTabLock(() => doRefresh(seen)).finally(() => {
      inflight = null
    })
  }
  return inflight
}

async function withCrossTabLock<T>(fn: () => Promise<T>): Promise<T> {
  if (typeof navigator !== 'undefined' && navigator.locks) {
    return navigator.locks.request('kv-refresh', fn)
  }
  return fn()
}

async function doRefresh(seen: string | null): Promise<string | null> {
  // Another tab may have rotated the cookie while we waited for the lock; the
  // rotated cookie is already in the jar, so refreshing now is safe.
  if (accessToken !== seen && accessToken !== null) return accessToken
  let res: Response
  try {
    res = await fetch('/api/auth/refresh', { method: 'POST', credentials: 'same-origin' })
  } catch {
    return null
  }
  if (!res.ok) {
    if (res.status === 401 || res.status === 403) setAccessToken(null)
    return null
  }
  const body = (await res.json()) as { access_token: string }
  setAccessToken(body.access_token)
  return body.access_token
}

interface RequestOptions {
  method?: string
  body?: unknown
  auth?: boolean
  signal?: AbortSignal
  responseType?: 'json' | 'blob'
}

// api calls an /api endpoint and returns the parsed JSON (or undefined for
// 202/204). On 401 it refreshes once and retries; if that fails the user is
// logged out (token listeners receive null).
export async function api<T>(path: string, opts: RequestOptions = {}): Promise<T> {
  const auth = opts.auth ?? true
  let res = await send(path, opts, auth)
  if (res.status === 401 && auth) {
    const token = await refreshAccessToken()
    if (!token) {
      setAccessToken(null)
      throw await toError(res)
    }
    res = await send(path, opts, auth)
  }
  if (!res.ok) throw await toError(res)
  if (res.status === 202 || res.status === 204) return undefined as T
  return (opts.responseType === 'blob' ? await res.blob() : await res.json()) as T
}

function send(path: string, opts: RequestOptions, auth: boolean): Promise<Response> {
  const headers: Record<string, string> = {}
  const binary = opts.body instanceof Blob
  if (opts.body !== undefined) headers['Content-Type'] = binary ? 'application/octet-stream' : 'application/json'
  if (auth && accessToken) headers.Authorization = `Bearer ${accessToken}`
  return fetch(path, {
    method: opts.method ?? 'GET',
    headers,
    body: binary ? (opts.body as Blob) : opts.body === undefined ? undefined : JSON.stringify(opts.body),
    signal: opts.signal,
    credentials: 'same-origin',
  })
}

async function toError(res: Response): Promise<ApiError> {
  try {
    const body = (await res.json()) as { error?: ApiErrorBody }
    if (body.error?.code) return new ApiError(res.status, body.error)
  } catch {
    // fall through
  }
  return new ApiError(res.status, { code: 'internal_error', message: 'Terjadi kesalahan' })
}

// errorMessage maps API error codes to friendly Indonesian sentences.
export function errorMessage(err: unknown): string {
  if (!(err instanceof ApiError))
    return 'Tidak bisa terhubung ke server. Periksa koneksi lalu coba lagi.'
  switch (err.code) {
    case 'validation_failed':
      return 'Periksa kembali isian yang ditandai.'
    case 'invalid_credentials':
      return 'Email atau password salah.'
    case 'email_not_verified':
      return 'Email kamu belum diverifikasi.'
    case 'invalid_otp':
      return 'Kode verifikasi salah.'
    case 'otp_expired':
      return 'Kode verifikasi sudah kedaluwarsa. Minta kode baru.'
    case 'otp_attempts_exceeded':
      return 'Terlalu banyak percobaan. Minta kode baru.'
    case 'rate_limited':
      return 'Terlalu banyak percobaan. Tunggu sebentar lalu coba lagi.'
    case 'not_found':
      return 'Data tidak ditemukan.'
    case 'forbidden':
      return err.message || 'Kamu tidak punya akses untuk ini.'
    case 'room_busy':
    case 'no_copies':
    case 'loan_limit':
    case 'not_loanable':
    case 'conflict':
    case 'invite_expired':
      return err.message
    case 'profile_incomplete':
      return 'Lengkapi profil terlebih dahulu.'
    case 'unauthenticated':
    case 'session_revoked':
      return 'Sesi berakhir. Silakan masuk lagi.'
    default:
      return 'Terjadi kesalahan pada server. Coba lagi nanti.'
  }
}
