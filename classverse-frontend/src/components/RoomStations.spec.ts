import { flushPromises, mount } from '@vue/test-utils'
import { expect, it, vi } from 'vitest'
import { moduleApi } from '@/api/modules'
import RoomStations from './RoomStations.vue'

const global = { stubs: { ModulePath: true, RouterLink: { template: '<a><slot /></a>' } } }
it('offers science stations and the real module catalog, without simulated pupils or scores', async () => {
  const w = mount(RoomStations, { props: { orgId: 'o', mapKey: 'science', station: 'tatasurya', teacher: false }, global })
  expect(w.text()).toContain('Model Tata Surya')
  expect(w.text()).toContain('simulasi interaktif belum tersedia')
  await w.get('.btn-primary').trigger('click')
  expect(w.findComponent({ name: 'ModulePath' }).props('buildingKey')).toBe('sains')
  await w.findAll('.station-tab')[2]!.trigger('click')
  expect(w.emitted('select')?.[0]).toEqual(['kimia'])
})
it('staff tools link to existing management, recording and grading are not fake actions', async () => {
  const get = vi.spyOn(moduleApi, 'list').mockResolvedValue({ items: [{ id: 'm', title: 'Materi nyata', building_key: 'sains', level: 'SD', description: '', published: false, updated_at: '2026-11-13T00:00:00Z', steps: [] }] })
  const w = mount(RoomStations, { props: { orgId: 'o', mapKey: 'staff', station: 'kerja', teacher: true }, global })
  await flushPromises()
  expect(get).toHaveBeenCalledWith('o', { drafts: true, limit: 20, offset: 0 })
  expect(w.text()).toContain('Materi nyata')
  expect(w.text()).toContain('Draf')
  expect(w.text()).toContain('Buat dan kelola modul')
  expect(w.find('a').exists()).toBe(true)
  expect(w.text()).not.toContain('82')
  expect(w.text()).not.toContain('Bu Sari')
})
