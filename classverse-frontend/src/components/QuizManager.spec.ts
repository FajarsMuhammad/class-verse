import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import { ApiError } from '@/api/client'
import { quizApi } from '@/api/quizzes'
import type { Quiz } from '@/api/types'
import QuizManager from './QuizManager.vue'

vi.mock('@/realtime/mapdata', () => ({
  fetchMap: () => Promise.resolve({ buildings: [{ key: 'sains', name: 'Menara Sains', staffOnly: false }, { key: 'ruang-guru', name: 'Ruang Guru', staffOnly: true }] }),
}))

const quiz = (extra: Partial<Quiz> = {}): Quiz => ({
  id: 'q1', building_key: 'sains', level: 'SD', title: 'Kuis Planet', description: '', pass_score: 60, published: false,
  updated_at: '2026-01-01T00:00:00Z', question_count: 1, best_score: null, attempts: 0,
  questions: [{ id: 'x1', prompt: 'Planet terbesar?', options: ['Bumi', 'Jupiter', 'Mars', 'Venus'], answer: 1, explanation: '' }],
  ...extra,
})

beforeEach(() => vi.spyOn(window, 'confirm').mockReturnValue(true))
afterEach(() => vi.restoreAllMocks())

it('lists drafts compactly and only shows options in preview', async () => {
  const list = vi.spyOn(quizApi, 'list').mockResolvedValue({ items: [quiz()] })
  const w = mount(QuizManager, { props: { orgId: 'o' } })
  await flushPromises()
  expect(list).toHaveBeenCalledWith('o', { drafts: true })
  expect(w.text()).toContain('Menara Sains · SD · 1 soal · lulus 60')
  expect(w.text()).toContain('Draf')
  expect(w.find('.correct').exists()).toBe(false)
  await w.get('.catalog-row button').trigger('click')
  await w.get('.editor-actions button').trigger('click')
  expect(w.get('.correct').text()).toContain('Jupiter')
  await w.get('[data-action="back"]').trigger('click')
  // Staff-only buildings cannot hold quizzes.
  await w.find('.head .btn-primary').trigger('click')
  expect(w.findAll('form select')[0]!.findAll('option').map((o) => o.text())).toEqual(['Menara Sains'])
})

it('creates a quiz with the default pass score of 60', async () => {
  vi.spyOn(quizApi, 'list').mockResolvedValue({ items: [] })
  const create = vi.spyOn(quizApi, 'create').mockResolvedValue(quiz())
  const w = mount(QuizManager, { props: { orgId: 'o' } })
  await flushPromises()
  await w.find('.head .btn-primary').trigger('click')
  await w.find('input[maxlength="120"]').setValue('Kuis Bintang')
  expect((w.find('input[type=number]').element as HTMLInputElement).value).toBe('60')
  await w.find('form').trigger('submit')
  await flushPromises()
  expect(create).toHaveBeenCalledWith('o', expect.objectContaining({ title: 'Kuis Bintang', pass_score: 60, published: false }))
})

it('adds a question with four options and the chosen answer, and shows field errors', async () => {
  vi.spyOn(quizApi, 'list').mockResolvedValue({ items: [quiz({ questions: [], question_count: 0 })] })
  const add = vi.spyOn(quizApi, 'addQuestion')
    .mockRejectedValueOnce(new ApiError(400, { code: 'validation_failed', message: 'x', fields: { options: 'Setiap pilihan 1–200 karakter dan tidak boleh sama' } }))
    .mockResolvedValueOnce(quiz().questions![0]!)
  const w = mount(QuizManager, { props: { orgId: 'o' } })
  await flushPromises()
  await w.get('.catalog-row button').trigger('click')
  await w.find('.add').trigger('click')
  await w.find('textarea').setValue('Planet merah?')
  const opts = w.findAll('.option-row input[type=text]')
  expect(opts).toHaveLength(4)
  for (const [i, text] of ['Bumi', 'Jupiter', 'Mars', 'Venus'].entries()) await opts[i]!.setValue(text)
  await w.findAll('.option-row input[type=radio]')[2]!.setValue(true)
  await w.find('.question-form').trigger('submit')
  await flushPromises()
  expect(w.text()).toContain('tidak boleh sama')
  await w.find('.question-form').trigger('submit')
  await flushPromises()
  expect(add).toHaveBeenLastCalledWith('o', 'q1', { prompt: 'Planet merah?', options: ['Bumi', 'Jupiter', 'Mars', 'Venus'], answer: 2, explanation: '' })
})

it('keeps empty quizzes as drafts until questions exist', async () => {
  vi.spyOn(quizApi, 'list').mockResolvedValue({ items: [quiz({ questions: [], question_count: 0 })] })
  const update = vi.spyOn(quizApi, 'update')
  const w = mount(QuizManager, { props: { orgId: 'o' } })
  await flushPromises()
  await w.get('.catalog-row button').trigger('click')
  expect(w.get('.editor-actions .btn-primary').attributes('disabled')).toBeDefined()
  expect(w.text()).toContain('Tambahkan minimal satu soal')
  expect(update).not.toHaveBeenCalled()
})

it('publishes from the editor and preserves server validation errors', async () => {
  vi.spyOn(quizApi, 'list').mockResolvedValue({ items: [quiz()] })
  const update = vi.spyOn(quizApi, 'update').mockRejectedValue(new ApiError(400, { code: 'validation_failed', message: 'Kuis belum valid' }))
  const w = mount(QuizManager, { props: { orgId: 'o' } })
  await flushPromises()
  await w.get('.catalog-row button').trigger('click')
  await w.get('.editor-actions .btn-primary').trigger('click')
  await flushPromises()
  expect(update).toHaveBeenCalledWith('o', 'q1', expect.objectContaining({ published: true }))
  expect(w.get('[role=alert]').text()).toContain('Periksa kembali isian')
})

it('deletes and reorders questions after confirmation', async () => {
  const two = quiz({ question_count: 2, questions: [
    { id: 'x1', prompt: 'Satu', options: ['a', 'b', 'c', 'd'], answer: 0, explanation: '' },
    { id: 'x2', prompt: 'Dua', options: ['a', 'b', 'c', 'd'], answer: 0, explanation: '' },
  ] })
  vi.spyOn(quizApi, 'list').mockResolvedValue({ items: [two] })
  const reorder = vi.spyOn(quizApi, 'reorder').mockResolvedValue()
  const del = vi.spyOn(quizApi, 'deleteQuestion').mockResolvedValue()
  const w = mount(QuizManager, { props: { orgId: 'o' } })
  await flushPromises()
  await w.get('.catalog-row button').trigger('click')
  await w.find('[aria-label="Turunkan Satu"]').trigger('click')
  await flushPromises()
  expect(reorder).toHaveBeenCalledWith('o', 'q1', ['x2', 'x1'])
  await w.find('[aria-label="Hapus soal Dua"]').trigger('click')
  await flushPromises()
  expect(del).toHaveBeenCalledWith('o', 'q1', 'x2')
})

 it('keeps the catalog compact and filters by level before opening one item', async () => {
  vi.spyOn(quizApi, 'list').mockResolvedValue({ items: [quiz(), { ...quiz(), id: 'other', level: 'SMP', title: 'Jenjang lain' }] })
  const w = mount(QuizManager, { props: { orgId: 'o' } })
  await flushPromises()
  expect(w.findAll('.catalog-row')).toHaveLength(2)
  expect(w.find('.questions').exists()).toBe(false)
  await w.get('[aria-label="Filter jenjang"]').setValue('SMP')
  expect(w.findAll('.catalog-row')).toHaveLength(1)
  expect(w.get('.catalog-row').text()).toContain('Jenjang lain')
  await w.get('.catalog-row button').trigger('click')
  expect(w.find('.catalog').exists()).toBe(false)
  expect(w.get('.editor-title').text()).toBe('Jenjang lain')
  await w.get('[data-action="back"]').trigger('click')
  expect(w.get('[aria-label="Filter jenjang"]').element).toHaveProperty('value', 'SMP')
 })

it('opens the first question form after creating a draft at the filtered level', async () => {
  const created = quiz({ id: 'created', title: 'Baru', level: 'SMP', questions: [], question_count: 0 })
  vi.spyOn(quizApi, 'list').mockResolvedValueOnce({ items: [] }).mockResolvedValue({ items: [created] })
  const create = vi.spyOn(quizApi, 'create').mockResolvedValue(created)
  const w = mount(QuizManager, { props: { orgId: 'o' } })
  await flushPromises()
  await w.get('[aria-label="Filter jenjang"]').setValue('SMP')
  await w.get('.head button').trigger('click')
  await w.get('input[maxlength="120"]').setValue('Baru')
  expect(w.find('input[type=checkbox]').exists()).toBe(false)
  await w.get('form').trigger('submit')
  await flushPromises()
  expect(create).toHaveBeenCalledWith('o', expect.objectContaining({ level: 'SMP', published: false }))
  expect(w.get('.editor-title').text()).toBe('Baru')
  expect(w.find('.question-form').exists()).toBe(true)
  expect(w.find('.catalog').exists()).toBe(false)
  vi.mocked(window.confirm).mockReturnValue(false)
  await w.get('[data-action="back"]').trigger('click')
  expect(w.find('.question-form').exists()).toBe(true)
})
