import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import ModulePath from './ModulePath.vue'
import { moduleApi } from '@/api/modules'
import { quizApi } from '@/api/quizzes'
import type { Module, ModuleStep } from '@/api/types'

const step = (title: string, extra: Partial<ModuleStep> = {}): ModuleStep => ({
  id: title, kind: 'materi', title, body: 'Isi <b>tidak</b> dirender', file_id: null, file_kind: null, video_id: null, quiz_id: null,
  minutes: 5, reader_position: 0, done: false, ...extra,
})
const mod = (steps: ModuleStep[]): Module => ({
  id: 'm1', building_key: 'sains', level: 'SD', title: 'Tata Surya', description: 'Planet', published: true,
  updated_at: '2026-01-01T00:00:00Z', steps,
})

// Quizzes load next to modules; default to none so module-only tests stay focused.
beforeEach(() => { vi.spyOn(quizApi, 'list').mockResolvedValue({ items: [] }) })
afterEach(() => vi.restoreAllMocks())

const stubs = {
  FileReader: { props: ['file'], template: '<div class="reader">{{ file.name }}:{{ file.kind }}:{{ file.position }}</div>' },
  StepViewer: { props: ['step'], template: '<div class="viewer">{{ step.title }}</div>' },
  QuizPlayer: { props: ['quizId', 'title'], emits: ['finished', 'close'], template: '<div class="player">{{ quizId }}:{{ title }}</div>' },
}

it('lists a building path per level, sums minutes, and refetches when the level changes', async () => {
  const list = vi.spyOn(moduleApi, 'list').mockResolvedValue({ items: [mod([step('A'), step('B', { minutes: 7 })])] })
  const w = mount(ModulePath, { props: { orgId: 'o', buildingKey: 'sains', buildingName: 'Menara Sains' }, global: { stubs } })
  await flushPromises()
  expect(list).toHaveBeenLastCalledWith('o', { building_key: 'sains', level: 'SD', limit: 20, offset: 0 })
  expect(w.text()).toContain('2 langkah · 12 menit')
  list.mockResolvedValue({ items: [] })
  await w.findAll('.level')[2]!.trigger('click')
  await flushPromises()
  expect(list).toHaveBeenLastCalledWith('o', { building_key: 'sains', level: 'SMA', limit: 20, offset: 0 })
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

it('saves completion out of order and retains local state on a refused write', async () => {
  const { ApiError } = await import('@/api/client')
  vi.spyOn(moduleApi, 'list').mockResolvedValue({ items: [mod([step('A'), step('B')])] })
  const done = vi.spyOn(moduleApi, 'setDone').mockResolvedValueOnce().mockRejectedValueOnce(
    new ApiError(500, { code: 'internal_error', message: 'x' }))
  const w = mount(ModulePath, { props: { orgId: 'o', buildingKey: 'sains', buildingName: 'Menara Sains' }, global: { stubs } })
  await flushPromises()
  const buttons = w.findAll('.complete')
  await buttons[1]!.trigger('click')
  await flushPromises()
  expect(done).toHaveBeenLastCalledWith('o', 'm1', 'B', true)
  expect(w.text()).toContain('1 dari 2 selesai')
  expect(buttons[1]!.attributes('aria-pressed')).toBe('true')
  expect(w.emitted('changed')).toHaveLength(1)
  await buttons[1]!.trigger('click')
  await flushPromises()
  expect(done).toHaveBeenLastCalledWith('o', 'm1', 'B', false)
  expect(buttons[1]!.attributes('aria-pressed')).toBe('true')
  expect(w.get('[role=alert]').text()).toContain('Terjadi kesalahan')
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

it('opens a quiz step in the player, has no manual completion, and marks it done after a pass', async () => {
  vi.spyOn(moduleApi, 'list').mockResolvedValue({ items: [mod([step('Kuis kecil', { kind: 'kuis', quiz_id: 'q1' })])] })
  const done = vi.spyOn(moduleApi, 'setDone')
  const w = mount(ModulePath, { props: { orgId: 'o', buildingKey: 'sains', buildingName: 'Menara Sains' }, global: { stubs } })
  await flushPromises()
  expect(w.find('.complete').exists()).toBe(false)
  expect(w.text()).toContain('Selesai otomatis saat lulus')
  await w.find('.open').trigger('click')
  expect(w.find('.player').text()).toBe('q1:Kuis kecil')
  w.findComponent(stubs.QuizPlayer).vm.$emit('finished', { passed: false })
  await flushPromises()
  expect(w.find('.step.done').exists()).toBe(false)
  w.findComponent(stubs.QuizPlayer).vm.$emit('finished', { passed: true })
  await flushPromises()
  expect(w.find('.step.done').exists()).toBe(true)
  expect(done).not.toHaveBeenCalled()
})

it('lists building quizzes with best score and opens them', async () => {
  vi.spyOn(moduleApi, 'list').mockResolvedValue({ items: [] })
  const quizzes = vi.spyOn(quizApi, 'list').mockResolvedValue({ items: [
    { id: 'q1', building_key: 'sains', level: 'SD', title: 'Kuis Planet', description: '', pass_score: 60, published: true,
      updated_at: '2026-01-01T00:00:00Z', question_count: 10, best_score: 80, attempts: 2 },
    { id: 'q2', building_key: 'sains', level: 'SD', title: 'Kuis Bintang', description: '', pass_score: 70, published: true,
      updated_at: '2026-01-01T00:00:00Z', question_count: 5, best_score: null, attempts: 0 },
  ] })
  const w = mount(ModulePath, { props: { orgId: 'o', buildingKey: 'sains', buildingName: 'Menara Sains' }, global: { stubs } })
  await flushPromises()
  expect(quizzes).toHaveBeenLastCalledWith('o', { building_key: 'sains', level: 'SD', limit: 20, offset: 0 })
  const cards = w.findAll('.quiz-card')
  expect(cards[0]!.text()).toContain('10 soal · nilai terbaik 80')
  expect(cards[1]!.text()).toContain('5 soal · belum dicoba')
  await cards[1]!.find('button').trigger('click')
  expect(w.find('.player').text()).toBe('q2:Kuis Bintang')
})
