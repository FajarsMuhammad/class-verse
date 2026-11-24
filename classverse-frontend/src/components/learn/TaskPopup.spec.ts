import { flushPromises, mount } from '@vue/test-utils'
import { beforeAll, beforeEach, expect, it, vi } from 'vitest'
import TaskPopup from './TaskPopup.vue'
import { assignmentApi } from '@/api/assignments'

vi.mock('@/api/assignments', () => ({ deadline: () => 'Tanpa tenggat', assignmentApi: {
  list: vi.fn<typeof assignmentApi.list>(), steps: vi.fn<typeof assignmentApi.steps>(), complete: vi.fn<typeof assignmentApi.complete>(),
} }))
beforeAll(() => {
  HTMLDialogElement.prototype.showModal ??= function (this: HTMLDialogElement) { this.open = true }
  HTMLDialogElement.prototype.close ??= function (this: HTMLDialogElement) { this.open = false; this.dispatchEvent(new Event('close')) }
})
const task = (id: string, extra = {}) => ({ id, title: `Tugas ${id}`, class_name: '1A', year_label: '2026/2027', building_key: 'angka', level: 'SD' as const, due_at: null, cancelled: false, done: 0, total: 2, ...extra })
const step = (i: number, extra = {}) => ({ id: `s${i}`, kind: 'materi' as const, title: `Langkah ${i}`, body: `Isi ${i}`, file_id: null, file_kind: null, video_id: null, quiz_id: null, minutes: 5, done: false, unlocked: i === 0, ...extra })
const global = { stubs: { teleport: true, FileReader: true, QuizPlayer: { name: 'QuizPlayer', props: ['assignmentStepId'], emits: ['finished', 'close'], template: '<div class="quiz" />' } } }
beforeEach(() => {
  vi.resetAllMocks()
  vi.mocked(assignmentApi.list).mockResolvedValue({ items: [task('a', { done: 1 }), task('b', { done: 2 }), task('c', { cancelled: true })] })
  vi.mocked(assignmentApi.steps).mockResolvedValue({ items: [step(0), step(1)] })
})

it('filters the list by status and opens a task at its first open step; locked steps cannot be selected', async () => {
  const w = mount(TaskPopup, { props: { orgId: 'o' }, global })
  await flushPromises()
  expect(w.text()).toContain('1 aktif · 1 selesai · 1 dibatalkan')
  await w.findAll('.tabs button')[3]!.trigger('click')
  expect(w.findAll('.card')).toHaveLength(1)
  expect(w.get('.card').classes()).toContain('cancelled')
  await w.findAll('.tabs button')[0]!.trigger('click')
  await w.get('.card button').trigger('click')
  await flushPromises()
  expect(assignmentApi.steps).toHaveBeenCalledWith('o', 'a')
  expect(w.get('[aria-current=step]').text()).toContain('Langkah 0')
  expect(w.text()).toContain('Isi 0')
  const navSteps = w.findAll('nav .step')
  expect(navSteps[1]!.attributes('disabled')).toBeDefined()
  expect(w.get('.next').attributes('disabled')).toBeDefined()
  expect(assignmentApi.complete).not.toHaveBeenCalled()
})

it('completes a step through the server, then the next step unlocks; failures keep the step open', async () => {
  vi.mocked(assignmentApi.complete).mockRejectedValueOnce(new Error('x')).mockResolvedValue()
  const w = mount(TaskPopup, { props: { orgId: 'o', open: task('a') }, global })
  await flushPromises()
  await w.get('.mark').trigger('click')
  await flushPromises()
  expect(w.find('[role=alert]').exists()).toBe(true)
  expect(w.find('.done-badge').exists()).toBe(false)
  vi.mocked(assignmentApi.steps).mockResolvedValue({ items: [step(0, { done: true }), step(1, { unlocked: true })] })
  await w.get('.mark').trigger('click')
  await flushPromises()
  expect(assignmentApi.complete).toHaveBeenLastCalledWith('o', 'a', 's0')
  expect(w.find('.done-badge').exists()).toBe(true)
  expect(w.text()).toContain('1 dari 2 selesai')
  await w.get('.next').trigger('click')
  expect(w.get('[aria-current=step]').text()).toContain('Langkah 1')
  expect(w.emitted('changed')).toHaveLength(1)
})

it('a quiz step has no manual completion and starts the quiz through the task', async () => {
  vi.mocked(assignmentApi.steps).mockResolvedValue({ items: [step(0, { kind: 'kuis', quiz_id: 'q', body: '' })] })
  const w = mount(TaskPopup, { props: { orgId: 'o', open: task('a') }, global })
  await flushPromises()
  expect(w.find('.mark').exists()).toBe(false)
  await w.get('.quiz-start').trigger('click')
  expect(w.getComponent({ name: 'QuizPlayer' }).props('assignmentStepId')).toBe('s0')
  w.getComponent({ name: 'QuizPlayer' }).vm.$emit('finished')
  await flushPromises()
  expect(assignmentApi.steps).toHaveBeenCalledTimes(2)
})

it('a cancelled task is read-only', async () => {
  const w = mount(TaskPopup, { props: { orgId: 'o', open: task('c', { cancelled: true }) }, global })
  await flushPromises()
  expect(w.text()).toContain('Tugas dibatalkan')
  expect(w.find('.mark').exists()).toBe(false)
})

it('searches on the server, debounced', async () => {
  vi.useFakeTimers()
  const w = mount(TaskPopup, { props: { orgId: 'o' }, global })
  await flushPromises()
  await w.get('input[type=search]').setValue('pecahan')
  vi.advanceTimersByTime(300)
  vi.useRealTimers()
  await flushPromises()
  expect(assignmentApi.list).toHaveBeenLastCalledWith('o', 0, 'pecahan')
})
