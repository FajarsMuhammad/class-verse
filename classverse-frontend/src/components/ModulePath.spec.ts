import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, expect, it, vi } from 'vitest'
import ModulePath from './ModulePath.vue'
import { moduleApi } from '@/api/modules'
import type { Module, ModuleStep } from '@/api/types'

const step = (title: string, extra: Partial<ModuleStep> = {}): ModuleStep => ({
  id: title, kind: 'materi', title, body: 'Isi <b>tidak</b> dirender', file_id: null, file_kind: null, video_id: null,
  minutes: 5, reader_position: 0, ...extra,
})
const mod = (steps: ModuleStep[]): Module => ({
  id: 'm1', building_key: 'sains', level: 'SD', title: 'Tata Surya', description: 'Planet', published: true,
  updated_at: '2026-01-01T00:00:00Z', steps,
})

afterEach(() => vi.restoreAllMocks())

const stubs = {
  FileReader: { props: ['file'], template: '<div class="reader">{{ file.name }}:{{ file.kind }}:{{ file.position }}</div>' },
  StepViewer: { props: ['step'], template: '<div class="viewer">{{ step.title }}</div>' },
}

it('lists a building path per level, sums minutes, and refetches when the level changes', async () => {
  const list = vi.spyOn(moduleApi, 'list').mockResolvedValue({ items: [mod([step('A'), step('B', { minutes: 7 })])] })
  const w = mount(ModulePath, { props: { orgId: 'o', buildingKey: 'sains', buildingName: 'Menara Sains' }, global: { stubs } })
  await flushPromises()
  expect(list).toHaveBeenLastCalledWith('o', { building_key: 'sains', level: 'SD' })
  expect(w.text()).toContain('2 langkah · 12 menit')
  list.mockResolvedValue({ items: [] })
  await w.findAll('.level')[2]!.trigger('click')
  await flushPromises()
  expect(list).toHaveBeenLastCalledWith('o', { building_key: 'sains', level: 'SMA' })
  expect(w.text()).toContain('Belum ada modul SMA di Menara Sains.')
})

it('opens files in the reader with their kind and position, and text/video in the viewer', async () => {
  vi.spyOn(moduleApi, 'list').mockResolvedValue({ items: [mod([
    step('Dengar', { kind: 'berkas', file_id: 'f1', file_kind: 'audio', reader_position: 42 }),
    step('Tonton', { kind: 'video', video_id: 'dQw4w9WgXcQ' }),
  ])] })
  const w = mount(ModulePath, { props: { orgId: 'o', buildingKey: 'sains', buildingName: 'Menara Sains' }, global: { stubs } })
  await flushPromises()
  const buttons = w.findAll('.open')
  expect(buttons[0]!.text()).toContain('Dengarkan')
  await buttons[0]!.trigger('click')
  expect(w.get('.reader').text()).toBe('Dengar:audio:42')
  await buttons[1]!.trigger('click')
  expect(w.get('.viewer').text()).toBe('Tonton')
})

it('shows a retry when loading fails', async () => {
  const { ApiError } = await import('@/api/client')
  const list = vi.spyOn(moduleApi, 'list').mockRejectedValueOnce(new ApiError(500, { code: 'internal_error', message: 'x' }))
    .mockResolvedValue({ items: [] })
  const w = mount(ModulePath, { props: { orgId: 'o', buildingKey: 'sains', buildingName: 'Menara Sains' }, global: { stubs } })
  await flushPromises()
  await w.get('[role=alert] button').trigger('click')
  await flushPromises()
  expect(list).toHaveBeenCalledTimes(2)
  expect(w.find('[role=alert]').exists()).toBe(false)
})
