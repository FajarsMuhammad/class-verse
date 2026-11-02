import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import RoomForm from './RoomForm.vue'
import { roomApi } from '@/api/rooms'
import type { Building } from '@/realtime/mapdata'

vi.mock('@/api/rooms', () => ({
  roomApi: { create: vi.fn<typeof roomApi.create>(), update: vi.fn<typeof roomApi.update>() },
}))
const buildings = [{ key: 'kelas-a', name: 'Kelas A' }] as Building[]
const room = {
  id: 'room-1',
  name: 'Biologi',
  building_key: 'kelas-a',
  map_key: 'classroom' as const,
  scheduled_at: null,
  capacity: 40,
  people: 0,
  live: false,
  org_id: 'org-1',
  created_at: '',
}

beforeEach(() => {
  vi.clearAllMocks()
  HTMLDialogElement.prototype.showModal = vi.fn<() => void>()
  HTMLDialogElement.prototype.close = vi.fn<() => void>()
})

describe('RoomForm', () => {
  it('creates immediate room with server-owned capacity omitted', async () => {
    vi.mocked(roomApi.create).mockResolvedValue(room)
    const wrapper = mount(RoomForm, { props: { orgId: 'org-1', buildings } })
    await wrapper.get('#room-name').setValue('  Biologi  ')
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(roomApi.create).toHaveBeenCalledWith('org-1', {
      name: 'Biologi',
      building_key: 'kelas-a',
      map_key: 'classroom',
      scheduled_at: null,
    })
    expect(wrapper.emitted('saved')?.[0]).toEqual([room])
  })

  it('keeps map and building immutable on edit', async () => {
    vi.mocked(roomApi.update).mockResolvedValue(room)
    const wrapper = mount(RoomForm, { props: { orgId: 'org-1', buildings, room } })
    expect(wrapper.find('#room-building').exists()).toBe(false)
    expect(wrapper.find('#room-map').exists()).toBe(false)
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(roomApi.update).toHaveBeenCalledWith('room-1', { name: 'Biologi', scheduled_at: null })
  })

  it('rejects short names without submitting', async () => {
    const wrapper = mount(RoomForm, { props: { orgId: 'org-1', buildings } })
    await wrapper.get('#room-name').setValue('ab')
    await wrapper.get('form').trigger('submit')
    expect(roomApi.create).not.toHaveBeenCalled()
    expect(wrapper.get('[role="alert"]').text()).toContain('3–60')
  })

  it('converts local schedule to UTC and retains form on API failure', async () => {
    vi.mocked(roomApi.create).mockRejectedValue(new Error('network'))
    const wrapper = mount(RoomForm, { props: { orgId: 'org-1', buildings } })
    await wrapper.get('#room-name').setValue('Kimia')
    await wrapper.get('[type="checkbox"]').setValue(true)
    await wrapper.get('#room-schedule').setValue('2027-01-02T09:30')
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(roomApi.create).toHaveBeenCalledWith(
      'org-1',
      expect.objectContaining({ scheduled_at: new Date('2027-01-02T09:30').toISOString() }),
    )
    expect(wrapper.find('[role="alert"]').exists()).toBe(true)
    expect(wrapper.emitted('saved')).toBeUndefined()
  })
})
