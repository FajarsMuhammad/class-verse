import { createPinia, setActivePinia } from 'pinia'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { RouteLocationNormalized } from 'vue-router'

import { setAccessToken } from '@/api/client'
import { authGuard } from '../index'

function route(name: string, meta: Record<string, boolean> = {}, fullPath = '/x') {
  return { name, meta, fullPath } as unknown as RouteLocationNormalized
}

function stubBackend(me: object | null) {
  vi.stubGlobal(
    'fetch',
    vi.fn<(url: string) => Promise<Response>>((url) => {
      if (url === '/api/auth/refresh') {
        return Promise.resolve(
          me
            ? Response.json({ access_token: 't' })
            : Response.json({ error: { code: 'unauthenticated', message: 'x' } }, { status: 401 }),
        )
      }
      return Promise.resolve(Response.json(me))
    }),
  )
}

describe('authGuard', () => {
  beforeEach(() => setActivePinia(createPinia()))
  afterEach(() => {
    vi.unstubAllGlobals()
    setAccessToken(null)
  })

  it('sends guests to login', async () => {
    stubBackend(null)
    expect(await authGuard(route('home', { requiresAuth: true }, '/'))).toEqual({
      name: 'login',
      query: {},
    })
  })

  it('sends users without birth date to complete profile', async () => {
    stubBackend({ id: 'u', needs_profile: true })
    expect(await authGuard(route('home', { requiresAuth: true }, '/'))).toEqual({
      name: 'complete-profile',
    })
  })

  it('sends logged-in users away from guest pages', async () => {
    stubBackend({ id: 'u', needs_profile: false })
    expect(await authGuard(route('login', { guestOnly: true }))).toEqual({ name: 'home' })
  })

  it('lets complete users through', async () => {
    stubBackend({ id: 'u', needs_profile: false })
    expect(await authGuard(route('home', { requiresAuth: true }, '/'))).toBe(true)
  })
})
