import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, expect, it, vi } from 'vitest'
import DoorRooms from './DoorRooms.vue'
import { roomApi } from '@/api/rooms'
import type { Building } from '@/realtime/mapdata'

vi.mock('@/api/rooms', () => ({ roomApi: { list: vi.fn<typeof roomApi.list>() } }))
beforeEach(() => {
  vi.mocked(roomApi.list).mockReset()
  vi.mocked(roomApi.list).mockResolvedValue({
    items: [
      {
        id: 'r1',
        org_id: 'o1',
        name: 'Biologi',
        building_key: 'kelas-a',
        map_key: 'classroom',
        capacity: 40,
        people: 0,
        live: false,
        scheduled_at: null,
        created_at: '',
        all_chat_enabled: true,
      },
    ],
    next_cursor: null,
  })
})

it('shows rooms before reaching door but permits travel only near door', async () => {
  const wrapper = mount(DoorRooms, {
    props: {
      orgId: 'o1',
      building: { key: 'kelas-a', name: 'Kelas A' } as Building,
      disabled: true,
      atDoor: false,
    },
  })
  await flushPromises()
  expect(roomApi.list).toHaveBeenCalledWith('o1', { building: 'kelas-a', cursor: undefined })
  expect(wrapper.text()).toContain('Biologi')
  expect(wrapper.text()).toContain('Jalan ke pintu')
  expect(wrapper.get('button').attributes('disabled')).toBeDefined()
  await wrapper.setProps({ disabled: false, atDoor: true })
  await wrapper.get('button').trigger('click')
  expect(wrapper.emitted('travel')).toEqual([['r1']])
})
