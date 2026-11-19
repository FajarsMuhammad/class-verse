import { flushPromises, mount } from '@vue/test-utils'
import { expect, it, vi } from 'vitest'
import ManageOverview from './ManageOverview.vue'
vi.mock('@/api/modules', () => ({ moduleApi: { list: async () => ({ items: [{ title: 'Draf', published: false }] }) } }))
vi.mock('@/api/quizzes', () => ({ quizApi: { list: async () => ({ items: [{ title: 'Kosong', published: false, question_count: 0 }] }) } }))
vi.mock('@/api/classes', () => ({ classApi: { years: async () => ({ items: [] }) } }))
vi.mock('@/api/assignments', () => ({ assignmentApi: { list: async () => ({ items: [] }) } }))
it('shows API-backed counts and prioritizes empty quizzes before drafts', async () => {
  const w = mount(ManageOverview, { props: { orgId: 'org' } })
  await flushPromises()
  expect(w.findAll('.summary strong').map(n => n.text())).toEqual(['0', '0', '0', '0'])
  expect(w.findAll('.attention li').map(n => n.text())).toEqual(['Kosong · belum punya soalBuka', 'Draf · modul drafBuka'])
  await w.get('.attention button').trigger('click')
  expect(w.emitted('select')).toEqual([['quizzes']])
})
