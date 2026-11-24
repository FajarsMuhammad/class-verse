import { afterEach, describe, expect, it, vi } from 'vitest'
import { guestClient } from './guests'
import { getAccessToken, setAccessToken } from './client'

afterEach(() => { vi.unstubAllGlobals(); setAccessToken(null) })
describe('guestClient isolation', () => {
  it('keeps guest bearer separate from account auth and uses stable retry key in body', async () => {
    setAccessToken('account-only')
    const fetch = vi.fn<typeof globalThis.fetch>().mockResolvedValue({ ok: true, json: async () => ({ token: 'guest-only', status: 'pending' }) } as Response)
    vi.stubGlobal('fetch', fetch)
    const guest = guestClient()
    await guest.request('CODE', 'Guest Name', 'retry-secret')
    await guest.status()
    expect(getAccessToken()).toBe('account-only')
    expect(fetch.mock.calls[0]![1]).toMatchObject({ credentials: 'omit', cache: 'no-store' })
    expect(fetch.mock.calls[0]![1]!.body).toContain('retry-secret')
    expect(fetch.mock.calls[0]![0]).not.toContain('retry-secret')
    expect((fetch.mock.calls[1]![1]!.headers as Record<string, string>).Authorization).toBe('Bearer guest-only')
    guest.clear()
    expect(await guest.getToken()).toBeNull()
  })
  it('never calls account refresh when guest token expires', async () => {
    const fetch = vi.fn<typeof globalThis.fetch>().mockResolvedValue({ ok: false, status: 401, json: async () => ({ error: { code: 'unauthenticated', message: 'Expired' } }) } as Response)
    vi.stubGlobal('fetch', fetch)
    await expect(guestClient().status()).rejects.toMatchObject({ status: 401 })
    expect(fetch).toHaveBeenCalledTimes(1)
    expect(fetch.mock.calls[0]![0]).toBe('/api/guest/me')
  })
})
