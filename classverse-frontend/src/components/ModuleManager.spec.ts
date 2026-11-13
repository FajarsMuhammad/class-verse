import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import ModuleManager from './ModuleManager.vue'
import { ApiError } from '@/api/client'
import { fileApi } from '@/api/files'
import { moduleApi } from '@/api/modules'
import type { Module, ModuleStep } from '@/api/types'

vi.mock('@/realtime/mapdata', () => ({
  fetchMap: () => Promise.resolve({ buildings: [
    { key: 'sains', name: 'Menara Sains', staffOnly: false },
    { key: 'ruang-guru', name: 'Ruang Guru', staffOnly: true },
  ] }),
}))

const step = (id: string): ModuleStep => ({
  id, kind: 'materi', title: `Langkah ${id}`, body: 'x', file_id: null, file_kind: null, video_id: null, minutes: 5, reader_position: 0,
})
const draft: Module = {
  id: 'm1', building_key: 'sains', level: 'SD', title: 'Tata Surya', description: '', published: false,
  updated_at: '2026-01-01T00:00:00Z', steps: [step('a'), step('b'), step('c')],
}

beforeEach(() => {
  vi.spyOn(moduleApi, 'list').mockResolvedValue({ items: [draft] })
  vi.spyOn(fileApi, 'list').mockResolvedValue({ items: [], used_bytes: 0, quota_bytes: 1 })
})
afterEach(() => vi.restoreAllMocks())

it('creates a module only in learning buildings and shows drafts', async () => {
  const create = vi.spyOn(moduleApi, 'create').mockResolvedValue(draft)
  const w = mount(ModuleManager, { props: { orgId: 'o' } })
  await flushPromises()
  expect(w.text()).toContain('Draf')
  expect(w.text()).toContain('Menara Sains · SD · 3 langkah')
  await w.get('.head .btn-primary').trigger('click')
  const options = w.findAll('form select')[0]!.findAll('option').map((o) => o.text())
  expect(options).toEqual(['Menara Sains'])
  await w.get('form input').setValue('Gerak Planet')
  await w.get('form').trigger('submit')
  await flushPromises()
  expect(create).toHaveBeenCalledWith('o', { building_key: 'sains', level: 'SD', title: 'Gerak Planet', description: '', published: false })
  expect(w.get('[role=status]').text()).toBe('Gerak Planet disimpan sebagai draf.')
})

it('moves a step with the full new order and blocks moving past the ends', async () => {
  const reorder = vi.spyOn(moduleApi, 'reorder').mockResolvedValue()
  const w = mount(ModuleManager, { props: { orgId: 'o' } })
  await flushPromises()
  expect(w.get('[aria-label="Naikkan Langkah a"]').attributes('disabled')).toBeDefined()
  expect(w.get('[aria-label="Turunkan Langkah c"]').attributes('disabled')).toBeDefined()
  await w.get('[aria-label="Turunkan Langkah a"]').trigger('click')
  await flushPromises()
  expect(reorder).toHaveBeenCalledWith('o', 'm1', ['b', 'a', 'c'])
})

it('adds a video step and shows server field errors next to the field', async () => {
  const addStep = vi.spyOn(moduleApi, 'addStep').mockRejectedValue(
    new ApiError(400, { code: 'validation_failed', message: 'x', fields: { video_id: 'Tempel tautan YouTube yang valid' } }))
  const w = mount(ModuleManager, { props: { orgId: 'o' } })
  await flushPromises()
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
