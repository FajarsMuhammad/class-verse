import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeAll, beforeEach, expect, it, vi } from 'vitest'
import BuildingModules from './BuildingModules.vue'
import { moduleApi } from '@/api/modules'
import { quizApi } from '@/api/quizzes'
import { ApiError } from '@/api/client'
import type { Module, ModuleStep } from '@/api/types'

beforeAll(() => {
  HTMLDialogElement.prototype.showModal ??= function (this: HTMLDialogElement) { this.open = true }
  HTMLDialogElement.prototype.close ??= function (this: HTMLDialogElement) { this.open = false; this.dispatchEvent(new Event('close')) }
})
const step = (title: string, extra: Partial<ModuleStep> = {}): ModuleStep => ({
  id: title, kind: 'materi', title, body: 'Isi <b>tidak</b> dirender', file_id: null, file_kind: null, video_id: null, quiz_id: null,
  minutes: 5, reader_position: 0, done: false, ...extra,
})
const mod = (steps: ModuleStep[], extra: Partial<Module> = {}): Module => ({
  id: 'm1', building_key: 'sains', level: 'SD', title: 'Tata Surya', description: 'Planet', published: true,
  updated_at: '2026-01-01T00:00:00Z', steps, ...extra,
})
const global = { stubs: {
  teleport: true,
  FileReader: { name: 'FileReader', props: ['file'], template: '<div class="reader">{{ file.name }}:{{ file.kind }}:{{ file.position }}</div>' },
  QuizPlayer: { name: 'QuizPlayer', props: ['quizId', 'title', 'stepId'], emits: ['finished', 'close'], template: '<div class="player">{{ quizId }}:{{ title }}</div>' },
} }
const mountIt = (props = {}) => mount(BuildingModules, { props: { orgId: 'o', buildingKey: 'sains', ...props }, global })
beforeEach(() => { vi.spyOn(quizApi, 'list').mockResolvedValue({ items: [] }) })
afterEach(() => vi.restoreAllMocks())

it('shows module cards per level with progress and refetches when the level changes', async () => {
  const list = vi.spyOn(moduleApi, 'list').mockResolvedValue({ items: [mod([step('A', { done: true }), step('B', { minutes: 7 })])] })
  const w = mountIt()
  await flushPromises()
  expect(w.text()).toContain('Modul Menara Sains')
  expect(list).toHaveBeenLastCalledWith('o', { building_key: 'sains', level: 'SD', limit: 20, offset: 0 })
  expect(w.get('.card').text()).toContain('2 langkah · 12 menit · 1 dari 2 selesai')
  expect(w.get('.card .state').text()).toBe('Sedang dipelajari')
  list.mockResolvedValue({ items: [] })
  await w.findAll('.levels button')[2]!.trigger('click')
  await flushPromises()
  expect(list).toHaveBeenLastCalledWith('o', { building_key: 'sains', level: 'SMA', limit: 20, offset: 0 })
  expect(w.text()).toContain('Belum ada modul untuk jenjang ini.')
})

it('opens a module in the same popup, renders text as plain text, and goes back to the catalog', async () => {
  vi.spyOn(moduleApi, 'list').mockResolvedValue({ items: [mod([step('A', { body: '<img src=x onerror=alert(1)>Halo' })])] })
  const w = mountIt()
  await flushPromises()
  await w.get('.card button').trigger('click')
  expect(w.findAll('dialog')).toHaveLength(1)
  expect(w.find('img').exists()).toBe(false)
  expect(w.get('.plain').text()).toBe('<img src=x onerror=alert(1)>Halo')
  await w.findAll('button').find((b) => b.text() === 'Semua modul')!.trigger('click')
  expect(w.find('.card').exists()).toBe(true)
})

it('embeds video privately and opens files in the reader at their saved position', async () => {
  vi.spyOn(moduleApi, 'list').mockResolvedValue({ items: [mod([
    step('Tonton', { kind: 'video', video_id: 'dQw4w9WgXcQ', body: '' }),
    step('Dengar', { kind: 'berkas', file_id: 'f1', file_kind: 'audio', reader_position: 42, body: '' }),
  ])] })
  const w = mountIt()
  await flushPromises()
  await w.get('.card button').trigger('click')
  const frame = w.get('iframe')
  expect(frame.attributes('src')).toBe('https://www.youtube-nocookie.com/embed/dQw4w9WgXcQ?rel=0')
  expect(frame.attributes('sandbox')).toContain('allow-scripts')
  await w.get('.next').trigger('click')
  await w.get('.file button').trigger('click')
  expect(w.get('.reader').text()).toBe('Dengar:audio:42')
})

it('steps are all open; completion saves out of order, can be undone, and a refused write keeps the state', async () => {
  vi.spyOn(moduleApi, 'list').mockResolvedValue({ items: [mod([step('A'), step('B')])] })
  const done = vi.spyOn(moduleApi, 'setDone').mockResolvedValueOnce().mockRejectedValueOnce(new ApiError(500, { code: 'internal_error', message: 'x' }))
  const w = mountIt()
  await flushPromises()
  await w.get('.card button').trigger('click')
  expect(w.findAll('nav .step').every((b) => b.attributes('disabled') === undefined)).toBe(true)
  await w.findAll('nav .step')[1]!.trigger('click')
  await w.get('.mark').trigger('click')
  await flushPromises()
  expect(done).toHaveBeenLastCalledWith('o', 'm1', 'B', true)
  expect(w.text()).toContain('1 dari 2 selesai')
  expect(w.emitted('changed')).toHaveLength(1)
  await w.findAll('button').find((b) => b.text() === 'Batal selesai')!.trigger('click')
  await flushPromises()
  expect(done).toHaveBeenLastCalledWith('o', 'm1', 'B', false)
  expect(w.find('.done-badge').exists()).toBe(true)
  expect(w.get('[role=alert]').text()).toContain('Terjadi kesalahan')
})

it('a quiz step completes only when the quiz is passed; building quizzes show the best score', async () => {
  vi.spyOn(moduleApi, 'list').mockResolvedValue({ items: [mod([step('Kuis kecil', { kind: 'kuis', quiz_id: 'q1', body: '' })])] })
  vi.mocked(quizApi.list).mockResolvedValue({ items: [
    { id: 'q2', building_key: 'sains', level: 'SD', title: 'Kuis Planet', description: '', pass_score: 60, published: true,
      updated_at: '2026-01-01T00:00:00Z', question_count: 10, best_score: 80, attempts: 2 },
  ] })
  const setDone = vi.spyOn(moduleApi, 'setDone')
  const w = mountIt()
  await flushPromises()
  expect(w.get('.quiz-card').text()).toContain('10 soal · nilai terbaik 80')
  await w.get('.card button').trigger('click')
  expect(w.find('.mark').exists()).toBe(false)
  await w.get('.quiz-start').trigger('click')
  expect(w.get('.player').text()).toBe('q1:Kuis kecil')
  w.getComponent({ name: 'QuizPlayer' }).vm.$emit('finished', { passed: false, quiz_id: 'q1', score: 40 })
  await flushPromises()
  expect(w.find('.done-badge').exists()).toBe(false)
  w.getComponent({ name: 'QuizPlayer' }).vm.$emit('finished', { passed: true, quiz_id: 'q1', score: 90 })
  await flushPromises()
  expect(w.find('.done-badge').exists()).toBe(true)
  expect(setDone).not.toHaveBeenCalled()
})

it('a deep link opens the module and step directly', async () => {
  vi.spyOn(moduleApi, 'list').mockResolvedValue({ items: [mod([step('A'), step('B')])] })
  const w = mountIt({ moduleId: 'm1', stepId: 'B' })
  await flushPromises()
  expect(w.get('[aria-current=step]').text()).toContain('B')
})
