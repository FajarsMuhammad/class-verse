import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, expect, it, vi } from 'vitest'
import GradebookView from './GradebookView.vue'
import { gradebookApi } from '@/api/gradebook'
import { classApi } from '@/api/classes'

const store = { active: { id: 'org' }, isTeacher: false }
vi.mock('@/stores/org', () => ({ useOrgStore: () => store }))
vi.mock('@/components/AppShell.vue', () => ({ default: { template: '<div><slot /></div>' } }))
vi.mock('@/api/gradebook', () => ({ gradebookApi: {
  me: vi.fn<typeof gradebookApi.me>(), student: vi.fn<typeof gradebookApi.student>(),
  roster: vi.fn<typeof gradebookApi.roster>(), csv: vi.fn<typeof gradebookApi.csv>(),
} }))
vi.mock('@/api/classes', () => ({ classApi: { years: vi.fn<typeof classApi.years>(), classes: vi.fn<typeof classApi.classes>() } }))
const report = { user_id: 'u', name: 'Ani', modules: [], assignments: [], average_best: null,
  quizzes: [{ id: 'q', title: 'Kuis', building_key: 'sains', archived: false, attempts: 0, best: null, last: null }] }
beforeEach(() => {
  vi.resetAllMocks(); store.isTeacher = false
  vi.mocked(gradebookApi.me).mockResolvedValue(report)
  vi.mocked(classApi.years).mockResolvedValue({ items: [{ id: 'y', label: '2026/2027', active: true }] })
  vi.mocked(classApi.classes).mockResolvedValue({ items: [] })
})
it('shows own report with unattempted score as not done, never zero; no staff controls', async () => {
  const w = mount(GradebookView); await flushPromises()
  expect(gradebookApi.me).toHaveBeenCalledWith('org', {})
  expect(w.text()).toContain('terbaik Belum dikerjakan')
  expect(w.text()).not.toContain('Unduh CSV')
  expect(gradebookApi.roster).not.toHaveBeenCalled()
})
it('staff loads active-year roster, pages and opens student details', async () => {
  store.isTeacher = true
  vi.mocked(gradebookApi.roster).mockResolvedValue({ total: 21, items: [{ user_id: 'u', name: 'Ani', class_name: '', modules_done: 1, modules_total: 2, quizzes_attempted: 0, quizzes_total: 1, average_best: null }] })
  vi.mocked(gradebookApi.student).mockResolvedValue(report)
  const w = mount(GradebookView); await flushPromises()
  expect(gradebookApi.roster).toHaveBeenCalledWith('org', { year_id: 'y' }, 0)
  expect(w.text()).toContain('Belum ditempatkan')
  await w.findAll('button').find(b => b.text() === 'Berikutnya')!.trigger('click'); await flushPromises()
  expect(gradebookApi.roster).toHaveBeenLastCalledWith('org', { year_id: 'y' }, 20)
  await w.get('button[aria-label="Lihat rapor Ani"]').trigger('click'); await flushPromises()
  expect(gradebookApi.student).toHaveBeenCalledWith('org', 'u', { year_id: 'y' })
})
it('surfaces load errors with retry', async () => {
  vi.mocked(gradebookApi.me).mockRejectedValueOnce(new Error('x')).mockResolvedValue(report)
  const w = mount(GradebookView); await flushPromises()
  expect(w.find('[role="alert"]').exists()).toBe(true)
  await w.get('[role="alert"] button').trigger('click'); await flushPromises()
  expect(w.text()).toContain('Ani')
})
