import { flushPromises, mount } from '@vue/test-utils'
import { ref, shallowRef } from 'vue'
import { expect, it, vi } from 'vitest'
import RoomLearningPin from './RoomLearningPin.vue'
import type { RoomHandle } from '@/realtime/useRoom'
import type { WelcomeMsg } from '@/realtime/messages'
vi.mock('./BuildingModules.vue', () => ({ default: { props: ['moduleId', 'stepId'], template: '<div class="opened">{{ moduleId }} {{ stepId }}</div>' } }))
it('opens pinned module only on explicit action and hides staff actions for students', async () => {
  const room = {
    pin: ref({ module_id: 'm', step_id: 's', title: 'Belajar', building_key: 'sains', level: 'SMA' }),
    status: ref('open'), current: shallowRef({ room_id: 'r', map_key: 'classroom' } as WelcomeMsg),
    pinLearning: vi.fn<NonNullable<RoomHandle['pinLearning']>>(),
  } as unknown as RoomHandle
  const w = mount(RoomLearningPin, { props: { room, orgId: 'org', staff: false } })
  expect(w.find('.opened').exists()).toBe(false)
  expect(w.text()).not.toContain('Sematkan modul')
  await w.get('button').trigger('click'); await flushPromises()
  expect(w.get('.opened').text()).toBe('m s')
  room.current.value = { room_id: 'campus', map_key: 'campus' } as WelcomeMsg
  await flushPromises()
  expect(w.find('.pin').exists()).toBe(false)
})
