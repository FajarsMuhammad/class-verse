import { flushPromises, mount } from '@vue/test-utils'
import { ref, shallowRef } from 'vue'
import type { RoomHandle } from '@/realtime/useRoom'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import ModuleManager from './ModuleManager.vue'
import { ApiError } from '@/api/client'
import { fileApi } from '@/api/files'
import { moduleApi } from '@/api/modules'
import { quizApi } from '@/api/quizzes'
import type { Module, ModuleStep } from '@/api/types'

vi.mock('@/realtime/mapdata', () => ({
  fetchMap: () => Promise.resolve({ buildings: [
    { key: 'sains', name: 'Menara Sains', staffOnly: false },
    { key: 'ruang-guru', name: 'Ruang Guru', staffOnly: true },
  ] }),
}))

const step = (id: string): ModuleStep => ({
  id, kind: 'materi', title: `Langkah ${id}`, body: 'x', file_id: null, file_kind: null, video_id: null, quiz_id: null, minutes: 5, reader_position: 0, done: false,
})
const draft: Module = {
  id: 'm1', draft_version:1, building_key: 'sains', level: 'SD', title: 'Tata Surya', description: '', published: false,
  updated_at: '2026-01-01T00:00:00Z', steps: [step('a'), step('b'), step('c')],
}

beforeEach(() => {
  vi.spyOn(moduleApi, 'history').mockResolvedValue({items:[]})
  vi.spyOn(moduleApi, 'list').mockResolvedValue({ items: [draft] })
  vi.spyOn(fileApi, 'list').mockResolvedValue({ items: [], used_bytes: 0, quota_bytes: 1 })
})
afterEach(() => vi.restoreAllMocks())

it('creates reading sections with sequential draft versions', async () => {
  vi.spyOn(moduleApi, 'create').mockResolvedValue({ ...draft, steps: [] })
  const add = vi.spyOn(moduleApi, 'addStep').mockResolvedValue(step('new'))
  const w = mount(ModuleManager, { props: { orgId: 'o' } })
  await flushPromises()
  await w.get('.head .btn-primary').trigger('click')
  await w.get('input[value="reading"]').setValue(true)
  await w.get('input[maxlength="120"]').setValue('Materi baru')
  await w.get('form').trigger('submit')
  await flushPromises()
  expect(add).toHaveBeenCalledTimes(4)
  expect(add.mock.calls.map(call => call[2].title)).toEqual(['Pendahuluan', 'Materi', 'Contoh', 'Rangkuman'])
  expect(add.mock.calls.map(call => call[2].expected_version)).toEqual([1, 2, 3, 4])
  w.unmount()
})

it('creates a module only in learning buildings and shows drafts', async () => {
  const create = vi.spyOn(moduleApi, 'create').mockResolvedValue(draft)
  const w = mount(ModuleManager, { props: { orgId: 'o' } })
  await flushPromises()
  expect(w.text()).toContain('Draf')
  expect(w.text()).toContain('Menara Sains · SD · 3 langkah')
  expect(w.get('[role=tablist]').text()).toContain('Draf 1')
  await w.get('.head .btn-primary').trigger('click')
  expect(w.findAll('[role=radiogroup][aria-label=Gedung] label').map((o) => o.text())).toEqual(['Menara Sains'])
  await w.get('input[value="empty"]').setValue(true)
  await w.get('input[maxlength="120"]').setValue('Gerak Planet')
  await w.get('form').trigger('submit')
  await flushPromises()
  expect(create).toHaveBeenCalledWith('o', { building_key: 'sains', level: 'SD', title: 'Gerak Planet', description: '', published: false })
  expect(w.get('[role=status]').text()).toBe('Gerak Planet disimpan sebagai draf.')
})

it('moves a step with the full new order and blocks moving past the ends', async () => {
  const reorder = vi.spyOn(moduleApi, 'reorder').mockResolvedValue()
  const w = mount(ModuleManager, { props: { orgId: 'o' } })
  await flushPromises()
  await w.get('.catalog-row button').trigger('click')
  expect(w.get('[aria-label="Naikkan Langkah a"]').attributes('disabled')).toBeDefined()
  expect(w.get('[aria-label="Turunkan Langkah c"]').attributes('disabled')).toBeDefined()
  await w.get('[aria-label="Turunkan Langkah a"]').trigger('click')
  await flushPromises()
  expect(reorder).toHaveBeenCalledWith('o', 'm1', ['b', 'a', 'c'],1)
})

it('adds a video step and shows server field errors next to the field', async () => {
  const addStep = vi.spyOn(moduleApi, 'addStep').mockRejectedValue(
    new ApiError(400, { code: 'validation_failed', message: 'x', fields: { video_id: 'Tempel tautan YouTube yang valid' } }))
  const w = mount(ModuleManager, { props: { orgId: 'o' } })
  await flushPromises()
  await w.get('.catalog-row button').trigger('click')
  await w.get('.add').trigger('click')
  await w.get('form select').setValue('video')
  await w.findAll('form input')[0]!.setValue('Video planet')
  await w.get('form input[inputmode=url]').setValue('https://vimeo.com/1')
  await w.get('form').trigger('submit')
  await flushPromises()
  expect(addStep).toHaveBeenCalledWith('o', 'm1', expect.objectContaining({ kind: 'video', video_id: 'https://vimeo.com/1', file_id: null }))
  expect(w.text()).toContain('Tempel tautan YouTube yang valid')
  expect(w.get('form input[inputmode=url]').attributes('aria-invalid')).toBe('true')
})

it('adds a quiz step pointing at a published quiz of the same building', async () => {
  const quizzes = vi.spyOn(quizApi, 'list').mockResolvedValue({ items: [
    { id: 'q1', building_key: 'sains', level: 'SD', title: 'Kuis Planet', description: '', pass_score: 60, published: true,
      updated_at: '2026-01-01T00:00:00Z', question_count: 3, best_score: null, attempts: 0 },
  ] })
  const addStep = vi.spyOn(moduleApi, 'addStep').mockResolvedValue({ ...step('k'), kind: 'kuis', quiz_id: 'q1' })
  const w = mount(ModuleManager, { props: { orgId: 'o' } })
  await flushPromises()
  await w.get('.catalog-row button').trigger('click')
  await w.get('.add').trigger('click')
  await w.get('form select').setValue('kuis')
  await flushPromises()
  expect(quizzes).toHaveBeenCalledWith('o', { building_key: 'sains', offset: 0, limit: 20 })
  await w.findAll('form input')[0]!.setValue('Kuis kecil')
  await w.get('form select[aria-label="Kuis"]').setValue('q1')
  await w.get('form').trigger('submit')
  await flushPromises()
  expect(addStep).toHaveBeenCalledWith('o', 'm1', expect.objectContaining({ kind: 'kuis', quiz_id: 'q1', file_id: null }))
})

 it('keeps the catalog compact and filters by level before opening one item', async () => {
  vi.spyOn(moduleApi, 'list').mockResolvedValue({ items: [draft, { ...draft, id: 'other', level: 'SMP', title: 'Jenjang lain' }] })
  const w = mount(ModuleManager, { props: { orgId: 'o' } })
  await flushPromises()
  expect(w.findAll('.catalog-row')).toHaveLength(2)
  expect(w.find('.steps').exists()).toBe(false)
  await w.get('[aria-label="Filter jenjang"]').setValue('SMP')
  expect(w.findAll('.catalog-row')).toHaveLength(1)
  expect(w.get('.catalog-row').text()).toContain('Jenjang lain')
  await w.get('.catalog-row button').trigger('click')
  expect(w.find('.catalog').exists()).toBe(false)
  expect(w.get('.editor-title').text()).toBe('Jenjang lain')
  await w.get('[data-action="back"]').trigger('click')
  expect(w.get('[aria-label="Filter jenjang"]').element).toHaveProperty('value', 'SMP')
 })

it('starts the first content form after creating a filtered draft', async () => {
  const created = { ...draft, id: 'new-m', level: 'SMA' as const, steps: [] }
  vi.spyOn(moduleApi, 'list').mockResolvedValueOnce({ items: [] }).mockResolvedValue({ items: [created] })
  const create = vi.spyOn(moduleApi, 'create').mockResolvedValue(created)
  const w = mount(ModuleManager, { props: { orgId: 'o' } })
  await flushPromises()
  await w.get('[aria-label="Filter jenjang"]').setValue('SMA')
  await w.get('.head button').trigger('click')
  await w.get('input[value="empty"]').setValue(true)
  await w.get('input[maxlength="120"]').setValue('Baru')
  await w.get('form').trigger('submit')
  await flushPromises()
  expect(create).toHaveBeenCalledWith('o', expect.objectContaining({ level: 'SMA', published: false }))
  expect(w.find('[aria-label="Isi materi rich text"]').exists()).toBe(true)
  expect(w.find('.catalog').exists()).toBe(false)
})

it('pins only a published module from its detail, with a named current-room target', async () => {
  vi.spyOn(moduleApi, 'list').mockResolvedValue({ items: [{ ...draft, published: true }] })
  const pinLearning = vi.fn<NonNullable<RoomHandle['pinLearning']>>()
  const room = { status: ref('open'), current: shallowRef({ room_id: 'r', map_key: 'classroom', room_name: 'Kelas IPA' }), pinLearning } as unknown as RoomHandle
  const w = mount(ModuleManager, { props: { orgId: 'o', room } })
  await flushPromises()
  expect(w.find('[data-action="pin"]').exists()).toBe(false)
  await w.get('[aria-label="Buka Tata Surya"]').trigger('click')
  expect(w.get('[data-action="pin"]').text()).toContain('Kelas IPA')
  await w.get('[data-action="pin"]').trigger('click')
  await w.get('.pin-form select').setValue('b')
  await w.get('.pin-form').trigger('submit')
  expect(pinLearning).toHaveBeenCalledWith('m1', 'b')
  expect(w.emitted('pinned')).toHaveLength(1)
  room.status.value = 'closed'
  await flushPromises()
  expect(w.get('[data-action="pin"]').attributes('disabled')).toBeDefined()
})

it('explains that a room is required on campus and never offers drafts for pinning', async () => {
  const room = { status: ref('open'), current: shallowRef({ room_id: 'c', map_key: 'campus' }), pinLearning: vi.fn<NonNullable<RoomHandle['pinLearning']>>() } as unknown as RoomHandle
  const w = mount(ModuleManager, { props: { orgId: 'o', room } })
  await flushPromises()
  await w.get('.catalog-row button').trigger('click')
  expect(w.find('[data-action="pin"]').exists()).toBe(false)
  expect(w.text()).toContain('Masuk ruang untuk menyematkan modul')
})

it('filters by status tab and search, and offers Tugaskan only for published modules', async () => {
  vi.spyOn(moduleApi, 'list').mockImplementation(async (_org, query) => ({ items: query?.archived ? [] : [draft, { ...draft, id: 'p', title: 'Bilangan', published: true }] }))
  const w = mount(ModuleManager, { props: { orgId: 'o' } })
  await flushPromises()
  expect(w.findAll('[role=tab]').map((t) => t.text())).toEqual(['Semua 2', 'Terbit 1', 'Draf 1', 'Arsip 0'])
  expect(w.find('[aria-label="Tugaskan Tata Surya"]').exists()).toBe(false)
  await w.get('[aria-label="Tugaskan Bilangan"]').trigger('click')
  expect(w.emitted('assign')).toEqual([['p']])
  await w.findAll('[role=tab]')[2]!.trigger('click')
  expect(w.findAll('.catalog-row')).toHaveLength(1)
  expect(w.get('.catalog-row').text()).toContain('Lanjutkan edit')
  await w.findAll('[role=tab]')[0]!.trigger('click')
  await w.get('input[type=search]').setValue('bil')
  expect(w.findAll('.catalog-row').map((r) => r.text())).toEqual([expect.stringContaining('Bilangan')])
})
