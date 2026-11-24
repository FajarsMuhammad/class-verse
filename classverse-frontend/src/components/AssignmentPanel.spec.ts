import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, expect, it, vi } from 'vitest'
import AssignmentPanel from './AssignmentPanel.vue'
import { assignmentApi } from '@/api/assignments'

vi.mock('@/api/assignments', () => ({ deadline: () => 'Tanpa tenggat', assignmentApi: { list: vi.fn<typeof assignmentApi.list>() } }))
const task = (id: string, extra = {}) => ({ id, title: `Tugas ${id}`, class_name: '1A', year_label: '2026/2027', building_key: 'angka', level: 'SD' as const, due_at: null, cancelled: false, done: 0, total: 3, ...extra })
const global = { stubs: { TaskPopup: { name: 'TaskPopup', props: ['open'], emits: ['close'], template: '<div class="popup" />' } } }
beforeEach(() => vi.resetAllMocks())

it('lists at most three active tasks; cancelled and finished ones stay in "Lihat semua"', async () => {
  vi.mocked(assignmentApi.list).mockResolvedValue({ items: [
    task('a', { done: 1 }), task('b'), task('c'), task('d'), task('x', { cancelled: true }), task('y', { done: 3 }),
  ] })
  const w = mount(AssignmentPanel, { props: { orgId: 'o' }, global })
  await flushPromises()
  expect(w.get('.count').text()).toBe('4 aktif')
  const cards = w.findAll('.card')
  expect(cards).toHaveLength(3)
  expect(cards[0]!.text()).toContain('Matematika · 1A')
  expect(cards[0]!.text()).toContain('1/3')
  expect(cards[0]!.get('button').text()).toContain('Lanjutkan')
  expect(cards[1]!.get('button').text()).toContain('Mulai')
  expect(w.text()).not.toContain('Tugas x')
})

it('opens the task popup on a card or on "Lihat semua" and reloads when it closes', async () => {
  vi.mocked(assignmentApi.list).mockResolvedValue({ items: [task('a')] })
  const w = mount(AssignmentPanel, { props: { orgId: 'o' }, global })
  await flushPromises()
  await w.get('.card button').trigger('click')
  expect(w.getComponent({ name: 'TaskPopup' }).props('open')).toMatchObject({ id: 'a' })
  w.getComponent({ name: 'TaskPopup' }).vm.$emit('close')
  await flushPromises()
  expect(w.find('.popup').exists()).toBe(false)
  expect(assignmentApi.list).toHaveBeenCalledTimes(2)
  await w.get('.see-all').trigger('click')
  expect(w.getComponent({ name: 'TaskPopup' }).props('open')).toBeNull()
})

it('marks a deadline within three days and shows load errors with a retry', async () => {
  const soon = new Date(Date.now() + 24 * 3600 * 1000).toISOString()
  vi.mocked(assignmentApi.list).mockRejectedValueOnce(new Error('x')).mockResolvedValue({ items: [task('a', { due_at: soon })] })
  const w = mount(AssignmentPanel, { props: { orgId: 'o' }, global })
  await flushPromises()
  expect(w.find('[role=alert]').exists()).toBe(true)
  await w.get('[role=alert] button').trigger('click')
  await flushPromises()
  expect(w.get('.due').classes()).toContain('soon')
})
