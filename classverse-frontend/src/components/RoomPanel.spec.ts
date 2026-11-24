import { flushPromises, mount } from '@vue/test-utils'
import { expect, it, vi } from 'vitest'
import RoomPanel from './RoomPanel.vue'
import { roomApi } from '@/api/rooms'
import type { Room } from '@/api/types'

vi.mock('@/realtime/mapdata', () => ({ fetchMap: vi.fn<() => Promise<unknown>>().mockResolvedValue({ buildings: [
  { key: 'ruang-guru', name: 'Ruang Guru' }, { key: 'kelas-a', name: 'Rumah Angka' },
] }) }))
const room = (id: string, building: string, name: string) =>
  ({ id, building_key: building, name, map_key: 'classroom', capacity: 40, live: false, people: 0, scheduled_at: null }) as unknown as Room

it('groups rooms by building in map order and searches on the server', async () => {
  vi.useFakeTimers()
  const list = vi.spyOn(roomApi, 'list').mockResolvedValue({ items: [room('a', 'kelas-a', 'Kelas 1A'), room('g', 'ruang-guru', 'Ruang Guru')], next_cursor: null, counts: { all: 2, live: 1, archived: 3 } })
  const w = mount(RoomPanel, { props: { orgId: 'o' }, global: { stubs: { RouterLink: { template: '<a><slot /></a>' }, RoomForm: true } } })
  await flushPromises()
  expect(w.findAll('.group-title').map(h => h.text())).toEqual(['Ruang Guru', 'Rumah Angka'])
  await w.get('input[type=search]').setValue('1A')
  vi.advanceTimersByTime(300)
  await flushPromises()
  expect(list).toHaveBeenLastCalledWith('o', { cursor: undefined, building: '', q: '1A', counts: true, status: undefined })
  vi.useRealTimers()
})

it('switches status tabs on the server and offers Kontrol only for the live room the user is in', async () => {
  const live = { ...room('l', 'kelas-a', 'Kelas Live'), live: true, people: 3 }
  const list = vi.spyOn(roomApi, 'list').mockResolvedValue({ items: [live], next_cursor: null, counts: { all: 2, live: 1, archived: 3 } })
  const w = mount(RoomPanel, { props: { orgId: 'o', currentRoomId: 'l' }, global: { stubs: { RouterLink: { template: '<a><slot /></a>' }, RoomForm: true } } })
  await flushPromises()
  expect(w.findAll('[role=tab]').map(t => t.text())).toEqual(['Semua 2', 'Live 1', 'Arsip 3'])
  expect(w.findAll('button').some(b => b.text().startsWith('Kontrol'))).toBe(true)
  await w.findAll('[role=tab]')[1]!.trigger('click')
  await flushPromises()
  expect(list).toHaveBeenLastCalledWith('o', expect.objectContaining({ status: 'live', counts: true }))
  await w.findAll('[role=tab]')[2]!.trigger('click')
  await flushPromises()
  expect(list).toHaveBeenLastCalledWith('o', expect.objectContaining({ status: 'archived' }))
  expect(w.text()).toContain('Diarsipkan')
  expect(w.get('.badge').text()).toBe('Arsip')
  expect(w.findAll('button').some(b => b.text().startsWith('Kontrol'))).toBe(false)
  const away = mount(RoomPanel, { props: { orgId: 'o', currentRoomId: 'other' }, global: { stubs: { RouterLink: { template: '<a><slot /></a>' }, RoomForm: true } } })
  await flushPromises()
  expect(away.findAll('button').some(b => b.text().startsWith('Kontrol'))).toBe(false)
})
