import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import {
  api,
  ApiError,
  errorMessage,
  getAccessToken,
  onTokenChange,
  refreshAccessToken,
  setAccessToken,
} from '../client'

type Handler = (url: string, init: RequestInit) => Response | Promise<Response>

function stubFetch(handler: Handler) {
  const fn = vi.fn<(url: string, init?: RequestInit) => Promise<Response>>((url, init = {}) =>
    Promise.resolve(handler(url, init)),
  )
  vi.stubGlobal('fetch', fn)
  return fn
}

function authHeader(init: RequestInit | undefined): string | undefined {
  return (init?.headers as Record<string, string> | undefined)?.Authorization
}

const unauthenticated = () =>
  Response.json({ error: { code: 'unauthenticated', message: 'x' } }, { status: 401 })

it('explains why a room cannot be archived instead of reporting a server failure', () => {
  const message = 'Ruang masih dipakai. Arsipkan setelah kosong 5 menit.'
  expect(errorMessage(new ApiError(409, { code: 'room_busy', message }))).toBe(message)
})

describe('api client', () => {
  beforeEach(() => setAccessToken('old'))
  afterEach(() => {
    vi.unstubAllGlobals()
    setAccessToken(null)
  })

  it('sends the bearer token', async () => {
    const fetch = stubFetch(() => Response.json({ ok: true }))
    await api('/api/me')
    expect(authHeader(fetch.mock.calls[0]![1])).toBe('Bearer old')
  })

  it('refreshes once on 401 and retries with the new token', async () => {
    const fetch = stubFetch((url, init) => {
      if (url === '/api/auth/refresh') return Response.json({ access_token: 'new' })
      return authHeader(init) === 'Bearer new' ? Response.json({ id: 'u1' }) : unauthenticated()
    })

    await expect(api<{ id: string }>('/api/me')).resolves.toEqual({ id: 'u1' })
    expect(fetch.mock.calls.map((c) => c[0])).toEqual(['/api/me', '/api/auth/refresh', '/api/me'])
    expect(getAccessToken()).toBe('new')
  })

  it('logs out when the refresh fails', async () => {
    stubFetch((url) =>
      url === '/api/auth/refresh'
        ? Response.json({ error: { code: 'session_revoked', message: 'x' } }, { status: 401 })
        : unauthenticated(),
    )
    const seen: (string | null)[] = []
    const off = onTokenChange((t) => seen.push(t))

    await expect(api('/api/me')).rejects.toBeInstanceOf(ApiError)
    expect(getAccessToken()).toBeNull()
    expect(seen).toContain(null)
    off()
  })

  it('shares one refresh between concurrent callers', async () => {
    const fetch = stubFetch(() => Response.json({ access_token: 'new' }))
    const [a, b] = await Promise.all([refreshAccessToken(), refreshAccessToken()])
    expect(a).toBe('new')
    expect(b).toBe('new')
    expect(fetch).toHaveBeenCalledTimes(1)
  })

  it('maps API errors with fields', async () => {
    stubFetch(() =>
      Response.json(
        {
          error: {
            code: 'validation_failed',
            message: 'x',
            fields: { email: 'Email tidak valid' },
          },
        },
        { status: 400 },
      ),
    )
    const err = await api('/api/auth/register', { method: 'POST', body: {}, auth: false }).catch(
      (e: unknown) => e,
    )
    expect(err).toBeInstanceOf(ApiError)
    expect((err as ApiError).fields.email).toBe('Email tidak valid')
  })
})
