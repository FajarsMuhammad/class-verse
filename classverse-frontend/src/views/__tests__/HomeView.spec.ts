import { afterEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import HomeView from '../HomeView.vue'

function mockFetch(impl: () => Promise<Response>) {
  vi.stubGlobal('fetch', vi.fn(impl))
}

describe('HomeView', () => {
  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it.each([
    {
      name: 'backend healthy',
      fetch: () => Promise.resolve(Response.json({ status: 'ok' })),
      want: 'Server terhubung',
    },
    {
      name: 'backend returns error status',
      fetch: () => Promise.resolve(new Response('', { status: 502 })),
      want: 'Server tidak terhubung',
    },
    {
      name: 'backend unreachable',
      fetch: () => Promise.reject(new TypeError('Failed to fetch')),
      want: 'Server tidak terhubung',
    },
  ])('shows status when $name', async ({ fetch, want }) => {
    mockFetch(fetch)

    const wrapper = mount(HomeView)
    await flushPromises()

    expect(wrapper.get('[data-testid="backend-status"]').text()).toBe(want)
  })
})
