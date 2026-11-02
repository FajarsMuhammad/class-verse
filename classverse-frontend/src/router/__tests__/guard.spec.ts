import { createPinia, setActivePinia } from 'pinia'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { RouteLocationNormalized } from 'vue-router'

import { setAccessToken } from '@/api/client'
import { authGuard } from '../index'

function route(name: string, meta: Record<string, boolean> = {}, fullPath = '/x') {
  return { name, meta, fullPath } as unknown as RouteLocationNormalized
}

const orgA = { id: 'a', name: 'SMA A', slug: 'sma-a', role: 'participant' }
const orgB = { id: 'b', name: 'SMA B', slug: 'sma-b', role: 'teacher' }

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

const home = route('home', { requiresAuth: true, requiresOrg: true }, '/')

describe('authGuard', () => {
  beforeEach(() => {
    localStorage.clear()
    setActivePinia(createPinia())
  })
  afterEach(() => {
    vi.unstubAllGlobals()
    setAccessToken(null)
  })

  it('sends guests to login', async () => {
    stubBackend(null)
    expect(await authGuard(home)).toEqual({ name: 'login', query: {} })
  })

  it('sends users without birth date to complete profile', async () => {
    stubBackend({ id: 'u', needs_profile: true, orgs: [] })
    expect(await authGuard(home)).toEqual({ name: 'complete-profile' })
  })

  it('sends logged-in users away from guest pages', async () => {
    stubBackend({ id: 'u', needs_profile: false, orgs: [orgA] })
    expect(await authGuard(route('login', { guestOnly: true }))).toEqual({ name: 'home' })
  })

  it('sends users without organizations to start', async () => {
    stubBackend({ id: 'u', needs_profile: false, orgs: [] })
    expect(await authGuard(home)).toEqual({ name: 'start' })
  })

  it('lets a single-org user straight in', async () => {
    stubBackend({ id: 'u', needs_profile: false, orgs: [orgA] })
    expect(await authGuard(home)).toBe(true)
  })

  it('asks multi-org users to pick, unless one was stored', async () => {
    stubBackend({ id: 'u', needs_profile: false, orgs: [orgA, orgB] })
    expect(await authGuard(home)).toEqual({ name: 'select-org' })

    localStorage.setItem('kv-active-org', 'b')
    setActivePinia(createPinia())
    setAccessToken(null)
    expect(await authGuard(home)).toBe(true)
  })

  it('keeps participants out of teacher pages', async () => {
    stubBackend({ id: 'u', needs_profile: false, orgs: [orgA] })
    const manage = route('manage-rooms', {
      requiresAuth: true,
      requiresOrg: true,
      teacherOnly: true,
    })
    expect(await authGuard(manage)).toEqual({ name: 'home' })
  })
})
