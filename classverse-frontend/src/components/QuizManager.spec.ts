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

it('lists drafts with their questions and marks the correct option', async () => {
  const list = vi.spyOn(quizApi, 'list').mockResolvedValue({ items: [quiz()] })
  const w = mount(QuizManager, { props: { orgId: 'o' } })
  await flushPromises()
  expect(list).toHaveBeenCalledWith('o', { drafts: true })
  expect(w.text()).toContain('Menara Sains · SD · 1 soal · lulus 60')
  expect(w.text()).toContain('Draf')
  expect(w.find('.correct').text()).toContain('Jupiter')
  // Staff-only buildings cannot hold quizzes.
  await w.find('.head .btn-primary').trigger('click')
  expect(w.findAll('select')[0]!.findAll('option').map((o) => o.text())).toEqual(['Menara Sains'])
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

it('publishes only with questions and explains the rule', async () => {
  vi.spyOn(quizApi, 'list').mockResolvedValue({ items: [quiz({ questions: [], question_count: 0 })] })
  const update = vi.spyOn(quizApi, 'update').mockRejectedValue(
    new ApiError(400, { code: 'validation_failed', message: 'x', fields: { questions: 'Kuis terbit perlu minimal 1 soal' } }))
  const w = mount(QuizManager, { props: { orgId: 'o' } })
  await flushPromises()
  await w.find('.quiz-head .btn-secondary').trigger('click')
  await w.find('input[type=checkbox]').setValue(true)
  await w.find('form').trigger('submit')
  await flushPromises()
  expect(update).toHaveBeenCalledWith('o', 'q1', expect.objectContaining({ published: true }))
  expect(w.text()).toContain('Kuis terbit perlu minimal 1 soal')
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
  await w.find('[aria-label="Turunkan Satu"]').trigger('click')
  await flushPromises()
  expect(reorder).toHaveBeenCalledWith('o', 'q1', ['x2', 'x1'])
  await w.find('[aria-label="Hapus soal Dua"]').trigger('click')
  await flushPromises()
  expect(del).toHaveBeenCalledWith('o', 'q1', 'x2')
})
