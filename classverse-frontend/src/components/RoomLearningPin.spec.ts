import { flushPromises, mount } from '@vue/test-utils'
import { ref, shallowRef } from 'vue'
import { beforeEach, expect, it, vi } from 'vitest'
import RoomLearningPin from './RoomLearningPin.vue'
import type { RoomHandle } from '@/realtime/useRoom'
import type { WelcomeMsg } from '@/realtime/messages'
import { moduleApi } from '@/api/modules'

vi.mock('./BuildingModules.vue', () => ({ default: { props: ['moduleId', 'stepId'], template: '<div class="opened">{{ moduleId }} {{ stepId }}</div>' } }))
vi.mock('@/api/modules', () => ({ moduleApi: { list: vi.fn<typeof moduleApi.list>() } }))
const modules = [
  { id: 'm', title: 'Tata Surya', level: 'SD', published: true, steps: [{ id: 's', title: 'Planet' }] },
  { id: 'd', title: 'Draf', level: 'SD', published: false, steps: [{ id: 'x', title: 'X' }] },
] as unknown as Awaited<ReturnType<typeof moduleApi.list>>['items']
const makeRoom = (pinned: boolean) => ({
  pin: ref(pinned ? { module_id: 'm', step_id: 's', title: 'Tata Surya', building_key: 'sains', level: 'SD' } : null),
  status: ref('open'), current: shallowRef({ room_id: 'r', map_key: 'classroom' } as WelcomeMsg),
  pinLearning: vi.fn<NonNullable<RoomHandle['pinLearning']>>(),
}) as unknown as RoomHandle & { pinLearning: ReturnType<typeof vi.fn> }
beforeEach(() => { vi.mocked(moduleApi.list).mockResolvedValue({ items: modules }) })

it('participant opens the pin only on explicit action and sees no staff controls', async () => {
  const room = makeRoom(true)
  const w = mount(RoomLearningPin, { props: { room, orgId: 'org', staff: false } })
  expect(w.find('.opened').exists()).toBe(false)
  expect(w.text()).not.toMatch(/Pilih modul|Ganti sematan|Lepas sematan/)
  await w.get('.open').trigger('click'); await flushPromises()
  expect(w.get('.opened').text()).toBe('m s')
})

it('staff manage session status here but choose modules in module details', async () => {
  const room = makeRoom(false)
  const w = mount(RoomLearningPin, { props: { room, orgId: 'org', staff: true } })
  expect(w.text()).toContain('Materi sesi ini')
  await w.get('.choose').trigger('click')
  expect(w.emitted('change')).toHaveLength(1)
  expect(w.find('select').exists()).toBe(false)
  room.pin!.value = { module_id: 'm', step_id: 's', title: 'Tata Surya', building_key: 'sains', level: 'SD' }
  await flushPromises()
  expect(w.text()).toContain('Planet')
  expect(w.get('.open').text()).toBe('Buka materi')
  await w.findAll('button').find((b) => b.text() === 'Ganti')!.trigger('click')
  expect(w.emitted('change')).toHaveLength(2)
  await w.findAll('button').find((b) => b.text() === 'Lepas')!.trigger('click')
  expect(room.pinLearning).toHaveBeenLastCalledWith('')
})
