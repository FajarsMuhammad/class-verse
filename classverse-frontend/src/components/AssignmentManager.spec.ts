import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, expect, it, vi } from 'vitest'
import AssignmentManager from './AssignmentManager.vue'
import { assignmentApi } from '@/api/assignments'
import { classApi } from '@/api/classes'
import { moduleApi } from '@/api/modules'
vi.mock('@/api/assignments', () => ({ deadline: () => '', assignmentApi: {
  list: vi.fn<typeof assignmentApi.list>(), issue: vi.fn<typeof assignmentApi.issue>(), roster: vi.fn<typeof assignmentApi.roster>(), cancel: vi.fn<typeof assignmentApi.cancel>(),
} }))
vi.mock('@/api/classes', () => ({ classApi: { years: vi.fn<typeof classApi.years>(), classes: vi.fn<typeof classApi.classes>() } }))
vi.mock('@/api/modules', () => ({ moduleApi: { list: vi.fn<typeof moduleApi.list>() } }))
beforeEach(() => {
  vi.resetAllMocks()
  sessionStorage.clear()
  vi.mocked(assignmentApi.list).mockResolvedValue({ items: [] })
  vi.mocked(classApi.years).mockResolvedValue({ items: [{ id: 'y', label: '2026/2027', active: true }] })
  vi.mocked(classApi.classes).mockResolvedValue({ items: [{ id: 'c', academic_year_id: 'y', name: 'XI', students: 1, subject_teachers: 0, homeroom: null, my_role: null }] })
  vi.mocked(moduleApi.list).mockResolvedValue({ items: [{ id: 'm', published: true, title: 'Modul', steps: [{ id: 's' }] }] } as Awaited<ReturnType<typeof moduleApi.list>>)
})
it('retries uncertain issuance with the same client ID and locks payload edits', async () => {
  vi.mocked(assignmentApi.issue).mockRejectedValueOnce(new Error('offline')).mockResolvedValue({ id: 'a' })
  const w = mount(AssignmentManager, { props: { orgId: 'org' } })
  await flushPromises()
  const selects = w.findAll('select')
  await selects[1]!.setValue('c'); await selects[2]!.setValue('m')
  await w.get('form').trigger('submit'); await flushPromises()
  const first = vi.mocked(assignmentApi.issue).mock.calls[0]![1]
  expect(first.class_id).toBe('c')
  expect(first.module_id).toBe('m')
  expect(w.get('fieldset').attributes('disabled')).toBeDefined()
  await w.get('form').trigger('submit'); await flushPromises()
  expect(vi.mocked(assignmentApi.issue).mock.calls[1]![1].id).toBe(first.id)
})
