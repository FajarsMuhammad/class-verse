import { flushPromises, mount } from '@vue/test-utils'
import { expect, it, vi } from 'vitest'
import ManageOverview from './ManageOverview.vue'
const summary = vi.hoisted(() => vi.fn<(org: string) => Promise<Record<string, number>>>().mockResolvedValue({ published_modules: 120, published_quizzes: 130, students: 205, active_assignments: 201, empty_quizzes: 2, draft_modules: 3, missing_homerooms: 0, empty_classes: 0 }))
vi.mock('@/api/classes', () => ({ classApi: { summary } }))
it('uses global aggregate counts rather than list page sizes', async () => {
 const w = mount(ManageOverview, { props: { orgId: 'org' } })
 await flushPromises()
 expect(summary).toHaveBeenCalledWith('org')
 expect(w.findAll('.summary strong').map(n => n.text())).toEqual(['120', '130', '205', '201'])
 expect(w.findAll('.attention li').map(n => n.text())).toEqual(['2 kuis belum punya soalBuka', '3 modul drafBuka'])
 await w.get('.attention button').trigger('click')
 expect(w.emitted('select')).toEqual([['quizzes']])
})
