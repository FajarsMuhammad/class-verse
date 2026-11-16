import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeAll, expect, it, vi } from 'vitest'
import { ApiError } from '@/api/client'
import { quizApi } from '@/api/quizzes'
import type { QuizAttempt } from '@/api/types'
import QuizPlayer from './QuizPlayer.vue'

beforeAll(() => {
  // jsdom lacks <dialog> methods.
  HTMLDialogElement.prototype.showModal ??= function (this: HTMLDialogElement) { this.open = true }
  HTMLDialogElement.prototype.close ??= function (this: HTMLDialogElement) { this.open = false; this.dispatchEvent(new Event('close')) }
})
afterEach(() => vi.restoreAllMocks())

const open = (answers = [-1, -1]): QuizAttempt => ({
  id: 'a1', quiz_id: 'q1', pass_score: 60, finished: false, correct: null, score: null, passed: null, answers,
  questions: [
    { prompt: 'Planet terbesar?', options: ['Bumi', 'Jupiter', 'Mars', 'Venus'], answer: null, explanation: null },
    { prompt: '<b>Planet</b> merah?', options: ['Bumi', 'Jupiter', 'Mars', 'Venus'], answer: null, explanation: null },
  ],
})

const mountPlayer = () => mount(QuizPlayer, { attachTo: document.body, props: { orgId: 'o', quizId: 'q1', title: 'Kuis Planet' } })

it('shows one question at a time with four shape-labelled options and no key', async () => {
  vi.spyOn(quizApi, 'start').mockResolvedValue(open())
  const w = mountPlayer()
  await flushPromises()
  expect(w.text()).toContain('Soal 1 dari 2')
  const options = w.findAll('.option')
  expect(options).toHaveLength(4)
  // Not colour-only: every option carries a shape name for screen readers.
  expect(options.map((o) => o.attributes('aria-label'))).toEqual([
    'Lingkaran: Bumi', 'Segitiga: Jupiter', 'Kotak: Mars', 'Belah ketupat: Venus',
  ])
  expect(w.find('.explanation').exists()).toBe(false)
  w.unmount()
})

it('saves the choice on the server and only then marks it selected', async () => {
  vi.spyOn(quizApi, 'start').mockResolvedValue(open())
  const answer = vi.spyOn(quizApi, 'answer').mockResolvedValue()
  const w = mountPlayer()
  await flushPromises()
  await w.findAll('.option')[1]!.trigger('click')
  await flushPromises()
  expect(answer).toHaveBeenCalledWith('o', 'q1', 'a1', 0, 1)
  expect(w.findAll('.option')[1]!.attributes('aria-pressed')).toBe('true')
  w.unmount()
})

it('keeps the previous choice and shows the error when saving fails', async () => {
  vi.spyOn(quizApi, 'start').mockResolvedValue(open([2, -1]))
  vi.spyOn(quizApi, 'answer').mockRejectedValue(new ApiError(500, { code: 'internal', message: 'Gagal' }))
  const w = mountPlayer()
  await flushPromises()
  // Resuming opens the first unanswered question (2); go back to the answered one.
  expect(w.text()).toContain('Soal 2 dari 2')
  await w.find('.prev').trigger('click')
  await w.findAll('.option')[0]!.trigger('click')
  await flushPromises()
  expect(w.findAll('.option')[2]!.attributes('aria-pressed')).toBe('true')
  expect(w.find('[role=alert]').exists()).toBe(true)
  w.unmount()
})

it('confirms unanswered questions, then shows score, pass state and the key with explanation', async () => {
  vi.spyOn(quizApi, 'start').mockResolvedValue(open([1, -1]))
  const finish = vi.spyOn(quizApi, 'finish').mockResolvedValue({
    ...open([1, -1]), finished: true, correct: 1, score: 50, passed: false,
    questions: open().questions.map((q, i) => ({ ...q, answer: i === 0 ? 1 : 2, explanation: `Alasan ${i + 1}` })),
  })
  const confirm = vi.spyOn(window, 'confirm').mockReturnValue(false)
  const w = mountPlayer()
  await flushPromises()
  await w.find('.finish').trigger('click')
  expect(confirm).toHaveBeenCalledWith(expect.stringContaining('1 soal belum dijawab'))
  expect(finish).not.toHaveBeenCalled()
  confirm.mockReturnValue(true)
  await w.find('.finish').trigger('click')
  await flushPromises()
  expect(w.text()).toContain('Nilai 50')
  expect(w.text()).toContain('Belum lulus')
  expect(w.text()).toContain('nilai lulus 60')
  expect(w.text()).toContain('Alasan 2')
  // Plain text only: question HTML is never rendered.
  expect(w.html()).not.toContain('<b>Planet</b>')
  expect(w.emitted('finished')?.[0]).toEqual([expect.objectContaining({ score: 50 })])
  w.unmount()
})

it('navigates freely between questions', async () => {
  vi.spyOn(quizApi, 'start').mockResolvedValue(open())
  const w = mountPlayer()
  await flushPromises()
  expect(w.find('.prev').attributes('disabled')).toBeDefined()
  await w.find('.next').trigger('click')
  expect(w.text()).toContain('Soal 2 dari 2')
  expect(w.find('.next').attributes('disabled')).toBeDefined()
  w.unmount()
})
