import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, expect, it, vi } from 'vitest'
import AssignmentPanel from './AssignmentPanel.vue'
import { assignmentApi } from '@/api/assignments'
vi.mock('@/api/assignments', () => ({ deadline: () => 'Tanpa tenggat', assignmentApi: {
  list: vi.fn<typeof assignmentApi.list>(), steps: vi.fn<typeof assignmentApi.steps>(), complete: vi.fn<typeof assignmentApi.complete>(),
} }))
const task = { id: 'a', title: 'Tugas', class_name: 'XI', year_label: '2026/2027', building_key: 'sains', level: 'SMA' as const, due_at: null, cancelled: false, done: 0, total: 2 }
const steps = [0, 1].map(i => ({ id: String(i), kind: 'materi' as const, title: `Langkah ${i}`, body: i ? '' : 'Isi', file_id: null, file_kind: null, video_id: null, quiz_id: null, minutes: 5, done: false, unlocked: !i }))
beforeEach(() => {
  vi.resetAllMocks()
  vi.mocked(assignmentApi.list).mockResolvedValue({ items: [task] })
  vi.mocked(assignmentApi.steps).mockResolvedValue({ items: steps })
})
it('keeps next step locked; opening does not complete work', async () => {
  const w = mount(AssignmentPanel, { props: { orgId: 'org' } })
  await flushPromises()
  await w.get('.task-list button').trigger('click'); await flushPromises()
  const buttons = w.findAll('ol button')
  expect(buttons[1]!.attributes('disabled')).toBeDefined()
  await buttons[0]!.trigger('click')
  expect(w.text()).toContain('Isi')
  expect(assignmentApi.complete).not.toHaveBeenCalled()
  const complete = w.findAll('button').find(b => b.text() === 'Tandai selesai')!
  await complete.trigger('click'); await flushPromises()
  expect(assignmentApi.complete).toHaveBeenCalledWith('org', 'a', '0')
})
it('keeps completion unchanged and shows saving failure', async () => {
  vi.mocked(assignmentApi.complete).mockRejectedValue(new Error('Gagal simpan'))
  const w = mount(AssignmentPanel, { props: { orgId: 'org' } })
  await flushPromises(); await w.get('.task-list button').trigger('click'); await flushPromises()
  await w.get('ol button').trigger('click')
  await w.findAll('button').find(b => b.text() === 'Tandai selesai')!.trigger('click'); await flushPromises()
  expect(w.get('[role="alert"]').text()).toContain('Tidak bisa terhubung ke server')
  expect(w.get('.content button').text()).toBe('Tandai selesai')
})

it('pages the task list by server total', async () => {
  const list = vi.spyOn(assignmentApi, 'list').mockResolvedValue({ items: [task], total: 21 })
  const w = mount(AssignmentPanel, { props: { orgId: 'o' } })
  await flushPromises()
  await w.findAll('.remote-pagination button')[1]!.trigger('click')
  await flushPromises()
  expect(list).toHaveBeenLastCalledWith('o', 20)
})
